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

// A customer's proposed time and WSO2's answer to it: ServiceNow's own mechanism,
// customer_updated_on (the proposed START) and customer_updated_date_confirmation (WSO2's
// answer). See change_request_customer_proposal.go. The harness (planLength, proposeAs,
// acceptReq, counterReq, snap, rowCounts, migratedInCustomerApproval, ...) is in
// change_request_proposal_harness_integration_test.go. Fixtures use the repository's
// synthetic project A (contacts Alice and Bob) and shapes only.

// ---------------------------------------------------------------------------
// When does a proposal "wait"? The allowlist predicate, on every shape of data
// ---------------------------------------------------------------------------

// The predicate matrix. Each row is a change as the data can leave it -- a native one, a
// migrated one in flight, history, a date WSO2 wrote, an old date, a change waiting on an
// internal approval -- and says what the read model makes of it and what Accept does. A
// proposal waits only in Customer Approval with a finite proposed start that differs from
// the plan, no answer, and no approval but the customer's still asked; anything else is
// history or not ours to answer, and no act of this feature touches it.
func TestChangeRequestProposalIntegration_PredicateMatrix(t *testing.T) {
	future := time.Now().UTC().AddDate(1, 0, 0).Truncate(time.Second).Format(time.RFC3339)
	past := "2020-06-01T09:00:00Z"
	type row struct {
		name  string
		setup func(f *crFlow) string
		// answer is customerProposal.answer ("none": the field is absent).
		answer string
		// acceptRefusal: Accept's 409 (a fragment); "" when the proposal waits and Accept is not tried here.
		acceptRefusal string
		// recorded: ProposerRecorded expected when the proposal waits.
		recorded bool
		// canAccept / blocked: what a staff reader is told when the proposal waits.
		canAccept bool
		blocked   string
	}
	inCA := func(f *crFlow) string {
		id := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
		return id
	}
	// An approval that is NOT the customer's, still being asked, in each of the ways the data shows one.
	blockedBy := func(label *string, group *string, withCustomerGroup bool) func(f *crFlow) string {
		return func(f *crFlow) string {
			f.seedSREGroup()
			id := f.migratedInCustomerApproval()
			if !withCustomerGroup {
				f.execSQL(`UPDATE change_request SET customer_group_id = NULL WHERE id = $1`, id)
			}
			f.syncWritesConversation(id, sp(future), "")
			if label != nil {
				f.seedLooseStageInGroup(id, label, group, 30, map[string]string{crCABMemberUserID1: "REQUESTED"})
			} else {
				f.seedMigratedStage(id, group, 30, map[string]string{crCABMemberUserID1: "REQUESTED"})
			}
			return id
		}
	}
	cab, sre, review := crCABGroupID, crFlowSREGroupID, "Review"
	rows := []row{
		{name: "M1 a migrated change in Customer Approval, the sync wrote the proposal, ServiceNow's own customer stage (unlabeled, the customer group) asks Alice and Bob",
			setup: func(f *crFlow) string {
				id := f.migratedInCustomerApproval()
				f.syncWritesConversation(id, sp(future), "")
				return id
			}, answer: "pending", recorded: false, canAccept: true},
		{name: "M1b the same with no customer_group_id: the stage cannot be proved to be the customer's, so it blocks",
			setup: func(f *crFlow) string {
				id := f.migratedInCustomerApproval()
				f.execSQL(`UPDATE change_request SET customer_group_id = NULL WHERE id = $1`, id)
				f.syncWritesConversation(id, sp(future), "")
				return id
			}, answer: "unanswered", acceptRefusal: msgNothingWaiting},
		{name: "M2 AUTHORIZE with a live unlabeled CAB-group stage and a stale proposed date: never a proposal",
			setup: func(f *crFlow) string {
				id := f.migratedInCustomerApproval()
				f.setState(id, "AUTHORIZE")
				f.execSQL(`UPDATE approval_stage_approver SET state = 'CANCELLED' WHERE work_item_id = $1`, id)
				c := crCABGroupID
				f.seedMigratedStage(id, &c, 20, map[string]string{crCABMemberUserID1: "REQUESTED", crCABMemberUserID2: "REQUESTED"})
				f.syncWritesConversation(id, sp(future), "")
				return id
			}, answer: "unanswered", acceptRefusal: "but it is in Authorize"},
		{name: "M2b a native change waiting on its live CAB stage with a stale date",
			setup: func(f *crFlow) string {
				id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, false)
				f.setPlanned(id, rsStart1, rsEnd1)
				f.requestApproval(id)
				if err := f.decide(id, crFlowPeerAID, "approved"); err != nil {
					t.Fatalf("peer approval: %v", err)
				}
				f.expect(id, "in Authorize", "AUTHORIZE", "canceled")
				f.syncWritesConversation(id, sp(future), "")
				return id
			}, answer: "unanswered", acceptRefusal: "but it is in Authorize"},
		{name: "M3 CLOSED, an AGREE in the history, the proposal applied",
			setup: func(f *crFlow) string {
				id := f.migratedInCustomerApproval()
				f.setState(id, "CLOSED")
				f.syncWritesConversation(id, sp(rsStart1), "AGREE")
				return id
			}, answer: "agreed", acceptRefusal: "but it is in Closed"},
		{name: "M3b SCHEDULED, an AGREE whose date is not the plan (the sync moved them apart)",
			setup: func(f *crFlow) string {
				id := f.migratedInCustomerApproval()
				f.setState(id, "SCHEDULED")
				f.syncWritesConversation(id, sp(future), "AGREE")
				return id
			}, answer: "agreed", acceptRefusal: "but it is in Scheduled"},
		{name: "M4 Customer Approval, AGREE, the proposed date is the planned start",
			setup: func(f *crFlow) string {
				id := f.migratedInCustomerApproval()
				f.syncWritesConversation(id, sp(rsStart1), "AGREE")
				return id
			}, answer: "agreed", acceptRefusal: msgNothingWaiting},
		{name: "M4b Customer Approval, DISAGREE standing: WSO2 asked for another time",
			setup: func(f *crFlow) string {
				id := f.migratedInCustomerApproval()
				f.syncWritesConversation(id, sp(future), "DISAGREE")
				return id
			}, answer: "disagreed", acceptRefusal: msgNothingWaiting},
		{name: "M5a an approval of the CAB's own group (unlabeled) is still asked",
			setup: blockedBy(nil, &cab, true), answer: "unanswered", acceptRefusal: msgNothingWaiting},
		{name: "M5b an approval labelled Review is still asked",
			setup: blockedBy(&review, nil, true), answer: "unanswered", acceptRefusal: msgNothingWaiting},
		{name: "M5c an approval of a group nobody has named is still asked",
			setup: blockedBy(nil, &sre, true), answer: "unanswered", acceptRefusal: msgNothingWaiting},
		{name: "M5d an unlabeled approval with no group is still asked",
			setup: blockedBy(nil, nil, true), answer: "unanswered", acceptRefusal: msgNothingWaiting},
		{name: "M6 a NULL state with a proposed date",
			setup: func(f *crFlow) string {
				id := f.migratedInCustomerApproval()
				f.setState(id, "")
				f.syncWritesConversation(id, sp(future), "")
				return id
			}, answer: "unanswered", acceptRefusal: "but it is in New"},
		{name: "M7 Customer Approval, no answer, the proposed date is the planned start",
			setup: func(f *crFlow) string {
				id := f.migratedInCustomerApproval()
				f.syncWritesConversation(id, sp(rsStart1), "")
				return id
			}, answer: "unanswered", acceptRefusal: msgNothingWaiting},
		{name: "M8 a date a WSO2 user wrote in ServiceNow (the last writer is staff): waits, proposer not recorded",
			setup: func(f *crFlow) string {
				id := inCA(f)
				f.syncWritesConversation(id, sp(future), "")
				return id
			}, answer: "pending", recorded: false, canAccept: true},
		{name: "M9 a stale date from an old cycle, in the past: waits, Accept is blocked",
			setup: func(f *crFlow) string {
				id := inCA(f)
				f.syncWritesConversation(id, sp(past), "")
				return id
			}, answer: "pending", recorded: false, canAccept: false, blocked: "has already passed"},
		{name: "M10 a date the customer proposed through the API: waits, proposer recorded",
			setup: func(f *crFlow) string {
				id := inCA(f)
				f.mustPropose(id, crScopeUserA1, rsStart2)
				return id
			}, answer: "pending", recorded: true, canAccept: true},
		{name: "M10b the same after a staff edit replaced the last writer: waits, proposer no longer recorded",
			setup: func(f *crFlow) string {
				id := inCA(f)
				f.mustPropose(id, crScopeUserA1, rsStart2)
				if _, err := f.patch(id, domain.PatchChangeRequestRequest{WorkNote: sp("looking at it")}); err != nil {
					t.Fatalf("a staff edit: %v", err)
				}
				return id
			}, answer: "pending", recorded: false, canAccept: true},
		{name: "M11 an infinite proposed date reads as none and does not break the read",
			setup: func(f *crFlow) string {
				id := inCA(f)
				f.execSQL(`UPDATE change_request SET customer_updated_on = 'infinity'::timestamptz WHERE id = $1`, id)
				return id
			}, answer: "none"},
		{name: "M12 no proposal at all",
			setup: inCA, answer: "none"},
	}
	for _, r := range rows {
		r := r
		t.Run(r.name, func(t *testing.T) {
			f := newCustomerGroupFlow(t)
			id := r.setup(f)
			cr := f.get(id)
			got := "none"
			if cr.CustomerProposal != nil {
				got = cr.CustomerProposal.Answer
			}
			if got != r.answer {
				t.Fatalf("answer = %q, want %q (%+v)", got, r.answer, cr.CustomerProposal)
			}
			if r.answer == "none" {
				if cr.CustomerProposal != nil {
					t.Fatalf("customerProposal = %+v, want absent", cr.CustomerProposal)
				}
				return
			}
			p := cr.CustomerProposal
			if p.StartOn == "" {
				t.Fatal("the proposal carries no start")
			}
			if r.answer == "pending" {
				// What a staff reader is told about who proposed it and whether Accept would work.
				if p.ProposerRecorded == nil || *p.ProposerRecorded != r.recorded {
					t.Fatalf("proposerRecorded = %v, want %v", p.ProposerRecorded, r.recorded)
				}
				if r.recorded != (p.ProposedByEmail != nil) || (!r.recorded && (p.ProposedByName != nil || p.ProposedOn != nil)) {
					t.Fatalf("proposer fields = %v / %v / %v with recorded=%v", p.ProposedByEmail, p.ProposedByName, p.ProposedOn, r.recorded)
				}
				if p.CanAccept == nil || *p.CanAccept != r.canAccept {
					t.Fatalf("canAccept = %v, want %v", p.CanAccept, r.canAccept)
				}
				if r.blocked != "" && (p.AcceptBlockedReason == nil || !strings.Contains(*p.AcceptBlockedReason, r.blocked)) {
					t.Fatalf("acceptBlockedReason = %v, want it to say %q", p.AcceptBlockedReason, r.blocked)
				}
				if r.canAccept && p.AcceptBlockedReason != nil {
					t.Fatalf("acceptBlockedReason = %q while Accept is possible", *p.AcceptBlockedReason)
				}
				if p.EndOn == nil {
					t.Fatal("a waiting proposal over a planned window carries its end")
				}
				return
			}
			// History or not ours to answer: no waiting-only fact, and Accept is refused with
			// nothing written.
			if p.EndOn != nil || p.ProposerRecorded != nil || p.CanAccept != nil || p.ProposedByEmail != nil {
				t.Fatalf("a proposal that is not waiting carries waiting-only facts: %+v", p)
			}
			before := f.snap(id)
			_, err := f.patch(id, domain.PatchChangeRequestRequest{
				ConfirmCustomerUpdatedDate: sp("agree"), ExpectedCustomerUpdatedOn: sp(p.StartOn),
				ExpectedPlannedStartOn: cr.PlannedStartOn, ExpectedPlannedEndOn: cr.PlannedEndOn})
			f.wantConflictContaining("Accept on a proposal that is not waiting", err, r.acceptRefusal)
			f.wantRefusedSame("Accept on a proposal that is not waiting", id, before, err)
		})
	}
}

const msgNothingWaiting = "no new time proposed by the customer is waiting for a response on this change request"
