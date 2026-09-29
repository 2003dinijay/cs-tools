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

package escalation

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// stubScheduleReader answers like the real endpoint: it filters the members it
// holds by the same three criteria the query does, so a test that asks for the
// wrong thing gets the wrong answer rather than everything.
type stubScheduleReader struct {
	members    []teamMember
	onDuty     []onDutyAssignment
	membersErr error
	onDutyErr  error

	gotTeamKeys []string
	gotRoles    []string
	gotTiers    []string
	gotAt       time.Time
	memberCalls int
	onDutyCalls int
}

func (s *stubScheduleReader) TeamMembers(_ context.Context, teamKeys, roles, tiers []string) ([]teamMember, error) {
	s.gotTeamKeys, s.gotRoles, s.gotTiers = teamKeys, roles, tiers
	s.memberCalls++
	if s.membersErr != nil {
		return nil, s.membersErr
	}
	var out []teamMember
	for _, m := range s.members {
		if !inList(teamKeys, m.TeamKey) {
			continue
		}
		if len(roles) > 0 && !inList(roles, m.Role) {
			continue
		}
		if len(tiers) > 0 && !inList(tiers, m.AlertTier) {
			continue
		}
		out = append(out, m)
	}
	return out, nil
}

func (s *stubScheduleReader) OnDutyAt(_ context.Context, at time.Time) ([]onDutyAssignment, error) {
	s.gotAt = at
	s.onDutyCalls++
	return s.onDuty, s.onDutyErr
}

func inList(list []string, v string) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}

func onDutyFor(userID, email, teamKey string) onDutyAssignment {
	var a onDutyAssignment
	a.Engineer.UserID, a.Engineer.Email, a.Engineer.Name = userID, email, email
	a.TeamKey = teamKey
	return a
}

func member(team, email, role, tier string) teamMember {
	return teamMember{TeamKey: team, Email: email, Name: email, UserID: email, Role: role, AlertTier: tier}
}

// The seven ABTs plus Americas, as deployed.
var testTeams = TeamKeys{
	ABTs:       []string{"apollo", "artemis", "atlas", "castor", "draco", "phoenix", "vega"},
	Americas:   "americas",
	Leadership: "cre-leadership",
}

func testResolver(stub *stubScheduleReader) TeamScheduleResolver {
	return NewTeamScheduleResolver(stub, testTeams, nil)
}

func emails(rs []Recipient) []string {
	out := make([]string, 0, len(rs))
	for _, r := range rs {
		out = append(out, r.Email)
	}
	return out
}

// Every row of the sheet must be reachable, and reachable by exactly the
// (shift, is-assigned-to-an-ABT) pair it names. A row nothing routes to is a
// rule that silently does not exist.
func TestRuleFor_EveryRowIsReachable(t *testing.T) {
	cases := []struct {
		shift  Shift
		team   string
		wantID string
	}{
		{ShiftLKMorning, "vega", "R1a"},
		{ShiftLKMorning, "", "R1a"},
		{ShiftLKWeekend, "vega", "R1b"},
		{ShiftLK, "vega", "R2"},
		{ShiftLK, "not-an-abt", "R3"},
		{ShiftLKEvening, "vega", "R4a"},
		{ShiftLKEvening, "not-an-abt", "R4b"},
		{ShiftUSA, "vega", "R5"},
		{ShiftUSAWeekend, "vega", "R6"},
	}
	r := testResolver(&stubScheduleReader{})
	for _, tc := range cases {
		t.Run(tc.wantID+"/"+string(tc.shift), func(t *testing.T) {
			got, ok := r.RuleFor(RoutingContext{Shift: tc.shift, AssignedCRETeam: tc.team})
			if !ok {
				t.Fatalf("no rule matched shift %s, team %q", tc.shift, tc.team)
			}
			if got.ID != tc.wantID {
				t.Errorf("rule = %s, want %s", got.ID, tc.wantID)
			}
		})
	}
}

// The whole point of the new table: LEVEL_1 is the one lead of the incident's
// own team, LEVEL_2 is every ABT's lead. The previous model had these the
// other way round, so this is the assertion that pins the inversion.
func TestResolve_LeadRungsAreInverted(t *testing.T) {
	stub := &stubScheduleReader{members: []teamMember{
		member("vega", "vega.lead@example.com", roleLead, ""),
		member("atlas", "atlas.lead@example.com", roleLead, ""),
		member("apollo", "apollo.lead@example.com", roleLead, ""),
		member("vega", "vega.sublead@example.com", roleSubLead, ""),
	}}
	r := testResolver(stub)
	rc := RoutingContext{Shift: ShiftLK, AssignedCRETeam: "vega", At: time.Now()}

	one, err := r.Resolve(context.Background(), Level1, rc)
	if err != nil {
		t.Fatal(err)
	}
	if got := emails(one); len(got) != 1 || got[0] != "vega.lead@example.com" {
		t.Errorf("LEVEL_1 = %v, want only the incident's own team lead", got)
	}

	all, err := r.Resolve(context.Background(), Level2, rc)
	if err != nil {
		t.Fatal(err)
	}
	if got := emails(all); len(got) != 3 {
		t.Errorf("LEVEL_2 = %v, want every ABT lead", got)
	}
	// And a sub lead is nobody's rung any more.
	for _, e := range append(emails(one), emails(all)...) {
		if strings.Contains(e, "sublead") {
			t.Errorf("a sub lead was called (%s); the updated rules have no sub-lead rung", e)
		}
	}
}

func TestResolve_Level0PerRule(t *testing.T) {
	at := time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC)

	t.Run("R2 calls the incident's own ABT nominees", func(t *testing.T) {
		stub := &stubScheduleReader{members: []teamMember{
			member("vega", "v1@example.com", "engineer", "T1"),
			member("vega", "v2@example.com", "engineer", "T2"),
			member("vega", "v3@example.com", "engineer", "T3"),
			member("vega", "v9@example.com", "engineer", ""), // not nominated
			member("atlas", "a1@example.com", "engineer", "T1"),
		}}
		got, err := testResolver(stub).Resolve(context.Background(), Level0,
			RoutingContext{Shift: ShiftLK, AssignedCRETeam: "vega", At: at})
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"v1@example.com", "v2@example.com", "v3@example.com"}
		if !equalStrings(emails(got), want) {
			t.Errorf("LEVEL_0 = %v, want %v", emails(got), want)
		}
	})

	t.Run("R3 calls one nominee from every ABT", func(t *testing.T) {
		var members []teamMember
		for _, team := range testTeams.ABTs {
			members = append(members,
				member(team, team+".t1@example.com", "engineer", "T1"),
				member(team, team+".t2@example.com", "engineer", "T2"))
		}
		stub := &stubScheduleReader{members: members}
		got, err := testResolver(stub).Resolve(context.Background(), Level0,
			RoutingContext{Shift: ShiftLK, AssignedCRETeam: "not-an-abt", At: at})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != len(testTeams.ABTs) {
			t.Fatalf("LEVEL_0 reached %d people, want one per ABT (%d)", len(got), len(testTeams.ABTs))
		}
		// Lowest tier, so T1 rather than T2, and one per team not two.
		for _, e := range emails(got) {
			if !strings.Contains(e, ".t1@") {
				t.Errorf("%s was called; the lowest tier should answer first", e)
			}
		}
	})

	t.Run("R4a pairs the incident's own rota member with one other", func(t *testing.T) {
		stub := &stubScheduleReader{onDuty: []onDutyAssignment{
			onDutyFor("u1", "atlas.on@example.com", "atlas"),
			onDutyFor("u2", "vega.on@example.com", "vega"),
			onDutyFor("u3", "draco.on@example.com", "draco"),
		}}
		got, err := testResolver(stub).Resolve(context.Background(), Level0,
			RoutingContext{Shift: ShiftLKEvening, AssignedCRETeam: "vega", At: at})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 {
			t.Fatalf("LEVEL_0 = %v, want exactly two", emails(got))
		}
		if got[0].Email != "vega.on@example.com" {
			t.Errorf("first call = %s, want the incident's own team's rota member", got[0].Email)
		}
	})

	t.Run("R4b calls the whole evening rota when no ABT owns it", func(t *testing.T) {
		stub := &stubScheduleReader{onDuty: []onDutyAssignment{
			onDutyFor("u1", "a@example.com", "atlas"),
			onDutyFor("u2", "b@example.com", "vega"),
		}}
		got, err := testResolver(stub).Resolve(context.Background(), Level0,
			RoutingContext{Shift: ShiftLKEvening, AssignedCRETeam: "not-an-abt", At: at})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 {
			t.Errorf("LEVEL_0 = %v, want the whole rota", emails(got))
		}
	})

	t.Run("R5 calls the Americas nominees", func(t *testing.T) {
		stub := &stubScheduleReader{members: []teamMember{
			member("americas", "am1@example.com", "engineer", "T1"),
			member("americas", "am2@example.com", "engineer", "T2"),
			member("vega", "v1@example.com", "engineer", "T1"),
		}}
		got, err := testResolver(stub).Resolve(context.Background(), Level0,
			RoutingContext{Shift: ShiftUSA, AssignedCRETeam: "vega", At: at})
		if err != nil {
			t.Fatal(err)
		}
		if !equalStrings(emails(got), []string{"am1@example.com", "am2@example.com"}) {
			t.Errorf("LEVEL_0 = %v, want only the Americas nominees", emails(got))
		}
	})
}

// The evening pairing has to pick the same second person every time, or a
// retry reaches somebody the first attempt did not and the rung is untestable.
func TestResolve_RotaPairIsDeterministic(t *testing.T) {
	stub := &stubScheduleReader{onDuty: []onDutyAssignment{
		onDutyFor("u3", "c@example.com", "draco"),
		onDutyFor("u1", "a@example.com", "atlas"),
		onDutyFor("u2", "b@example.com", "vega"),
	}}
	r := testResolver(stub)
	rc := RoutingContext{Shift: ShiftLKEvening, AssignedCRETeam: "vega", At: time.Now()}

	first, err := r.Resolve(context.Background(), Level0, rc)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		again, err := r.Resolve(context.Background(), Level0, rc)
		if err != nil {
			t.Fatal(err)
		}
		if !equalStrings(emails(first), emails(again)) {
			t.Fatalf("pass %d gave %v, first gave %v", i, emails(again), emails(first))
		}
	}
}

// The heads sit outside every ABT, in their own team.
func TestResolve_HeadsComeFromTheLeadershipTeam(t *testing.T) {
	stub := &stubScheduleReader{members: []teamMember{
		member("cre-leadership", "cre.head@example.com", roleCREHead, ""),
		member("cre-leadership", "cs.head@example.com", roleCSHead, ""),
		member("vega", "vega.lead@example.com", roleLead, ""),
	}}
	r := testResolver(stub)
	rc := RoutingContext{Shift: ShiftLK, AssignedCRETeam: "vega", At: time.Now()}

	for _, tc := range []struct {
		level Level
		want  string
	}{{Level3, "cre.head@example.com"}, {Level4, "cs.head@example.com"}} {
		got, err := r.Resolve(context.Background(), tc.level, rc)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].Email != tc.want {
			t.Errorf("%s = %v, want %s", tc.level, emails(got), tc.want)
		}
	}
}

// A failure to ask is an error; a rung with nobody on it is not.
func TestResolve_ErrorsOnlyWhenItCannotAsk(t *testing.T) {
	boom := errors.New("entity-service is down")
	stub := &stubScheduleReader{membersErr: boom}
	_, err := testResolver(stub).Resolve(context.Background(), Level1,
		RoutingContext{Shift: ShiftLK, AssignedCRETeam: "vega"})
	if !errors.Is(err, boom) {
		t.Errorf("err = %v, want the underlying failure", err)
	}

	empty := &stubScheduleReader{}
	got, err := testResolver(empty).Resolve(context.Background(), Level1,
		RoutingContext{Shift: ShiftLK, AssignedCRETeam: "vega"})
	if err != nil {
		t.Errorf("an unstaffed rung must not be an error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("recipients = %v, want none", emails(got))
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// stubHistory is a call log the pairing rule can read.
type stubHistory struct {
	seen map[string]time.Time
	err  error
}

func (s stubHistory) LastCalled(_ context.Context, emails []string) (map[string]time.Time, error) {
	if s.err != nil {
		return nil, s.err
	}
	out := map[string]time.Time{}
	for _, e := range emails {
		if at, ok := s.seen[e]; ok {
			out[e] = at
		}
	}
	return out, nil
}

// The evening pairing's second call rotates: whoever has gone longest without
// one goes next. Without this the same person takes every out-of-hours
// incident, which is the whole reason the rule is not "the first name".
func TestResolve_RotaPairRotatesBySinceLastCalled(t *testing.T) {
	now := time.Date(2026, 9, 28, 19, 0, 0, 0, time.UTC)
	stub := &stubScheduleReader{onDuty: []onDutyAssignment{
		onDutyFor("u1", "own@example.com", "vega"),
		onDutyFor("u2", "recent@example.com", "atlas"),
		onDutyFor("u3", "stale@example.com", "draco"),
	}}
	history := stubHistory{seen: map[string]time.Time{
		"recent@example.com": now.Add(-1 * time.Hour),
		"stale@example.com":  now.Add(-72 * time.Hour),
	}}

	r := testResolver(stub).WithCallHistory(history)
	got, err := r.Resolve(context.Background(), Level0,
		RoutingContext{Shift: ShiftLKEvening, AssignedCRETeam: "vega", At: now})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"own@example.com", "stale@example.com"}
	if !equalStrings(emails(got), want) {
		t.Errorf("LEVEL_0 = %v, want %v (the longest wait goes next)", emails(got), want)
	}
}

// Somebody never called has waited longest of all -- that is what brings a new
// person into the rotation rather than leaving them permanently unpicked.
func TestResolve_RotaPairPrefersSomebodyNeverCalled(t *testing.T) {
	now := time.Date(2026, 9, 28, 19, 0, 0, 0, time.UTC)
	stub := &stubScheduleReader{onDuty: []onDutyAssignment{
		onDutyFor("u1", "own@example.com", "vega"),
		onDutyFor("u2", "called@example.com", "atlas"),
		onDutyFor("u3", "never@example.com", "draco"),
	}}
	history := stubHistory{seen: map[string]time.Time{
		"called@example.com": now.Add(-99 * time.Hour),
	}}

	got, err := testResolver(stub).WithCallHistory(history).
		Resolve(context.Background(), Level0,
			RoutingContext{Shift: ShiftLKEvening, AssignedCRETeam: "vega", At: now})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[1].Email != "never@example.com" {
		t.Errorf("second call = %v, want never@example.com", emails(got))
	}
}

// A history read that fails must not fail the rung: fairness is worth less
// than the page going out.
func TestResolve_RotaPairSurvivesAHistoryFailure(t *testing.T) {
	stub := &stubScheduleReader{onDuty: []onDutyAssignment{
		onDutyFor("u1", "own@example.com", "vega"),
		onDutyFor("u2", "other@example.com", "atlas"),
	}}
	got, err := testResolver(stub).WithCallHistory(stubHistory{err: errors.New("redis down")}).
		Resolve(context.Background(), Level0,
			RoutingContext{Shift: ShiftLKEvening, AssignedCRETeam: "vega", At: time.Now()})
	if err != nil {
		t.Fatalf("a history failure must not fail the rung: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("recipients = %v, want the pair anyway", emails(got))
	}
}
