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

package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/servicenow"
)

type mockSplProjectClient struct {
	getProjectsFn        func(ctx context.Context, phrase *string, offset, limit int) ([]servicenow.ProjectDetails, error)
	getProjectByIDFn     func(ctx context.Context, projectID string) (servicenow.ProjectDetails, error)
	getProjectContactsFn func(ctx context.Context, projectID string, offset, limit int) ([]servicenow.Contact, error)
	getCasesByProjectFn  func(ctx context.Context, projectID string, stateFilters, caseTypeFilters []string, offset, limit int) ([]servicenow.CaseDetails, error)
}

func (m *mockSplProjectClient) GetProjects(ctx context.Context, phrase *string, offset, limit int) ([]servicenow.ProjectDetails, error) {
	return m.getProjectsFn(ctx, phrase, offset, limit)
}
func (m *mockSplProjectClient) GetProjectByID(ctx context.Context, projectID string) (servicenow.ProjectDetails, error) {
	return m.getProjectByIDFn(ctx, projectID)
}
func (m *mockSplProjectClient) GetProjectContacts(ctx context.Context, projectID string, offset, limit int) ([]servicenow.Contact, error) {
	return m.getProjectContactsFn(ctx, projectID, offset, limit)
}
func (m *mockSplProjectClient) GetCasesByProject(ctx context.Context, projectID string, stateFilters, caseTypeFilters []string, offset, limit int) ([]servicenow.CaseDetails, error) {
	return m.getCasesByProjectFn(ctx, projectID, stateFilters, caseTypeFilters, offset, limit)
}

func TestSplGetProjectByID_NotFound(t *testing.T) {
	client := &mockSplProjectClient{
		getProjectByIDFn: func(_ context.Context, _ string) (servicenow.ProjectDetails, error) {
			return servicenow.ProjectDetails{}, servicenow.ErrProjectByIDNotFound
		},
	}
	h := NewSplProjectHandler(client, splAllowedGroups)

	r := withUser(httptest.NewRequest(http.MethodGet, "/spl/projects/PRJ404", nil))
	r.SetPathValue("projectId", "PRJ404")
	w := httptest.NewRecorder()
	h.GetProjectByID(w, r)
	assertStatus(t, w, http.StatusNotFound)
}

func TestSplGetProjectCases_ParsesRepeatedAndCommaSeparatedFilters(t *testing.T) {
	var capturedStates, capturedTypes []string
	client := &mockSplProjectClient{
		getCasesByProjectFn: func(_ context.Context, _ string, stateFilters, caseTypeFilters []string, _, _ int) ([]servicenow.CaseDetails, error) {
			capturedStates = stateFilters
			capturedTypes = caseTypeFilters
			return nil, nil
		},
	}
	h := NewSplProjectHandler(client, splAllowedGroups)

	r := withUser(httptest.NewRequest(http.MethodGet, "/spl/projects/PRJ1/cases?offset=0&limit=10&stateFilters=Open,Closed&caseTypeFilters=Bug", nil))
	r.SetPathValue("projectId", "PRJ1")
	w := httptest.NewRecorder()
	h.GetProjectCases(w, r)

	assertStatus(t, w, http.StatusOK)
	if len(capturedStates) != 2 || capturedStates[0] != "Open" || capturedStates[1] != "Closed" {
		t.Errorf("stateFilters = %v, want [Open Closed]", capturedStates)
	}
	if len(capturedTypes) != 1 || capturedTypes[0] != "Bug" {
		t.Errorf("caseTypeFilters = %v, want [Bug]", capturedTypes)
	}
}

func TestSplGetProjects_RequiresGroupMembership(t *testing.T) {
	h := NewSplProjectHandler(&mockSplProjectClient{}, []string{"sales-team"})
	r := withUser(httptest.NewRequest(http.MethodGet, "/spl/projects?offset=0&limit=10", nil))
	w := httptest.NewRecorder()
	h.GetProjects(w, r)
	assertStatus(t, w, http.StatusForbidden)
}
