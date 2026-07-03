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
	"encoding/json"
	"fmt"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	integrationservice "github.com/wso2-open-operations/cs-tools/entity-service/internal/servicenow-integration-service"
)

type snConversationService struct {
	client *integrationservice.Client
}

// NewServiceNowConversationService constructs a ConversationService backed by the Choreo API.
func NewServiceNowConversationService(client *integrationservice.Client) ConversationService {
	return &snConversationService{client: client}
}

func (s *snConversationService) GetConversationMessages(ctx context.Context, req domain.GetConversationMessagesRequest) (domain.GetConversationMessagesResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.GetConversationMessagesResponse{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		return domain.GetConversationMessagesResponse{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}

	payload := snSearchCommentsPayload{
		ReferenceID:   uuidToSysid(req.ConversationID),
		ReferenceType: "conversation",
		Pagination:    snProjectPagination{Limit: req.Pagination.Limit, Offset: req.Pagination.Offset},
	}

	raw, err := s.client.Post(ctx, "/comments/search", token, payload)
	if err != nil {
		return domain.GetConversationMessagesResponse{}, err
	}

	var snResp snSearchCommentsResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.GetConversationMessagesResponse{}, fmt.Errorf("sn conversation messages: parse response: %w", err)
	}

	messages := make([]domain.ConversationMessage, 0, len(snResp.Comments))
	for _, c := range snResp.Comments {
		createdAt, err := time.Parse(snCreatedOnLayout, c.CreatedOn)
		if err != nil {
			return domain.GetConversationMessagesResponse{}, fmt.Errorf("sn conversation messages: parse createdOn %q: %w", c.CreatedOn, err)
		}
		messages = append(messages, domain.ConversationMessage{
			ID:                 sysidToUUID(c.ID),
			ConversationID:     sysidToUUID(c.ReferenceID),
			Content:            c.Content,
			Type:               c.Type,
			CreatedOn:          createdAt,
			CreatedBy:          c.CreatedBy,
			CreatedByFirstName: c.CreatedByFirstName,
			CreatedByLastName:  c.CreatedByLastName,
			CreatedByFullName:  c.CreatedByFullName,
		})
	}

	total := snResp.TotalRecords
	return domain.GetConversationMessagesResponse{
		Comments: messages,
		Total:    total,
		Limit:    req.Pagination.Limit,
		Offset:   req.Pagination.Offset,
		HasMore:  req.Pagination.Offset+len(messages) < total,
	}, nil
}
