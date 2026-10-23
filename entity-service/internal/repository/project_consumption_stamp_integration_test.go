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
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// consumptionStampPool builds a private schema with the project columns the consumption
// update writes. Skipped without INSTANCE_INGESTION_TEST_DSN.
func consumptionStampPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("INSTANCE_INGESTION_TEST_DSN")
	if dsn == "" {
		t.Skip("INSTANCE_INGESTION_TEST_DSN not set")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)
	schema := fmt.Sprintf("consumption_stamp_test_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
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
	for _, ddl := range []string{
		`CREATE TYPE choreo_application_status_enum AS ENUM ('PENDING', 'CREATED_APPLICATION', 'SUBSCRIBED_APPLICATION', 'GENERATED_CREDENTIALS', 'COMPLETED')`,
		`CREATE TABLE project (
			id UUID PRIMARY KEY, name VARCHAR(255), description TEXT, choreo_application_id VARCHAR(255), choreo_application_status choreo_application_status_enum,
			product_consumption_client_id VARCHAR(255), product_consumption_client_secret VARCHAR(255),
			product_consumption_primary_secret_key VARCHAR(255), product_consumption_secondary_secret_key VARCHAR(255),
			product_consumption_license_secrets TEXT, consumption_tracking_file_generated_on TIMESTAMPTZ,
			updated_on TIMESTAMPTZ, updated_by VARCHAR(255))`,
		`INSERT INTO project (id, choreo_application_status) VALUES ('7d8783f7-3b65-4b10-9140-4c6aa5e45a87', 'PENDING')`,
	} {
		if _, err := pool.Exec(ctx, ddl); err != nil {
			t.Fatal(err)
		}
	}
	return pool
}

// Reaching COMPLETED stamps when the licence file's secrets were generated, as ServiceNow's
// status 5 update did; the earlier steps don't, and a later lower status keeps the stamp.
func TestProjectConsumption_StampsTheFileGeneratedDateOnCompletion(t *testing.T) {
	pool := consumptionStampPool(t)
	repo := repository.NewProjectRepository(repository.NewScoped(pool))
	ctx := internalCtx()
	const id = "7d8783f7-3b65-4b10-9140-4c6aa5e45a87"
	stamp := func() *time.Time {
		t.Helper()
		var v *time.Time
		if err := pool.QueryRow(context.Background(), `SELECT consumption_tracking_file_generated_on FROM project WHERE id = $1`, id).Scan(&v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	app, key := "app-1", "abc"

	for _, status := range []int{2, 3, 4} {
		if err := repo.UpdateProjectConsumption(ctx, id, domain.ProjectConsumptionUpdateRequest{Status: status, ApplicationID: &app}, "pcs"); err != nil {
			t.Fatal(err)
		}
		if got := stamp(); got != nil {
			t.Fatalf("status %d stamped the file-generated date: %v", status, got)
		}
	}

	if err := repo.UpdateProjectConsumption(ctx, id, domain.ProjectConsumptionUpdateRequest{Status: 5, PrimarySecretKey: &key}, "pcs"); err != nil {
		t.Fatal(err)
	}
	first := stamp()
	if first == nil {
		t.Fatal("reaching COMPLETED did not stamp the file-generated date")
	}

	if err := repo.UpdateProjectConsumption(ctx, id, domain.ProjectConsumptionUpdateRequest{Status: 4}, "pcs"); err != nil {
		t.Fatal(err)
	}
	if got := stamp(); got == nil || !got.Equal(*first) {
		t.Errorf("a later lower status changed the stamp: %v, want %v", got, *first)
	}

	// A repeated status 5 with no secrets is not a regeneration: the stamp stays.
	if err := repo.UpdateProjectConsumption(ctx, id, domain.ProjectConsumptionUpdateRequest{Status: 5}, "pcs"); err != nil {
		t.Fatal(err)
	}
	if got := stamp(); got == nil || !got.Equal(*first) {
		t.Errorf("a repeated status 5 without secrets changed the stamp: %v, want %v", got, *first)
	}

	// Status 5 that brings new secrets is a regeneration, as in ServiceNow: the stamp moves.
	newKey := "def"
	if err := repo.UpdateProjectConsumption(ctx, id, domain.ProjectConsumptionUpdateRequest{Status: 5, PrimarySecretKey: &newKey}, "pcs"); err != nil {
		t.Fatal(err)
	}
	if got := stamp(); got == nil || !got.After(*first) {
		t.Errorf("regenerated secrets did not move the stamp: %v, first %v", got, *first)
	}
}

// A status-4 update that carries the secret key reads back as status 5, so it is stamped
// too; a legacy row that already reads as 5 is not stamped by an update bringing nothing.
func TestProjectConsumption_StampFollowsTheStatusTheProjectReadsAs(t *testing.T) {
	pool := consumptionStampPool(t)
	repo := repository.NewProjectRepository(repository.NewScoped(pool))
	ctx := internalCtx()
	const fresh, legacy = "7d8783f7-3b65-4b10-9140-4c6aa5e45a87", "8e9894a8-4c76-4c21-a251-5d7bb6f56b98"
	if _, err := pool.Exec(context.Background(), `INSERT INTO project (id, choreo_application_status, product_consumption_primary_secret_key)
		VALUES ($1, 'GENERATED_CREDENTIALS', 'old')`, legacy); err != nil {
		t.Fatal(err)
	}
	stamp := func(id string) *time.Time {
		t.Helper()
		var v *time.Time
		if err := pool.QueryRow(context.Background(), `SELECT consumption_tracking_file_generated_on FROM project WHERE id = $1`, id).Scan(&v); err != nil {
			t.Fatal(err)
		}
		return v
	}

	key := "abc"
	if err := repo.UpdateProjectConsumption(ctx, fresh, domain.ProjectConsumptionUpdateRequest{Status: 4, PrimarySecretKey: &key}, "pcs"); err != nil {
		t.Fatal(err)
	}
	if got, err := repo.GetProjectConsumptionStatus(ctx, fresh); err != nil || got.Status != 5 {
		t.Fatalf("status after a status-4 update with the key: %+v, %v", got, err)
	}
	if stamp(fresh) == nil {
		t.Error("a status-4 update carrying the key, which reads as 5, was not stamped")
	}

	app := "app-1"
	if err := repo.UpdateProjectConsumption(ctx, legacy, domain.ProjectConsumptionUpdateRequest{Status: 4, ApplicationID: &app}, "pcs"); err != nil {
		t.Fatal(err)
	}
	if got := stamp(legacy); got != nil {
		t.Errorf("an update bringing no secrets stamped a legacy row: %v", got)
	}
}

// Replaying a status-5 request with the same secret is not a regeneration, and a
// completed legacy row with no date does not get today's date from a later status 5.
func TestProjectConsumption_StampIgnoresReplaysAndLegacyCompletedRows(t *testing.T) {
	pool := consumptionStampPool(t)
	repo := repository.NewProjectRepository(repository.NewScoped(pool))
	ctx := internalCtx()
	const fresh, legacy = "7d8783f7-3b65-4b10-9140-4c6aa5e45a87", "9fa9a5b9-5d87-4d32-b362-6e8cc7067ca9"
	if _, err := pool.Exec(context.Background(), `INSERT INTO project (id, choreo_application_status, product_consumption_primary_secret_key)
		VALUES ($1, 'COMPLETED', 'old')`, legacy); err != nil {
		t.Fatal(err)
	}
	stamp := func(id string) *time.Time {
		t.Helper()
		var v *time.Time
		if err := pool.QueryRow(context.Background(), `SELECT consumption_tracking_file_generated_on FROM project WHERE id = $1`, id).Scan(&v); err != nil {
			t.Fatal(err)
		}
		return v
	}

	key := "abc"
	req := domain.ProjectConsumptionUpdateRequest{Status: 5, PrimarySecretKey: &key}
	if err := repo.UpdateProjectConsumption(ctx, fresh, req, "pcs"); err != nil {
		t.Fatal(err)
	}
	first := stamp(fresh)
	if first == nil {
		t.Fatal("completion was not stamped")
	}
	if err := repo.UpdateProjectConsumption(ctx, fresh, req, "pcs"); err != nil { // the same request again
		t.Fatal(err)
	}
	if got := stamp(fresh); got == nil || !got.Equal(*first) {
		t.Errorf("a replayed request moved the stamp: %v, want %v", got, *first)
	}

	if err := repo.UpdateProjectConsumption(ctx, legacy, domain.ProjectConsumptionUpdateRequest{Status: 5}, "pcs"); err != nil {
		t.Fatal(err)
	}
	if got := stamp(legacy); got != nil {
		t.Errorf("a legacy completed row was stamped by a status 5 bringing nothing: %v", got)
	}
}
