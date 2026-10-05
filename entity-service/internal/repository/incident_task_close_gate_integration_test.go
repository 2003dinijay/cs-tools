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

// Integration tests for the incident close gate (an incident only closes
// once all of its incident tasks are closed) and
// IncidentTaskRepository.UpdateIncidentTask, against a real Postgres with
// migrations through 0189 applied. Skipped unless
// INCIDENT_TASK_GATE_TEST_DSN is set.
package repository_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const gateTaskSubject = "incident-task-gate integration test"

func incidentTaskGatePool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("INCIDENT_TASK_GATE_TEST_DSN")
	if dsn == "" {
		t.Skip("INCIDENT_TASK_GATE_TEST_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	seedIncidentCreateFixture(t, pool)
	ctx := repository.WithSystemIdentity(context.Background())
	cleanup := func() {
		_, _ = repository.NewScoped(pool).Exec(ctx, `DELETE FROM work_item WHERE subject = $1`, gateTaskSubject)
	}
	cleanup()
	t.Cleanup(cleanup)
	return pool
}

// resolvedIncident creates an incident and moves it to RESOLVED with a
// resolution code and notes, so CLOSED only has the task gate left to pass.
func resolvedIncident(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	ctx := repository.WithSystemIdentity(context.Background())
	repo := repository.NewIncidentRepository(repository.NewScoped(pool))
	resp, err := repo.CreateIncident(ctx, icRequest(), "HIGH", nil, "jane.doe@test.local")
	if err != nil {
		t.Fatalf("CreateIncident: %v", err)
	}
	state, code, notes := "RESOLVED", "SOLVED_PERMANENTLY", "fixed the gateway config"
	if err := repo.UpdateIncidentLifecycle(ctx, resp.Incident.ID, repository.IncidentLifecycleUpdate{
		State: &state, ResolutionCode: &code, ResolutionNotes: &notes,
	}, "jane.doe@test.local"); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	return resp.Incident.ID
}

// addTask inserts an incident task in the given state; nil leaves state NULL.
func addTask(t *testing.T, pool *pgxpool.Pool, incidentID string, state *string) (id, number string) {
	t.Helper()
	ctx := repository.WithSystemIdentity(context.Background())
	if err := repository.NewScoped(pool).QueryRow(ctx, `
		WITH wi AS (
			INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, subject, type)
			VALUES (gen_random_uuid(), NOW(), NOW(), 'test', 'test', next_portal_work_item_number(), $1, 'INCIDENT_TASK')
			RETURNING id, number
		), it AS (
			INSERT INTO incident_task (id, opened_on, state, is_active, incident_id)
			SELECT id, NOW(), $2::TEXT::incident_task_state_enum, TRUE, $3::uuid FROM wi
			RETURNING id
		)
		SELECT wi.id::TEXT, wi.number FROM wi JOIN it ON it.id = wi.id`,
		gateTaskSubject, state, incidentID).Scan(&id, &number); err != nil {
		t.Fatalf("insert task: %v", err)
	}
	return id, number
}

func closeIncident(t *testing.T, pool *pgxpool.Pool, id string) error {
	t.Helper()
	state := "CLOSED"
	return repository.NewIncidentRepository(repository.NewScoped(pool)).UpdateIncidentLifecycle(
		repository.WithSystemIdentity(context.Background()), id,
		repository.IncidentLifecycleUpdate{State: &state}, "jane.doe@test.local")
}

func incidentState(t *testing.T, pool *pgxpool.Pool, id string) string {
	t.Helper()
	var s string
	if err := repository.NewScoped(pool).QueryRow(repository.WithSystemIdentity(context.Background()),
		`SELECT state::TEXT FROM incident WHERE id = $1`, id).Scan(&s); err != nil {
		t.Fatalf("read incident state: %v", err)
	}
	return s
}

func strp(s string) *string { return &s }

// TestIncidentCloseGate_OpenTasksBlockClose: any task not in a closed state
// (OPEN, WORK_IN_PROGRESS, PENDING, or NULL) keeps the incident from
// closing, with a ConflictError naming exactly those tasks.
func TestIncidentCloseGate_OpenTasksBlockClose(t *testing.T) {
	pool := incidentTaskGatePool(t)
	incID := resolvedIncident(t, pool)
	_, openNum := addTask(t, pool, incID, strp("OPEN"))
	_, wipNum := addTask(t, pool, incID, strp("WORK_IN_PROGRESS"))
	_, nullNum := addTask(t, pool, incID, nil)
	_, doneNum := addTask(t, pool, incID, strp("CLOSED_COMPLETE"))

	err := closeIncident(t, pool, incID)
	var ce *apierror.ConflictError
	if !errors.As(err, &ce) {
		t.Fatalf("expected ConflictError, got %T: %v", err, err)
	}
	for _, n := range []string{openNum, wipNum, nullNum} {
		if !strings.Contains(ce.Msg, n) {
			t.Errorf("message %q does not name open task %s", ce.Msg, n)
		}
	}
	if strings.Contains(ce.Msg, doneNum) {
		t.Errorf("message %q names closed task %s", ce.Msg, doneNum)
	}
	if got := incidentState(t, pool, incID); got != "RESOLVED" {
		t.Errorf("incident state = %s, want RESOLVED (nothing written)", got)
	}
}

// TestIncidentCloseGate_ClosingTasksUnblocksClose walks the real sequence:
// close each task through UpdateIncidentTask, then the incident closes.
func TestIncidentCloseGate_ClosingTasksUnblocksClose(t *testing.T) {
	pool := incidentTaskGatePool(t)
	incID := resolvedIncident(t, pool)
	taskA, _ := addTask(t, pool, incID, strp("OPEN"))
	taskB, _ := addTask(t, pool, incID, strp("OPEN"))
	tasks := repository.NewIncidentTaskRepository(repository.NewScoped(pool))
	ctx := repository.WithSystemIdentity(context.Background())

	if err := tasks.UpdateIncidentTask(ctx, domain.UpdateIncidentTaskRequest{ID: taskA, State: strp("CLOSED_COMPLETE")}, "ic-engineer@test.local"); err != nil {
		t.Fatalf("close task A: %v", err)
	}
	if err := closeIncident(t, pool, incID); err == nil {
		t.Fatal("incident closed with task B still open")
	}
	if err := tasks.UpdateIncidentTask(ctx, domain.UpdateIncidentTaskRequest{ID: taskB, State: strp("CLOSED_SKIPPED"), CloseNotes: strp("not needed")}, "ic-engineer@test.local"); err != nil {
		t.Fatalf("close task B: %v", err)
	}
	if err := closeIncident(t, pool, incID); err != nil {
		t.Fatalf("close incident with every task closed: %v", err)
	}
	if got := incidentState(t, pool, incID); got != "CLOSED" {
		t.Errorf("incident state = %s, want CLOSED", got)
	}
}

// TestIncidentCloseGate_NoTasks: an incident with no tasks closes as before.
func TestIncidentCloseGate_NoTasks(t *testing.T) {
	pool := incidentTaskGatePool(t)
	incID := resolvedIncident(t, pool)
	if err := closeIncident(t, pool, incID); err != nil {
		t.Fatalf("close incident with no tasks: %v", err)
	}
}

// TestUpdateIncidentTask_CloseAndReopen checks the side effects: closing
// stamps closed_on/closed_by_id and deactivates; reopening reverses both.
func TestUpdateIncidentTask_CloseAndReopen(t *testing.T) {
	pool := incidentTaskGatePool(t)
	incID := resolvedIncident(t, pool)
	taskID, _ := addTask(t, pool, incID, strp("OPEN"))
	tasks := repository.NewIncidentTaskRepository(repository.NewScoped(pool))
	ctx := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)

	read := func() (state string, active bool, closedSet bool, closedBy *string) {
		if err := scoped.QueryRow(ctx, `SELECT state::TEXT, is_active, closed_on IS NOT NULL, closed_by_id::TEXT FROM incident_task WHERE id = $1`, taskID).
			Scan(&state, &active, &closedSet, &closedBy); err != nil {
			t.Fatalf("read task: %v", err)
		}
		return
	}

	// Email matched case-insensitively: the fixture stores ic-engineer@test.local.
	if err := tasks.UpdateIncidentTask(ctx, domain.UpdateIncidentTaskRequest{ID: taskID, State: strp("CLOSED_INCOMPLETE"), CloseNotes: strp("ran out of time")}, "IC-Engineer@test.local"); err != nil {
		t.Fatalf("close: %v", err)
	}
	state, active, closedSet, closedBy := read()
	if state != "CLOSED_INCOMPLETE" || active || !closedSet || closedBy == nil || *closedBy != icEngineerID {
		t.Errorf("after close: state=%s active=%v closed_on set=%v closed_by=%v, want CLOSED_INCOMPLETE/false/true/%s", state, active, closedSet, closedBy, icEngineerID)
	}
	detail, err := tasks.GetIncidentTask(ctx, taskID)
	if err != nil || detail.CloseNotes == nil || *detail.CloseNotes != "ran out of time" {
		t.Errorf("GetIncidentTask closeNotes = %v (err %v), want the saved notes", detail.CloseNotes, err)
	}

	if err := tasks.UpdateIncidentTask(ctx, domain.UpdateIncidentTaskRequest{ID: taskID, State: strp("OPEN")}, "ic-engineer@test.local"); err != nil {
		t.Fatalf("reopen: %v", err)
	}
	state, active, closedSet, closedBy = read()
	if state != "OPEN" || !active || closedSet || closedBy != nil {
		t.Errorf("after reopen: state=%s active=%v closed_on set=%v closed_by=%v, want OPEN/true/false/<nil>", state, active, closedSet, closedBy)
	}

	err = tasks.UpdateIncidentTask(ctx, domain.UpdateIncidentTaskRequest{ID: "f0000000-0000-0000-0000-000000000001", State: strp("OPEN")}, "x@test.local")
	var nf *apierror.NotFoundError
	if !errors.As(err, &nf) {
		t.Errorf("unknown task: got %T: %v, want NotFoundError", err, err)
	}
}
