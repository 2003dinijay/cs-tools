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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/github"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// The specialist handoff on Postgres ports ServiceNow's
// x_wso2_customer_0.IncidentHandoffUtils.handOff, which mirrors the
// "Escalate to Special Ops" UI action (sys_ui_action
// 1b8e482c1b0b3010d64e64a2604bcb51). It writes Postgres only; ServiceNow is
// never called. Discovery script 68 (csm-flow-service docs) has the UI
// action and its history.

// The routing -- which team, and so which group, a service's incidents are
// handed to, by sub-team, and where the GitHub issue goes -- is data: the
// Special Ops teams are team rows linked to their group (team.group_id), and
// specialist_handoff_route (migration 0194) says which serve which service,
// seeded with IncidentHandoffUtils' IHU_SERVICE_ROUTING. ServiceNow
// hard-codes it.

// maxEscalationTeamLen is team.key's length.
const maxEscalationTeamLen = 64

// handoffReasons are the two reasons the UI action's modal offers: the
// description written into the reason note, and the runbook task's subject.
var handoffReasons = map[domain.IncidentSpecialistHandoffReasonCode]struct {
	description string
	taskSubject func(number string) string
}{
	domain.IncidentSpecialistHandoffReasonNoRunbook: {
		description: "Runbook is not available",
		taskSubject: func(n string) string { return "[Runbook Task] No entry available for " + n },
	},
	domain.IncidentSpecialistHandoffReasonRunbookNotWorking: {
		description: "Runbook doesn't solve the incident",
		taskSubject: func(n string) string { return "[Runbook Task] Entry didn't solve the incident " + n },
	},
}

// handoffIssueCreator is the slice of the GitHub client a handoff needs.
type handoffIssueCreator interface {
	CreateIssue(ctx context.Context, owner, repository, title, body string, labels []string) (*github.CreatedIssue, error)
}

// WithHandoffIssueCreator gives a Postgres-backed IncidentService the GitHub
// client its specialist handoffs file issues with. A nil client, or a
// ServiceNow-backed service, is left as it is.
func WithHandoffIssueCreator(svc IncidentService, client handoffIssueCreator) IncidentService {
	if pg, ok := svc.(*incidentService); ok && client != nil {
		pg.handoffIssues = client
	}
	return svc
}

// specialistHandoffConflict is IncidentHandoffUtils' 409 for an incident the
// handoff cannot take.
func specialistHandoffConflict(detail string) error {
	return &apierror.ConflictError{Msg: "Incident is not eligible for a specialist handoff: " + detail}
}

// planSpecialistHandoff is IncidentHandoffUtils.checkEligibility plus the
// writes handOff makes, decided on the locked incident. route is the
// specialist_handoff_route the incident goes to: the requested sub-team's,
// or the service's default when the service has no such sub-team (as
// ServiceNow ignores a team for Asgardeo).
//
// The runbook task goes to the same Special Ops group as the incident.
// ServiceNow sends it to WSO2 SRE Team, which no longer exists; the Special
// Ops team now owns its runbooks.
func planSpecialistHandoff(req domain.HandOffIncidentToSpecialistRequest, snap repository.SpecialistHandoffSnapshot) (repository.SpecialistHandoffPlan, repository.SpecialistHandoffRoute, error) {
	var def, route *repository.SpecialistHandoffRoute
	for i := range snap.Routes {
		r := &snap.Routes[i]
		switch {
		case r.IsDefault:
			def = r
		case req.EscalationTeam != nil && r.TeamKey == string(*req.EscalationTeam):
			route = r
		}
	}
	if def == nil {
		return repository.SpecialistHandoffPlan{}, repository.SpecialistHandoffRoute{}, specialistHandoffConflict("No specialist group is configured for this incident's service.")
	}
	if snap.State != string(domain.IncidentStateInProgress) {
		return repository.SpecialistHandoffPlan{}, *def, specialistHandoffConflict("Only an In Progress incident can be handed off.")
	}
	if def.GroupID != nil && snap.AssignmentGroupID != nil && strings.EqualFold(*snap.AssignmentGroupID, *def.GroupID) {
		return repository.SpecialistHandoffPlan{}, *def, specialistHandoffConflict("The incident already sits with the specialist group for this service.")
	}
	if route == nil {
		route = def
	}
	if route.GroupID == nil {
		return repository.SpecialistHandoffPlan{}, *route, specialistHandoffConflict("The " + route.TeamName + " team has no assignment group configured.")
	}

	reason := handoffReasons[req.ReasonCode]
	blob, err := handoffReasonBlob(req, reason.description)
	if err != nil {
		return repository.SpecialistHandoffPlan{}, *route, err
	}
	return repository.SpecialistHandoffPlan{
		GroupID:     *route.GroupID,
		TaskSubject: reason.taskSubject(snap.Number),
		TaskGroupID: route.GroupID,
		WorkNotes:   []string{blob},
	}, *route, nil
}

// handoffReasonBlob is the reason work note, byte for byte the JSON the UI
// action's modal writes: {"reasonCode":...,"reasonDescription":...,
// "escalationTeam":...}, escalationTeam null when none was picked. HTML
// escaping is off because JavaScript's JSON.stringify does none.
func handoffReasonBlob(req domain.HandOffIncidentToSpecialistRequest, description string) (string, error) {
	var team *string
	if req.EscalationTeam != nil {
		t := string(*req.EscalationTeam)
		team = &t
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(struct {
		ReasonCode        string  `json:"reasonCode"`
		ReasonDescription string  `json:"reasonDescription"`
		EscalationTeam    *string `json:"escalationTeam"`
	}{string(req.ReasonCode), description, team}); err != nil {
		return "", fmt.Errorf("specialist handoff: encode reason: %w", err)
	}
	return strings.TrimSuffix(buf.String(), "\n"), nil
}

// handoffEscalatedNote is the note handOff writes after the GitHub attempt,
// with the issue link when one was opened.
func handoffEscalatedNote(actorLabel string, issue *domain.IncidentSpecialistHandoffGithubIssue) string {
	note := "Escalated to Special Ops team. Escalated by " + actorLabel
	if issue != nil {
		note += " Opened an internal issue. Please access the ticket using the link " + issue.URL +
			" to add more details to the ticket if needed"
	}
	return note
}

// handoffActor is who performs the handoff: their email from the caller's
// token, and the "Name(email)" label the notes carry -- the user's name when
// this database knows them, the email otherwise, as IncidentHandoffUtils'
// _actorLabel does.
func (s *incidentService) handoffActor(ctx context.Context) (email, label string, err error) {
	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		return "", "", &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}
	email, err = emailFromJWT(token)
	if err != nil {
		return "", "", &apierror.ValidationError{Msg: "x-user-id-token: " + err.Error()}
	}
	name := email
	if s.userRepo != nil {
		if u, uerr := s.userRepo.GetUserByEmail(ctx, email); uerr == nil {
			if n := strings.TrimSpace(u.FirstName + " " + u.LastName); n != "" {
				name = n
			}
		}
	}
	return email, name + "(" + email + ")", nil
}

// HandOffIncidentToSpecialist implements IncidentService for Postgres.
//
// In one transaction it applies IncidentHandoffUtils' eligibility rules
// (In Progress; a Choreo or Asgardeo incident; not already with that
// service's specialist group), moves the incident to the specialist group,
// clears its assignee, opens the runbook task and writes the reason note.
// Then, outside the transaction and best effort -- the handoff stands
// whatever GitHub does, as in IncidentHandoffUtils -- it files the internal
// GitHub issue and writes the "Escalated to Special Ops team." note.
func (s *incidentService) HandOffIncidentToSpecialist(ctx context.Context, req domain.HandOffIncidentToSpecialistRequest) (domain.HandOffIncidentToSpecialistResponse, error) {
	if err := validateHandOffRequest(req); err != nil {
		return domain.HandOffIncidentToSpecialistResponse{}, err
	}
	// Any team key a route can hold; one with no route for the incident's
	// service falls back to the service's default route.
	if req.EscalationTeam != nil {
		if t := strings.TrimSpace(string(*req.EscalationTeam)); t == "" || len(t) > maxEscalationTeamLen {
			return domain.HandOffIncidentToSpecialistResponse{}, &apierror.ValidationError{Msg: "invalid escalationTeam: " + string(*req.EscalationTeam)}
		}
	}
	email, label, err := s.handoffActor(ctx)
	if err != nil {
		return domain.HandOffIncidentToSpecialistResponse{}, err
	}

	var route repository.SpecialistHandoffRoute
	written, err := s.repo.ApplySpecialistHandoff(ctx, req.IncidentID, email,
		func(snap repository.SpecialistHandoffSnapshot) (repository.SpecialistHandoffPlan, error) {
			plan, r, perr := planSpecialistHandoff(req, snap)
			route = r
			return plan, perr
		})
	if err != nil {
		return domain.HandOffIncidentToSpecialistResponse{}, err
	}

	result := domain.IncidentSpecialistHandoffResult{
		ReasonCode:        req.ReasonCode,
		ReasonDescription: handoffReasons[req.ReasonCode].description,
		EscalationTeam:    req.EscalationTeam,
		Task: domain.IncidentSpecialistHandoffTask{
			ID: written.TaskID, Number: written.TaskNumber, Subject: handoffReasons[req.ReasonCode].taskSubject(written.Before.Number),
		},
	}
	if written.Before.AssignmentGroupID != nil {
		result.PreviousAssignmentGroup = &domain.EntityRef{ID: *written.Before.AssignmentGroupID, Name: derefString(written.Before.AssignmentGroupName)}
	}

	if req.CreateGithubIssue == nil || *req.CreateGithubIssue {
		result.GithubIssue, result.GithubIssueError = s.fileHandoffIssue(ctx, route, written.Before)
	}
	if _, err := s.repo.CreateIncidentComment(ctx, req.IncidentID, domain.CommentTypeWorkNote, handoffEscalatedNote(label, result.GithubIssue), email); err != nil {
		// The handoff itself is committed; only this note is missing.
		slog.ErrorContext(ctx, "specialist handoff: escalated note not written", "incidentId", req.IncidentID, "error", err)
	}

	view, err := s.repo.GetIncidentByID(ctx, req.IncidentID)
	if err != nil {
		return domain.HandOffIncidentToSpecialistResponse{}, err
	}
	result.Incident = view
	if view.AssignmentGroup != nil {
		result.AssignmentGroup = *view.AssignmentGroup
	}
	return domain.HandOffIncidentToSpecialistResponse{
		Message: "Incident handed off to the specialist group.",
		Handoff: result,
	}, nil
}

// fileHandoffIssue opens the internal GitHub issue in the route's
// repository, titled and bodied with the incident's subject and description.
// It never fails the handoff: a missing client, a route with no repository,
// or a GitHub error comes back as the error text.
func (s *incidentService) fileHandoffIssue(ctx context.Context, route repository.SpecialistHandoffRoute, inc repository.SpecialistHandoffSnapshot) (*domain.IncidentSpecialistHandoffGithubIssue, *string) {
	if s.handoffIssues == nil {
		msg := "GitHub issue creation is not configured on this deployment"
		return nil, &msg
	}
	owner, repo := derefString(route.GithubOwner), derefString(route.GithubRepo)
	if owner == "" || repo == "" {
		msg := "No GitHub repository is configured for this specialist route"
		return nil, &msg
	}
	created, err := s.handoffIssues.CreateIssue(ctx, owner, repo, inc.Subject, derefString(inc.Description), nil)
	if err != nil {
		slog.WarnContext(ctx, "specialist handoff: GitHub issue not created", "incidentId", inc.IncidentID, "error", err)
		msg := "GitHub issue creation failed: " + err.Error()
		return nil, &msg
	}
	return &domain.IncidentSpecialistHandoffGithubIssue{URL: created.HTMLURL, Number: created.Number, Repo: repo}, nil
}

// ListSpecialistHandoffTeams implements IncidentService for Postgres: the
// sub-teams specialist_handoff_route offers.
func (s *incidentService) ListSpecialistHandoffTeams(ctx context.Context) (domain.SpecialistHandoffTeamsResponse, error) {
	teams, err := s.repo.ListSpecialistHandoffTeams(ctx)
	if err != nil {
		return domain.SpecialistHandoffTeamsResponse{}, err
	}
	return domain.SpecialistHandoffTeamsResponse{Teams: teams}, nil
}
