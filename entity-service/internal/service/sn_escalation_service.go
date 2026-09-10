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
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	integrationservice "github.com/wso2-open-operations/cs-tools/entity-service/internal/servicenow-integration-service"
)

// snEscalationNotifiedUser mirrors the Choreo notificationSentTo entry shape.
type snEscalationNotifiedUser struct {
	ID       string  `json:"id"`
	UserName string  `json:"userName"`
	Name     *string `json:"name"`
	Email    *string `json:"email"`
}

func (u snEscalationNotifiedUser) toDomain() domain.EscalationNotifiedUser {
	return domain.EscalationNotifiedUser{
		ID:       sysidToUUID(u.ID),
		UserName: u.UserName,
		Name:     u.Name,
		Email:    u.Email,
	}
}

func toDomainEscalationNotifiedUsers(users []snEscalationNotifiedUser) []domain.EscalationNotifiedUser {
	out := make([]domain.EscalationNotifiedUser, 0, len(users))
	for _, u := range users {
		out = append(out, u.toDomain())
	}
	return out
}

// snEscalation mirrors the Choreo Escalation shape (search results).
type snEscalation struct {
	ID                 string                     `json:"id"`
	Case               snReferenceTableItem       `json:"case"`
	CurrentLevel       snChoiceOption             `json:"currentLevel"`
	PreviousLevel      snChoiceOption             `json:"previousLevel"`
	CreatedBy          string                     `json:"createdBy"`
	CreatedOn          string                     `json:"createdOn"`
	UpdatedOn          string                     `json:"updatedOn"`
	Reason             *string                    `json:"reason"`
	NotificationSentTo []snEscalationNotifiedUser `json:"notificationSentTo"`
}

func (e snEscalation) toDomain() domain.Escalation {
	return domain.Escalation{
		ID:                 sysidToUUID(e.ID),
		Case:               e.Case.toDomain(),
		CurrentLevel:       e.CurrentLevel.toDomain(),
		PreviousLevel:      e.PreviousLevel.toDomain(),
		CreatedBy:          e.CreatedBy,
		CreatedOn:          e.CreatedOn,
		UpdatedOn:          e.UpdatedOn,
		Reason:             e.Reason,
		NotificationSentTo: toDomainEscalationNotifiedUsers(e.NotificationSentTo),
	}
}

// snSearchEscalationsFilters mirrors the Choreo EscalationSearchPayload.filters shape.
type snSearchEscalationsFilters struct {
	CaseIDs       []string `json:"caseIds,omitempty"`
	CurrentLevels []int    `json:"currentLevels,omitempty"`
}

// snEscalationSort mirrors the Choreo EscalationSearchPayload.sortBy shape.
type snEscalationSort struct {
	Field string `json:"field"`
	Order string `json:"order"`
}

// snSearchEscalationsPayload mirrors the Choreo POST /escalations/search request body.
type snSearchEscalationsPayload struct {
	Filters    *snSearchEscalationsFilters `json:"filters,omitempty"`
	SortBy     *snEscalationSort           `json:"sortBy,omitempty"`
	Pagination snProjectPagination         `json:"pagination"`
}

// snSearchEscalationsResponse mirrors the Choreo POST /escalations/search response.
type snSearchEscalationsResponse struct {
	Escalations  []snEscalation `json:"escalations"`
	TotalRecords int            `json:"totalRecords"`
	Limit        int            `json:"limit"`
	Offset       int            `json:"offset"`
}

var validEscalationSortField = map[domain.EscalationSortField]bool{
	domain.EscalationSortFieldCreatedOn: true,
	domain.EscalationSortFieldUpdatedOn: true,
}

var validEscalationSortOrder = map[domain.EscalationSortOrder]bool{
	domain.EscalationSortOrderAsc:  true,
	domain.EscalationSortOrderDesc: true,
}

type snEscalationService struct {
	client *integrationservice.Client
}

// NewServiceNowEscalationService constructs an EscalationService backed by the Choreo API.
func NewServiceNowEscalationService(client *integrationservice.Client) EscalationService {
	return &snEscalationService{client: client}
}

func (s *snEscalationService) SearchEscalations(ctx context.Context, req domain.SearchEscalationsRequest) (domain.SearchEscalationsResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchEscalationsResponse{}, err
	}
	if req.Filters != nil {
		if err := validateUUIDs("caseIds", req.Filters.CaseIDs); err != nil {
			return domain.SearchEscalationsResponse{}, err
		}
	}
	if req.SortBy != nil {
		if req.SortBy.Field != "" && !validEscalationSortField[req.SortBy.Field] {
			return domain.SearchEscalationsResponse{}, &apierror.ValidationError{Msg: "sortBy.field contains invalid value: " + string(req.SortBy.Field)}
		}
		if req.SortBy.Order != "" && !validEscalationSortOrder[req.SortBy.Order] {
			return domain.SearchEscalationsResponse{}, &apierror.ValidationError{Msg: "sortBy.order contains invalid value: " + string(req.SortBy.Order)}
		}
	}

	token := middleware.UserIDTokenFromContext(ctx)

	payload := snSearchEscalationsPayload{
		Pagination: snProjectPagination{Limit: req.Pagination.Limit, Offset: req.Pagination.Offset},
	}
	if req.Filters != nil {
		payload.Filters = &snSearchEscalationsFilters{
			CaseIDs:       uuidsToSysids(req.Filters.CaseIDs),
			CurrentLevels: req.Filters.CurrentLevels,
		}
	}
	if req.SortBy != nil {
		payload.SortBy = &snEscalationSort{Field: string(req.SortBy.Field), Order: string(req.SortBy.Order)}
	}

	raw, err := s.client.Post(ctx, "/escalations/search", token, payload)
	if err != nil {
		return domain.SearchEscalationsResponse{}, err
	}

	var snResp snSearchEscalationsResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.SearchEscalationsResponse{}, fmt.Errorf("sn search escalations: parse response: %w", err)
	}

	escalations := make([]domain.Escalation, 0, len(snResp.Escalations))
	for _, e := range snResp.Escalations {
		escalations = append(escalations, e.toDomain())
	}

	return domain.SearchEscalationsResponse{
		Escalations: escalations,
		Total:       snResp.TotalRecords,
		Limit:       req.Pagination.Limit,
		Offset:      req.Pagination.Offset,
	}, nil
}

// snCreateEscalationPayload mirrors the Choreo POST /escalations request body,
// already normalized (action upper-cased and defaulted to ESCALATE, reason
// defaulted to "") — mirrors the Ballerina reference's own normalizedPayload
// construction in the resource function, done here in the service layer.
type snCreateEscalationPayload struct {
	CaseID string `json:"caseId"`
	Reason string `json:"reason"`
	Action string `json:"action"`
}

// snCreatedEscalation mirrors the Choreo EscalationCreateResponse.escalation
// shape — notably it has no updatedOn, unlike snEscalation (search results).
type snCreatedEscalation struct {
	ID                 string                     `json:"id"`
	Case               snReferenceTableItem       `json:"case"`
	CurrentLevel       snChoiceOption             `json:"currentLevel"`
	PreviousLevel      snChoiceOption             `json:"previousLevel"`
	CreatedBy          string                     `json:"createdBy"`
	CreatedOn          string                     `json:"createdOn"`
	Reason             *string                    `json:"reason"`
	NotificationSentTo []snEscalationNotifiedUser `json:"notificationSentTo"`
}

// snCreateEscalationResponse mirrors the Choreo POST /escalations response.
type snCreateEscalationResponse struct {
	Message    string              `json:"message"`
	Escalation snCreatedEscalation `json:"escalation"`
}

func (s *snEscalationService) CreateEscalation(ctx context.Context, req domain.CreateEscalationRequest) (domain.CreateEscalationResponse, error) {
	if err := validateUUIDs("caseId", []string{req.CaseID}); err != nil {
		return domain.CreateEscalationResponse{}, err
	}

	action := domain.EscalationActionEscalate
	if req.Action != nil {
		action = domain.EscalationAction(strings.ToUpper(string(*req.Action)))
	}
	if action != domain.EscalationActionEscalate && action != domain.EscalationActionDeescalate {
		return domain.CreateEscalationResponse{}, &apierror.ValidationError{
			Msg: fmt.Sprintf("invalid action %q. Allowed actions: %s, %s", action, domain.EscalationActionEscalate, domain.EscalationActionDeescalate),
		}
	}
	reason := ""
	if req.Reason != nil {
		reason = *req.Reason
	}
	if action == domain.EscalationActionEscalate && strings.TrimSpace(reason) == "" {
		return domain.CreateEscalationResponse{}, &apierror.ValidationError{Msg: "reason is required when action is ESCALATE"}
	}

	token := middleware.UserIDTokenFromContext(ctx)

	payload := snCreateEscalationPayload{
		CaseID: uuidToSysid(req.CaseID),
		Reason: reason,
		Action: string(action),
	}

	raw, err := s.client.Post(ctx, "/escalations", token, payload)
	if err != nil {
		return domain.CreateEscalationResponse{}, err
	}

	var snResp snCreateEscalationResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.CreateEscalationResponse{}, fmt.Errorf("sn create escalation: parse response: %w", err)
	}

	return domain.CreateEscalationResponse{
		Message: snResp.Message,
		Escalation: domain.CreatedEscalation{
			ID:                 sysidToUUID(snResp.Escalation.ID),
			Case:               snResp.Escalation.Case.toDomain(),
			CurrentLevel:       snResp.Escalation.CurrentLevel.toDomain(),
			PreviousLevel:      snResp.Escalation.PreviousLevel.toDomain(),
			CreatedBy:          snResp.Escalation.CreatedBy,
			CreatedOn:          snResp.Escalation.CreatedOn,
			Reason:             snResp.Escalation.Reason,
			NotificationSentTo: toDomainEscalationNotifiedUsers(snResp.Escalation.NotificationSentTo),
		},
	}, nil
}
