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

// Exercises IncidentRepository.CreateIncident (the plain-Postgres create)
// against a real Postgres with migrations through 0185 applied: priority,
// subcategory, configuration item, assigned engineer, watch list and the
// comment/work-note journal rows all land in one transaction, and a bad
// watcher or a subcategory from another category rolls the whole create
// back. Skipped without INCIDENT_CREATE_TEST_DSN.
//
//	INCIDENT_CREATE_TEST_DSN=postgres://... go test ./internal/repository/ -run IncidentCreate

package repository_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	icCallerID   = "d1000000-0000-0000-0000-000000000001"
	icEngineerID = "d1000000-0000-0000-0000-000000000002"
	icWatcherID  = "d1000000-0000-0000-0000-000000000003"
	icServiceID  = "d2000000-0000-0000-0000-000000000001"
	icCIID       = "d3000000-0000-0000-0000-000000000001"
	icSubject    = "incident-create integration test"
)

func incidentCreatePool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("INCIDENT_CREATE_TEST_DSN")
	if dsn == "" {
		t.Skip("INCIDENT_CREATE_TEST_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func seedIncidentCreateFixture(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)
	cleanup := func() {
		_, _ = scoped.Exec(ctx, `DELETE FROM work_item WHERE subject = $1`, icSubject)
		_, _ = pool.Exec(ctx, `DELETE FROM service WHERE id = $1`, icServiceID)
		_, _ = pool.Exec(ctx, `DELETE FROM "user" WHERE id = ANY($1::uuid[])`, []string{icCallerID, icEngineerID, icWatcherID})
	}
	cleanup()
	t.Cleanup(cleanup)

	now := time.Now().UTC()
	for _, u := range []struct{ id, name, email string }{
		{icCallerID, "ic-caller", "ic-caller@test.local"},
		{icEngineerID, "ic-engineer", "ic-engineer@test.local"},
		{icWatcherID, "ic-watcher", "IC-Watcher@test.local"},
	} {
		if _, err := pool.Exec(ctx, `INSERT INTO "user" (id, created_on, updated_on, user_name, email) VALUES ($1, $2, $2, $3, $4)`,
			u.id, now, u.name, u.email); err != nil {
			t.Fatalf("seed user: %v", err)
		}
	}
	if _, err := pool.Exec(ctx, `INSERT INTO service (id, created_on, updated_on, created_by, updated_by, name, number)
		VALUES ($1, $2, $2, 'test', 'test', 'IC Service', 'IC-SVC-1')`, icServiceID, now); err != nil {
		t.Fatalf("seed service: %v", err)
	}
}

func icRequest() domain.CreateIncidentRequest {
	sub := domain.IncidentSubcategoryPartialOutage
	ci := icCIID
	engineer := icEngineerID
	comments := "customer-visible description"
	notes := "internal work note"
	return domain.CreateIncidentRequest{
		Subject:             icSubject,
		CallerID:            icCallerID,
		Category:            domain.IncidentCategoryServiceInterruption,
		Subcategory:         &sub,
		ServiceID:           icServiceID,
		ConfigurationItemID: &ci,
		Impact:              domain.IncidentImpactMedium,
		Urgency:             domain.IncidentUrgencyHigh,
		AssignedEngineerID:  &engineer,
		WatchList:           []string{icWatcherID, "ic-watcher@TEST.local"},
		AdditionalComments:  &comments,
		WorkNotes:           &notes,
	}
}

func TestIncidentCreate_PersistsEveryField(t *testing.T) {
	pool := incidentCreatePool(t)
	seedIncidentCreateFixture(t, pool)
	scoped := repository.NewScoped(pool)
	repo := repository.NewIncidentRepository(scoped)
	ctx := repository.WithSystemIdentity(context.Background())

	sub := "Partial Outage"
	resp, err := repo.CreateIncident(ctx, icRequest(), "HIGH", &sub, "jane.doe@test.local")
	if err != nil {
		t.Fatalf("CreateIncident: %v", err)
	}
	if resp.Incident.ID == "" || resp.Incident.Number == "" || resp.Incident.CreatedBy != "jane.doe@test.local" {
		t.Fatalf("unexpected response: %+v", resp)
	}

	var state, priority, subLabel, ci, assignedTo string
	if err := scoped.QueryRow(ctx, `
		SELECT inc.state::text, inc.priority::text, sc.label, inc.cmdb_ci_id::text, wi.assigned_to_id::text
		FROM incident inc
		JOIN work_item wi ON wi.id = inc.id
		JOIN incident_subcategory sc ON sc.id = inc.subcategory_id
		WHERE inc.id = $1`, resp.Incident.ID).Scan(&state, &priority, &subLabel, &ci, &assignedTo); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if state != "NEW" || priority != "HIGH" || subLabel != "Partial Outage" || ci != icCIID || assignedTo != icEngineerID {
		t.Errorf("got state=%s priority=%s subcategory=%s ci=%s assignedTo=%s", state, priority, subLabel, ci, assignedTo)
	}

	var watchers int
	if err := scoped.QueryRow(ctx, `SELECT count(*) FROM work_item_watcher WHERE work_item_id = $1 AND user_id = $2`,
		resp.Incident.ID, icWatcherID).Scan(&watchers); err != nil {
		t.Fatalf("read watchers: %v", err)
	}
	if watchers != 1 {
		t.Errorf("watchers = %d, want 1 (an id and an email for the same user collapse)", watchers)
	}

	rows, err := scoped.Query(ctx, `SELECT type::text, content, created_by FROM comment WHERE work_item_id = $1 ORDER BY type`, resp.Incident.ID)
	if err != nil {
		t.Fatalf("read comments: %v", err)
	}
	defer rows.Close()
	got := map[string]string{}
	for rows.Next() {
		var typ, content, by string
		if err := rows.Scan(&typ, &content, &by); err != nil {
			t.Fatalf("scan comment: %v", err)
		}
		if by != "jane.doe@test.local" {
			t.Errorf("comment %s created_by = %s", typ, by)
		}
		got[typ] = content
	}
	if got["COMMENT"] != "customer-visible description" || got["WORK_NOTE"] != "internal work note" || len(got) != 2 {
		t.Errorf("journal rows = %v", got)
	}
}

func TestIncidentCreate_RollsBackOnBadInput(t *testing.T) {
	pool := incidentCreatePool(t)
	seedIncidentCreateFixture(t, pool)
	scoped := repository.NewScoped(pool)
	repo := repository.NewIncidentRepository(scoped)
	ctx := repository.WithSystemIdentity(context.Background())

	t.Run("unknown watcher", func(t *testing.T) {
		req := icRequest()
		req.WatchList = []string{"nobody@test.local"}
		sub := "Partial Outage"
		_, err := repo.CreateIncident(ctx, req, "HIGH", &sub, "jane.doe@test.local")
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) {
			t.Fatalf("expected ValidationError, got %T: %v", err, err)
		}
	})
	t.Run("subcategory from another category", func(t *testing.T) {
		sub := "dns" // NETWORK, not SERVICE_INTERRUPTION
		_, err := repo.CreateIncident(ctx, icRequest(), "HIGH", &sub, "jane.doe@test.local")
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) {
			t.Fatalf("expected ValidationError, got %T: %v", err, err)
		}
	})

	var n int
	if err := scoped.QueryRow(ctx, `SELECT count(*) FROM work_item WHERE subject = $1`, icSubject).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Errorf("%d work_item rows left behind by rejected creates, want 0", n)
	}
}
