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
	"strings"
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
// are this file's own judgment calls for the two migration-engagement
// signals (hasAbandonedMigrations/hasMigrationDelays), not a confirmed spec.
// The other flags' own windows (hasRecentCases/noSupportCases6mo/
// hasRecentEscalations) are genuinely derivable from real, populated Postgres
// data (checked directly against staging: 3,820 cases created in the last
// 180 days, 76 MIGRATION-type engagements, 349+ account-linked escalations).
// Revisit if a product owner specifies different windows.
const (
	customerHealthRecentWindow     = 180 * 24 * time.Hour // "recent cases" / "no support cases" cutoff
	customerHealthEscalationWindow = 90 * 24 * time.Hour  // "recent escalations" cutoff
	customerHealthDelayWindow      = 60 * 24 * time.Hour  // a still-open MIGRATION engagement older than this is "delayed"
	customerHealthAbandonedWindow  = 180 * 24 * time.Hour // a still-open MIGRATION engagement older than this is "abandoned"
)

// postgresCustomerHealthClient implements customerHealthSNClient entirely
// against entity-service (Postgres). It derives seven account/project risk
// flags (hasNoGoLive, hasRecentCases, hasEolProduct, hasAbandonedMigrations,
// hasMigrationDelays, hasRecentEscalations, noSupportCases6mo) from real,
// already-synced Postgres columns (project.onboarding_status,
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
// service's own CLAUDE.md, "Case-like work_item types") -- deliberately
// excluding "announcement", which must never count as a case for this
// feature's own "recent cases" signal. "type" only accepts an "in" filter
// on entity-service's case search (no "notIn"), so this list is used as an
// inclusion filter everywhere "every case-like type but announcement" is
// needed, rather than an exclusion.
var customerHealthCaseLikeTypes = []string{"case", "service_request", "engagement", "security_report_analysis"}

// isCustomerHealthCaseLikeType reports whether t is one of
// customerHealthCaseLikeTypes -- used to filter an already-fetched case list
// client-side (GetCustomerHealthDetail's per-project recentCases) the same
// way the type:in filter narrows a fresh entity-service search
// (accountHasCasesSince).
func isCustomerHealthCaseLikeType(t string) bool {
	for _, want := range customerHealthCaseLikeTypes {
		if t == want {
			return true
		}
	}
	return false
}

// GetCustomerHealthSummary implements customerHealthSNClient.
//
// Found live: product/abtTeam/risks were accepted on the wire (matching the
// existing interface every caller already uses) but silently never applied
// -- confirmed against a captured record of real traffic where selecting
// "risks", a "product", or any region at all either changed nothing
// (product/risks) or returned zero results for every value tried (region,
// which was comparing against the wrong account field -- see
// filteredAccounts' own doc comment). All five of
// email/phrase/risks/region/product/abtTeam are applied now.
func (c *postgresCustomerHealthClient) GetCustomerHealthSummary(ctx context.Context, email, phrase, risks *string, region []string, product, abtTeam *string, offset, limit int) (*servicenow.AccountSummaryResponse, error) {
	accounts, total, err := c.filteredAccounts(ctx, customerHealthAccountQuery{
		email:   email,
		phrase:  phrase,
		region:  region,
		product: product,
		abtTeam: abtTeam,
		risks:   parseCustomerHealthRisks(risks),
	}, offset, limit)
	if err != nil {
		return nil, err
	}

	data := make([]servicenow.AccountSummary, 0, len(accounts))
	for _, a := range accounts {
		flags, err := c.accountFlags(ctx, a.ID, a.ActivationDate)
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
	raw, err := c.entity.SearchAccounts(ctx, accountBody)
	if err != nil {
		return nil, err
	}
	var accountResp entitySearchAccountsResponse
	if err := json.Unmarshal(raw, &accountResp); err != nil {
		return nil, fmt.Errorf("unmarshal entity-service account response: %w", err)
	}
	accountName := ""
	var activationDate *string
	found := false
	for _, a := range accountResp.Accounts {
		if a.ID == accountID {
			accountName = a.Name
			activationDate = a.ActivationDate
			found = true
			break
		}
	}
	if !found {
		return nil, servicenow.ErrAccountNotFound
	}

	projects, err := c.accountProjects(ctx, accountID)
	if err != nil {
		return nil, err
	}

	// Fetched ONCE for the whole account and grouped by project client-side
	// below, rather than once per project per flag -- a real, found
	// performance bug: the original per-project-per-flag design ran up to
	// several pages of case search PER PROJECT PER FLAG
	// (recent/abandoned/delayed/escalated-cases each re-fetched the
	// project's own cases from scratch), which for a real account with 21
	// projects and 1,585 cases took minutes and was still running when
	// manually verified against the live database -- confirmed hung,
	// not merely slow. One bounded account-wide fetch plus in-memory
	// grouping/filtering is both correct and fast regardless of how many
	// projects an account has.
	allCases, err := c.accountCasesFull(ctx, accountID)
	if err != nil {
		return nil, err
	}
	casesByProject := make(map[string][]entitySearchCaseView, len(projects))
	caseByID := make(map[string]entitySearchCaseView, len(allCases))
	allCaseIDs := make([]string, len(allCases))
	for i, cv := range allCases {
		allCaseIDs[i] = cv.ID
		caseByID[cv.ID] = cv
		if cv.Project != nil {
			casesByProject[cv.Project.ID] = append(casesByProject[cv.Project.ID], cv)
		}
	}
	escalations, err := c.searchEscalationsForCaseIDs(ctx, allCaseIDs)
	if err != nil {
		return nil, err
	}
	escalationsByProject := make(map[string][]entityEscalation, len(projects))
	for _, e := range escalations {
		if cv, ok := caseByID[e.Case.ID]; ok && cv.Project != nil {
			escalationsByProject[cv.Project.ID] = append(escalationsByProject[cv.Project.ID], e)
		}
	}

	details := make([]servicenow.ProjectDetail, 0, len(projects))
	for _, p := range projects {
		detail, err := c.projectDetail(ctx, p, activationDate, casesByProject[p.ID], escalationsByProject[p.ID])
		if err != nil {
			return nil, err
		}
		details = append(details, detail)
	}

	return &servicenow.AccountDetail{AccountName: accountName, CustomerProjects: details}, nil
}

// customerHealthAccountCasePageLimit/customerHealthAccountCasePageCap bound
// accountCasesFull's own pagination. customerHealthAccountCasePageCap is a
// runaway-loop backstop, not an expected real limit (20,000 cases is far
// beyond any real account's case count this portal manages today -- one
// real account, checked directly against staging, has 1,585 -- the same
// posture customer_health.go's own maxBatches and filteredAccounts' own
// accountScanPageCap already document): an earlier, smaller cap here (2,000)
// was itself still a silent-truncation risk for a sufficiently large
// account, flagged live by review. If the backstop is ever actually reached,
// that means the real data has grown far beyond what was ever sized for, so
// this now fails loudly (an error) rather than silently returning an
// incomplete case list to GetCustomerHealthDetail's drill-down lists and the
// 90-day escalation flag.
const (
	customerHealthAccountCasePageLimit = 50
	customerHealthAccountCasePageCap   = 400 // 400 * 50 = 20,000 cases per account
)

// accountCasesFull pages through every case-like work item belonging to
// accountID, with the fields GetCustomerHealthDetail needs to group and
// filter them per project client-side (type, engagementType, severity,
// state, createdOn, project) -- replaces what used to be a fresh,
// re-fetched-per-flag query per project (see GetCustomerHealthDetail's own
// doc comment on the performance bug this fixes).
func (c *postgresCustomerHealthClient) accountCasesFull(ctx context.Context, accountID string) ([]entitySearchCaseView, error) {
	var cases []entitySearchCaseView
	complete := false
	for page := 0; page < customerHealthAccountCasePageCap; page++ {
		body, err := json.Marshal(entitySearchCasesRequest{
			Filters: entitySearchCasesFilters{Filters: []entityCaseFieldFilter{
				{Field: "accountId", Op: "in", Values: []string{accountID}},
				{Field: "type", Op: "in", Values: customerHealthCaseLikeTypes},
			}},
			Pagination: entityPagination{Limit: customerHealthAccountCasePageLimit, Offset: page * customerHealthAccountCasePageLimit},
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
		if len(resp.Cases) < customerHealthAccountCasePageLimit || (page+1)*customerHealthAccountCasePageLimit >= resp.Total {
			complete = true
			break
		}
	}
	if !complete {
		return nil, fmt.Errorf("account %s has more cases than this lookup's %d-case safety bound can fetch", accountID, customerHealthAccountCasePageCap*customerHealthAccountCasePageLimit)
	}
	return cases, nil
}

// searchEscalationsForCaseIDs is a thin wrapper over POST /escalations/search
// for an explicit case-id list, shared by GetCustomerHealthDetail (one
// account-wide call) and accountHasRecentEscalations (the summary flag).
func (c *postgresCustomerHealthClient) searchEscalationsForCaseIDs(ctx context.Context, caseIDs []string) ([]entityEscalation, error) {
	if len(caseIDs) == 0 {
		return nil, nil
	}
	// entity-service rejects a limit above 50 (confirmed directly: "limit
	// cannot exceed 50") -- paginate like every other search in this file
	// rather than assuming one page covers an account's escalations (one
	// real account had 257 of them).
	const (
		pageLimit = 50
		pageCap   = 20 // 20 * 50 = 1000 escalations per account
	)
	var escalations []entityEscalation
	for page := 0; page < pageCap; page++ {
		body, err := json.Marshal(entitySearchEscalationsRequest{
			Filters:    entitySearchEscalationsFilters{CaseIDs: caseIDs},
			SortBy:     entityEscalationSort{Field: "createdOn", Order: "desc"},
			Pagination: entityPagination{Limit: pageLimit, Offset: page * pageLimit},
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
		escalations = append(escalations, resp.Escalations...)
		if len(resp.Escalations) < pageLimit || (page+1)*pageLimit >= resp.Total {
			break
		}
	}
	return escalations, nil
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

// customerHealthAccountLifeCycle/customerHealthAccountRequiresProject scope
// every customer-health account query to the confirmed, correct rule for
// this feature: account.life_cycle = 'Customer' and the account has at
// least one project.
//
// This supersedes an earlier version of this fix, which guessed "active"
// (account.deactivation_date IS NULL) was the right scope, reasoned from an
// unrelated account search/picker endpoint's own exclusion of accounts
// whose status was "lost". That guess was wrong on two counts, found once
// the correct rule was confirmed and checked directly against staging data:
// (1) the right field is account.life_cycle -- a distinct column from
// account.classification (a business-segment axis: Enterprise/Partner/
// Cloud/OEM) -- not deactivation_date at all; deactivation_date IS NULL
// matched far more accounts than the real endpoint's count, nowhere close.
// (2) the account must also have at least one project -- missing this
// alone still overcounts, since life_cycle = 'Customer' alone still matches
// a much larger set than the real endpoint's count. Confirmed together:
// life_cycle = 'Customer' AND at least one project matches exactly the
// account count the real endpoint reports for the identical unfiltered
// request. Notably the correct rule has NO active/inactive check at all --
// some of those matching accounts are themselves deactivated
// (deactivation_date IS NOT NULL) and must still be included, which is why
// this is no longer combined with an Active filter.
const (
	customerHealthAccountLifeCycle       = "Customer"
	customerHealthAccountRequiresProject = true
)

// customerHealthAccountQuery bundles every optional account-list filter
// GetCustomerHealthSummary accepts into one place, rather than a long
// positional parameter list.
type customerHealthAccountQuery struct {
	email, phrase, product, abtTeam *string
	region                          []string
	// risks is the already-split form of the comma-separated "risks" query
	// param -- see parseCustomerHealthRisks. An account matches when it
	// exhibits ANY one of these (an OR within the list), same as every
	// other filter here is ANDed against it.
	risks []string
}

// parseCustomerHealthRisks splits the comma-separated risks query param into
// the flag keys customerHealthAccountMatchesAnyRisk checks against --
// "hasNoGoLive", "hasRecentCases", "noSupportCases6mo", "hasEolProduct",
// "hasAbandonedMigrations", "hasMigrationDelays", "hasRecentEscalations" --
// the same field names customerHealthAccountFlags/servicenow.AccountSummary
// already use, so the two can never drift apart.
func parseCustomerHealthRisks(risks *string) []string {
	if risks == nil {
		return nil
	}
	var out []string
	for _, r := range strings.Split(*risks, ",") {
		if r = strings.TrimSpace(r); r != "" {
			out = append(out, r)
		}
	}
	return out
}

// customerHealthAccountMatchesAnyRisk reports whether flags exhibits any one
// of the requested risk keys -- an unrecognized key matches nothing (rather
// than erroring), the same fail-safe posture a stale/future webapp build
// sending an unknown key should get.
func customerHealthAccountMatchesAnyRisk(flags customerHealthAccountFlags, risks []string) bool {
	for _, r := range risks {
		switch r {
		case "hasNoGoLive":
			if flags.goLiveStatus.IsRisk {
				return true
			}
		case "hasRecentCases":
			if flags.hasRecentCases {
				return true
			}
		case "noSupportCases6mo":
			if !flags.hasRecentCases {
				return true
			}
		case "hasEolProduct":
			if flags.hasEolProduct {
				return true
			}
		case "hasAbandonedMigrations":
			if flags.hasAbandonedMigrations {
				return true
			}
		case "hasMigrationDelays":
			if flags.hasMigrationDelays {
				return true
			}
		case "hasRecentEscalations":
			if flags.hasRecentEscalations {
				return true
			}
		}
	}
	return false
}

// filteredAccounts resolves the page of accounts GetCustomerHealthSummary
// should compute flags for. email/phrase map onto entity-service's own
// SearchAccounts filters directly; region/product/abtTeam/risks have no
// backing SearchAccounts filter at all (checked directly:
// SearchAccountsFilters has searchQuery/active/pod/ownerEmail/
// classification/lifeCycle/hasProject only), so all four are applied
// client-side over a bounded working set instead, the same shape as the
// pre-existing region-only scan this replaces.
//
// Found live: every one of these four silently did nothing (product/
// abtTeam/risks) or matched nothing at all (region). The region bug was
// comparing against the wrong account field entirely -- q.region's values
// ("ASIA", "NA - WEST", "Benelux + France", ...) are sub-region values
// (account.sub_region), while the filter compared them against
// account.region (a different, coarser field: "APAC", "EU", "US WEST", ...)
// -- confirmed directly against staging data, where neither column's values
// ever overlap with the other's. "ABT team" matches an account's own
// CreTeam or SreTeam name (account.cre_team_id/sre_team_id, resolved to a
// group reference) -- confirmed directly: every real customer account's
// CreTeam name is one of the names the "All Teams" dropdown itself lists.
// "product" matches any deployed product (active or not) under any of the
// account's projects' deployments.
func (c *postgresCustomerHealthClient) filteredAccounts(ctx context.Context, q customerHealthAccountQuery, offset, limit int) ([]entityAccountView, int, error) {
	filters := entitySearchAccountsFilters{
		LifeCycle:  customerHealthAccountLifeCycle,
		HasProject: customerHealthAccountRequiresProject,
	}
	if q.phrase != nil {
		filters.SearchQuery = *q.phrase
	}
	if q.email != nil {
		filters.OwnerEmail = *q.email
	}

	wantProduct := ""
	if q.product != nil {
		wantProduct = strings.TrimSpace(*q.product)
	}
	wantABTTeam := ""
	if q.abtTeam != nil {
		wantABTTeam = strings.TrimSpace(*q.abtTeam)
	}
	needsScan := len(q.region) > 0 || len(q.risks) > 0 || wantProduct != "" || wantABTTeam != ""

	if !needsScan {
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

	wantRegion := make(map[string]bool, len(q.region))
	for _, r := range q.region {
		wantRegion[r] = true
	}
	var matched []entityAccountView
	// accountScanPageCap is a runaway-loop backstop, not an expected real
	// limit (10,000 accounts is far beyond any real account count this
	// portal manages, the same posture customer_health.go's own maxBatches
	// documents) -- a smaller cap here previously stopped the scan before
	// reaching every account, silently under-reporting both the matched set
	// and TotalCount for a region filter once the account list grew past it.
	const (
		accountScanPageLimit = 50
		accountScanPageCap   = 200 // 200 * 50 = 10,000 accounts scanned
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
			if len(wantRegion) > 0 && (a.SubRegion == nil || !wantRegion[*a.SubRegion]) {
				continue
			}
			if wantABTTeam != "" {
				onTeam := (a.CreTeam != nil && strings.EqualFold(a.CreTeam.Name, wantABTTeam)) ||
					(a.SreTeam != nil && strings.EqualFold(a.SreTeam.Name, wantABTTeam))
				if !onTeam {
					continue
				}
			}
			if wantProduct != "" {
				hasProduct, err := c.accountHasProduct(ctx, a.ID, wantProduct)
				if err != nil {
					return nil, 0, err
				}
				if !hasProduct {
					continue
				}
			}
			if len(q.risks) > 0 {
				flags, err := c.accountFlags(ctx, a.ID, a.ActivationDate)
				if err != nil {
					return nil, 0, err
				}
				if !customerHealthAccountMatchesAnyRisk(flags, q.risks) {
					continue
				}
			}
			matched = append(matched, a)
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

// accountHasProduct reports whether any deployed product (active or not)
// under any of accountID's own projects' deployments has this product name
// (case-insensitive). Reuses deployedEolProducts' own second return value
// (every deployed product under the given deployments, not just the EOL
// ones) rather than a separate entity-service call.
func (c *postgresCustomerHealthClient) accountHasProduct(ctx context.Context, accountID, product string) (bool, error) {
	projects, err := c.accountProjects(ctx, accountID)
	if err != nil {
		return false, err
	}
	for _, p := range projects {
		deployments, err := c.projectDeployments(ctx, p.ID, true)
		if err != nil {
			return false, err
		}
		if len(deployments) == 0 {
			continue
		}
		_, all, err := c.deployedEolProducts(ctx, deploymentIDsOf(deployments))
		if err != nil {
			return false, err
		}
		for _, dp := range all {
			if strings.EqualFold(dp.Product.Name, product) {
				return true, nil
			}
		}
	}
	return false, nil
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

// customerHealthLegacyActivationCutoff is the legacy system's own
// grandfather-clause date ("2024-01-01 00:00:00", confirmed directly against
// its own source): an account active before this is treated as having gone
// live regardless of whether a go-live date was ever recorded for its
// projects.
var customerHealthLegacyActivationCutoff = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

// projectGoLiveStatus ports the legacy system's own go-live-status
// computation -- its exact source was obtained directly and its branch
// structure/thresholds are reproduced here faithfully. Status strings were
// cross-checked against a live sample of real accounts (not assumed from
// the source alone) and corrected where they disagreed -- see the
// "Customer for over 2 years" correction in the legacy-activation branch
// below for one found this way.
// goLiveDate is the project's own
// GoLiveDate (RFC3339, entity-service's ProjectView.GoLiveDate /
// project.onboarding_go_live_date -- the field the legacy system's own
// equivalent checks); activationDate is the owning account's own
// ActivationDate ("YYYY-MM-DD", entity-service's AccountView.ActivationDate
// / account.activation_date -- the field the legacy system's own equivalent
// checks). The legacy system's "project missing"/"account missing" branches
// don't apply here: both are already resolved by the caller before this
// runs.
//
// A previous version of this check used the project's OnboardingStatus
// (project.onboarding_status) instead -- a different, unrelated field the
// legacy rule never reads for this purpose at all, found live to silently
// under-report: a project with no go-live date and no account activation
// date ("N/A (No Start Date)" in the legacy rule) was reported as "Live"
// here, because OnboardingStatus's "Not-Applicable" value was (wrongly)
// treated as "nothing to worry about".
func projectGoLiveStatus(goLiveDate, activationDate *string, now time.Time) servicenow.GoLiveStatus {
	if goLiveDate != nil && *goLiveDate != "" {
		return servicenow.GoLiveStatus{Status: "Has Gone Live", IsRisk: false, State: "success"}
	}

	var activation time.Time
	haveActivation := false
	if activationDate != nil && *activationDate != "" {
		if t, err := time.Parse("2006-01-02", *activationDate); err == nil {
			activation = t
			haveActivation = true
		}
	}

	if haveActivation && activation.Before(customerHealthLegacyActivationCutoff) {
		// Status is plain "Has Gone Live", not a "... over 2 years" variant --
		// confirmed against real data: every account in a live sample whose
		// activation predates the cutoff (with no go-live date recorded)
		// reports plain "Has Gone Live" from the legacy system, never a
		// distinct label for this branch. An earlier version of this port
		// invented a "Has Gone Live ( Customer for over 2 years )" string
		// here with no real-data confirmation; this branch is kept distinct
		// from the goLiveDate-is-set branch above (not collapsed into one
		// early return) only to preserve the legacy rule's own branch
		// structure for a future maintainer, even though the two currently
		// produce an identical result.
		return servicenow.GoLiveStatus{Status: "Has Gone Live", IsRisk: false, State: "success"}
	}
	if !haveActivation {
		return servicenow.GoLiveStatus{Status: "N/A (No Start Date)", IsRisk: false, State: "pending"}
	}
	if activation.After(now) {
		return servicenow.GoLiveStatus{Status: "N/A (Activation Date in Future)", IsRisk: false, State: "pending"}
	}

	// totalMonths mirrors the legacy rule's own
	// floor(totalDays / 30.4375) -- the average days-per-month constant
	// it uses, not a calendar-month diff.
	totalMonths := int(now.Sub(activation).Hours() / 24 / 30.4375)
	switch {
	case totalMonths >= 6:
		return servicenow.GoLiveStatus{Status: "Hasn't Gone Live in 6 months", IsRisk: true, State: "failure"}
	case totalMonths == 0:
		return servicenow.GoLiveStatus{Status: "Recently Activated (Less than a month)", IsRisk: false, State: "pending"}
	default:
		unit := "months"
		if totalMonths == 1 {
			unit = "month"
		}
		return servicenow.GoLiveStatus{Status: fmt.Sprintf("Hasn't Gone Live Since %d %s", totalMonths, unit), IsRisk: false, State: "pending"}
	}
}

// customerHealthDefaultGoLiveStatus is the legacy rule's own default
// ({isRisk: false, state: 'success', status: 'Has Gone Live'}) -- what an
// account with no projects, or whose every project has actually gone live,
// reports.
var customerHealthDefaultGoLiveStatus = servicenow.GoLiveStatus{Status: "Has Gone Live", IsRisk: false, State: "success"}

// accountGoLiveStatus aggregates projectGoLiveStatus across every one of an
// account's projects the same way the legacy rule does: starting from the
// "Has Gone Live" default, a project's "failure" or "pending" result
// overwrites the running summary (a "success" result never does), and once
// "failure" is reached nothing evaluated afterward can change it -- the
// legacy rule's own guard against downgrading an already-failed summary.
func accountGoLiveStatus(projects []entityProjectView, activationDate *string, now time.Time) servicenow.GoLiveStatus {
	status := customerHealthDefaultGoLiveStatus
	for _, p := range projects {
		if status.State == "failure" {
			break
		}
		result := projectGoLiveStatus(p.GoLiveDate, activationDate, now)
		if result.State == "failure" || result.State == "pending" {
			status = result
		}
	}
	return status
}

// accountFlags computes the seven summary-level flags for one account,
// aggregating across every one of its projects (any project flagged ->
// the account is flagged) for the project-scoped signals (go-live,
// EOL product, migration delays/abandonment), and account-wide queries for
// the case/escalation-based ones. activationDate is the account's own
// ActivationDate (entity-service's AccountView.ActivationDate), needed by
// accountGoLiveStatus -- see that function's own doc comment.
func (c *postgresCustomerHealthClient) accountFlags(ctx context.Context, accountID string, activationDate *string) (customerHealthAccountFlags, error) {
	projects, err := c.accountProjects(ctx, accountID)
	if err != nil {
		return customerHealthAccountFlags{}, err
	}

	goLiveStatus := accountGoLiveStatus(projects, activationDate, time.Now())

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

	return customerHealthAccountFlags{
		goLiveStatus:           goLiveStatus,
		hasRecentCases:         hasRecentCases,
		hasEolProduct:          hasEolProduct,
		hasAbandonedMigrations: hasAbandoned,
		hasMigrationDelays:     hasDelayed,
		hasRecentEscalations:   hasRecentEscalations,
	}, nil
}

// projectHasEolProduct checks whether any deployed product under projectID's
// own deployments is running a product version past its support_eol_date.
//
// Fetches with includeInactive: true -- the legacy rule this ports has no
// active-only filter at all, and a deactivated/decommissioned deployment can
// still carry real EOL product history that must count toward the risk flag
// (confirmed by a live reproduction against a real staging account: the
// deployment holding its EOL product history was inactive, so the
// active-only default made this flag a false negative).
func (c *postgresCustomerHealthClient) projectHasEolProduct(ctx context.Context, projectID string) (bool, error) {
	deployments, err := c.projectDeployments(ctx, projectID, true)
	if err != nil || len(deployments) == 0 {
		return false, err
	}
	eol, _, err := c.deployedEolProducts(ctx, deploymentIDsOf(deployments))
	return len(eol) > 0, err
}

// deploymentIDsOf extracts just the ids from a deployment list, for the
// SearchDeployedProducts DeploymentIDs filter.
func deploymentIDsOf(deployments []entityDeploymentView) []string {
	ids := make([]string, len(deployments))
	for i, d := range deployments {
		ids[i] = d.ID
	}
	return ids
}

// customerHealthDeploymentPageLimit/customerHealthDeploymentPageCap bound
// projectDeployments'/deployedEolProducts' own pagination -- entity-service's
// 50-per-page cap applies to the total, not just the first page (confirmed:
// both searches report their own Total alongside LIMIT/OFFSET results), so a
// project with more than 50 active deployments, or more than 50 deployed
// products under the deployments being checked, silently evaluated only its
// first page until this was fixed -- projectHasEolProduct could then miss a
// later EOL product and report a false negative.
const (
	customerHealthDeploymentPageLimit = 50
	customerHealthDeploymentPageCap   = 20 // 20 * 50 = 1000 deployments/products per project
)

func (c *postgresCustomerHealthClient) projectDeployments(ctx context.Context, projectID string, includeInactive bool) ([]entityDeploymentView, error) {
	var deployments []entityDeploymentView
	for page := 0; page < customerHealthDeploymentPageCap; page++ {
		body, err := json.Marshal(entityDeploymentsSearchRequest{
			Pagination:      entityPagination{Limit: customerHealthDeploymentPageLimit, Offset: page * customerHealthDeploymentPageLimit},
			ProjectIDs:      []string{projectID},
			IncludeInactive: includeInactive,
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
		deployments = append(deployments, resp.Deployments...)
		if len(resp.Deployments) < customerHealthDeploymentPageLimit || (page+1)*customerHealthDeploymentPageLimit >= resp.Total {
			break
		}
	}
	return deployments, nil
}

// deployedEolProducts returns the deployed products under deploymentIDs whose
// product version is past its support_eol_date, alongside every deployed
// product (used by projectDetail to build the "softwareModel" drill-down).
func (c *postgresCustomerHealthClient) deployedEolProducts(ctx context.Context, deploymentIDs []string) ([]entityDeployedProductEolView, []entityDeployedProductEolView, error) {
	var all []entityDeployedProductEolView
	for page := 0; page < customerHealthDeploymentPageCap; page++ {
		body, err := json.Marshal(entitySearchDeployedProductsEolRequest{
			Pagination:    entityPagination{Limit: customerHealthDeploymentPageLimit, Offset: page * customerHealthDeploymentPageLimit},
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
		all = append(all, resp.DeployedProducts...)
		if len(resp.DeployedProducts) < customerHealthDeploymentPageLimit || (page+1)*customerHealthDeploymentPageLimit >= resp.Total {
			break
		}
	}
	now := time.Now()
	var eol []entityDeployedProductEolView
	for _, dp := range all {
		if dp.Version != nil && dp.Version.SupportEoLDate != nil {
			if t, err := time.Parse(time.RFC3339, *dp.Version.SupportEoLDate); err == nil && t.Before(now) {
				eol = append(eol, dp)
			}
		}
	}
	return eol, all, nil
}

// accountHasCasesSince reports whether the account has any case-like work
// item created at or after since -- restricted to customerHealthCaseLikeTypes
// (excludes "announcement"), matching the legacy rule's own "recent cases"
// query exactly. A previous version applied no type filter at all here, so
// an account whose only recent work items were announcements (which the
// legacy rule never counts) was wrongly reported as having recent cases.
func (c *postgresCustomerHealthClient) accountHasCasesSince(ctx context.Context, accountID string, since time.Time) (bool, error) {
	total, err := c.caseCount(ctx, accountID, "", []entityCaseFieldFilter{
		{Field: "type", Op: "in", Values: customerHealthCaseLikeTypes},
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
		{Field: "engagementType", Op: "in", Values: []string{"migration"}},
		{Field: "state", Op: "notIn", Values: []string{"closed"}},
		{Field: "createdOn", Op: "lte", Values: []string{now.Add(-customerHealthAbandonedWindow).UTC().Format(time.RFC3339)}},
	})
	if err != nil {
		return false, false, err
	}
	delayedCount, err := c.caseCount(ctx, accountID, "engagement", []entityCaseFieldFilter{
		{Field: "engagementType", Op: "in", Values: []string{"migration"}},
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
//
// Found live: entity-service's own case-search type allow-list genuinely
// includes "announcement" alongside the four real case-like types (case,
// service_request, engagement, security_report_analysis) -- a plain
// accountId-scoped search with no type filter at all legitimately returns
// announcement rows too (WSO2-authored broadcast notices, never something a
// customer filed). An empty caseType here used to mean "no type filter",
// which silently counted an account's announcements as recent/support
// cases -- confirmed against a real account whose only work items were
// three announcements, which still flipped hasRecentCases to true. caseType
// == "" now means "any of the real case-like types", not "unrestricted".
func (c *postgresCustomerHealthClient) caseCount(ctx context.Context, accountID, caseType string, extra []entityCaseFieldFilter) (int, error) {
	filters := []entityCaseFieldFilter{{Field: "accountId", Op: "in", Values: []string{accountID}}}
	if caseType != "" {
		filters = append(filters, entityCaseFieldFilter{Field: "type", Op: "in", Values: []string{caseType}})
	} else {
		filters = append(filters, entityCaseFieldFilter{Field: "type", Op: "in", Values: customerHealthCaseLikeTypes})
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

// accountHasRecentEscalations reports whether any of the account's cases has
// an escalation created within customerHealthEscalationWindow. Reuses
// accountCasesFull's own (wider, and now a loud failure rather than a silent
// truncation past its backstop -- see accountCasesFull's own doc comment)
// pagination bound rather than a separately-tuned, narrower one -- an
// earlier version here (accountCaseIDsForHealth, 500-case cap) could miss an
// escalation on a case beyond that cap for a real account with more than 500
// cases (one real account checked has 1,585), silently under-reporting this
// flag. There is no reason for this flag's own case-id lookup to be any less
// complete than GetCustomerHealthDetail's identical one.
func (c *postgresCustomerHealthClient) accountHasRecentEscalations(ctx context.Context, accountID string) (bool, error) {
	cases, err := c.accountCasesFull(ctx, accountID)
	if err != nil || len(cases) == 0 {
		return false, err
	}
	caseIDs := make([]string, len(cases))
	for i, cv := range cases {
		caseIDs[i] = cv.ID
	}
	escalations, err := c.searchEscalationsForCaseIDs(ctx, caseIDs)
	if err != nil {
		return false, err
	}
	cutoff := time.Now().Add(-customerHealthEscalationWindow)
	for _, e := range escalations {
		if t, err := time.Parse(time.RFC3339, e.CreatedOn); err == nil && t.After(cutoff) {
			return true, nil
		}
	}
	return false, nil
}

// projectDetail builds one project's ProjectDetail row for GetCustomerHealthDetail,
// including the drill-down lists (recent cases by priority, abandoned/delayed
// migration case links, EOL software models, escalated case links). cases/
// escalations are the ones GetCustomerHealthDetail already fetched once for
// the whole account and grouped by project -- this function does no case or
// escalation search of its own, only the (bounded, per-project) deployment/
// deployed-product lookups.
func (c *postgresCustomerHealthClient) projectDetail(ctx context.Context, p entityProjectView, activationDate *string, cases []entitySearchCaseView, escalations []entityEscalation) (servicenow.ProjectDetail, error) {
	since := time.Now().Add(-customerHealthRecentWindow).UTC()
	recentCases := filterCases(cases, func(cv entitySearchCaseView) bool {
		if !isCustomerHealthCaseLikeType(cv.Type) {
			return false
		}
		t, err := time.Parse(time.RFC3339, cv.CreatedOn)
		return err == nil && !t.Before(since)
	})
	recentGroups := groupCasesByPriority(recentCases)

	now := time.Now().UTC()
	abandonedCutoff := now.Add(-customerHealthAbandonedWindow)
	delayedCutoff := now.Add(-customerHealthDelayWindow)
	abandonedCases := filterCases(cases, func(cv entitySearchCaseView) bool {
		return isOpenMigrationBefore(cv, abandonedCutoff)
	})
	delayedCases := filterCases(cases, func(cv entitySearchCaseView) bool {
		return isOpenMigrationBefore(cv, delayedCutoff)
	})

	// Active-only: this is what feeds the plain "Deployments" list below
	// (deploymentRefs), which must keep showing only a project's current
	// deployments, unchanged by the EOL fix just below.
	deps, err := c.projectDeployments(ctx, p.ID, false)
	if err != nil {
		return servicenow.ProjectDetail{}, err
	}
	// includeInactive: true for the EOL computation specifically -- see
	// projectHasEolProduct's own doc comment. Fetched separately from deps
	// (above) so the "Deployments" list's own active-only semantics are
	// untouched; the legacy system's own EOL drill-down is built from
	// deployments regardless of active state too.
	eolDeps, err := c.projectDeployments(ctx, p.ID, true)
	if err != nil {
		return servicenow.ProjectDetail{}, err
	}
	deploymentIDs := deploymentIDsOf(eolDeps)
	var eolDeployments []entityDeployedProductEolView
	var deployedProducts []entityDeployedProductEolView
	if len(deploymentIDs) > 0 {
		eolDeployments, deployedProducts, err = c.deployedEolProducts(ctx, deploymentIDs)
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
	deploymentRefs := make([]servicenow.Deployment, 0, len(deps))
	for _, d := range deps {
		deploymentRefs = append(deploymentRefs, servicenow.Deployment{SysID: d.ID, Name: d.Name})
	}
	_ = deployedProducts // every deployed product under the project; not surfaced beyond the EOL subset today

	escalatedCases := escalationLinks(escalations, cases)

	status := projectGoLiveStatus(p.GoLiveDate, activationDate, time.Now())

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

// filterCases returns the subset of cases for which keep returns true --
// the client-side replacement for what used to be a separate, re-fetched
// entity-service query per flag per project (see GetCustomerHealthDetail's
// own doc comment on the performance bug this fixes).
func filterCases(cases []entitySearchCaseView, keep func(entitySearchCaseView) bool) []entitySearchCaseView {
	out := make([]entitySearchCaseView, 0, len(cases))
	for _, cv := range cases {
		if keep(cv) {
			out = append(out, cv)
		}
	}
	return out
}

// isOpenMigrationBefore reports whether cv is a still-open (state != closed)
// MIGRATION-type engagement created at or before cutoff -- the shared test
// behind both hasAbandonedMigrations (cutoff = customerHealthAbandonedWindow
// ago) and hasMigrationDelays (cutoff = customerHealthDelayWindow ago, a
// shorter/easier-to-trip window over the identical underlying signal).
func isOpenMigrationBefore(cv entitySearchCaseView, cutoff time.Time) bool {
	if cv.Type != "engagement" || cv.EngagementType == nil || *cv.EngagementType != "migration" {
		return false
	}
	if cv.State != nil && *cv.State == "closed" {
		return false
	}
	t, err := time.Parse(time.RFC3339, cv.CreatedOn)
	return err == nil && !t.After(cutoff)
}

// escalationLinks converts the account-wide escalations list already fetched
// by GetCustomerHealthDetail into CaseLink drill-down entries for one
// project's own cases (cases is that project's own case subset, used only
// to resolve a case's display number).
func escalationLinks(escalations []entityEscalation, cases []entitySearchCaseView) []servicenow.CaseLink {
	caseByID := make(map[string]entitySearchCaseView, len(cases))
	for _, cv := range cases {
		caseByID[cv.ID] = cv
	}
	links := make([]servicenow.CaseLink, 0, len(escalations))
	for _, e := range escalations {
		link := servicenow.CaseLink{EscalationSysID: e.ID}
		if cv, ok := caseByID[e.Case.ID]; ok {
			link.Number = cv.Number
			link.EscalationNumber = cv.Number
		}
		links = append(links, link)
	}
	return links
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
