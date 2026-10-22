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

// The rota access SQL against a live PostgreSQL, on the schedule fixtures.
// Skipped unless ENTITY_TEST_DATABASE_URL is set; see
// schedule_repo_integration_test.go for how to build the database.

package repository

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

const schedMarkerActor = "rota.access.admin@example.test"

// auditSince is the database's clock now. The schedule's history is never
// cleared -- that is what it is for -- so a test counts only what it wrote.
func auditSince(t *testing.T, ctx context.Context, pool *pgxpool.Pool) time.Time {
	t.Helper()
	var at time.Time
	if err := pool.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&at); err != nil {
		t.Fatalf("clock: %v", err)
	}
	return at
}

// A marker's grant counts while they are internal staff on the team, and
// stops on its own when they leave it -- no clean-up needed for a leaver.
func TestScheduleIntegration_RotaMarkerCountsWhileOnTheTeam(t *testing.T) {
	schedule, pool := newScheduleIntegrationRepo(t)
	access := NewRotaAccessRepository(pool)
	ctx := context.Background()
	since := auditSince(t, ctx, pool)
	mustExec(t, pool, `
		INSERT INTO user_role (id, created_on, updated_on, created_by, updated_by, user_id, role_id)
		SELECT gen_random_uuid(), NOW(), NOW(), 'fixture', 'fixture', $1, r.id
		  FROM role r WHERE r.name = 'internal'`, schedMemberID)

	added, err := access.GrantRotaMarker(ctx, schedMemberID, schedTeamKey, schedMarkerActor)
	if err != nil || !added {
		t.Fatalf("GrantRotaMarker = %v, %v", added, err)
	}
	if again, err := access.GrantRotaMarker(ctx, schedMemberID, schedTeamKey, schedMarkerActor); err != nil || again {
		t.Fatalf("a second grant = %v, %v; want no change", again, err)
	}
	teams, err := schedule.RotaMarkerTeamsFor(ctx, schedMemberEmail)
	if err != nil || !slices.Equal(teams, []string{schedTeamKey}) {
		t.Fatalf("RotaMarkerTeamsFor = %v, %v", teams, err)
	}
	if n := countRows(t, pool, `SELECT count(*) FROM team_schedule_audit
		WHERE table_name = 'team_schedule_rota_marker' AND action = 'INSERT' AND actor = $1 AND changed_at >= $2`, schedMarkerActor, since); n != 1 {
		t.Fatalf("audit rows for the grant = %d, want 1", n)
	}

	// Leaves the team: the grant stays listed, inactive, and stops counting.
	mustExec(t, pool, `DELETE FROM team_member WHERE user_id = $1`, schedMemberID)
	if teams, _ := schedule.RotaMarkerTeamsFor(ctx, schedMemberEmail); len(teams) != 0 {
		t.Fatalf("a marker who left the team still marks %v", teams)
	}
	markers, err := access.ListRotaMarkers(ctx, []string{"CRE"})
	if err != nil {
		t.Fatalf("ListRotaMarkers: %v", err)
	}
	var found bool
	for _, m := range markers {
		if m.UserID == schedMemberID && m.TeamKey == schedTeamKey {
			found = true
			if m.Active || m.Family != "CRE" || m.GrantedBy != schedMarkerActor {
				t.Fatalf("listed %+v; want inactive, CRE, granted by %s", m, schedMarkerActor)
			}
		}
	}
	if !found {
		t.Fatal("the grant was not listed")
	}
	if sre, _ := access.ListRotaMarkers(ctx, []string{"SRE"}); slices.ContainsFunc(sre, func(m domain.RotaMarkerGrant) bool { return m.UserID == schedMemberID }) {
		t.Fatal("a CRE team's marker was listed under SRE")
	}

	removed, err := access.RevokeRotaMarker(ctx, schedMemberID, schedTeamKey, schedMarkerActor)
	if err != nil || !removed {
		t.Fatalf("RevokeRotaMarker = %v, %v", removed, err)
	}
	if n := countRows(t, pool, `SELECT count(*) FROM team_schedule_audit
		WHERE table_name = 'team_schedule_rota_marker' AND action = 'DELETE' AND actor = $1 AND changed_at >= $2`, schedMarkerActor, since); n != 1 {
		t.Fatalf("audit rows for the removal = %d, want 1", n)
	}
}

// Rota admin grants live in user_role. Granting twice adds one row, removing
// takes every copy, and only rota roles reach the schedule's history.
func TestScheduleIntegration_RotaAdminGrantAndRemoval(t *testing.T) {
	_, pool := newScheduleIntegrationRepo(t)
	access := NewRotaAccessRepository(pool)
	ctx := context.Background()
	since := auditSince(t, ctx, pool)

	for i, want := range []bool{true, false} {
		added, err := access.GrantRotaAdmin(ctx, schedMemberID, "sre_rota_admin", schedMarkerActor)
		if err != nil || added != want {
			t.Fatalf("grant #%d = %v, %v; want %v", i+1, added, err, want)
		}
	}
	admins, err := access.ListRotaAdmins(ctx)
	if err != nil {
		t.Fatalf("ListRotaAdmins: %v", err)
	}
	var listed int
	for _, a := range admins {
		if a.UserID == schedMemberID && a.Family == "SRE" {
			listed++
			if a.Active {
				t.Fatalf("listed %+v as active; the member is not internal staff", a)
			}
		}
	}
	if listed != 1 {
		t.Fatalf("listed the grant %d times, want once", listed)
	}

	removed, err := access.RevokeRotaAdmin(ctx, schedMemberID, "sre_rota_admin", schedMarkerActor)
	if err != nil || !removed {
		t.Fatalf("RevokeRotaAdmin = %v, %v", removed, err)
	}
	if again, _ := access.RevokeRotaAdmin(ctx, schedMemberID, "sre_rota_admin", schedMarkerActor); again {
		t.Fatal("a second removal reported removing something")
	}
	if n := countRows(t, pool, `SELECT count(*) FROM team_schedule_audit
		WHERE table_name = 'user_role' AND actor = $1 AND action IN ('INSERT', 'DELETE') AND changed_at >= $2`, schedMarkerActor, since); n != 2 {
		t.Fatalf("audit rows for the rota admin grant and removal = %d, want 2", n)
	}
	// The fixture's own internal-role grant is not a rota change.
	if n := countRows(t, pool, `SELECT count(*) FROM team_schedule_audit a
		JOIN role r ON r.id = (a.new_row ->> 'role_id')::uuid
		WHERE a.table_name = 'user_role' AND r.name = 'internal' AND a.changed_at >= $1`, since); n != 0 {
		t.Fatalf("%d internal-role grants reached the schedule's history", n)
	}
}

func TestScheduleIntegration_RotaAccessLookups(t *testing.T) {
	_, pool := newScheduleIntegrationRepo(t)
	access := NewRotaAccessRepository(pool)
	ctx := context.Background()

	if fam, ok, err := access.RosteredTeamFamily(ctx, "SCHEDSREFIXTURE"); err != nil || !ok || fam != "SRE" {
		t.Fatalf("RosteredTeamFamily(sre) = %q, %v, %v", fam, ok, err)
	}
	if _, ok, err := access.RosteredTeamFamily(ctx, "no-such-team"); err != nil || ok {
		t.Fatalf("a missing team was found: %v, %v", ok, err)
	}
	if ok, err := access.UserIsInternal(ctx, schedAdminID); err != nil || !ok {
		t.Fatalf("UserIsInternal(admin) = %v, %v", ok, err)
	}
	if ok, err := access.UserIsInternal(ctx, schedOutsiderID); err != nil || ok {
		t.Fatalf("UserIsInternal(outsider) = %v, %v", ok, err)
	}
}

// Adding puts the person on the team once as an engineer, however often it
// is asked; removing is refused, with nothing written, while upcoming turns,
// a Case Alerts tier or (unless allowed) a lead role keep them on it, and
// otherwise takes them off with their rota marker grant for the team.
func TestScheduleIntegration_TeamMemberAddAndRemove(t *testing.T) {
	_, pool := newScheduleIntegrationRepo(t)
	access := NewRotaAccessRepository(pool)
	ctx := context.Background()
	t.Cleanup(func() {
		mustExec(t, pool, `DELETE FROM team_schedule_assignment WHERE created_by = 'member-fixture'`)
		mustExec(t, pool, `DELETE FROM team_member WHERE user_id = $1`, schedOutsiderID)
		mustExec(t, pool, `DELETE FROM team_schedule_rota_marker WHERE user_id = $1`, schedOutsiderID)
	})
	members := func(userID string) int {
		return countRows(t, pool, `SELECT count(*) FROM team_member m JOIN team t ON t.id = m.team_id
			WHERE t.key = $1 AND m.user_id = $2`, schedTeamKey, userID)
	}

	for i, want := range []bool{true, false} {
		added, err := access.AddTeamMember(ctx, schedOutsiderID, schedTeamKey, schedMarkerActor)
		if err != nil || added != want {
			t.Fatalf("add #%d = %v, %v; want %v", i+1, added, err, want)
		}
	}
	if n := members(schedOutsiderID); n != 1 {
		t.Fatalf("memberships after two adds = %d, want 1", n)
	}
	var role, createdBy string
	if err := pool.QueryRow(ctx, `SELECT m.role, m.created_by FROM team_member m JOIN team t ON t.id = m.team_id
		WHERE t.key = $1 AND m.user_id = $2`, schedTeamKey, schedOutsiderID).Scan(&role, &createdBy); err != nil {
		t.Fatalf("read membership: %v", err)
	}
	if role != "engineer" || createdBy != schedMarkerActor {
		t.Fatalf("membership = %s by %s; want engineer by %s", role, createdBy, schedMarkerActor)
	}
	if _, err := access.AddTeamMember(ctx, schedOutsiderID, "no-such-team", schedMarkerActor); err == nil {
		t.Fatal("adding to a team that does not exist succeeded")
	}

	// An upcoming turn keeps them on; a past one does not.
	mustExec(t, pool, `
		INSERT INTO team_schedule_assignment
		  (user_id, team_id, team_key, shift_id, rota_date, starts_at, ends_at, created_by, updated_by)
		SELECT $1::uuid, $2::uuid, $3, s.id, (NOW() + interval '2 days')::date,
		       NOW() + interval '2 days', NOW() + interval '2 days 4 hours', 'member-fixture', 'member-fixture'
		  FROM team_schedule_shift s ORDER BY s.code LIMIT 1`, schedOutsiderID, schedTeamID, schedTeamKey)
	mustExec(t, pool, `
		INSERT INTO team_schedule_assignment
		  (user_id, team_id, team_key, shift_id, rota_date, starts_at, ends_at, created_by, updated_by)
		SELECT $1::uuid, $2::uuid, $3, s.id, (NOW() - interval '9 days')::date,
		       NOW() - interval '9 days', NOW() - interval '9 days' + interval '4 hours', 'member-fixture', 'member-fixture'
		  FROM team_schedule_shift s ORDER BY s.code LIMIT 1`, schedOutsiderID, schedTeamID, schedTeamKey)
	res, err := access.RemoveTeamMember(ctx, schedOutsiderID, schedTeamKey, schedMarkerActor, true)
	if err != nil || res.Removed || !res.Member || res.UpcomingTurns != 1 {
		t.Fatalf("remove with an upcoming turn = %+v, %v; want kept with 1 upcoming turn", res, err)
	}
	if n := members(schedOutsiderID); n != 1 {
		t.Fatalf("a kept member was removed (%d rows)", n)
	}
	mustExec(t, pool, `DELETE FROM team_schedule_assignment WHERE created_by = 'member-fixture' AND ends_at > NOW()`)

	// A responder tier keeps them on.
	mustExec(t, pool, `UPDATE team_member m SET alert_tier = 'T3' FROM team t
		WHERE t.id = m.team_id AND t.key = $1 AND m.user_id = $2`, schedTeamKey, schedOutsiderID)
	if res, err := access.RemoveTeamMember(ctx, schedOutsiderID, schedTeamKey, schedMarkerActor, true); err != nil || res.Removed || res.AlertTier != "T3" {
		t.Fatalf("remove with a tier = %+v, %v; want kept with T3", res, err)
	}
	mustExec(t, pool, `UPDATE team_member m SET alert_tier = NULL FROM team t
		WHERE t.id = m.team_id AND t.key = $1 AND m.user_id = $2`, schedTeamKey, schedOutsiderID)

	// Nothing keeps them: they go, with their marker grant for the team.
	mustExec(t, pool, `INSERT INTO team_schedule_rota_marker (created_by, updated_by, user_id, team_key)
		VALUES ('member-fixture', 'member-fixture', $1, $2)`, schedOutsiderID, schedTeamKey)
	res, err = access.RemoveTeamMember(ctx, schedOutsiderID, schedTeamKey, schedMarkerActor, false)
	if err != nil || !res.Removed {
		t.Fatalf("remove = %+v, %v; want removed", res, err)
	}
	if n := members(schedOutsiderID); n != 0 {
		t.Fatalf("memberships after removal = %d, want 0", n)
	}
	if n := countRows(t, pool, `SELECT count(*) FROM team_schedule_rota_marker WHERE user_id = $1`, schedOutsiderID); n != 0 {
		t.Fatalf("the marker grant outlived the membership (%d rows)", n)
	}
	if n := countRows(t, pool, `SELECT count(*) FROM team_schedule_assignment WHERE user_id = $1`, schedOutsiderID); n != 1 {
		t.Fatalf("a past turn was touched (%d rows left, want 1)", n)
	}
	if res, err := access.RemoveTeamMember(ctx, schedOutsiderID, schedTeamKey, schedMarkerActor, false); err != nil || res.Member || res.Removed {
		t.Fatalf("removing again = %+v, %v; want not a member", res, err)
	}

	// A lead stays unless the caller may remove one.
	res, err = access.RemoveTeamMember(ctx, schedLeadID, schedTeamKey, schedMarkerActor, false)
	if err != nil || res.Removed || res.Role != "lead" {
		t.Fatalf("a rota admin removing the lead = %+v, %v; want kept as lead", res, err)
	}
	if n := members(schedLeadID); n == 0 {
		t.Fatal("the lead was removed")
	}
}
