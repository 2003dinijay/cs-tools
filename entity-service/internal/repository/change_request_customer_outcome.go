// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package repository

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// This file owns what a CUSTOMER -- an external caller: a registered contact of
// the change request's project, signed in to the customer portal -- can do to a
// change request through PATCH /change-requests/{id}, and how that lines up with
// the approvals a customer is also asked to give in the Approvals tab.
//
// Customers answer a change request in the customer portal, which sends
// PATCH {isCustomerApproved} / {isCustomerReviewed} (the contract the portal was
// built against ServiceNow with) or PATCH {plannedStartOn} ("propose new
// implementation time"). The approvals are the second mechanism:
// provisionCustomerStage asks the project's registered contacts through a
// "Customer Approval" / "Customer Review" stage, decided through
// DecideChangeRequestApproval. They are ONE mechanism with two doors here:
//
//   - PATCH {isCustomerApproved: true|false} in Customer Approval,
//     {isCustomerReviewed: true|false} in Customer Review, is the caller deciding
//     their own pending approval on that stage (answerCustomerStageViaPatch ->
//     decideChangeRequestApprovalTx, the very function the decision route runs).
//     So the calling contact's row becomes Approved / Rejected, the siblings
//     Cancelled, the state moves (Customer Approval -> Scheduled | Canceled,
//     Customer Review -> Closed | Rollback), and the flag is stamped by the
//     approving outcome, once.
//   - PATCH {plannedStartOn, plannedEndOn} in Customer Approval is the process
//     diagram's "Time Change" loop started by the customer: the same Re-schedule
//     a WSO2 user triggers with {state: authorize, ...} (new window applied, a
//     fresh CAB / ECAB approval, the customer asked again once it is given).
//
// The read side of the same rules is customerCanAnswer: the change request
// detail tells a customer, per viewer, whether the answer would be accepted
// right now (domain.ChangeRequest.CustomerCanAnswer), so the portal offers
// Approve / Reject / Propose exactly when they can work.
//
// Everything else an external caller could send is refused: entity-service does
// not rely on the portal in front of it to say what a customer may change
// (row-level security lets any member of the project update the row; it has no
// notion of fields), so the whitelist lives here too.

// externalPatchRefusal is the 403 for an external caller whose PATCH carries
// anything but the customer's own answer or a proposed window.
const externalPatchRefusal = "customers can only record the customer's approval or review (isCustomerApproved / isCustomerReviewed) or propose a new implementation time (plannedStartOn / plannedEndOn) on a change request; no other field can be changed"

// isExternalCaller reports whether ctx carries the identity of a customer or
// partner: an identity was resolved for the request and it is neither
// unrestricted (internal staff, an internal client credential) nor staff who
// also hold an external record (SearchScope.HasInternalAccess -- "external wins"
// limits what they may LIST, it does not make them a customer). A context with
// no identity is not external: Scoped refuses such a context outright, and the
// callers that legitimately have none (tests, background jobs) stamp the system
// identity.
func isExternalCaller(ctx context.Context) bool {
	scope, ok := CallerIdentityFromContext(ctx)
	return ok && !scope.Unrestricted && !scope.HasInternalAccess
}

type customerPatchKind int

const (
	// customerPatchAnswer: isCustomerApproved / isCustomerReviewed.
	customerPatchAnswer customerPatchKind = iota + 1
	// customerPatchProposal: plannedStartOn / plannedEndOn.
	customerPatchProposal
)

// customerPatch is an external caller's PATCH, classified.
type customerPatch struct {
	kind customerPatchKind
	// spec is the customer stage being answered (customerPatchAnswer).
	spec *customerStageSpec
	// approved is the answer: true approves / confirms, false rejects.
	approved bool
	// flag names the field in messages.
	flag string
}

// classifyExternalPatch decides what an external caller's PATCH is, or refuses
// it. The request must be exactly one of the customer's answer (one of
// isCustomerApproved / isCustomerReviewed) or a proposed window, and carry
// nothing else: the whitelist is "these four fields", checked by clearing them
// and requiring the rest of the request to be empty, so a field added to the
// request later is refused until someone decides a customer may set it.
func classifyExternalPatch(req domain.PatchChangeRequestRequest) (customerPatch, error) {
	rest := req
	rest.IsCustomerApproved, rest.IsCustomerReviewed, rest.PlannedStartOn, rest.PlannedEndOn = nil, nil, nil, nil
	if !reflect.DeepEqual(rest, domain.PatchChangeRequestRequest{}) {
		return customerPatch{}, &apierror.ForbiddenError{Msg: externalPatchRefusal}
	}
	hasAnswer := req.IsCustomerApproved != nil || req.IsCustomerReviewed != nil
	hasWindow := req.PlannedStartOn != nil || req.PlannedEndOn != nil
	switch {
	case hasAnswer && hasWindow:
		return customerPatch{}, &apierror.ValidationError{Msg: "a proposed implementation time (plannedStartOn / plannedEndOn) and the customer's approval or review (isCustomerApproved / isCustomerReviewed) must be sent in separate requests"}
	case req.IsCustomerApproved != nil && req.IsCustomerReviewed != nil:
		return customerPatch{}, &apierror.ValidationError{Msg: "send either isCustomerApproved or isCustomerReviewed, not both"}
	case req.IsCustomerApproved != nil:
		return customerPatch{kind: customerPatchAnswer, spec: &customerApprovalStageSpec, approved: *req.IsCustomerApproved, flag: "isCustomerApproved"}, nil
	case req.IsCustomerReviewed != nil:
		return customerPatch{kind: customerPatchAnswer, spec: &customerReviewStageSpec, approved: *req.IsCustomerReviewed, flag: "isCustomerReviewed"}, nil
	case hasWindow:
		return customerPatch{kind: customerPatchProposal}, nil
	}
	return customerPatch{}, &apierror.ValidationError{Msg: "at least one field must be provided"}
}

// lockCustomerAnswerRow bumps the change request's work_item (updated_on /
// updated_by, what a PATCH always does) and thereby locks it FIRST, then returns
// its project id. Taking the work_item lock before the change_request one is the
// order every other PATCH takes them in; going the other way round would let a
// customer's answer and a concurrent edit deadlock each other. Under row-level
// security a caller who is not a member of the project updates no row, which is
// reported as not found, exactly as the ordinary PATCH reports it.
func lockCustomerAnswerRow(ctx context.Context, tx pgx.Tx, id, actorEmail string) (*string, error) {
	var projectID *string
	err := tx.QueryRow(ctx,
		`UPDATE work_item SET updated_on = NOW(), updated_by = $2
		 WHERE id = $1 AND type = 'CHANGE_REQUEST' RETURNING project_id::text`, id, actorEmail).Scan(&projectID)
	if errors.Is(err, pgx.ErrNoRows) || IsRLSPolicyViolation(err) {
		return nil, &apierror.NotFoundError{Msg: "change request not found"}
	}
	if err != nil {
		return nil, fmt.Errorf("answer change request: lock work item: %w", err)
	}
	return projectID, nil
}

// requireRegisteredContact refuses (403) a caller who is not a REGISTERED
// PORTAL_USER contact of the change request's own project -- the same test the
// customer flag has always used (callerMayGrantChangeRequestCustomerFlag), and
// the reason a customer of ANOTHER project is refused whatever role the portal
// gave them.
func requireRegisteredContact(ctx context.Context, tx pgx.Tx, projectID *string, actorEmail string) error {
	ok, err := callerMayGrantChangeRequestCustomerFlag(ctx, tx, projectID, actorEmail)
	if err != nil {
		return err
	}
	if !ok {
		return &apierror.ForbiddenError{Msg: "only a registered PORTAL_USER contact on this change request's own project may give the customer's answer on it"}
	}
	return nil
}

// customerApproverUserID resolves the caller's "user" id from their email: the
// id their approver rows are recorded against. Where an email has more than one
// user row, the one holding a pending approval on this change request is chosen.
func customerApproverUserID(ctx context.Context, tx crQuerier, workItemID, actorEmail string) (string, error) {
	var userID string
	err := tx.QueryRow(ctx, `
		SELECT u.id::text FROM "user" u
		WHERE LOWER(u.email) = LOWER($2)
		ORDER BY EXISTS (SELECT 1 FROM approval_stage_approver asa
		                  WHERE asa.work_item_id = $1 AND asa.approver_user_id = u.id AND asa.status = 'requested') DESC,
		         u.created_on ASC
		LIMIT 1`, workItemID, actorEmail).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", &apierror.ForbiddenError{Msg: "no user record was found for the caller"}
	}
	if err != nil {
		return "", fmt.Errorf("answer change request: resolve caller: %w", err)
	}
	return userID, nil
}

// customerCanAnswer reports whether viewerEmail -- a customer -- could give the
// customer's approval / review of the change request id (in the given state,
// of the project projectID) right now. It is the read-only twin of
// answerCustomerStageViaPatch: the same checks, in the same order, stopping at
// the first that would refuse the answer, but locking and writing nothing, so
// that what the portal offers and what the PATCH accepts cannot drift apart:
//
//  1. the state is Customer Approval or Customer Review (customerStageSpecForState),
//     else false -- the answer belongs to one state;
//  2. the viewer is a REGISTERED PORTAL_USER contact of the change request's own
//     project (requireRegisteredContact's test);
//  3. the customer's request is still pending: a live customer stage of that
//     state (liveCustomerStageForState);
//  4. the viewer holds a REQUESTED approval on that live stage -- not a contact
//     the request was never sent to, not one whose row a sibling's answer
//     cancelled, not one whose stage a Re-schedule superseded;
//  5. nothing blocks them from deciding it (approverDecisionBlock: the creator
//     of the change request never approves it).
//
// markCanDecide answers the same question for the approvals read; they share
// approverDecisionBlock and changeRequestCreatorsForApprover for the who-may
// rule and the live-stage helpers for the what-is-pending rule.
//
// An error is a failure to find out (never "no"); the caller decides what an
// unknown answer means.
func customerCanAnswer(ctx context.Context, q crQuerier, id string, projectID *string, state, viewerEmail string) (bool, error) {
	spec := customerStageSpecForState(strings.ToUpper(strings.TrimSpace(state)))
	if spec == nil || strings.TrimSpace(viewerEmail) == "" {
		return false, nil
	}
	ok, err := callerMayGrantChangeRequestCustomerFlag(ctx, q, projectID, viewerEmail)
	if err != nil || !ok {
		return false, err
	}
	live, err := liveCustomerStageForState(ctx, q, id, spec.state)
	if err != nil || live == nil {
		return false, err
	}
	userID, err := customerApproverUserID(ctx, q, id, viewerEmail)
	if err != nil {
		var forbidden *apierror.ForbiddenError
		if errors.As(err, &forbidden) {
			return false, nil
		}
		return false, err
	}
	var asked bool
	if err := q.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM approval_stage_approver
		                WHERE stage_id = $1::uuid AND approver_user_id = $2::uuid AND status = 'requested')`,
		live.stageID, userID).Scan(&asked); err != nil {
		return false, fmt.Errorf("customer can answer: read the viewer's pending approval: %w", err)
	}
	if !asked {
		return false, nil
	}
	creatorIDs, err := changeRequestCreatorsForApprover(ctx, q, id, userID, viewerEmail)
	if err != nil {
		return false, fmt.Errorf("customer can answer: %w", err)
	}
	if err := approverDecisionBlock(ctx, q, userID, creatorIDs, spec.kind); err != nil {
		var forbidden *apierror.ForbiddenError
		if errors.As(err, &forbidden) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// markCustomerCanAnswer sets domain.ChangeRequest.CustomerCanAnswer for the
// caller reading cr, when the caller is a customer (an external caller): true
// or false, per customerCanAnswer. Left nil for everyone else (staff, internal
// callers, a context with no identity) -- the field answers a customer's
// question, and a staff view has no customer answer of its own to give -- and
// when the check itself fails: the detail read is not worth failing for it, and
// an absent field tells the client the answer is unknown rather than "no".
func (r *changeRequestRepo) markCustomerCanAnswer(ctx context.Context, cr *domain.ChangeRequest) {
	if !isExternalCaller(ctx) {
		return
	}
	scope, _ := CallerIdentityFromContext(ctx)
	var projectID *string
	if cr.Project.ID != "" {
		pid := cr.Project.ID
		projectID = &pid
	}
	state := ""
	if cr.State != nil {
		state = *cr.State
	}
	can, err := customerCanAnswer(ctx, r.db, cr.ID, projectID, state, scope.ViewerEmail)
	if err != nil {
		slog.WarnContext(ctx, "get change request: customerCanAnswer left unset", "changeRequestId", cr.ID, "error", err)
		return
	}
	cr.CustomerCanAnswer = &can
}

// stateForMessage is the change's (upper-case) state as a message names it: a
// NULL state is a pre-lifecycle legacy row and counts as New, like everywhere
// else.
func stateForMessage(state string) string {
	if strings.TrimSpace(state) == "" {
		return crStateNew
	}
	return state
}

// answerCustomerStageViaPatch is PATCH {isCustomerApproved|isCustomerReviewed}
// from an external caller: the caller answering the customer's approval / review
// of the change request, recorded exactly as the decision route records it.
//
// In order (the first failing check wins, and nothing is written until all pass):
//
//  1. the change request must be visible to the caller (404 otherwise);
//  2. the caller must be a registered PORTAL_USER contact of its own project (403:
//     a customer of another project, a contact who holds no PORTAL_USER role, one
//     who is only invited);
//  3. it must be in the state the answer belongs to -- Customer Approval for
//     isCustomerApproved, Customer Review for isCustomerReviewed -- else 409, the
//     stale-approval refusal (staleApprovalRefusal) the decision route gives. This
//     is also what a second contact gets once the first has answered;
//  4. a rejection of a flag that is already true is refused (400): once the
//     customer's approval / review is recorded it is final;
//  5. the customer's request must still be pending (a live stage), else 409: with
//     nobody asked there is nothing to approve here, and WSO2 records the answer;
//  6. the answer is the caller's own pending approval, decided by
//     decideChangeRequestApprovalTx: the creator may not (403), a contact who was
//     not asked may not (403), the caller's row becomes Approved / Rejected, the
//     others Cancelled, the state moves and an approval stamps the flag.
//
// Returns the change request's id.
func answerCustomerStageViaPatch(ctx context.Context, tx pgx.Tx, id string, p customerPatch, actorEmail string) (string, error) {
	spec := p.spec
	projectID, err := lockCustomerAnswerRow(ctx, tx, id, actorEmail)
	if err != nil {
		return "", err
	}
	if err := requireRegisteredContact(ctx, tx, projectID, actorEmail); err != nil {
		return "", err
	}
	gates, err := lockChangeRequestGateSnapshot(ctx, tx, id)
	if err != nil {
		return "", err
	}
	if gates.state != spec.state {
		return "", staleApprovalRefusal(spec.kind, stateForMessage(gates.state))
	}

	if !p.approved {
		var stamped bool
		if err := tx.QueryRow(ctx,
			fmt.Sprintf(`SELECT COALESCE(%s, false) FROM change_request WHERE id = $1`, spec.approvedFlagColumn), id).Scan(&stamped); err != nil {
			return "", fmt.Errorf("answer change request: read %s: %w", spec.approvedFlagColumn, err)
		}
		if stamped {
			return "", &apierror.ValidationError{Msg: p.flag + " is locked once set to true and cannot be reverted to false"}
		}
	}

	live, err := liveCustomerStageForState(ctx, tx, id, gates.state)
	if err != nil {
		return "", fmt.Errorf("answer change request: %w", err)
	}
	if live == nil {
		return "", &apierror.ConflictError{Msg: fmt.Sprintf(
			"no customer %s is pending on this change request: it has not been requested from the project's registered contacts, so there is nothing to answer here and WSO2 records the customer's %s", spec.what, spec.what)}
	}

	userID, err := customerApproverUserID(ctx, tx, id, actorEmail)
	if err != nil {
		return "", err
	}
	decision := "rejected"
	if p.approved {
		decision = "approved"
	}
	if _, err := decideChangeRequestApprovalTx(ctx, tx, id, userID, decision, actorEmail); err != nil {
		return "", err
	}
	return id, nil
}

// prepareCustomerProposal checks an external caller's proposed implementation
// window and turns the request into the Re-schedule it is: patchChangeRequestTx
// carries on with {state: authorize, plannedStartOn?, plannedEndOn?}, the exact
// request a WSO2 user sends to re-plan a change in Customer Approval, so the new
// window, the fresh CAB / ECAB stage (or, for a Standard change, the customer
// asked again) and the "Time Change = Yes" check all come from the one
// implementation.
//
// Refused before anything is written: a change request that is not visible
// (404), a caller who is not a registered contact of its project (403), one who
// is the change's creator (403), a change not in Customer Approval (409 -- the
// customer's approval is the only place a new time can be proposed), and a
// change on hold (409).
func prepareCustomerProposal(ctx context.Context, tx pgx.Tx, id string, req domain.PatchChangeRequestRequest, actorEmail string) (domain.PatchChangeRequestRequest, error) {
	projectID, err := lockCustomerAnswerRow(ctx, tx, id, actorEmail)
	if err != nil {
		return req, err
	}
	if err := requireRegisteredContact(ctx, tx, projectID, actorEmail); err != nil {
		return req, err
	}
	gates, err := lockChangeRequestGateSnapshot(ctx, tx, id)
	if err != nil {
		return req, err
	}
	if gates.state != crStateCustomerApproval {
		return req, &apierror.ConflictError{Msg: fmt.Sprintf(
			"a new implementation time can only be proposed while the change request is in Customer Approval, but it is in %s",
			changeRequestStateDisplayName(stateForMessage(gates.state)))}
	}

	userID, err := customerApproverUserID(ctx, tx, id, actorEmail)
	if err != nil {
		return req, err
	}
	creatorIDs, err := changeRequestCreatorsForApprover(ctx, tx, id, userID, actorEmail)
	if err != nil {
		return req, fmt.Errorf("propose implementation time: %w", err)
	}
	if err := approverDecisionBlock(ctx, tx, userID, creatorIDs, stageKindCustomerApproval); err != nil {
		return req, err
	}

	var onHold *bool
	if err := tx.QueryRow(ctx, `SELECT is_on_hold FROM change_request WHERE id = $1`, id).Scan(&onHold); err != nil {
		return req, fmt.Errorf("propose implementation time: read on-hold state: %w", err)
	}
	if onHold != nil && *onHold {
		return req, &apierror.ConflictError{Msg: "this change request is on hold, so a new implementation time cannot be proposed now"}
	}

	reschedule := domain.ChangeRequestStateAuthorize
	req.State = &reschedule
	return req, nil
}
