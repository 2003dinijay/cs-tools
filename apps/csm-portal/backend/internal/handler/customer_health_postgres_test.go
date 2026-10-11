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
	"errors"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/servicenow"
)

// fakeEntityCustomerHealthClient is a scriptable entityCustomerHealthClient
// for testing postgresCustomerHealthClient without a real entity-service.
// Each method keys its canned response off the request it was sent, since a
// single GetCustomerHealthSummary call fans out into several different
// searches (accounts, projects, deployments, deployed products, cases,
// escalations).
type fakeEntityCustomerHealthClient struct {
	accounts               entitySearchAccountsResponse
	projects               entitySearchProjectsResponse
	deployments            entityDeploymentsSearchResponse
	deployedProducts       entitySearchDeployedProductsEolResponse
	caseResponses          []entitySearchCasesResponse // consumed in call order; last one repeats
	escalations            entitySearchEscalationsResponse
	caseCallCount          int
	lastAccountsReqRaw     []byte   // last request body SearchAccounts was called with
	lastCasesReqsRaw       [][]byte // every request body SearchCases was called with, in call order
	lastDeploymentsReqsRaw [][]byte // every request body SearchDeployments was called with, in call order
}

func (f *fakeEntityCustomerHealthClient) SearchAccounts(ctx context.Context, body []byte) ([]byte, error) {
	f.lastAccountsReqRaw = body
	return json.Marshal(f.accounts)
}

func (f *fakeEntityCustomerHealthClient) SearchProjects(ctx context.Context, body []byte) ([]byte, error) {
	return json.Marshal(f.projects)
}

func (f *fakeEntityCustomerHealthClient) SearchCases(ctx context.Context, body []byte) ([]byte, error) {
	f.lastCasesReqsRaw = append(f.lastCasesReqsRaw, body)
	idx := f.caseCallCount
	if idx >= len(f.caseResponses) {
		idx = len(f.caseResponses) - 1
	}
	f.caseCallCount++
	if idx < 0 {
		return json.Marshal(entitySearchCasesResponse{})
	}
	return json.Marshal(f.caseResponses[idx])
}

func (f *fakeEntityCustomerHealthClient) SearchDeployments(ctx context.Context, body []byte) ([]byte, error) {
	f.lastDeploymentsReqsRaw = append(f.lastDeploymentsReqsRaw, body)
	return json.Marshal(f.deployments)
}

func (f *fakeEntityCustomerHealthClient) SearchDeployedProducts(ctx context.Context, body []byte) ([]byte, error) {
	return json.Marshal(f.deployedProducts)
}

func (f *fakeEntityCustomerHealthClient) SearchEscalations(ctx context.Context, body []byte) ([]byte, error) {
	return json.Marshal(f.escalations)
}

func strPtr(s string) *string { return &s }

func TestPostgresCustomerHealthClient_GetCustomerHealthSummary_NoRiskAccount(t *testing.T) {
	fake := &fakeEntityCustomerHealthClient{
		accounts: entitySearchAccountsResponse{
			Accounts: []entityAccountView{{ID: "acc-1", Name: "Acme Corp"}},
			Total:    1,
		},
		projects: entitySearchProjectsResponse{
			Projects: []entityProjectView{{ID: "proj-1", Name: "Acme Platform", OnboardingStatus: strPtr("Completed")}},
			Total:    1,
		},
		deployments:      entityDeploymentsSearchResponse{},
		deployedProducts: entitySearchDeployedProductsEolResponse{},
		caseResponses:    []entitySearchCasesResponse{{Total: 5}, {Total: 0}, {Total: 0}},
		escalations:      entitySearchEscalationsResponse{},
	}
	c := NewPostgresCustomerHealthClient(fake)

	resp, err := c.GetCustomerHealthSummary(context.Background(), nil, nil, nil, nil, nil, nil, 0, 10)
	if err != nil {
		t.Fatalf("GetCustomerHealthSummary: %v", err)
	}
	if resp.TotalCount != 1 || len(resp.Data) != 1 {
		t.Fatalf("got %+v", resp)
	}
	acc := resp.Data[0]
	if acc.AccountSysID != "acc-1" || acc.AccountName == nil || *acc.AccountName != "Acme Corp" {
		t.Errorf("account identity wrong: %+v", acc)
	}
	if acc.HasNoGoLive.IsRisk {
		t.Errorf("expected no go-live risk for a Completed project, got %+v", acc.HasNoGoLive)
	}
	if !acc.HasRecentCases || acc.NoSupportCases6mo {
		t.Errorf("expected recent cases true / no-support false, got recent=%v noSupport=%v", acc.HasRecentCases, acc.NoSupportCases6mo)
	}
	if acc.HasEolProduct {
		t.Errorf("expected no EOL product, got true")
	}
}

// TestPostgresCustomerHealthClient_GetCustomerHealthSummary_NoGoLiveAndNoCases
// pins projectGoLiveStatus/accountGoLiveStatus's real risk branch: an account
// activated more than 6 months ago whose project has never recorded a
// go-live date is the one case the real ServiceNow script flags as an actual
// risk ("Hasn't Gone Live in 6 months", isRisk: true) -- OnboardingStatus is
// irrelevant to this decision (see projectGoLiveStatus's own doc comment),
// so the fixture project carries none at all.
func TestPostgresCustomerHealthClient_GetCustomerHealthSummary_NoGoLiveAndNoCases(t *testing.T) {
	sevenMonthsAgo := time.Now().AddDate(0, -7, 0).Format("2006-01-02")
	fake := &fakeEntityCustomerHealthClient{
		accounts: entitySearchAccountsResponse{
			Accounts: []entityAccountView{{ID: "acc-2", Name: "Beta Inc", ActivationDate: &sevenMonthsAgo}},
			Total:    1,
		},
		projects: entitySearchProjectsResponse{
			Projects: []entityProjectView{{ID: "proj-2", Name: "Beta Platform"}},
			Total:    1,
		},
		caseResponses: []entitySearchCasesResponse{{Total: 0}, {Total: 0}, {Total: 0}},
	}
	c := NewPostgresCustomerHealthClient(fake)

	resp, err := c.GetCustomerHealthSummary(context.Background(), nil, nil, nil, nil, nil, nil, 0, 10)
	if err != nil {
		t.Fatalf("GetCustomerHealthSummary: %v", err)
	}
	acc := resp.Data[0]
	if !acc.HasNoGoLive.IsRisk || acc.HasNoGoLive.State != "failure" {
		t.Errorf("expected a real go-live risk for an account activated 7 months ago with no go-live date, got %+v", acc.HasNoGoLive)
	}
	if acc.HasRecentCases || !acc.NoSupportCases6mo {
		t.Errorf("expected no recent cases / noSupportCases6mo true, got recent=%v noSupport=%v", acc.HasRecentCases, acc.NoSupportCases6mo)
	}
}

// TestPostgresCustomerHealthClient_GetCustomerHealthSummary_NoStartDate pins
// the real bug this fix closes: an account with no activation date recorded
// and a project with no go-live date is "N/A (No Start Date)" (pending, not
// a risk) in the real ServiceNow script -- a previous version of this code
// checked the project's OnboardingStatus instead and silently reported such
// an account as "Live".
func TestPostgresCustomerHealthClient_GetCustomerHealthSummary_NoStartDate(t *testing.T) {
	fake := &fakeEntityCustomerHealthClient{
		accounts: entitySearchAccountsResponse{
			Accounts: []entityAccountView{{ID: "acc-3", Name: "Gamma LLC"}},
			Total:    1,
		},
		projects: entitySearchProjectsResponse{
			Projects: []entityProjectView{{ID: "proj-3", Name: "Gamma Platform", OnboardingStatus: strPtr("Not-Applicable")}},
			Total:    1,
		},
		caseResponses: []entitySearchCasesResponse{{Total: 0}, {Total: 0}, {Total: 0}},
	}
	c := NewPostgresCustomerHealthClient(fake)

	resp, err := c.GetCustomerHealthSummary(context.Background(), nil, nil, nil, nil, nil, nil, 0, 10)
	if err != nil {
		t.Fatalf("GetCustomerHealthSummary: %v", err)
	}
	acc := resp.Data[0]
	if acc.HasNoGoLive.IsRisk {
		t.Errorf("an account with no activation date recorded must never be reported as an actual risk, got %+v", acc.HasNoGoLive)
	}
	if acc.HasNoGoLive.Status != "N/A (No Start Date)" || acc.HasNoGoLive.State != "pending" {
		t.Errorf("got %+v, want status %q state %q", acc.HasNoGoLive, "N/A (No Start Date)", "pending")
	}
}

// TestPostgresCustomerHealthClient_GetCustomerHealthSummary_ExcludesAnnouncementsFromRecentCases
// pins the real bug this fix closes: the real ServiceNow script's own
// "recent cases" query explicitly excludes Announcement-type work items
// (CustomerHealthAnalysis._checkHasRecentCases); a previous version of
// accountHasCasesSince applied no type filter at all, so an account whose
// only recent work item was an announcement was wrongly reported as having
// recent cases.
func TestPostgresCustomerHealthClient_GetCustomerHealthSummary_ExcludesAnnouncementsFromRecentCases(t *testing.T) {
	fake := &fakeEntityCustomerHealthClient{
		accounts: entitySearchAccountsResponse{
			Accounts: []entityAccountView{{ID: "acc-4", Name: "Delta Co"}},
			Total:    1,
		},
		projects:      entitySearchProjectsResponse{},
		caseResponses: []entitySearchCasesResponse{{Total: 0}, {Total: 0}, {Total: 0}},
	}
	c := NewPostgresCustomerHealthClient(fake)

	if _, err := c.GetCustomerHealthSummary(context.Background(), nil, nil, nil, nil, nil, nil, 0, 10); err != nil {
		t.Fatalf("GetCustomerHealthSummary: %v", err)
	}

	var req entitySearchCasesRequest
	if err := json.Unmarshal(fake.lastCasesReqsRaw[0], &req); err != nil {
		t.Fatalf("unmarshal captured request: %v", err)
	}
	var sawTypeFilter bool
	for _, f := range req.Filters.Filters {
		if f.Field != "type" {
			continue
		}
		sawTypeFilter = true
		if f.Op != "in" {
			t.Fatalf("type filter op = %q, want %q", f.Op, "in")
		}
		if len(f.Values) != len(customerHealthCaseLikeTypes) {
			t.Fatalf("type filter values = %v, want %v", f.Values, customerHealthCaseLikeTypes)
		}
		for _, v := range f.Values {
			if v == "announcement" {
				t.Fatalf("recent-cases search must not include announcement, got %v", f.Values)
			}
		}
	}
	if !sawTypeFilter {
		t.Fatalf("recent-cases search carried no type filter at all: %+v", req.Filters.Filters)
	}
}

func TestPostgresCustomerHealthClient_RegionFilter(t *testing.T) {
	apac := "APAC"
	emea := "EMEA"
	fake := &fakeEntityCustomerHealthClient{
		accounts: entitySearchAccountsResponse{
			Accounts: []entityAccountView{
				{ID: "acc-1", Name: "APAC Co", Region: &apac},
				{ID: "acc-2", Name: "EMEA Co", Region: &emea},
			},
			Total: 2,
		},
		projects:      entitySearchProjectsResponse{},
		caseResponses: []entitySearchCasesResponse{{Total: 0}},
	}
	c := NewPostgresCustomerHealthClient(fake)

	resp, err := c.GetCustomerHealthSummary(context.Background(), nil, nil, nil, []string{"APAC"}, nil, nil, 0, 10)
	if err != nil {
		t.Fatalf("GetCustomerHealthSummary: %v", err)
	}
	if resp.TotalCount != 1 || len(resp.Data) != 1 || resp.Data[0].AccountSysID != "acc-1" {
		t.Fatalf("region filter did not narrow to the APAC account: %+v", resp)
	}
}

// Regression: found live, comparing a HAR capture of the old ServiceNow-backed
// summary endpoint against this one for the identical request -- the two
// reported very different account counts, because filteredAccounts applied
// no active/inactive filter at all. Pins that every SearchAccounts call
// this method makes carries active: true on the wire.
func TestPostgresCustomerHealthClient_GetCustomerHealthSummary_FiltersToActiveAccounts(t *testing.T) {
	fake := &fakeEntityCustomerHealthClient{
		accounts:      entitySearchAccountsResponse{Accounts: []entityAccountView{{ID: "acc-1", Name: "Acme Corp"}}, Total: 1},
		caseResponses: []entitySearchCasesResponse{{Total: 0}},
	}
	c := NewPostgresCustomerHealthClient(fake)

	if _, err := c.GetCustomerHealthSummary(context.Background(), nil, nil, nil, nil, nil, nil, 0, 10); err != nil {
		t.Fatalf("GetCustomerHealthSummary: %v", err)
	}

	var req entitySearchAccountsRequest
	if err := json.Unmarshal(fake.lastAccountsReqRaw, &req); err != nil {
		t.Fatalf("unmarshal captured request: %v", err)
	}
	if req.Filters.Active == nil || !*req.Filters.Active {
		t.Fatalf("expected filters.active=true, got %+v", req.Filters)
	}
}

func TestGroupCasesByPriority(t *testing.T) {
	high := "high"
	cases := []entitySearchCaseView{
		{ID: "c1", Number: "CS001", Severity: &high},
		{ID: "c2", Number: "CS002", Severity: &high},
		{ID: "c3", Number: "CS003"},
	}
	groups := groupCasesByPriority(cases)
	if len(groups) != 2 {
		t.Fatalf("got %d groups, want 2", len(groups))
	}
	if groups[0].Priority != "high" || groups[0].Count != 2 {
		t.Errorf("groups[0] = %+v", groups[0])
	}
	if groups[1].Priority != "Unspecified" || groups[1].Count != 1 {
		t.Errorf("groups[1] = %+v", groups[1])
	}
}

func TestDeployedEolProducts_FiltersPastEolDate(t *testing.T) {
	past := time.Now().Add(-24 * time.Hour).UTC().Format(time.RFC3339)
	future := time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)
	fake := &fakeEntityCustomerHealthClient{
		deployedProducts: entitySearchDeployedProductsEolResponse{
			DeployedProducts: []entityDeployedProductEolView{
				{Product: entityRef{Name: "WSO2 API Manager"}, Version: &entityDeployedProductVersionEolRef{SupportEoLDate: &past}},
				{Product: entityRef{Name: "WSO2 Identity Server"}, Version: &entityDeployedProductVersionEolRef{SupportEoLDate: &future}},
				{Product: entityRef{Name: "No Version Info"}},
			},
			Total: 3,
		},
	}
	c := NewPostgresCustomerHealthClient(fake)

	eol, all, err := c.deployedEolProducts(context.Background(), []string{"dep-1"})
	if err != nil {
		t.Fatalf("deployedEolProducts: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("got %d total deployed products, want 3", len(all))
	}
	if len(eol) != 1 || eol[0].Product.Name != "WSO2 API Manager" {
		t.Fatalf("got %+v, want only the past-EOL product", eol)
	}
}

// TestProjectHasEolProduct_RequestsIncludeInactive pins the fix for a real
// false negative: the real ServiceNow script's own EOL check
// (_getDeploymentsForProject) has no active-only filter, but
// SearchDeployments' own default does, so a deactivated deployment carrying
// real EOL product history was silently excluded. projectHasEolProduct must
// ask entity-service for every deployment regardless of active state.
func TestProjectHasEolProduct_RequestsIncludeInactive(t *testing.T) {
	fake := &fakeEntityCustomerHealthClient{
		deployments: entityDeploymentsSearchResponse{
			Deployments: []entityDeploymentView{{ID: "dep-inactive"}},
			Total:       1,
		},
	}
	c := NewPostgresCustomerHealthClient(fake)

	if _, err := c.projectHasEolProduct(context.Background(), "proj-1"); err != nil {
		t.Fatalf("projectHasEolProduct: %v", err)
	}
	if len(fake.lastDeploymentsReqsRaw) != 1 {
		t.Fatalf("got %d SearchDeployments calls, want 1", len(fake.lastDeploymentsReqsRaw))
	}
	var req entityDeploymentsSearchRequest
	if err := json.Unmarshal(fake.lastDeploymentsReqsRaw[0], &req); err != nil {
		t.Fatalf("unmarshal captured request: %v", err)
	}
	if !req.IncludeInactive {
		t.Errorf("projectHasEolProduct must request includeInactive: true, got %+v", req)
	}
}

// TestProjectDetail_DeploymentsListStaysActiveOnly pins that the EOL-check
// fix above does not change the plain "Deployments" list shown on the
// customer-health detail page: that list's own fetch must keep
// includeInactive: false (today's exact behavior), only the EOL computation
// fetches with it true.
func TestProjectDetail_DeploymentsListStaysActiveOnly(t *testing.T) {
	fake := &fakeEntityCustomerHealthClient{
		deployments: entityDeploymentsSearchResponse{},
	}
	c := NewPostgresCustomerHealthClient(fake)

	if _, err := c.projectDetail(context.Background(), entityProjectView{ID: "proj-1"}, nil, nil, nil); err != nil {
		t.Fatalf("projectDetail: %v", err)
	}
	if len(fake.lastDeploymentsReqsRaw) != 2 {
		t.Fatalf("got %d SearchDeployments calls, want 2 (deploymentRefs + EOL check)", len(fake.lastDeploymentsReqsRaw))
	}
	var refsReq, eolReq entityDeploymentsSearchRequest
	if err := json.Unmarshal(fake.lastDeploymentsReqsRaw[0], &refsReq); err != nil {
		t.Fatalf("unmarshal first captured request: %v", err)
	}
	if err := json.Unmarshal(fake.lastDeploymentsReqsRaw[1], &eolReq); err != nil {
		t.Fatalf("unmarshal second captured request: %v", err)
	}
	if refsReq.IncludeInactive {
		t.Errorf("the Deployments-list fetch must stay includeInactive: false, got %+v", refsReq)
	}
	if !eolReq.IncludeInactive {
		t.Errorf("the EOL-check fetch must request includeInactive: true, got %+v", eolReq)
	}
}

func TestGetCustomerHealthDetail_AccountNotFound(t *testing.T) {
	fake := &fakeEntityCustomerHealthClient{
		accounts: entitySearchAccountsResponse{Accounts: []entityAccountView{}, Total: 0},
	}
	c := NewPostgresCustomerHealthClient(fake)

	_, err := c.GetCustomerHealthDetail(context.Background(), "acc-missing")
	if !errors.Is(err, servicenow.ErrAccountNotFound) {
		t.Fatalf("got err=%v, want servicenow.ErrAccountNotFound", err)
	}
}

func TestGetCustomerHealthDetail_AccountFound(t *testing.T) {
	fake := &fakeEntityCustomerHealthClient{
		accounts: entitySearchAccountsResponse{
			Accounts: []entityAccountView{{ID: "acc-1", Name: "Acme Corp"}},
			Total:    1,
		},
	}
	c := NewPostgresCustomerHealthClient(fake)

	detail, err := c.GetCustomerHealthDetail(context.Background(), "acc-1")
	if err != nil {
		t.Fatalf("GetCustomerHealthDetail: %v", err)
	}
	if detail.AccountName != "Acme Corp" {
		t.Errorf("got AccountName=%q, want %q", detail.AccountName, "Acme Corp")
	}
}

// TestAccountCasesFull_ErrorsPastItsSafetyBound pins the fix for a review
// finding: accountCasesFull must fail loudly, not silently return a
// truncated case list, when an account has more cases than its own
// customerHealthAccountCasePageCap*customerHealthAccountCasePageLimit
// backstop can fetch. A full, never-ending page (every response as large as
// the limit, Total far beyond what's ever fetched) forces the loop to run
// out its page cap without ever observing completion.
func TestAccountCasesFull_ErrorsPastItsSafetyBound(t *testing.T) {
	fullPage := make([]entitySearchCaseView, customerHealthAccountCasePageLimit)
	for i := range fullPage {
		fullPage[i] = entitySearchCaseView{ID: "case"}
	}
	fake := &fakeEntityCustomerHealthClient{
		caseResponses: []entitySearchCasesResponse{{Cases: fullPage, Total: 1_000_000}},
	}
	c := NewPostgresCustomerHealthClient(fake)

	_, err := c.accountCasesFull(context.Background(), "acc-huge")
	if err == nil {
		t.Fatal("expected an error once the page-cap backstop is reached, got nil")
	}
}
