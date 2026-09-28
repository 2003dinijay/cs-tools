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

package repository_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// Runs against a real Postgres with migration 000085 applied (the RLS
// policy, keyed on announcement_type from migration 000084_announcement_add_type
// and the "Security Announcement" work_item_tag -- see
// announcement_is_security's own doc comment for why both signals) --
// exercises the actual caseRepo.GetCaseByID/SearchCases Go code, not just the
// SQL policy in isolation, since that is the only way to prove
// setCallerIdentity's transaction wiring actually works end to end.
// Skipped without ANNOUNCEMENT_VISIBILITY_TEST_DSN, so an ordinary
// `go test ./...` stays hermetic.
//
//	ANNOUNCEMENT_VISIBILITY_TEST_DSN=postgres://... go test ./internal/repository/ -run AnnouncementVisibility
func announcementVisibilityPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("ANNOUNCEMENT_VISIBILITY_TEST_DSN")
	if dsn == "" {
		t.Skip("ANNOUNCEMENT_VISIBILITY_TEST_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

const (
	avProjectID        = "b0000000-0000-0000-0000-000000000099"
	avAccountID        = "a0000000-0000-0000-0000-000000000099"
	avContactID        = "c0000000-0000-0000-0000-000000000099"
	avGeneralID        = "31111111-1111-1111-1111-111111111111"
	avSecurityID       = "32222222-2222-2222-2222-222222222222"
	avSecurityViaTagID = "33333333-3333-3333-3333-333333333333"
)

// seedAnnouncementVisibilityFixtures creates one project with five contacts
// (one per real project_group -- General Access, Security Only, Full
// Access, Lead User Group, Business Contact Group alone) and three
// announcements (general, security via announcement_type, and security via
// the work_item_tag fallback only). project_role/project_group/
// project_group_role are reference data populated by the external
// ServiceNow sync job, never by this test -- looked up by name rather than
// (re)created, and the test fails with a clear message rather than silently
// no-op'ing if a target database is missing any of them.
func seedAnnouncementVisibilityFixtures(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	// WithSystemIdentity: work_item/announcement/work_item_tag all have RLS
	// now (migrations 000085/0147) -- including an internal-only DELETE
	// policy added specifically because this cleanup needs it (the same
	// sla_delete lesson from migration 0142). scoped, not just pool,
	// backs every write below that touches one of these three tables.
	ctx := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)

	cleanup := func() {
		_, _ = scoped.Exec(ctx, `DELETE FROM work_item WHERE project_id = $1`, avProjectID)
		_, _ = pool.Exec(ctx, `DELETE FROM project_contact WHERE project_id = $1`, avProjectID)
		_, _ = pool.Exec(ctx, `DELETE FROM project WHERE id = $1`, avProjectID)
		_, _ = pool.Exec(ctx, `DELETE FROM account_contact WHERE id = $1`, avContactID)
		_, _ = pool.Exec(ctx, `DELETE FROM account WHERE id = $1`, avAccountID)
	}
	cleanup()
	t.Cleanup(cleanup)

	mustExec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed: %s: %v", sql, err)
		}
	}
	// mustExecScoped is mustExec's counterpart for the three RLS-protected
	// tables (work_item/announcement/work_item_tag) this fixture writes.
	mustExecScoped := func(sql string, args ...any) {
		t.Helper()
		if _, err := scoped.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed: %s: %v", sql, err)
		}
	}
	now := time.Now().UTC()

	mustExec(`INSERT INTO account (id, created_on, updated_on, created_by, updated_by, name, number, sf_id)
		VALUES ($1, $2, $2, 'test', 'test', 'AV Test Account', 'AV-ACC-1', 'AV-SF-ACC-1')`, avAccountID, now)
	mustExec(`INSERT INTO account_contact (id, created_on, updated_on, created_by, updated_by, user_name, account_id)
		VALUES ($1, $2, $2, 'test', 'test', 'AV Test Contact', $3)`, avContactID, now, avAccountID)
	mustExec(`INSERT INTO project (id, created_on, updated_on, created_by, updated_by, key, sf_id, name, account_id)
		VALUES ($1, $2, $2, 'test', 'test', 'AVTESTPROJ', 'AV-SF-PROJ-1', 'AV Test Project', $3)`, avProjectID, now, avAccountID)

	contacts := map[string]string{
		"av-general@test.local":           "General Access",
		"av-secure-only@test.local":       "Security Only",
		"av-full-access@test.local":       "Full Access",
		"av-lead@test.local":              "Lead User Group",
		"av-biz-contact-alone@test.local": "Business Contact  Group",
	}
	for email, group := range contacts {
		var groupID string
		err := pool.QueryRow(ctx, `SELECT id FROM project_group WHERE "group" = $1`, group).Scan(&groupID)
		if err != nil {
			t.Fatalf("reference data missing: project_group %q not found (expected to be pre-seeded by the sync job): %v", group, err)
		}
		var contactRowID string
		// state = 'REGISTERED' matters now, not just historically for
		// announcement's own role-based policy: work_item itself is
		// RLS-protected too (migration 0147), and its is_project_member()
		// check requires state = 'REGISTERED' -- a project_contact row left
		// at its NULL default would pass announcement's own role check but
		// fail work_item's, hiding the row from the join entirely regardless
		// of role.
		err = pool.QueryRow(ctx, `INSERT INTO project_contact (id, created_on, updated_on, created_by, updated_by, email, account_contact_id, project_id, state)
			VALUES (gen_random_uuid(), $1, $1, 'test', 'test', $2, $3, $4, 'REGISTERED') RETURNING id`,
			now, email, avContactID, avProjectID).Scan(&contactRowID)
		if err != nil {
			t.Fatalf("seed project_contact %s: %v", email, err)
		}
		mustExec(`INSERT INTO project_contact_group (id, created_on, updated_on, created_by, updated_by, project_contact_id, project_group_id)
			VALUES (gen_random_uuid(), $1, $1, 'test', 'test', $2, $3)`, now, contactRowID, groupID)
	}

	mustExecScoped(`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type, project_id)
		VALUES ($1, $2, $2, 'test', 'test', 'AV-TEST-GEN-1', 'AV-WSO2-GEN-1', 'AV test general announcement', 'ANNOUNCEMENT', $3)`,
		avGeneralID, now, avProjectID)
	mustExecScoped(`INSERT INTO announcement (id, announcement_type) VALUES ($1, 'GENERAL')`, avGeneralID)

	mustExecScoped(`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type, project_id)
		VALUES ($1, $2, $2, 'test', 'test', 'AV-TEST-SEC-1', 'AV-WSO2-SEC-1', 'AV test security announcement', 'ANNOUNCEMENT', $3)`,
		avSecurityID, now, avProjectID)
	// is_security_announcement, not announcement_type, is what the RLS
	// policy actually reads (confirmed directly against the deployed
	// policy: it has no reference to announcement_type at all) -- this
	// column is maintained by application code outside this schema
	// (combining the announcement_type and "Security Announcement"-tag
	// signals this file's own package doc comment describes), which this
	// fixture simulates by setting it directly rather than reproducing
	// that external computation here. Found live: the very first run of
	// this test against a real database left it at its FALSE default,
	// silently making every "security" scenario indistinguishable from
	// "general" under RLS.
	mustExecScoped(`INSERT INTO announcement (id, announcement_type, is_security_announcement) VALUES ($1, 'SECURITY', true)`, avSecurityID)

	// Mirrors a real finding (checked live against ServiceNow-synced data): a
	// currently-open, CVSS 10.0 security bulletin had announcement_type wrongly
	// GENERAL, with only its "Security Announcement" work_item_tag correct --
	// exercises announcement_is_security's tag-based fallback signal, not just
	// its announcement_type check.
	mustExecScoped(`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type, project_id)
		VALUES ($1, $2, $2, 'test', 'test', 'AV-TEST-SEC-2', 'AV-WSO2-SEC-2', 'AV test security via tag only', 'ANNOUNCEMENT', $3)`,
		avSecurityViaTagID, now, avProjectID)
	// is_security_announcement is still true here despite announcement_type
	// staying (wrongly) GENERAL -- see the comment above this row's own
	// work_item insert: this simulates the tag-based fallback signal having
	// already corrected it, exactly like avSecurityID's own doc comment
	// describes for the straightforward case.
	mustExecScoped(`INSERT INTO announcement (id, announcement_type, is_security_announcement) VALUES ($1, 'GENERAL', true)`, avSecurityViaTagID)

	var tagID string
	err := pool.QueryRow(ctx, `SELECT id FROM tag WHERE LOWER(name) = LOWER('Security Announcement') LIMIT 1`).Scan(&tagID)
	if errors.Is(err, pgx.ErrNoRows) {
		err = pool.QueryRow(ctx, `INSERT INTO tag (id, created_on, updated_on, created_by, updated_by, name)
			VALUES (gen_random_uuid(), $1, $1, 'test', 'test', 'Security Announcement') RETURNING id`, now).Scan(&tagID)
	}
	if err != nil {
		t.Fatalf("find or create Security Announcement tag: %v", err)
	}
	mustExecScoped(`INSERT INTO work_item_tag (id, created_on, updated_on, created_by, updated_by, work_item_id, tag_id)
		VALUES (gen_random_uuid(), $1, $1, 'test', 'test', $2, $3)`, now, avSecurityViaTagID, tagID)
}

// caseTypeOf runs GetCaseByID and reports whether the announcement was
// visible (true) or came back NotFound (false) -- GetCaseByID never reveals
// existence to a caller who can't see a row, so NotFound is precisely the
// expected "not visible" signal here, not a test-infra error.
func announcementVisible(t *testing.T, repo repository.CaseRepository, id string, scope repository.SearchScope) bool {
	t.Helper()
	_, err := repo.GetCaseByID(context.Background(), id, scope)
	if err == nil {
		return true
	}
	var notFound *apierror.NotFoundError
	if errors.As(err, &notFound) {
		return false
	}
	t.Fatalf("GetCaseByID(%s): unexpected error: %v", id, err)
	return false
}

func TestAnnouncementVisibilityIntegration(t *testing.T) {
	pool := announcementVisibilityPool(t)
	seedAnnouncementVisibilityFixtures(t, pool)
	repo := repository.NewCaseRepository(repository.NewScoped(pool))

	scoped := func(email string) repository.SearchScope {
		return repository.SearchScope{ProjectIDs: []string{avProjectID}, ViewerEmail: email}
	}

	cases := []struct {
		name               string
		scope              repository.SearchScope
		wantGeneral        bool
		wantSecurity       bool
		wantSecurityViaTag bool
	}{
		{"General Access", scoped("av-general@test.local"), true, false, false},
		{"Security Only", scoped("av-secure-only@test.local"), false, true, true},
		{"Full Access", scoped("av-full-access@test.local"), true, true, true},
		{"Lead User Group", scoped("av-lead@test.local"), true, false, false},
		{"Business Contact Group alone", scoped("av-biz-contact-alone@test.local"), false, false, false},
		{"Unknown email, fails closed", scoped("nobody@nowhere.local"), false, false, false},
		{"Internal caller sees all three", repository.SearchScope{Unrestricted: true}, true, true, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotGeneral := announcementVisible(t, repo, avGeneralID, tc.scope)
			gotSecurity := announcementVisible(t, repo, avSecurityID, tc.scope)
			gotSecurityViaTag := announcementVisible(t, repo, avSecurityViaTagID, tc.scope)
			if gotGeneral != tc.wantGeneral {
				t.Errorf("general announcement visible = %v, want %v", gotGeneral, tc.wantGeneral)
			}
			if gotSecurity != tc.wantSecurity {
				t.Errorf("security announcement visible = %v, want %v", gotSecurity, tc.wantSecurity)
			}
			if gotSecurityViaTag != tc.wantSecurityViaTag {
				t.Errorf("security-via-tag announcement (announcement_type wrongly GENERAL) visible = %v, want %v", gotSecurityViaTag, tc.wantSecurityViaTag)
			}
		})
	}
}

// TestAnnouncementVisibilitySearchCasesIntegration exercises SearchCases
// specifically (not just GetCaseByID): its errgroup-based concurrent
// COUNT/data queries each open their own transaction, a genuinely different
// code path from GetCaseByID's single transaction.
func TestAnnouncementVisibilitySearchCasesIntegration(t *testing.T) {
	pool := announcementVisibilityPool(t)
	seedAnnouncementVisibilityFixtures(t, pool)
	repo := repository.NewCaseRepository(repository.NewScoped(pool))

	req := domain.SearchCasesRequest{
		Parsed:     domain.ParsedCaseFilters{ProjectIDs: []string{avProjectID}},
		SortBy:     domain.CaseSort{Field: domain.CaseSortFieldCreatedOn, Order: domain.CaseSortOrderAsc},
		Pagination: domain.Pagination{Limit: 10, Offset: 0},
	}

	cases := []struct {
		name      string
		scope     repository.SearchScope
		wantCount int
	}{
		{"General Access sees 1 (general only)", repository.SearchScope{ProjectIDs: []string{avProjectID}, ViewerEmail: "av-general@test.local"}, 1},
		{"Security Only sees 2 (security + tag-only security)", repository.SearchScope{ProjectIDs: []string{avProjectID}, ViewerEmail: "av-secure-only@test.local"}, 2},
		{"Full Access sees 3 (all)", repository.SearchScope{ProjectIDs: []string{avProjectID}, ViewerEmail: "av-full-access@test.local"}, 3},
		{"Business Contact alone sees 0", repository.SearchScope{ProjectIDs: []string{avProjectID}, ViewerEmail: "av-biz-contact-alone@test.local"}, 0},
		{"Internal caller sees 3 (all)", repository.SearchScope{Unrestricted: true}, 3},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			results, total, err := repo.SearchCases(context.Background(), req, tc.scope)
			if err != nil {
				t.Fatalf("SearchCases: %v", err)
			}
			if total != tc.wantCount {
				t.Errorf("total = %d, want %d", total, tc.wantCount)
			}
			if len(results) != tc.wantCount {
				t.Errorf("len(results) = %d, want %d", len(results), tc.wantCount)
			}
		})
	}
}

const (
	avNoSecProjectID      = "b0000000-0000-0000-0000-000000000098"
	avNoSecAccountID      = "a0000000-0000-0000-0000-000000000098"
	avNoSecContactID      = "c0000000-0000-0000-0000-000000000098"
	avNoSecAnnouncementID = "34444444-4444-4444-4444-444444444444"
)

// seedAnnouncementSecurityFallbackFixture creates a SEPARATE project from
// seedAnnouncementVisibilityFixtures' own -- deliberately with only a
// General Access (PORTAL_USER) contact and no Security Only/Full Access
// contact at all -- plus one security announcement in it, to exercise the
// fallback migration 0149 added: a security announcement in a project
// with no security contact is visible to ordinary portal users instead of
// being invisible to everyone but internal callers. The main fixture's own
// project cannot exercise this: it deliberately includes a Security Only
// contact, so project_has_security_contact is always true there.
func seedAnnouncementSecurityFallbackFixture(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)

	cleanup := func() {
		// work_item now has FORCE ROW LEVEL SECURITY (migration 0147) --
		// scoped, not a raw pool.Exec, or this DELETE silently affects zero
		// rows under its internal-only DELETE policy, leaving the row (and
		// its child announcement row) behind for the next run. Matched by
		// id as well as project_id: a row orphaned by a prior buggy
		// cleanup run can have project_id already NULLed out by the
		// project-delete cascade below having run first.
		_, _ = scoped.Exec(ctx, `DELETE FROM work_item WHERE project_id = $1 OR id = $2`, avNoSecProjectID, avNoSecAnnouncementID)
		_, _ = pool.Exec(ctx, `DELETE FROM project_contact WHERE project_id = $1`, avNoSecProjectID)
		_, _ = pool.Exec(ctx, `DELETE FROM project WHERE id = $1`, avNoSecProjectID)
		_, _ = pool.Exec(ctx, `DELETE FROM account_contact WHERE id = $1`, avNoSecContactID)
		_, _ = pool.Exec(ctx, `DELETE FROM account WHERE id = $1`, avNoSecAccountID)
	}
	cleanup()
	t.Cleanup(cleanup)

	mustExec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed: %s: %v", sql, err)
		}
	}
	mustExecScoped := func(sql string, args ...any) {
		t.Helper()
		if _, err := scoped.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed: %s: %v", sql, err)
		}
	}
	now := time.Now().UTC()

	mustExec(`INSERT INTO account (id, created_on, updated_on, created_by, updated_by, name, number, sf_id)
		VALUES ($1, $2, $2, 'test', 'test', 'AV No-Sec Test Account', 'AV-ACC-2', 'AV-SF-ACC-2')`, avNoSecAccountID, now)
	mustExec(`INSERT INTO account_contact (id, created_on, updated_on, created_by, updated_by, user_name, account_id)
		VALUES ($1, $2, $2, 'test', 'test', 'AV No-Sec Test Contact', $3)`, avNoSecContactID, now, avNoSecAccountID)
	mustExec(`INSERT INTO project (id, created_on, updated_on, created_by, updated_by, key, sf_id, name, account_id)
		VALUES ($1, $2, $2, 'test', 'test', 'AVNOSECPROJ', 'AV-SF-PROJ-2', 'AV No-Sec Test Project', $3)`, avNoSecProjectID, now, avNoSecAccountID)

	var groupID string
	if err := pool.QueryRow(ctx, `SELECT id FROM project_group WHERE "group" = 'General Access'`).Scan(&groupID); err != nil {
		t.Fatalf("reference data missing: project_group \"General Access\" not found: %v", err)
	}
	var contactRowID string
	if err := pool.QueryRow(ctx, `INSERT INTO project_contact (id, created_on, updated_on, created_by, updated_by, email, account_contact_id, project_id, state)
		VALUES (gen_random_uuid(), $1, $1, 'test', 'test', 'av-nosec-portal-user@test.local', $2, $3, 'REGISTERED') RETURNING id`,
		now, avNoSecContactID, avNoSecProjectID).Scan(&contactRowID); err != nil {
		t.Fatalf("seed project_contact: %v", err)
	}
	mustExec(`INSERT INTO project_contact_group (id, created_on, updated_on, created_by, updated_by, project_contact_id, project_group_id)
		VALUES (gen_random_uuid(), $1, $1, 'test', 'test', $2, $3)`, now, contactRowID, groupID)

	mustExecScoped(`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type, project_id)
		VALUES ($1, $2, $2, 'test', 'test', 'AV-NOSEC-1', 'AV-WSO2-NOSEC-1', 'AV no-security-contact fallback test', 'ANNOUNCEMENT', $3)`,
		avNoSecAnnouncementID, now, avNoSecProjectID)
	mustExecScoped(`INSERT INTO announcement (id, announcement_type) VALUES ($1, 'SECURITY')`, avNoSecAnnouncementID)
}

// TestAnnouncementSecurityFallbackNoSecurityContactIntegration is the
// regression test for migration 0149's own fallback: a security
// announcement in a project with no SECURITY_CONTACT at all must be
// visible to that project's ordinary portal users, not just internal
// callers -- confirmed live against this database copy (several real
// projects have PORTAL_USER contacts but no security contact) before this
// migration was written.
func TestAnnouncementSecurityFallbackNoSecurityContactIntegration(t *testing.T) {
	pool := announcementVisibilityPool(t)
	seedAnnouncementSecurityFallbackFixture(t, pool)
	repo := repository.NewCaseRepository(repository.NewScoped(pool))

	if !announcementVisible(t, repo, avNoSecAnnouncementID, repository.SearchScope{Unrestricted: true}) {
		t.Error("internal caller: security announcement in a no-security-contact project = not visible, want visible")
	}
	if !announcementVisible(t, repo, avNoSecAnnouncementID, repository.SearchScope{ProjectIDs: []string{avNoSecProjectID}, ViewerEmail: "av-nosec-portal-user@test.local"}) {
		t.Error("PORTAL_USER, no security contact in project: security announcement = not visible, want visible (the fallback)")
	}
	if announcementVisible(t, repo, avNoSecAnnouncementID, repository.SearchScope{ProjectIDs: []string{avNoSecProjectID}, ViewerEmail: "nobody@nowhere.local"}) {
		t.Error("unrelated caller: security announcement in a no-security-contact project = visible, want not visible")
	}
}
