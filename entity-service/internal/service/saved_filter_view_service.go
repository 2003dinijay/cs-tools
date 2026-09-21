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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

var validSavedFilterListKey = map[domain.SavedFilterListKey]bool{
	domain.SavedFilterListKeyCases:          true,
	domain.SavedFilterListKeyIncidents:      true,
	domain.SavedFilterListKeyChangeRequests: true,
	domain.SavedFilterListKeyProblems:       true,
}

type savedFilterViewService struct {
	repo repository.SavedFilterViewRepository
}

// NewSavedFilterViewService constructs a SavedFilterViewService.
func NewSavedFilterViewService(repo repository.SavedFilterViewRepository) SavedFilterViewService {
	return &savedFilterViewService{repo: repo}
}

func (s *savedFilterViewService) List(ctx context.Context, listKey domain.SavedFilterListKey) (domain.SavedFilterViewList, error) {
	if err := validateListKey(listKey); err != nil {
		return domain.SavedFilterViewList{}, err
	}
	userID, err := s.currentUserID(ctx)
	if err != nil {
		return domain.SavedFilterViewList{}, err
	}
	views, err := s.repo.List(ctx, userID, listKey)
	if err != nil {
		return domain.SavedFilterViewList{}, err
	}
	if views == nil {
		views = []domain.SavedFilterView{}
	}
	return domain.SavedFilterViewList{Views: views}, nil
}

func (s *savedFilterViewService) Save(ctx context.Context, req domain.SaveSavedFilterViewRequest) (domain.SavedFilterViewList, error) {
	if err := validateListKey(req.ListKey); err != nil {
		return domain.SavedFilterViewList{}, err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return domain.SavedFilterViewList{}, &apierror.ValidationError{Msg: "name is required"}
	}
	if utf8.RuneCountInString(name) > 255 {
		return domain.SavedFilterViewList{}, &apierror.ValidationError{Msg: "name must be at most 255 characters"}
	}
	userID, err := s.currentUserID(ctx)
	if err != nil {
		return domain.SavedFilterViewList{}, err
	}

	existing, err := s.repo.List(ctx, userID, req.ListKey)
	if err != nil {
		return domain.SavedFilterViewList{}, err
	}
	overwrite := false
	for _, v := range existing {
		if strings.EqualFold(v.Name, name) {
			overwrite = true
			break
		}
	}
	if !overwrite && len(existing) >= domain.MaxSavedFilterViews {
		return domain.SavedFilterViewList{}, &apierror.ValidationError{Msg: "at most 50 saved views are allowed for this list"}
	}

	views, err := s.repo.Save(ctx, userID, req.ListKey, name, req.Qs)
	if err != nil {
		return domain.SavedFilterViewList{}, err
	}
	return domain.SavedFilterViewList{Views: views}, nil
}

func (s *savedFilterViewService) Delete(ctx context.Context, listKey domain.SavedFilterListKey, name string) (domain.SavedFilterViewList, error) {
	if err := validateListKey(listKey); err != nil {
		return domain.SavedFilterViewList{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return domain.SavedFilterViewList{}, &apierror.ValidationError{Msg: "name is required"}
	}
	userID, err := s.currentUserID(ctx)
	if err != nil {
		return domain.SavedFilterViewList{}, err
	}
	views, err := s.repo.Delete(ctx, userID, listKey, name)
	if err != nil {
		return domain.SavedFilterViewList{}, err
	}
	return domain.SavedFilterViewList{Views: views}, nil
}

func (s *savedFilterViewService) Reorder(ctx context.Context, req domain.ReorderSavedFilterViewRequest) (domain.SavedFilterViewList, error) {
	if err := validateListKey(req.ListKey); err != nil {
		return domain.SavedFilterViewList{}, err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return domain.SavedFilterViewList{}, &apierror.ValidationError{Msg: "name is required"}
	}
	if req.Direction != domain.SavedFilterMoveUp && req.Direction != domain.SavedFilterMoveDown {
		return domain.SavedFilterViewList{}, &apierror.ValidationError{Msg: "direction must be up or down"}
	}
	userID, err := s.currentUserID(ctx)
	if err != nil {
		return domain.SavedFilterViewList{}, err
	}
	views, err := s.repo.Move(ctx, userID, req.ListKey, name, req.Direction)
	if err != nil {
		return domain.SavedFilterViewList{}, err
	}
	return domain.SavedFilterViewList{Views: views}, nil
}

func (s *savedFilterViewService) currentUserID(ctx context.Context) (string, error) {
	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		return "", &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}
	userID, err := savedFilterAsgardeoUserID(token)
	if err != nil {
		return "", &apierror.ValidationError{Msg: "x-user-id-token: " + err.Error()}
	}
	return userID, nil
}

// savedFilterAsgardeoUserID reads the JWT userid claim for this feature only.
// Other services still resolve identity via email → "user".id.
func savedFilterAsgardeoUserID(token string) (string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("malformed JWT: expected 3 parts, got %d", len(parts))
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("malformed JWT payload: %w", err)
	}
	var claims struct {
		UserID string `json:"userid"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", fmt.Errorf("malformed JWT claims: %w", err)
	}
	if claims.UserID == "" {
		return "", fmt.Errorf("userid claim not present in token")
	}
	return claims.UserID, nil
}

func validateListKey(listKey domain.SavedFilterListKey) error {
	if !validSavedFilterListKey[listKey] {
		return &apierror.ValidationError{Msg: "listKey must be one of cases, incidents, change_requests, problems"}
	}
	return nil
}
