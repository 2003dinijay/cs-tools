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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

type mockInstanceIngestionRepo struct {
	resolveFn    func(ctx context.Context, product, version string) (*string, error)
	upsertNodeFn func(ctx context.Context, req domain.InstanceMetaRequest) (string, error)
	insertInfoFn func(ctx context.Context, req domain.InstanceInfoRequest) (string, error)
	insertStatFn func(ctx context.Context, req domain.InstanceUsageIngestRequest) error
	importBatchFn func(ctx context.Context, req domain.DailyUsageBatchRequest) (int, error)
	getSecretFn  func(ctx context.Context, projectKey string) (string, error)
}

func (m *mockInstanceIngestionRepo) ResolveProductVersionID(ctx context.Context, product, version string) (*string, error) {
	if m.resolveFn != nil {
		return m.resolveFn(ctx, product, version)
	}
	id := "test-version-id"
	return &id, nil
}

func (m *mockInstanceIngestionRepo) UpsertDeploymentNode(ctx context.Context, req domain.InstanceMetaRequest) (string, error) {
	if m.upsertNodeFn != nil {
		return m.upsertNodeFn(ctx, req)
	}
	return "test-node-uuid", nil
}

func (m *mockInstanceIngestionRepo) InsertDeploymentInformation(ctx context.Context, req domain.InstanceInfoRequest) (string, error) {
	if m.insertInfoFn != nil {
		return m.insertInfoFn(ctx, req)
	}
	return "test-info-uuid", nil
}

func (m *mockInstanceIngestionRepo) InsertUsageStats(ctx context.Context, req domain.InstanceUsageIngestRequest) error {
	if m.insertStatFn != nil {
		return m.insertStatFn(ctx, req)
	}
	return nil
}

func (m *mockInstanceIngestionRepo) ImportDailyUsagesBatch(ctx context.Context, req domain.DailyUsageBatchRequest) (int, error) {
	if m.importBatchFn != nil {
		return m.importBatchFn(ctx, req)
	}
	return len(req.Usages), nil
}

func (m *mockInstanceIngestionRepo) GetProjectSecretKey(ctx context.Context, projectKey string) (string, error) {
	if m.getSecretFn != nil {
		return m.getSecretFn(ctx, projectKey)
	}
	return "test-secret-key-123", nil
}

func TestInstanceIngestionService(t *testing.T) {
	ctx := context.Background()

	t.Run("UpsertInstanceMeta succeeds with valid payload", func(t *testing.T) {
		repo := &mockInstanceIngestionRepo{}
		svc := NewInstanceIngestionService(repo)

		resp, err := svc.UpsertInstanceMeta(ctx, domain.InstanceMetaRequest{
			NodeID:           "node-123",
			ProjectKey:       "PROJ",
			DeploymentNumber: "DEP001",
			Product:          "MI",
			Version:          "4.0.0",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.ID != "test-node-uuid" {
			t.Errorf("expected ID 'test-node-uuid', got %q", resp.ID)
		}
	})

	t.Run("UpsertInstanceMeta validates empty nodeId", func(t *testing.T) {
		repo := &mockInstanceIngestionRepo{}
		svc := NewInstanceIngestionService(repo)

		_, err := svc.UpsertInstanceMeta(ctx, domain.InstanceMetaRequest{NodeID: "  "})
		if err == nil {
			t.Fatal("expected validation error, got nil")
		}
		var valErr *apierror.ValidationError
		if !errors.As(err, &valErr) {
			t.Errorf("expected *apierror.ValidationError, got %T", err)
		}
	})

	t.Run("InsertInstanceInfo succeeds", func(t *testing.T) {
		repo := &mockInstanceIngestionRepo{}
		svc := NewInstanceIngestionService(repo)

		resp, err := svc.InsertInstanceInfo(ctx, domain.InstanceInfoRequest{
			NodeID:     "node-123",
			JDKVersion: "17",
			CoreCount:  4,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.ID != "test-info-uuid" {
			t.Errorf("expected ID 'test-info-uuid', got %q", resp.ID)
		}
	})

	t.Run("InsertUsageStats validates required fields", func(t *testing.T) {
		repo := &mockInstanceIngestionRepo{}
		svc := NewInstanceIngestionService(repo)

		_, err := svc.InsertUsageStats(ctx, domain.InstanceUsageIngestRequest{
			NodeID:    "node-123",
			CountType: "",
		})
		if err == nil {
			t.Fatal("expected validation error for empty countType, got nil")
		}

		resp, err := svc.InsertUsageStats(ctx, domain.InstanceUsageIngestRequest{
			NodeID:    "node-123",
			CountType: "TRANSACTION_COUNT",
			Count:     100,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Status != "SUCCESS" {
			t.Errorf("expected status 'SUCCESS', got %q", resp.Status)
		}
	})

	t.Run("ImportDailyUsagesBatch succeeds", func(t *testing.T) {
		repo := &mockInstanceIngestionRepo{}
		svc := NewInstanceIngestionService(repo)

		resp, err := svc.ImportDailyUsagesBatch(ctx, domain.DailyUsageBatchRequest{
			NodeID: "node-123",
			Usages: []domain.DailyUsageBatchItem{
				{Date: "2026-10-15", Type: "CORES", Value: 8},
				{Date: "2026-10-15", Type: "TRANSACTIONS", Value: 500},
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.ImportedCount != 2 {
			t.Errorf("expected importedCount 2, got %d", resp.ImportedCount)
		}
	})

	t.Run("GetProjectSecretKey succeeds", func(t *testing.T) {
		repo := &mockInstanceIngestionRepo{}
		svc := NewInstanceIngestionService(repo)

		resp, err := svc.GetProjectSecretKey(ctx, "CUPRSUBDEV")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.SecretKey != "test-secret-key-123" {
			t.Errorf("expected secretKey 'test-secret-key-123', got %q", resp.SecretKey)
		}
	})
}
