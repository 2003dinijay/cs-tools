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

package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// autoPublishSecurityTagLabel must match the webapp's own
// SECURITY_ANNOUNCEMENT_TAG_LABEL constant
// (CreateCustomerAnnouncementForm.tsx) exactly — AutoPublish attaches the
// identical tag a manual, browser-driven Publish would, just from a
// different (server-side) caller.
const autoPublishSecurityTagLabel = "Security Announcement"

// caseFanOutClient is the narrow subset of CaseService AutoPublish needs to
// create real cases and attach the mandatory security tag, in-process. This
// can't go through the public HTTP case-creation endpoints at all — both
// CaseService.CreateCase and AddCaseTag hard-require a real browser user's
// x-user-id-token to resolve who's acting (see case_service.go), which
// neither this service nor a scheduled job ever has. Calling the same
// CaseService methods directly, in-process, sidesteps that entirely: the
// actor (CreatedBy) is already known from the announcement_request row
// itself, so there's nothing to resolve from a token at all.
type caseFanOutClient interface {
	CreateCase(ctx context.Context, req domain.CreateCaseRequest) (domain.CreateCaseResponse, error)
	AddCaseTag(ctx context.Context, caseID, label string) (domain.Tag, error)
}

type announcementRequestService struct {
	repo   repository.AnnouncementRequestRepository
	cases  caseFanOutClient
	access AccessService
}

// NewAnnouncementRequestService constructs an AnnouncementRequestService
// backed by the given repository. cases/access back AutoPublish only (see
// its own doc comment) — every other method here ignores both.
func NewAnnouncementRequestService(repo repository.AnnouncementRequestRepository, cases caseFanOutClient, access AccessService) AnnouncementRequestService {
	return &announcementRequestService{repo: repo, cases: cases, access: access}
}

// requireInternalCaller rejects anyone whose AccessScope is not Unrestricted
// — mirrors onboarding_step_service.go's/sla_status_service.go's own helper
// of the same name and same reasoning: AutoPublish has no per-caller scope
// short of "internal service" that would be safe to hand this out under, the
// same rationale sla_status_service.go's own copy documents.
func (s *announcementRequestService) requireInternalCaller(ctx context.Context) error {
	scope, err := s.access.ResolveScope(ctx)
	if err != nil {
		return err
	}
	if !scope.Unrestricted {
		return &apierror.ForbiddenError{Msg: "auto-publish is only available to internal services"}
	}
	return nil
}

// CreateDraft implements AnnouncementRequestService.
func (s *announcementRequestService) CreateDraft(ctx context.Context, req domain.CreateAnnouncementRequestRequest) (domain.AnnouncementRequest, error) {
	if req.Kind != domain.AnnouncementRequestKindCustomer && req.Kind != domain.AnnouncementRequestKindEOL {
		return domain.AnnouncementRequest{}, &apierror.ValidationError{Msg: "kind must be \"customer\" or \"eol\""}
	}
	if strings.TrimSpace(req.CreatedBy) == "" {
		return domain.AnnouncementRequest{}, &apierror.ValidationError{Msg: "createdBy is required"}
	}
	return s.repo.Create(ctx, req)
}

// Get implements AnnouncementRequestService.
func (s *announcementRequestService) Get(ctx context.Context, id string) (domain.AnnouncementRequest, error) {
	if strings.TrimSpace(id) == "" {
		return domain.AnnouncementRequest{}, &apierror.ValidationError{Msg: "id is required"}
	}
	return s.repo.Get(ctx, id)
}

// Search implements AnnouncementRequestService.
func (s *announcementRequestService) Search(ctx context.Context, req domain.SearchAnnouncementRequestsRequest) (domain.SearchAnnouncementRequestsResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchAnnouncementRequestsResponse{}, err
	}
	if req.State != nil && !isValidAnnouncementRequestState(*req.State) {
		return domain.SearchAnnouncementRequestsResponse{}, &apierror.ValidationError{Msg: "state must be one of: draft, pending_approval, approved, published"}
	}
	if req.ReadyForScheduledPublish && req.State != nil {
		return domain.SearchAnnouncementRequestsResponse{}, &apierror.ValidationError{Msg: "readyForScheduledPublish cannot be combined with state"}
	}

	requests, total, err := s.repo.Search(ctx, req)
	if err != nil {
		return domain.SearchAnnouncementRequestsResponse{}, err
	}
	return domain.SearchAnnouncementRequestsResponse{
		Requests: requests,
		Total:    total,
		Limit:    req.Pagination.Limit,
		Offset:   req.Pagination.Offset,
		HasMore:  req.Pagination.Offset+len(requests) < total,
	}, nil
}

// Update implements AnnouncementRequestService.
//
// What actually happens is entirely decided by the row's *current* state,
// not by anything the caller chooses:
//   - draft: a plain field update, no state change.
//   - pending_approval: the same field update, but also reverts to draft
//     as one atomic side effect (clearing the dry-run record and the
//     frozen audience snapshot) — the content is out for real review over
//     email, so a silent change under the reviewer isn't safe, and the old
//     dry run no longer describes whatever's about to be re-submitted.
//   - approved: subject/description/security-flag may still be updated in
//     place with no state change and no audience change — a human has
//     already said yes over email, so this is a deliberate trade-off,
//     accepted explicitly: a post-approval edit is not re-verified against
//     a fresh dry run before Publish. AudienceDefinition is rejected here
//     (the approved snapshot must never silently change).
//   - published: rejected — nothing about a published request is editable
//     through this entity; the real cases it fanned out into are the
//     record from this point on.
func (s *announcementRequestService) Update(ctx context.Context, id string, req domain.UpdateAnnouncementRequestRequest) (domain.AnnouncementRequest, error) {
	if strings.TrimSpace(req.ActorID) == "" {
		return domain.AnnouncementRequest{}, &apierror.ValidationError{Msg: "actorId is required"}
	}

	current, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.AnnouncementRequest{}, err
	}

	switch current.State {
	case domain.AnnouncementRequestStateDraft:
		return s.repo.Update(ctx, id, domain.AnnouncementRequestStateDraft, req)
	case domain.AnnouncementRequestStatePendingApproval:
		return s.repo.RevertToDraft(ctx, id, req)
	case domain.AnnouncementRequestStateApproved:
		if len(req.AudienceDefinition) > 0 {
			return domain.AnnouncementRequest{}, &apierror.ValidationError{Msg: "audience cannot be changed on an approved request — the approved audience snapshot is frozen"}
		}
		return s.repo.Update(ctx, id, domain.AnnouncementRequestStateApproved, req)
	default: // published
		return domain.AnnouncementRequest{}, &apierror.ConflictError{Msg: "a published announcement request cannot be edited"}
	}
}

// RecordDryRun implements AnnouncementRequestService. Allowed only from
// draft — recording a dry run against a request that's already left draft
// makes no sense (pending_approval/approved already snapshot a specific
// version, and a published request is done).
func (s *announcementRequestService) RecordDryRun(ctx context.Context, id string, req domain.RecordAnnouncementDryRunRequest) (domain.AnnouncementRequest, error) {
	if strings.TrimSpace(req.CaseID) == "" {
		return domain.AnnouncementRequest{}, &apierror.ValidationError{Msg: "caseId is required"}
	}
	if strings.TrimSpace(req.ActorID) == "" {
		return domain.AnnouncementRequest{}, &apierror.ValidationError{Msg: "actorId is required"}
	}

	current, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.AnnouncementRequest{}, err
	}
	if current.State != domain.AnnouncementRequestStateDraft {
		return domain.AnnouncementRequest{}, &apierror.ConflictError{Msg: "a dry run can only be recorded while the request is in draft, not " + string(current.State)}
	}
	return s.repo.RecordDryRun(ctx, id, req)
}

// Submit implements AnnouncementRequestService. Rejects unless the current
// state is draft and a dry run has already been recorded — the dry-run case
// is the only preview the approver ever sees (there is no other rendering
// surface in this slice), so submitting without one would send an approval
// request for content nobody has actually looked at rendered.
func (s *announcementRequestService) Submit(ctx context.Context, id string, req domain.SubmitAnnouncementRequestRequest) (domain.AnnouncementRequest, error) {
	if strings.TrimSpace(req.ActorID) == "" {
		return domain.AnnouncementRequest{}, &apierror.ValidationError{Msg: "actorId is required"}
	}
	if len(req.ResolvedProjectIDs) == 0 {
		return domain.AnnouncementRequest{}, &apierror.ValidationError{Msg: "resolvedProjectIds must not be empty"}
	}

	current, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.AnnouncementRequest{}, err
	}
	if current.State != domain.AnnouncementRequestStateDraft {
		return domain.AnnouncementRequest{}, &apierror.ConflictError{Msg: "only a draft can be submitted for approval, not " + string(current.State)}
	}
	if current.DryRunCaseID == nil {
		return domain.AnnouncementRequest{}, &apierror.ConflictError{Msg: "a dry run must be recorded before submitting for approval"}
	}
	return s.repo.Submit(ctx, id, req)
}

// Approve implements AnnouncementRequestService. Rejects unless the current
// state is pending_approval. There is deliberately no approver-role check
// here — the real approval decision already happened over email, outside
// this service; this call only records that whoever is working the
// request says it's been approved.
func (s *announcementRequestService) Approve(ctx context.Context, id, actorID, actorEmail string) (domain.AnnouncementRequest, error) {
	if strings.TrimSpace(actorID) == "" {
		return domain.AnnouncementRequest{}, &apierror.ValidationError{Msg: "actorId is required"}
	}

	current, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.AnnouncementRequest{}, err
	}
	if current.State != domain.AnnouncementRequestStatePendingApproval {
		return domain.AnnouncementRequest{}, &apierror.ConflictError{Msg: "only a request pending approval can be approved, not " + string(current.State)}
	}
	return s.repo.Approve(ctx, id, actorID, actorEmail)
}

// MarkPublished implements AnnouncementRequestService. Rejects unless the
// current state is approved. This never creates the real per-project cases
// itself — the caller (the webapp's own publish flow) does that fan-out
// exactly as it already does today; this call only records that it
// happened, by whom, when, and which case ids resulted.
func (s *announcementRequestService) MarkPublished(ctx context.Context, id, actorID, actorEmail string, caseIDs []string) (domain.AnnouncementRequest, error) {
	if strings.TrimSpace(actorID) == "" {
		return domain.AnnouncementRequest{}, &apierror.ValidationError{Msg: "actorId is required"}
	}
	if len(caseIDs) == 0 {
		return domain.AnnouncementRequest{}, &apierror.ValidationError{Msg: "caseIds must not be empty"}
	}

	current, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.AnnouncementRequest{}, err
	}
	if current.State != domain.AnnouncementRequestStateApproved {
		return domain.AnnouncementRequest{}, &apierror.ConflictError{Msg: "only an approved request can be published, not " + string(current.State)}
	}
	// Publish sends real cases to real customers, so it's restricted to the
	// request's own creator -- an approver's job is only to approve; per
	// Danidu's explicit instruction, they must not also be able to trigger
	// the send themselves. Self-approval (a creator approving their own
	// request) is deliberately still allowed for now, unlike this -- there's
	// no approver-role concept yet to enforce a "different person" rule at
	// that step, and that's an intentional, temporary gap to revisit once
	// one exists. This check has no such excuse: CreatedBy is always known
	// and unambiguous.
	if current.CreatedBy != actorID {
		return domain.AnnouncementRequest{}, &apierror.ForbiddenError{Msg: "only the request's creator can publish it"}
	}
	return s.repo.MarkPublished(ctx, id, actorID, actorEmail, caseIDs)
}

// Schedule implements AnnouncementRequestService. Sets or clears (nil)
// scheduledFor for an approved request -- creator-only, same rule and same
// reasoning as MarkPublished's own check: scheduling *is* choosing when
// Publish happens, so it needs the identical restriction. A non-nil
// scheduledFor must be strictly in the future; scheduling "now" or the past
// makes no sense here -- use Publish directly instead.
func (s *announcementRequestService) Schedule(ctx context.Context, id, actorID, actorEmail string, scheduledFor *time.Time) (domain.AnnouncementRequest, error) {
	if strings.TrimSpace(actorID) == "" {
		return domain.AnnouncementRequest{}, &apierror.ValidationError{Msg: "actorId is required"}
	}
	if scheduledFor != nil && !scheduledFor.After(time.Now()) {
		return domain.AnnouncementRequest{}, &apierror.ValidationError{Msg: "scheduledFor must be in the future"}
	}

	current, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.AnnouncementRequest{}, err
	}
	if current.State != domain.AnnouncementRequestStateApproved {
		return domain.AnnouncementRequest{}, &apierror.ConflictError{Msg: "only an approved request can be scheduled, not " + string(current.State)}
	}
	if current.CreatedBy != actorID {
		return domain.AnnouncementRequest{}, &apierror.ForbiddenError{Msg: "only the request's creator can schedule it"}
	}
	return s.repo.SetSchedule(ctx, id, scheduledFor)
}

// AutoPublish implements AnnouncementRequestService. Internal-caller-only
// (see requireInternalCaller) — the automatic-publish path
// operations/csm-scheduled-tasks' publish_scheduled_announcements sub-cron
// calls once ScheduledFor has arrived. Runs the identical fan-out a manual,
// browser-driven Publish does (create a case per unresolved project, attach
// the mandatory security tag, record the outcome to the delivery ledger,
// mark published once every project has succeeded) but in-process, using
// the row's own CreatedBy/CreatedByEmail for case attribution — there is no
// browser session to authenticate this call with at all, unlike the
// webapp's own usePublishAnnouncementRequest hook, which remains completely
// unchanged and still drives the manual path itself.
//
// Resumable exactly like the manual flow's own hydration: it reads
// ListDeliveries first and only attempts whatever's still outstanding, so
// calling this repeatedly for the same request (as the sub-cron does, once
// per tick, until it succeeds) never re-creates an already-succeeded
// project's case. A pass that still has outstanding failures or tag
// failures returns a ConflictError and leaves the row approved — the next
// tick retries just what's left, identical to the manual "Retry failed
// projects" button.
//
// Sequential, not concurrent, unlike the webapp's own fan-out (which limits
// concurrency purely for a human's browser-side responsiveness) — this runs
// as a background job with no one waiting on it, so the simplicity of one
// project at a time outweighs any benefit from parallelizing here.
func (s *announcementRequestService) AutoPublish(ctx context.Context, id string) (domain.AnnouncementRequest, error) {
	if err := s.requireInternalCaller(ctx); err != nil {
		return domain.AnnouncementRequest{}, err
	}

	current, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.AnnouncementRequest{}, err
	}
	if current.State != domain.AnnouncementRequestStateApproved {
		return domain.AnnouncementRequest{}, &apierror.ConflictError{Msg: "only an approved request can be auto-published, not " + string(current.State)}
	}
	if current.ScheduledFor == nil || current.ScheduledFor.After(time.Now()) {
		return domain.AnnouncementRequest{}, &apierror.ConflictError{Msg: "this request's scheduled time has not arrived"}
	}
	if len(current.ResolvedProjectIDs) == 0 {
		return domain.AnnouncementRequest{}, &apierror.ConflictError{Msg: "this request has no resolved audience to publish to"}
	}

	deliveries, err := s.repo.ListDeliveries(ctx, id)
	if err != nil {
		return domain.AnnouncementRequest{}, err
	}

	succeeded := make(map[string]bool, len(deliveries))
	caseIDByProject := make(map[string]string, len(deliveries))
	failedTagCaseByProject := make(map[string]string)
	for _, d := range deliveries {
		switch d.Status {
		case domain.AnnouncementRequestDeliveryStatusSucceeded:
			succeeded[d.ProjectID] = true
			if d.CaseID != nil {
				caseIDByProject[d.ProjectID] = *d.CaseID
			}
		case domain.AnnouncementRequestDeliveryStatusTagFailed:
			// The case is real either way — must not be re-created, only
			// its tag retried below.
			succeeded[d.ProjectID] = true
			if d.CaseID != nil {
				caseIDByProject[d.ProjectID] = *d.CaseID
				failedTagCaseByProject[d.ProjectID] = *d.CaseID
			}
		}
		// failed: nothing to seed — case creation is retried in the pending
		// loop below, same as a project with no delivery row at all.
	}

	var passEntries []domain.RecordAnnouncementRequestDeliveryInput
	var stillFailingTags []string

	// Retry any earlier tag failures first, reusing the case that already
	// exists rather than creating a second one for the same project.
	for projectID, caseID := range failedTagCaseByProject {
		caseID := caseID
		if _, err := s.cases.AddCaseTag(ctx, caseID, autoPublishSecurityTagLabel); err != nil {
			stillFailingTags = append(stillFailingTags, projectID)
			passEntries = append(passEntries, domain.RecordAnnouncementRequestDeliveryInput{ProjectID: projectID, CaseID: &caseID, Status: domain.AnnouncementRequestDeliveryStatusTagFailed})
			continue
		}
		passEntries = append(passEntries, domain.RecordAnnouncementRequestDeliveryInput{ProjectID: projectID, CaseID: &caseID, Status: domain.AnnouncementRequestDeliveryStatusSucceeded})
	}

	// Fan out to every resolved project with no successful delivery yet.
	var stillFailingCases []string
	for _, projectID := range current.ResolvedProjectIDs {
		if succeeded[projectID] {
			continue
		}
		created, err := s.cases.CreateCase(ctx, domain.CreateCaseRequest{
			CreatedBy:   current.CreatedBy,
			Type:        "announcement",
			ProjectID:   projectID,
			Subject:     current.Subject,
			Description: current.Description,
		})
		if err != nil {
			stillFailingCases = append(stillFailingCases, projectID)
			passEntries = append(passEntries, domain.RecordAnnouncementRequestDeliveryInput{ProjectID: projectID, Status: domain.AnnouncementRequestDeliveryStatusFailed})
			continue
		}
		caseID := created.Case.ID
		caseIDByProject[projectID] = caseID
		if current.IsSecurityAnnouncement {
			if _, err := s.cases.AddCaseTag(ctx, caseID, autoPublishSecurityTagLabel); err != nil {
				passEntries = append(passEntries, domain.RecordAnnouncementRequestDeliveryInput{ProjectID: projectID, CaseID: &caseID, Status: domain.AnnouncementRequestDeliveryStatusTagFailed})
				continue
			}
		}
		passEntries = append(passEntries, domain.RecordAnnouncementRequestDeliveryInput{ProjectID: projectID, CaseID: &caseID, Status: domain.AnnouncementRequestDeliveryStatusSucceeded})
	}

	if len(passEntries) > 0 {
		if _, err := s.RecordDeliveries(ctx, id, current.CreatedBy, passEntries); err != nil {
			return domain.AnnouncementRequest{}, err
		}
	}
	if len(stillFailingCases) > 0 || len(stillFailingTags) > 0 {
		return domain.AnnouncementRequest{}, &apierror.ConflictError{Msg: fmt.Sprintf(
			"not yet fully delivered — %d project(s) failed case creation, %d project(s) failed the security tag; will retry next tick",
			len(stillFailingCases), len(stillFailingTags),
		)}
	}

	caseIDs := make([]string, 0, len(caseIDByProject))
	for _, caseID := range caseIDByProject {
		caseIDs = append(caseIDs, caseID)
	}
	actorEmail := ""
	if current.CreatedByEmail != nil {
		actorEmail = *current.CreatedByEmail
	}
	return s.MarkPublished(ctx, id, current.CreatedBy, actorEmail, caseIDs)
}

// AddUpdate implements AnnouncementRequestService. Rejects unless the
// current state is published and actorID matches the request's own
// CreatedBy -- same creator-only restriction as MarkPublished, and for the
// same reason: this is what gates who can post a follow-up that will be
// applied as a real comment on every one of PublishedCaseIDs.
func (s *announcementRequestService) AddUpdate(ctx context.Context, id, actorID, actorEmail, content string) (domain.AnnouncementRequestUpdate, error) {
	if strings.TrimSpace(actorID) == "" {
		return domain.AnnouncementRequestUpdate{}, &apierror.ValidationError{Msg: "actorId is required"}
	}
	if strings.TrimSpace(content) == "" {
		return domain.AnnouncementRequestUpdate{}, &apierror.ValidationError{Msg: "content is required"}
	}

	current, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.AnnouncementRequestUpdate{}, err
	}
	if current.State != domain.AnnouncementRequestStatePublished {
		return domain.AnnouncementRequestUpdate{}, &apierror.ConflictError{Msg: "an update can only be posted for a published request, not " + string(current.State)}
	}
	if current.CreatedBy != actorID {
		return domain.AnnouncementRequestUpdate{}, &apierror.ForbiddenError{Msg: "only the request's creator can post an update"}
	}
	return s.repo.CreateUpdate(ctx, id, content, actorID, actorEmail)
}

// ListUpdates implements AnnouncementRequestService.
func (s *announcementRequestService) ListUpdates(ctx context.Context, id string) (domain.SearchAnnouncementRequestUpdatesResponse, error) {
	if strings.TrimSpace(id) == "" {
		return domain.SearchAnnouncementRequestUpdatesResponse{}, &apierror.ValidationError{Msg: "id is required"}
	}
	// Confirms the request itself exists (a NotFoundError, not an empty
	// list, for a bad id) before listing what may legitimately be zero
	// updates for a real one.
	if _, err := s.repo.Get(ctx, id); err != nil {
		return domain.SearchAnnouncementRequestUpdatesResponse{}, err
	}
	updates, err := s.repo.ListUpdates(ctx, id)
	if err != nil {
		return domain.SearchAnnouncementRequestUpdatesResponse{}, err
	}
	return domain.SearchAnnouncementRequestUpdatesResponse{Updates: updates}, nil
}

var validAnnouncementRequestDeliveryStatus = map[domain.AnnouncementRequestDeliveryStatus]bool{
	domain.AnnouncementRequestDeliveryStatusSucceeded: true,
	domain.AnnouncementRequestDeliveryStatusTagFailed: true,
	domain.AnnouncementRequestDeliveryStatusFailed:    true,
}

// RecordDeliveries implements AnnouncementRequestService. Rejects unless the
// current state is approved -- the state Publish's own fan-out runs in,
// before the request reaches published. Every delivery's projectId must be
// one this request actually resolved to (current.ResolvedProjectIDs); a
// caller recording a delivery for any other project is a bug in the caller,
// not a legitimate partial-batch case.
func (s *announcementRequestService) RecordDeliveries(ctx context.Context, id, actorID string, deliveries []domain.RecordAnnouncementRequestDeliveryInput) (domain.SearchAnnouncementRequestDeliveriesResponse, error) {
	if strings.TrimSpace(actorID) == "" {
		return domain.SearchAnnouncementRequestDeliveriesResponse{}, &apierror.ValidationError{Msg: "actorId is required"}
	}
	if len(deliveries) == 0 {
		return domain.SearchAnnouncementRequestDeliveriesResponse{}, &apierror.ValidationError{Msg: "deliveries must not be empty"}
	}

	current, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.SearchAnnouncementRequestDeliveriesResponse{}, err
	}
	if current.State != domain.AnnouncementRequestStateApproved {
		return domain.SearchAnnouncementRequestDeliveriesResponse{}, &apierror.ConflictError{Msg: "deliveries can only be recorded while the request is approved, not " + string(current.State)}
	}
	if current.CreatedBy != actorID {
		return domain.SearchAnnouncementRequestDeliveriesResponse{}, &apierror.ForbiddenError{Msg: "only the request's creator can record deliveries"}
	}

	resolved := make(map[string]bool, len(current.ResolvedProjectIDs))
	for _, p := range current.ResolvedProjectIDs {
		resolved[p] = true
	}
	for i, d := range deliveries {
		if strings.TrimSpace(d.ProjectID) == "" {
			return domain.SearchAnnouncementRequestDeliveriesResponse{}, &apierror.ValidationError{Msg: fmt.Sprintf("deliveries[%d].projectId is required", i)}
		}
		if !resolved[d.ProjectID] {
			return domain.SearchAnnouncementRequestDeliveriesResponse{}, &apierror.ValidationError{Msg: fmt.Sprintf("deliveries[%d].projectId %q is not part of this request's resolved audience", i, d.ProjectID)}
		}
		if !validAnnouncementRequestDeliveryStatus[d.Status] {
			return domain.SearchAnnouncementRequestDeliveriesResponse{}, &apierror.ValidationError{Msg: fmt.Sprintf("deliveries[%d].status must be one of: succeeded, tag_failed, failed", i)}
		}
		if (d.Status == domain.AnnouncementRequestDeliveryStatusSucceeded || d.Status == domain.AnnouncementRequestDeliveryStatusTagFailed) &&
			(d.CaseID == nil || strings.TrimSpace(*d.CaseID) == "") {
			return domain.SearchAnnouncementRequestDeliveriesResponse{}, &apierror.ValidationError{Msg: fmt.Sprintf("deliveries[%d].caseId is required for status %q", i, d.Status)}
		}
		if d.Status == domain.AnnouncementRequestDeliveryStatusFailed && d.CaseID != nil {
			return domain.SearchAnnouncementRequestDeliveriesResponse{}, &apierror.ValidationError{Msg: fmt.Sprintf("deliveries[%d].caseId must be omitted for status %q — a failed delivery never has a real case", i, d.Status)}
		}
	}

	saved, err := s.repo.UpsertDeliveries(ctx, id, deliveries)
	if err != nil {
		return domain.SearchAnnouncementRequestDeliveriesResponse{}, err
	}
	return domain.SearchAnnouncementRequestDeliveriesResponse{Deliveries: saved}, nil
}

// ListDeliveries implements AnnouncementRequestService.
func (s *announcementRequestService) ListDeliveries(ctx context.Context, id string) (domain.SearchAnnouncementRequestDeliveriesResponse, error) {
	if strings.TrimSpace(id) == "" {
		return domain.SearchAnnouncementRequestDeliveriesResponse{}, &apierror.ValidationError{Msg: "id is required"}
	}
	if _, err := s.repo.Get(ctx, id); err != nil {
		return domain.SearchAnnouncementRequestDeliveriesResponse{}, err
	}
	deliveries, err := s.repo.ListDeliveries(ctx, id)
	if err != nil {
		return domain.SearchAnnouncementRequestDeliveriesResponse{}, err
	}
	return domain.SearchAnnouncementRequestDeliveriesResponse{Deliveries: deliveries}, nil
}

func isValidAnnouncementRequestState(s domain.AnnouncementRequestState) bool {
	switch s {
	case domain.AnnouncementRequestStateDraft,
		domain.AnnouncementRequestStatePendingApproval,
		domain.AnnouncementRequestStateApproved,
		domain.AnnouncementRequestStatePublished:
		return true
	default:
		return false
	}
}
