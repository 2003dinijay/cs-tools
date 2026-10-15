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
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// InstanceIngestionService defines the service operations for instance ingestion.
type InstanceIngestionService interface {
	UpsertInstanceMeta(ctx context.Context, req domain.InstanceMetaRequest) (domain.InstanceMetaResponse, error)
	InsertInstanceInfo(ctx context.Context, req domain.InstanceInfoRequest) (domain.InstanceInfoResponse, error)
	InsertUsageStats(ctx context.Context, req domain.InstanceUsageIngestRequest) (domain.InstanceUsageIngestResponse, error)
	ImportDailyUsagesBatch(ctx context.Context, req domain.DailyUsageBatchRequest) (domain.DailyUsageBatchResponse, error)
	GetProjectSecretKey(ctx context.Context, projectKey string) (domain.ProjectSecretKeyResponse, error)
}

type instanceIngestionService struct {
	repo repository.InstanceIngestionRepository
}

// NewInstanceIngestionService constructs an InstanceIngestionService.
func NewInstanceIngestionService(repo repository.InstanceIngestionRepository) InstanceIngestionService {
	return &instanceIngestionService{repo: repo}
}

// UpsertInstanceMeta validates and persists deployment node metadata.
func (s *instanceIngestionService) UpsertInstanceMeta(ctx context.Context, req domain.InstanceMetaRequest) (domain.InstanceMetaResponse, error) {
	req.NodeID = strings.TrimSpace(req.NodeID)
	if req.NodeID == "" {
		return domain.InstanceMetaResponse{}, &apierror.ValidationError{Msg: "nodeId is required"}
	}
	req.ProjectKey = strings.TrimSpace(req.ProjectKey)
	req.DeploymentNumber = strings.TrimSpace(req.DeploymentNumber)
	req.Product = strings.TrimSpace(req.Product)
	req.Version = strings.TrimSpace(req.Version)

	id, err := s.repo.UpsertDeploymentNode(ctx, req)
	if err != nil {
		return domain.InstanceMetaResponse{}, err
	}
	return domain.InstanceMetaResponse{ID: id}, nil
}

// InsertInstanceInfo validates and persists host environment snapshot.
func (s *instanceIngestionService) InsertInstanceInfo(ctx context.Context, req domain.InstanceInfoRequest) (domain.InstanceInfoResponse, error) {
	req.NodeID = strings.TrimSpace(req.NodeID)
	if req.NodeID == "" {
		return domain.InstanceInfoResponse{}, &apierror.ValidationError{Msg: "nodeId is required"}
	}
	req.JDKVersion = strings.TrimSpace(req.JDKVersion)
	req.Hash = strings.TrimSpace(req.Hash)
	req.CreatedTime = strings.TrimSpace(req.CreatedTime)

	id, err := s.repo.InsertDeploymentInformation(ctx, req)
	if err != nil {
		return domain.InstanceInfoResponse{}, err
	}
	return domain.InstanceInfoResponse{ID: id}, nil
}

// InsertUsageStats validates and persists hourly & daily usage telemetry.
func (s *instanceIngestionService) InsertUsageStats(ctx context.Context, req domain.InstanceUsageIngestRequest) (domain.InstanceUsageIngestResponse, error) {
	req.NodeID = strings.TrimSpace(req.NodeID)
	if req.NodeID == "" {
		return domain.InstanceUsageIngestResponse{}, &apierror.ValidationError{Msg: "nodeId is required"}
	}
	req.CountType = strings.TrimSpace(req.CountType)
	if req.CountType == "" {
		return domain.InstanceUsageIngestResponse{}, &apierror.ValidationError{Msg: "countType is required"}
	}
	req.CreatedTime = strings.TrimSpace(req.CreatedTime)

	if err := s.repo.InsertUsageStats(ctx, req); err != nil {
		return domain.InstanceUsageIngestResponse{}, err
	}
	return domain.InstanceUsageIngestResponse{Status: "SUCCESS"}, nil
}

// ImportDailyUsagesBatch validates and persists batch imported daily usage records.
func (s *instanceIngestionService) ImportDailyUsagesBatch(ctx context.Context, req domain.DailyUsageBatchRequest) (domain.DailyUsageBatchResponse, error) {
	req.NodeID = strings.TrimSpace(req.NodeID)
	if req.NodeID == "" {
		return domain.DailyUsageBatchResponse{}, &apierror.ValidationError{Msg: "nodeId is required"}
	}
	req.ProjectKey = strings.TrimSpace(req.ProjectKey)
	req.DeploymentNumber = strings.TrimSpace(req.DeploymentNumber)
	req.Product = strings.TrimSpace(req.Product)
	req.Version = strings.TrimSpace(req.Version)
	req.UserEmail = strings.TrimSpace(req.UserEmail)

	count, err := s.repo.ImportDailyUsagesBatch(ctx, req)
	if err != nil {
		return domain.DailyUsageBatchResponse{}, err
	}
	return domain.DailyUsageBatchResponse{
		ImportedCount: count,
		Status:        "SUCCESS",
	}, nil
}

// GetProjectSecretKey retrieves the primary secret key for HMAC validation.
func (s *instanceIngestionService) GetProjectSecretKey(ctx context.Context, projectKey string) (domain.ProjectSecretKeyResponse, error) {
	trimmed := strings.TrimSpace(projectKey)
	if trimmed == "" {
		return domain.ProjectSecretKeyResponse{}, &apierror.ValidationError{Msg: "project key is required"}
	}

	secretKey, err := s.repo.GetProjectSecretKey(ctx, trimmed)
	if err != nil {
		return domain.ProjectSecretKeyResponse{}, err
	}
	return domain.ProjectSecretKeyResponse{SecretKey: secretKey}, nil
}
