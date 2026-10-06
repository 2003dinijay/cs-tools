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
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// A customer proposes a new implementation time with PATCH {plannedStartOn,
// plannedEndOn}: the whole window, moved with its length kept, which is how a
// customer re-plans a change (a start alone is refused as soon as it passes the
// stored end, see the bad-window test below). Same harness as the other
// customer-outcome tests; the proposal is the Re-schedule of every change type
// started by a contact.

// The proposal of a whole window, in each of the date formats the portal can
// send, for each change type: the window is applied as given and its length is
// unchanged, the customer's pending request is superseded, and -- per type --
// the change goes back through CAB / ECAB (Normal / Emergency) or stays in
// Customer Approval with the customer asked again (Standard).
func TestChangeRequestCustomerProposalIntegration_WholeWindowKeepsTheDuration(t *testing.T) {
	// What the stored window reads back as after a proposal: RFC 3339, UTC.
	const wantStart, wantEnd = "2030-03-08T09:00:00Z", "2030-03-08T11:00:00Z"
	for _, tc := range []struct {
		name       string
		typ        domain.ChangeRequestType
		start, end string // as the customer sends them
	}{
		{"Normal, RFC 3339 in UTC", domain.ChangeRequestTypeNormal, "2030-03-08T09:00:00Z", "2030-03-08T11:00:00Z"},
		{"Emergency, the webapp's 'YYYY-MM-DD HH:MM:SS' (UTC)", domain.ChangeRequestTypeEmergency, "2030-03-08 09:00:00", "2030-03-08 11:00:00"},
		{"Standard, RFC 3339 with an offset", domain.ChangeRequestTypeStandard, "2030-03-08T14:30:00+05:30", "2030-03-08T16:30:00+05:30"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			f := newCustomerGroupFlow(t)
			seedApprovalGroupMembers(t, f.scoped, crECABGroupID, crECABMemberUserID)
			id := f.createWithProject(tc.typ, sp(crScopeProjectA), true, false)
			f.setPlanned(id, rsStart1, rsEnd1)
			before := windowOf(t, f.get(id))

			var cabApprover, wantStages, wantStagesAfterApproval string
			switch tc.typ {
			case domain.ChangeRequestTypeNormal:
				f.driveToCustomerApproval(id)
				cabApprover = crCABMemberUserID1
				wantStages = "Peer Approval,CAB Approval,Customer Approval,CAB Approval"
				wantStagesAfterApproval = "Peer Approval,CAB Approval,Customer Approval,CAB Approval,Customer Approval"
			case domain.ChangeRequestTypeEmergency:
				f.requestApproval(id)
				if err := f.decide(id, crECABMemberUserID, "approved"); err != nil {
					t.Fatalf("ECAB approval: %v", err)
				}
				f.expect(id, "in Customer Approval", "CUSTOMER_APPROVAL", "authorize", "canceled")
				cabApprover = crECABMemberUserID
				wantStages = "ECAB Approval,Customer Approval,ECAB Approval"
				wantStagesAfterApproval = "ECAB Approval,Customer Approval,ECAB Approval,Customer Approval"
			default:
				f.requestApproval(id)
				f.expect(id, "in Customer Approval", "CUSTOMER_APPROVAL", "authorize", "canceled")
				wantStages = "Customer Approval,Customer Approval"
			}
			f.wantCanAnswer(id, "before the proposal", true, crScopeUserA1, crScopeUserA2)

			if _, err := f.patchAsContact(id, crScopeUserA1, domain.PatchChangeRequestRequest{PlannedStartOn: sp(tc.start), PlannedEndOn: sp(tc.end)}); err != nil {
				t.Fatalf("proposal: %v", err)
			}
			f.wantPlanned(id, "after the proposal", wantStart, wantEnd)
			if after := windowOf(t, f.get(id)); after != before {
				t.Fatalf("the window's length changed from %v to %v", before, after)
			}
			if got := f.stageLabels(id); got != wantStages {
				t.Fatalf("stages after the proposal = %s, want %s", got, wantStages)
			}
			// The customer's request they were just asked is superseded: its rows are
			// cancelled, whatever the type.
			custom := f.customerStages(id)
			assertApprovers(t, "the superseded customer request", custom[0].approvers, map[string]string{crScopeUserA1: "cancelled", crScopeUserA2: "cancelled"})
			if a, _ := f.customerOutcome(id); a {
				t.Fatal("a proposal stamped the customer's approval")
			}

			if tc.typ == domain.ChangeRequestTypeStandard {
				// Nothing internal to repeat: still in Customer Approval, asked again.
				f.expect(id, "after the proposal", "CUSTOMER_APPROVAL", "authorize", "canceled")
				assertApprovers(t, "the fresh customer request", custom[1].approvers, map[string]string{crScopeUserA1: "requested", crScopeUserA2: "requested"})
				f.wantCanAnswer(id, "after the proposal", true, crScopeUserA1, crScopeUserA2)
			} else {
				f.expect(id, "after the proposal", "AUTHORIZE", "canceled")
				f.wantCanAnswer(id, "while the new plan awaits internal approval", false, crScopeUserA1, crScopeUserA2)
				if err := f.decide(id, cabApprover, "approved"); err != nil {
					t.Fatalf("approval of the new plan: %v", err)
				}
				f.expect(id, "after the new plan is approved", "CUSTOMER_APPROVAL", "authorize", "canceled")
				if got := f.stageLabels(id); got != wantStagesAfterApproval {
					t.Fatalf("stages after the new plan is approved = %s, want %s", got, wantStagesAfterApproval)
				}
				f.wantCanAnswer(id, "once the customer is asked again", true, crScopeUserA1, crScopeUserA2)
			}

			// The customer answers the new plan, and the window is the proposed one.
			if _, err := f.approveAs(id, crScopeUserA2, true); err != nil {
				t.Fatalf("approval of the new plan: %v", err)
			}
			f.expect(id, "after the customer approved the new plan", "SCHEDULED", "implement", "canceled")
			f.wantPlanned(id, "after the customer approved the new plan", wantStart, wantEnd)
		})
	}
}

// What a customer is told when the window they propose cannot be applied: a
// readable validation message, nothing about the database, and nothing changed.
func TestChangeRequestCustomerProposalIntegration_BadWindowMessages(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, false)
	f.setPlanned(id, rsStart1, rsEnd1)
	f.driveToCustomerApproval(id)
	stagesBefore := f.stageLabels(id)

	const (
		notChanged  = "re-scheduling requires a changed planned start or end: send plannedStartOn and/or plannedEndOn with a value different from the stored one"
		endsBefore  = "the planned start must not be after the planned end"
		notADate    = "plannedStartOn and plannedEndOn must be valid date-times (RFC 3339)"
		nothingSent = "at least one field must be provided"
	)
	for _, tc := range []struct {
		name string
		req  domain.PatchChangeRequestRequest
		want string
	}{
		{"the stored window again", domain.PatchChangeRequestRequest{PlannedStartOn: sp(rsStart1), PlannedEndOn: sp(rsEnd1)}, notChanged},
		{"the stored start alone", domain.PatchChangeRequestRequest{PlannedStartOn: sp(rsStart1)}, notChanged},
		{"the stored end alone", domain.PatchChangeRequestRequest{PlannedEndOn: sp(rsEnd1)}, notChanged},
		{"the same instant in another offset", domain.PatchChangeRequestRequest{PlannedStartOn: sp("2030-03-01T14:30:00+05:30")}, notChanged},
		{"a window that ends before it starts", domain.PatchChangeRequestRequest{PlannedStartOn: sp(rsStart3), PlannedEndOn: sp(rsEnd1)}, endsBefore},
		{"a start after the stored end (the end is not moved for the customer)", domain.PatchChangeRequestRequest{PlannedStartOn: sp(rsStart3)}, endsBefore},
		{"an end before the stored start", domain.PatchChangeRequestRequest{PlannedEndOn: sp("2030-02-28T11:00:00Z")}, endsBefore},
		{"text that is not a date", domain.PatchChangeRequestRequest{PlannedStartOn: sp("next tuesday")}, notADate},
		{"an end that is not a date", domain.PatchChangeRequestRequest{PlannedStartOn: sp(rsStart2), PlannedEndOn: sp("later")}, notADate},
		{"an empty start", domain.PatchChangeRequestRequest{PlannedStartOn: sp("")}, notADate},
		{"nothing at all", domain.PatchChangeRequestRequest{}, nothingSent},
	} {
		_, err := f.patchAsContact(id, crScopeUserA1, tc.req)
		f.wantValidationError(tc.name, err, tc.want)
		if msg := err.Error(); strings.Contains(strings.ToLower(msg), "timestamptz") || strings.Contains(strings.ToLower(msg), "sqlstate") || strings.Contains(msg, "pq:") || strings.Contains(msg, "ERROR:") {
			t.Fatalf("%s: the message leaks the database: %q", tc.name, msg)
		}
	}

	// A zero-length window (start == end) is not "ending before it starts".
	if _, err := f.patchAsContact(id, crScopeUserA2, domain.PatchChangeRequestRequest{PlannedStartOn: sp(rsStart2), PlannedEndOn: sp(rsStart2)}); err != nil {
		t.Fatalf("a zero-length window: %v", err)
	}
	if got := windowOf(t, f.get(id)); got != 0*time.Second {
		t.Fatalf("zero-length window stored as %v", got)
	}

	// Everything refused above left the change as it was (the one proposal that
	// was accepted is the last step; the refusals ran against the stored window).
	if stagesBefore == f.stageLabels(id) {
		t.Fatal("the accepted proposal did not re-schedule the change")
	}
}

// Refused proposals change nothing: asserted separately from the message table
// so the check runs with no accepted proposal after it.
func TestChangeRequestCustomerProposalIntegration_RefusedWindowChangesNothing(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, false)
	f.setPlanned(id, rsStart1, rsEnd1)
	f.driveToCustomerApproval(id)
	stages := f.stageLabels(id)

	for _, req := range []domain.PatchChangeRequestRequest{
		{PlannedStartOn: sp(rsStart1), PlannedEndOn: sp(rsEnd1)},
		{PlannedStartOn: sp(rsStart3), PlannedEndOn: sp(rsEnd1)},
		{PlannedStartOn: sp(rsStart3)},
		{PlannedStartOn: sp("next tuesday")},
	} {
		if _, err := f.patchAsContact(id, crScopeUserA1, req); err == nil {
			t.Fatalf("%+v was accepted", req)
		}
	}
	f.expect(id, "after the refused proposals", "CUSTOMER_APPROVAL", "authorize", "canceled")
	f.wantPlanned(id, "after the refused proposals", rsStart1, rsEnd1)
	if got := f.stageLabels(id); got != stages {
		t.Fatalf("stages = %s, want them untouched (%s)", got, stages)
	}
	assertApprovers(t, "customer request untouched", f.customerStages(id)[0].approvers, map[string]string{crScopeUserA1: "requested", crScopeUserA2: "requested"})
	f.wantCanAnswer(id, "after the refused proposals", true, crScopeUserA1, crScopeUserA2)
}
