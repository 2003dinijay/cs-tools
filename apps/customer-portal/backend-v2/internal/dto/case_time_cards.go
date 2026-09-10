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

// CaseTimeCardFilters holds the optional filter criteria for
// POST /projects/{id}/cases/time-cards/search — projectIds is never
// client-supplied, always injected from the path by the calling handler.
type CaseTimeCardFilters struct {
	StartDate *string  `json:"startDate,omitempty"`
	EndDate   *string  `json:"endDate,omitempty"`
	States    []string `json:"states,omitempty"`
}

// CaseTimeCardSearchRequest is the portal's request body for
// POST /projects/{id}/cases/time-cards/search.
type CaseTimeCardSearchRequest struct {
	Filters    *CaseTimeCardFilters `json:"filters,omitempty"`
	Pagination entity.Pagination    `json:"pagination"`
}

// BuildEntityCaseTimeCardSearchRequest builds entity-service's
// SearchTimeCardsRequest, forcing filters.projectIds to [projectID].
func BuildEntityCaseTimeCardSearchRequest(projectID string, req CaseTimeCardSearchRequest) entity.SearchTimeCardsRequest {
	filters := &entity.TimeCardFilters{ProjectIDs: []string{projectID}}
	if req.Filters != nil {
		filters.StartDate = req.Filters.StartDate
		filters.EndDate = req.Filters.EndDate
		// Translated, never forwarded — same reason as the sibling
		// BuildEntitySearchTimeCardsRequest. See timeCardStatesToEnums.
		filters.States = timeCardStatesToEnums(req.Filters.States)
	}
	return entity.SearchTimeCardsRequest{Filters: filters, Pagination: req.Pagination}
}

// CaseTimeCardCaseRef is the case reference embedded in a case time-card summary.
type CaseTimeCardCaseRef struct {
	ID        string         `json:"id"`
	Number    string         `json:"number"`
	Name      string         `json:"name"`
	UpdatedOn string         `json:"updatedOn"`
	Project   *ReferenceItem `json:"project"`
	// CreatedBy is what the time-tracking card renders as
	// "Created by <email>" (TimeTrackingCard.tsx); without it the card showed
	// only the case number and title. createdOn/updatedBy come from the same
	// upstream object and complete the audit set the Ballerina response carried.
	CreatedOn *string `json:"createdOn,omitempty"`
	CreatedBy *string `json:"createdBy,omitempty"`
	UpdatedBy *string `json:"updatedBy,omitempty"`
}

// CaseTimeCardBillingInfo is a billable/non-billable time breakdown.
type CaseTimeCardBillingInfo struct {
	TotalTime float64 `json:"totalTime"`
	Count     int     `json:"count"`
}

// CaseTimeCard is the time-card rollup for a single case.
type CaseTimeCard struct {
	Case        CaseTimeCardCaseRef     `json:"case"`
	TotalTime   float64                 `json:"totalTime"`
	TotalCount  int                     `json:"totalCount"`
	Billable    CaseTimeCardBillingInfo `json:"billable"`
	NonBillable CaseTimeCardBillingInfo `json:"nonBillable"`
}

// CaseTimeCardSearchResponse is the portal's response for
// POST /projects/{id}/cases/time-cards/search. TotalRecords (not Total) to
// match the frontend's shared PaginationResponse envelope —
// useSearchProjectCaseTimeCards.ts's getNextPageParam and
// fetchAllCaseTimeCards.ts's export pagination both read totalRecords, so a
// mismatch here stalls "load more" and truncates CSV/PDF exports to the
// first page.
type CaseTimeCardSearchResponse struct {
	CaseTimeCards []CaseTimeCard `json:"caseTimeCards"`
	TotalRecords  int            `json:"totalRecords"`
	Offset        int            `json:"offset"`
	Limit         int            `json:"limit"`
}

// MapCaseTimeCardSearchResponse builds the portal response from entity-service's
// SearchCaseTimeCardsResponse, grouping time-card entries by their case.
func MapCaseTimeCardSearchResponse(r entity.SearchCaseTimeCardsResponse) CaseTimeCardSearchResponse {
	items := make([]CaseTimeCard, 0, len(r.Cases))
	for _, c := range r.Cases {
		item := CaseTimeCard{
			Case: CaseTimeCardCaseRef{
				ID:        c.Case.ID,
				Number:    c.Case.Number,
				Name:      c.Case.Name,
				UpdatedOn: c.Case.UpdatedOn,
				CreatedOn: c.Case.CreatedOn,
				CreatedBy: c.Case.CreatedBy,
				UpdatedBy: c.Case.UpdatedBy,
			},
			TotalTime:   c.TotalTime,
			TotalCount:  c.TotalCount,
			Billable:    CaseTimeCardBillingInfo{TotalTime: c.Billable.TotalTime, Count: c.Billable.Count},
			NonBillable: CaseTimeCardBillingInfo{TotalTime: c.NonBillable.TotalTime, Count: c.NonBillable.Count},
		}
		if c.Case.Project != nil {
			item.Case.Project = &ReferenceItem{ID: c.Case.Project.ID, Label: c.Case.Project.Name}
		}
		items = append(items, item)
	}
	return CaseTimeCardSearchResponse{
		CaseTimeCards: items,
		TotalRecords:  r.Total,
		Offset:        r.Offset,
		Limit:         r.Limit,
	}
}
