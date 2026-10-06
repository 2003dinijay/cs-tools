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

// Integration tests for IncidentRepository.ApplySpecialistHandoff and the
// specialist-handoff summary GetIncidentByID derives, against a real Postgres
// with migrations through 0194 applied. Skipped unless
// INCIDENT_HANDOFF_TEST_DSN is set.
package repository_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	hoSpecialGroup = "e7000000-0000-0000-0000-000000000001"
	hoSubGroup     = "e7000000-0000-0000-0000-000000000002"
	hoTaskSubject  = "[Runbook Task] handoff integration test"
	hoActor        = "ic-engineer@test.local"
)

func handoffPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("INCIDENT_HANDOFF_TEST_DSN")
	if dsn == "" {
		t.Skip("INCIDENT_HANDOFF_TEST_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	seedIncidentCreateFixture(t, pool)
	ctx := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)
	cleanup := func() {
		_, _ = scoped.Exec(ctx, `DELETE FROM work_item WHERE subject = $1`, hoTaskSubject)
		_, _ = pool.Exec(ctx, `DELETE FROM team WHERE key IN ('ho-test-special-ops', 'ho-test-sub-team')`)
		_, _ = pool.Exec(ctx, `DELETE FROM "group" WHERE id = ANY($1::uuid[])`, []string{hoSpecialGroup, hoSubGroup})
	}
	cleanup()
	t.Cleanup(cleanup)
	now := time.Now().UTC()
	for id, name := range map[string]string{hoSpecialGroup: "Test Special Ops", hoSubGroup: "Test Sub Special Ops"} {
		if _, err := pool.Exec(ctx, `INSERT INTO "group" (id, created_on, updated_on, created_by, updated_by, name) VALUES ($1, $2, $2, 'test', 'test', $3)`, id, now, name); err != nil {
			t.Fatalf("seed group: %v", err)
		}
	}
	// The test service's routes, as migration 0194 seeds Choreo's: a default
	// team and one sub-team, each a team row linked to its group.
	if _, err := pool.Exec(ctx, `
		WITH teams AS (
			INSERT INTO team (id, created_on, updated_on, created_by, updated_by, name, type, key, group_id)
			VALUES (gen_random_uuid(), NOW(), NOW(), 'test', 'test', 'Test Special Ops', 'SPECIAL-OPS', 'ho-test-special-ops', $2::uuid),
			       (gen_random_uuid(), NOW(), NOW(), 'test', 'test', 'Test Sub Team',    'SPECIAL-OPS', 'ho-test-sub-team',    $3::uuid)
			RETURNING id, key
		)
		INSERT INTO specialist_handoff_route (service_id, team_id, is_default, github_owner, github_repo)
		SELECT $1::uuid, id, key = 'ho-test-special-ops', 'test-owner', 'test-repo' FROM teams`,
		icServiceID, hoSpecialGroup, hoSubGroup); err != nil {
		t.Fatalf("seed routes: %v", err)
	}
	return pool
}

// inProgressIncident creates an incident assigned to the engineer, In Progress.
func inProgressIncident(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	ctx := repository.WithSystemIdentity(context.Background())
	repo := repository.NewIncidentRepository(repository.NewScoped(pool))
	resp, err := repo.CreateIncident(ctx, icRequest(), "HIGH", nil, "jane.doe@test.local")
	if err != nil {
		t.Fatalf("CreateIncident: %v", err)
	}
	state := "IN_PROGRESS"
	if err := repo.UpdateIncidentLifecycle(ctx, resp.Incident.ID, repository.IncidentLifecycleUpdate{State: &state}, "jane.doe@test.local"); err != nil {
		t.Fatalf("move to In Progress: %v", err)
	}
	return resp.Incident.ID
}

const hoBlob = `{"reasonCode":"runbook-not-working","reasonDescription":"Runbook doesn't solve the incident","escalationTeam":"ho-test-sub-team"}`

func hoPlan(snap repository.SpecialistHandoffSnapshot) (repository.SpecialistHandoffPlan, error) {
	group := hoSubGroup
	return repository.SpecialistHandoffPlan{GroupID: group, TaskSubject: hoTaskSubject, TaskGroupID: &group, WorkNotes: []string{hoBlob}}, nil
}

// TestSpecialistHandoff_WritesAndReadsBack: the handoff moves the group,
// clears the assignee, opens the runbook task and writes the reason note;
// GetIncidentByID then derives the summary from them, GitHub link included.
func TestSpecialistHandoff_WritesAndReadsBack(t *testing.T) {
	pool := handoffPool(t)
	incID := inProgressIncident(t, pool)
	ctx := repository.WithSystemIdentity(context.Background())
	repo := repository.NewIncidentRepository(repository.NewScoped(pool))

	if v, err := repo.GetIncidentByID(ctx, incID); err != nil || v.SpecialistHandoff != nil ||
		v.CanHandOffToSpecialist == nil || !*v.CanHandOffToSpecialist {
		t.Fatalf("before the handoff: summary %+v can %v err %v, want no summary and eligible", v.SpecialistHandoff, v.CanHandOffToSpecialist, err)
	}

	var seen repository.SpecialistHandoffSnapshot
	written, err := repo.ApplySpecialistHandoff(ctx, incID, hoActor, func(s repository.SpecialistHandoffSnapshot) (repository.SpecialistHandoffPlan, error) {
		seen = s
		return hoPlan(s)
	})
	if err != nil {
		t.Fatalf("ApplySpecialistHandoff: %v", err)
	}
	if seen.State != "IN_PROGRESS" || seen.ServiceID == nil || *seen.ServiceID != icServiceID || seen.Number == "" {
		t.Errorf("snapshot %+v, want the In Progress incident on %s", seen, icServiceID)
	}
	routes := map[string]repository.SpecialistHandoffRoute{}
	for _, r := range seen.Routes {
		routes[r.TeamKey] = r
	}
	def, sub := routes["ho-test-special-ops"], routes["ho-test-sub-team"]
	if len(seen.Routes) != 2 || !def.IsDefault || sub.IsDefault || sub.GroupID == nil || *sub.GroupID != hoSubGroup ||
		sub.TeamName != "Test Sub Team" || sub.GithubRepo == nil || *sub.GithubRepo != "test-repo" {
		t.Errorf("snapshot routes %+v, want the default team and the sub-team with their groups", seen.Routes)
	}
	if written.GroupName != "Test Sub Special Ops" || !strings.HasPrefix(written.TaskNumber, "CS-PORTAL-") {
		t.Errorf("written %+v", written)
	}

	// The follow-up note the service writes once GitHub has answered.
	if _, err := repo.CreateIncidentComment(ctx, incID, domain.CommentTypeWorkNote,
		"Escalated to Special Ops team. Escalated by IC Engineer(ic-engineer@test.local) Opened an internal issue. Please access the ticket using the link https://github.com/wso2-enterprise/choreo/issues/42 to add more details to the ticket if needed", hoActor); err != nil {
		t.Fatalf("escalated note: %v", err)
	}

	var task struct {
		state, priority, service, group string
		active                          bool
	}
	if err := repository.NewScoped(pool).QueryRow(ctx, `
		SELECT it.state::text, it.priority::text, it.service_id::text, wi.assignment_group_id::text, it.is_active
		FROM incident_task it JOIN work_item wi ON wi.id = it.id WHERE it.id = $1 AND it.incident_id = $2`, written.TaskID, incID).
		Scan(&task.state, &task.priority, &task.service, &task.group, &task.active); err != nil {
		t.Fatalf("read task: %v", err)
	}
	if task.state != "OPEN" || task.priority != "CRITICAL" || task.service != icServiceID || task.group != hoSubGroup || !task.active {
		t.Errorf("task %+v, want OPEN CRITICAL on the incident's service, in the Special Ops group, active", task)
	}

	v, err := repo.GetIncidentByID(ctx, incID)
	if err != nil {
		t.Fatalf("GetIncidentByID: %v", err)
	}
	if v.AssignmentGroup == nil || v.AssignmentGroup.ID != hoSubGroup || v.AssignedTo != nil {
		t.Errorf("group %+v assignee %+v, want the specialist group and no assignee", v.AssignmentGroup, v.AssignedTo)
	}
	// Handed to a sub-team's group, not the default's: still eligible, as in
	// ServiceNow (only the default group hides the button).
	if v.CanHandOffToSpecialist == nil || !*v.CanHandOffToSpecialist {
		t.Errorf("after a sub-team handoff: canHandOffToSpecialist %v, want true", v.CanHandOffToSpecialist)
	}
	s := v.SpecialistHandoff
	if s == nil {
		t.Fatal("SpecialistHandoff is nil after a handoff")
	}
	if s.ReasonCode != "runbook-not-working" || s.ReasonDescription != "Runbook doesn't solve the incident" ||
		s.EscalationTeam == nil || *s.EscalationTeam != "ho-test-sub-team" || s.HandedOffBy == nil || *s.HandedOffBy != hoActor || s.HandedOffAt == "" {
		t.Errorf("summary reason/team/by: %+v", s)
	}
	if s.GithubIssueURL == nil || *s.GithubIssueURL != "https://github.com/wso2-enterprise/choreo/issues/42" {
		t.Errorf("github url %v", s.GithubIssueURL)
	}
	if s.AssignmentGroup.ID != hoSubGroup || s.Task.Number != written.TaskNumber || s.Task.Subject != hoTaskSubject ||
		s.Task.StateLabel == nil || *s.Task.StateLabel != "Open" {
		t.Errorf("summary group/task: %+v / %+v", s.AssignmentGroup, s.Task)
	}
}

// TestSpecialistHandoff_RefusedPlanWritesNothing: an eligibility refusal
// rolls the whole handoff back.
func TestSpecialistHandoff_RefusedPlanWritesNothing(t *testing.T) {
	pool := handoffPool(t)
	incID := inProgressIncident(t, pool)
	ctx := repository.WithSystemIdentity(context.Background())
	repo := repository.NewIncidentRepository(repository.NewScoped(pool))
	before, _ := repo.GetIncidentByID(ctx, incID)

	refusal := &apierror.ConflictError{Msg: "not eligible"}
	_, err := repo.ApplySpecialistHandoff(ctx, incID, hoActor, func(repository.SpecialistHandoffSnapshot) (repository.SpecialistHandoffPlan, error) {
		return repository.SpecialistHandoffPlan{}, refusal
	})
	if !errors.Is(err, refusal) {
		t.Fatalf("got %v, want the plan's refusal", err)
	}
	after, _ := repo.GetIncidentByID(ctx, incID)
	if (before.AssignmentGroup == nil) != (after.AssignmentGroup == nil) || after.SpecialistHandoff != nil {
		t.Errorf("incident changed: group %+v -> %+v, summary %+v", before.AssignmentGroup, after.AssignmentGroup, after.SpecialistHandoff)
	}
	var tasks int
	_ = repository.NewScoped(pool).QueryRow(ctx, `SELECT count(*) FROM incident_task WHERE incident_id = $1`, incID).Scan(&tasks)
	if tasks != 0 {
		t.Errorf("%d tasks written by a refused handoff", tasks)
	}
}

// TestSpecialistHandoff_UnknownGroupAndIncident: a specialist group missing
// from this database is a ValidationError; an unknown incident NotFound.
func TestSpecialistHandoff_UnknownGroupAndIncident(t *testing.T) {
	pool := handoffPool(t)
	incID := inProgressIncident(t, pool)
	ctx := repository.WithSystemIdentity(context.Background())
	repo := repository.NewIncidentRepository(repository.NewScoped(pool))

	_, err := repo.ApplySpecialistHandoff(ctx, incID, hoActor, func(repository.SpecialistHandoffSnapshot) (repository.SpecialistHandoffPlan, error) {
		return repository.SpecialistHandoffPlan{GroupID: "e7000000-0000-0000-0000-0000000000ff", TaskSubject: hoTaskSubject}, nil
	})
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("unknown group: %T %v, want ValidationError", err, err)
	}
	_, err = repo.ApplySpecialistHandoff(ctx, "e7000000-0000-0000-0000-0000000000aa", hoActor, hoPlan)
	var nf *apierror.NotFoundError
	if !errors.As(err, &nf) {
		t.Errorf("unknown incident: %T %v, want NotFoundError", err, err)
	}
}

// TestListSpecialistHandoffTeams: the dialog's options are the teams of
// active, non-default routes, by key and name.
func TestListSpecialistHandoffTeams(t *testing.T) {
	pool := handoffPool(t)
	ctx := repository.WithSystemIdentity(context.Background())
	repo := repository.NewIncidentRepository(repository.NewScoped(pool))
	teams, err := repo.ListSpecialistHandoffTeams(ctx)
	if err != nil {
		t.Fatalf("ListSpecialistHandoffTeams: %v", err)
	}
	var sub, def bool
	for _, tm := range teams {
		sub = sub || (tm.Key == "ho-test-sub-team" && tm.Label == "Test Sub Team")
		def = def || tm.Key == "ho-test-special-ops"
	}
	if !sub || def {
		t.Errorf("teams %+v, want the sub-team and not the default team", teams)
	}
}

// TestCanHandOffToSpecialist mirrors canEscalateToSpecialOps: only an In
// Progress incident on a routed service, not in the default group.
func TestCanHandOffToSpecialist(t *testing.T) {
	pool := handoffPool(t)
	incID := inProgressIncident(t, pool)
	ctx := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)
	repo := repository.NewIncidentRepository(scoped)
	can := func() bool {
		t.Helper()
		v, err := repo.GetIncidentByID(ctx, incID)
		if err != nil || v.CanHandOffToSpecialist == nil {
			t.Fatalf("GetIncidentByID: %v / %v", err, v.CanHandOffToSpecialist)
		}
		return *v.CanHandOffToSpecialist
	}
	if !can() {
		t.Error("In Progress on a routed service: want eligible")
	}
	if _, err := scoped.Exec(ctx, `UPDATE work_item SET assignment_group_id = $2 WHERE id = $1`, incID, hoSpecialGroup); err != nil {
		t.Fatal(err)
	}
	if can() {
		t.Error("already in the default Special Ops group: want not eligible")
	}
	if _, err := scoped.Exec(ctx, `UPDATE work_item SET assignment_group_id = NULL WHERE id = $1`, incID); err != nil {
		t.Fatal(err)
	}
	if _, err := scoped.Exec(ctx, `UPDATE incident SET state = 'NEW' WHERE id = $1`, incID); err != nil {
		t.Fatal(err)
	}
	if can() {
		t.Error("not In Progress: want not eligible")
	}
}
