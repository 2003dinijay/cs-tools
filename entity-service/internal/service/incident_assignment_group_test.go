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
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

const (
	testSupportGroup  = "5aaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	testExplicitGroup = "5bbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
)

func strPtrGroup(s string) *string { return &s }

// createCapturing returns a stub repo with the given support groups that
// records the request the native create receives.
func createCapturing(groups map[string]string, got *domain.CreateIncidentRequest) *stubIncidentRepo {
	return &stubIncidentRepo{
		supportGroups: groups,
		createIncident: func(_ context.Context, req domain.CreateIncidentRequest, _ string, _ *string, _ string) (domain.CreateIncidentResponse, error) {
			*got = req
			resp := domain.CreateIncidentResponse{}
			resp.Incident.ID = "66666666-6666-6666-6666-666666666666"
			return resp, nil
		},
	}
}

func userCtx() context.Context {
	return auth.WithIdentity(context.Background(), auth.Identity{Validated: true, UserEmail: "jane.doe@example.com"})
}

// *** ONE CALL CARRIES EVERYTHING. *** A caller that names only the service --
// an alert-born incident, any M2M client -- gets the service's support group
// as the assignment group, without looking it up itself.
func TestCreateIncident_DerivesAssignmentGroupFromService(t *testing.T) {
	var got domain.CreateIncidentRequest
	req := validCreateIncidentRequest()
	svc := NewIncidentService(createCapturing(map[string]string{req.ServiceID: testSupportGroup}, &got), &mockEventPublisher{})

	if _, err := svc.CreateIncident(userCtx(), req); err != nil {
		t.Fatalf("CreateIncident: %v", err)
	}
	if got.AssignmentGroupID == nil || *got.AssignmentGroupID != testSupportGroup {
		t.Errorf("assignmentGroupId = %v, want the service's support group %s", got.AssignmentGroupID, testSupportGroup)
	}
}

// A caller that knows better -- an alarm naming its own team -- is not
// overridden.
func TestCreateIncident_ExplicitAssignmentGroupWins(t *testing.T) {
	var got domain.CreateIncidentRequest
	req := validCreateIncidentRequest()
	req.AssignmentGroupID = strPtrGroup(testExplicitGroup)
	svc := NewIncidentService(createCapturing(map[string]string{req.ServiceID: testSupportGroup}, &got), &mockEventPublisher{})

	if _, err := svc.CreateIncident(userCtx(), req); err != nil {
		t.Fatalf("CreateIncident: %v", err)
	}
	if got.AssignmentGroupID == nil || *got.AssignmentGroupID != testExplicitGroup {
		t.Errorf("assignmentGroupId = %v, want the explicitly sent %s", got.AssignmentGroupID, testExplicitGroup)
	}
}

// A service with no support group leaves the incident unassigned, as before.
func TestCreateIncident_ServiceWithoutSupportGroupStaysUnassigned(t *testing.T) {
	var got domain.CreateIncidentRequest
	svc := NewIncidentService(createCapturing(nil, &got), &mockEventPublisher{})

	if _, err := svc.CreateIncident(userCtx(), validCreateIncidentRequest()); err != nil {
		t.Fatalf("CreateIncident: %v", err)
	}
	if got.AssignmentGroupID != nil {
		t.Errorf("assignmentGroupId = %v, want none", *got.AssignmentGroupID)
	}
}

// In dual-write mode the derived group must reach ServiceNow too, so both
// systems agree from the first write.
func TestCreateIncident_DualWriteSendsTheDerivedGroupToServiceNow(t *testing.T) {
	req := validCreateIncidentRequest()
	var toSN domain.CreateIncidentRequest
	mirror := &stubMirrorIncidentService{
		createIncident: func(_ context.Context, r domain.CreateIncidentRequest) (domain.CreateIncidentResponse, error) {
			toSN = r
			resp := domain.CreateIncidentResponse{}
			resp.Incident.ID, resp.Incident.Number, resp.Incident.CreatedBy = "77777777-7777-7777-7777-777777777777", "INC0000001", "jane.doe@example.com"
			return resp, nil
		},
	}
	var toPG domain.CreateIncidentRequest
	repo := &stubIncidentRepo{
		supportGroups: map[string]string{req.ServiceID: testSupportGroup},
		createIncidentFromServiceNow: func(_ context.Context, r domain.CreateIncidentRequest, id, _, _ string) (domain.CreateIncidentResponse, error) {
			toPG = r
			resp := domain.CreateIncidentResponse{}
			resp.Incident.ID = id
			return resp, nil
		},
	}
	svc := NewIncidentServiceWithSNMirror(repo, nil, mirror, nil, nil)

	if _, err := svc.CreateIncident(userCtx(), req); err != nil {
		t.Fatalf("CreateIncident: %v", err)
	}
	for name, r := range map[string]domain.CreateIncidentRequest{"ServiceNow": toSN, "Postgres": toPG} {
		if r.AssignmentGroupID == nil || *r.AssignmentGroupID != testSupportGroup {
			t.Errorf("%s got assignmentGroupId %v, want %s", name, r.AssignmentGroupID, testSupportGroup)
		}
	}
}
