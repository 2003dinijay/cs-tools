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
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/servicenow"
)

// fakeEntityCustomerHealthClient is a scriptable entityCustomerHealthClient
// for testing postgresCustomerHealthClient without a real entity-service.
// Each method keys its canned response off the request it was sent, since a
// single detail call fans out into several different searches (projects,
// deployments, deployed products, cases, escalations).
//
// mu guards every field a method call can mutate (caseCallCount, the
// last*ReqsRaw slices). The read-only fixture fields
// (projects/deployments/.../caseResponses) are set once by each test before
// the call under test runs and never written again, so they need no lock.
type fakeEntityCustomerHealthClient struct {
	projects               entitySearchProjectsResponse
	deployments            entityDeploymentsSearchResponse
	deployedProducts       entitySearchDeployedProductsEolResponse
	caseResponses          []entitySearchCasesResponse // consumed in call order; last one repeats
	escalations            entitySearchEscalationsResponse
	mu                     sync.Mutex
	caseCallCount          int
	lastCasesReqsRaw       [][]byte // every request body SearchCases was called with, in call order
	lastDeploymentsReqsRaw [][]byte // every request body SearchDeployments was called with, in call order
	// account/accountNotFound back GetAccount (GET /accounts/{id}), the
	// real by-primary-key lookup GetCustomerHealthDetail uses.
	account         *entityAccountView
	accountNotFound bool
}

func (f *fakeEntityCustomerHealthClient) GetAccount(ctx context.Context, id string) ([]byte, error) {
	if f.accountNotFound || f.account == nil {
		return nil, &apierror.Error{StatusCode: http.StatusNotFound, Body: `{"message":"account not found"}`}
	}
	return json.Marshal(f.account)
}

func (f *fakeEntityCustomerHealthClient) SearchProjects(ctx context.Context, body []byte) ([]byte, error) {
	return json.Marshal(f.projects)
}

func (f *fakeEntityCustomerHealthClient) SearchCases(ctx context.Context, body []byte) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
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
	f.mu.Lock()
	f.lastDeploymentsReqsRaw = append(f.lastDeploymentsReqsRaw, body)
	f.mu.Unlock()
	return json.Marshal(f.deployments)
}

func (f *fakeEntityCustomerHealthClient) SearchDeployedProducts(ctx context.Context, body []byte) ([]byte, error) {
	return json.Marshal(f.deployedProducts)
}

func (f *fakeEntityCustomerHealthClient) SearchEscalations(ctx context.Context, body []byte) ([]byte, error) {
	return json.Marshal(f.escalations)
}

func strPtr(s string) *string { return &s }

// TestProjectGoLiveStatus_LegacyActivationReportsPlainHasGoneLive pins a real
// bug found by cross-checking a live sample of real accounts (not just
// reading the ServiceNow source): an activation date before the fixed
// 2024-01-01 grandfather-clause cutoff, with no go-live date recorded,
// reports plain "Has Gone Live" from the real system -- every one of four
// real sampled accounts predating the cutoff agreed. An earlier version of
// this port returned a distinct "Has Gone Live ( Customer for over 2 years )"
// string here that no real account ever actually produces.
func TestProjectGoLiveStatus_LegacyActivationReportsPlainHasGoneLive(t *testing.T) {
	now := time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC)
	cases := []string{"2015-11-27", "2017-10-14", "2022-12-22", "2023-09-10"}
	for _, activation := range cases {
		activation := activation
		t.Run(activation, func(t *testing.T) {
			got := projectGoLiveStatus(nil, &activation, now)
			want := servicenow.GoLiveStatus{Status: "Has Gone Live", IsRisk: false, State: "success"}
			if got != want {
				t.Errorf("projectGoLiveStatus(nil, %q, %v) = %+v, want %+v", activation, now, got, want)
			}
		})
	}
}

// TestProjectGoLiveStatus_EveryBranch pins every branch of the legacy rule's
// own go-live-status computation this ports, cross-checked against real
// account data where available (see
// TestProjectGoLiveStatus_LegacyActivationReportsPlainHasGoneLive).
func TestProjectGoLiveStatus_EveryBranch(t *testing.T) {
	now := time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC)
	goLive := "2026-01-01T00:00:00Z"
	noStartDate := ""
	futureActivation := "2027-01-01"
	recentActivation := now.AddDate(0, 0, -10).Format("2006-01-02") // ~10 days ago: < 1 month
	threeMonthsAgo := now.AddDate(0, -3, 0).Format("2006-01-02")    // a real risk-free "pending" window
	sevenMonthsAgo := now.AddDate(0, -7, 0).Format("2006-01-02")    // the real risk window
	legacyActivation := "2020-01-01"

	cases := []struct {
		name           string
		goLiveDate     *string
		activationDate *string
		want           servicenow.GoLiveStatus
	}{
		{"go-live date set", &goLive, nil, servicenow.GoLiveStatus{Status: "Has Gone Live", IsRisk: false, State: "success"}},
		{"no go-live, no activation", nil, nil, servicenow.GoLiveStatus{Status: "N/A (No Start Date)", IsRisk: false, State: "pending"}},
		{"no go-live, blank activation", nil, &noStartDate, servicenow.GoLiveStatus{Status: "N/A (No Start Date)", IsRisk: false, State: "pending"}},
		{"legacy pre-cutoff activation", nil, &legacyActivation, servicenow.GoLiveStatus{Status: "Has Gone Live", IsRisk: false, State: "success"}},
		{"activation date in the future", nil, &futureActivation, servicenow.GoLiveStatus{Status: "N/A (Activation Date in Future)", IsRisk: false, State: "pending"}},
		{"recently activated (under a month)", nil, &recentActivation, servicenow.GoLiveStatus{Status: "Recently Activated (Less than a month)", IsRisk: false, State: "pending"}},
		{"3 months since activation", nil, &threeMonthsAgo, servicenow.GoLiveStatus{Status: "Hasn't Gone Live Since 3 months", IsRisk: false, State: "pending"}},
		{"7 months since activation: the real risk", nil, &sevenMonthsAgo, servicenow.GoLiveStatus{Status: "Hasn't Gone Live in 6 months", IsRisk: true, State: "failure"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := projectGoLiveStatus(tc.goLiveDate, tc.activationDate, now)
			if got != tc.want {
				t.Errorf("projectGoLiveStatus(%v, %v, %v) = %+v, want %+v", tc.goLiveDate, tc.activationDate, now, got, tc.want)
			}
		})
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
	fake := &fakeEntityCustomerHealthClient{accountNotFound: true}
	c := NewPostgresCustomerHealthClient(fake)

	_, err := c.GetCustomerHealthDetail(context.Background(), "acc-missing")
	if !errors.Is(err, servicenow.ErrAccountNotFound) {
		t.Fatalf("got err=%v, want servicenow.ErrAccountNotFound", err)
	}
}

// TestGetCustomerHealthDetail_AccountFound pins the real fix: this used to
// resolve the account via POST /accounts/search with Filters.SearchQuery set
// to the account's own UUID, which entity-service's SearchQuery filter can
// never match (it only compares against account.name/sf_id/number, never
// account.id) -- so every account detail lookup 404'd regardless of whether
// the account existed. GetAccount (GET /accounts/{id}, a real by-primary-key
// lookup) is what this must call instead.
func TestGetCustomerHealthDetail_AccountFound(t *testing.T) {
	fake := &fakeEntityCustomerHealthClient{
		account: &entityAccountView{ID: "acc-1", Name: "Acme Corp"},
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

// TestGetCustomerHealthDetail_ProjectSysIDIsFlat pins a real, confirmed bug:
// ProjectDetail.SysID (and every other sys_id-shaped field this function
// constructs) was left as entity-service's own dashed UUID, while
// GetAccountHealthStatus (internal/risk) returns ProjectSysID in the flat
// 32-character shape the rest of this feature's wire contract uses -- the
// webapp matches a project's health-status row back to its own card by
// comparing these two fields for equality, so the mismatch meant marking a
// project healthy/at-risk never appeared to do anything: the lookup could
// never succeed.
func TestGetCustomerHealthDetail_ProjectSysIDIsFlat(t *testing.T) {
	fake := &fakeEntityCustomerHealthClient{
		account: &entityAccountView{ID: "acc-1", Name: "Acme Corp"},
		projects: entitySearchProjectsResponse{
			Projects: []entityProjectView{{ID: "12f3e20a-1bfd-3610-0bb3-da47b04bcb44", Name: "Project One"}},
			Total:    1,
		},
		caseResponses: []entitySearchCasesResponse{{Total: 0}},
	}
	c := NewPostgresCustomerHealthClient(fake)

	detail, err := c.GetCustomerHealthDetail(context.Background(), "acc-1")
	if err != nil {
		t.Fatalf("GetCustomerHealthDetail: %v", err)
	}
	if len(detail.CustomerProjects) != 1 {
		t.Fatalf("got %d projects, want 1", len(detail.CustomerProjects))
	}
	const wantFlat = "12f3e20a1bfd36100bb3da47b04bcb44"
	if got := detail.CustomerProjects[0].SysID; got != wantFlat {
		t.Errorf("ProjectDetail.SysID = %q, want flat sys_id %q (matching GetAccountHealthStatus's own ProjectSysID shape)", got, wantFlat)
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
