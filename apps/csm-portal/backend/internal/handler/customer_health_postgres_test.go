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
	accounts         entitySearchAccountsResponse
	projects         entitySearchProjectsResponse
	deployments      entityDeploymentsSearchResponse
	deployedProducts entitySearchDeployedProductsEolResponse
	caseResponses    []entitySearchCasesResponse // consumed in call order; last one repeats
	escalations      entitySearchEscalationsResponse
	caseCallCount    int
}

func (f *fakeEntityCustomerHealthClient) SearchAccounts(ctx context.Context, body []byte) ([]byte, error) {
	return json.Marshal(f.accounts)
}

func (f *fakeEntityCustomerHealthClient) SearchProjects(ctx context.Context, body []byte) ([]byte, error) {
	return json.Marshal(f.projects)
}

func (f *fakeEntityCustomerHealthClient) SearchCases(ctx context.Context, body []byte) ([]byte, error) {
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

func TestPostgresCustomerHealthClient_GetCustomerHealthSummary_NoGoLiveAndNoCases(t *testing.T) {
	fake := &fakeEntityCustomerHealthClient{
		accounts: entitySearchAccountsResponse{
			Accounts: []entityAccountView{{ID: "acc-2", Name: "Beta Inc"}},
			Total:    1,
		},
		projects: entitySearchProjectsResponse{
			Projects: []entityProjectView{{ID: "proj-2", Name: "Beta Platform", OnboardingStatus: strPtr("In-Progress")}},
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
	if !acc.HasNoGoLive.IsRisk {
		t.Errorf("expected no-go-live risk for an In-Progress project, got %+v", acc.HasNoGoLive)
	}
	if acc.HasRecentCases || !acc.NoSupportCases6mo {
		t.Errorf("expected no recent cases / noSupportCases6mo true, got recent=%v noSupport=%v", acc.HasRecentCases, acc.NoSupportCases6mo)
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
