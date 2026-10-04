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
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/events"
)

// pgCreatingRepo records what DATA_SOURCE=postgres writes when it creates an incident itself.
type pgCreatingRepo struct {
	stubIncidentRepo
	createdBy string
	createReq domain.CreateIncidentRequest
	comments  []domain.CommentType
	notes     []string
	createErr error
}

func newPGCreatingRepo() *pgCreatingRepo {
	r := &pgCreatingRepo{}
	r.createIncident = func(_ context.Context, req domain.CreateIncidentRequest, createdBy string) (domain.CreateIncidentResponse, error) {
		if r.createErr != nil {
			return domain.CreateIncidentResponse{}, r.createErr
		}
		r.createdBy = createdBy
		r.createReq = req
		var resp domain.CreateIncidentResponse
		resp.Incident.ID = "66666666-6666-6666-6666-666666666666"
		resp.Incident.Number = "INC0000001"
		return resp, nil
	}
	r.createIncidentComment = func(_ context.Context, _ string, kind domain.CommentType, content, _ string) (domain.CaseComment, error) {
		r.comments = append(r.comments, kind)
		r.notes = append(r.notes, content)
		return domain.CaseComment{}, nil
	}
	r.getIncidentByID = func(_ context.Context, id string) (domain.IncidentView, error) { return newTestIncidentView(id), nil }
	return r
}

func alertIncidentRequest() domain.CreateIncidentRequest {
	notes := "AlarmName: prod-rds-cpu-utilization-high"
	group := "aaaaaaaa-0000-4000-8000-000000000001"
	return domain.CreateIncidentRequest{
		CallerID: "11111111-1111-1111-1111-111111111111", Category: "SERVICE_INTERRUPTION",
		ServiceID: "22222222-2222-2222-2222-222222222222", Impact: "HIGH", Urgency: "HIGH",
		Subject: "prod-rds-cpu-utilization-high", WorkNotes: &notes, AssignmentGroupID: &group,
	}
}

// An alert-born incident arrives from a service (sre-alert-core-service through csm-integration-service)
// with no end-user token. DATA_SOURCE=postgres creates it as the system actor, hands the alert to the
// repository's create (which saves it as the first work note in the same transaction as the incident),
// and publishes incident.created -- what the SRE escalation ladder starts from.
func TestIncidentService_PostgresCreatesAServiceCallersIncident(t *testing.T) {
	repo := newPGCreatingRepo()
	publisher := &mockEventPublisher{}
	svc := NewIncidentServiceWithPublisher(repo, nil, publisher)

	resp, err := svc.CreateIncident(context.Background(), alertIncidentRequest())
	if err != nil {
		t.Fatalf("CreateIncident: %v", err)
	}
	if resp.Incident.Number != "INC0000001" {
		t.Errorf("number = %q", resp.Incident.Number)
	}
	if repo.createdBy != incidentSystemActorEmail {
		t.Errorf("createdBy = %q, want the system actor %q", repo.createdBy, incidentSystemActorEmail)
	}
	if repo.createReq.WorkNotes == nil || *repo.createReq.WorkNotes != "AlarmName: prod-rds-cpu-utilization-high" {
		t.Errorf("create carried work notes %v, want the alert, saved with the incident", repo.createReq.WorkNotes)
	}
	if len(repo.comments) != 0 {
		t.Errorf("service wrote %d separate comment(s); the notes belong in the create's own transaction", len(repo.comments))
	}
	if len(publisher.calls) != 1 || publisher.calls[0].eventType != events.TypeIncidentCreated ||
		publisher.calls[0].entityID != "66666666-6666-6666-6666-666666666666" {
		t.Fatalf("published %+v, want one incident.created for the new incident", publisher.calls)
	}
}

// A portal user's token still names them as the creator.
func TestIncidentService_PostgresCreateNamesAUserCaller(t *testing.T) {
	repo := newPGCreatingRepo()
	svc := NewIncidentServiceWithPublisher(repo, nil, &mockEventPublisher{})
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	if _, err := svc.CreateIncident(ctx, alertIncidentRequest()); err != nil {
		t.Fatal(err)
	}
	if repo.createdBy != "jane.doe@example.com" {
		t.Errorf("createdBy = %q, want the token's email", repo.createdBy)
	}
}

// Nothing is published, and no note written, for an incident that was never created.
func TestIncidentService_PostgresCreateFailurePublishesNothing(t *testing.T) {
	repo := newPGCreatingRepo()
	repo.createErr = errors.New("insert failed")
	publisher := &mockEventPublisher{}
	svc := NewIncidentServiceWithPublisher(repo, nil, publisher)
	if _, err := svc.CreateIncident(context.Background(), alertIncidentRequest()); err == nil {
		t.Fatal("CreateIncident succeeded, want the insert error")
	}
	if len(publisher.calls) != 0 || len(repo.comments) != 0 {
		t.Errorf("published %d, commented %d; want neither", len(publisher.calls), len(repo.comments))
	}
}

// sre-alert-core-service pushes each later alert as a work note (PATCH /incidents/{id}); with no
// ServiceNow behind it, the note is written and nothing is mirrored.
func TestIncidentService_PostgresUpdateWritesWorkNotes(t *testing.T) {
	repo := newPGCreatingRepo()
	svc := NewIncidentServiceWithPublisher(repo, nil, &mockEventPublisher{})
	note := "Duplicate alert ALT000000002"
	resp, err := svc.UpdateIncident(context.Background(), domain.UpdateIncidentRequest{ID: testDeploymentUUID, WorkNotes: &note})
	if err != nil {
		t.Fatalf("UpdateIncident: %v", err)
	}
	if resp.Incident.ID == nil || len(repo.comments) != 1 || repo.comments[0] != domain.CommentTypeWorkNote {
		t.Errorf("comments = %v, want one work note", repo.comments)
	}
}
