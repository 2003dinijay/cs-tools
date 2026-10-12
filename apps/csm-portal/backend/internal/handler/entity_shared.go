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

import "fmt"

// entity-service wire types shared by the SPL Postgres clients that remain
// (Reports, Usage Metrics, Lookups, User Scan) after Accounts/Projects/Cases
// reads merged onto CS Portal's own /accounts, /projects, and /cases routes
// -- this backend's own copy of entity-service's JSON contract, since
// entity-service is a separate Go module and its domain types can't be
// imported directly (every existing entity-service caller in this codebase
// already works this way, see internal/entity/customer.go). Field
// names/JSON tags mirror entity-service's internal/domain package exactly.

type entityPagination struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

type entityRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type entityUserReference struct {
	ID    *string `json:"id"`
	Email string  `json:"email"`
	Name  string  `json:"name"`
}

// entityAccountRef mirrors entity-service's own domain.AccountRef exactly
// (id/name/type only -- AccountRef has no "number" field).
type entityAccountRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type entityCaseFieldFilter struct {
	Field  string   `json:"field"`
	Op     string   `json:"op"`
	Values []string `json:"values,omitempty"`
}

type entitySearchCasesFilters struct {
	SearchQuery string                  `json:"searchQuery,omitempty"`
	Filters     []entityCaseFieldFilter `json:"filters,omitempty"`
}

type entityCaseSort struct {
	Field string `json:"field"`
	Order string `json:"order"`
}

type entitySearchCasesRequest struct {
	Filters    entitySearchCasesFilters `json:"filters"`
	SortBy     entityCaseSort           `json:"sortBy"`
	Pagination entityPagination         `json:"pagination"`
}

type entitySearchCaseView struct {
	ID               string               `json:"id"`
	InternalID       string               `json:"internalId"`
	Number           string               `json:"number"`
	CreatedOn        string               `json:"createdOn"`
	CreatedBy        *entityUserReference `json:"createdBy"`
	Subject          *string              `json:"subject"`
	Description      *string              `json:"description"`
	State            *string              `json:"state"`
	Severity         *string              `json:"severity"`
	Type             string               `json:"type"`
	EngagementType   *string              `json:"engagementType"`
	Product          *entityRef           `json:"product"`
	Project          *entityRef           `json:"project"`
	ProjectKey       *string              `json:"projectKey"`
	AssignedEngineer *entityUserReference `json:"assignedEngineer"`
	AccountDetails   *entityAccountRef    `json:"account"`
}

type entitySearchCasesResponse struct {
	Cases []entitySearchCaseView `json:"cases"`
	Total int                    `json:"total"`
}

type entityProjectView struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Key       string  `json:"key"`
	StartDate *string `json:"startDate"`
	EndDate   *string `json:"endDate"`
	// ClosureState mirrors entity-service's own ProjectView (embedded
	// ProjectClosureFields) -- project.wso2_closure_state, populated on
	// both its data sources.
	ClosureState *string `json:"closureState"`
	// OnboardingStatus mirrors entity-service's own ProjectView.OnboardingStatus
	// (project.onboarding_status) -- nil when onboarding isn't tracked for
	// this project.
	OnboardingStatus *string `json:"onboardingStatus"`
	// GoLiveDate mirrors entity-service's own ProjectView.GoLiveDate
	// (project.onboarding_go_live_date, RFC3339 -- entity-service marshals
	// *time.Time, not a plain date string like AccountView.ActivationDate).
	// This is the field the customer-health go-live computation actually
	// checks -- OnboardingStatus above is a different, unrelated signal and
	// must not be used for this.
	GoLiveDate *string `json:"goLiveDate"`
}

type entitySearchProjectsRequest struct {
	Pagination  entityPagination `json:"pagination"`
	SearchQuery string           `json:"searchQuery,omitempty"`
	AccountID   string           `json:"accountId,omitempty"`
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

// entitySearchDeployedProductsEolRequest mirrors entityDeployedProductsSearchRequest
// (usage_metrics_postgres.go) -- a separate, narrower request type isn't
// needed; this file only ever needs DeploymentIDs/Pagination, both already on
// that shared shape. Kept here as a type alias-by-field-match is not possible
// in Go, so the two request builders stay independent but wire-identical.
type entitySearchDeployedProductsEolRequest struct {
	Pagination    entityPagination `json:"pagination"`
	DeploymentIDs []string         `json:"deploymentIds"`
}

// entityDeployedProductVersionEolRef is a narrower view of entity-service's
// DeployedProductVersionRef than usage_metrics_postgres.go's own
// entityDeployedProductView.Version (*entityRef, id/name only) -- this one
// specifically needs SupportEoLDate, which that file's shape doesn't carry.
type entityDeployedProductVersionEolRef struct {
	Name           string  `json:"name"`
	SupportEoLDate *string `json:"supportEoLDate"`
}

type entityDeployedProductEolView struct {
	Product entityRef                           `json:"product"`
	Version *entityDeployedProductVersionEolRef `json:"version"`
}

type entitySearchDeployedProductsEolResponse struct {
	DeployedProducts []entityDeployedProductEolView `json:"deployedProducts"`
	Total            int                            `json:"total"`
}

type entitySearchAccountsFilters struct {
	SearchQuery string `json:"searchQuery,omitempty"`
	OwnerEmail  string `json:"ownerEmail,omitempty"`
	// Active mirrors entity-service's own SearchAccountsFilters.Active
	// (account.deactivation_date IS NULL/IS NOT NULL) -- nil means both
	// active and deactivated accounts match.
	Active *bool `json:"active,omitempty"`
	// LifeCycle mirrors entity-service's own SearchAccountsFilters.LifeCycle
	// (account.life_cycle, e.g. "Customer"/"Partner"/"Prospect"/"Lost
	// Customer"/"Lost Prospect") -- a distinct dimension from account
	// classification (business segment: Enterprise/Partner/Cloud/OEM). This
	// is what the legacy system's own account-scoping rule for this feature
	// actually filters on.
	LifeCycle string `json:"lifeCycle,omitempty"`
	// HasProject mirrors entity-service's own SearchAccountsFilters.HasProject
	// -- true additionally requires the account to have at least one
	// project, matching the legacy rule's own join-existence check against
	// the account's projects.
	HasProject bool `json:"hasProject,omitempty"`
}

type entitySearchAccountsRequest struct {
	Pagination entityPagination            `json:"pagination"`
	Filters    entitySearchAccountsFilters `json:"filters,omitempty"`
}

type entityAccountView struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Region *string `json:"region"`
	// SubRegion mirrors entity-service's own AccountView.SubRegion
	// (account.sub_region) -- a finer-grained geography than Region (e.g.
	// "ASIA"/"NA - WEST" under Region "APAC"/"NA"). This, not Region, is
	// what the customer-health region filter's dropdown values actually
	// match against -- see customer_health_postgres.go's own filteredAccounts.
	SubRegion *string `json:"subRegion"`
	// CreTeam/SreTeam mirror entity-service's own AccountView.CreTeam/SreTeam
	// (account.cre_team_id/sre_team_id, resolved to a named group reference)
	// -- what the customer-health "ABT team" filter matches an account's
	// team assignment against.
	CreTeam *entityRef `json:"creTeam"`
	SreTeam *entityRef `json:"sreTeam"`
	// ActivationDate mirrors entity-service's own AccountView.ActivationDate
	// (account.activation_date, plain "YYYY-MM-DD" -- not RFC3339) -- needed
	// by the customer-health go-live computation (see customer_health_postgres.go).
	ActivationDate *string `json:"activationDate"`
}

type entitySearchAccountsResponse struct {
	Accounts []entityAccountView `json:"accounts"`
	Total    int                 `json:"total"`
}

// caseStateToDisplay translates entity-service's domain.CaseState wire
// values (lowercase snake_case, e.g. "work_in_progress") to the six display
// labels SPL's reports show, which are exactly ServiceNow's own state labels
// (e.g. "Work In Progress").
var caseStateToDisplay = map[string]string{
	"open":              "Open",
	"work_in_progress":  "Work In Progress",
	"awaiting_info":     "Awaiting Info",
	"solution_proposed": "Solution Proposed",
	"waiting_on_wso2":   "Waiting on WSO2",
	"reopened":          "Reopened",
	"closed":            "Closed",
}

// derefStr returns "" for a nil pointer, matching servicenow.CaseDetails's
// own convention of empty-string-not-omitted for a field ServiceNow itself
// never leaves absent.
func derefStr(s *string) string {
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
