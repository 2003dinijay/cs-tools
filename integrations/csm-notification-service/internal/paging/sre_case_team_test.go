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
	"slices"
	"testing"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/events"
)

// resolveCaseSRE resolves one SRE rung for an S0 case on CRE team Atlas whose account SRE team is sreTeam.
func resolveCaseSRE(t *testing.T, r TeamScheduleResolver, level Level, sreTeam string) []string {
	t.Helper()
	got, err := r.Resolve(context.Background(), level, RoutingContext{AssignedCRETeam: "Atlas", SRETeam: sreTeam, Ladder: LadderSRE, At: testClock})
	if err != nil {
		t.Fatalf("resolve %s: %v", level, err)
	}
	return emails(got)
}

// An S0 case whose account has an SRE team climbs that team's tiers only, not every SaaS team's.
func TestResolveSRE_CaseUsesTheAccountsSRETeam(t *testing.T) {
	r := sreResolver(morningRota())
	if got := resolveCaseSRE(t, r, Level0, "Artemis"); !slices.Equal(got, []string{"r-l1@example.com"}) {
		t.Errorf("L1 = %v; want Artemis's L1 only", got)
	}
	if got := resolveCaseSRE(t, r, Level0, "SRE - Apollo"); !slices.Equal(got, []string{"a-l1@example.com"}) {
		t.Errorf("L1 through an alias = %v; want Apollo's L1 only", got)
	}
}

// Unstaffed, the L3 rung calls the account SRE team's lead, not both SaaS leads.
func TestResolveSRE_CaseUnstaffedCallsOnlyItsSRETeamsLead(t *testing.T) {
	stub := &stubScheduleReader{catalogue: nightCoverCatalogue(), members: leads()}
	r := NewTeamScheduleResolver(stub, sreTeams, nil)
	if got := resolveCaseSRE(t, r, Level2, "Artemis"); !slices.Equal(got, []string{"r-lead@example.com"}) {
		t.Errorf("L3 = %v; want the Artemis lead only", got)
	}
	if got := resolveCaseSRE(t, r, Level2, ""); len(got) != 2 {
		t.Errorf("no account SRE team: L3 = %v; want one lead of each SaaS team", got)
	}
	if got := resolveCaseSRE(t, r, Level2, "MS/PC SRE Group"); len(got) != 2 {
		t.Errorf("an SRE team the rota does not know: L3 = %v; want one lead of each SaaS team", got)
	}
}

// The account SRE team picks the case's card space; without one, or with one that has no space, it is every teamlessChats team.
func TestTeamChatSpaces_CaseChatTeamsFollowTheAccountsSRETeam(t *testing.T) {
	e := splitSREEngine(splitReader(), bothSaaS)
	plan := func(sreTeam string) Plan {
		return Plan{Trigger: Trigger{Team: "Atlas", Record: RecordCase, Routing: RoutingContext{AssignedCRETeam: "Atlas", SRETeam: sreTeam}}}
	}
	for sreTeam, want := range map[string][]string{
		"Artemis SRE Group": {"artemis"},
		"":                  {"apollo", "artemis"},
		"MS/PC SRE Group":   {"apollo", "artemis"},
	} {
		if got := e.chatTeams(plan(sreTeam)); !slices.Equal(got, want) {
			t.Errorf("chatTeams(SRE team %q) = %v; want %v", sreTeam, got, want)
		}
	}

	// A teamChats key that is not one of sre.teams.abts pages every team, so its cards go to every teamless space too.
	stray := splitSREEngine(splitReader(), func(l *LadderConfig) {
		bothSaaS(l)
		l.TeamChats["ms/pc sre group"] = Chat{WebhookURLEnv: "SRE_CHAT_WEBHOOK_URL_MSPC", Audience: "MS/PC SRE"}
	})
	if got := stray.chatTeams(plan("MS/PC SRE Group")); !slices.Equal(got, []string{"apollo", "artemis"}) {
		t.Errorf("an account SRE team with a space but not in abts: chatTeams = %v; want apollo, artemis", got)
	}
}

// The account SRE team's own channel narrows the case's SRE ladder.
func TestTeamChannel_CaseFollowsTheAccountsSRETeam(t *testing.T) {
	calls, card := 0, &fakeChat{}
	e := teamChannelEngine(&calls, card)
	tr := Trigger{Team: "Atlas", Record: RecordCase, Routing: RoutingContext{AssignedCRETeam: "Atlas", SRETeam: "Artemis SRE Group"}}
	if got := e.channelFor(tr); got != ChannelCall {
		t.Errorf("channelFor = %s; want Artemis's call", got)
	}
	tr.Routing.SRETeam = ""
	if got := e.channelFor(tr); got != ChannelBoth {
		t.Errorf("no account SRE team: channelFor = %s; want the ladder's both", got)
	}
}

// Both starting events carry the account SRE team into routing, leaving Team as the CRE team.
func TestCaseTriggers_CarryTheAccountsSRETeam(t *testing.T) {
	created := triggerFromCaseCreated("c1", events.CaseCreatedPayload{Priority: "S0", Team: "Atlas", SRETeamName: "Apollo SRE Group"})
	changed := triggerFromSeverityChanged("c1", events.SeverityChangedPayload{NewSeverity: "S0", Team: "Atlas", SRETeamName: "Apollo SRE Group"}, testClock)
	for name, tr := range map[string]Trigger{"case.created": created, "case.severity_changed": changed} {
		if tr.Team != "Atlas" || tr.Routing.AssignedCRETeam != "Atlas" || tr.Routing.SRETeam != "Apollo SRE Group" {
			t.Errorf("%s: Team %q, AssignedCRETeam %q, SRETeam %q; want Atlas, Atlas, Apollo SRE Group",
				name, tr.Team, tr.Routing.AssignedCRETeam, tr.Routing.SRETeam)
		}
		if tr.sreTeam() != "Apollo SRE Group" {
			t.Errorf("%s: sreTeam() = %q; want Apollo SRE Group", name, tr.sreTeam())
		}
	}
}
