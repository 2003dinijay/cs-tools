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
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestIncidentAssignableGroupSQL_ParamPosition(t *testing.T) {
	if got := incidentAssignableGroupSQL(3); !strings.Contains(got, "ANY($3::uuid[])") {
		t.Errorf("SQL does not bind the allowlist at $3: %s", got)
	}
	if got := extraGroupIDsArg(nil); got == nil || len(got) != 0 {
		t.Errorf("extraGroupIDsArg(nil) = %#v, want an empty non-nil slice (nil would bind NULL)", got)
	}
}

// TestIncidentExtraAssignmentGroupsLive checks the allowlist against a real
// database (skipped unless ENTITY_TEST_DATABASE_URL is set; point it at a test
// database). It seeds a service-supported group, an allowlisted group that
// supports no service, an inactive allowlisted group, an allowlisted id with no
// row, and a non-allowlisted group, then checks create, picker and case-alert
// route options against each. Rows are deleted afterwards.
func TestIncidentExtraAssignmentGroupsLive(t *testing.T) {
	dsn := os.Getenv("ENTITY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ENTITY_TEST_DATABASE_URL not set; this test needs a real database")
	}
	ctx := WithSystemIdentity(context.Background())
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	const (
		supported     = "5e5e5e5e-0000-4000-8000-0000000000a1"
		extra         = "5e5e5e5e-0000-4000-8000-0000000000a2"
		extraInactive = "5e5e5e5e-0000-4000-8000-0000000000a3"
		extraNoRow    = "5e5e5e5e-0000-4000-8000-0000000000a4"
		other         = "5e5e5e5e-0000-4000-8000-0000000000a5"
		service       = "5e5e5e5e-0000-4000-8000-0000000000a6"
		prefix        = "extra-group live test"
	)
	defer func() {
		bg := context.Background()
		if _, err := pool.Exec(bg, `DELETE FROM service WHERE id = $1::uuid`, service); err != nil {
			t.Errorf("CLEANUP FAILED, delete service %s by hand: %v", service, err)
		}
		if _, err := pool.Exec(bg, `DELETE FROM "group" WHERE id = ANY($1::uuid[])`, []string{supported, extra, extraInactive, other}); err != nil {
			t.Errorf("CLEANUP FAILED, delete the %s groups by hand: %v", prefix, err)
		}
	}()
	if _, err := pool.Exec(ctx, `
INSERT INTO "group" (id, created_on, updated_on, created_by, updated_by, name, is_active)
VALUES ($1::uuid, now(), now(), 'test', 'test', $5 || ' supported', TRUE),
       ($2::uuid, now(), now(), 'test', 'test', $5 || ' extra', NULL),
       ($3::uuid, now(), now(), 'test', 'test', $5 || ' extra inactive', FALSE),
       ($4::uuid, now(), now(), 'test', 'test', $5 || ' other', TRUE)`,
		supported, extra, extraInactive, other, prefix); err != nil {
		t.Fatalf("seed groups: %v", err)
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO service (id, created_on, updated_on, created_by, updated_by, name, number, support_group_id)
VALUES ($1::uuid, now(), now(), 'test', 'test', $3, 'SVC-TEST-X1', $2::uuid)`, service, supported, prefix+" service"); err != nil {
		t.Fatalf("seed service: %v", err)
	}

	allow := []string{extra, extraInactive, extraNoRow}
	withList := NewIncidentRepository(NewScoped(pool), allow...)
	without := NewIncidentRepository(NewScoped(pool))

	for _, c := range []struct {
		name, id      string
		withList, off bool
	}{
		{"service support group", supported, true, true},
		{"allowlisted group with no service", extra, true, false},
		{"allowlisted but inactive", extraInactive, false, false},
		{"allowlisted id with no row", extraNoRow, false, false},
		{"not allowlisted, no service", other, false, false},
	} {
		if got, err := withList.IsSupportGroup(ctx, c.id); err != nil || got != c.withList {
			t.Errorf("IsSupportGroup with allowlist, %s: got %v err %v, want %v", c.name, got, err, c.withList)
		}
		if got, err := without.IsSupportGroup(ctx, c.id); err != nil || got != c.off {
			t.Errorf("IsSupportGroup without allowlist, %s: got %v err %v, want %v", c.name, got, err, c.off)
		}
	}

	ids := func(repo GroupRepository, q string) map[string]bool {
		t.Helper()
		groups, total, err := repo.SearchSupportGroups(ctx, q, 50, 0)
		if err != nil {
			t.Fatalf("SearchSupportGroups(%q): %v", q, err)
		}
		if total != len(groups) {
			t.Errorf("total %d != %d rows", total, len(groups))
		}
		out := map[string]bool{}
		for _, g := range groups {
			out[g.ID] = true
		}
		return out
	}
	// The picker lists exactly what the create accepts, with and without a
	// search term and with and without the allowlist.
	for _, q := range []string{prefix, ""} {
		got := ids(NewGroupRepository(pool, allow...), q)
		if !got[supported] || !got[extra] || got[extraInactive] || got[extraNoRow] || got[other] {
			t.Errorf("picker with allowlist, q=%q: got %v", q, got)
		}
	}
	got := ids(NewGroupRepository(pool), prefix)
	if !got[supported] || got[extra] || got[extraInactive] || got[other] {
		t.Errorf("picker without allowlist: got %v", got)
	}

	// Case-alert route options keep the plain support-group set: an allowlisted
	// group never appears there, and is not found by id.
	alerts := NewCaseAlertSettingsRepository(pool)
	rows, err := alerts.AssignmentGroups(ctx)
	if err != nil {
		t.Fatalf("AssignmentGroups: %v", err)
	}
	sawSupported := false
	for _, r := range rows {
		if r.ID == extra || r.ID == extraInactive {
			t.Errorf("case-alert assignment groups include allowlisted group %s", r.ID)
		}
		sawSupported = sawSupported || r.ID == supported
	}
	if !sawSupported {
		t.Errorf("case-alert assignment groups lost the service support group %s", supported)
	}
	if _, ok, err := alerts.AssignmentGroupByID(ctx, extra); err != nil || ok {
		t.Errorf("AssignmentGroupByID(allowlisted) = ok %v err %v, want not found", ok, err)
	}
	if _, ok, err := alerts.AssignmentGroupByID(ctx, supported); err != nil || !ok {
		t.Errorf("AssignmentGroupByID(supported) = ok %v err %v, want found", ok, err)
	}
}
