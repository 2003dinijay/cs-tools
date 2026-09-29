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
	"testing"
	"time"
)

type stubScheduleReader struct {
	members    []teamMember
	onDuty     []onDutyAssignment
	membersErr error
	onDutyErr  error

	gotTeamKeys []string
	gotRoles    []string
	gotAt       time.Time
	memberCalls int
}

func (s *stubScheduleReader) TeamMembers(_ context.Context, teamKeys, roles []string) ([]teamMember, error) {
	s.gotTeamKeys, s.gotRoles = teamKeys, roles
	s.memberCalls++
	return s.members, s.membersErr
}

func (s *stubScheduleReader) OnDutyAt(_ context.Context, at time.Time) ([]onDutyAssignment, error) {
	s.gotAt = at
	return s.onDuty, s.onDutyErr
}

func onDutyFor(userID, email, teamKey string) onDutyAssignment {
	var a onDutyAssignment
	a.Engineer.UserID, a.Engineer.Email, a.Engineer.Name = userID, email, email
	a.TeamKey = teamKey
	return a
}

// LEVEL_0 is the intersection, not either half: being rostered is not enough,
// and being a sub lead who is off shift is not enough either.
func TestTeamScheduleResolver_Level0IntersectsRotaWithRank(t *testing.T) {
	at := time.Date(2026, 9, 27, 22, 0, 0, 0, time.UTC)
	stub := &stubScheduleReader{
		onDuty: []onDutyAssignment{
			onDutyFor("u1", "rostered.notlead@example.com", "vega"),
			onDutyFor("u2", "rostered.sublead@example.com", "vega"),
		},
		members: []teamMember{{UserID: "u2", Email: "rostered.sublead@example.com", Role: roleSubLead}},
	}

	got, err := NewTeamScheduleResolver(stub, "").Resolve(context.Background(), Level0, RoutingContext{At: at})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Email != "rostered.sublead@example.com" {
		t.Fatalf("recipients = %+v, want only the rostered sub lead", got)
	}
	if !stub.gotAt.Equal(at) {
		t.Errorf("asked about %v, want the trigger instant %v", stub.gotAt, at)
	}
}

// Somebody rostered on two windows covering the same instant is one person to
// ring, not two.
func TestTeamScheduleResolver_Level0DoesNotCallTwice(t *testing.T) {
	stub := &stubScheduleReader{
		onDuty: []onDutyAssignment{
			onDutyFor("u1", "on.both@example.com", "vega"),
			onDutyFor("u1", "on.both@example.com", "americas"),
		},
		members: []teamMember{{UserID: "u1", Email: "on.both@example.com", Role: roleSubLead}},
	}
	got, err := NewTeamScheduleResolver(stub, "").Resolve(context.Background(), Level0, RoutingContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("recipients = %+v, want one", got)
	}
}

// Nobody on shift is a rung that reaches nobody, which the ladder climbs past.
// It must not be an error, and it must not cost a second call.
func TestTeamScheduleResolver_Level0NobodyOnShift(t *testing.T) {
	stub := &stubScheduleReader{}
	got, err := NewTeamScheduleResolver(stub, "").Resolve(context.Background(), Level0, RoutingContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("recipients = %+v, want none", got)
	}
	if stub.memberCalls != 0 {
		t.Error("asked for ranks despite nobody being rostered")
	}
}

func TestTeamScheduleResolver_RungsAskForTheRightThing(t *testing.T) {
	cases := []struct {
		level    Level
		team     string
		wantKey  string
		wantRole string
	}{
		{Level1, "Vega", "vega", roleSubLead},
		{Level2, "  VEGA  ", "vega", roleLead},
		{Level3, "Vega", "cre-leadership", roleCREHead},
		{Level4, "Vega", "cre-leadership", roleCSHead},
	}
	for _, tc := range cases {
		t.Run(tc.level.String(), func(t *testing.T) {
			stub := &stubScheduleReader{}
			if _, err := NewTeamScheduleResolver(stub, "").Resolve(
				context.Background(), tc.level, RoutingContext{AssignedCRETeam: tc.team}); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(stub.gotTeamKeys) != 1 || stub.gotTeamKeys[0] != tc.wantKey {
				t.Errorf("teamKeys = %v, want [%s]", stub.gotTeamKeys, tc.wantKey)
			}
			if len(stub.gotRoles) != 1 || stub.gotRoles[0] != tc.wantRole {
				t.Errorf("roles = %v, want [%s]", stub.gotRoles, tc.wantRole)
			}
		})
	}
}

// An incident with no team is a real state the rule table routes to a pool.
// There is no pool to read yet, so the rung reaches nobody -- but it must not
// ask entity-service about a team named "".
func TestTeamScheduleResolver_NoTeamAsksNothing(t *testing.T) {
	stub := &stubScheduleReader{}
	got, err := NewTeamScheduleResolver(stub, "").Resolve(context.Background(), Level1, RoutingContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 || stub.memberCalls != 0 {
		t.Errorf("recipients = %+v, calls = %d, want none of either", got, stub.memberCalls)
	}
}

// Not being able to ask is an error. It must not read as "nobody to call",
// which would silently skip a rung on an incident nobody has answered.
func TestTeamScheduleResolver_UnreachableIsAnError(t *testing.T) {
	boom := errors.New("entity-service refused")
	for name, stub := range map[string]*stubScheduleReader{
		"membership lookup": {membersErr: boom},
		"rota lookup":       {onDutyErr: boom},
	} {
		t.Run(name, func(t *testing.T) {
			level := Level1
			if stub.onDutyErr != nil {
				level = Level0
			}
			_, err := NewTeamScheduleResolver(stub, "").Resolve(
				context.Background(), level, RoutingContext{AssignedCRETeam: "vega"})
			if !errors.Is(err, boom) {
				t.Fatalf("err = %v, want it surfaced", err)
			}
		})
	}
}
