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

package servicenow

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewClient(Config{BaseURL: srv.URL, Username: "u", Password: "p", TeamScheduleURL: "https://schedule.example.com/", EscalationTemplateID: "tmpl-1"})
}

func TestGetAccounts_BuildsExpectedQuery(t *testing.T) {
	var capturedQuery string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.Query().Get("sysparm_query")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result":[]}`))
	})

	email := "user@example.com"
	if _, err := c.GetAccounts(context.Background(), &email, nil, nil, 0, 10, false); err != nil {
		t.Fatalf("GetAccounts returned error: %v", err)
	}
	want := "u_owner.email=user@example.com^ORu_renewal_account_manager.email=user@example.com^ORu_technical_owner.email=user@example.com"
	if capturedQuery != want {
		t.Errorf("sysparm_query = %q, want %q", capturedQuery, want)
	}
}

func TestGetAccounts_RejectsUnsafeEmail(t *testing.T) {
	called := false
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	email := "user@example.com^OR active=true"
	_, err := c.GetAccounts(context.Background(), &email, nil, nil, 0, 10, false)
	if err == nil {
		t.Fatal("expected an error for an unsafe email value, got nil")
	}
	var unsafe *ErrUnsafeQueryValue
	if !errors.As(err, &unsafe) {
		t.Fatalf("expected *ErrUnsafeQueryValue, got %T: %v", err, err)
	}
	if called {
		t.Error("upstream should not have been called for an unsafe query value")
	}
}

func TestGetAccounts_MapsResponseToPortalShape(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"result": []map[string]any{
				{
					"number": "ACC001", "name": "Acme", "u_region": "APAC", "u_arr_today": "1000",
					"u_owner.name": "Alice", "u_technical_owner.name": "Bob",
					"u_integration_cs_team.sys_id": "cs-team-1",
				},
			},
		})
	})

	result, err := c.GetAccounts(context.Background(), nil, nil, nil, 0, 10, false)
	if err != nil {
		t.Fatalf("GetAccounts returned error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("len(result) = %d, want 1", len(result))
	}
	got := result[0]
	if got.Number != "ACC001" || got.Name != "Acme" || got.AccountManager != "Alice" || got.TechnicalOwner != "Bob" {
		t.Errorf("unexpected account details: %+v", got)
	}
	if got.IntegrationCSTeamScheduleURL != "https://schedule.example.com/cs-team-1" {
		t.Errorf("IntegrationCSTeamScheduleURL = %q, want %q", got.IntegrationCSTeamScheduleURL, "https://schedule.example.com/cs-team-1")
	}
}

func TestGetAccountByID_NotFound(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result":[]}`))
	})

	_, err := c.GetAccountByID(context.Background(), "ACC404")
	if !errors.Is(err, ErrAccountNotFound) {
		t.Fatalf("expected ErrAccountNotFound, got %v", err)
	}
}

func TestEscalateCase_LinksExistingActiveEscalation(t *testing.T) {
	var patchCalled bool
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/now/table/customer_account":
			_, _ = w.Write([]byte(`{"result":[{"sys_id":"acct-sys-1"}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/now/table/sn_customerservice_case":
			if strings.Contains(r.URL.Query().Get("sysparm_query"), "active_account_escalation") {
				// isCaseEscalated: report no match, so EscalateCase proceeds to link it.
				_, _ = w.Write([]byte(`{"result":[]}`))
				return
			}
			_, _ = w.Write([]byte(`{"result":[{"sys_id":"case-sys-1"}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/now/table/sn_customerservice_escalation":
			_, _ = w.Write([]byte(`{"result":[{"sys_id":"esc-sys-1"}]}`))
		case r.Method == http.MethodPatch:
			patchCalled = true
			_, _ = w.Write([]byte(`{"result":{"number":"CS0001","sys_id":"case-sys-1"}}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})

	req := EscalationRequest{Justification: "urgent", RequestSource: "Customer", Reason: "Inactivity", Severity: "High Severity"}
	result, err := c.EscalateCase(context.Background(), "ACC1", "CS0001", req, "agent@example.com")
	if err != nil {
		t.Fatalf("EscalateCase returned error: %v", err)
	}
	if !patchCalled {
		t.Error("expected the case to be PATCHed to link it to the escalation")
	}
	if result.LinkedCaseNumber != "CS0001" || result.SysID != "case-sys-1" {
		t.Errorf("unexpected result: %+v", result)
	}
}

func TestEscalateCase_ConflictWhenAlreadyEscalated(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/now/table/customer_account":
			_, _ = w.Write([]byte(`{"result":[{"sys_id":"acct-sys-1"}]}`))
		case r.URL.Path == "/api/now/table/sn_customerservice_escalation":
			_, _ = w.Write([]byte(`{"result":[{"sys_id":"esc-sys-1"}]}`))
		case r.URL.Path == "/api/now/table/sn_customerservice_case":
			// Both the case sys_id lookup and the isCaseEscalated check hit
			// this same table; either way, report a match.
			_, _ = w.Write([]byte(`{"result":[{"sys_id":"case-sys-1"}]}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})

	req := EscalationRequest{Justification: "urgent", RequestSource: "Customer", Reason: "Inactivity", Severity: "High Severity"}
	_, err := c.EscalateCase(context.Background(), "ACC1", "CS0001", req, "agent@example.com")
	if !errors.Is(err, ErrEscalationConflict) {
		t.Fatalf("expected ErrEscalationConflict, got %v", err)
	}
}
