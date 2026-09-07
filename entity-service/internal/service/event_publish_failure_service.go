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

// Package service is declared in interfaces.go.
package service

import (
	"context"
	"encoding/json"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

type eventPublishFailureService struct {
	repo repository.EventPublishFailureRepository
}

// NewEventPublishFailureService constructs an EventPublishFailureService
// backed by the given repository.
func NewEventPublishFailureService(repo repository.EventPublishFailureRepository) EventPublishFailureService {
	return &eventPublishFailureService{repo: repo}
}

// CreateEventPublishFailure implements EventPublishFailureService.
func (s *eventPublishFailureService) CreateEventPublishFailure(ctx context.Context, req domain.CreateEventPublishFailureRequest) (domain.EventPublishFailure, error) {
	if req.EventType == "" {
		return domain.EventPublishFailure{}, &apierror.ValidationError{Msg: "eventType is required"}
	}
	if req.EntityID == "" {
		return domain.EventPublishFailure{}, &apierror.ValidationError{Msg: "entityId is required"}
	}
	if len(req.Payload) == 0 {
		return domain.EventPublishFailure{}, &apierror.ValidationError{Msg: "payload is required"}
	}
	// json.RawMessage accepts any valid JSON (null, a string, a number, an
	// array, ...), but the documented (and stored) shape is an object —
	// unmarshaling into a map is the cheapest way to reject anything else.
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(req.Payload, &obj); err != nil {
		return domain.EventPublishFailure{}, &apierror.ValidationError{Msg: "payload must be a JSON object"}
	}
	if req.Error == "" {
		return domain.EventPublishFailure{}, &apierror.ValidationError{Msg: "error is required"}
	}
	return s.repo.Create(ctx, req)
}

// ResolveEventPublishFailure implements EventPublishFailureService.
func (s *eventPublishFailureService) ResolveEventPublishFailure(ctx context.Context, id string) (domain.EventPublishFailure, error) {
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.EventPublishFailure{}, err
	}
	return s.repo.MarkResolved(ctx, id)
}

// SearchEventPublishFailures implements EventPublishFailureService.
func (s *eventPublishFailureService) SearchEventPublishFailures(ctx context.Context, req domain.SearchEventPublishFailuresRequest) (domain.SearchEventPublishFailuresResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchEventPublishFailuresResponse{}, err
	}
	failures, total, err := s.repo.Search(ctx, req)
	if err != nil {
		return domain.SearchEventPublishFailuresResponse{}, err
	}
	return domain.SearchEventPublishFailuresResponse{
		Failures: failures,
		Total:    total,
		Limit:    req.Pagination.Limit,
		Offset:   req.Pagination.Offset,
		HasMore:  req.Pagination.Offset+len(failures) < total,
	}, nil
}
