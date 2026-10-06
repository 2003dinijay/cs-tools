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

// handoffRouting is one service's specialist routing.
type handoffRouting struct {
	defaultGroup string
	teams        map[domain.IncidentSpecialistHandoffEscalationTeam]string
	githubOwner  string
	githubRepo   string
}

// handoffRoutingByService is IncidentHandoffUtils' IHU_SERVICE_ROUTING, keyed
// by the service's Postgres id (its ServiceNow sys_id). The UI action also
// branches on eleven further Choreo-family services, but its display
// condition (WSO2AgentWorkspaceUtils.canEscalateToSpecialOps) never offers
// the button for them, so -- like IncidentHandoffUtils -- they are not here.
var handoffRoutingByService = map[string]handoffRouting{
	// Choreo
	"b9c999f8-1b86-a010-00ae-86acdd4bcb61": {
		defaultGroup: "fe0d8868-1b0b-3010-d64e-64a2604bcb3c", // Choreo Special Ops
		teams: map[domain.IncidentSpecialistHandoffEscalationTeam]string{
			domain.IncidentSpecialistHandoffTeamChoreoRuntime: "80dade5d-1b70-0710-a002-c9d3604bcbd7", // Choreo Runtime Special Ops
			domain.IncidentSpecialistHandoffTeamChoreoAPIM:    "a79a1e9d-1b70-0710-a002-c9d3604bcb20", // Choreo APIM Special Ops
		},
		githubOwner: "wso2-enterprise",
		githubRepo:  "choreo",
	},
	// Asgardeo
	"97ed1b8b-1ba2-6c10-00ae-86acdd4bcbd3": {
		defaultGroup: "7fb4f4c6-1b4b-3810-aea4-a936604bcb90", // Asgardeo Special Ops
		githubOwner:  "wso2-enterprise",
		githubRepo:   "asgardeo-product",
	},
}

// handoffRunbookTaskGroup is the runbook task's group, "WSO2 SRE Team" --
// the sys_id the UI action hard-codes.
const handoffRunbookTaskGroup = "f991f369-1b88-b410-cb68-98aebd4bcb13"

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
// writes handOff makes, decided on the locked incident.
func planSpecialistHandoff(req domain.HandOffIncidentToSpecialistRequest, snap repository.SpecialistHandoffSnapshot) (repository.SpecialistHandoffPlan, handoffRouting, error) {
	var routing handoffRouting
	ok := false
	if snap.ServiceID != nil {
		routing, ok = handoffRoutingByService[strings.ToLower(*snap.ServiceID)]
	}
	if !ok {
		return repository.SpecialistHandoffPlan{}, routing, specialistHandoffConflict("No specialist group is configured for this incident's service.")
	}
	if snap.State != string(domain.IncidentStateInProgress) {
		return repository.SpecialistHandoffPlan{}, routing, specialistHandoffConflict("Only an In Progress incident can be handed off.")
	}
	if snap.AssignmentGroupID != nil && strings.EqualFold(*snap.AssignmentGroupID, routing.defaultGroup) {
		return repository.SpecialistHandoffPlan{}, routing, specialistHandoffConflict("The incident already sits with the specialist group for this service.")
	}

	group := routing.defaultGroup
	if req.EscalationTeam != nil {
		if g, ok := routing.teams[*req.EscalationTeam]; ok {
			group = g
		}
	}
	reason := handoffReasons[req.ReasonCode]
	blob, err := handoffReasonBlob(req, reason.description)
	if err != nil {
		return repository.SpecialistHandoffPlan{}, routing, err
	}
	return repository.SpecialistHandoffPlan{
		GroupID:     group,
		TaskSubject: reason.taskSubject(snap.Number),
		TaskGroupID: handoffRunbookTaskGroup,
		WorkNotes:   []string{blob},
	}, routing, nil
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
	email, label, err := s.handoffActor(ctx)
	if err != nil {
		return domain.HandOffIncidentToSpecialistResponse{}, err
	}

	var routing handoffRouting
	written, err := s.repo.ApplySpecialistHandoff(ctx, req.IncidentID, email,
		func(snap repository.SpecialistHandoffSnapshot) (repository.SpecialistHandoffPlan, error) {
			plan, r, perr := planSpecialistHandoff(req, snap)
			routing = r
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
		result.GithubIssue, result.GithubIssueError = s.fileHandoffIssue(ctx, routing, written.Before)
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

// fileHandoffIssue opens the internal GitHub issue, titled and bodied with
// the incident's subject and description. It never fails the handoff: a
// missing client or a GitHub error comes back as the error text.
func (s *incidentService) fileHandoffIssue(ctx context.Context, routing handoffRouting, inc repository.SpecialistHandoffSnapshot) (*domain.IncidentSpecialistHandoffGithubIssue, *string) {
	if s.handoffIssues == nil {
		msg := "GitHub issue creation is not configured on this deployment"
		return nil, &msg
	}
	created, err := s.handoffIssues.CreateIssue(ctx, routing.githubOwner, routing.githubRepo, inc.Subject, derefString(inc.Description), nil)
	if err != nil {
		slog.WarnContext(ctx, "specialist handoff: GitHub issue not created", "incidentId", inc.IncidentID, "error", err)
		msg := "GitHub issue creation failed: " + err.Error()
		return nil, &msg
	}
	return &domain.IncidentSpecialistHandoffGithubIssue{URL: created.HTMLURL, Number: created.Number, Repo: routing.githubRepo}, nil
}
