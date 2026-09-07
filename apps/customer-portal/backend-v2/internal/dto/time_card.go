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

import "github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"

// timeCardStateLabels supplies portal-facing display text for
// entity-service's TimeCardState enum. Unlike case status/severity, the
// frontend's time-card state filter already sends/expects the plain enum
// string itself as the id (webapp's TimeCardSearchFilters.states: string[]
// — no ServiceNow numeric key involved here), so no id-lookup table is
// needed, just display text for the {id, label} pair.
var timeCardStateLabels = map[string]string{
	"pending":   "Pending",
	"submitted": "Submitted",
	"approved":  "Approved",
	"rejected":  "Rejected",
	"processed": "Processed",
	"recalled":  "Recalled",
}

func timeCardStateRef(state *string) *IDLabelRef {
	if state == nil || *state == "" {
		return nil
	}
	label := timeCardStateLabels[*state]
	if label == "" {
		label = *state
	}
	return &IDLabelRef{ID: *state, Label: label}
}

// TimeCardCaseRef is a reference to the case associated with a time card —
// the frontend's TimeCard.case type is IdLabelRef intersected with a
// required number field.
type TimeCardCaseRef struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Number string `json:"number"`
}

// TimeCardSummary is one item of the portal's response for
// POST /projects/{id}/time-cards/search — shaped to match the frontend's
// own TimeCard type (apps/customer-portal/webapp/src/features/usage-metrics/
// types/timeTracking.ts) field-for-field: CreatedOn (not WorkDate),
// ReportedBy (not User), and State/ApprovedBy/Project/Case all as
// IDLabelRef.
//
// Deliberately excludes entity-service's per-category time breakdowns
// (timeAnalyzing, timeSettingUp, timeReproducingDebugging,
// timeProvidingSolution, timePatching), issueComplexity, workLogComment,
// rejectionReason, and the eligible-approvers list — internal WSO2 support
// bookkeeping never exposed to the customer portal.
type TimeCardSummary struct {
	ID          string           `json:"id"`
	TotalTime   float64          `json:"totalTime"`
	CreatedOn   string           `json:"createdOn"`
	HasBillable bool             `json:"hasBillable"`
	State       *IDLabelRef      `json:"state,omitempty"`
	ReportedBy  *IDLabelRef      `json:"reportedBy,omitempty"`
	ApprovedBy  *IDLabelRef      `json:"approvedBy,omitempty"`
	Project     *IDLabelRef      `json:"project,omitempty"`
	Case        *TimeCardCaseRef `json:"case,omitempty"`
}

// SearchTimeCardsResponse is the portal's response for
// POST /projects/{id}/time-cards/search. TotalRecords (not Total) to match
// the frontend's shared pagination envelope.
type SearchTimeCardsResponse struct {
	TimeCards    []TimeCardSummary `json:"timeCards"`
	TotalRecords int               `json:"totalRecords"`
	Limit        int               `json:"limit"`
	Offset       int               `json:"offset"`
}

// MapSearchTimeCards builds the portal response from entity-service's
// SearchTimeCardsResponse. CreatedOn is populated from WorkDate —
// entity-service's own TimeCardView.CreatedOn is documented as "a
// deprecated alias that always mirrors WorkDate", so reading WorkDate
// directly is equivalent and avoids depending on the deprecated field.
func MapSearchTimeCards(r entity.SearchTimeCardsResponse) SearchTimeCardsResponse {
	items := make([]TimeCardSummary, 0, len(r.TimeCards))
	for _, t := range r.TimeCards {
		items = append(items, TimeCardSummary{
			ID:          t.ID,
			TotalTime:   t.TotalTime,
			CreatedOn:   t.WorkDate,
			HasBillable: t.HasBillable,
			State:       timeCardStateRef(t.State),
			ReportedBy:  timeCardRefToIDLabel(t.User),
			ApprovedBy:  timeCardRefToIDLabel(t.ApprovedBy),
			Project:     timeCardRefToIDLabel(t.Project),
			Case:        mapTimeCardCaseRef(t.Case),
		})
	}
	return SearchTimeCardsResponse{
		TimeCards:    items,
		TotalRecords: r.Total,
		Limit:        r.Limit,
		Offset:       r.Offset,
	}
}

func timeCardRefToIDLabel(r *entity.TimeCardRef) *IDLabelRef {
	if r == nil {
		return nil
	}
	return &IDLabelRef{ID: r.ID, Label: r.Name}
}

func mapTimeCardCaseRef(r *entity.TimeCardCaseRef) *TimeCardCaseRef {
	if r == nil {
		return nil
	}
	return &TimeCardCaseRef{ID: r.ID, Label: r.Name, Number: r.Number}
}

// TimeCardSearchFilters holds the optional filter criteria for
// POST /projects/{id}/time-cards/search, matching the frontend's own
// TimeCardSearchFilters type. No ProjectIDs field: project scoping comes
// exclusively from the {id} path parameter.
type TimeCardSearchFilters struct {
	StartDate *string  `json:"startDate,omitempty"`
	EndDate   *string  `json:"endDate,omitempty"`
	States    []string `json:"states,omitempty"`
}

// TimeCardSearchRequest is the portal's request body for
// POST /projects/{id}/time-cards/search.
type TimeCardSearchRequest struct {
	Filters    TimeCardSearchFilters `json:"filters"`
	SortBy     entity.TimeCardSort   `json:"sortBy"`
	Pagination entity.Pagination     `json:"pagination"`
}

// BuildEntitySearchTimeCardsRequest translates the portal's request into
// entity-service's SearchTimeCardsRequest. projectID (the {id} path
// parameter) always populates Filters.ProjectIDs — never the request body.
func BuildEntitySearchTimeCardsRequest(projectID string, req TimeCardSearchRequest) entity.SearchTimeCardsRequest {
	return entity.SearchTimeCardsRequest{
		Filters: &entity.TimeCardFilters{
			ProjectIDs: []string{projectID},
			StartDate:  req.Filters.StartDate,
			EndDate:    req.Filters.EndDate,
			// Translated, never forwarded: the frontend sends ServiceNow
			// choice-list values ("Approved"), which entity-service rejects
			// with a 400. See timeCardStatesToEnums.
			States: timeCardStatesToEnums(req.Filters.States),
		},
		SortBy:     req.SortBy,
		Pagination: req.Pagination,
	}
}
