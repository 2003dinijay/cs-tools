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
	"errors"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// The customer requirements lock as a truth table: state x field x direction.
//
// Row ids are "<STATE>/<column>" with STATE the upper-case change_request_state_enum
// label ("NULL" for a pre-lifecycle row with no state) and the columns below. The
// CSM webapp's Edit dialog (EditChangeRequestDialog.tsx / utils/changeRequests.ts)
// computes the same rule client-side from the stored (state, flag, hasProject) and
// has a Vitest table with THE SAME ROW IDS and outcome codes: when one changes, so
// does the other (the server stays the authority, and answers 400). There is no
// file shared between the two modules on purpose (the container tests copy only
// entity-service).
//
// Outcome codes:
//
//	ok              accepted
//	frozen          the Customer Project can no longer be changed
//	cannot-turn-off a ticked box can no longer be unticked
//	gate-passed     the gate the box controls has been passed
//	needs-project   the box cannot be ticked: there is no Customer Project to ask
//	return-to-new   the state cannot go back to New
//	rolled-back     the state cannot change out of Rollback at all
//
// Columns:
//
//	project-change     projectId of ANOTHER project than the stored one
//	project-resend     projectId equal to the stored one
//	project-set        projectId on a change that has none stored (NULL -> X)
//	approval-on        customerApprovalRequired false -> true, project stored
//	approval-on-bare   the same with no Customer Project stored
//	approval-off       customerApprovalRequired true -> false
//	review-on          customerReviewRequired false -> true, project stored
//	review-on-bare     the same with no Customer Project stored
//	review-off         customerReviewRequired true -> false
//	to-new             {state: "new"}
type lockTruthRow struct {
	state                                    string
	projectChange, projectResend, projectSet string
	approvalOn, approvalOnBare, approvalOff  string
	reviewOn, reviewOnBare, reviewOff        string
	toNew                                    string
}

// The rows are written out, one per state, and NOT derived from the production
// helpers: they are the statement of the rule the helpers are held to.
var lockTruthTable = []lockTruthRow{
	{"NULL", "ok", "ok", "ok", "ok", "ok", "ok", "ok", "ok", "ok", "ok"},
	{"NEW", "ok", "ok", "ok", "ok", "ok", "ok", "ok", "ok", "ok", "ok"},
	{"ASSESS", "frozen", "ok", "frozen", "ok", "needs-project", "cannot-turn-off", "ok", "needs-project", "cannot-turn-off", "return-to-new"},
	{"AUTHORIZE", "frozen", "ok", "frozen", "ok", "needs-project", "cannot-turn-off", "ok", "needs-project", "cannot-turn-off", "return-to-new"},
	{"CUSTOMER_APPROVAL", "frozen", "ok", "frozen", "gate-passed", "gate-passed", "cannot-turn-off", "ok", "needs-project", "cannot-turn-off", "return-to-new"},
	{"SCHEDULED", "frozen", "ok", "frozen", "gate-passed", "gate-passed", "cannot-turn-off", "ok", "needs-project", "cannot-turn-off", "return-to-new"},
	{"IMPLEMENT", "frozen", "ok", "frozen", "gate-passed", "gate-passed", "cannot-turn-off", "ok", "needs-project", "cannot-turn-off", "return-to-new"},
	{"REVIEW", "frozen", "ok", "frozen", "gate-passed", "gate-passed", "cannot-turn-off", "ok", "needs-project", "cannot-turn-off", "return-to-new"},
	{"CUSTOMER_REVIEW", "frozen", "ok", "frozen", "gate-passed", "gate-passed", "cannot-turn-off", "gate-passed", "gate-passed", "cannot-turn-off", "return-to-new"},
	{"ROLLBACK", "frozen", "ok", "frozen", "gate-passed", "gate-passed", "cannot-turn-off", "gate-passed", "gate-passed", "cannot-turn-off", "rolled-back"},
	{"CLOSED", "frozen", "ok", "frozen", "gate-passed", "gate-passed", "cannot-turn-off", "gate-passed", "gate-passed", "cannot-turn-off", "return-to-new"},
	{"CANCELED", "frozen", "ok", "frozen", "gate-passed", "gate-passed", "cannot-turn-off", "gate-passed", "gate-passed", "cannot-turn-off", "return-to-new"},
}

// lockOutcome maps a refusal to its outcome code by its message.
func lockOutcome(t *testing.T, err error) string {
	t.Helper()
	if err == nil {
		return "ok"
	}
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("refusal is %T (%v), want a *apierror.ValidationError (a readable 400)", err, err)
	}
	switch m := ve.Msg; {
	case strings.Contains(m, "projectId can no longer be changed"):
		return "frozen"
	case strings.Contains(m, "can no longer be turned off"):
		return "cannot-turn-off"
	case strings.Contains(m, "can no longer be changed: the change request has already"):
		return "gate-passed"
	case strings.Contains(m, "cannot be turned on"):
		return "needs-project"
	case strings.Contains(m, "cannot return to it"):
		return "return-to-new"
	case strings.Contains(m, "rollback is final"):
		return "rolled-back"
	default:
		t.Fatalf("unrecognised refusal %q", m)
		return ""
	}
}

func TestCustomerRequirementsLock_TruthTable(t *testing.T) {
	stored, other := "3bbbbbbb-0000-0000-0000-000000000011", "3bbbbbbb-0000-0000-0000-000000000012"
	newState := domain.ChangeRequestStateNew
	for _, row := range lockTruthTable {
		state := row.state
		if state == "NULL" {
			state = ""
		}
		withProject := changeRequestGateSnapshot{state: state, projectID: &stored}
		noProject := changeRequestGateSnapshot{state: state}
		check := func(column, want string, err error) {
			t.Helper()
			if got := lockOutcome(t, err); got != want {
				t.Errorf("%s/%s = %s (%v), want %s", row.state, column, got, err, want)
			}
		}
		check("project-change", row.projectChange, validateCreationPhaseEdits(withProject, domain.PatchChangeRequestRequest{ProjectID: &other}))
		check("project-resend", row.projectResend, validateCreationPhaseEdits(withProject, domain.PatchChangeRequestRequest{ProjectID: &stored}))
		check("project-set", row.projectSet, validateCreationPhaseEdits(noProject, domain.PatchChangeRequestRequest{ProjectID: &stored}))

		on, off := true, false
		withProjectOff, withProjectOn := withProject, withProject
		withProjectOn.approvalRequired, withProjectOn.reviewRequired = true, true
		bareOff := noProject
		check("approval-on", row.approvalOn, validateCreationPhaseEdits(withProjectOff, domain.PatchChangeRequestRequest{CustomerApprovalRequired: &on}))
		check("approval-on-bare", row.approvalOnBare, validateCreationPhaseEdits(bareOff, domain.PatchChangeRequestRequest{CustomerApprovalRequired: &on}))
		check("approval-off", row.approvalOff, validateCreationPhaseEdits(withProjectOn, domain.PatchChangeRequestRequest{CustomerApprovalRequired: &off}))
		check("review-on", row.reviewOn, validateCreationPhaseEdits(withProjectOff, domain.PatchChangeRequestRequest{CustomerReviewRequired: &on}))
		check("review-on-bare", row.reviewOnBare, validateCreationPhaseEdits(bareOff, domain.PatchChangeRequestRequest{CustomerReviewRequired: &on}))
		check("review-off", row.reviewOff, validateCreationPhaseEdits(withProjectOn, domain.PatchChangeRequestRequest{CustomerReviewRequired: &off}))
		check("to-new", row.toNew, validateCreationPhaseEdits(withProject, domain.PatchChangeRequestRequest{State: &newState}))

		// An unchanged value is accepted in every state, whatever else is true.
		check("approval-resend-on", "ok", validateCreationPhaseEdits(withProjectOn, domain.PatchChangeRequestRequest{CustomerApprovalRequired: &on}))
		check("approval-resend-off", "ok", validateCreationPhaseEdits(bareOff, domain.PatchChangeRequestRequest{CustomerApprovalRequired: &off}))
		check("review-resend-on", "ok", validateCreationPhaseEdits(withProjectOn, domain.PatchChangeRequestRequest{CustomerReviewRequired: &on}))
		check("review-resend-off", "ok", validateCreationPhaseEdits(bareOff, domain.PatchChangeRequestRequest{CustomerReviewRequired: &off}))
	}
}

// Every state of the enum has a row, and the rows are the states there are.
func TestCustomerRequirementsLock_TruthTableCoversEveryState(t *testing.T) {
	seen := map[string]bool{}
	for _, row := range lockTruthTable {
		seen[row.state] = true
	}
	for state := range knownChangeRequestStates {
		if !seen[state] {
			t.Errorf("the truth table has no row for state %s", state)
		}
	}
	if !seen["NULL"] {
		t.Error("the truth table has no row for a NULL state")
	}
	if len(seen) != len(knownChangeRequestStates)+1 {
		t.Errorf("the truth table has rows for %d states, want %d (every state plus NULL)", len(seen), len(knownChangeRequestStates)+1)
	}
}

// The first failing rule wins, in the order 1 (state new), 2 (project), then the
// approval box, then the review box.
func TestCustomerRequirementsLock_FirstFailingRuleWins(t *testing.T) {
	stored, other := "3bbbbbbb-0000-0000-0000-000000000011", "3bbbbbbb-0000-0000-0000-000000000012"
	newState := domain.ChangeRequestStateNew
	off := false
	snap := changeRequestGateSnapshot{state: "ASSESS", projectID: &stored, approvalRequired: true, reviewRequired: true}
	got := validateCreationPhaseEdits(snap, domain.PatchChangeRequestRequest{
		State: &newState, ProjectID: &other, CustomerApprovalRequired: &off, CustomerReviewRequired: &off})
	if o := lockOutcome(t, got); o != "return-to-new" {
		t.Fatalf("state + project + both boxes = %s, want return-to-new first", o)
	}
	got = validateCreationPhaseEdits(snap, domain.PatchChangeRequestRequest{
		ProjectID: &other, CustomerApprovalRequired: &off, CustomerReviewRequired: &off})
	if o := lockOutcome(t, got); o != "frozen" {
		t.Fatalf("project + both boxes = %s, want frozen before the boxes", o)
	}
	got = validateCreationPhaseEdits(snap, domain.PatchChangeRequestRequest{CustomerApprovalRequired: &off, CustomerReviewRequired: &off})
	if err := (*apierror.ValidationError)(nil); !errors.As(got, &err) || !strings.HasPrefix(err.Msg, "customerApprovalRequired") {
		t.Fatalf("both boxes = %v, want the approval box named first", got)
	}
}

// The projectId comparison is not fooled by case or padding.
func TestCustomerRequirementsLock_ProjectResendIsCaseInsensitive(t *testing.T) {
	stored := "3bbbbbbb-0000-0000-0000-00000000aaaa"
	for _, resent := range []string{stored, strings.ToUpper(stored), " " + stored + " "} {
		r := resent
		if err := checkCustomerProjectEdit("SCHEDULED", &stored, &r); err != nil {
			t.Errorf("resending %q after New was refused: %v", resent, err)
		}
	}
}

// Request Approval: refused when a box is set and there is no project, only for a
// change that is still in the creation phase.
func TestCheckRequestApprovalHasProject(t *testing.T) {
	for _, tc := range []struct {
		name                         string
		state                        string
		approval, review, hasProject bool
		wantErr                      bool
	}{
		{"New, approval, no project", "NEW", true, false, false, true},
		{"New, review, no project", "NEW", false, true, false, true},
		{"New, both, no project", "NEW", true, true, false, true},
		{"NULL state counts as New", "", true, false, false, true},
		{"New, nothing set, no project", "NEW", false, false, false, false},
		{"New, approval, project", "NEW", true, true, true, false},
		// A resend after Request Approval is not Request Approval.
		{"Assess, approval, no project (a resend)", "ASSESS", true, false, false, false},
		{"Customer Approval, approval, no project (a resend)", "CUSTOMER_APPROVAL", true, true, false, false},
	} {
		err := checkRequestApprovalHasProject(tc.state, tc.approval, tc.review, tc.hasProject)
		if (err != nil) != tc.wantErr {
			t.Errorf("%s: err = %v, want error: %v", tc.name, err, tc.wantErr)
		}
		if err != nil {
			var ve *apierror.ValidationError
			if !errors.As(err, &ve) || ve.Msg != changeRequestApprovalNeedsProject {
				t.Errorf("%s: err = %v, want the 400 %q", tc.name, err, changeRequestApprovalNeedsProject)
			}
		}
	}
}

func TestChangeRequestPatchNeedsGate(t *testing.T) {
	s, b, l := "x", true, []string{}
	state := domain.ChangeRequestStateAssess
	for name, req := range map[string]domain.PatchChangeRequestRequest{
		"state": {State: &state}, "projectId": {ProjectID: &s}, "approval box": {CustomerApprovalRequired: &b}, "review box": {CustomerReviewRequired: &b},
		"deploymentIds": {DeploymentIDs: &l}, "deploymentProductIds": {DeploymentProductIDs: &l}, "deploymentId": {DeploymentID: &s}, "deployedProductId": {DeployedProductID: &s},
	} {
		if !changeRequestPatchNeedsGate(req) {
			t.Errorf("a PATCH carrying %s skips the creation-phase gate", name)
		}
	}
	for name, req := range map[string]domain.PatchChangeRequestRequest{
		"title": {Title: &s}, "comment": {Comment: &s}, "onHold": {OnHold: &b}, "dates": {PlannedStartOn: &s},
	} {
		if changeRequestPatchNeedsGate(req) {
			t.Errorf("a PATCH carrying only %s takes the work_item lock for nothing", name)
		}
	}
}
