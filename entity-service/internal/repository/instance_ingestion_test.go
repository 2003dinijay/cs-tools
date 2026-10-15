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

package repository

import (
	"context"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

func TestInstanceIngestionValidation(t *testing.T) {
	t.Run("UpsertDeploymentNode requires nodeId", func(t *testing.T) {
		repo := NewInstanceIngestionRepository(nil)
		_, err := repo.UpsertDeploymentNode(context.Background(), domain.InstanceMetaRequest{
			NodeID: "",
		})
		if err == nil {
			t.Fatal("expected error when nodeId is empty, got nil")
		}
	})

	t.Run("InsertDeploymentInformation requires nodeId", func(t *testing.T) {
		repo := NewInstanceIngestionRepository(nil)
		_, err := repo.InsertDeploymentInformation(context.Background(), domain.InstanceInfoRequest{
			NodeID: "",
		})
		if err == nil {
			t.Fatal("expected error when nodeId is empty, got nil")
		}
	})

	t.Run("InsertUsageStats requires nodeId and countType", func(t *testing.T) {
		repo := NewInstanceIngestionRepository(nil)
		err := repo.InsertUsageStats(context.Background(), domain.InstanceUsageIngestRequest{
			NodeID: "",
		})
		if err == nil {
			t.Fatal("expected error when nodeId is empty, got nil")
		}

		err = repo.InsertUsageStats(context.Background(), domain.InstanceUsageIngestRequest{
			NodeID:    "test-node",
			CountType: "",
		})
		if err == nil {
			t.Fatal("expected error when countType is empty, got nil")
		}
	})

	t.Run("ImportDailyUsagesBatch requires nodeId", func(t *testing.T) {
		repo := NewInstanceIngestionRepository(nil)
		_, err := repo.ImportDailyUsagesBatch(context.Background(), domain.DailyUsageBatchRequest{
			NodeID: "",
		})
		if err == nil {
			t.Fatal("expected error when nodeId is empty, got nil")
		}
	})

	t.Run("GetProjectSecretKey requires projectKey", func(t *testing.T) {
		repo := NewInstanceIngestionRepository(nil)
		_, err := repo.GetProjectSecretKey(context.Background(), "  ")
		if err == nil {
			t.Fatal("expected error when project key is whitespace, got nil")
		}
	})
}
