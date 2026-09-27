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

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/servicenow"
)

// entityProjectsClient is the subset of internal/entity.CustomerEntityClient
// this file needs.
type entityProjectsClient interface {
	SearchProjects(ctx context.Context, body []byte) ([]byte, error)
	GetProject(ctx context.Context, id string) ([]byte, error)
	SearchProjectContacts(ctx context.Context, projectID string, body []byte) ([]byte, error)
}

// entitySearchProjectsRequest mirrors entity-service's own
// domain.SearchProjectsRequest exactly: a FLAT request body (searchQuery/
// accountId are top-level fields, not nested under a "filters" object like
// accounts/cases have) — confirmed against a real running instance
// ("unknown field \"filters\" in request body" when this was nested).
type entitySearchProjectsRequest struct {
	Pagination  entityPagination `json:"pagination"`
	SearchQuery string           `json:"searchQuery,omitempty"`
	AccountID   string           `json:"accountId,omitempty"`
}

type entityProjectView struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Key       string  `json:"key"`
	StartDate *string `json:"startDate"`
	EndDate   *string `json:"endDate"`
}

type entitySearchProjectsResponse struct {
	Projects []entityProjectView `json:"projects"`
	Total    int                 `json:"total"`
}

type entityProjectAccountRef struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Number string `json:"number"`
}

type entityProjectDetailsView struct {
	ID                  string                  `json:"id"`
	Account             entityProjectAccountRef `json:"account"`
	Name                string                  `json:"name"`
	Key                 string                  `json:"key"`
	SubscriptionType    string                  `json:"subscriptionType"`
	StartDate           *string                 `json:"startDate"`
	EndDate             *string                 `json:"endDate"`
	ClosureState        *string                 `json:"closureState"`
	TotalQueryHours     *float64                `json:"totalQueryHours"`
	RemainingQueryHours *float64                `json:"remainingQueryHours"`
}

type entityProjectContact struct {
	Name              *string `json:"name"`
	Email             string  `json:"email"`
	RegistrationState string  `json:"registrationState"`
}

type entitySearchProjectContactsResponse struct {
	Contacts []entityProjectContact `json:"contacts"`
	Total    int                    `json:"total"`
}

// postgresSplProjectClient implements splProjectClient for GetProjects/
// GetProjectByID/GetProjectContacts/GetCasesByProject by calling
// entity-service (Postgres) instead of ServiceNow directly. There is no
// entity-service equivalent of ServiceNow's project "number" (distinct from
// "key") -- the Postgres project table only has key -- so Number is always
// set to Key here, a documented, deliberate substitution, not a bug.
type postgresSplProjectClient struct {
	entity entityProjectsClient
	cases  entityCasesClient
}

// NewPostgresSplProjectClient builds a postgresSplProjectClient. entity and
// cases are typically the same *entity.CustomerEntityClient every other CS
// Portal handler already uses.
func NewPostgresSplProjectClient(entity entityProjectsClient, cases entityCasesClient) *postgresSplProjectClient {
	return &postgresSplProjectClient{entity: entity, cases: cases}
}

func dateOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func hoursOrEmpty(f *float64) string {
	if f == nil {
		return ""
	}
	return fmt.Sprintf("%g", *f)
}

// GetProjects implements splProjectClient.
func (c *postgresSplProjectClient) GetProjects(ctx context.Context, phrase *string, offset, limit int) ([]servicenow.ProjectDetails, error) {
	req := entitySearchProjectsRequest{Pagination: entityPagination{Limit: limit, Offset: offset}}
	if phrase != nil {
		req.SearchQuery = *phrase
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal entity-service projects request: %w", err)
	}
	raw, err := c.entity.SearchProjects(ctx, body)
	if err != nil {
		return nil, err
	}
	var resp entitySearchProjectsResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal entity-service projects response: %w", err)
	}

	projects := make([]servicenow.ProjectDetails, 0, len(resp.Projects))
	for _, p := range resp.Projects {
		projects = append(projects, servicenow.ProjectDetails{
			Number:    p.Key,
			SysID:     p.ID,
			Name:      p.Name,
			Key:       p.Key,
			StartDate: dateOrEmpty(p.StartDate),
			EndDate:   dateOrEmpty(p.EndDate),
		})
	}
	return projects, nil
}

// GetProjectByID implements splProjectClient.
func (c *postgresSplProjectClient) GetProjectByID(ctx context.Context, projectID string) (servicenow.ProjectDetails, error) {
	raw, err := c.entity.GetProject(ctx, projectID)
	if err != nil {
		return servicenow.ProjectDetails{}, err
	}
	var v entityProjectDetailsView
	if err := json.Unmarshal(raw, &v); err != nil {
		return servicenow.ProjectDetails{}, fmt.Errorf("unmarshal entity-service project response: %w", err)
	}
	return servicenow.ProjectDetails{
		Number:              v.Key,
		SysID:               v.ID,
		Name:                v.Name,
		Key:                 v.Key,
		StartDate:           dateOrEmpty(v.StartDate),
		EndDate:             dateOrEmpty(v.EndDate),
		ClosureState:        derefStr(v.ClosureState),
		AccountNumber:       v.Account.Number,
		AccountName:         v.Account.Name,
		TotalQueryHours:     hoursOrEmpty(v.TotalQueryHours),
		RemainingQueryHours: hoursOrEmpty(v.RemainingQueryHours),
	}, nil
}

// GetProjectContacts implements splProjectClient.
func (c *postgresSplProjectClient) GetProjectContacts(ctx context.Context, projectID string, offset, limit int) ([]servicenow.Contact, error) {
	body, err := json.Marshal(struct {
		Pagination entityPagination `json:"pagination"`
	}{Pagination: entityPagination{Limit: limit, Offset: offset}})
	if err != nil {
		return nil, fmt.Errorf("marshal entity-service project contacts request: %w", err)
	}
	raw, err := c.entity.SearchProjectContacts(ctx, projectID, body)
	if err != nil {
		return nil, err
	}
	var resp entitySearchProjectContactsResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal entity-service project contacts response: %w", err)
	}

	contacts := make([]servicenow.Contact, 0, len(resp.Contacts))
	for _, ct := range resp.Contacts {
		contacts = append(contacts, servicenow.Contact{
			ContactName: derefStr(ct.Name),
			Email:       ct.Email,
			State:       ct.RegistrationState,
		})
	}
	return contacts, nil
}

// GetCasesByProject implements splProjectClient. stateFilters/caseTypeFilters
// are accepted for interface compatibility but not applied here -- SPL's
// own ServiceNow-backed implementation only ever calls this with empty
// filters in practice (the frontend's project case list doesn't offer
// those controls), and entity-service's case search has no "type" filter
// at all (confirmed: rejected as unsupported), so silently ignoring rather
// than erroring matches this session's established "known gap" pattern.
func (c *postgresSplProjectClient) GetCasesByProject(ctx context.Context, projectID string, stateFilters, caseTypeFilters []string, offset, limit int) ([]servicenow.CaseDetails, error) {
	req := entitySearchCasesRequest{
		Filters:    entitySearchCasesFilters{Filters: []entityCaseFieldFilter{{Field: "projectId", Op: "in", Values: []string{projectID}}}},
		SortBy:     entityCaseSort{Field: "createdOn", Order: "desc"},
		Pagination: entityPagination{Limit: limit, Offset: offset},
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal entity-service cases-by-project request: %w", err)
	}
	raw, err := c.cases.SearchCases(ctx, body)
	if err != nil {
		return nil, err
	}
	var resp entitySearchCasesResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal entity-service cases-by-project response: %w", err)
	}

	cases := make([]servicenow.CaseDetails, 0, len(resp.Cases))
	for _, v := range resp.Cases {
		cases = append(cases, toCaseDetails(v))
	}
	return cases, nil
}
