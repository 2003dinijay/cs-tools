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
	"sort"
	"strings"
	"time"
)

// The SRE half of TeamScheduleResolver. Kept apart from the CRE half because
// the two share nothing but the rota client: the CRE ladder routes by a rule
// table of shifts and ranks, the SRE ladder by on-call tier.
//
//	LEVEL_0  L1 support   whoever holds tier L1 at the trigger instant
//	LEVEL_1  L2 support   tier L2
//	LEVEL_2  L3 support   tier L3
//	LEVEL_3  L4 support   OPT-IN (timing.includeL4) and NOT CONFIRMED: the
//	                      lead of the SRE team answering the incident
//	LEVEL_4  nobody       the SRE ladder has no fifth rung
//
// Every rung reaches ONE person. Weekdays 12:00-15:00 IST the TZ1 and TZ2
// escalation windows are both live, so a tier can have two holders at once;
// the SRE team confirmed only one of them is called.

// familySRE is how the catalogue spells the SRE family, for teams and windows
// alike (entity-service folds the registry's SRE-ABT into it).
const familySRE = "SRE"

// keyOf turns an assignment group into a rota team key, honouring a
// configured alias first.
func (r TeamScheduleResolver) keyOf(team string) string {
	key := teamKeyFor(team)
	if alias, ok := r.aliases[key]; ok {
		return alias
	}
	return key
}

// isDefaultGroup reports whether the assignment group is one of
// sre.teams.defaultGroups: a group holding incidents that have no SRE team of
// their own. The group's own name is matched as well as its alias, so an
// alias for "Default" does not hide it.
func (r TeamScheduleResolver) isDefaultGroup(team string) bool {
	key := teamKeyFor(team)
	if key == "" {
		return false
	}
	return contains(r.defaultGroups, key) || contains(r.defaultGroups, r.keyOf(team))
}

func (r TeamScheduleResolver) isSRETeam(key string) bool {
	for _, k := range r.sreTeamKeys {
		if k == key {
			return true
		}
	}
	return false
}

// LadderFor implements LadderClassifier: an incident climbs the SRE ladder
// when the team it is assigned to is an SRE team, and the CRE one otherwise --
// including when it has no team, or one the rota does not know.
//
// The configured SRE team list answers first, the same way the configured
// ABT list answers the CRE table's "is this an ABT" column. Only with no list
// configured is the catalogue asked, so a deployment without the file still
// classifies.
func (r TeamScheduleResolver) LadderFor(ctx context.Context, rc RoutingContext) (Ladder, error) {
	key := r.keyOf(rc.AssignedCRETeam)
	if key == "" {
		return LadderCRE, nil
	}
	if r.isDefaultGroup(rc.AssignedCRETeam) {
		return LadderSRE, nil
	}
	if len(r.sreTeamKeys) > 0 {
		if r.isSRETeam(key) {
			return LadderSRE, nil
		}
		return LadderCRE, nil
	}
	cat, err := r.entity.ScheduleCatalogue(ctx)
	if err != nil {
		return LadderCRE, err
	}
	for _, t := range cat.Teams {
		if strings.EqualFold(t.Key, key) {
			if strings.EqualFold(t.Family, familySRE) {
				return LadderSRE, nil
			}
			return LadderCRE, nil
		}
	}
	return LadderCRE, nil
}

// TeamFamily implements TeamFamilyResolver: which family the incident's
// assignment group belongs to, for routing.
//
//	sre   an SRE team -- the configured sre.teams.abts, or the catalogue's SRE
//	      family when none are configured
//	cre   any other team the rota knows
//	none  no assignment group, or one neither the configuration nor the rota
//	      knows
func (r TeamScheduleResolver) TeamFamily(ctx context.Context, rc RoutingContext) (string, error) {
	key := r.keyOf(rc.AssignedCRETeam)
	if key == "" {
		return TeamFamilyNone, nil
	}
	if r.isSRETeam(key) || r.isDefaultGroup(rc.AssignedCRETeam) {
		return TeamFamilySRE, nil
	}
	if contains(r.abtTeamKeys, key) || key == r.americasTeamKey {
		return TeamFamilyCRE, nil
	}
	cat, err := r.entity.ScheduleCatalogue(ctx)
	if err != nil {
		return TeamFamilyNone, err
	}
	for _, t := range cat.Teams {
		if strings.EqualFold(t.Key, key) {
			if strings.EqualFold(t.Family, familySRE) {
				return TeamFamilySRE, nil
			}
			return TeamFamilyCRE, nil
		}
	}
	return TeamFamilyNone, nil
}

// sreTier is the rota tier each SRE rung reads.
var sreTier = map[Level]string{Level0: "L1", Level1: "L2", Level2: "L3"}

// ownSRETeam is the incident's own SRE team key, or "" for one that has none.
func (r TeamScheduleResolver) ownSRETeam(rc RoutingContext) string {
	own := r.keyOf(rc.AssignedCRETeam)
	if r.isDefaultGroup(rc.AssignedCRETeam) {
		// A "Default" incident has no SRE team: whoever is on duty answers.
		return ""
	}
	if !r.isSRETeam(own) && len(r.sreTeamKeys) > 0 {
		// A CRE incident climbing the SRE ladder (a P0) belongs to no SRE
		// team; every SRE team is then equally placed to answer.
		return ""
	}
	return own
}

// rotaFor decides which SRE rota pages an incident: its own team's, or
// sre.teams.defaultRota for an incident with no SRE team of its own (a case at
// S0, a monitoring alert raised with no team). "" when the catalogue names no
// rotas at all: an entity-service from before rotas, where every SRE window
// is read, as it always was.
func (r TeamScheduleResolver) rotaFor(cat scheduleCatalogue, own string) string {
	if !cat.hasRotas() {
		return ""
	}
	if own != "" {
		if rota := cat.teamRota(own); rota != "" {
			return rota
		}
	}
	return r.defaultRota
}

// SRERota implements SRERotaResolver.
func (r TeamScheduleResolver) SRERota(ctx context.Context, rc RoutingContext) (string, error) {
	cat, err := r.entity.ScheduleCatalogue(ctx)
	if err != nil {
		return "", err
	}
	return r.rotaFor(cat, r.ownSRETeam(rc)), nil
}

// SaaSSRETeam implements SpecialistResolver: whether an assignment group is a
// SaaS SRE team, the only kind whose incidents may be escalated to an SME.
//
// The team's rota answers when the catalogue has rotas. Without them, a team
// counts when it is an SRE team (sre.teams.abts, or the catalogue's SRE family
// with no list configured) and not typed sre-iaas -- team.type is what the
// rota is derived from, so the answer is the same one.
func (r TeamScheduleResolver) SaaSSRETeam(ctx context.Context, group string) (bool, error) {
	key := r.keyOf(group)
	if key == "" {
		return false, nil
	}
	cat, err := r.entity.ScheduleCatalogue(ctx)
	if err != nil {
		return false, err
	}
	if cat.hasRotas() {
		return strings.EqualFold(cat.teamRota(key), RotaSRESaaS), nil
	}
	sre := r.isSRETeam(key)
	if !sre && len(r.sreTeamKeys) == 0 {
		for _, t := range cat.Teams {
			if strings.EqualFold(t.Key, key) && strings.EqualFold(t.Family, familySRE) {
				sre = true
			}
		}
	}
	if !sre {
		return false, nil
	}
	members, err := r.entity.TeamMembers(ctx, []string{key}, nil, nil, nil)
	if err != nil {
		return false, err
	}
	for _, m := range members {
		if strings.EqualFold(m.TeamType, teamTypeSREIaaS) {
			return false, nil
		}
	}
	return true, nil
}

// teamTypeSREIaaS is the team.type of the IaaS SRE team (migration 0200).
const teamTypeSREIaaS = "sre-iaas"

// resolveSRE answers one SRE rung.
//
// L1 and L2 reach the ONE holder of their tier. L3 does too, but when nobody
// at all holds L1, L2 or L3 for the incident, the L3 rung calls the team lead
// instead -- or, for an incident with no SRE team of its own, the lead of
// every team on the rota that pages it (both SaaS teams by default) -- so an
// unstaffed rota still reaches somebody. The opt-in L4 rung (the lead) is then
// skipped: the lead has already been called.
func (r TeamScheduleResolver) resolveSRE(ctx context.Context, level Level, rc RoutingContext) ([]Recipient, error) {
	own := r.ownSRETeam(rc)

	switch level {
	case Level0, Level1:
		pick, found, err := r.onCallTier(ctx, rc.At, own, sreTier[level], rc.Rota)
		if err != nil || !found {
			return nil, err
		}
		return []Recipient{pick.Recipient}, nil
	case Level2:
		picks, err := r.tierPicks(ctx, rc.At, own, rc.Rota, "L1", "L2", "L3")
		if err != nil {
			return nil, err
		}
		if pick, ok := picks["L3"]; ok {
			return []Recipient{pick.Recipient}, nil
		}
		if len(picks) > 0 {
			// Somebody holds L1 or L2: a missing L3 is just an empty rung.
			return nil, nil
		}
		return r.leadsForUnstaffedRota(ctx, own, rc.Rota)
	case Level3:
		// L4 is the lead of the team answering the incident: its own, or -- for
		// an incident that has none -- the team of the first tier somebody
		// holds, L1, then L2, then L3.
		picks, err := r.tierPicks(ctx, rc.At, own, rc.Rota, "L1", "L2", "L3")
		if err != nil || len(picks) == 0 {
			// Nobody on L1-L3: the leads were called on the L3 rung.
			return nil, err
		}
		team := own
		if team == "" {
			for _, tier := range []string{"L1", "L2", "L3"} {
				if pick, ok := picks[tier]; ok && pick.team != "" {
					team = pick.team
					break
				}
			}
		}
		if team == "" {
			return nil, nil
		}
		leads, err := r.leadsOf(ctx, []string{team})
		if err != nil || len(leads) == 0 {
			return nil, err
		}
		return leads[:1], nil
	}
	return nil, nil
}

// leadsForUnstaffedRota is who the L3 rung calls when nobody holds L1, L2 or
// L3: the incident's own team lead, or -- for an incident with no SRE team of
// its own -- one lead from every team on the rota that pages it (sre.teams.abts
// when the catalogue names no rotas).
func (r TeamScheduleResolver) leadsForUnstaffedRota(ctx context.Context, own, rota string) ([]Recipient, error) {
	if own != "" {
		leads, err := r.leadsOf(ctx, []string{own})
		if err != nil || len(leads) == 0 {
			return nil, err
		}
		return leads[:1], nil
	}
	cat, err := r.entity.ScheduleCatalogue(ctx)
	if err != nil {
		return nil, err
	}
	var teams []string
	if cat.hasRotas() {
		if rota == "" {
			rota = r.rotaFor(cat, "")
		}
		for _, t := range cat.Teams {
			if rota != "" && strings.EqualFold(deref(t.RotaCode), rota) && t.Key != "" {
				teams = append(teams, strings.ToLower(t.Key))
			}
		}
	} else {
		teams = append(teams, r.sreTeamKeys...)
	}
	if len(teams) == 0 {
		return nil, nil
	}
	members, err := r.entity.TeamMembers(ctx, teams, []string{roleLead}, nil, nil)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(members, func(i, j int) bool { return members[i].Email < members[j].Email })
	var out []Recipient
	seen := map[string]bool{}
	for _, m := range members {
		team := strings.ToLower(m.TeamKey)
		if seen[team] {
			continue
		}
		seen[team] = true
		out = append(out, recipientsOf([]teamMember{m})...)
	}
	sortRecipients(out)
	return out, nil
}

// tierHolder is one candidate for an SRE rung.
type tierHolder struct {
	Recipient
	team    string
	zone    string
	onLeave bool
}

// preferAvailable keeps only the holders who are not on leave when there are
// any; otherwise it keeps them all. On-leave holders are only there at all
// when the ladder's onLeave is call (the on-duty read leaves them out
// otherwise), so this is "ring someone on leave only when nobody else holds
// the tier".
func preferAvailable(holders []tierHolder) []tierHolder {
	var available []tierHolder
	for _, h := range holders {
		if !h.onLeave {
			available = append(available, h)
		}
	}
	if len(available) > 0 {
		return available
	}
	return holders
}

// sharedCrewZone is the SaaS night window: TZ3 (21:00-06:00 IST, weekends
// included) is one crew for both SaaS teams, so its holders answer either
// team's incident. Every other window is the team's own.
const sharedCrewZone = "TZ3"

// onCallTier picks the ONE person holding tier on an SRE window at that
// instant. See tierPicks.
func (r TeamScheduleResolver) onCallTier(ctx context.Context, at time.Time, ownTeam, tier, rota string) (tierHolder, bool, error) {
	picks, err := r.tierPicks(ctx, at, ownTeam, rota, tier)
	if err != nil {
		return tierHolder{}, false, err
	}
	pick, ok := picks[tier]
	return pick, ok, nil
}

// tierPicks picks the ONE person holding each of tiers on an SRE window at
// that instant, from one read of the rota. A tier nobody holds is absent.
//
// An incident with an SRE team of its own reaches only that team's people,
// plus the shared night crew (sharedCrewZone): when the team has nobody on a
// tier by day, the rung is empty rather than another team's holder -- the
// ladder climbs to the team's next tier, and the L3 rung calls the team lead
// when L1-L3 are all empty. An incident with no SRE team of its own reaches
// whoever holds the tier on its rota.
//
// Then, in order: someone not on leave over someone on leave; the zone whose
// L1 block is live, which is the zone that owns the incident right now and is
// what makes the 12:00-15:00 overlap call one person rather than two; then the
// configured SRE team order; then email, so a retry reaches the same person.
//
// Only the windows of one rota count -- SaaS SRE and IaaS SRE each run their
// own chain, and a SaaS incident must never reach an IaaS engineer or the
// other way round. rota is the one the engine stamped; empty asks rotaFor.
// With no rotas in the catalogue at all every SRE window counts, as before.
func (r TeamScheduleResolver) tierPicks(ctx context.Context, at time.Time, ownTeam, rota string, tiers ...string) (map[string]tierHolder, error) {
	cat, err := r.entity.ScheduleCatalogue(ctx)
	if err != nil {
		return nil, err
	}
	if rota == "" {
		rota = r.rotaFor(cat, ownTeam)
	}
	zoneRota := cat.zoneRotas()
	type window struct{ zone, tier string }
	sre := map[string]window{}
	for _, s := range cat.Shifts {
		if !strings.EqualFold(s.Family, familySRE) {
			continue
		}
		if rota != "" && !strings.EqualFold(zoneRota[deref(s.ZoneCode)], rota) {
			continue
		}
		sre[s.Code] = window{zone: deref(s.ZoneCode), tier: deref(s.Tier)}
	}
	onDuty, err := r.entity.OnDutyAt(ctx, at, r.callOnLeave)
	if err != nil {
		return nil, err
	}

	byTier := map[string][]tierHolder{}
	l1Zone := ""
	for _, a := range onDuty {
		w, ok := sre[a.ShiftCode]
		if !ok || a.Engineer.UserID == "" {
			continue
		}
		held := deref(a.Tier)
		if held == "" {
			held = w.tier
		}
		zone := deref(a.ZoneCode)
		if zone == "" {
			zone = w.zone
		}
		if strings.EqualFold(held, "L1") && (l1Zone == "" || zone < l1Zone) {
			l1Zone = zone
		}
		team := r.keyOf(a.TeamKey)
		if ownTeam != "" && team != ownTeam && !strings.EqualFold(zone, sharedCrewZone) {
			continue
		}
		key := strings.ToUpper(held)
		byTier[key] = append(byTier[key], tierHolder{
			Recipient: Recipient{Email: a.Engineer.Email, Name: a.Engineer.Name, UserID: a.Engineer.UserID, ShiftCode: a.ShiftCode},
			team:      team,
			zone:      zone,
			onLeave:   a.OnLeave,
		})
	}

	order := func(team string) int {
		for i, k := range r.sreTeamKeys {
			if k == team {
				return i
			}
		}
		return len(r.sreTeamKeys)
	}
	out := map[string]tierHolder{}
	for _, tier := range tiers {
		holders := preferAvailable(byTier[strings.ToUpper(tier)])
		if len(holders) == 0 {
			continue
		}
		sort.SliceStable(holders, func(i, j int) bool {
			a, b := holders[i], holders[j]
			if (a.zone == l1Zone) != (b.zone == l1Zone) {
				return a.zone == l1Zone
			}
			if order(a.team) != order(b.team) {
				return order(a.team) < order(b.team)
			}
			return a.Email < b.Email
		})
		out[tier] = holders[0]
	}
	return out, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// familySME is how the catalogue spells the Special Ops family.
const familySME = "SME"

// resolveSME answers one rung of an SME ladder: the ONE person holding its
// tier (L1, L2, L3 for LEVEL_0..LEVEL_2) on an SME-family window of the SME
// team at that instant -- several on the tier, the one longest since last
// called, then by email. Someone rostered on the window with no tier counts
// as L1: SME shifts had no tiers before, and nothing already rostered stops
// being paged.
func (r TeamScheduleResolver) resolveSME(ctx context.Context, level Level, rc RoutingContext) ([]Recipient, error) {
	tier, ok := sreTier[level]
	if !ok {
		return nil, nil
	}
	team := r.keyOf(rc.SMETeam)
	if team == "" {
		return nil, nil
	}
	cat, err := r.entity.ScheduleCatalogue(ctx)
	if err != nil {
		return nil, err
	}
	sme := map[string]string{} // SME window code -> its fixed tier, if any
	for _, s := range cat.Shifts {
		if strings.EqualFold(s.Family, familySME) {
			sme[s.Code] = deref(s.Tier)
		}
	}
	onDuty, err := r.entity.OnDutyAt(ctx, rc.At, rc.CallOnLeave)
	if err != nil {
		return nil, err
	}
	var pool, away []Recipient
	seen := map[string]bool{}
	for _, a := range onDuty {
		fixed, isSME := sme[a.ShiftCode]
		email := strings.ToLower(strings.TrimSpace(a.Engineer.Email))
		if !isSME || email == "" || seen[email] || r.keyOf(a.TeamKey) != team {
			continue
		}
		held := deref(a.Tier)
		if held == "" {
			held = fixed
		}
		if held == "" {
			held = "L1"
		}
		if !strings.EqualFold(held, tier) {
			continue
		}
		seen[email] = true
		p := Recipient{Email: a.Engineer.Email, Name: a.Engineer.Name, UserID: a.Engineer.UserID, ShiftCode: a.ShiftCode}
		if a.OnLeave {
			away = append(away, p)
			continue
		}
		pool = append(pool, p)
	}
	if len(pool) == 0 {
		// onLeave: call -- nobody available holds the tier, so ring whoever
		// does although they are away.
		pool = away
	}
	sort.SliceStable(pool, func(i, j int) bool { return pool[i].Email < pool[j].Email })
	return r.takeLongestSinceCalled(ctx, pool, 1), nil
}
