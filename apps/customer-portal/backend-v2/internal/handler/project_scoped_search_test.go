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
	"reflect"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/middleware"
)

const testProjectID = "11111111-1111-1111-1111-111111111111"

// fakeEntityCaseClient records the SearchCasesRequest it was called with.
// entityCaseClient is embedded (nil) so only SearchCases needs implementing;
// calling any other method would nil-panic, which is fine since these tests
// never call one.
type fakeEntityCaseClient struct {
	entityCaseClient
	gotSearchCasesReq       entity.SearchCasesRequest
	gotSearchAttachmentsReq entity.SearchAttachmentsRequest
}

func (f *fakeEntityCaseClient) SearchCases(ctx context.Context, req entity.SearchCasesRequest) (entity.SearchCasesResponse, error) {
	f.gotSearchCasesReq = req
	return entity.SearchCasesResponse{}, nil
}

// fakeEntityDeploymentClient records the SearchDeploymentsRequest it was
// called with, same embedding trick as fakeEntityCaseClient.
type fakeEntityDeploymentClient struct {
	entityDeploymentClient
	gotSearchDeploymentsReq entity.SearchDeploymentsRequest
}

func (f *fakeEntityDeploymentClient) SearchDeployments(ctx context.Context, req entity.SearchDeploymentsRequest) (entity.SearchDeploymentsResponse, error) {
	f.gotSearchDeploymentsReq = req
	return entity.SearchDeploymentsResponse{}, nil
}

func authedRequest(method, url, body string) *http.Request {
	req := httptest.NewRequest(method, url, strings.NewReader(body))
	req = req.WithContext(middleware.WithUserInfo(req.Context(), &middleware.UserInfo{
		Email: "customer@example.com", UserID: "user-1",
	}))
	return req
}

// TestSearchCases_RouteIsProjectScoped is an end-to-end check, through a real
// http.ServeMux registered with the exact same pattern main.go uses
// ("POST /projects/{id}/cases/search"), that a request to that URL reaches
// CaseHandler.SearchCases with r.PathValue("id") populated, and that the
// project ID from the URL — not any request-body field, since
// dto.CaseSearchFilters has no projectIds field at all — ends up as the
// projectId filter sent to entity-service. This guards against the routing
// bug where these endpoints were registered as flat POST /cases/search,
// which the frontend never called (it always sends
// POST /projects/{id}/cases/search) and so 404'd on every request.
func TestSearchCases_RouteIsProjectScoped(t *testing.T) {
	fake := &fakeEntityCaseClient{}
	h := NewCaseHandler(fake)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /projects/{id}/cases/search", h.SearchCases)

	req := authedRequest(http.MethodPost, "/projects/"+testProjectID+"/cases/search",
		`{"filters":{"searchQuery":"login error"},"pagination":{"limit":10,"offset":0}}`)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	want := []entity.CaseFieldFilter{
		{Field: "projectId", Op: "in", Values: []string{testProjectID}},
	}
	if !reflect.DeepEqual(fake.gotSearchCasesReq.Filters.Filters, want) {
		t.Fatalf("entity-service SearchCases filters = %+v, want %+v", fake.gotSearchCasesReq.Filters.Filters, want)
	}
	if fake.gotSearchCasesReq.Filters.SearchQuery != "login error" {
		t.Fatalf("SearchQuery = %q, want %q", fake.gotSearchCasesReq.Filters.SearchQuery, "login error")
	}
}

// TestSearchCases_MissingProjectIDPathValueRejected verifies a malformed
// project ID in the path is rejected with 400 before entity-service is ever
// called.
func TestSearchCases_MissingProjectIDPathValueRejected(t *testing.T) {
	fake := &fakeEntityCaseClient{}
	h := NewCaseHandler(fake)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /projects/{id}/cases/search", h.SearchCases)

	req := authedRequest(http.MethodPost, "/projects/not-a-uuid/cases/search", `{}`)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

// TestSearchDeployments_RouteIsProjectScoped mirrors
// TestSearchCases_RouteIsProjectScoped for POST /projects/{id}/deployments/search,
// and additionally verifies a client-supplied projectIds in the body is
// overwritten by the path value rather than merged with or preferred over it.
func TestSearchDeployments_RouteIsProjectScoped(t *testing.T) {
	fake := &fakeEntityDeploymentClient{}
	h := NewDeploymentHandler(fake)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /projects/{id}/deployments/search", h.SearchDeployments)

	req := authedRequest(http.MethodPost, "/projects/"+testProjectID+"/deployments/search",
		`{"projectIds":["some-other-project-id"],"pagination":{"limit":10,"offset":0}}`)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	want := []string{testProjectID}
	if !reflect.DeepEqual(fake.gotSearchDeploymentsReq.ProjectIDs, want) {
		t.Fatalf("ProjectIDs = %+v, want %+v (path value must win over body)", fake.gotSearchDeploymentsReq.ProjectIDs, want)
	}
}
