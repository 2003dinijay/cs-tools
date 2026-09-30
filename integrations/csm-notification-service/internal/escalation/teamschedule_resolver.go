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
	"log/slog"
	"sort"
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
	// rules is the table this resolver routes by. Held rather than read from a
	// package variable so a deployment can correct a row without a release.
	rules []Rule
	// abtTeamKeys are the ABTs, in the order a "one from each" rung walks
	// them. Also what answers the rule table's "is assigned to an ABT team"
	// column -- a question the old table needed a publisher-supplied flag for,
	// and which nobody ever populated.
	abtTeamKeys []string
	// americasTeamKey is the team covering the night shift.
	americasTeamKey string
	// teamLeadKeys is which teams the "Team leads" rung spans; the ABTs when
	// configuration names none.
	teamLeadKeys []string
	// history answers when somebody was last called, for the evening
	// pairing's "one other member" rule. Optional: without it the pairing
	// falls back to a stable order, which is deterministic but not fair.
	history callHistory
	// leadershipTeamKey is the team the two heads belong to. They sit outside
	// every ABT on purpose, so that a head still resolves to no ABT for
	// /users/me -- the absence the Team Schedule page reads as "belongs to
	// neither group".
	leadershipTeamKey string
}

// teamScheduleReader is the slice of EntityClient this needs, named so tests
// can stand in for it without an HTTP server.
type teamScheduleReader interface {
	TeamMembers(ctx context.Context, teamKeys, roles, alertTiers []string) ([]teamMember, error)
	OnDutyAt(ctx context.Context, at time.Time) ([]onDutyAssignment, error)
}

// callHistory answers when each of these people was last called. Satisfied by
// *Store; nil is a valid value and means "no history to go on".
type callHistory interface {
	LastCalled(ctx context.Context, emails []string) (map[string]time.Time, error)
}

// WithCallHistory returns a copy that spreads the evening pairing's second
// call across the rota by who has gone longest without one.
func (r TeamScheduleResolver) WithCallHistory(h callHistory) TeamScheduleResolver {
	r.history = h
	return r
}

func NewTeamScheduleResolver(entity teamScheduleReader, teams TeamKeys, rules []Rule) TeamScheduleResolver {
	if strings.TrimSpace(teams.Leadership) == "" {
		teams.Leadership = defaultLeadershipTeamKey
	}
	if len(rules) == 0 {
		rules = DefaultRules
	}
	keys := make([]string, 0, len(teams.ABTs))
	for _, k := range teams.ABTs {
		if k = teamKeyFor(k); k != "" {
			keys = append(keys, k)
		}
	}
	leadKeys := make([]string, 0, len(teams.TeamLeads))
	for _, k := range teams.TeamLeads {
		if k = teamKeyFor(k); k != "" {
			leadKeys = append(leadKeys, k)
		}
	}
	if len(leadKeys) == 0 {
		leadKeys = keys
	}
	return TeamScheduleResolver{
		entity:            entity,
		rules:             rules,
		abtTeamKeys:       keys,
		teamLeadKeys:      leadKeys,
		americasTeamKey:   teamKeyFor(teams.Americas),
		leadershipTeamKey: teams.Leadership,
	}
}

// TeamKeys names the teams the rule table refers to by role rather than by
// name. They are configuration because they are deployment facts -- there are
// seven ABTs today and there will not always be -- and because a rung that
// silently reaches nobody because a team was renamed is the failure this whole
// resolver exists to avoid.
type TeamKeys struct {
	// ABTs are the ABT team keys.
	ABTs []string `yaml:"abts"`
	// TeamLeads is which teams the "Team leads" rung spans. Empty means every
	// ABT.
	//
	// It is configurable because the spreadsheet and the roster disagree and
	// only you can say which is right: the sheet counts that rung as three
	// calls everywhere it appears, while "the lead of every ABT" is seven with
	// seven ABTs. Naming three teams here makes it three; leaving it empty
	// keeps the literal reading. Either way the resolver and the sheet can be
	// made to agree without a release.
	TeamLeads []string `yaml:"teamLeads"`
	// Americas is the team covering the night shift.
	Americas string `yaml:"americas"`
	// Leadership is the team the two heads belong to.
	Leadership string `yaml:"leadership"`
}

const defaultLeadershipTeamKey = "cre-leadership"

// Role names, as migration 0170 constrains team_member.role.
//
// roleSubLead is no longer a rung: the updated rules make LEVEL_1 the team's
// one lead and LEVEL_2 every lead, so nothing resolves to a sub lead any more.
// The value stays in the schema and here because rows still carry it, and
// removing it would be a data migration for no gain.
const (
	roleSubLead = "sub_lead"
	roleLead    = "lead"
	roleCREHead = "cre_head"
	roleCSHead  = "cs_head"
)

// Resolve implements Resolver by looking the incident's rule up and asking
// that rule's source for the rung.
//
// An empty slice is a valid answer everywhere below: a rung that reaches
// nobody is logged and climbed past, never an error. An error is reserved for
// not being able to ask at all -- entity-service unreachable, or refusing.
func (r TeamScheduleResolver) Resolve(ctx context.Context, level Level, rc RoutingContext) ([]Recipient, error) {
	if level < Level0 || level > Level4 {
		return nil, fmt.Errorf("escalation: no rung %s", level)
	}
	rule, ok := r.RuleFor(rc)
	if !ok {
		// No row covers this shift. Not an error: the ladder reports the miss
		// and climbs, which is the same shape as a rung with nobody on it.
		return nil, nil
	}
	return r.fromSource(ctx, rule.Levels[level], rc)
}

// RuleFor is which row of the table an incident routes by.
//
// "Is assigned to an ABT team" is answered from the configured ABT keys, not
// from a flag on the payload. The old table needed a publisher to say, no
// publisher ever did, and every incident routed as UNKNOWN_ABT as a result.
// The question is answerable from data already in hand, so it is answered.
func (r TeamScheduleResolver) RuleFor(rc RoutingContext) (Rule, bool) {
	key := teamKeyFor(rc.AssignedCRETeam)
	return MatchRule(r.rules, rc.Shift, r.isABT(key), key != "")
}

func (r TeamScheduleResolver) isABT(teamKey string) bool {
	for _, k := range r.abtTeamKeys {
		if k == teamKey {
			return true
		}
	}
	return false
}

// fromSource answers one rung.
func (r TeamScheduleResolver) fromSource(ctx context.Context, src LevelSource, rc RoutingContext) ([]Recipient, error) {
	switch src {
	case SourceNone:
		return nil, nil

	case SourceRotaMembers:
		return r.rotaMembers(ctx, rc.At)

	case SourceRotaPair:
		return r.rotaPair(ctx, rc.At, teamKeyFor(rc.AssignedCRETeam))

	case SourceAlertDutyOwnABT:
		key := teamKeyFor(rc.AssignedCRETeam)
		if key == "" {
			return nil, nil
		}
		return r.alertDuty(ctx, []string{key})

	case SourceAlertDutyEachABT:
		return r.oneNomineePerTeam(ctx, r.abtTeamKeys)

	case SourceAlertDutyAmericas:
		return r.alertDuty(ctx, r.americasKeys())

	case SourceRotaMemberAndAlertDutyAmericas:
		rota, err := r.rotaMembers(ctx, rc.At)
		if err != nil {
			return nil, err
		}
		nominees, err := r.alertDuty(ctx, r.americasKeys())
		if err != nil {
			return nil, err
		}
		// One rota member, as the sheet's own count says, plus the nominees.
		if len(rota) > 1 {
			rota = rota[:1]
		}
		return dedupeRecipients(append(rota, nominees...)), nil

	case SourceTeamLead:
		return r.abtMembers(ctx, rc.AssignedCRETeam, roleLead)

	case SourceAllTeamLeads:
		return r.leadsOf(ctx, r.teamLeadKeys)

	case SourceAmericasTeamLeads:
		return r.leadsOf(ctx, r.americasKeys())

	case SourceAmericasTeamLead:
		return r.leadsOf(ctx, r.americasKeys())

	case SourceCREHead:
		return r.head(ctx, roleCREHead)

	case SourceCSHead:
		return r.head(ctx, roleCSHead)
	}
	return nil, fmt.Errorf("escalation: unknown level source %q", src)
}

func (r TeamScheduleResolver) americasKeys() []string {
	if r.americasTeamKey == "" {
		return nil
	}
	return []string{r.americasTeamKey}
}

// alertTiers is every nomination, in the order the sheet writes them.
var alertTiers = []string{"T1", "T2", "T3"}

// rotaMembers is everybody rostered at that instant, in a stable order.
func (r TeamScheduleResolver) rotaMembers(ctx context.Context, at time.Time) ([]Recipient, error) {
	onDuty, err := r.entity.OnDutyAt(ctx, at)
	if err != nil {
		return nil, err
	}
	var out []Recipient
	seen := map[string]bool{}
	for _, a := range onDuty {
		if a.Engineer.UserID == "" || seen[a.Engineer.UserID] {
			continue
		}
		seen[a.Engineer.UserID] = true
		out = append(out, Recipient{
			Email: a.Engineer.Email, Name: a.Engineer.Name, ShiftCode: a.ShiftCode,
		})
	}
	sortRecipients(out)
	return out, nil
}

// rotaPair is the evening rule: the rostered member from the incident's own
// team, and one other member of the same rota.
//
// "One other" is deliberately deterministic -- the next member in the rota's
// stable order -- so the same incident always calls the same two people. An
// arbitrary pick would make a retry reach somebody different from the first
// attempt and make the rung untestable.
func (r TeamScheduleResolver) rotaPair(ctx context.Context, at time.Time, teamKey string) ([]Recipient, error) {
	onDuty, err := r.entity.OnDutyAt(ctx, at)
	if err != nil {
		return nil, err
	}

	var own, others []Recipient
	seen := map[string]bool{}
	for _, a := range onDuty {
		if a.Engineer.UserID == "" || seen[a.Engineer.UserID] {
			continue
		}
		seen[a.Engineer.UserID] = true
		rec := Recipient{Email: a.Engineer.Email, Name: a.Engineer.Name, ShiftCode: a.ShiftCode}
		if teamKey != "" && teamKeyFor(a.TeamKey) == teamKey {
			own = append(own, rec)
			continue
		}
		others = append(others, rec)
	}
	sortRecipients(own)
	sortRecipients(others)

	var out []Recipient
	if len(own) > 0 {
		out = append(out, own[0])
	}
	pool := others
	if len(pool) == 0 && len(own) > 1 {
		// Nobody from another team is on: a second member of the same team is
		// still a second pair of eyes, which is what the rule is for.
		pool = own[1:]
	}
	if second, ok := r.longestSinceCalled(ctx, pool); ok {
		out = append(out, second)
	}
	return out, nil
}

// longestSinceCalled picks whoever has gone longest without a call, so the
// evening's second call rotates around the rota instead of always landing on
// the same person.
//
// Never called counts as the longest wait of all, which is what brings
// somebody new into the rotation the first time. Ties -- including the case
// where there is no history at all -- break on email, so the answer stays
// deterministic and a retry reaches the same person as the first attempt.
func (r TeamScheduleResolver) longestSinceCalled(ctx context.Context, pool []Recipient) (Recipient, bool) {
	if len(pool) == 0 {
		return Recipient{}, false
	}
	if r.history == nil {
		return pool[0], true
	}

	emails := make([]string, 0, len(pool))
	for _, p := range pool {
		emails = append(emails, p.Email)
	}
	seen, err := r.history.LastCalled(ctx, emails)
	if err != nil {
		// Fairness is not worth failing a rung over: fall back to the stable
		// order, which is still deterministic.
		slog.WarnContext(ctx, "escalation: could not read call history; pairing falls back to a stable order",
			"err", err)
		return pool[0], true
	}

	best := pool[0]
	bestAt, bestKnown := seen[best.Email]
	for _, cand := range pool[1:] {
		at, known := seen[cand.Email]
		switch {
		case !known && bestKnown:
			best, bestAt, bestKnown = cand, at, false
		case known && bestKnown && at.Before(bestAt):
			best, bestAt = cand, at
		}
	}
	return best, true
}

// alertDuty is every nominee of the named teams.
func (r TeamScheduleResolver) alertDuty(ctx context.Context, teamKeys []string) ([]Recipient, error) {
	if len(teamKeys) == 0 {
		return nil, nil
	}
	members, err := r.entity.TeamMembers(ctx, teamKeys, nil, alertTiers)
	if err != nil {
		return nil, err
	}
	out := recipientsOf(members)
	sortRecipients(out)
	return out, nil
}

// oneNomineePerTeam takes a single nominee from each team, lowest tier first.
//
// This is R3: an incident assigned to no ABT reaches one person in every ABT,
// so whichever team it turns out to belong to has somebody already looking.
// With seven ABTs that is seven calls -- more than the sheet's own count for
// that row, which is recorded on the rule and checked against.
func (r TeamScheduleResolver) oneNomineePerTeam(ctx context.Context, teamKeys []string) ([]Recipient, error) {
	if len(teamKeys) == 0 {
		return nil, nil
	}
	members, err := r.entity.TeamMembers(ctx, teamKeys, nil, alertTiers)
	if err != nil {
		return nil, err
	}

	// Lowest tier wins, then email, so the choice is stable across calls.
	byTeam := map[string]teamMember{}
	for _, m := range members {
		cur, seen := byTeam[m.TeamKey]
		if !seen || m.AlertTier < cur.AlertTier ||
			(m.AlertTier == cur.AlertTier && m.Email < cur.Email) {
			byTeam[m.TeamKey] = m
		}
	}

	var out []Recipient
	for _, key := range teamKeys {
		if m, ok := byTeam[key]; ok {
			out = append(out, Recipient{Email: m.Email, Name: m.Name})
		}
	}
	return out, nil
}

// leadsOf is the lead of each named team.
func (r TeamScheduleResolver) leadsOf(ctx context.Context, teamKeys []string) ([]Recipient, error) {
	if len(teamKeys) == 0 {
		return nil, nil
	}
	members, err := r.entity.TeamMembers(ctx, teamKeys, []string{roleLead}, nil)
	if err != nil {
		return nil, err
	}
	out := recipientsOf(members)
	sortRecipients(out)
	return out, nil
}

func sortRecipients(rs []Recipient) {
	sort.Slice(rs, func(i, j int) bool { return rs[i].Email < rs[j].Email })
}

func dedupeRecipients(rs []Recipient) []Recipient {
	seen := map[string]bool{}
	out := rs[:0]
	for _, r := range rs {
		if r.Email == "" || seen[r.Email] {
			continue
		}
		seen[r.Email] = true
		out = append(out, r)
	}
	return out
}

func (r TeamScheduleResolver) abtMembers(ctx context.Context, team, role string) ([]Recipient, error) {
	key := teamKeyFor(team)
	if key == "" {
		// An incident with no team is a real, reported state, not a failure:
		// the rule table routes it to a shift-wide pool. There is no pool to
		// read here yet, so this rung reaches nobody and the ladder climbs.
		return nil, nil
	}
	members, err := r.entity.TeamMembers(ctx, []string{key}, []string{role}, nil)
	if err != nil {
		return nil, err
	}
	return recipientsOf(members), nil
}

func (r TeamScheduleResolver) head(ctx context.Context, role string) ([]Recipient, error) {
	members, err := r.entity.TeamMembers(ctx, []string{r.leadershipTeamKey}, []string{role}, nil)
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
