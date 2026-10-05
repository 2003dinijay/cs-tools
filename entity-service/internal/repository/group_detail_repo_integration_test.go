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

package repository

import (
	"context"
	"errors"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// GetGroupDetail against a real Postgres. Skipped without
// CHANGE_REQUEST_TEST_DSN, the convention the change-request integration tests
// use (this feature belongs to that flow: the group opened is an approval
// stage's assignment group).
//
//	CHANGE_REQUEST_TEST_DSN=postgres://... go test ./internal/repository/ -run GroupDetailIntegration
//
// This file is in package repository (not repository_test) so it can hold the
// list against namedGroup / groupMemberIDs, the functions the approval pools
// provision from.

// Fixture ids: one prefix, so cleanup is a single sweep and nothing collides
// with the compose seed or the other integration tests.
const (
	gdPrefix = "7a7a7a7a-0000-4000-8000-"

	gdGroupA     = gdPrefix + "00000000a001" // "GV Approvers": manager, description, email
	gdGroupADup  = gdPrefix + "00000000a002" // a second mirror row with the SAME name
	gdGroupOther = gdPrefix + "00000000b001" // different name
	gdGroupEmpty = gdPrefix + "00000000c001" // no members, no manager, no description
	gdGroupNoNam = gdPrefix + "00000000d001" // name NULL
	gdUnknown    = gdPrefix + "00000000ffff" // no such group

	gdTeamSameName = gdPrefix + "00000000e001" // team named like gdGroupA
	gdTeamUnrel    = gdPrefix + "00000000e002" // team no group shares a name with

	gdGroupAName = "GV Approvers"
)

type gdUser struct {
	id       string
	userName string
	name     *string
	first    *string
	last     *string
	email    *string
	active   *bool
}

func gdS(v string) *string { return &v }
func gdB(v bool) *bool     { return &v }

var gdUsers = []gdUser{
	{id: gdPrefix + "000000000a01", userName: "gv-zoe", name: gdS("Zoe Zed"), email: gdS("zoe@example.test"), active: gdB(true)},
	{id: gdPrefix + "000000000a02", userName: "gv-bob", name: gdS("bob baker"), email: gdS("bob@example.test"), active: gdB(true)}, // lower case sorts by name, not by case
	{id: gdPrefix + "000000000a03", userName: "gv-ian", name: gdS("Ian Inactive"), email: gdS("ian@example.test"), active: gdB(false)},
	{id: gdPrefix + "000000000a04", userName: "gv-fay", first: gdS("Fay"), last: gdS("Fallback"), email: gdS("fay@example.test"), active: gdB(true)}, // no name: first + last
	{id: gdPrefix + "000000000a05", userName: "gv-nia", name: gdS("Nia Null"), email: gdS("nia@example.test")},                                       // is_active NULL counts as active
	{id: gdPrefix + "000000000a06", userName: "gv-tom", name: gdS("Tom Team"), email: gdS("tom@example.test"), active: gdB(true)},                    // member only through the same-named team
	{id: gdPrefix + "000000000a07", userName: "gv-olga", name: gdS("Olga Other"), email: gdS("olga@example.test"), active: gdB(true)},                // other group only
	{id: gdPrefix + "000000000a08", userName: "gv-dan", name: gdS("Dan Duplicate"), email: gdS("dan@example.test"), active: gdB(true)},               // member of the same-named duplicate group
	{id: gdPrefix + "000000000a09", userName: "gv-mia", name: gdS("Mia Manager"), email: gdS("mia@example.test"), active: gdB(true)},                 // the manager, not a member
	{id: gdPrefix + "000000000a0a", userName: "gv-noname", email: gdS("noname@example.test"), active: gdB(true)},                                     // nothing but an email
}

func gdUserID(userName string) string {
	for _, u := range gdUsers {
		if u.userName == userName {
			return u.id
		}
	}
	panic("unknown fixture user " + userName)
}

// gdMembership is one team_member row.
type gdMembership struct {
	id     string
	teamID string
	userID string
	group  *string
	role   string
}

func gdMemberships() []gdMembership {
	grp := func(id string) *string { return &id }
	n := 0
	m := func(teamID, user string, group *string, role string) gdMembership {
		n++
		return gdMembership{id: gdPrefix + "0000000f" + string(rune('0'+n/10)) + string(rune('0'+n%10)) + "00", teamID: teamID, userID: gdUserID(user), group: group, role: role}
	}
	return []gdMembership{
		// by group_id
		m(gdTeamUnrel, "gv-zoe", grp(gdGroupA), "member"),
		m(gdTeamUnrel, "gv-bob", grp(gdGroupA), "member"),
		m(gdTeamUnrel, "gv-ian", grp(gdGroupA), "member"), // inactive
		m(gdTeamUnrel, "gv-fay", grp(gdGroupA), "member"),
		m(gdTeamUnrel, "gv-nia", grp(gdGroupA), "member"),
		// a user with two rows: by group_id and through the same-named team, one of them lead
		m(gdTeamUnrel, "gv-bob", grp(gdGroupA), "member"),
		m(gdTeamSameName, "gv-zoe", nil, "lead"),
		// by team_id of a team named like the group, no group_id at all
		m(gdTeamSameName, "gv-tom", nil, "member"),
		// by group_id of a second group with the same name
		m(gdTeamUnrel, "gv-dan", grp(gdGroupADup), "member"),
		// a different group
		m(gdTeamUnrel, "gv-olga", grp(gdGroupOther), "member"),
		// a user with only an email
		m(gdTeamUnrel, "gv-noname", grp(gdGroupA), "member"),
	}
}

func gdSetup(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("CHANGE_REQUEST_TEST_DSN")
	if dsn == "" {
		t.Skip("CHANGE_REQUEST_TEST_DSN not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	clean := func(fail func(format string, args ...any)) {
		// Members first, then groups (a group's manager is a user), then teams
		// and users.
		for _, sql := range []string{
			`DELETE FROM team_member WHERE id::text LIKE '` + gdPrefix + `%' OR user_id::text LIKE '` + gdPrefix + `%'`,
			`DELETE FROM "group" WHERE id::text LIKE '` + gdPrefix + `%'`,
			`DELETE FROM team WHERE id::text LIKE '` + gdPrefix + `%'`,
			`DELETE FROM "user" WHERE id::text LIKE '` + gdPrefix + `%'`,
		} {
			if _, err := pool.Exec(context.Background(), sql); err != nil {
				fail("cleanup (%.50s): %v", sql, err)
			}
		}
	}
	clean(t.Fatalf)
	t.Cleanup(func() { clean(t.Errorf) })

	mustExec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed (%.60s): %v", sql, err)
		}
	}
	for _, u := range gdUsers {
		mustExec(`INSERT INTO "user" (id, created_on, updated_on, created_by, updated_by, user_name, name, first_name, last_name, email, is_active, user_type)
		          VALUES ($1, now(), now(), 'gv-test', 'gv-test', $2, $3, $4, $5, $6, $7, 'INTERNAL'::user_type_enum)`,
			u.id, u.userName, u.name, u.first, u.last, u.email, u.active)
	}
	mustExec(`INSERT INTO "group" (id, created_on, updated_on, created_by, updated_by, name, description, group_email, manager_id, is_active) VALUES
	          ($1, now(), now(), 'gv-test', 'gv-test', $2, 'Approves the GV changes', 'gv-approvers@example.test', $3, true),
	          ($4, now(), now(), 'gv-test', 'gv-test', $2, NULL, NULL, NULL, true),
	          ($5, now(), now(), 'gv-test', 'gv-test', 'GV Other Group', NULL, NULL, NULL, true),
	          ($6, now(), now(), 'gv-test', 'gv-test', 'GV Empty Group', NULL, NULL, NULL, true),
	          ($7, now(), now(), 'gv-test', 'gv-test', NULL, NULL, NULL, NULL, true)`,
		gdGroupA, gdGroupAName, gdUserID("gv-mia"), gdGroupADup, gdGroupOther, gdGroupEmpty, gdGroupNoNam)
	mustExec(`INSERT INTO team (id, created_on, updated_on, created_by, updated_by, name, type, key) VALUES
	          ($1, now(), now(), 'gv-test', 'gv-test', $2, 'ABT', 'gv-team-same-name'),
	          ($3, now(), now(), 'gv-test', 'gv-test', 'GV Unrelated Team', 'ABT', 'gv-team-unrelated')`,
		gdTeamSameName, gdGroupAName, gdTeamUnrel)
	for _, m := range gdMemberships() {
		mustExec(`INSERT INTO team_member (id, created_on, updated_on, created_by, updated_by, team_id, user_id, group_id, role)
		          VALUES ($1, now(), now(), 'gv-test', 'gv-test', $2, $3, $4, $5)`,
			m.id, m.teamID, m.userID, m.group, m.role)
	}
	return pool
}

func gdNames(members []domain.GroupMember) []string {
	out := make([]string, 0, len(members))
	for _, m := range members {
		out = append(out, m.Name)
	}
	return out
}

func gdIDs(members []domain.GroupMember) []string {
	out := make([]string, 0, len(members))
	for _, m := range members {
		out = append(out, strings.ToLower(m.ID))
	}
	sort.Strings(out)
	return out
}

func TestGroupDetailIntegration_ListsActiveMembersInNameOrder(t *testing.T) {
	pool := gdSetup(t)
	got, err := NewGroupDetailRepository(pool).GetGroupDetail(context.Background(), gdGroupA)
	if err != nil {
		t.Fatalf("GetGroupDetail: %v", err)
	}

	// Case-insensitive name order; the inactive user (Ian) is left out; "Fay
	// Fallback" comes from first + last, "noname@example.test" from the email;
	// Nia (is_active NULL) counts as active; Tom is a member through the
	// same-named team and Dan through the same-named duplicate group; Olga
	// (another group) and Mia (only the manager) are not members.
	want := []string{"bob baker", "Dan Duplicate", "Fay Fallback", "Nia Null", "noname@example.test", "Tom Team", "Zoe Zed"}
	if names := gdNames(got.Members); !reflect.DeepEqual(names, want) {
		t.Fatalf("members = %v, want %v", names, want)
	}
	if got.Total != len(want) {
		t.Fatalf("total = %d, want %d", got.Total, len(want))
	}
	for _, m := range got.Members {
		if strings.Contains(m.Name, "Ian") || strings.Contains(m.Name, "Olga") || strings.Contains(m.Name, "Mia") {
			t.Fatalf("%q must not be listed", m.Name)
		}
	}
}

func TestGroupDetailIntegration_ReturnsTheGroupRow(t *testing.T) {
	pool := gdSetup(t)
	got, err := NewGroupDetailRepository(pool).GetGroupDetail(context.Background(), gdGroupA)
	if err != nil {
		t.Fatalf("GetGroupDetail: %v", err)
	}
	if got.ID != gdGroupA || got.Name != gdGroupAName {
		t.Fatalf("id/name = %q/%q, want %q/%q", got.ID, got.Name, gdGroupA, gdGroupAName)
	}
	if got.Description == nil || *got.Description != "Approves the GV changes" {
		t.Fatalf("description = %v", got.Description)
	}
	if got.Email == nil || *got.Email != "gv-approvers@example.test" {
		t.Fatalf("email = %v", got.Email)
	}
	if got.Manager == nil || got.Manager.ID != gdUserID("gv-mia") || got.Manager.Name != "Mia Manager" {
		t.Fatalf("manager = %+v, want Mia Manager", got.Manager)
	}
}

func TestGroupDetailIntegration_MemberFields(t *testing.T) {
	pool := gdSetup(t)
	got, err := NewGroupDetailRepository(pool).GetGroupDetail(context.Background(), gdGroupA)
	if err != nil {
		t.Fatalf("GetGroupDetail: %v", err)
	}
	by := map[string]domain.GroupMember{}
	for _, m := range got.Members {
		by[m.Name] = m
	}

	zoe := by["Zoe Zed"]
	if zoe.ID != gdUserID("gv-zoe") || zoe.Email == nil || *zoe.Email != "zoe@example.test" {
		t.Fatalf("zoe = %+v", zoe)
	}
	// user_type is whatever "user".user_type holds (a trigger may derive it from
	// the user's roles), passed through verbatim.
	var wantType *string
	if err := pool.QueryRow(context.Background(), `SELECT user_type::text FROM "user" WHERE id = $1`, zoe.ID).Scan(&wantType); err != nil {
		t.Fatalf("read user_type: %v", err)
	}
	if (zoe.UserType == nil) != (wantType == nil) || (wantType != nil && *zoe.UserType != *wantType) {
		t.Fatalf("zoe userType = %v, want %v", zoe.UserType, wantType)
	}
	// Zoe has a plain group_id row AND a lead row through the same-named team
	// -- one member, and the lead row wins.
	if zoe.Role == nil || *zoe.Role != "lead" {
		t.Fatalf("zoe role = %v, want lead", zoe.Role)
	}
	// Bob has two plain rows: listed once, a plain member.
	n := 0
	for _, m := range got.Members {
		if m.ID == gdUserID("gv-bob") {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("bob listed %d times, want exactly once", n)
	}
	if bob := by["bob baker"]; bob.Role == nil || *bob.Role != "member" {
		t.Fatalf("bob role = %v, want member", bob.Role)
	}
}

func TestGroupDetailIntegration_GroupWithoutMembersIsEmptyNotAnError(t *testing.T) {
	pool := gdSetup(t)
	got, err := NewGroupDetailRepository(pool).GetGroupDetail(context.Background(), gdGroupEmpty)
	if err != nil {
		t.Fatalf("GetGroupDetail: %v", err)
	}
	if got.Name != "GV Empty Group" || got.Total != 0 {
		t.Fatalf("got %+v", got)
	}
	if got.Members == nil || len(got.Members) != 0 {
		t.Fatalf("members = %#v, want an empty non-nil slice (it is serialised as [])", got.Members)
	}
	if got.Manager != nil || got.Description != nil || got.Email != nil {
		t.Fatalf("a bare group must carry no manager/description/email, got %+v", got)
	}
}

func TestGroupDetailIntegration_UnknownGroupIsNotFound(t *testing.T) {
	pool := gdSetup(t)
	_, err := NewGroupDetailRepository(pool).GetGroupDetail(context.Background(), gdUnknown)
	var nf *apierror.NotFoundError
	if !errors.As(err, &nf) {
		t.Fatalf("err = %v, want a NotFoundError", err)
	}
}

// A group row with no name has nothing to resolve by name, so only its own
// group_id rows count -- and nothing is matched against a NULL name.
func TestGroupDetailIntegration_GroupWithoutANameHasOnlyItsOwnMembers(t *testing.T) {
	pool := gdSetup(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `INSERT INTO team_member (id, created_on, updated_on, created_by, updated_by, team_id, user_id, group_id)
	        VALUES ($1, now(), now(), 'gv-test', 'gv-test', $2, $3, $4)`,
		gdPrefix+"0000000f0990", gdTeamUnrel, gdUserID("gv-zoe"), gdGroupNoNam); err != nil {
		t.Fatalf("seed: %v", err)
	}
	got, err := NewGroupDetailRepository(pool).GetGroupDetail(ctx, gdGroupNoNam)
	if err != nil {
		t.Fatalf("GetGroupDetail: %v", err)
	}
	if got.Name != "" {
		t.Fatalf("name = %q, want empty", got.Name)
	}
	if names := gdNames(got.Members); !reflect.DeepEqual(names, []string{"Zoe Zed"}) {
		t.Fatalf("members = %v, want [Zoe Zed]", names)
	}
}

// The list a user sees is the list the approval pools provision from: for a
// group resolved by name (CAB / ECAB / Devops) it is exactly namedGroup's member
// set, and for a group addressed by id (the assigned-group pool) it covers
// groupMemberIDs'. The only intended difference is that inactive users are
// not shown.
func TestGroupDetailIntegration_MatchesTheApprovalPools(t *testing.T) {
	pool := gdSetup(t)
	ctx := context.Background()
	got, err := NewGroupDetailRepository(pool).GetGroupDetail(ctx, gdGroupA)
	if err != nil {
		t.Fatalf("GetGroupDetail: %v", err)
	}

	_, poolMembers, exists, err := namedGroup(ctx, pool, gdGroupAName)
	if err != nil || !exists {
		t.Fatalf("namedGroup: exists=%v err=%v", exists, err)
	}
	var activePool []string
	for _, id := range poolMembers {
		var active bool
		if err := pool.QueryRow(ctx, `SELECT COALESCE(is_active, TRUE) FROM "user" WHERE id = $1`, id).Scan(&active); err != nil {
			t.Fatalf("read user %s: %v", id, err)
		}
		if active {
			activePool = append(activePool, strings.ToLower(id))
		}
	}
	sort.Strings(activePool)
	if shown := gdIDs(got.Members); !reflect.DeepEqual(shown, activePool) {
		t.Fatalf("group page members %v != pool members (active) %v", shown, activePool)
	}

	byID, err := groupMemberIDs(ctx, pool, gdGroupA)
	if err != nil {
		t.Fatalf("groupMemberIDs: %v", err)
	}
	shown := map[string]bool{}
	for _, id := range gdIDs(got.Members) {
		shown[id] = true
	}
	for _, id := range byID {
		var active bool
		if err := pool.QueryRow(ctx, `SELECT COALESCE(is_active, TRUE) FROM "user" WHERE id = $1`, id).Scan(&active); err != nil {
			t.Fatalf("read user %s: %v", id, err)
		}
		if active && !shown[strings.ToLower(id)] {
			t.Fatalf("assigned-group pool member %s is missing from the group page", id)
		}
	}
}
