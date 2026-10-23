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

package paging

import (
	"context"
	"testing"
	"time"
)

// leads is both SaaS teams' leads, apollo with two.
func leads() []teamMember {
	return []teamMember{
		member("apollo", "a-lead@example.com", roleLead, ""),
		member("apollo", "a-lead2@example.com", roleLead, ""),
		member("artemis", "r-lead@example.com", roleLead, ""),
	}
}

// By day, Apollo's L1 on leave leaves the L1 rung empty: Artemis's L1 is not
// called, and the next rung is Apollo's own L2.
func TestResolveSRE_ByDayATeamNeverBorrowsAnotherTeamsTier(t *testing.T) {
	rota := morningRota()
	rota.onDuty[0].OnLeave = true // a-l1
	r := sreResolver(rota)
	if got := resolveSRE(t, r, Level0, "Apollo"); len(got) != 0 {
		t.Errorf("L1 = %v; want nobody, not Artemis's L1", got)
	}
	if got := resolveSRE(t, r, Level1, "Apollo"); len(got) != 1 || got[0] != "a-l2@example.com" {
		t.Errorf("L2 = %v; want Apollo's own L2", got)
	}
	// Apollo's own L1 on leave is still rung when onLeave is call: nobody
	// else on the team holds the tier.
	got, err := r.Resolve(context.Background(), Level0, RoutingContext{AssignedCRETeam: "Apollo", Ladder: LadderSRE, At: testClock, CallOnLeave: true})
	if err != nil || len(got) != 1 || got[0].Email != "a-l1@example.com" {
		t.Errorf("L1 with onLeave call = %v, %v; want Apollo's own L1", emails(got), err)
	}
}

// A team with somebody on L1 or L2 but nobody on L3 gets an empty L3 rung,
// not another team's L3 and not its lead.
func TestResolveSRE_EmptyL3WithL1HeldIsJustEmpty(t *testing.T) {
	rota := &stubScheduleReader{onDuty: []onDutyAssignment{
		held("a-l1", "apollo", "SRE_TZ1_L1", ""),
		held("r-l3", "artemis", "SRE_TZ1", "L3"),
	}, members: leads()}
	if got := resolveSRE(t, sreResolver(rota), Level2, "Apollo"); len(got) != 0 {
		t.Errorf("L3 = %v; want nobody", got)
	}
}

// Nobody in the team on L1-L3 (Artemis holds them all): the L3 rung calls the
// team lead, one of them, and the opt-in L4 rung then calls nobody.
func TestResolveSRE_UnstaffedTeamCallsItsLeadOnTheL3Rung(t *testing.T) {
	rota := &stubScheduleReader{onDuty: []onDutyAssignment{
		held("r-l1", "artemis", "SRE_TZ1_L1", ""),
		held("r-l2", "artemis", "SRE_TZ1", "L2"),
		held("r-l3", "artemis", "SRE_TZ1", "L3"),
	}, members: leads()}
	r := sreResolver(rota)
	for _, level := range []Level{Level0, Level1} {
		if got := resolveSRE(t, r, level, "Apollo"); len(got) != 0 {
			t.Errorf("%s = %v; want nobody", level, got)
		}
	}
	if got := resolveSRE(t, r, Level2, "Apollo"); len(got) != 1 || got[0] != "a-lead2@example.com" && got[0] != "a-lead@example.com" {
		t.Errorf("L3 = %v; want exactly one Apollo lead", got)
	}
	if got := resolveSRE(t, r, Level3, "Apollo"); len(got) != 0 {
		t.Errorf("L4 = %v; the lead was already called on the L3 rung", got)
	}
}

// At night the TZ3 crew is shared, so an Apollo incident still reaches an
// Artemis holder on it (no lead call).
func TestResolveSRE_SharedNightCrewStillAnswersBothTeams(t *testing.T) {
	r := nightCoverWeek(t)
	if got := ladderAt(t, r, "Apollo", ist(2026, 10, 14, 2, 0)); got != [3]string{"ap-n1", "ar-n2", "ap-n3"} {
		t.Errorf("Apollo at 02:00 = %v; want the shared night crew", got)
	}
}

// An incident with no SRE team of its own (a default group, or a CRE P0 with
// no SRE team) and nobody on L1-L3: the L3 rung calls one lead of each team
// on the rota that pages it -- both SaaS teams.
func TestResolveSRE_TeamlessUnstaffedCallsBothSaaSLeads(t *testing.T) {
	stub := &stubScheduleReader{catalogue: nightCoverCatalogue(), members: leads()}
	teams := sreTeams
	teams.DefaultGroups = []string{"default"}
	r := NewTeamScheduleResolver(stub, teams, nil)
	for _, group := range []string{"Default", "Atlas"} {
		got := resolveSRE(t, r, Level2, group)
		if len(got) != 2 || got[1] != "r-lead@example.com" {
			t.Errorf("%s L3 = %v; want one Apollo lead and the Artemis lead", group, got)
		}
	}
}

// An incident with no SRE team of its own whose L1 nobody holds: the opt-in
// L4 rung still calls the lead of the team that answers, taken from L2, else
// L3 -- not nobody.
func TestResolveSRE_TeamlessL4FollowsTheFirstHeldTier(t *testing.T) {
	cases := map[string][]onDutyAssignment{
		"L2 held": {held("r-l2", "artemis", "SRE_TZ1", "L2")},
		"L3 held": {held("r-l3", "artemis", "SRE_TZ1", "L3")},
		"L2 before L3": {
			held("r-l2", "artemis", "SRE_TZ1", "L2"),
			held("a-l3", "apollo", "SRE_TZ1", "L3"),
		},
	}
	for name, onDuty := range cases {
		t.Run(name, func(t *testing.T) {
			r := sreResolver(&stubScheduleReader{onDuty: onDuty, members: leads()})
			if got := resolveSRE(t, r, Level3, "Atlas"); len(got) != 1 || got[0] != "r-lead@example.com" {
				t.Errorf("L4 = %v; want the Artemis lead", got)
			}
		})
	}
}

// The plan calls the lead at the L3 slot (10 minutes), nobody before.
func TestBuildPlan_UnstaffedTeamLeadRingsAtTheL3Slot(t *testing.T) {
	rota := &stubScheduleReader{members: []teamMember{member("apollo", "a-lead@example.com", roleLead, "")}}
	r := sreResolver(rota).WithPhoneBook(PhoneBook{TestCallTo: "+94770000000"})
	tr := Trigger{
		IncidentID: testIncidentID, Priority: "S0", Kind: TriggerNewIncident, At: testClock,
		Routing: RoutingContext{AssignedCRETeam: "apollo", Ladder: LadderSRE, At: testClock},
	}
	plan, err := BuildPlan(context.Background(), tr, DefaultPolicy, r, ChannelCall)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Calls) != 1 || plan.Calls[0].Level != Level2 || plan.Calls[0].At.Sub(testClock) != 10*time.Minute ||
		plan.Calls[0].Recipient.Email != "a-lead@example.com" {
		t.Fatalf("calls = %+v; want one call to the Apollo lead at +10m on the L3 rung", plan.Calls)
	}
}

// IaaS follows the same rule: nobody in IaaS on L1-L3 (SaaS has people on duty)
// leaves L1 and L2 empty and calls the IaaS lead on the L3 rung, at +10m -- never
// a SaaS engineer and never a SaaS lead.
func TestResolveSRE_UnstaffedIaaSCallsTheIaaSLead(t *testing.T) {
	stub := &stubScheduleReader{
		catalogue: rotaCatalogue(true),
		onDuty: []onDutyAssignment{
			held("a-l1", "apollo", "SRE_TZ1_L1", ""),
			held("a-l2", "apollo", "SRE_TZ1", "L2"),
		},
		members: append(leads(), member("iaas", "i-lead@example.com", roleLead, "")),
	}
	r := rotaResolver(stub, "")
	for _, level := range []Level{Level0, Level1} {
		if got := resolveSRE(t, r, level, "SRE IaaS"); len(got) != 0 {
			t.Errorf("IaaS %s = %v; want nobody", level, got)
		}
	}
	if got := resolveSRE(t, r, Level2, "SRE IaaS"); len(got) != 1 || got[0] != "i-lead@example.com" {
		t.Errorf("IaaS L3 = %v; want the IaaS lead", got)
	}

	tr := Trigger{
		IncidentID: testIncidentID, Priority: "S1", Kind: TriggerNewIncident, At: testClock,
		Routing: RoutingContext{AssignedCRETeam: "SRE IaaS", Ladder: LadderSRE, At: testClock, Rota: RotaSREIaaS},
	}
	plan, err := BuildPlan(context.Background(), tr, DefaultPolicy, r.WithPhoneBook(PhoneBook{TestCallTo: "+94770000000"}), ChannelCall)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Calls) != 1 || plan.Calls[0].Level != Level2 || plan.Calls[0].At.Sub(testClock) != 10*time.Minute ||
		plan.Calls[0].Recipient.Email != "i-lead@example.com" {
		t.Fatalf("calls = %+v; want one call to the IaaS lead at +10m", plan.Calls)
	}
}
