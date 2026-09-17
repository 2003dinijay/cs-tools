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
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// fakeDeploymentService is a minimal, in-memory DeploymentService stub used
// only to drive SearchProjectsByProductVersion's fetchAllDeploymentProjects
// helper. It paginates over a fixed slice exactly the way a real
// implementation would (offset/limit, HasMore derived from the remainder),
// so the caller's paging loop exercises real page-boundary behaviour.
// CreateDeployment/UpdateDeployment are never called by that code path.
type fakeDeploymentService struct {
	deployments []domain.DeploymentView
	calls       int
}

func (f *fakeDeploymentService) SearchDeployments(_ context.Context, req domain.SearchDeploymentsRequest) (domain.SearchDeploymentsResponse, error) {
	f.calls++
	total := len(f.deployments)
	start := req.Pagination.Offset
	if start > total {
		start = total
	}
	end := start + req.Pagination.Limit
	if end > total {
		end = total
	}
	return domain.SearchDeploymentsResponse{
		Deployments: f.deployments[start:end],
		Total:       total,
		Limit:       req.Pagination.Limit,
		Offset:      req.Pagination.Offset,
		HasMore:     end < total,
	}, nil
}

func (f *fakeDeploymentService) CreateDeployment(context.Context, domain.CreateDeploymentRequest) (domain.CreateDeploymentResponse, error) {
	panic("fakeDeploymentService: CreateDeployment not implemented")
}

func (f *fakeDeploymentService) UpdateDeployment(context.Context, domain.UpdateDeploymentRequest) (domain.UpdateDeploymentResponse, error) {
	panic("fakeDeploymentService: UpdateDeployment not implemented")
}

// alwaysMoreDeploymentService always reports HasMore: true and returns a
// full page of throwaway deployments regardless of offset, used only to
// force fetchAllDeploymentProjects' safety bound to trip.
type alwaysMoreDeploymentService struct{}

func (alwaysMoreDeploymentService) SearchDeployments(_ context.Context, req domain.SearchDeploymentsRequest) (domain.SearchDeploymentsResponse, error) {
	views := make([]domain.DeploymentView, req.Pagination.Limit)
	for i := range views {
		views[i] = domain.DeploymentView{
			ID:      fmt.Sprintf("dep-%d-%d", req.Pagination.Offset, i),
			Project: domain.EntityRef{ID: "proj-x", Name: "Project X"},
		}
	}
	return domain.SearchDeploymentsResponse{Deployments: views, Total: 1 << 30, HasMore: true}, nil
}

func (alwaysMoreDeploymentService) CreateDeployment(context.Context, domain.CreateDeploymentRequest) (domain.CreateDeploymentResponse, error) {
	panic("alwaysMoreDeploymentService: CreateDeployment not implemented")
}

func (alwaysMoreDeploymentService) UpdateDeployment(context.Context, domain.UpdateDeploymentRequest) (domain.UpdateDeploymentResponse, error) {
	panic("alwaysMoreDeploymentService: UpdateDeployment not implemented")
}

var (
	testPBVProductSysid   = sysid32('3')
	testPBVVersionSysid   = sysid32('4')
	testPBVOtherProdSysid = sysid32('5')
	testPBVOtherVerSysid  = sysid32('6')
	testPBVProductUUID    = sysidToUUID(testPBVProductSysid)
	testPBVVersionUUID    = sysidToUUID(testPBVVersionSysid)
)

func TestSNDeployedProductService_SearchProjectsByProductVersion_RejectsInvalidProductID(t *testing.T) {
	svc := NewServiceNowDeployedProductService(newTestSNClient(t, http.NewServeMux()), &fakeDeploymentService{})

	_, err := svc.SearchProjectsByProductVersion(contextWithUserIDToken("token"), domain.SearchProjectsByProductVersionRequest{
		ProductID:        "not-a-uuid",
		ProductVersionID: testPBVVersionUUID,
	})
	if _, ok := err.(*apierror.ValidationError); !ok {
		t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
	}
}

func TestSNDeployedProductService_SearchProjectsByProductVersion_RejectsInvalidProductVersionID(t *testing.T) {
	svc := NewServiceNowDeployedProductService(newTestSNClient(t, http.NewServeMux()), &fakeDeploymentService{})

	_, err := svc.SearchProjectsByProductVersion(contextWithUserIDToken("token"), domain.SearchProjectsByProductVersionRequest{
		ProductID:        testPBVProductUUID,
		ProductVersionID: "not-a-uuid",
	})
	if _, ok := err.(*apierror.ValidationError); !ok {
		t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
	}
}

// deployedProductFixture builds the wire shape for one deployed product
// entry, matching a given deployment/product/version sysid triple.
func deployedProductFixture(id, deploymentSysid, productSysid, versionSysid string) map[string]any {
	return map[string]any{
		"id":         id,
		"deployment": map[string]any{"id": deploymentSysid, "name": "irrelevant"},
		"product":    map[string]any{"id": productSysid, "name": "irrelevant"},
		"version":    map[string]any{"id": versionSysid, "name": "irrelevant"},
		"createdOn":  "2026-01-01 00:00:00",
		"updatedOn":  "2026-01-02 00:00:00",
	}
}

// TestSNDeployedProductService_SearchProjectsByProductVersion_MatchesAndDedupes
// verifies the core join/filter/dedup logic: two deployments belonging to
// the same project both run the matching product+version (must collapse to
// one project entry), a third deployment in a different project also
// matches (must appear as its own entry), and a fourth deployment running a
// different product must be excluded entirely.
func TestSNDeployedProductService_SearchProjectsByProductVersion_MatchesAndDedupes(t *testing.T) {
	depA1, depA2, depB, depOther := sysid32('1'), sysid32('2'), sysid32('7'), sysid32('8')
	projA, projB := domain.EntityRef{ID: "proj-a-uuid", Name: "Project A"}, domain.EntityRef{ID: "proj-b-uuid", Name: "Project B"}

	deploymentSvc := &fakeDeploymentService{deployments: []domain.DeploymentView{
		{ID: sysidToUUID(depA1), Project: projA},
		{ID: sysidToUUID(depA2), Project: projA},
		{ID: sysidToUUID(depB), Project: projB},
		{ID: sysidToUUID(depOther), Project: projB},
	}}

	mux := http.NewServeMux()
	mux.HandleFunc("/deployed-products/search", func(w http.ResponseWriter, r *http.Request) {
		items := []map[string]any{
			deployedProductFixture(sysid32('a'), depA1, testPBVProductSysid, testPBVVersionSysid),
			deployedProductFixture(sysid32('b'), depA2, testPBVProductSysid, testPBVVersionSysid),
			deployedProductFixture(sysid32('c'), depB, testPBVProductSysid, testPBVVersionSysid),
			deployedProductFixture(sysid32('d'), depOther, testPBVOtherProdSysid, testPBVOtherVerSysid),
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"deployedProducts": items, "totalRecords": len(items), "offset": 0, "limit": 50,
		})
	})

	svc := NewServiceNowDeployedProductService(newTestSNClient(t, mux), deploymentSvc)

	resp, err := svc.SearchProjectsByProductVersion(contextWithUserIDToken("token"), domain.SearchProjectsByProductVersionRequest{
		ProductID:        testPBVProductUUID,
		ProductVersionID: testPBVVersionUUID,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Total != 2 {
		t.Fatalf("expected 2 distinct matching projects, got %d: %+v", resp.Total, resp.Projects)
	}
	names := map[string]bool{}
	for _, p := range resp.Projects {
		names[p.Name] = true
	}
	if !names["Project A"] || !names["Project B"] {
		t.Fatalf("expected Project A and Project B, got %+v", resp.Projects)
	}
}

// TestSNDeployedProductService_SearchProjectsByProductVersion_PaginatesDedupedResult
// verifies the caller's own pagination window is applied after, not before,
// deduplication and sorting — a limit/offset here must slice the deduped
// project set, not the raw deployed-product matches.
func TestSNDeployedProductService_SearchProjectsByProductVersion_PaginatesDedupedResult(t *testing.T) {
	deps := []string{sysid32('1'), sysid32('2'), sysid32('3')}
	projects := []domain.EntityRef{
		{ID: "proj-a", Name: "Alpha"},
		{ID: "proj-b", Name: "Bravo"},
		{ID: "proj-c", Name: "Charlie"},
	}

	deploymentSvc := &fakeDeploymentService{deployments: []domain.DeploymentView{
		{ID: sysidToUUID(deps[0]), Project: projects[0]},
		{ID: sysidToUUID(deps[1]), Project: projects[1]},
		{ID: sysidToUUID(deps[2]), Project: projects[2]},
	}}

	mux := http.NewServeMux()
	mux.HandleFunc("/deployed-products/search", func(w http.ResponseWriter, r *http.Request) {
		items := []map[string]any{
			deployedProductFixture(sysid32('a'), deps[0], testPBVProductSysid, testPBVVersionSysid),
			deployedProductFixture(sysid32('b'), deps[1], testPBVProductSysid, testPBVVersionSysid),
			deployedProductFixture(sysid32('c'), deps[2], testPBVProductSysid, testPBVVersionSysid),
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"deployedProducts": items, "totalRecords": len(items), "offset": 0, "limit": 50,
		})
	})

	svc := NewServiceNowDeployedProductService(newTestSNClient(t, mux), deploymentSvc)

	resp, err := svc.SearchProjectsByProductVersion(contextWithUserIDToken("token"), domain.SearchProjectsByProductVersionRequest{
		Pagination:       domain.Pagination{Limit: 1, Offset: 1},
		ProductID:        testPBVProductUUID,
		ProductVersionID: testPBVVersionUUID,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Total != 3 || !resp.HasMore || len(resp.Projects) != 1 {
		t.Fatalf("unexpected pagination result: %+v", resp)
	}
	if resp.Projects[0].Name != "Bravo" {
		t.Fatalf("expected the second name-sorted project (Bravo), got %q", resp.Projects[0].Name)
	}
}

// TestSNDeployedProductService_SearchProjectsByProductVersion_PagesThroughMultipleDeployedProductPages
// verifies the per-chunk deployed-products loop keeps paging (using its own
// offset) until the upstream's hasMore is false, rather than stopping after
// one page — the match here only appears on the second page.
func TestSNDeployedProductService_SearchProjectsByProductVersion_PagesThroughMultipleDeployedProductPages(t *testing.T) {
	dep1, dep2 := sysid32('1'), sysid32('2')
	proj := domain.EntityRef{ID: "proj-a", Name: "Project A"}

	deploymentSvc := &fakeDeploymentService{deployments: []domain.DeploymentView{
		{ID: sysidToUUID(dep1), Project: proj},
		{ID: sysidToUUID(dep2), Project: proj},
	}}

	var requestOffsets []int
	mux := http.NewServeMux()
	mux.HandleFunc("/deployed-products/search", func(w http.ResponseWriter, r *http.Request) {
		var payload snDeployedProductSearchPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		requestOffsets = append(requestOffsets, payload.Pagination.Offset)

		var items []map[string]any
		switch payload.Pagination.Offset {
		case 0:
			items = []map[string]any{deployedProductFixture(sysid32('a'), dep1, testPBVOtherProdSysid, testPBVOtherVerSysid)}
		default:
			items = []map[string]any{deployedProductFixture(sysid32('b'), dep2, testPBVProductSysid, testPBVVersionSysid)}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"deployedProducts": items, "totalRecords": 2, "offset": payload.Pagination.Offset, "limit": 1,
		})
	})

	svc := NewServiceNowDeployedProductService(newTestSNClient(t, mux), deploymentSvc)

	resp, err := svc.SearchProjectsByProductVersion(contextWithUserIDToken("token"), domain.SearchProjectsByProductVersionRequest{
		ProductID:        testPBVProductUUID,
		ProductVersionID: testPBVVersionUUID,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Total != 1 || len(resp.Projects) != 1 || resp.Projects[0].Name != "Project A" {
		t.Fatalf("expected the second-page match to surface Project A, got %+v", resp)
	}
	if len(requestOffsets) < 2 {
		t.Fatalf("expected the deployed-products search to be called for more than one page, got offsets %v", requestOffsets)
	}
}

// TestSNDeployedProductService_SearchProjectsByProductVersion_DeploymentEnumerationErrorsRatherThanTruncate
// forces fetchAllDeploymentProjects' safety bound to be exceeded (an upstream
// that always claims hasMore: true) and asserts this fails loudly with a
// ServiceUnavailableError rather than silently resolving against a partial,
// incomplete deployment-to-project map.
func TestSNDeployedProductService_SearchProjectsByProductVersion_DeploymentEnumerationErrorsRatherThanTruncate(t *testing.T) {
	svc := NewServiceNowDeployedProductService(newTestSNClient(t, http.NewServeMux()), alwaysMoreDeploymentService{})

	_, err := svc.SearchProjectsByProductVersion(contextWithUserIDToken("token"), domain.SearchProjectsByProductVersionRequest{
		ProductID:        testPBVProductUUID,
		ProductVersionID: testPBVVersionUUID,
	})
	if _, ok := err.(*apierror.ServiceUnavailableError); !ok {
		t.Fatalf("expected *apierror.ServiceUnavailableError, got %T: %v", err, err)
	}
}

// TestSNDeployedProductService_SearchProjectsByProductVersion_DeployedProductEnumerationErrorsRatherThanTruncate
// mirrors the test above for the per-chunk deployed-products loop: an
// upstream that always claims hasMore: true for a single deployment's
// deployed-products page must fail loudly once the page bound is exceeded,
// not return an incomplete/wrong match set.
func TestSNDeployedProductService_SearchProjectsByProductVersion_DeployedProductEnumerationErrorsRatherThanTruncate(t *testing.T) {
	dep := sysid32('1')
	deploymentSvc := &fakeDeploymentService{deployments: []domain.DeploymentView{
		{ID: sysidToUUID(dep), Project: domain.EntityRef{ID: "proj-a", Name: "Project A"}},
	}}

	mux := http.NewServeMux()
	mux.HandleFunc("/deployed-products/search", func(w http.ResponseWriter, r *http.Request) {
		items := []map[string]any{deployedProductFixture(sysid32('a'), dep, testPBVOtherProdSysid, testPBVOtherVerSysid)}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"deployedProducts": items, "totalRecords": 1 << 30, "offset": 0, "limit": 50,
		})
	})

	svc := NewServiceNowDeployedProductService(newTestSNClient(t, mux), deploymentSvc)

	_, err := svc.SearchProjectsByProductVersion(contextWithUserIDToken("token"), domain.SearchProjectsByProductVersionRequest{
		ProductID:        testPBVProductUUID,
		ProductVersionID: testPBVVersionUUID,
	})
	if _, ok := err.(*apierror.ServiceUnavailableError); !ok {
		t.Fatalf("expected *apierror.ServiceUnavailableError, got %T: %v", err, err)
	}
}
