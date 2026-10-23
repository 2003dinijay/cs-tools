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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

type mockInstanceIngestionSvc struct {
	upsertMetaRes domain.InstanceMetaResponse
	upsertMetaErr error

	insertInfoRes domain.InstanceInfoResponse
	insertInfoErr error

	insertUsageRes domain.InstanceUsageIngestResponse
	insertUsageErr error

	importBatchRes domain.DailyUsageBatchResponse
	importBatchErr error

	rollupRes domain.UsageRollupResponse
	rollupErr error
	gotDaily  domain.UsageRollupDailyRequest
	gotMonth  domain.UsageRollupMonthlyRequest

	getSecretRes domain.ProjectSecretKeyResponse
	getSecretErr error
	gotKey       string

	depSecretRes domain.DeploymentConsumptionSecretResponse
	depSecretErr error
	gotNumber    string

	gotSince, gotLimit string

	gotProject, gotDeployment, gotImportsLimit string
}

func (m *mockInstanceIngestionSvc) ListUnresolvedProducts(_ context.Context, since, limit string) (domain.UnresolvedProductsResponse, error) {
	m.gotSince, m.gotLimit = since, limit
	return domain.UnresolvedProductsResponse{Nodes: []domain.UnresolvedProductNode{{NodeID: "n1", ReportedProduct: "wso2xx"}}, Count: 1}, nil
}

func (m *mockInstanceIngestionSvc) RollupDailyUsage(_ context.Context, req domain.UsageRollupDailyRequest) (domain.UsageRollupResponse, error) {
	m.gotDaily = req
	return m.rollupRes, m.rollupErr
}

func (m *mockInstanceIngestionSvc) RollupMonthlyUsage(_ context.Context, req domain.UsageRollupMonthlyRequest) (domain.UsageRollupResponse, error) {
	m.gotMonth = req
	return m.rollupRes, m.rollupErr
}

func (m *mockInstanceIngestionSvc) ListUsageReportImports(_ context.Context, projectKey, deploymentNumber, limit string) (domain.UsageReportImportsResponse, error) {
	m.gotProject, m.gotDeployment, m.gotImportsLimit = projectKey, deploymentNumber, limit
	return domain.UsageReportImportsResponse{Imports: []domain.UsageReportImport{{ID: "i1"}}, Count: 1}, nil
}

func (m *mockInstanceIngestionSvc) GetDeploymentConsumptionSecret(_ context.Context, number string) (domain.DeploymentConsumptionSecretResponse, error) {
	m.gotNumber = number
	if m.depSecretErr != nil {
		return domain.DeploymentConsumptionSecretResponse{}, m.depSecretErr
	}
	return m.depSecretRes, nil
}

func (m *mockInstanceIngestionSvc) UpsertInstanceMeta(_ context.Context, _ domain.InstanceMetaRequest) (domain.InstanceMetaResponse, error) {
	if m.upsertMetaErr != nil {
		return domain.InstanceMetaResponse{}, m.upsertMetaErr
	}
	return m.upsertMetaRes, nil
}

func (m *mockInstanceIngestionSvc) InsertInstanceInfo(_ context.Context, _ domain.InstanceInfoRequest) (domain.InstanceInfoResponse, error) {
	if m.insertInfoErr != nil {
		return domain.InstanceInfoResponse{}, m.insertInfoErr
	}
	return m.insertInfoRes, nil
}

func (m *mockInstanceIngestionSvc) InsertUsageStats(_ context.Context, _ domain.InstanceUsageIngestRequest) (domain.InstanceUsageIngestResponse, error) {
	if m.insertUsageErr != nil {
		return domain.InstanceUsageIngestResponse{}, m.insertUsageErr
	}
	return m.insertUsageRes, nil
}

func (m *mockInstanceIngestionSvc) ImportDailyUsagesBatch(_ context.Context, _ domain.DailyUsageBatchRequest) (domain.DailyUsageBatchResponse, error) {
	if m.importBatchErr != nil {
		return domain.DailyUsageBatchResponse{}, m.importBatchErr
	}
	return m.importBatchRes, nil
}

func (m *mockInstanceIngestionSvc) GetProjectSecretKey(_ context.Context, key string) (domain.ProjectSecretKeyResponse, error) {
	m.gotKey = key
	if m.getSecretErr != nil {
		return domain.ProjectSecretKeyResponse{}, m.getSecretErr
	}
	return m.getSecretRes, nil
}

func TestInstanceIngestionHandler(t *testing.T) {
	t.Run("UpsertInstanceMeta 200 OK", func(t *testing.T) {
		svc := &mockInstanceIngestionSvc{
			upsertMetaRes: domain.InstanceMetaResponse{ID: "node-uuid-123"},
		}
		h := NewInstanceIngestionHandler(svc)

		reqBody := `{"nodeId":"node-1","projectKey":"PRJ","deploymentNumber":"DEP1","product":"MI","version":"4.0.0"}`
		req := httptest.NewRequest(http.MethodPost, "/instances/meta", strings.NewReader(reqBody))
		w := httptest.NewRecorder()

		h.UpsertInstanceMeta(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}

		var resp domain.InstanceMetaResponse
		_ = json.NewDecoder(w.Body).Decode(&resp)
		if resp.ID != "node-uuid-123" {
			t.Errorf("expected node-uuid-123, got %q", resp.ID)
		}
	})

	t.Run("UpsertInstanceMeta 400 Validation Error", func(t *testing.T) {
		svc := &mockInstanceIngestionSvc{
			upsertMetaErr: &apierror.ValidationError{Msg: "nodeId is required"},
		}
		h := NewInstanceIngestionHandler(svc)

		req := httptest.NewRequest(http.MethodPost, "/instances/meta", strings.NewReader(`{}`))
		w := httptest.NewRecorder()

		h.UpsertInstanceMeta(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d", w.Code)
		}
	})

	t.Run("InsertInstanceInfo 200 OK", func(t *testing.T) {
		svc := &mockInstanceIngestionSvc{
			insertInfoRes: domain.InstanceInfoResponse{ID: "info-uuid-123"},
		}
		h := NewInstanceIngestionHandler(svc)

		reqBody := `{"nodeId":"node-1","jdkVersion":"17","coreCount":8}`
		req := httptest.NewRequest(http.MethodPost, "/instances/info", strings.NewReader(reqBody))
		w := httptest.NewRecorder()

		h.InsertInstanceInfo(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}
	})

	t.Run("InsertUsageStats 200 OK", func(t *testing.T) {
		svc := &mockInstanceIngestionSvc{
			insertUsageRes: domain.InstanceUsageIngestResponse{Status: "SUCCESS"},
		}
		h := NewInstanceIngestionHandler(svc)

		reqBody := `{"nodeId":"node-1","countType":"CORES","count":16}`
		req := httptest.NewRequest(http.MethodPost, "/instances/usage-stats", strings.NewReader(reqBody))
		w := httptest.NewRecorder()

		h.InsertUsageStats(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}
	})

	t.Run("ImportDailyUsagesBatch 200 OK", func(t *testing.T) {
		svc := &mockInstanceIngestionSvc{
			importBatchRes: domain.DailyUsageBatchResponse{ImportedCount: 3, Status: "SUCCESS"},
		}
		h := NewInstanceIngestionHandler(svc)

		reqBody := `{"nodeId":"node-1","usages":[{"date":"2026-10-15","type":"CORES","value":8}]}`
		req := httptest.NewRequest(http.MethodPost, "/instances/daily-usages/batch", strings.NewReader(reqBody))
		w := httptest.NewRecorder()

		h.ImportDailyUsagesBatch(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}
		var resp domain.DailyUsageBatchResponse
		_ = json.NewDecoder(w.Body).Decode(&resp)
		if resp.ImportedCount != 3 {
			t.Errorf("expected 3, got %d", resp.ImportedCount)
		}
	})

	t.Run("GetProjectSecretKey 200 OK", func(t *testing.T) {
		svc := &mockInstanceIngestionSvc{
			getSecretRes: domain.ProjectSecretKeyResponse{SecretKey: "secret-abc"},
		}
		h := NewInstanceIngestionHandler(svc)

		req := httptest.NewRequest(http.MethodGet, "/projects/CUPRSUBDEV/secret-key", nil)
		req.SetPathValue("key", "CUPRSUBDEV")
		w := httptest.NewRecorder()

		h.GetProjectSecretKey(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}
		if svc.gotKey != "CUPRSUBDEV" {
			t.Errorf("service got key %q, want CUPRSUBDEV", svc.gotKey)
		}
		if got := w.Header().Get("Cache-Control"); got != "no-store" {
			t.Errorf("Cache-Control = %q, want no-store: the body is a credential", got)
		}
		var resp domain.ProjectSecretKeyResponse
		_ = json.NewDecoder(w.Body).Decode(&resp)
		if resp.SecretKey != "secret-abc" {
			t.Errorf("expected 'secret-abc', got %q", resp.SecretKey)
		}
	})

	t.Run("GetProjectSecretKey 404 when the project has no key", func(t *testing.T) {
		svc := &mockInstanceIngestionSvc{
			getSecretErr: &apierror.NotFoundError{Msg: `project "X" has no product consumption secret key`},
		}
		h := NewInstanceIngestionHandler(svc)

		req := httptest.NewRequest(http.MethodGet, "/projects/X/secret-key", nil)
		req.SetPathValue("key", "X")
		w := httptest.NewRecorder()

		h.GetProjectSecretKey(w, req)
		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", w.Code)
		}
	})

	t.Run("GetDeploymentConsumptionSecret 200 OK", func(t *testing.T) {
		svc := &mockInstanceIngestionSvc{depSecretRes: domain.DeploymentConsumptionSecretResponse{
			DeploymentNumber: "DEP1", ProjectID: "p-1", ProjectKey: "PROJ", SecretKey: "abc",
		}}
		h := NewInstanceIngestionHandler(svc)
		req := httptest.NewRequest(http.MethodGet, "/deployments/by-number/DEP1/consumption-secret", nil)
		req.SetPathValue("number", "DEP1")
		w := httptest.NewRecorder()

		h.GetDeploymentConsumptionSecret(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		if svc.gotNumber != "DEP1" {
			t.Errorf("service got number %q", svc.gotNumber)
		}
		if got := w.Header().Get("Cache-Control"); got != "no-store" {
			t.Errorf("Cache-Control = %q, want no-store", got)
		}
		var resp domain.DeploymentConsumptionSecretResponse
		_ = json.NewDecoder(w.Body).Decode(&resp)
		if resp.ProjectKey != "PROJ" || resp.SecretKey != "abc" || resp.ProjectID != "p-1" {
			t.Errorf("response = %+v", resp)
		}
	})

	t.Run("GetDeploymentConsumptionSecret 404", func(t *testing.T) {
		svc := &mockInstanceIngestionSvc{depSecretErr: &apierror.NotFoundError{Msg: "deployment not found"}}
		h := NewInstanceIngestionHandler(svc)
		req := httptest.NewRequest(http.MethodGet, "/deployments/by-number/X/consumption-secret", nil)
		req.SetPathValue("number", "X")
		w := httptest.NewRecorder()

		h.GetDeploymentConsumptionSecret(w, req)
		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", w.Code)
		}
	})
}

func TestInstanceIngestionHandler_ListUnresolvedProducts(t *testing.T) {
	svc := &mockInstanceIngestionSvc{}
	h := NewInstanceIngestionHandler(svc)
	req := httptest.NewRequest(http.MethodGet, "/instances/unresolved-products?since=2026-10-01T00:00:00Z&limit=20", nil)
	w := httptest.NewRecorder()
	h.ListUnresolvedProducts(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	var resp domain.UnresolvedProductsResponse
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if svc.gotSince != "2026-10-01T00:00:00Z" || svc.gotLimit != "20" || resp.Count != 1 {
		t.Errorf("service saw since %q limit %q, response %+v", svc.gotSince, svc.gotLimit, resp)
	}
}

func TestInstanceIngestionHandler_UsageRollups(t *testing.T) {
	t.Run("daily 200 OK passes the date on", func(t *testing.T) {
		svc := &mockInstanceIngestionSvc{rollupRes: domain.UsageRollupResponse{RowsWritten: 4, Status: "SUCCESS"}}
		h := NewInstanceIngestionHandler(svc)
		req := httptest.NewRequest(http.MethodPost, "/instances/usage-rollups/daily", strings.NewReader(`{"date":"2026-10-15"}`))
		w := httptest.NewRecorder()
		h.RollupDailyUsage(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}
		var resp domain.UsageRollupResponse
		_ = json.NewDecoder(w.Body).Decode(&resp)
		if svc.gotDaily.Date != "2026-10-15" || resp.RowsWritten != 4 {
			t.Errorf("service saw %+v, response %+v", svc.gotDaily, resp)
		}
	})

	t.Run("monthly 200 OK passes the month on", func(t *testing.T) {
		svc := &mockInstanceIngestionSvc{rollupRes: domain.UsageRollupResponse{RowsWritten: 2, Status: "SUCCESS"}}
		h := NewInstanceIngestionHandler(svc)
		req := httptest.NewRequest(http.MethodPost, "/instances/usage-rollups/monthly", strings.NewReader(`{"year":2026,"month":9}`))
		w := httptest.NewRecorder()
		h.RollupMonthlyUsage(w, req)
		if w.Code != http.StatusOK || svc.gotMonth.Year != 2026 || svc.gotMonth.Month != 9 {
			t.Fatalf("status %d, service saw %+v", w.Code, svc.gotMonth)
		}
	})

	t.Run("validation errors are 400", func(t *testing.T) {
		svc := &mockInstanceIngestionSvc{rollupErr: &apierror.ValidationError{Msg: "month must be between 1 and 12"}}
		h := NewInstanceIngestionHandler(svc)
		req := httptest.NewRequest(http.MethodPost, "/instances/usage-rollups/monthly", strings.NewReader(`{"year":2026,"month":13}`))
		w := httptest.NewRecorder()
		h.RollupMonthlyUsage(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})
}

func TestInstanceIngestionHandler_ListUsageReportImports(t *testing.T) {
	svc := &mockInstanceIngestionSvc{}
	h := NewInstanceIngestionHandler(svc)
	req := httptest.NewRequest(http.MethodGet, "/instances/daily-usages/imports?projectKey=SCTPSUB&deploymentNumber=DEP000002519&limit=5", nil)
	w := httptest.NewRecorder()
	h.ListUsageReportImports(w, req)
	if w.Code != http.StatusOK || svc.gotProject != "SCTPSUB" || svc.gotDeployment != "DEP000002519" || svc.gotImportsLimit != "5" {
		t.Fatalf("status %d, service saw %q %q %q", w.Code, svc.gotProject, svc.gotDeployment, svc.gotImportsLimit)
	}
}
