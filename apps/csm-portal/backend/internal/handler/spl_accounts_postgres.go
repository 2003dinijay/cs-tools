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

// entityAccountsClient is the subset of internal/entity.CustomerEntityClient
// this file needs.
type entityAccountsClient interface {
	SearchAccounts(ctx context.Context, body []byte) ([]byte, error)
	GetAccount(ctx context.Context, id string) ([]byte, error)
}

type entitySearchAccountsFilters struct {
	SearchQuery string `json:"searchQuery,omitempty"`
	Active      *bool  `json:"active,omitempty"`
	OwnerEmail  string `json:"ownerEmail,omitempty"`
}

type entitySearchAccountsRequest struct {
	Pagination entityPagination            `json:"pagination"`
	Filters    entitySearchAccountsFilters `json:"filters"`
}

type entityPersonRef struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Email *string `json:"email"`
}

type entityAccountView struct {
	ID                     string           `json:"id"`
	Name                   string           `json:"name"`
	Number                 string           `json:"number"`
	Region                 *string          `json:"region"`
	Country                *string          `json:"country"`
	City                   *string          `json:"city"`
	DriveLocation          *string          `json:"driveLocation"`
	ArrToday               *string          `json:"arrToday"`
	TechnicalOwner         *entityPersonRef `json:"technicalOwner"`
	AccountManager         *entityPersonRef `json:"accountManager"`
	CustomerSuccessManager *entityPersonRef `json:"customerSuccessManager"`
	// CreTeam is the account's CRE/ABT team — entity-service's team table is
	// synced from the same ServiceNow sys_user_group source as "group"
	// (cre_team_id points into it), so this id is what SPL's ABT-team-members
	// lookup (GetABTTeamMembers) needs as its teamId.
	CreTeam *entityRef `json:"creTeam"`
}

type entitySearchAccountsResponse struct {
	Accounts []entityAccountView `json:"accounts"`
	Total    int                 `json:"total"`
}

// postgresSplAccountClient implements splAccountClient for GetAccounts/
// GetAccountByID/GetProjectsByAccount by calling entity-service (Postgres)
// instead of ServiceNow directly. GetEscalationsByAccount and EscalateCase
// are NOT migrated: entity-service's own escalation-create path is an
// explicit stub ("creating an escalation is not available on this data
// source: no defined rule for the next escalation level or notification
// recipients exists in this schema"), and escalation-by-account reads have
// no working case-by-account filter to resolve through either — both stay
// on the wrapped ServiceNow client, a real, currently-blocked gap, not an
// oversight.
//
// Known, real gaps carried over from entity-service itself: ARR is nil on
// this data source (see AccountView's own doc comment) -- AccountDetails.ARR
// comes back empty. IntegrationCSTeamName/SysID are populated from CreTeam
// (see toAccountDetails); the remaining IntegrationCSTeam* sub-fields
// (email/manager name/manager email/schedule url) have no Postgres
// equivalent at all and are always empty too.
type postgresSplAccountClient struct {
	entity   entityAccountsClient
	projects entityProjectsClient
	sn       splAccountClient
}

// NewPostgresSplAccountClient builds a postgresSplAccountClient. entity and
// projects are typically the same *entity.CustomerEntityClient every other
// CS Portal handler already uses; sn is the existing ServiceNow client, kept
// for escalations only.
func NewPostgresSplAccountClient(entity entityAccountsClient, projects entityProjectsClient, sn splAccountClient) *postgresSplAccountClient {
	return &postgresSplAccountClient{entity: entity, projects: projects, sn: sn}
}

func personRefName(p *entityPersonRef) string {
	if p == nil {
		return ""
	}
	return p.Name
}

func toAccountDetails(v entityAccountView) servicenow.AccountDetails {
	d := servicenow.AccountDetails{
		Number:                 v.Number,
		Name:                   v.Name,
		Region:                 derefStr(v.Region),
		Country:                derefStr(v.Country),
		City:                   derefStr(v.City),
		DriveLocation:          derefStr(v.DriveLocation),
		ARR:                    derefStr(v.ArrToday),
		AccountManager:         personRefName(v.AccountManager),
		TechnicalOwner:         personRefName(v.TechnicalOwner),
		CustomerSuccessManager: personRefName(v.CustomerSuccessManager),
	}
	// IntegrationCSTeamName/SysID are populated from CreTeam -- the id SPL's
	// ABT-team-members lookup needs (see entityAccountView.CreTeam's own doc
	// comment). ManagerName/ManagerEmail/ScheduleURL have no Postgres
	// equivalent and stay empty.
	if v.CreTeam != nil {
		d.IntegrationCSTeamName = v.CreTeam.Name
		d.IntegrationCSTeamSysID = v.CreTeam.ID
	}
	return d
}

func (c *postgresSplAccountClient) searchAccounts(ctx context.Context, req entitySearchAccountsRequest) (entitySearchAccountsResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return entitySearchAccountsResponse{}, fmt.Errorf("marshal entity-service accounts request: %w", err)
	}
	raw, err := c.entity.SearchAccounts(ctx, body)
	if err != nil {
		return entitySearchAccountsResponse{}, err
	}
	var resp entitySearchAccountsResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return entitySearchAccountsResponse{}, fmt.Errorf("unmarshal entity-service accounts response: %w", err)
	}
	return resp, nil
}

// GetAccounts implements splAccountClient. userType (TO/AM distinction) is
// not modeled -- entity-service's OwnerEmail filter matches any of
// technical owner/account manager/renewal account manager, same as
// ServiceNow's own default (no userType) behavior; a caller-specified
// userType is accepted but not narrowed further, a documented simplification.
func (c *postgresSplAccountClient) GetAccounts(ctx context.Context, email, userType, phrase *string, offset, limit int, active bool) ([]servicenow.AccountDetails, error) {
	req := entitySearchAccountsRequest{Pagination: entityPagination{Limit: limit, Offset: offset}, Filters: entitySearchAccountsFilters{Active: &active}}
	if email != nil {
		req.Filters.OwnerEmail = *email
	}
	if phrase != nil {
		req.Filters.SearchQuery = *phrase
	}
	resp, err := c.searchAccounts(ctx, req)
	if err != nil {
		return nil, err
	}
	accounts := make([]servicenow.AccountDetails, 0, len(resp.Accounts))
	for _, v := range resp.Accounts {
		accounts = append(accounts, toAccountDetails(v))
	}
	return accounts, nil
}

// GetAccountByID implements splAccountClient. accountNumber is SPL's
// ServiceNow-style account number, not entity-service's internal UUID --
// resolved via search-then-exact-match, the same pattern
// postgresSplCaseClient.resolveCaseByNumber already uses for cases.
func (c *postgresSplAccountClient) GetAccountByID(ctx context.Context, accountNumber string) (servicenow.AccountDetails, error) {
	resp, err := c.searchAccounts(ctx, entitySearchAccountsRequest{
		Pagination: entityPagination{Limit: 5, Offset: 0},
		Filters:    entitySearchAccountsFilters{SearchQuery: accountNumber},
	})
	if err != nil {
		return servicenow.AccountDetails{}, err
	}
	for _, v := range resp.Accounts {
		if v.Number == accountNumber {
			return toAccountDetails(v), nil
		}
	}
	return servicenow.AccountDetails{}, servicenow.ErrAccountNotFound
}

// GetProjectsByAccount implements splAccountClient. Resolves accountNumber
// to entity-service's internal account UUID first (search-then-exact-match,
// same as GetAccountByID), then searches projects filtered by that account
// id -- project.account_id already existed, but the filter itself had to be
// wired in (see project_repo.go's own comment on this).
func (c *postgresSplAccountClient) GetProjectsByAccount(ctx context.Context, accountNumber string, offset, limit int) ([]servicenow.ProjectDetails, error) {
	accountResp, err := c.searchAccounts(ctx, entitySearchAccountsRequest{
		Pagination: entityPagination{Limit: 5, Offset: 0},
		Filters:    entitySearchAccountsFilters{SearchQuery: accountNumber},
	})
	if err != nil {
		return nil, err
	}
	var accountID string
	for _, v := range accountResp.Accounts {
		if v.Number == accountNumber {
			accountID = v.ID
			break
		}
	}
	if accountID == "" {
		return nil, servicenow.ErrAccountNotFound
	}

	body, err := json.Marshal(entitySearchProjectsRequest{
		Pagination: entityPagination{Limit: limit, Offset: offset},
		AccountID:  accountID,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal entity-service projects-by-account request: %w", err)
	}
	raw, err := c.projects.SearchProjects(ctx, body)
	if err != nil {
		return nil, err
	}
	var resp entitySearchProjectsResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal entity-service projects-by-account response: %w", err)
	}
	projects := make([]servicenow.ProjectDetails, 0, len(resp.Projects))
	for _, p := range resp.Projects {
		projects = append(projects, servicenow.ProjectDetails{
			Number:        p.Key,
			SysID:         p.ID,
			Name:          p.Name,
			Key:           p.Key,
			StartDate:     dateOrEmpty(p.StartDate),
			EndDate:       dateOrEmpty(p.EndDate),
			AccountNumber: accountNumber,
		})
	}
	return projects, nil
}

// GetEscalationsByAccount implements splAccountClient by delegating to the
// wrapped ServiceNow client — see this type's own doc comment for why.
func (c *postgresSplAccountClient) GetEscalationsByAccount(ctx context.Context, accountNumber string, offset, limit int) ([]servicenow.EscalationDetail, error) {
	return c.sn.GetEscalationsByAccount(ctx, accountNumber, offset, limit)
}

// EscalateCase implements splAccountClient by delegating to the wrapped
// ServiceNow client — see this type's own doc comment for why.
func (c *postgresSplAccountClient) EscalateCase(ctx context.Context, accountNumber, caseNumber string, request servicenow.EscalationRequest, submittedByEmail string) (servicenow.EscalationResponse, error) {
	return c.sn.EscalateCase(ctx, accountNumber, caseNumber, request, submittedByEmail)
}
