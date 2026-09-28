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

// This is an integration test: it asserts a schema FACT (relrowsecurity /
// relforcerowsecurity on pg_class) that can only be observed against a real
// Postgres, not reasoned about from Go code. It exists specifically as the
// regression guard the RLS migration series' own plan called for: nothing
// else in this codebase would notice if a future migration accidentally
// re-disabled RLS on one of these tables, or added a new caller-scoped
// table without FORCE, or a schema-owning role change (e.g. a future
// staging/production role swap) silently made FORCE moot. Same DSN and
// skip-when-unset pattern as every other integration test in this package
// (reuses caseStatsPool):
//
//	CASE_STATS_TEST_DSN=postgres://... go test ./internal/repository/ -run RLSSchemaIntegration

package repository_test

import (
	"context"
	"testing"
)

// rlsProtectedTables is every table this migration series (000085,
// 000093-000101) put under FORCE ROW LEVEL SECURITY, gathered directly from
// the migrations/ directory rather than hand-maintained from memory --
// see this file's own test for how it's cross-checked against that
// directory. Keep this list and the migrations in sync: a table added here
// with no matching migration, or a migration adding FORCE without a
// matching entry here, is exactly the drift this test exists to catch.
var rlsProtectedTables = []string{
	"announcement",
	"case_escalation",
	"case_escalation_notification_list",
	"sla",
	"customer_call",
	"time_card",
	"time_card_approver",
	"change_request",
	"approval_stage",
	"approval_stage_approver",
	"conversation",
	"work_item",
	"case",
	"comment",
	"comment_edit_history",
	"case_attachment",
	"work_item_tag",
	"work_item_watcher",
	"work_item_activity",
	"incident",
	"incident_task",
	"problem",
}

// TestRLSSchemaIntegration_EveryProtectedTableHasForceRowLevelSecurity is the
// guardrail the RLS migration series' own plan called for: a schema test
// asserting relrowsecurity/relforcerowsecurity are both true for every
// table this series protects. Checked against pg_class directly (not
// pg_tables, which only exposes relrowsecurity, not relforcerowsecurity) so
// a table whose RLS is enabled but not forced -- e.g. FORCE quietly dropped
// by a later migration, or never applied because the connecting role turned
// out to be a superuser/table owner in some environment -- is caught, not
// just outright disabled RLS.
func TestRLSSchemaIntegration_EveryProtectedTableHasForceRowLevelSecurity(t *testing.T) {
	pool := caseStatsPool(t)
	ctx := context.Background()

	rows, err := pool.Query(ctx, `
		SELECT c.relname, c.relrowsecurity, c.relforcerowsecurity
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relname = ANY($1) AND n.nspname = current_schema()`,
		rlsProtectedTables,
	)
	if err != nil {
		t.Fatalf("query pg_class: %v", err)
	}
	defer rows.Close()

	found := make(map[string]struct{ enabled, forced bool }, len(rlsProtectedTables))
	for rows.Next() {
		var name string
		var enabled, forced bool
		if err := rows.Scan(&name, &enabled, &forced); err != nil {
			t.Fatalf("scan pg_class row: %v", err)
		}
		found[name] = struct{ enabled, forced bool }{enabled, forced}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate pg_class rows: %v", err)
	}

	for _, table := range rlsProtectedTables {
		state, ok := found[table]
		if !ok {
			t.Errorf("table %q not found in current_schema() -- renamed, dropped, or this test's schema assumption (current_schema() matching the test DSN's own search_path) no longer holds", table)
			continue
		}
		if !state.enabled {
			t.Errorf("table %q: relrowsecurity = false, want true (ROW LEVEL SECURITY not enabled)", table)
		}
		if !state.forced {
			t.Errorf("table %q: relforcerowsecurity = false, want true (FORCE ROW LEVEL SECURITY not set -- a non-superuser table owner would bypass its own policies)", table)
		}
	}
}
