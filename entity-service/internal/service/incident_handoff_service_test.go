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

package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/github"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	hoIncidentID      = "11111111-1111-4111-8111-111111111111"
	hoChoreoService   = "b9c999f8-1b86-a010-00ae-86acdd4bcb61"
	hoAsgardeoService = "97ed1b8b-1ba2-6c10-00ae-86acdd4bcbd3"
	hoChoreoSpecial   = "fe0d8868-1b0b-3010-d64e-64a2604bcb3c"
	hoChoreoRuntime   = "80dade5d-1b70-0710-a002-c9d3604bcbd7"
	hoChoreoAPIM      = "a79a1e9d-1b70-0710-a002-c9d3604bcb20"
	hoAsgardeoSpecial = "7fb4f4c6-1b4b-3810-aea4-a936604bcb90"
)

func hoStr(s string) *string { return &s }

func hoTeam(t domain.IncidentSpecialistHandoffEscalationTeam) *domain.IncidentSpecialistHandoffEscalationTeam {
	return &t
}

// hoRoutes are the rows migration 0194 seeds for a service.
func hoRoutes(service string) []repository.SpecialistHandoffRoute {
	choreo := func(key, name, group string, def bool) repository.SpecialistHandoffRoute {
		return repository.SpecialistHandoffRoute{TeamKey: key, TeamName: name, IsDefault: def, GroupID: hoStr(group),
			GithubOwner: hoStr("wso2-enterprise"), GithubRepo: hoStr("choreo")}
	}
	switch service {
	case hoChoreoService:
		return []repository.SpecialistHandoffRoute{
			choreo("choreo-special-ops", "Choreo Special Ops", hoChoreoSpecial, true),
			choreo("choreo-runtime-team", "Choreo Runtime Team", hoChoreoRuntime, false),
			choreo("choreo-apim-team", "Choreo APIM Team", hoChoreoAPIM, false),
		}
	case hoAsgardeoService:
		return []repository.SpecialistHandoffRoute{{TeamKey: "asgardeo-special-ops", TeamName: "Asgardeo Special Ops", IsDefault: true,
			GroupID: hoStr(hoAsgardeoSpecial), GithubOwner: hoStr("wso2-enterprise"), GithubRepo: hoStr("asgardeo-product")}}
	}
	return nil
}

func hoSnapshot(service, group, state string) repository.SpecialistHandoffSnapshot {
	snap := repository.SpecialistHandoffSnapshot{
		IncidentID: hoIncidentID, Number: "INC0099001", Subject: "Gateway 502s", State: state,
		Description: hoStr("All gateways return 502."), Routes: hoRoutes(service),
	}
	if service != "" {
		snap.ServiceID = &service
	}
	if group != "" {
		snap.AssignmentGroupID = &group
		snap.AssignmentGroupName = hoStr("Choreo Operations")
	}
	return snap
}

// TestPlanSpecialistHandoff_Eligibility ports IncidentHandoffUtils
// .checkEligibility: In Progress only, a service with a default route only,
// not already with that route's group -- and a team with no group refuses.
func TestPlanSpecialistHandoff_Eligibility(t *testing.T) {
	req := domain.HandOffIncidentToSpecialistRequest{IncidentID: hoIncidentID, ReasonCode: domain.IncidentSpecialistHandoffReasonNoRunbook}
	noGroup := hoSnapshot(hoAsgardeoService, "", "IN_PROGRESS")
	noGroup.Routes[0].GroupID = nil
	onlySubTeams := hoSnapshot(hoChoreoService, "", "IN_PROGRESS")
	onlySubTeams.Routes = onlySubTeams.Routes[1:]
	for name, snap := range map[string]repository.SpecialistHandoffSnapshot{
		"not In Progress":         hoSnapshot(hoChoreoService, "", "NEW"),
		"no service":              hoSnapshot("", "", "IN_PROGRESS"),
		"service without routes":  hoSnapshot("22222222-2222-4222-8222-222222222222", "", "IN_PROGRESS"),
		"service without default": onlySubTeams,
		"already Choreo SpecOps":  hoSnapshot(hoChoreoService, hoChoreoSpecial, "IN_PROGRESS"),
		"already Asgardeo Ops":    hoSnapshot(hoAsgardeoService, hoAsgardeoSpecial, "IN_PROGRESS"),
		"team without group":      noGroup,
	} {
		t.Run(name, func(t *testing.T) {
			_, _, err := planSpecialistHandoff(req, snap)
			var ce *apierror.ConflictError
			if !errors.As(err, &ce) {
				t.Fatalf("expected ConflictError, got %T: %v", err, err)
			}
		})
	}

	// A sub-team group is not the default group, so -- as in ServiceNow -- it
	// can still be handed off again.
	if _, _, err := planSpecialistHandoff(req, hoSnapshot(hoChoreoService, hoChoreoRuntime, "IN_PROGRESS")); err != nil {
		t.Errorf("incident in Choreo Runtime Special Ops: %v, want it eligible", err)
	}
}

// TestPlanSpecialistHandoff_Routing: the service's routes pick the group; a
// team the service has no route for falls back to its default, as the UI
// action ignores a team for Asgardeo. The runbook task goes to the same
// group as the incident.
func TestPlanSpecialistHandoff_Routing(t *testing.T) {
	cases := []struct {
		name    string
		service string
		team    *domain.IncidentSpecialistHandoffEscalationTeam
		want    string
		repo    string
	}{
		{"choreo default", hoChoreoService, nil, hoChoreoSpecial, "choreo"},
		{"choreo runtime", hoChoreoService, hoTeam(domain.IncidentSpecialistHandoffTeamChoreoRuntime), hoChoreoRuntime, "choreo"},
		{"choreo apim", hoChoreoService, hoTeam(domain.IncidentSpecialistHandoffTeamChoreoAPIM), hoChoreoAPIM, "choreo"},
		{"choreo unknown team", hoChoreoService, hoTeam("moesif-team"), hoChoreoSpecial, "choreo"},
		{"asgardeo", hoAsgardeoService, nil, hoAsgardeoSpecial, "asgardeo-product"},
		{"asgardeo ignores team", hoAsgardeoService, hoTeam(domain.IncidentSpecialistHandoffTeamChoreoRuntime), hoAsgardeoSpecial, "asgardeo-product"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := domain.HandOffIncidentToSpecialistRequest{IncidentID: hoIncidentID, ReasonCode: domain.IncidentSpecialistHandoffReasonNoRunbook, EscalationTeam: c.team}
			plan, route, err := planSpecialistHandoff(req, hoSnapshot(c.service, "", "IN_PROGRESS"))
			if err != nil {
				t.Fatalf("plan: %v", err)
			}
			if plan.GroupID != c.want || derefString(route.GithubRepo) != c.repo || derefString(route.GithubOwner) != "wso2-enterprise" {
				t.Errorf("group %s repo %s/%s, want %s wso2-enterprise/%s", plan.GroupID, derefString(route.GithubOwner), derefString(route.GithubRepo), c.want, c.repo)
			}
			if plan.TaskGroupID == nil || *plan.TaskGroupID != c.want {
				t.Errorf("runbook task group %v, want the Special Ops group %s", plan.TaskGroupID, c.want)
			}
		})
	}
}

// TestPlanSpecialistHandoff_NotesAndTask: the reason note is byte for byte
// what the UI action's modal writes, and the task subject is the UI action's.
func TestPlanSpecialistHandoff_NotesAndTask(t *testing.T) {
	cases := []struct {
		reason  domain.IncidentSpecialistHandoffReasonCode
		team    *domain.IncidentSpecialistHandoffEscalationTeam
		blob    string
		subject string
	}{
		{domain.IncidentSpecialistHandoffReasonNoRunbook, nil,
			`{"reasonCode":"no-runbook","reasonDescription":"Runbook is not available","escalationTeam":null}`,
			"[Runbook Task] No entry available for INC0099001"},
		{domain.IncidentSpecialistHandoffReasonRunbookNotWorking, hoTeam(domain.IncidentSpecialistHandoffTeamChoreoAPIM),
			`{"reasonCode":"runbook-not-working","reasonDescription":"Runbook doesn't solve the incident","escalationTeam":"choreo-apim-team"}`,
			"[Runbook Task] Entry didn't solve the incident INC0099001"},
	}
	for _, c := range cases {
		req := domain.HandOffIncidentToSpecialistRequest{IncidentID: hoIncidentID, ReasonCode: c.reason, EscalationTeam: c.team}
		plan, _, err := planSpecialistHandoff(req, hoSnapshot(hoChoreoService, "", "IN_PROGRESS"))
		if err != nil {
			t.Fatalf("plan: %v", err)
		}
		if len(plan.WorkNotes) != 1 || plan.WorkNotes[0] != c.blob {
			t.Errorf("work notes %q, want [%s]", plan.WorkNotes, c.blob)
		}
		if plan.TaskSubject != c.subject {
			t.Errorf("task subject %q, want %q", plan.TaskSubject, c.subject)
		}
	}
}

type fakeHandoffIssues struct {
	calls  int
	owner  string
	repo   string
	title  string
	body   string
	result *github.CreatedIssue
	err    error
}

func (f *fakeHandoffIssues) CreateIssue(_ context.Context, owner, repository, title, body string, _ []string) (*github.CreatedIssue, error) {
	f.calls++
	f.owner, f.repo, f.title, f.body = owner, repository, title, body
	return f.result, f.err
}

// handoffHarness runs HandOffIncidentToSpecialist against a stub repository
// that applies the real plan to snap, and records the follow-up note.
func handoffHarness(t *testing.T, snap repository.SpecialistHandoffSnapshot, issues *fakeHandoffIssues, req domain.HandOffIncidentToSpecialistRequest) (domain.HandOffIncidentToSpecialistResponse, error, *repository.SpecialistHandoffPlan, []string) {
	t.Helper()
	var applied *repository.SpecialistHandoffPlan
	var notes []string
	repo := &stubIncidentRepo{
		applySpecialistHandoff: func(_ context.Context, id, actor string, plan func(repository.SpecialistHandoffSnapshot) (repository.SpecialistHandoffPlan, error)) (repository.SpecialistHandoffWritten, error) {
			if id != hoIncidentID || actor != "jane.doe@example.com" {
				t.Errorf("ApplySpecialistHandoff(%s, %s)", id, actor)
			}
			p, err := plan(snap)
			if err != nil {
				return repository.SpecialistHandoffWritten{}, err
			}
			applied = &p
			return repository.SpecialistHandoffWritten{Before: snap, GroupName: "Choreo Special Ops", TaskID: "33333333-3333-4333-8333-333333333333", TaskNumber: "CS-PORTAL-000123"}, nil
		},
		createIncidentComment: func(_ context.Context, _ string, ct domain.CommentType, content, _ string) (domain.CaseComment, error) {
			if ct != domain.CommentTypeWorkNote {
				t.Errorf("follow-up note type %s, want work note", ct)
			}
			notes = append(notes, content)
			return domain.CaseComment{}, nil
		},
		getIncidentByID: func(_ context.Context, id string) (domain.IncidentView, error) {
			return domain.IncidentView{ID: &id, AssignmentGroup: &domain.EntityRef{ID: hoChoreoSpecial, Name: "Choreo Special Ops"}}, nil
		},
	}
	svc := NewIncidentService(repo, nil)
	if issues != nil {
		svc = WithHandoffIssueCreator(svc, issues)
	}
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	resp, err := svc.HandOffIncidentToSpecialist(ctx, req)
	return resp, err, applied, notes
}

func TestHandOffIncidentToSpecialist_FilesIssueAndNotes(t *testing.T) {
	issues := &fakeHandoffIssues{result: &github.CreatedIssue{Number: 42, HTMLURL: "https://github.com/wso2-enterprise/choreo/issues/42"}}
	req := domain.HandOffIncidentToSpecialistRequest{IncidentID: hoIncidentID, ReasonCode: domain.IncidentSpecialistHandoffReasonNoRunbook}
	resp, err, applied, notes := handoffHarness(t, hoSnapshot(hoChoreoService, "44444444-4444-4444-8444-444444444444", "IN_PROGRESS"), issues, req)
	if err != nil {
		t.Fatalf("handoff: %v", err)
	}
	if applied == nil || applied.GroupID != hoChoreoSpecial {
		t.Fatalf("applied plan %+v", applied)
	}
	if issues.calls != 1 || issues.owner != "wso2-enterprise" || issues.repo != "choreo" || issues.title != "Gateway 502s" || issues.body != "All gateways return 502." {
		t.Errorf("CreateIssue(%s/%s, %q, %q) x%d", issues.owner, issues.repo, issues.title, issues.body, issues.calls)
	}
	want := "Escalated to Special Ops team. Escalated by jane.doe@example.com(jane.doe@example.com) Opened an internal issue. Please access the ticket using the link https://github.com/wso2-enterprise/choreo/issues/42 to add more details to the ticket if needed"
	if len(notes) != 1 || notes[0] != want {
		t.Errorf("follow-up notes %q, want [%q]", notes, want)
	}
	h := resp.Handoff
	if h.GithubIssue == nil || h.GithubIssue.Number != 42 || h.GithubIssue.Repo != "choreo" || h.GithubIssueError != nil {
		t.Errorf("github result %+v / %v", h.GithubIssue, h.GithubIssueError)
	}
	if h.AssignmentGroup.ID != hoChoreoSpecial || h.PreviousAssignmentGroup == nil || h.PreviousAssignmentGroup.Name != "Choreo Operations" {
		t.Errorf("groups %+v / %+v", h.AssignmentGroup, h.PreviousAssignmentGroup)
	}
	if h.Task.Number != "CS-PORTAL-000123" || h.Task.Subject != "[Runbook Task] No entry available for INC0099001" {
		t.Errorf("task %+v", h.Task)
	}
	if h.ReasonDescription != "Runbook is not available" || h.Incident.ID == nil {
		t.Errorf("reason %q incident %v", h.ReasonDescription, h.Incident.ID)
	}
}

func TestHandOffIncidentToSpecialist_GithubOutcomes(t *testing.T) {
	short := "Escalated to Special Ops team. Escalated by jane.doe@example.com(jane.doe@example.com)"
	off := false
	cases := []struct {
		name      string
		issues    *fakeHandoffIssues
		create    *bool
		wantCalls int
		wantError string
	}{
		{"not configured", nil, nil, 0, "GitHub issue creation is not configured on this deployment"},
		{"github fails", &fakeHandoffIssues{err: errors.New("403 forbidden")}, nil, 1, "GitHub issue creation failed: 403 forbidden"},
		{"not requested", &fakeHandoffIssues{}, &off, 0, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := domain.HandOffIncidentToSpecialistRequest{IncidentID: hoIncidentID, ReasonCode: domain.IncidentSpecialistHandoffReasonNoRunbook, CreateGithubIssue: c.create}
			resp, err, _, notes := handoffHarness(t, hoSnapshot(hoAsgardeoService, "", "IN_PROGRESS"), c.issues, req)
			if err != nil {
				t.Fatalf("handoff must succeed whatever GitHub does: %v", err)
			}
			if c.issues != nil && c.issues.calls != c.wantCalls {
				t.Errorf("CreateIssue calls %d, want %d", c.issues.calls, c.wantCalls)
			}
			gotErr := ""
			if resp.Handoff.GithubIssueError != nil {
				gotErr = *resp.Handoff.GithubIssueError
			}
			if gotErr != c.wantError || resp.Handoff.GithubIssue != nil {
				t.Errorf("githubIssueError %q issue %+v, want %q and none", gotErr, resp.Handoff.GithubIssue, c.wantError)
			}
			if len(notes) != 1 || notes[0] != short {
				t.Errorf("follow-up notes %q, want [%q]", notes, short)
			}
		})
	}
}

func TestHandOffIncidentToSpecialist_RejectsBadRequestBeforeWriting(t *testing.T) {
	svc := NewIncidentService(&stubIncidentRepo{}, nil) // ApplySpecialistHandoff panics if reached
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	for name, req := range map[string]domain.HandOffIncidentToSpecialistRequest{
		"no reason":  {IncidentID: hoIncidentID},
		"bad reason": {IncidentID: hoIncidentID, ReasonCode: "because"},
		"blank team": {IncidentID: hoIncidentID, ReasonCode: domain.IncidentSpecialistHandoffReasonNoRunbook, EscalationTeam: hoTeam(" ")},
		"long team":  {IncidentID: hoIncidentID, ReasonCode: domain.IncidentSpecialistHandoffReasonNoRunbook, EscalationTeam: hoTeam(domain.IncidentSpecialistHandoffEscalationTeam(strings.Repeat("x", 65)))},
		"bad id":     {IncidentID: "nope", ReasonCode: domain.IncidentSpecialistHandoffReasonNoRunbook},
	} {
		_, err := svc.HandOffIncidentToSpecialist(ctx, req)
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) {
			t.Errorf("%s: got %T %v, want ValidationError", name, err, err)
		}
	}
	if _, err := svc.HandOffIncidentToSpecialist(context.Background(), domain.HandOffIncidentToSpecialistRequest{IncidentID: hoIncidentID, ReasonCode: domain.IncidentSpecialistHandoffReasonNoRunbook}); err == nil || !strings.Contains(err.Error(), "x-user-id-token") {
		t.Errorf("missing token: %v, want an x-user-id-token error", err)
	}
}

func TestHandOffIncidentToSpecialist_RouteWithoutRepoFilesNoIssue(t *testing.T) {
	snap := hoSnapshot(hoAsgardeoService, "", "IN_PROGRESS")
	snap.Routes[0].GithubRepo = nil
	issues := &fakeHandoffIssues{}
	req := domain.HandOffIncidentToSpecialistRequest{IncidentID: hoIncidentID, ReasonCode: domain.IncidentSpecialistHandoffReasonNoRunbook}
	resp, err, _, _ := handoffHarness(t, snap, issues, req)
	if err != nil {
		t.Fatalf("handoff: %v", err)
	}
	if issues.calls != 0 || resp.Handoff.GithubIssueError == nil || *resp.Handoff.GithubIssueError != "No GitHub repository is configured for this specialist route" {
		t.Errorf("calls %d error %v", issues.calls, resp.Handoff.GithubIssueError)
	}
}

func TestListSpecialistHandoffTeams(t *testing.T) {
	want := []domain.SpecialistHandoffTeam{{Key: "choreo-apim-team", Label: "Choreo APIM Team"}}
	svc := NewIncidentService(&stubIncidentRepo{specialistHandoffTeams: want}, nil)
	got, err := svc.ListSpecialistHandoffTeams(context.Background())
	if err != nil || len(got.Teams) != 1 || got.Teams[0] != want[0] {
		t.Errorf("Postgres teams %+v err %v, want %+v", got.Teams, err, want)
	}
	sn, _ := (&snIncidentService{}).ListSpecialistHandoffTeams(context.Background())
	if len(sn.Teams) != 2 {
		t.Errorf("ServiceNow teams %+v, want the two its API accepts", sn.Teams)
	}
}
