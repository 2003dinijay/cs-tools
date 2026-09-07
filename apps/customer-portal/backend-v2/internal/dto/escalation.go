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

package dto

import (
	"strings"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
)

// Escalation actions accepted by POST /cases/{caseId}/escalations.
const (
	EscalationActionEscalate   = "ESCALATE"
	EscalationActionDeescalate = "DEESCALATE"
)

// EscalationCreateRequest is the portal's request body for
// POST /cases/{caseId}/escalations — caseId is never client-supplied, always
// injected from the path by the calling handler.
type EscalationCreateRequest struct {
	Reason *string `json:"reason,omitempty"`
	Action *string `json:"action,omitempty"`
}

// ValidateEscalationAction normalizes req.Action (uppercased, defaulted to
// ESCALATE) and validates it against the allow-list, plus the "reason
// required when escalating" rule. ok is false when the action is invalid or
// a required reason is missing/blank; errMsg is the caller-safe message to
// return as a 400 in that case.
func ValidateEscalationAction(req EscalationCreateRequest) (action string, ok bool, errMsg string) {
	action = EscalationActionEscalate
	if req.Action != nil {
		action = strings.ToUpper(*req.Action)
	}
	if action != EscalationActionEscalate && action != EscalationActionDeescalate {
		return "", false, "Invalid action. Allowed values are ESCALATE or DEESCALATE."
	}
	reason := ""
	if req.Reason != nil {
		reason = *req.Reason
	}
	if action == EscalationActionEscalate && strings.TrimSpace(reason) == "" {
		return "", false, "Reason is required when escalating a case."
	}
	return action, true, ""
}

// BuildEntityCreateEscalationRequest builds entity-service's
// CreateEscalationRequest from the portal request, the case ID injected from
// the path, and the already-normalized action.
func BuildEntityCreateEscalationRequest(caseID, action string, req EscalationCreateRequest) entity.CreateEscalationRequest {
	return entity.CreateEscalationRequest{CaseID: caseID, Reason: req.Reason, Action: &action}
}

// EscalationNotifiedUser is a user notified about an escalation.
type EscalationNotifiedUser struct {
	ID       string  `json:"id"`
	UserName string  `json:"userName"`
	Name     *string `json:"name"`
	Email    *string `json:"email"`
}

func mapEscalationNotifiedUsers(users []entity.EscalationNotifiedUser) []EscalationNotifiedUser {
	out := make([]EscalationNotifiedUser, 0, len(users))
	for _, u := range users {
		out = append(out, EscalationNotifiedUser{ID: u.ID, UserName: u.UserName, Name: u.Name, Email: u.Email})
	}
	return out
}

// CreatedEscalation holds the escalation details returned after creation.
type CreatedEscalation struct {
	ID                 string                   `json:"id"`
	Case               ReferenceItem            `json:"case"`
	CurrentLevel       ReferenceItem            `json:"currentLevel"`
	PreviousLevel      ReferenceItem            `json:"previousLevel"`
	CreatedBy          string                   `json:"createdBy"`
	CreatedOn          string                   `json:"createdOn"`
	Reason             *string                  `json:"reason"`
	NotificationSentTo []EscalationNotifiedUser `json:"notificationSentTo"`
}

// EscalationCreateResponse is the portal's response for POST /cases/{caseId}/escalations.
type EscalationCreateResponse struct {
	Message    string            `json:"message"`
	Escalation CreatedEscalation `json:"escalation"`
}

// MapEscalationCreateResponse builds the portal response from entity-service's
// CreateEscalationResponse.
func MapEscalationCreateResponse(r entity.CreateEscalationResponse) EscalationCreateResponse {
	e := r.Escalation
	return EscalationCreateResponse{
		Message: r.Message,
		Escalation: CreatedEscalation{
			ID:                 e.ID,
			Case:               ReferenceItem{ID: e.Case.ID, Label: e.Case.Name},
			CurrentLevel:       ReferenceItem{ID: e.CurrentLevel.ID, Label: e.CurrentLevel.Label},
			PreviousLevel:      ReferenceItem{ID: e.PreviousLevel.ID, Label: e.PreviousLevel.Label},
			CreatedBy:          e.CreatedBy,
			CreatedOn:          e.CreatedOn,
			Reason:             e.Reason,
			NotificationSentTo: mapEscalationNotifiedUsers(e.NotificationSentTo),
		},
	}
}

// EscalationSearchSort specifies the sort field/order for
// POST /cases/{caseId}/escalations/search.
type EscalationSearchSort struct {
	Field string `json:"field"`
	Order string `json:"order"`
}

// EscalationSearchRequest is the portal's request body for
// POST /cases/{caseId}/escalations/search — caseId is never client-supplied,
// always injected from the path (any caseIds the client sends are discarded),
// by design.
type EscalationSearchRequest struct {
	SortBy     *EscalationSearchSort `json:"sortBy,omitempty"`
	Pagination entity.Pagination     `json:"pagination"`
}

// BuildEntitySearchEscalationsRequest builds entity-service's
// SearchEscalationsRequest, forcing filters.caseIds to [caseID].
func BuildEntitySearchEscalationsRequest(caseID string, req EscalationSearchRequest) entity.SearchEscalationsRequest {
	out := entity.SearchEscalationsRequest{
		Filters:    &entity.SearchEscalationsFilters{CaseIDs: []string{caseID}},
		Pagination: req.Pagination,
	}
	if req.SortBy != nil {
		out.SortBy = &entity.EscalationSort{Field: req.SortBy.Field, Order: req.SortBy.Order}
	}
	return out
}

// Escalation is a single escalation row in search results.
type Escalation struct {
	ID                 string                   `json:"id"`
	Case               ReferenceItem            `json:"case"`
	CurrentLevel       ReferenceItem            `json:"currentLevel"`
	PreviousLevel      ReferenceItem            `json:"previousLevel"`
	CreatedBy          string                   `json:"createdBy"`
	CreatedOn          string                   `json:"createdOn"`
	UpdatedOn          string                   `json:"updatedOn"`
	Reason             *string                  `json:"reason"`
	NotificationSentTo []EscalationNotifiedUser `json:"notificationSentTo"`
}

// EscalationSearchResponse is the portal's response for
// POST /cases/{caseId}/escalations/search.
type EscalationSearchResponse struct {
	Escalations  []Escalation `json:"escalations"`
	TotalRecords int          `json:"totalRecords"`
	Offset       int          `json:"offset"`
	Limit        int          `json:"limit"`
}

// MapEscalationSearchResponse builds the portal response from entity-service's
// SearchEscalationsResponse.
func MapEscalationSearchResponse(r entity.SearchEscalationsResponse) EscalationSearchResponse {
	escalations := make([]Escalation, 0, len(r.Escalations))
	for _, e := range r.Escalations {
		escalations = append(escalations, Escalation{
			ID:                 e.ID,
			Case:               ReferenceItem{ID: e.Case.ID, Label: e.Case.Name},
			CurrentLevel:       ReferenceItem{ID: e.CurrentLevel.ID, Label: e.CurrentLevel.Label},
			PreviousLevel:      ReferenceItem{ID: e.PreviousLevel.ID, Label: e.PreviousLevel.Label},
			CreatedBy:          e.CreatedBy,
			CreatedOn:          e.CreatedOn,
			UpdatedOn:          e.UpdatedOn,
			Reason:             e.Reason,
			NotificationSentTo: mapEscalationNotifiedUsers(e.NotificationSentTo),
		})
	}
	return EscalationSearchResponse{
		Escalations:  escalations,
		TotalRecords: r.Total,
		Offset:       r.Offset,
		Limit:        r.Limit,
	}
}
