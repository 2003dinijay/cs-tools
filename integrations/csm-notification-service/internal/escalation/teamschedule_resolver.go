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
	"fmt"
	"strings"
	"time"
)

// TeamScheduleResolver answers each rung from the Team Schedule, replacing the
// hand-maintained roster RosterResolver reads.
//
// # THE RUNG MODEL HERE IS AN ASSUMPTION
//
// Who each rung is was described by the CS team and has not been confirmed.
// It exists so there is a complete, valid flow to exercise end to end while
// the real one is settled, and it is expected to change. Nothing below should
// be read as the decided design.
//
//	LEVEL_0  the sub lead rostered on the rotation covering that instant
//	LEVEL_1  the incident's own ABT's sub leads, called together
//	LEVEL_2  that ABT's lead
//	LEVEL_3  the CRE head
//	LEVEL_4  the CS head
//
// LEVEL_0 takes two calls, not one: who is rostered comes from the rota, and
// what rank they hold comes from the membership behind it. They are separate
// because rank changes rarely and the rota changes daily, and because folding
// rank into the rota response would widen a shape the Team Schedule page
// already renders.
type TeamScheduleResolver struct {
	entity teamScheduleReader
	// leadershipTeamKey is the team the two heads belong to. They sit outside
	// every ABT on purpose, so that a head still resolves to no ABT for
	// /users/me -- the absence the Team Schedule page reads as "belongs to
	// neither group".
	leadershipTeamKey string
}

// teamScheduleReader is the slice of EntityClient this needs, named so tests
// can stand in for it without an HTTP server.
type teamScheduleReader interface {
	TeamMembers(ctx context.Context, teamKeys, roles []string) ([]teamMember, error)
	OnDutyAt(ctx context.Context, at time.Time) ([]onDutyAssignment, error)
}

func NewTeamScheduleResolver(entity teamScheduleReader, leadershipTeamKey string) TeamScheduleResolver {
	if strings.TrimSpace(leadershipTeamKey) == "" {
		leadershipTeamKey = defaultLeadershipTeamKey
	}
	return TeamScheduleResolver{entity: entity, leadershipTeamKey: leadershipTeamKey}
}

const defaultLeadershipTeamKey = "cre-leadership"

// Role names, as migration 000106 constrains team_member.role.
const (
	roleSubLead = "sub_lead"
	roleLead    = "lead"
	roleCREHead = "cre_head"
	roleCSHead  = "cs_head"
)

// Resolve implements Resolver.
//
// An empty slice is a valid answer everywhere below: a rung that reaches
// nobody is logged and climbed past, never an error. An error is reserved for
// not being able to ask at all -- entity-service unreachable, or refusing.
func (r TeamScheduleResolver) Resolve(ctx context.Context, level Level, rc RoutingContext) ([]Recipient, error) {
	switch level {
	case Level0:
		return r.onCallSubLead(ctx, rc.At)
	case Level1:
		return r.abtMembers(ctx, rc.AssignedCRETeam, roleSubLead)
	case Level2:
		return r.abtMembers(ctx, rc.AssignedCRETeam, roleLead)
	case Level3:
		return r.head(ctx, roleCREHead)
	case Level4:
		return r.head(ctx, roleCSHead)
	}
	return nil, fmt.Errorf("escalation: no rung %s", level)
}

// onCallSubLead intersects who is rostered at that instant with who is a sub
// lead.
//
// The intersection is done here rather than by asking entity-service for it
// because the two facts live in different places and neither endpoint owns
// both. The cost is one extra round trip per ladder, paid once when the plan
// is built rather than per attempt.
func (r TeamScheduleResolver) onCallSubLead(ctx context.Context, at time.Time) ([]Recipient, error) {
	onDuty, err := r.entity.OnDutyAt(ctx, at)
	if err != nil {
		return nil, err
	}
	if len(onDuty) == 0 {
		return nil, nil
	}

	teams := make([]string, 0, len(onDuty))
	seenTeam := map[string]bool{}
	for _, a := range onDuty {
		if a.TeamKey != "" && !seenTeam[a.TeamKey] {
			seenTeam[a.TeamKey] = true
			teams = append(teams, a.TeamKey)
		}
	}
	subLeads, err := r.entity.TeamMembers(ctx, teams, []string{roleSubLead})
	if err != nil {
		return nil, err
	}

	// Keyed by user rather than by (user, team): a sub lead rostered under one
	// team is still a sub lead, and the rota's team_id is nullable for a
	// registry-only team, so pairing on it would silently drop people.
	isSubLead := make(map[string]bool, len(subLeads))
	for _, m := range subLeads {
		isSubLead[m.UserID] = true
	}

	var out []Recipient
	seenUser := map[string]bool{}
	for _, a := range onDuty {
		id := a.Engineer.UserID
		if !isSubLead[id] || seenUser[id] {
			continue
		}
		seenUser[id] = true
		out = append(out, Recipient{Email: a.Engineer.Email, Name: a.Engineer.Name})
	}
	return out, nil
}

func (r TeamScheduleResolver) abtMembers(ctx context.Context, team, role string) ([]Recipient, error) {
	key := teamKeyFor(team)
	if key == "" {
		// An incident with no team is a real, reported state, not a failure:
		// the rule table routes it to a shift-wide pool. There is no pool to
		// read here yet, so this rung reaches nobody and the ladder climbs.
		return nil, nil
	}
	members, err := r.entity.TeamMembers(ctx, []string{key}, []string{role})
	if err != nil {
		return nil, err
	}
	return recipientsOf(members), nil
}

func (r TeamScheduleResolver) head(ctx context.Context, role string) ([]Recipient, error) {
	members, err := r.entity.TeamMembers(ctx, []string{r.leadershipTeamKey}, []string{role})
	if err != nil {
		return nil, err
	}
	return recipientsOf(members), nil
}

// teamKeyFor turns the team an incident names into the key the rota uses.
//
// KNOWN GAP. The incident carries ServiceNow's assignment group name ("Vega",
// possibly "Team Vega"); the rota is keyed "vega". Lower-casing and trimming
// is enough for the names seeded today and is certainly not enough in general
// -- a group whose name is not simply its key in another case will resolve to
// nobody, and the rung will read as unstaffed rather than unmapped. A real
// mapping belongs wherever the group is defined, not guessed at here.
func teamKeyFor(team string) string {
	return strings.ToLower(strings.TrimSpace(team))
}

// recipientsOf carries no Phone: "user" has no phone column, so a rota
// resolved recipient can only be reached over chat today. The ladder already
// records a recipient without a number as [NO_NUMBER] and carries on, so this
// degrades rather than fails -- but a voice run needs numbers from somewhere
// before it can reach anybody.
func recipientsOf(members []teamMember) []Recipient {
	var out []Recipient
	for _, m := range members {
		out = append(out, Recipient{Email: m.Email, Name: m.Name})
	}
	return out
}
