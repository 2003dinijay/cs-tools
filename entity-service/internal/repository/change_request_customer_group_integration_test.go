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

package repository_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// The customer group answers Customer Approval / Customer Review through
// approval stages of its own ("Customer Approval" / "Customer Review"). Same
// harness as TestChangeRequestFlowIntegration_* (crFlow, DSN-gated by
// CHANGE_REQUEST_TEST_DSN).

const (
	// crCustGroupID is the change's Customer Group; crCustGroup2ID a second
	// one (group change); crCustEmptyGroupID a group whose only member is the
	// creator (nobody eligible).
	crCustGroupID      = "3aaaaaaa-0000-0000-0000-0000000000b1"
	crCustGroup2ID     = "3aaaaaaa-0000-0000-0000-0000000000b2"
	crCustEmptyGroupID = "3aaaaaaa-0000-0000-0000-0000000000b3"

	crCustMember1ID = "3aaaaaaa-0000-0000-0000-000000000b11"
	crCustMember2ID = "3aaaaaaa-0000-0000-0000-000000000b12"
	crCustMember3ID = "3aaaaaaa-0000-0000-0000-000000000b13"
)

const (
	stageCustApproval = "Customer Approval"
	stageCustReview   = "Customer Review"
)

// seedCustomerGroups creates the customer groups and their members:
//
//	crCustGroupID      members 1, 2
//	crCustGroup2ID     member 3
//	crCustEmptyGroupID the creator only
func (f *crFlow) seedCustomerGroups() {
	f.t.Helper()
	for id, name := range map[string]string{
		crCustGroupID: "CR Flow Customer Group", crCustGroup2ID: "CR Flow Customer Group 2", crCustEmptyGroupID: "CR Flow Customer Group Empty",
	} {
		gid := id
		// Registered before the members are seeded, so it runs after they are
		// removed; the change requests that reference it go first.
		f.t.Cleanup(func() {
			_, _ = f.scoped.Exec(f.sys, `DELETE FROM work_item WHERE subject = $1`, crFlowSubject)
			_, _ = f.scoped.Exec(f.sys, `DELETE FROM "group" WHERE id = $1`, gid)
		})
		if _, err := f.scoped.Exec(f.sys,
			`INSERT INTO "group" (id, created_on, updated_on, created_by, updated_by, name) VALUES ($1, now(), now(), 'cr-flow-test', 'cr-flow-test', $2)
			 ON CONFLICT (id) DO NOTHING`, id, name); err != nil {
			f.t.Fatalf("seed customer group %s: %v", name, err)
		}
	}
	seedApprovalGroupMembers(f.t, f.scoped, crCustGroupID, crCustMember1ID, crCustMember2ID)
	seedApprovalGroupMembers(f.t, f.scoped, crCustGroup2ID, crCustMember3ID)
	// The creator belongs to the (otherwise empty) group; their user row is
	// owned by seedAssignedGroup, which must have run.
	f.addMember(crCustEmptyGroupID, crFlowCreatorID)
}

func (f *crFlow) addMember(groupID, userID string) {
	f.t.Helper()
	if _, err := f.scoped.Exec(f.sys,
		`INSERT INTO team_member (id, created_on, updated_on, created_by, updated_by, team_id, user_id, group_id)
		 VALUES (gen_random_uuid(), now(), now(), 'cr-flow-test', 'cr-flow-test', $1::uuid, $2, $3::uuid)`,
		seededGroupID, userID, groupID); err != nil {
		f.t.Fatalf("add member %s to %s: %v", userID, groupID, err)
	}
}

// createWithCustomerGroup is createGated with a Customer Group (nil = none).
func (f *crFlow) createWithCustomerGroup(typ domain.ChangeRequestType, customerGroupID *string, approval, review bool) string {
	f.t.Helper()
	g := crFlowGroupID
	resp, err := f.repo.CreateChangeRequest(f.sys, domain.CreateChangeRequestRequest{
		Subject: crFlowSubject, Type: &typ, GroupID: &g, CustomerGroupID: customerGroupID,
		CustomerApprovalRequired: boolp(approval), CustomerReviewRequired: boolp(review),
	}, crFlowEmail(crFlowCreatorID))
	if err != nil {
		f.t.Fatalf("CreateChangeRequest(%s): %v", typ, err)
	}
	if _, err := f.scoped.Exec(f.sys, `UPDATE change_request SET requested_by_user_id = $1::uuid WHERE id = $2`, crFlowCreatorID, resp.ChangeRequest.ID); err != nil {
		f.t.Fatalf("set requested_by: %v", err)
	}
	return resp.ChangeRequest.ID
}

func sp(s string) *string { return &s }

func (f *crFlow) setCustomerGroup(id string, groupID *string) {
	f.t.Helper()
	if _, err := f.patch(id, domain.PatchChangeRequestRequest{CustomerGroupID: &groupID}); err != nil {
		f.t.Fatalf("PATCH customerGroupId: %v", err)
	}
}

// customerStages returns the change's Customer Approval / Customer Review
// stages in creation order.
func (f *crFlow) customerStages(id string) []crFlowStage {
	f.t.Helper()
	var out []crFlowStage
	for _, st := range f.stages(id) {
		if st.label == stageCustApproval || st.label == stageCustReview {
			out = append(out, st)
		}
	}
	return out
}

// liveStages counts the stages with a REQUESTED approver.
func liveStages(stages []crFlowStage) int {
	n := 0
	for _, st := range stages {
		for _, status := range st.approvers {
			if status == "requested" {
				n++
				break
			}
		}
	}
	return n
}

func (f *crFlow) labels(id string) []string {
	f.t.Helper()
	var out []string
	for _, st := range f.stages(id) {
		out = append(out, st.label)
	}
	return out
}

func (f *crFlow) approvalsAs(id, viewerID string) domain.ChangeRequestApprovals {
	f.t.Helper()
	ctx := repository.WithCallerIdentity(context.Background(), repository.SearchScope{Unrestricted: true, ViewerEmail: crFlowEmail(viewerID)})
	got, err := f.repo.GetChangeRequestApprovals(ctx, id)
	if err != nil {
		f.t.Fatalf("GetChangeRequestApprovals: %v", err)
	}
	return got
}

func (f *crFlow) canDecideAs(id, viewerID string) map[string]bool {
	f.t.Helper()
	out := map[string]bool{}
	for _, a := range f.approvalsAs(id, viewerID).Approvals {
		for _, ap := range a.Approvers {
			if ap.CanDecide {
				out[a.Stage+"/"+ap.ID] = true
			}
		}
	}
	return out
}

func (f *crFlow) wantForbidden(what string, err error, contains string) {
	f.t.Helper()
	var fe *apierror.ForbiddenError
	if !errors.As(err, &fe) {
		f.t.Fatalf("%s: err = %v (%T), want *apierror.ForbiddenError", what, err, err)
	}
	if !strings.Contains(fe.Msg, contains) {
		f.t.Fatalf("%s: message %q should contain %q", what, fe.Msg, contains)
	}
}

// driveToCustomerApproval takes a Normal change from New through Request
// Approval, peer approval and CAB approval, to Customer Approval.
func (f *crFlow) driveToCustomerApproval(id string) {
	f.t.Helper()
	f.requestApproval(id)
	f.expect(id, "after Request Approval", "ASSESS", "authorize", "canceled")
	f.approvePeerAndCAB(id, "CUSTOMER_APPROVAL", "canceled")
}

// driveToCustomerReview takes a change that is Scheduled through Implement and
// Review to Customer Review.
func (f *crFlow) driveToCustomerReview(id string) {
	f.t.Helper()
	f.step(id, domain.ChangeRequestStateImplement, "IMPLEMENT", "review", "canceled")
	f.step(id, domain.ChangeRequestStateReview, "REVIEW", "customer_review", "canceled")
	f.step(id, domain.ChangeRequestStateCustomerReview, "CUSTOMER_REVIEW", "canceled")
}

func newCustomerGroupFlow(t *testing.T) *crFlow {
	t.Helper()
	f := newCRFlow(t)
	f.seedAssignedGroup()
	seedApprovalGroupMembers(t, f.scoped, crCABGroupID, crCABMemberUserID1, crCABMemberUserID2)
	f.seedCustomerGroups()
	return f
}

// Normal, both boxes ticked, customer group set: state, stages, approvers,
// legalNextStates and the recorded outcome after every step.
func TestChangeRequestFlowIntegration_CustomerGroupNormalLifecycle(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithCustomerGroup(domain.ChangeRequestTypeNormal, sp(crCustGroupID), true, true)

	cr := f.get(id)
	if cr.CustomerGroup == nil || cr.CustomerGroup.ID != crCustGroupID {
		t.Fatalf("customerGroup read back = %+v, want %s", cr.CustomerGroup, crCustGroupID)
	}
	f.driveToCustomerApproval(id)

	// Entering Customer Approval provisioned the stage: one REQUESTED row per
	// group member, assignment group = the customer group, and the manual
	// "record the customer's approval" is not offered (asserted by
	// driveToCustomerApproval: legalNextStates is [canceled]).
	stages := f.stages(id)
	if got := f.labels(id); strings.Join(got, ",") != "Peer Approval,CAB Approval,Customer Approval" {
		t.Fatalf("stages after CAB approval = %v", got)
	}
	ca := stages[2]
	if ca.groupID != crCustGroupID {
		t.Fatalf("Customer Approval stage group = %s, want the customer group %s", ca.groupID, crCustGroupID)
	}
	assertApprovers(t, "Customer Approval", ca.approvers, map[string]string{crCustMember1ID: "requested", crCustMember2ID: "requested"})
	if approved, _ := f.customerOutcome(id); approved {
		t.Fatal("is_customer_approved already true before any member decided")
	}

	// The Approvals read response names the stage and group.
	view := f.approvalsAs(id, crCustMember1ID)
	if len(view.Approvals) != 3 || view.Approvals[2].Stage != stageCustApproval || view.Approvals[2].ApproverName != "CR Flow Customer Group" {
		t.Fatalf("approvals = %+v, want a third stage %q named after the group", view.Approvals, stageCustApproval)
	}

	// A member approves: the change is scheduled, the other member's row is
	// cancelled, and the customer's approval is recorded.
	if err := f.decide(id, crCustMember1ID, "approved"); err != nil {
		t.Fatalf("customer approval: %v", err)
	}
	f.expect(id, "after the customer's approval", "SCHEDULED", "implement", "canceled")
	assertApprovers(t, "Customer Approval after", f.stages(id)[2].approvers, map[string]string{crCustMember1ID: "approved", crCustMember2ID: "cancelled"})
	if approved, reviewed := f.customerOutcome(id); !approved || reviewed {
		t.Fatalf("outcome after customer approval = approved:%v reviewed:%v, want true/false", approved, reviewed)
	}

	f.step(id, domain.ChangeRequestStateImplement, "IMPLEMENT", "review", "canceled")
	f.step(id, domain.ChangeRequestStateReview, "REVIEW", "customer_review", "canceled")
	// The Review stage still gets provisioned (the customer stage must not
	// shift its checkpoint ordinal), and no customer review stage exists yet.
	if got := f.labels(id); strings.Join(got, ",") != "Peer Approval,CAB Approval,Customer Approval,Review" {
		t.Fatalf("stages in Review = %v", got)
	}
	f.step(id, domain.ChangeRequestStateCustomerReview, "CUSTOMER_REVIEW", "canceled")
	if got := f.labels(id); strings.Join(got, ",") != "Peer Approval,CAB Approval,Customer Approval,Review,Customer Review" {
		t.Fatalf("stages in Customer Review = %v", got)
	}
	cr5 := f.stages(id)[4]
	if cr5.groupID != crCustGroupID {
		t.Fatalf("Customer Review stage group = %s, want %s", cr5.groupID, crCustGroupID)
	}
	assertApprovers(t, "Customer Review", cr5.approvers, map[string]string{crCustMember1ID: "requested", crCustMember2ID: "requested"})

	// The other member decides this time.
	if err := f.decide(id, crCustMember2ID, "approved"); err != nil {
		t.Fatalf("customer review: %v", err)
	}
	f.expect(id, "after the customer's review", "CLOSED")
	assertApprovers(t, "Customer Review after", f.stages(id)[4].approvers, map[string]string{crCustMember1ID: "cancelled", crCustMember2ID: "approved"})
	if approved, reviewed := f.customerOutcome(id); !approved || !reviewed {
		t.Fatalf("final outcome = approved:%v reviewed:%v, want true/true", approved, reviewed)
	}
}

// Rejections: Customer Approval rejected cancels the change; Customer Review
// rejected moves it to Rollback. Neither stamps the customer flag.
func TestChangeRequestFlowIntegration_CustomerGroupRejections(t *testing.T) {
	t.Run("customer approval rejected cancels", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.createWithCustomerGroup(domain.ChangeRequestTypeNormal, sp(crCustGroupID), true, false)
		f.driveToCustomerApproval(id)
		if err := f.decide(id, crCustMember2ID, "rejected"); err != nil {
			t.Fatalf("reject: %v", err)
		}
		f.expect(id, "after the customer rejected", "CANCELED")
		assertApprovers(t, "Customer Approval", f.stages(id)[2].approvers, map[string]string{crCustMember1ID: "cancelled", crCustMember2ID: "rejected"})
		if approved, _ := f.customerOutcome(id); approved {
			t.Fatal("is_customer_approved = true on a rejected approval")
		}
	})
	t.Run("customer review rejected rolls back", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.createWithCustomerGroup(domain.ChangeRequestTypeStandard, sp(crCustGroupID), false, true)
		f.requestApproval(id)
		f.expect(id, "after Request Approval (Standard, no customer approval)", "SCHEDULED", "implement", "canceled")
		f.driveToCustomerReview(id)
		if err := f.decide(id, crCustMember1ID, "rejected"); err != nil {
			t.Fatalf("reject: %v", err)
		}
		f.expect(id, "after the customer rejected the review", "ROLLBACK")
		assertApprovers(t, "Customer Review", f.customerStages(id)[0].approvers, map[string]string{crCustMember1ID: "rejected", crCustMember2ID: "cancelled"})
		if _, reviewed := f.customerOutcome(id); reviewed {
			t.Fatal("is_customer_reviewed = true on a rejected review")
		}
	})
}

// Standard + Customer Approval: Request Approval lands in Customer Approval
// with the stage already provisioned.
func TestChangeRequestFlowIntegration_CustomerGroupStandardRequestApproval(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithCustomerGroup(domain.ChangeRequestTypeStandard, sp(crCustGroupID), true, false)
	f.requestApproval(id)
	f.expect(id, "after Request Approval", "CUSTOMER_APPROVAL", "canceled")
	st := f.customerStages(id)
	if len(st) != 1 || st[0].label != stageCustApproval {
		t.Fatalf("customer stages = %+v", st)
	}
	assertApprovers(t, "Customer Approval", st[0].approvers, map[string]string{crCustMember1ID: "requested", crCustMember2ID: "requested"})
	if err := f.decide(id, crCustMember1ID, "approved"); err != nil {
		t.Fatalf("approve: %v", err)
	}
	f.expect(id, "after approval", "SCHEDULED", "implement", "canceled")
}

// Emergency + Customer Approval: ECAB approval enters Customer Approval and
// provisions the customer stage.
func TestChangeRequestFlowIntegration_CustomerGroupEmergencyECABCascade(t *testing.T) {
	f := newCustomerGroupFlow(t)
	seedApprovalGroupMembers(t, f.scoped, crECABGroupID, crECABMemberUserID)
	id := f.createWithCustomerGroup(domain.ChangeRequestTypeEmergency, sp(crCustGroupID), true, false)
	f.requestApproval(id)
	f.expect(id, "after Request Approval", "AUTHORIZE", "canceled")
	if n := len(f.customerStages(id)); n != 0 {
		t.Fatalf("customer stage provisioned before ECAB approved: %d", n)
	}
	if err := f.decide(id, crECABMemberUserID, "approved"); err != nil {
		t.Fatalf("ECAB approval: %v", err)
	}
	f.expect(id, "after ECAB approval", "CUSTOMER_APPROVAL", "canceled")
	if st := f.customerStages(id); len(st) != 1 || st[0].label != stageCustApproval || liveStages(st) != 1 {
		t.Fatalf("customer stages = %+v", st)
	}
}

// The creator is never an approver, even as a member of the customer group;
// a non-member cannot decide and is told why.
func TestChangeRequestFlowIntegration_CustomerGroupCreatorAndNonMember(t *testing.T) {
	f := newCustomerGroupFlow(t)
	f.addMember(crCustGroupID, crFlowCreatorID)
	id := f.createWithCustomerGroup(domain.ChangeRequestTypeNormal, sp(crCustGroupID), true, false)
	f.driveToCustomerApproval(id)

	assertApprovers(t, "Customer Approval", f.stages(id)[2].approvers,
		map[string]string{crCustMember1ID: "requested", crCustMember2ID: "requested", crFlowCreatorID: "cancelled"})

	err := f.decide(id, crFlowCreatorID, "approved")
	f.wantForbidden("creator deciding", err, "creator of a change request cannot approve")
	// A non-member (an outsider, a peer) gets a readable 403, not a bare 404.
	for _, uid := range []string{crFlowOutsiderID, crFlowPeerAID, crCABMemberUserID1, crCustMember3ID} {
		err := f.decide(id, uid, "approved")
		f.wantForbidden("non-member "+uid, err, `only members of the customer group "CR Flow Customer Group"`)
	}
	f.expect(id, "after refused decisions", "CUSTOMER_APPROVAL", "canceled")

	// canDecide: true only for a member, on their own REQUESTED row.
	if got := f.canDecideAs(id, crCustMember1ID); len(got) != 1 || !got[stageCustApproval+"/"+crCustMember1ID] {
		t.Fatalf("canDecide for a member = %v, want only their own Customer Approval row", got)
	}
	for _, uid := range []string{crFlowCreatorID, crFlowOutsiderID, crCustMember3ID, crCABMemberUserID1} {
		if got := f.canDecideAs(id, uid); len(got) != 0 {
			t.Fatalf("canDecide for %s = %v, want none", uid, got)
		}
	}
}

// First responder wins; the loser's later attempt is refused.
func TestChangeRequestFlowIntegration_CustomerGroupFirstResponderWins(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithCustomerGroup(domain.ChangeRequestTypeStandard, sp(crCustGroupID), true, false)
	f.requestApproval(id)
	if err := f.decide(id, crCustMember2ID, "approved"); err != nil {
		t.Fatalf("first responder: %v", err)
	}
	f.expect(id, "after the first responder", "SCHEDULED", "implement", "canceled")
	err := f.decide(id, crCustMember1ID, "rejected")
	var nf *apierror.NotFoundError
	if !errors.As(err, &nf) {
		t.Fatalf("late decision err = %v (%T), want NotFoundError (no pending approval)", err, err)
	}
	f.expect(id, "after the late decision", "SCHEDULED", "implement", "canceled")
	if got := f.canDecideAs(id, crCustMember1ID); len(got) != 0 {
		t.Fatalf("canDecide for the superseded member = %v", got)
	}
}

// Fallback: no customer group, or nobody eligible in it -> no stage, and the
// manual paths work exactly as before.
func TestChangeRequestFlowIntegration_CustomerGroupFallbackToManual(t *testing.T) {
	cases := []struct {
		name  string
		group *string
	}{
		{"no customer group", nil},
		{"group with only the creator", sp(crCustEmptyGroupID)},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			f := newCustomerGroupFlow(t)
			id := f.createWithCustomerGroup(domain.ChangeRequestTypeNormal, tc.group, true, true)
			f.requestApproval(id)
			f.approvePeerAndCAB(id, "CUSTOMER_APPROVAL", "scheduled", "canceled")
			if n := len(f.customerStages(id)); n != 0 {
				t.Fatalf("customer stage provisioned: %d", n)
			}
			f.step(id, domain.ChangeRequestStateScheduled, "SCHEDULED", "implement", "canceled")
			if approved, _ := f.customerOutcome(id); !approved {
				t.Fatal("manual record did not stamp is_customer_approved")
			}
			f.step(id, domain.ChangeRequestStateImplement, "IMPLEMENT", "review", "canceled")
			f.step(id, domain.ChangeRequestStateReview, "REVIEW", "customer_review", "canceled")
			f.step(id, domain.ChangeRequestStateCustomerReview, "CUSTOMER_REVIEW", "closed", "canceled")
			if n := len(f.customerStages(id)); n != 0 {
				t.Fatalf("customer review stage provisioned: %d", n)
			}
			f.step(id, domain.ChangeRequestStateClosed, "CLOSED")
			if _, reviewed := f.customerOutcome(id); !reviewed {
				t.Fatal("manual close did not stamp is_customer_reviewed")
			}
		})
	}
}

// A deactivated member is not an eligible approver.
func TestChangeRequestFlowIntegration_CustomerGroupInactiveMemberSkipped(t *testing.T) {
	f := newCustomerGroupFlow(t)
	if _, err := f.scoped.Exec(f.sys, `UPDATE "user" SET is_active = false WHERE id = $1`, crCustMember2ID); err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	id := f.createWithCustomerGroup(domain.ChangeRequestTypeStandard, sp(crCustGroupID), true, false)
	f.requestApproval(id)
	assertApprovers(t, "Customer Approval", f.customerStages(id)[0].approvers, map[string]string{crCustMember1ID: "requested"})
}

// Group set later (change already in the state), idempotent; changed while
// live (old cancelled, new provisioned, never two live); cleared (cancelled,
// manual path back); and cancelling the change cancels the pending rows.
func TestChangeRequestFlowIntegration_CustomerGroupSetChangedAndCleared(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithCustomerGroup(domain.ChangeRequestTypeStandard, nil, true, true)
	f.requestApproval(id)
	f.expect(id, "in Customer Approval, no group", "CUSTOMER_APPROVAL", "scheduled", "canceled")

	// Set later: the stage appears; the manual path closes.
	f.setCustomerGroup(id, sp(crCustGroupID))
	f.expect(id, "after the group was set", "CUSTOMER_APPROVAL", "canceled")
	st := f.customerStages(id)
	if len(st) != 1 || st[0].groupID != crCustGroupID {
		t.Fatalf("stages after set = %+v", st)
	}
	assertApprovers(t, "after set", st[0].approvers, map[string]string{crCustMember1ID: "requested", crCustMember2ID: "requested"})
	// Resending the same group, or any unrelated PATCH, changes nothing.
	f.setCustomerGroup(id, sp(crCustGroupID))
	if _, err := f.patch(id, domain.PatchChangeRequestRequest{Title: sp("renamed")}); err != nil {
		t.Fatalf("unrelated PATCH: %v", err)
	}
	if st := f.customerStages(id); len(st) != 1 || liveStages(st) != 1 {
		t.Fatalf("stages after idempotent resend = %+v", st)
	}

	// Changed while live: the old stage is cancelled, a new one is provisioned.
	f.setCustomerGroup(id, sp(crCustGroup2ID))
	f.expect(id, "after the group changed", "CUSTOMER_APPROVAL", "canceled")
	st = f.customerStages(id)
	if len(st) != 2 || liveStages(st) != 1 {
		t.Fatalf("stages after change = %+v, want 2 (1 live)", st)
	}
	assertApprovers(t, "old stage", st[0].approvers, map[string]string{crCustMember1ID: "cancelled", crCustMember2ID: "cancelled"})
	if st[1].groupID != crCustGroup2ID {
		t.Fatalf("new stage group = %s, want %s", st[1].groupID, crCustGroup2ID)
	}
	assertApprovers(t, "new stage", st[1].approvers, map[string]string{crCustMember3ID: "requested"})
	// The former group's members can no longer decide; the new one's can.
	f.wantForbidden("old group member", f.decide(id, crCustMember1ID, "approved"), "only members of the customer group")

	// Cleared: nothing live, the manual path is back.
	f.setCustomerGroup(id, nil)
	f.expect(id, "after the group was cleared", "CUSTOMER_APPROVAL", "scheduled", "canceled")
	if liveStages(f.customerStages(id)) != 0 {
		t.Fatalf("a live customer stage remains after clearing the group")
	}
	// Set to the first group again: a fresh stage (the earlier one was only cancelled).
	f.setCustomerGroup(id, sp(crCustGroupID))
	if st := f.customerStages(id); len(st) != 3 || liveStages(st) != 1 {
		t.Fatalf("stages after re-setting the group = %+v, want 3 (1 live)", st)
	}

	// Cancelling the change cancels what is pending.
	f.step(id, domain.ChangeRequestStateCanceled, "CANCELED")
	if liveStages(f.customerStages(id)) != 0 {
		t.Fatalf("a live customer stage remains on a cancelled change")
	}
}

// The group set later also works for Customer Review.
func TestChangeRequestFlowIntegration_CustomerGroupSetLaterInCustomerReview(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithCustomerGroup(domain.ChangeRequestTypeStandard, nil, false, true)
	f.requestApproval(id)
	f.expect(id, "after Request Approval", "SCHEDULED", "implement", "canceled")
	f.step(id, domain.ChangeRequestStateImplement, "IMPLEMENT", "review", "canceled")
	f.step(id, domain.ChangeRequestStateReview, "REVIEW", "customer_review", "canceled")
	f.step(id, domain.ChangeRequestStateCustomerReview, "CUSTOMER_REVIEW", "closed", "canceled")

	f.setCustomerGroup(id, sp(crCustGroupID))
	f.expect(id, "after the group was set", "CUSTOMER_REVIEW", "canceled")
	st := f.customerStages(id)
	if len(st) != 1 || st[0].label != stageCustReview || st[0].groupID != crCustGroupID {
		t.Fatalf("customer stages = %+v", st)
	}
	if err := f.decide(id, crCustMember2ID, "approved"); err != nil {
		t.Fatalf("approve review: %v", err)
	}
	f.expect(id, "after the review", "CLOSED")
	if _, reviewed := f.customerOutcome(id); !reviewed {
		t.Fatal("is_customer_reviewed not stamped")
	}
}

// While a customer stage is live the manual transitions are refused with a
// readable 400, and nothing is stamped.
func TestChangeRequestFlowIntegration_CustomerGroupRefusesManualTransition(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithCustomerGroup(domain.ChangeRequestTypeStandard, sp(crCustGroupID), true, true)
	f.requestApproval(id)
	_, err := f.patchState(id, domain.ChangeRequestStateScheduled)
	f.wantValidationError("manual scheduled", err, "approving or rejecting it in the change request's approvals")
	f.wantValidationError("manual scheduled", err, `"CR Flow Customer Group"`)
	f.expect(id, "after the refused scheduled", "CUSTOMER_APPROVAL", "canceled")
	if approved, _ := f.customerOutcome(id); approved {
		t.Fatal("refused PATCH stamped is_customer_approved")
	}
	if err := f.decide(id, crCustMember1ID, "approved"); err != nil {
		t.Fatalf("approve: %v", err)
	}
	f.step(id, domain.ChangeRequestStateImplement, "IMPLEMENT", "review", "canceled")
	f.step(id, domain.ChangeRequestStateReview, "REVIEW", "customer_review", "canceled")
	f.step(id, domain.ChangeRequestStateCustomerReview, "CUSTOMER_REVIEW", "canceled")
	_, err = f.patchState(id, domain.ChangeRequestStateClosed)
	f.wantValidationError("manual closed", err, "approving or rejecting it in the change request's approvals")
	f.expect(id, "after the refused close", "CUSTOMER_REVIEW", "canceled")
	if _, reviewed := f.customerOutcome(id); reviewed {
		t.Fatal("refused PATCH stamped is_customer_reviewed")
	}
	// Cancel is still allowed.
	f.step(id, domain.ChangeRequestStateCanceled, "CANCELED")
}

// scripts/csm-compose/seed-entity-service.sql ships two fixtures for the local
// stack: CHG-FIXED-007 (Customer Approval) and CHG-FIXED-008 (Customer Review),
// each with the customer group set and its stage provisioned for jane.doe and
// john.smith. Read-only here (deciding would consume them); skipped when the
// database was not built from the seed.
func TestChangeRequestFlowIntegration_SeedCustomerGroupFixtures(t *testing.T) {
	f := newCRFlow(t)
	for _, tc := range []struct {
		id, number, state, stage string
	}{
		{"00000000-0000-0000-0000-000000001303", "CHG-FIXED-007", "customer_approval", stageCustApproval},
		{"00000000-0000-0000-0000-000000001304", "CHG-FIXED-008", "customer_review", stageCustReview},
	} {
		var n string
		if err := f.scoped.QueryRow(f.sys, `SELECT number FROM work_item WHERE id = $1`, tc.id).Scan(&n); err != nil {
			t.Skipf("seed fixture %s not loaded: %v", tc.number, err)
		}
		cr := f.get(tc.id)
		if cr.State == nil || *cr.State != tc.state {
			t.Fatalf("%s state = %v, want %s", tc.number, cr.State, tc.state)
		}
		if cr.CustomerGroup == nil || cr.CustomerGroup.Name != "Example Corp Customer Approvers" {
			t.Fatalf("%s customerGroup = %+v", tc.number, cr.CustomerGroup)
		}
		assertStates(t, tc.number+" legalNextStates", cr.LegalNextStates, "canceled")
		ctx := repository.WithCallerIdentity(context.Background(), repository.SearchScope{Unrestricted: true, ViewerEmail: "jane.doe@example.com"})
		view, err := f.repo.GetChangeRequestApprovals(ctx, tc.id)
		if err != nil {
			t.Fatalf("%s approvals: %v", tc.number, err)
		}
		if len(view.Approvals) != 1 || view.Approvals[0].Stage != tc.stage || view.Approvals[0].ApproverName != "Example Corp Customer Approvers" {
			t.Fatalf("%s approvals = %+v", tc.number, view.Approvals)
		}
		if len(view.Approvals[0].Approvers) != 2 {
			t.Fatalf("%s approvers = %+v, want jane.doe and john.smith", tc.number, view.Approvals[0].Approvers)
		}
		can := 0
		for _, ap := range view.Approvals[0].Approvers {
			if ap.CanDecide {
				can++
				if ap.Name != "Jane Doe" {
					t.Fatalf("%s canDecide on %q, want only the viewer (Jane Doe)", tc.number, ap.Name)
				}
			}
		}
		if can != 1 {
			t.Fatalf("%s canDecide rows for jane.doe = %d, want 1", tc.number, can)
		}
	}
}
