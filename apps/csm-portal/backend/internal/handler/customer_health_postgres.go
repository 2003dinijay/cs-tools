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
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/servicenow"
)

// entityCustomerHealthClient is the subset of internal/entity.CustomerEntityClient
// needed to compute customer-health signals from Postgres.
type entityCustomerHealthClient interface {
	SearchAccounts(ctx context.Context, body []byte) ([]byte, error)
	SearchProjects(ctx context.Context, body []byte) ([]byte, error)
	SearchCases(ctx context.Context, body []byte) ([]byte, error)
	SearchDeployments(ctx context.Context, body []byte) ([]byte, error)
	SearchDeployedProducts(ctx context.Context, body []byte) ([]byte, error)
	SearchEscalations(ctx context.Context, body []byte) ([]byte, error)
}

// customerHealthRecentWindow/customerHealthDelayWindow/customerHealthAbandonedWindow
// are this file's own judgment calls, not a ported ServiceNow spec: the
// original implementation (SupportPortalLite's getCustomerHealthSummary/
// getCustomerHealthDetail) called a bespoke ServiceNow scoped-app script
// (/api/wso2/customerhealthanalysis/...) with no accessible source anywhere
// in this codebase or the Ballerina backend it was ported from -- confirmed
// by reading both directly. The seven flags these windows back
// (hasRecentCases/noSupportCases6mo/hasAbandonedMigrations/hasMigrationDelays/
// hasRecentEscalations) are genuinely derivable from real, populated Postgres
// data (checked directly against staging: 3,820 cases created in the last
// 180 days, 76 MIGRATION-type engagements, 349+ account-linked escalations),
// but the exact thresholds ServiceNow used are not visible anywhere, so these
// are reasonable approximations rather than a faithful port. Revisit if a
// product owner specifies different windows.
const (
	customerHealthRecentWindow     = 180 * 24 * time.Hour // "recent cases" / "no support cases" cutoff
	customerHealthEscalationWindow = 90 * 24 * time.Hour  // "recent escalations" cutoff
	customerHealthDelayWindow      = 60 * 24 * time.Hour  // a still-open MIGRATION engagement older than this is "delayed"
	customerHealthAbandonedWindow  = 180 * 24 * time.Hour // a still-open MIGRATION engagement older than this is "abandoned"
)

// customerHealthCasePageCap/customerHealthCasePageLimit bound the per-account/
// per-project case-id pagination this file does for drill-down lists and
// escalation lookups, mirroring the identical pattern in
// accounts_postgres.go's accountCaseIDs.
const (
	customerHealthCasePageLimit = 50
	customerHealthCasePageCap   = 10 // 10 * 50 = 500 cases per account/project
)

// postgresCustomerHealthClient implements customerHealthSNClient entirely
// against entity-service (Postgres) -- no ServiceNow dependency at all. The
// original ServiceNow implementation computed these seven account/project
// risk flags (hasNoGoLive, hasRecentCases, hasEolProduct,
// hasAbandonedMigrations, hasMigrationDelays, hasRecentEscalations,
// noSupportCases6mo) inside a bespoke scoped-app script this codebase has no
// access to; this type derives the same signals from real, already-synced
// Postgres columns instead (project.onboarding_status,
// deployed_product's product_version.support_eol_date, work_item.account_id,
// case_escalation, engagement.type='MIGRATION') -- see the window constants
// above for the judgment calls involved.
type postgresCustomerHealthClient struct {
	entity entityCustomerHealthClient
}

// NewPostgresCustomerHealthClient builds a postgresCustomerHealthClient.
// entity is the same *entity.CustomerEntityClient every other CS Portal
// handler uses.
func NewPostgresCustomerHealthClient(entity entityCustomerHealthClient) *postgresCustomerHealthClient {
	return &postgresCustomerHealthClient{entity: entity}
}

// customerHealthCaseLikeTypes mirrors the case-like work_item types
// entity-service's own case search already treats as one family (see that
// service's own CLAUDE.md, "Case-like work_item types").
var customerHealthCaseLikeTypes = []string{"case", "service_request", "engagement", "security_report_analysis"}

// GetCustomerHealthSummary implements customerHealthSNClient. product/abtTeam/
// risks are accepted (matching the existing interface every caller already
// uses) but not applied as filters: none of them map onto an account-level
// column or an already-indexed join entity-service exposes today (which
// product an account's projects deploy, which ABT team "owns" an account, and
// the original "risks" free-text filter all need either a new entity-service
// query shape or a product decision about what they should mean on this data
// source) -- a known, explicitly flagged gap, same posture this codebase
// already uses elsewhere for a filter with no backing column (see
// changeRequestWhereClause's own doc comment in entity-service). email/phrase/
// region are applied.
func (c *postgresCustomerHealthClient) GetCustomerHealthSummary(ctx context.Context, email, phrase, risks *string, region []string, product, abtTeam *string, offset, limit int) (*servicenow.AccountSummaryResponse, error) {
	accounts, total, err := c.filteredAccounts(ctx, email, phrase, region, offset, limit)
	if err != nil {
		return nil, err
	}

	data := make([]servicenow.AccountSummary, 0, len(accounts))
	for _, a := range accounts {
		flags, err := c.accountFlags(ctx, a.ID)
		if err != nil {
			return nil, err
		}
		name := a.Name
		data = append(data, servicenow.AccountSummary{
			AccountSysID:           a.ID,
			AccountName:            &name,
			HasNoGoLive:            flags.goLiveStatus,
			HasRecentCases:         flags.hasRecentCases,
			HasEolProduct:          flags.hasEolProduct,
			HasAbandonedMigrations: flags.hasAbandonedMigrations,
			HasMigrationDelays:     flags.hasMigrationDelays,
			HasRecentEscalations:   flags.hasRecentEscalations,
			NoSupportCases6mo:      !flags.hasRecentCases,
		})
	}
	return &servicenow.AccountSummaryResponse{Data: data, TotalCount: total}, nil
}

// GetCustomerHealthDetail implements customerHealthSNClient: the same seven
// signals as GetCustomerHealthSummary, computed per-project instead of
// per-account, with drill-down lists built from the same case/deployed-
// product data each flag's own query already fetched.
func (c *postgresCustomerHealthClient) GetCustomerHealthDetail(ctx context.Context, accountID string) (*servicenow.AccountDetail, error) {
	accountBody, err := json.Marshal(entitySearchAccountsRequest{
		Pagination: entityPagination{Limit: 1, Offset: 0},
		Filters:    entitySearchAccountsFilters{SearchQuery: accountID},
	})
	if err != nil {
		return nil, fmt.Errorf("marshal entity-service account request: %w", err)
	}
	accountName := ""
	if raw, err := c.entity.SearchAccounts(ctx, accountBody); err == nil {
		var resp entitySearchAccountsResponse
		if json.Unmarshal(raw, &resp) == nil {
			for _, a := range resp.Accounts {
				if a.ID == accountID {
					accountName = a.Name
					break
				}
			}
		}
	}

	projects, err := c.accountProjects(ctx, accountID)
	if err != nil {
		return nil, err
	}

	details := make([]servicenow.ProjectDetail, 0, len(projects))
	for _, p := range projects {
		detail, err := c.projectDetail(ctx, accountID, p)
		if err != nil {
			return nil, err
		}
		details = append(details, detail)
	}

	return &servicenow.AccountDetail{AccountName: accountName, CustomerProjects: details}, nil
}

// customerHealthAccountFlags is the per-account summary-level result of
// accountFlags -- an aggregation across every one of the account's projects.
type customerHealthAccountFlags struct {
	goLiveStatus           servicenow.GoLiveStatus
	hasRecentCases         bool
	hasEolProduct          bool
	hasAbandonedMigrations bool
	hasMigrationDelays     bool
	hasRecentEscalations   bool
}

// filteredAccounts resolves the page of accounts GetCustomerHealthSummary
// should compute flags for. email/phrase map onto entity-service's own
// SearchAccounts filters directly; region has no backing SearchAccounts
// filter (checked directly: SearchAccountsFilters has searchQuery/active/pod/
// ownerEmail/classification only, no region), so it's applied client-side
// over a bounded working set instead.
func (c *postgresCustomerHealthClient) filteredAccounts(ctx context.Context, email, phrase *string, region []string, offset, limit int) ([]entityAccountView, int, error) {
	filters := entitySearchAccountsFilters{}
	if phrase != nil {
		filters.SearchQuery = *phrase
	}
	if email != nil {
		filters.OwnerEmail = *email
	}

	if len(region) == 0 {
		body, err := json.Marshal(entitySearchAccountsRequest{
			Pagination: entityPagination{Limit: limit, Offset: offset},
			Filters:    filters,
		})
		if err != nil {
			return nil, 0, fmt.Errorf("marshal entity-service accounts request: %w", err)
		}
		raw, err := c.entity.SearchAccounts(ctx, body)
		if err != nil {
			return nil, 0, err
		}
		var resp entitySearchAccountsResponse
		if err := json.Unmarshal(raw, &resp); err != nil {
			return nil, 0, fmt.Errorf("unmarshal entity-service accounts response: %w", err)
		}
		return resp.Accounts, resp.Total, nil
	}

	wantRegion := make(map[string]bool, len(region))
	for _, r := range region {
		wantRegion[r] = true
	}
	var matched []entityAccountView
	const (
		accountScanPageLimit = 50
		accountScanPageCap   = 40 // 40 * 50 = 2000 accounts scanned
	)
	for page := 0; page < accountScanPageCap; page++ {
		body, err := json.Marshal(entitySearchAccountsRequest{
			Pagination: entityPagination{Limit: accountScanPageLimit, Offset: page * accountScanPageLimit},
			Filters:    filters,
		})
		if err != nil {
			return nil, 0, fmt.Errorf("marshal entity-service accounts request: %w", err)
		}
		raw, err := c.entity.SearchAccounts(ctx, body)
		if err != nil {
			return nil, 0, err
		}
		var resp entitySearchAccountsResponse
		if err := json.Unmarshal(raw, &resp); err != nil {
			return nil, 0, fmt.Errorf("unmarshal entity-service accounts response: %w", err)
		}
		for _, a := range resp.Accounts {
			if a.Region != nil && wantRegion[*a.Region] {
				matched = append(matched, a)
			}
		}
		if len(resp.Accounts) < accountScanPageLimit || (page+1)*accountScanPageLimit >= resp.Total {
			break
		}
	}
	total := len(matched)
	start := min(offset, total)
	end := min(offset+limit, total)
	if end < start {
		end = start
	}
	return matched[start:end], total, nil
}

// accountProjects pages through an account's own projects.
func (c *postgresCustomerHealthClient) accountProjects(ctx context.Context, accountID string) ([]entityProjectView, error) {
	var projects []entityProjectView
	const (
		pageLimit = 50
		pageCap   = 10 // 500 projects per account
	)
	for page := 0; page < pageCap; page++ {
		body, err := json.Marshal(entitySearchProjectsRequest{
			Pagination: entityPagination{Limit: pageLimit, Offset: page * pageLimit},
			AccountID:  accountID,
		})
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
		projects = append(projects, resp.Projects...)
		if len(resp.Projects) < pageLimit || (page+1)*pageLimit >= resp.Total {
			break
		}
	}
	return projects, nil
}

// accountFlags computes the seven summary-level flags for one account,
// aggregating across every one of its projects (any project flagged ->
// the account is flagged) for the project-scoped signals (go-live,
// EOL product, migration delays/abandonment), and account-wide queries for
// the case/escalation-based ones.
func (c *postgresCustomerHealthClient) accountFlags(ctx context.Context, accountID string) (customerHealthAccountFlags, error) {
	projects, err := c.accountProjects(ctx, accountID)
	if err != nil {
		return customerHealthAccountFlags{}, err
	}

	noGoLive := false
	for _, p := range projects {
		if p.OnboardingStatus != nil && (*p.OnboardingStatus == "NOT_STARTED" || *p.OnboardingStatus == "IN_PROGRESS") {
			noGoLive = true
			break
		}
	}

	hasEolProduct := false
	for _, p := range projects {
		eol, err := c.projectHasEolProduct(ctx, p.ID)
		if err != nil {
			return customerHealthAccountFlags{}, err
		}
		if eol {
			hasEolProduct = true
			break
		}
	}

	hasRecentCases, err := c.accountHasCasesSince(ctx, accountID, time.Now().Add(-customerHealthRecentWindow))
	if err != nil {
		return customerHealthAccountFlags{}, err
	}

	hasAbandoned, hasDelayed, err := c.accountMigrationFlags(ctx, accountID)
	if err != nil {
		return customerHealthAccountFlags{}, err
	}

	hasRecentEscalations, err := c.accountHasRecentEscalations(ctx, accountID)
	if err != nil {
		return customerHealthAccountFlags{}, err
	}

	status := servicenow.GoLiveStatus{Status: "Live", IsRisk: false, State: "healthy"}
	if noGoLive {
		status = servicenow.GoLiveStatus{Status: "No Go-Live", IsRisk: true, State: "risk"}
	}

	return customerHealthAccountFlags{
		goLiveStatus:           status,
		hasRecentCases:         hasRecentCases,
		hasEolProduct:          hasEolProduct,
		hasAbandonedMigrations: hasAbandoned,
		hasMigrationDelays:     hasDelayed,
		hasRecentEscalations:   hasRecentEscalations,
	}, nil
}

// projectHasEolProduct checks whether any deployed product under projectID's
// own deployments is running a product version past its support_eol_date.
func (c *postgresCustomerHealthClient) projectHasEolProduct(ctx context.Context, projectID string) (bool, error) {
	deploymentIDs, err := c.projectDeploymentIDs(ctx, projectID)
	if err != nil || len(deploymentIDs) == 0 {
		return false, err
	}
	eol, _, err := c.deployedEolProducts(ctx, deploymentIDs)
	return len(eol) > 0, err
}

func (c *postgresCustomerHealthClient) projectDeploymentIDs(ctx context.Context, projectID string) ([]string, error) {
	body, err := json.Marshal(entityDeploymentsSearchRequest{
		Pagination: entityPagination{Limit: 50, Offset: 0},
		ProjectIDs: []string{projectID},
	})
	if err != nil {
		return nil, fmt.Errorf("marshal entity-service deployments request: %w", err)
	}
	raw, err := c.entity.SearchDeployments(ctx, body)
	if err != nil {
		return nil, err
	}
	var resp entityDeploymentsSearchResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal entity-service deployments response: %w", err)
	}
	ids := make([]string, len(resp.Deployments))
	for i, d := range resp.Deployments {
		ids[i] = d.ID
	}
	return ids, nil
}

// deployedEolProducts returns the deployed products under deploymentIDs whose
// product version is past its support_eol_date, alongside every deployed
// product (used by projectDetail to build the "softwareModel" drill-down).
func (c *postgresCustomerHealthClient) deployedEolProducts(ctx context.Context, deploymentIDs []string) ([]entityDeployedProductEolView, []entityDeployedProductEolView, error) {
	body, err := json.Marshal(entitySearchDeployedProductsEolRequest{
		Pagination:    entityPagination{Limit: 50, Offset: 0},
		DeploymentIDs: deploymentIDs,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("marshal entity-service deployed-products request: %w", err)
	}
	raw, err := c.entity.SearchDeployedProducts(ctx, body)
	if err != nil {
		return nil, nil, err
	}
	var resp entitySearchDeployedProductsEolResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, nil, fmt.Errorf("unmarshal entity-service deployed-products response: %w", err)
	}
	now := time.Now()
	var eol []entityDeployedProductEolView
	for _, dp := range resp.DeployedProducts {
		if dp.Version != nil && dp.Version.SupportEoLDate != nil {
			if t, err := time.Parse(time.RFC3339, *dp.Version.SupportEoLDate); err == nil && t.Before(now) {
				eol = append(eol, dp)
			}
		}
	}
	return eol, resp.DeployedProducts, nil
}

// accountHasCasesSince reports whether the account has any case-like work
// item created at or after since.
func (c *postgresCustomerHealthClient) accountHasCasesSince(ctx context.Context, accountID string, since time.Time) (bool, error) {
	total, err := c.caseCount(ctx, accountID, "", []entityCaseFieldFilter{
		{Field: "createdOn", Op: "gte", Values: []string{since.UTC().Format(time.RFC3339)}},
	})
	return total > 0, err
}

// accountMigrationFlags reports whether the account has a still-open
// MIGRATION-type engagement old enough to count as "abandoned"
// (customerHealthAbandonedWindow) or merely "delayed" (customerHealthDelayWindow).
func (c *postgresCustomerHealthClient) accountMigrationFlags(ctx context.Context, accountID string) (abandoned, delayed bool, err error) {
	now := time.Now()
	abandonedCount, err := c.caseCount(ctx, accountID, "engagement", []entityCaseFieldFilter{
		{Field: "engagementType", Op: "eq", Values: []string{"migration"}},
		{Field: "state", Op: "notIn", Values: []string{"closed"}},
		{Field: "createdOn", Op: "lte", Values: []string{now.Add(-customerHealthAbandonedWindow).UTC().Format(time.RFC3339)}},
	})
	if err != nil {
		return false, false, err
	}
	delayedCount, err := c.caseCount(ctx, accountID, "engagement", []entityCaseFieldFilter{
		{Field: "engagementType", Op: "eq", Values: []string{"migration"}},
		{Field: "state", Op: "notIn", Values: []string{"closed"}},
		{Field: "createdOn", Op: "lte", Values: []string{now.Add(-customerHealthDelayWindow).UTC().Format(time.RFC3339)}},
	})
	if err != nil {
		return false, false, err
	}
	return abandonedCount > 0, delayedCount > 0, nil
}

// caseCount returns the total matching count for an accountId-scoped case
// search, optionally narrowed to one work-item type, plus any extra filters
// -- a limit:1 request, reading only the response's own Total rather than
// fetching the matching rows.
func (c *postgresCustomerHealthClient) caseCount(ctx context.Context, accountID, caseType string, extra []entityCaseFieldFilter) (int, error) {
	filters := []entityCaseFieldFilter{{Field: "accountId", Op: "eq", Values: []string{accountID}}}
	if caseType != "" {
		filters = append(filters, entityCaseFieldFilter{Field: "type", Op: "eq", Values: []string{caseType}})
	}
	filters = append(filters, extra...)

	body, err := json.Marshal(entitySearchCasesRequest{
		Filters:    entitySearchCasesFilters{Filters: filters},
		Pagination: entityPagination{Limit: 1, Offset: 0},
	})
	if err != nil {
		return 0, fmt.Errorf("marshal entity-service cases request: %w", err)
	}
	raw, err := c.entity.SearchCases(ctx, body)
	if err != nil {
		return 0, err
	}
	var resp entitySearchCasesResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return 0, fmt.Errorf("unmarshal entity-service cases response: %w", err)
	}
	return resp.Total, nil
}

// accountCaseIDsForHealth pages through every case-like work item belonging
// to accountID, up to customerHealthCasePageCap pages -- the same shape as
// accounts_postgres.go's accountCaseIDs, duplicated locally rather than
// shared since the two files' pagination bounds are tuned independently and
// there is no third caller yet to justify extracting a shared helper.
func (c *postgresCustomerHealthClient) accountCaseIDsForHealth(ctx context.Context, accountID string) ([]string, error) {
	var ids []string
	for page := 0; page < customerHealthCasePageCap; page++ {
		body, err := json.Marshal(entitySearchCasesRequest{
			Filters:    entitySearchCasesFilters{Filters: []entityCaseFieldFilter{{Field: "accountId", Op: "eq", Values: []string{accountID}}}},
			Pagination: entityPagination{Limit: customerHealthCasePageLimit, Offset: page * customerHealthCasePageLimit},
		})
		if err != nil {
			return nil, fmt.Errorf("marshal entity-service cases request: %w", err)
		}
		raw, err := c.entity.SearchCases(ctx, body)
		if err != nil {
			return nil, err
		}
		var resp entitySearchCasesResponse
		if err := json.Unmarshal(raw, &resp); err != nil {
			return nil, fmt.Errorf("unmarshal entity-service cases response: %w", err)
		}
		for _, cv := range resp.Cases {
			ids = append(ids, cv.ID)
		}
		if len(resp.Cases) < customerHealthCasePageLimit || (page+1)*customerHealthCasePageLimit >= resp.Total {
			break
		}
	}
	return ids, nil
}

// accountHasRecentEscalations reports whether any of the account's cases has
// an escalation created within customerHealthEscalationWindow.
func (c *postgresCustomerHealthClient) accountHasRecentEscalations(ctx context.Context, accountID string) (bool, error) {
	caseIDs, err := c.accountCaseIDsForHealth(ctx, accountID)
	if err != nil || len(caseIDs) == 0 {
		return false, err
	}
	body, err := json.Marshal(entitySearchEscalationsRequest{
		Filters:    entitySearchEscalationsFilters{CaseIDs: caseIDs},
		SortBy:     entityEscalationSort{Field: "createdOn", Order: "desc"},
		Pagination: entityPagination{Limit: 50, Offset: 0},
	})
	if err != nil {
		return false, fmt.Errorf("marshal entity-service escalations request: %w", err)
	}
	raw, err := c.entity.SearchEscalations(ctx, body)
	if err != nil {
		return false, err
	}
	var resp entitySearchEscalationsResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return false, fmt.Errorf("unmarshal entity-service escalations response: %w", err)
	}
	cutoff := time.Now().Add(-customerHealthEscalationWindow)
	for _, e := range resp.Escalations {
		if t, err := time.Parse(time.RFC3339, e.CreatedOn); err == nil && t.After(cutoff) {
			return true, nil
		}
	}
	return false, nil
}

// projectDetail builds one project's ProjectDetail row for GetCustomerHealthDetail,
// including the drill-down lists (recent cases by priority, abandoned/delayed
// migration case links, EOL software models, escalated case links).
func (c *postgresCustomerHealthClient) projectDetail(ctx context.Context, accountID string, p entityProjectView) (servicenow.ProjectDetail, error) {
	since := time.Now().Add(-customerHealthRecentWindow)
	recentCases, err := c.projectCases(ctx, p.ID, "", []entityCaseFieldFilter{
		{Field: "createdOn", Op: "gte", Values: []string{since.UTC().Format(time.RFC3339)}},
	})
	if err != nil {
		return servicenow.ProjectDetail{}, err
	}
	recentGroups := groupCasesByPriority(recentCases)

	now := time.Now()
	abandonedCases, err := c.projectCases(ctx, p.ID, "engagement", []entityCaseFieldFilter{
		{Field: "engagementType", Op: "eq", Values: []string{"migration"}},
		{Field: "state", Op: "notIn", Values: []string{"closed"}},
		{Field: "createdOn", Op: "lte", Values: []string{now.Add(-customerHealthAbandonedWindow).UTC().Format(time.RFC3339)}},
	})
	if err != nil {
		return servicenow.ProjectDetail{}, err
	}
	delayedCases, err := c.projectCases(ctx, p.ID, "engagement", []entityCaseFieldFilter{
		{Field: "engagementType", Op: "eq", Values: []string{"migration"}},
		{Field: "state", Op: "notIn", Values: []string{"closed"}},
		{Field: "createdOn", Op: "lte", Values: []string{now.Add(-customerHealthDelayWindow).UTC().Format(time.RFC3339)}},
	})
	if err != nil {
		return servicenow.ProjectDetail{}, err
	}

	deploymentIDs, err := c.projectDeploymentIDs(ctx, p.ID)
	if err != nil {
		return servicenow.ProjectDetail{}, err
	}
	var eolDeployments []entityDeployedProductEolView
	var deployments []entityDeployedProductEolView
	if len(deploymentIDs) > 0 {
		eolDeployments, deployments, err = c.deployedEolProducts(ctx, deploymentIDs)
		if err != nil {
			return servicenow.ProjectDetail{}, err
		}
	}
	softwareModel := make([]servicenow.EolProduct, 0, len(eolDeployments))
	for _, dp := range eolDeployments {
		eolDate := ""
		if dp.Version != nil && dp.Version.SupportEoLDate != nil {
			eolDate = *dp.Version.SupportEoLDate
		}
		softwareModel = append(softwareModel, servicenow.EolProduct{
			Name:    dp.Product.Name,
			EolDate: eolDate,
		})
	}
	deploymentRefs := make([]servicenow.Deployment, 0, len(deploymentIDs))
	for _, id := range deploymentIDs {
		deploymentRefs = append(deploymentRefs, servicenow.Deployment{SysID: id})
	}
	_ = deployments // every deployed product under the project; not surfaced beyond the EOL subset today

	escalatedCases, err := c.projectEscalatedCases(ctx, p.ID)
	if err != nil {
		return servicenow.ProjectDetail{}, err
	}

	noGoLive := p.OnboardingStatus != nil && (*p.OnboardingStatus == "NOT_STARTED" || *p.OnboardingStatus == "IN_PROGRESS")
	status := servicenow.GoLiveStatus{Status: "Live", IsRisk: false, State: "healthy"}
	if noGoLive {
		status = servicenow.GoLiveStatus{Status: "No Go-Live", IsRisk: true, State: "risk"}
	}

	return servicenow.ProjectDetail{
		SysID:                   p.ID,
		Name:                    p.Name,
		HasRecentCases:          len(recentCases) > 0,
		DetailedRecentCases:     recentGroups,
		TotalRecentCases:        len(recentCases),
		HasAbandonedCases:       len(abandonedCases) > 0,
		DetailedAbandonedCases:  caseLinks(abandonedCases),
		IsUsingEolProduct:       len(eolDeployments) > 0,
		SoftwareModel:           softwareModel,
		Deployments:             deploymentRefs,
		HasMigrationDelays:      len(delayedCases) > 0,
		DetailedMigrationDelays: caseLinks(delayedCases),
		HasEscalatedCases:       len(escalatedCases) > 0,
		DetailedEscalatedCases:  escalatedCases,
		GoLiveStatus:            status,
	}, nil
}

// projectCases returns every case-like work item (optionally narrowed to one
// type) matching the given extra filters for projectID, up to
// customerHealthCasePageCap pages.
func (c *postgresCustomerHealthClient) projectCases(ctx context.Context, projectID, caseType string, extra []entityCaseFieldFilter) ([]entitySearchCaseView, error) {
	filters := []entityCaseFieldFilter{{Field: "projectId", Op: "eq", Values: []string{projectID}}}
	if caseType != "" {
		filters = append(filters, entityCaseFieldFilter{Field: "type", Op: "eq", Values: []string{caseType}})
	}
	filters = append(filters, extra...)

	var cases []entitySearchCaseView
	for page := 0; page < customerHealthCasePageCap; page++ {
		body, err := json.Marshal(entitySearchCasesRequest{
			Filters:    entitySearchCasesFilters{Filters: filters},
			Pagination: entityPagination{Limit: customerHealthCasePageLimit, Offset: page * customerHealthCasePageLimit},
		})
		if err != nil {
			return nil, fmt.Errorf("marshal entity-service cases request: %w", err)
		}
		raw, err := c.entity.SearchCases(ctx, body)
		if err != nil {
			return nil, err
		}
		var resp entitySearchCasesResponse
		if err := json.Unmarshal(raw, &resp); err != nil {
			return nil, fmt.Errorf("unmarshal entity-service cases response: %w", err)
		}
		cases = append(cases, resp.Cases...)
		if len(resp.Cases) < customerHealthCasePageLimit || (page+1)*customerHealthCasePageLimit >= resp.Total {
			break
		}
	}
	return cases, nil
}

// projectEscalatedCases returns a CaseLink per escalation raised against one
// of projectID's own cases.
func (c *postgresCustomerHealthClient) projectEscalatedCases(ctx context.Context, projectID string) ([]servicenow.CaseLink, error) {
	cases, err := c.projectCases(ctx, projectID, "", nil)
	if err != nil || len(cases) == 0 {
		return nil, err
	}
	caseIDs := make([]string, len(cases))
	caseByID := make(map[string]entitySearchCaseView, len(cases))
	for i, cv := range cases {
		caseIDs[i] = cv.ID
		caseByID[cv.ID] = cv
	}

	body, err := json.Marshal(entitySearchEscalationsRequest{
		Filters:    entitySearchEscalationsFilters{CaseIDs: caseIDs},
		SortBy:     entityEscalationSort{Field: "createdOn", Order: "desc"},
		Pagination: entityPagination{Limit: 50, Offset: 0},
	})
	if err != nil {
		return nil, fmt.Errorf("marshal entity-service escalations request: %w", err)
	}
	raw, err := c.entity.SearchEscalations(ctx, body)
	if err != nil {
		return nil, err
	}
	var resp entitySearchEscalationsResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal entity-service escalations response: %w", err)
	}

	links := make([]servicenow.CaseLink, 0, len(resp.Escalations))
	for _, e := range resp.Escalations {
		link := servicenow.CaseLink{EscalationSysID: e.ID}
		if cv, ok := caseByID[e.Case.ID]; ok {
			link.Number = cv.Number
			link.EscalationNumber = cv.Number
		}
		links = append(links, link)
	}
	return links, nil
}

// groupCasesByPriority buckets cases by their severity label for the
// "detailedRecentCases" drill-down -- severity is the closest equivalent
// this data source has to ServiceNow's own case "priority" field.
func groupCasesByPriority(cases []entitySearchCaseView) []servicenow.CaseGroup {
	byPriority := make(map[string][]servicenow.CaseInfo)
	var order []string
	for _, cv := range cases {
		priority := "Unspecified"
		if cv.Severity != nil && *cv.Severity != "" {
			priority = *cv.Severity
		}
		if _, ok := byPriority[priority]; !ok {
			order = append(order, priority)
		}
		byPriority[priority] = append(byPriority[priority], servicenow.CaseInfo{SysID: cv.ID, Number: cv.Number})
	}
	groups := make([]servicenow.CaseGroup, 0, len(order))
	for _, priority := range order {
		groups = append(groups, servicenow.CaseGroup{
			Priority: priority,
			Count:    len(byPriority[priority]),
			Cases:    byPriority[priority],
		})
	}
	return groups
}

// caseLinks converts a case list into CaseLink drill-down entries (no
// escalation side).
func caseLinks(cases []entitySearchCaseView) []servicenow.CaseLink {
	links := make([]servicenow.CaseLink, 0, len(cases))
	for _, cv := range cases {
		links = append(links, servicenow.CaseLink{SysID: cv.ID, Number: cv.Number})
	}
	return links
}
