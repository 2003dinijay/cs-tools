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

// Regression test for a real, confirmed production bug: caseFieldPredicates'
// State branch once reused a single placeholder across four different enum
// casts (case_state_enum[]/engagement_state_enum[]/
// service_request_state_enum[]/security_report_analysis_state_enum[]), on
// the mistaken assumption that each `::type` cast applies independently at
// its own occurrence. Postgres's prepared-statement parameter typing
// resolves a placeholder's type ONCE per index, not per occurrence, so the
// second cast onward became an implicit-cast request between two unrelated
// enum types with no cast path between them -- this failed in production
// with "cannot cast type case_state_enum[] to engagement_state_enum[]"
// (SQLSTATE 42846) at PREPARE time, something no unit test checking only the
// generated SQL *string* could ever catch (the string looked completely
// valid; the bug is in Postgres's own parameter-type unification, not SQL
// syntax). TestCaseFieldPredicatesStateExecutes actually PREPAREs and
// EXECUTEs the real caseFieldPredicates output via pgx's extended protocol
// against a live Postgres -- the exact code path and failure mode the
// production error came from; reverting the fix and re-running this test
// reproduces that exact error message.
//
// TestCaseFieldPredicatesStateEquivalence goes further: it confirms the
// rewritten, sargable predicate returns EXACTLY the same matching work_item
// ids as the original COALESCE(c.state, eng.state, sr.state, sra.state,
// ann.state) = ANY(...) predicate (caseLikeStateColumn, case_repo.go) --
// not just that both execute without error. The COALESCE form has no
// placeholder-type-unification bug (it casts to plain ::text[] once), so
// it's trustworthy as the baseline; this guards against the scarier failure
// mode a plain "does it error" check can't catch -- a rewrite that runs
// cleanly but silently returns the wrong rows. Exercises every single
// domain.CaseState value, every pairwise combination, and the full set.
//
// Both seed their own fixture (fixed, cleaned-up rows covering every state
// across all five case-like types, plus two real-world edge cases this
// repo's own CLAUDE.md documents as occurring in production: a work_item row
// with no matching extension row at all, and a "case" row whose own state
// column is NULL) via Scoped+WithSystemIdentity, the same pattern every
// other RLS-protected-table integration test in this package uses (see
// case_like_extension_rls_integration_test.go's seedCaseLikeExtensionFixture)
// -- so this runs correctly whether or not the target database enforces RLS
// on these tables, and leaves no residue behind.
//
// Skipped without CASE_FIELD_PREDICATES_TEST_DSN.
//
//	CASE_FIELD_PREDICATES_TEST_DSN=postgres://... go test ./internal/repository/ -run CaseFieldPredicatesState

package repository

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// caseFieldPredicatesTestJoins mirrors the relevant slice of caseSearchJoins
// (case_repo.go) -- just the five case-like extension tables the State
// branch's predicate actually references, enough to exercise the real
// PREPARE/EXECUTE path without needing the full join graph (project/
// account/deployment/...) SearchCases also carries.
const caseFieldPredicatesTestJoins = `LEFT JOIN "case" c ON c.id = wi.id
	LEFT JOIN engagement eng ON eng.id = wi.id
	LEFT JOIN service_request sr ON sr.id = wi.id
	LEFT JOIN security_report_analysis sra ON sra.id = wi.id
	LEFT JOIN announcement ann ON ann.id = wi.id`

func caseFieldPredicatesTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("CASE_FIELD_PREDICATES_TEST_DSN")
	if dsn == "" {
		t.Skip("CASE_FIELD_PREDICATES_TEST_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// allCaseLikeStates is every domain.CaseState value -- the full vocabulary
// case_state_enum/engagement_state_enum/service_request_state_enum/
// security_report_analysis_state_enum share (see case_field_predicates.go's
// own doc comment on the State branch for why that sharing is safe to rely
// on here).
var allCaseLikeStates = []domain.CaseState{
	domain.CaseStateOpen, domain.CaseStateWorkInProgress, domain.CaseStateWaitingOnWSO2,
	domain.CaseStateAwaitingInfo, domain.CaseStateReopened, domain.CaseStateSolutionProposed,
	domain.CaseStateClosed,
}

// seedCaseFieldPredicatesFixture inserts one work_item (+ matching extension
// row) per state, per case-like type -- all 7 states for case/engagement/
// service_request/security_report_analysis, both states (mapped to
// announcement's own OPEN/CLOSE vocabulary) for announcement -- plus two
// edge-case rows real production data has shown up with (see this file's own
// doc comment). Returns every inserted work_item id, and registers a
// t.Cleanup that deletes them all (the shared-PK extension tables cascade on
// work_item's own delete, same as every other case-like FK in this schema).
func seedCaseFieldPredicatesFixture(t *testing.T, pool *pgxpool.Pool) (ids []string) {
	t.Helper()
	ctx := WithSystemIdentity(context.Background())
	scoped := NewScoped(pool)

	insertWorkItem := func(wiType string) string {
		t.Helper()
		var id string
		err := scoped.QueryRow(ctx, `
			INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type)
			VALUES (gen_random_uuid(), now(), now(), 'cfp-test', 'cfp-test',
			        'CFP-'||gen_random_uuid()::text, 'CFP-WSO2-'||gen_random_uuid()::text,
			        'case field predicates fixture', $1::work_item_type_enum)
			RETURNING id::text`, wiType).Scan(&id)
		if err != nil {
			t.Fatalf("seed work_item(%s): %v", wiType, err)
		}
		ids = append(ids, id)
		return id
	}
	mustExec := func(sql string, args ...any) {
		t.Helper()
		if _, err := scoped.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed (%.80s): %v", sql, err)
		}
	}

	for _, s := range allCaseLikeStates {
		id := insertWorkItem("CASE")
		mustExec(`INSERT INTO "case" (id, state) VALUES ($1, $2::case_state_enum)`, id, strings.ToUpper(string(s)))

		id = insertWorkItem("ENGAGEMENT")
		mustExec(`INSERT INTO engagement (id, state) VALUES ($1, $2::engagement_state_enum)`, id, strings.ToUpper(string(s)))

		id = insertWorkItem("SERVICE_REQUEST")
		mustExec(`INSERT INTO service_request (id, state) VALUES ($1, $2::service_request_state_enum)`, id, strings.ToUpper(string(s)))

		id = insertWorkItem("SECURITY_REPORT_ANALYSIS")
		mustExec(`INSERT INTO security_report_analysis (id, state) VALUES ($1, $2::security_report_analysis_state_enum)`, id, strings.ToUpper(string(s)))
	}
	for _, s := range []string{"OPEN", "CLOSE"} {
		id := insertWorkItem("ANNOUNCEMENT")
		mustExec(`INSERT INTO announcement (id, state) VALUES ($1, $2::announcement_state_enum)`, id, s)
	}

	// Edge case: a work_item with no matching extension row at all.
	insertWorkItem("CASE")
	// Edge case: a "case" row whose own state column is NULL.
	nullID := insertWorkItem("CASE")
	mustExec(`INSERT INTO "case" (id, state) VALUES ($1, NULL)`, nullID)

	t.Cleanup(func() {
		for _, id := range ids {
			_, _ = scoped.Exec(ctx, `DELETE FROM work_item WHERE id = $1`, id)
		}
	})
	return ids
}

func TestCaseFieldPredicatesStateExecutes(t *testing.T) {
	pool := caseFieldPredicatesTestPool(t)
	seedCaseFieldPredicatesFixture(t, pool)
	ctx := WithSystemIdentity(context.Background())
	scoped := NewScoped(pool)

	for _, tc := range []struct {
		name   string
		states []domain.CaseState
	}{
		{"single state", []domain.CaseState{domain.CaseStateOpen}},
		{"state announcement maps (CLOSED -> CLOSE)", []domain.CaseState{domain.CaseStateClosed}},
		{"multiple states across every case-like table", allCaseLikeStates},
		{"state announcement has no equivalent for", []domain.CaseState{domain.CaseStateAwaitingInfo}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			preds, args, _, err := caseFieldPredicates(caseFieldSet{States: tc.states}, 1)
			if err != nil {
				t.Fatalf("caseFieldPredicates: %v", err)
			}
			query := fmt.Sprintf(`SELECT COUNT(*) FROM work_item wi %s WHERE %s`,
				caseFieldPredicatesTestJoins, preds[0])
			var count int
			if err := scoped.QueryRow(ctx, query, args...).Scan(&count); err != nil {
				t.Fatalf("PREPARE/EXECUTE failed -- this is the regression if it reoccurs: %v\nquery: %s", err, query)
			}
		})
	}
}

func TestCaseFieldPredicatesStateEquivalence(t *testing.T) {
	pool := caseFieldPredicatesTestPool(t)
	seedCaseFieldPredicatesFixture(t, pool)
	ctx := WithSystemIdentity(context.Background())
	scoped := NewScoped(pool)

	fetchIDs := func(query string, args ...any) []string {
		t.Helper()
		rows, err := scoped.Query(ctx, query, args...)
		if err != nil {
			t.Fatalf("query: %v\n%s", err, query)
		}
		defer rows.Close()
		var ids []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				t.Fatal(err)
			}
			ids = append(ids, id)
		}
		sort.Strings(ids)
		return ids
	}

	var combos [][]domain.CaseState
	for _, s := range allCaseLikeStates {
		combos = append(combos, []domain.CaseState{s})
	}
	for i := 0; i < len(allCaseLikeStates); i++ {
		for j := i + 1; j < len(allCaseLikeStates); j++ {
			combos = append(combos, []domain.CaseState{allCaseLikeStates[i], allCaseLikeStates[j]})
		}
	}
	combos = append(combos, allCaseLikeStates)

	for _, states := range combos {
		names := make([]string, len(states))
		for i, s := range states {
			names[i] = string(s)
		}
		t.Run(strings.Join(names, "+"), func(t *testing.T) {
			upperStates := make([]string, len(states))
			for i, s := range states {
				upperStates[i] = strings.ToUpper(string(s))
			}
			oldQuery := fmt.Sprintf(`SELECT wi.id::text FROM work_item wi %s WHERE %s = ANY($1::text[]) ORDER BY wi.id`,
				caseFieldPredicatesTestJoins, caseLikeStateColumn)
			oldIDs := fetchIDs(oldQuery, upperStates)

			preds, args, _, err := caseFieldPredicates(caseFieldSet{States: states}, 1)
			if err != nil {
				t.Fatal(err)
			}
			newQuery := fmt.Sprintf(`SELECT wi.id::text FROM work_item wi %s WHERE %s ORDER BY wi.id`,
				caseFieldPredicatesTestJoins, preds[0])
			newIDs := fetchIDs(newQuery, args...)

			if strings.Join(oldIDs, ",") != strings.Join(newIDs, ",") {
				t.Errorf("result mismatch for states=%v\n  old (COALESCE) = %v\n  new (sargable)  = %v", states, oldIDs, newIDs)
			}
		})
	}
}
