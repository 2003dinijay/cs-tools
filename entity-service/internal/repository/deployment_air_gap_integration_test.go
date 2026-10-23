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
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// airGapTestPool builds a private schema with the deployment columns this touches
// and applies migration 0246 to it. Skipped without INSTANCE_INGESTION_TEST_DSN.
func airGapTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("INSTANCE_INGESTION_TEST_DSN")
	if dsn == "" {
		t.Skip("INSTANCE_INGESTION_TEST_DSN not set")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(admin.Close)
	schema := fmt.Sprintf("air_gap_test_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() { _, _ = admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE") })

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	if _, err := pool.Exec(ctx, `CREATE TABLE deployment (
		id UUID PRIMARY KEY, number VARCHAR(50), name VARCHAR(255),
		updated_on TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_by VARCHAR(255) NOT NULL DEFAULT 'seed')`); err != nil {
		t.Fatal(err)
	}
	migration, err := os.ReadFile("../../migrations/0246_deployment_air_gapped.sql")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ { // safe to run again
		if _, err := pool.Exec(ctx, string(migration)); err != nil {
			t.Fatalf("apply 0246 (run %d): %v", i+1, err)
		}
	}
	if _, err := pool.Exec(ctx, `INSERT INTO deployment (id, number, name) VALUES ('b206552b-1bd7-7e14-a002-c9d3604bcbda', 'DEP000001136', 'Test')`); err != nil {
		t.Fatal(err)
	}
	return pool
}

func TestDeploymentAirGap_SetAndClear(t *testing.T) {
	pool := airGapTestPool(t)
	repo := repository.NewDeploymentAirGapRepository(repository.NewScoped(pool))
	ctx := internalCtx()
	const id = "b206552b-1bd7-7e14-a002-c9d3604bcbda"

	var before bool
	if err := pool.QueryRow(context.Background(), `SELECT air_gapped FROM deployment WHERE id = $1`, id).Scan(&before); err != nil || before {
		t.Fatalf("a deployment should start not air-gapped: %v %v", before, err)
	}

	got, err := repo.SetAirGapped(ctx, id, true, "csm.user@wso2.com")
	if err != nil {
		t.Fatal(err)
	}
	if !got.AirGapped || got.ID != id || got.Number == nil || *got.Number != "DEP000001136" || got.UpdatedBy != "csm.user@wso2.com" {
		t.Errorf("after setting: %+v", got)
	}

	got, err = repo.SetAirGapped(ctx, id, false, "other@wso2.com")
	if err != nil {
		t.Fatal(err)
	}
	if got.AirGapped || got.UpdatedBy != "other@wso2.com" {
		t.Errorf("after clearing: %+v", got)
	}

	_, err = repo.SetAirGapped(ctx, "00000000-0000-4000-8000-000000000000", true, "x")
	var nf *apierror.NotFoundError
	if !errors.As(err, &nf) {
		t.Errorf("an unknown deployment: got %v, want NotFoundError", err)
	}
}
