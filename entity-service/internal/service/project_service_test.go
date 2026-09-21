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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package service

import (
	"context"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// recordingProjectRepo is a repository.ProjectRepository whose SearchProjects
// records the request it was called with (or panics if it's never expected
// to be called at all) — used to prove SearchProjects's exclude-filter guard
// decides what reaches the repository, without needing a real Postgres
// connection.
type recordingProjectRepo struct {
	called  bool
	gotReq  domain.SearchProjectsRequest
	mustNot bool
}

func (r *recordingProjectRepo) SearchProjects(_ context.Context, req domain.SearchProjectsRequest, _ repository.SearchScope) ([]domain.Project, int, error) {
	if r.mustNot {
		panic("SearchProjects: repository must not be called for a rejected request")
	}
	r.called = true
	r.gotReq = req
	return nil, 0, nil
}

func (r *recordingProjectRepo) GetProjectByID(context.Context, string, repository.SearchScope) (domain.ProjectDetailsView, error) {
	panic("GetProjectByID: not exercised by these tests")
}

// TestSearchProjectsExcludeSubscriptionTypesRejected locks in that
// ExcludeSubscriptionTypes alone is still rejected for the Postgres data
// source: the project table has no subscription-type column at all (unlike
// key/wso2_closure_state, which back ExcludeProjectKeys/ExcludeClosureStates
// below), so there is nothing to filter on.
func TestSearchProjectsExcludeSubscriptionTypesRejected(t *testing.T) {
	repo := &recordingProjectRepo{mustNot: true}
	svc := NewProjectService(repo, alwaysUnrestrictedAccess{})

	_, err := svc.SearchProjects(t.Context(), domain.SearchProjectsRequest{
		ExcludeSubscriptionTypes: []domain.SubscriptionType{domain.SubscriptionTypeCloudSupport},
	})

	var valErr *apierror.ValidationError
	if err == nil {
		t.Fatal("expected a ValidationError, got nil")
	}
	if !isValidationError(err, &valErr) {
		t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
	}
}

// TestSearchProjectsExcludeClosureStatesAndProjectKeysPassThrough locks in
// the opposite: ExcludeClosureStates and ExcludeProjectKeys (individually,
// and together) are NOT rejected — they map onto real columns
// (wso2_closure_state, key) and reach the repository unchanged, for
// ProjectRepository.SearchProjects to apply as SQL filters.
func TestSearchProjectsExcludeClosureStatesAndProjectKeysPassThrough(t *testing.T) {
	tests := []struct {
		name    string
		req     domain.SearchProjectsRequest
		wantErr bool
	}{
		{
			name: "excludeClosureStates alone",
			req:  domain.SearchProjectsRequest{ExcludeClosureStates: []string{"Restricted", "Suspended"}},
		},
		{
			name: "excludeProjectKeys alone",
			req:  domain.SearchProjectsRequest{ExcludeProjectKeys: []string{"APEXIA"}},
		},
		{
			name: "both together",
			req: domain.SearchProjectsRequest{
				ExcludeClosureStates: []string{"Restricted"},
				ExcludeProjectKeys:   []string{"APEXIA"},
			},
		},
		{
			name: "neither set",
			req:  domain.SearchProjectsRequest{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &recordingProjectRepo{}
			svc := NewProjectService(repo, alwaysUnrestrictedAccess{})

			_, err := svc.SearchProjects(t.Context(), tt.req)
			if err != nil {
				t.Fatalf("SearchProjects(%+v) unexpected error: %v", tt.req, err)
			}
			if !repo.called {
				t.Fatal("expected the repository to be called, it wasn't")
			}
			if len(repo.gotReq.ExcludeClosureStates) != len(tt.req.ExcludeClosureStates) ||
				len(repo.gotReq.ExcludeProjectKeys) != len(tt.req.ExcludeProjectKeys) {
				t.Fatalf("repository received %+v, want the same exclude filters as %+v", repo.gotReq, tt.req)
			}
		})
	}
}
