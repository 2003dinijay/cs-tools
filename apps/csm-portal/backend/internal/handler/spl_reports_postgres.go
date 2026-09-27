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

package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/servicenow"
)

// entityReportsClient is the subset of internal/entity.CustomerEntityClient
// this file needs.
type entityReportsClient interface {
	SearchProjects(ctx context.Context, body []byte) ([]byte, error)
	GetProject(ctx context.Context, id string) ([]byte, error)
	SearchCases(ctx context.Context, body []byte) ([]byte, error)
	SearchTimeCards(ctx context.Context, body []byte) ([]byte, error)
}

type entityTimeCardRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type entityTimeCardView struct {
	ID          string             `json:"id"`
	TotalTime   float64            `json:"totalTime"`
	WorkDate    string             `json:"workDate"`
	HasBillable bool               `json:"hasBillable"`
	State       *string            `json:"state"`
	User        *entityTimeCardRef `json:"user"`
}

type entitySearchTimeCardsRequest struct {
	Pagination entityPagination            `json:"pagination"`
	Filters    entitySearchTimeCardsFilter `json:"filters"`
}

type entitySearchTimeCardsFilter struct {
	CaseID string `json:"caseId,omitempty"`
}

type entitySearchTimeCardsResponse struct {
	TimeCards []entityTimeCardView `json:"timeCards"`
	Total     int                  `json:"total"`
}

// postgresSplReportsClient implements splReportsClient for
// GetTimeLogBreakdown only, by calling entity-service (Postgres). GetSLAReport
// and GetProjectReportDetails are NOT migrated: both are backed by bespoke
// ServiceNow scoped-app endpoints (/api/wso2/case_sla/report,
// /api/wso2/cs_report/project-insights) computing statistics (SLA percentile
// breakdowns, monthly case counts, subscription/SLA summaries) that don't
// exist in any form on the Postgres side -- building them would mean
// inventing the underlying business rules (which percentile, what counts as
// "response" vs "workaround" vs "resolution" time) from the ServiceNow
// response shape alone, not wiring up already-defined data. Both stay on the
// wrapped ServiceNow client.
type postgresSplReportsClient struct {
	entity entityReportsClient
	sn     splReportsClient
}

// NewPostgresSplReportsClient builds a postgresSplReportsClient. entity is
// typically the same *entity.CustomerEntityClient every other CS Portal
// handler already uses; sn is the existing ServiceNow client, kept for the
// two reports above.
func NewPostgresSplReportsClient(entity entityReportsClient, sn splReportsClient) *postgresSplReportsClient {
	return &postgresSplReportsClient{entity: entity, sn: sn}
}

// GetSLAReport implements splReportsClient by delegating to the wrapped
// ServiceNow client — see this type's own doc comment for why.
func (c *postgresSplReportsClient) GetSLAReport(ctx context.Context, projectSysID, from, to string) (servicenow.SLAReportDetails, error) {
	return c.sn.GetSLAReport(ctx, projectSysID, from, to)
}

// GetProjectReportDetails implements splReportsClient by delegating to the
// wrapped ServiceNow client — see this type's own doc comment for why.
func (c *postgresSplReportsClient) GetProjectReportDetails(ctx context.Context, projectSysID, from, to string) (servicenow.CSReportDetails, error) {
	return c.sn.GetProjectReportDetails(ctx, projectSysID, from, to)
}

// formatHoursMinutes converts a fractional-hours value to ServiceNow's own
// "1h 2m"/"2m" display format (mirrors internal/servicenow/reports.go's
// formatTime, duplicated here rather than exported since it's a small,
// self-contained formatter and this file has no other dependency on that
// package's internals).
func formatHoursMinutes(hours float64) string {
	totalMinutes := int(hours*60 + 0.5) // round to nearest minute
	h := totalMinutes / 60
	m := totalMinutes % 60
	if h == 0 {
		return strconv.Itoa(m) + "m"
	}
	return strconv.Itoa(h) + "h " + strconv.Itoa(m) + "m"
}

// resolveProjectByNumber resolves SPL's projectId (ServiceNow's project
// "number", which Postgres has no distinct equivalent of -- see
// postgresSplProjectClient's own doc comment on why Number==Key here) to
// entity-service's internal project detail. Search-then-exact-match, the
// same pattern used throughout this migration for every number-keyed lookup.
func (c *postgresSplReportsClient) resolveProjectByNumber(ctx context.Context, projectNumber string) (entityProjectDetailsView, error) {
	body, err := json.Marshal(entitySearchProjectsRequest{
		Pagination:  entityPagination{Limit: 5, Offset: 0},
		SearchQuery: projectNumber,
	})
	if err != nil {
		return entityProjectDetailsView{}, fmt.Errorf("marshal entity-service projects request: %w", err)
	}
	raw, err := c.entity.SearchProjects(ctx, body)
	if err != nil {
		return entityProjectDetailsView{}, err
	}
	var resp entitySearchProjectsResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return entityProjectDetailsView{}, fmt.Errorf("unmarshal entity-service projects response: %w", err)
	}
	var projectID string
	for _, p := range resp.Projects {
		if p.Key == projectNumber {
			projectID = p.ID
			break
		}
	}
	if projectID == "" {
		return entityProjectDetailsView{}, servicenow.ErrProjectNotFound
	}

	detailRaw, err := c.entity.GetProject(ctx, projectID)
	if err != nil {
		return entityProjectDetailsView{}, err
	}
	var detail entityProjectDetailsView
	if err := json.Unmarshal(detailRaw, &detail); err != nil {
		return entityProjectDetailsView{}, fmt.Errorf("unmarshal entity-service project detail response: %w", err)
	}
	return detail, nil
}

// GetTimeLogBreakdown implements splReportsClient. Caps at entity-service's
// own hard limit of 50 cases (and 50 time cards per case) — confirmed
// against a real running instance ("limit cannot exceed 50"). A project
// with more cases than that will show a truncated breakdown rather than the
// full history; ServiceNow's own version has no such cap. A documented
// limitation, not a silent bug.
func (c *postgresSplReportsClient) GetTimeLogBreakdown(ctx context.Context, projectID string) (servicenow.TimeLogBreakdownDetails, error) {
	project, err := c.resolveProjectByNumber(ctx, projectID)
	if err != nil {
		return servicenow.TimeLogBreakdownDetails{}, err
	}

	casesBody, err := json.Marshal(entitySearchCasesRequest{
		Filters:    entitySearchCasesFilters{Filters: []entityCaseFieldFilter{{Field: "projectId", Op: "in", Values: []string{project.ID}}}},
		SortBy:     entityCaseSort{Field: "createdOn", Order: "desc"},
		Pagination: entityPagination{Limit: 50, Offset: 0},
	})
	if err != nil {
		return servicenow.TimeLogBreakdownDetails{}, fmt.Errorf("marshal entity-service cases-by-project request: %w", err)
	}
	casesRaw, err := c.entity.SearchCases(ctx, casesBody)
	if err != nil {
		return servicenow.TimeLogBreakdownDetails{}, err
	}
	var casesResp entitySearchCasesResponse
	if err := json.Unmarshal(casesRaw, &casesResp); err != nil {
		return servicenow.TimeLogBreakdownDetails{}, fmt.Errorf("unmarshal entity-service cases-by-project response: %w", err)
	}

	result := servicenow.TimeLogBreakdownDetails{
		ProjectName:         project.Name,
		ProjectKey:          project.Key,
		ProjectType:         project.SubscriptionType,
		RemainingQueryHours: hoursOrEmpty(project.RemainingQueryHours),
		TotalQueryHours:     hoursOrEmpty(project.TotalQueryHours),
		Cases:               make([]servicenow.TimeLogBreakdownCase, 0, len(casesResp.Cases)),
	}

	for _, cv := range casesResp.Cases {
		state := derefStr(cv.State)
		displayState, ok := caseStateToDisplay[state]
		if !ok {
			displayState = state
		}

		tcBody, err := json.Marshal(entitySearchTimeCardsRequest{
			Pagination: entityPagination{Limit: 50, Offset: 0},
			Filters:    entitySearchTimeCardsFilter{CaseID: cv.ID},
		})
		if err != nil {
			return servicenow.TimeLogBreakdownDetails{}, fmt.Errorf("marshal entity-service time-cards request: %w", err)
		}
		tcRaw, err := c.entity.SearchTimeCards(ctx, tcBody)
		if err != nil {
			return servicenow.TimeLogBreakdownDetails{}, err
		}
		var tcResp entitySearchTimeCardsResponse
		if err := json.Unmarshal(tcRaw, &tcResp); err != nil {
			return servicenow.TimeLogBreakdownDetails{}, fmt.Errorf("unmarshal entity-service time-cards response: %w", err)
		}

		caseResult := servicenow.TimeLogBreakdownCase{
			CaseNumber:       cv.Number,
			CaseID:           cv.InternalID,
			ShortDescription: derefStr(cv.Subject),
			State:            displayState,
			TimeCards:        make([]servicenow.TimeCardDetails, 0, len(tcResp.TimeCards)),
		}

		var totalHours, consumedHours float64
		for _, tc := range tcResp.TimeCards {
			totalHours += tc.TotalTime
			tcState := derefStr(tc.State)
			if tc.HasBillable && tcState == "approved" {
				consumedHours += tc.TotalTime
			}
			isBillable := "false"
			if tc.HasBillable {
				isBillable = "true"
			}
			caseResult.TimeCards = append(caseResult.TimeCards, servicenow.TimeCardDetails{
				Total:      formatHoursMinutes(tc.TotalTime),
				CreatedOn:  tc.WorkDate,
				IsBillable: isBillable,
				State:      tcState,
			})
		}
		caseResult.TotalHours = formatHoursMinutes(totalHours)
		caseResult.ConsumedQueryHours = formatHoursMinutes(consumedHours)

		result.Cases = append(result.Cases, caseResult)
	}

	return result, nil
}
