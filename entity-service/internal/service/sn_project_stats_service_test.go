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

package service

import (
	"errors"
	"net/http"
	"net/url"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// TestSNProjectStatsService_GetProjectCaseStats_CaseTypesAreDomainValuesNotUUIDs
// guards against reintroducing the bug where caseTypes was validated (and
// converted via uuidsToSysids) as if it were a slice of UUIDs. CaseTypes is
// this service's own domain vocabulary (case/service_request/
// security_report_analysis/announcement/engagement, matching the case-search
// type filter and this service's own openapi.yaml), translated to SN's
// caseTypes wire values via domainTypeKeysToSN -- e.g. "case" becomes
// "default_case" on the wire, exactly as case search's type filter does.
func TestSNProjectStatsService_GetProjectCaseStats_CaseTypesAreDomainValuesNotUUIDs(t *testing.T) {
	var gotQuery url.Values
	mux := http.NewServeMux()
	mux.HandleFunc("/projects/", func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		_, _ = w.Write([]byte(`{"totalCount":0,"activeCount":0,"outstandingCount":0,"actionRequiredCount":0,
			"stateCount":[],"resolvedCount":{"totalResolvedCount":0,"resolvedWithinSlaCount":0,"resolvedOutsideSlaCount":0},
			"caseTypes":[]}`))
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowProjectStatsService(client)

	projectID := "5aeff120-1b74-c210-2649-97a234bcb54a"
	req := domain.ProjectCaseStatsRequest{
		CaseTypes: []string{"case", "service_request", "announcement"},
	}

	if _, err := svc.GetProjectCaseStats(contextWithUserIDToken("token"), projectID, req); err != nil {
		t.Fatalf("unexpected error for valid domain case types: %v", err)
	}

	got := gotQuery["caseTypes"]
	want := []string{"default_case", "service_request", "announcement"}
	if len(got) != len(want) {
		t.Fatalf("expected %d caseTypes forwarded, got %v", len(want), got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Fatalf("expected caseTypes[%d]=%q (SN wire value), got %q", i, w, got[i])
		}
	}
}

// TestSNProjectStatsService_GetProjectCaseStats_AcceptsDefaultCaseAlias
// guards the production customer-portal frontend's actual case type value:
// "default_case" is ServiceNow's own raw caseType wire value, which the
// frontend sends directly (it predates this service's Postgres-backed
// "case" enum) and must keep working — see caseTypeAliases. It must
// normalize to the same canonical "case" that produces "default_case" on
// the wire, so this also guards against ever double-translating it into
// something else.
func TestSNProjectStatsService_GetProjectCaseStats_AcceptsDefaultCaseAlias(t *testing.T) {
	var gotQuery url.Values
	mux := http.NewServeMux()
	mux.HandleFunc("/projects/", func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		_, _ = w.Write([]byte(`{"totalCount":0,"activeCount":0,"outstandingCount":0,"actionRequiredCount":0,
			"stateCount":[],"resolvedCount":{"totalResolvedCount":0,"resolvedWithinSlaCount":0,"resolvedOutsideSlaCount":0},
			"caseTypes":[]}`))
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowProjectStatsService(client)

	projectID := "5aeff120-1b74-c210-2649-97a234bcb54a"
	req := domain.ProjectCaseStatsRequest{CaseTypes: []string{"default_case"}}

	if _, err := svc.GetProjectCaseStats(contextWithUserIDToken("token"), projectID, req); err != nil {
		t.Fatalf("unexpected error for the default_case alias: %v", err)
	}

	got := gotQuery["caseTypes"]
	if len(got) != 1 || got[0] != "default_case" {
		t.Fatalf(`expected caseTypes=["default_case"] forwarded to SN, got %v`, got)
	}
}

func TestSNProjectStatsService_GetProjectCaseStats_RejectsUnknownCaseType(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/projects/", func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not call upstream when caseTypes is invalid")
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowProjectStatsService(client)

	projectID := "5aeff120-1b74-c210-2649-97a234bcb54a"
	req := domain.ProjectCaseStatsRequest{CaseTypes: []string{"not_a_real_case_type"}}

	_, err := svc.GetProjectCaseStats(contextWithUserIDToken("token"), projectID, req)
	if err == nil {
		t.Fatal("expected an error for a caseTypes value that isn't in the domain vocabulary")
	}
	var valErr *apierror.ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
	}
}
