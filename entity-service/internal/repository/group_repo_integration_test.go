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

// This is an integration test: it runs GroupRepository.SearchGroups against a
// live PostgreSQL instance and checks the assignableOnly filter, which the
// assignment-group picker of the change request form sends. The picker used to
// list the whole team registry, including teams added by hand (such as the
// approval teams) that have no row in "group"; selecting one made the create fail,
// because ServiceNow knows no such group.
//
// It builds a private schema holding just team and "group" and drops it
// afterwards, so it needs no real data and touches none. Skipped without a DSN:
//
//	GROUP_REPO_TEST_DSN=postgres://... go test ./internal/repository/ -run GroupSearchAssignableOnly

package repository_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	groupTestSyncedID  = "f4a2d253-9747-5190-b20d-fe3bf253afb8" // in team and in "group"
	groupTestSyncedID2 = "4a15afee-1b95-9e50-0bb3-da47b04bcb87" // in team and in "group"
	groupTestHandMade  = "dddddddd-0000-4000-8000-000000000102" // in team only
)

func groupTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("GROUP_REPO_TEST_DSN")
	if dsn == "" {
		t.Skip("GROUP_REPO_TEST_DSN not set")
	}
	ctx := context.Background()

	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(admin.Close)

	schema := fmt.Sprintf("group_repo_test_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() { _, _ = admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE") })

	for _, ddl := range []string{
		`CREATE TABLE ` + schema + `."group" (id UUID PRIMARY KEY, name VARCHAR(255))`,
		`CREATE TABLE ` + schema + `.team (id UUID PRIMARY KEY, name VARCHAR(255) NOT NULL)`,
		`INSERT INTO ` + schema + `."group" (id, name) VALUES
			('` + groupTestSyncedID + `', 'Atlas SRE Group'),
			('` + groupTestSyncedID2 + `', 'Americas Support'),
			(gen_random_uuid(), 'A group with no team')`,
		`INSERT INTO ` + schema + `.team (id, name) VALUES
			('` + groupTestSyncedID + `', 'Atlas'),
			('` + groupTestSyncedID2 + `', 'Americas'),
			('` + groupTestHandMade + `', 'CAB Approval')`,
	} {
		if _, err := admin.Exec(ctx, ddl); err != nil {
			t.Fatalf("setup: %v", err)
		}
	}

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse dsn: %v", err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("connect with search_path: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func TestGroupSearchAssignableOnly(t *testing.T) {
	repo := repository.NewGroupRepository(groupTestPool(t))
	ctx := context.Background()

	names := func(t *testing.T, query string, assignableOnly bool) ([]string, int) {
		t.Helper()
		groups, total, err := repo.SearchGroups(ctx, query, assignableOnly, 20, 0)
		if err != nil {
			t.Fatalf("SearchGroups(%q, %v): %v", query, assignableOnly, err)
		}
		out := make([]string, len(groups))
		for i, g := range groups {
			out[i] = g.Name
		}
		return out, total
	}

	t.Run("by default the whole team registry is listed", func(t *testing.T) {
		got, total := names(t, "", false)
		if total != 3 || len(got) != 3 {
			t.Fatalf("got %v (total %d), want all three teams", got, total)
		}
	})

	t.Run("assignableOnly leaves out a team with no group row", func(t *testing.T) {
		got, total := names(t, "", true)
		if total != 2 || len(got) != 2 || got[0] != "Americas" || got[1] != "Atlas" {
			t.Fatalf("got %v (total %d), want Americas and Atlas", got, total)
		}
	})

	t.Run("a search for the hand-made team finds it only without the filter", func(t *testing.T) {
		if got, total := names(t, "CAB", false); total != 1 || len(got) != 1 {
			t.Errorf("without the filter: %v (total %d), want CAB Approval", got, total)
		}
		if got, total := names(t, "CAB", true); total != 0 || len(got) != 0 {
			t.Errorf("with the filter: %v (total %d), want nothing", got, total)
		}
	})

	t.Run("the filter and the search query combine", func(t *testing.T) {
		got, total := names(t, "Atl", true)
		if total != 1 || len(got) != 1 || got[0] != "Atlas" {
			t.Fatalf("got %v (total %d), want Atlas", got, total)
		}
	})
}
