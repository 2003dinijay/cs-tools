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
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/entity"
)

func splScanRequest(t *testing.T, payload SplUserScanRequest) *http.Request {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return withUser(httptest.NewRequest(http.MethodPost, "/spl/scan-user", bytes.NewReader(body)))
}

func TestSplScanUser_AuthGates(t *testing.T) {
	h := NewSplUserScanHandler(&mockSalesEntityClient{}, &mockCSEntityClient{}, []string{"csm-agents"})

	t.Run("requires authenticated user", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "/spl/scan-user", bytes.NewReader([]byte(`{}`)))
		w := httptest.NewRecorder()
		h.ScanUser(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
	})

	t.Run("rejects user outside allowedGroups", func(t *testing.T) {
		h2 := NewSplUserScanHandler(&mockSalesEntityClient{}, &mockCSEntityClient{}, []string{"some-other-group"})
		r := splScanRequest(t, SplUserScanRequest{Email: "a@b.com"})
		w := httptest.NewRecorder()
		h2.ScanUser(w, r)
		assertStatus(t, w, http.StatusForbidden)
	})

	t.Run("rejects malformed JSON body", func(t *testing.T) {
		r := withUser(httptest.NewRequest(http.MethodPost, "/spl/scan-user", bytes.NewReader([]byte(`{not json`))))
		w := httptest.NewRecorder()
		h.ScanUser(w, r)
		assertStatus(t, w, http.StatusBadRequest)
	})
}

func TestSplScanUser_SalesforceSide(t *testing.T) {
	// Every sub-test only exercises the Salesforce branches; the ServiceNow
	// side is given a "found, active, open" shape throughout so its results
	// are constant and don't obscure what's under test.
	neutralCS := &mockCSEntityClient{
		getUserByEmailFn: func(ctx context.Context, email string) (*entity.User, error) {
			return &entity.User{LockedOut: false, UserID: "sys-1"}, nil
		},
		getProjectByProjectKeyFn: func(ctx context.Context, projectKey string) (*entity.Project, error) {
			return &entity.Project{ProjectID: "proj-1", WSO2ClosureState: "Open"}, nil
		},
	}

	t.Run("contact not found, subscription not found", func(t *testing.T) {
		sales := &mockSalesEntityClient{}
		h := NewSplUserScanHandler(sales, neutralCS, []string{"csm-agents"})
		r := splScanRequest(t, SplUserScanRequest{Email: "nobody@example.com", SubscriptionKey: "key-1"})
		w := httptest.NewRecorder()
		h.ScanUser(w, r)
		assertStatus(t, w, http.StatusOK)
		resp := decodeJSON[[]SplScanResponse](t, w)
		sf := resp[0]
		if sf.System != "Salesforce" {
			t.Fatalf("system = %q, want Salesforce", sf.System)
		}
		if sf.SystemResult[0].State {
			t.Error("contact result should not be success")
		}
		if sf.SystemResult[0].Information != splInfoContactNotFound {
			t.Errorf("contact information = %+v, want ContactNotFound", sf.SystemResult[0].Information)
		}
		if sf.SystemResult[1].Information != splInfoSubscriptionNotFound {
			t.Errorf("subscription information = %+v, want SubscriptionNotFound", sf.SystemResult[1].Information)
		}
		if sf.SystemResult[2].Information != splInfoMembershipNotFound {
			t.Errorf("membership information = %+v, want MembershipNotFound", sf.SystemResult[2].Information)
		}
	})

	t.Run("contact found with no memberships", func(t *testing.T) {
		sales := &mockSalesEntityClient{
			getContactByEmailFn: func(ctx context.Context, email string) (*entity.Contact, error) {
				return &entity.Contact{ID: "c1", Account: struct {
					ID             string `json:"id"`
					Name           string `json:"name"`
					Classification string `json:"classification"`
				}{ID: "acct-1"}}, nil
			},
			getSubscriptionByKeyFn: func(ctx context.Context, subscriptionKey string) (*entity.Subscription, error) {
				return &entity.Subscription{ID: "sub-1", CustomerID: "acct-1"}, nil
			},
		}
		h := NewSplUserScanHandler(sales, neutralCS, []string{"csm-agents"})
		r := splScanRequest(t, SplUserScanRequest{Email: "a@b.com", SubscriptionKey: "key-1"})
		w := httptest.NewRecorder()
		h.ScanUser(w, r)
		resp := decodeJSON[[]SplScanResponse](t, w)
		sf := resp[0]
		if !sf.SystemResult[0].State {
			t.Error("contact result should be success")
		}
		if sf.SystemResult[2].Information != splInfoMembershipNotFoundInSubscription {
			t.Errorf("membership information = %+v, want MembershipNotFoundInSubscription", sf.SystemResult[2].Information)
		}
	})

	t.Run("valid customer membership succeeds", func(t *testing.T) {
		sales := &mockSalesEntityClient{
			getContactByEmailFn: func(ctx context.Context, email string) (*entity.Contact, error) {
				return &entity.Contact{
					ID: "c1",
					Account: struct {
						ID             string `json:"id"`
						Name           string `json:"name"`
						Classification string `json:"classification"`
					}{ID: "acct-1", Classification: "Enterprise"},
					Memberships: []entity.ContactMembership{{SubscriptionID: "sub-1", Type: splMembershipTypeCustomer}},
				}, nil
			},
			getSubscriptionByKeyFn: func(ctx context.Context, subscriptionKey string) (*entity.Subscription, error) {
				return &entity.Subscription{ID: "sub-1", CustomerID: "acct-1"}, nil
			},
		}
		h := NewSplUserScanHandler(sales, neutralCS, []string{"csm-agents"})
		r := splScanRequest(t, SplUserScanRequest{Email: "a@b.com", SubscriptionKey: "key-1", IsPartner: false})
		w := httptest.NewRecorder()
		h.ScanUser(w, r)
		resp := decodeJSON[[]SplScanResponse](t, w)
		sf := resp[0]
		if !sf.SystemResult[2].State {
			t.Errorf("membership result should be success, got %+v", sf.SystemResult[2])
		}
	})

	t.Run("customer membership on a partner account is invalid", func(t *testing.T) {
		sales := &mockSalesEntityClient{
			getContactByEmailFn: func(ctx context.Context, email string) (*entity.Contact, error) {
				return &entity.Contact{
					ID: "c1",
					Account: struct {
						ID             string `json:"id"`
						Name           string `json:"name"`
						Classification string `json:"classification"`
					}{ID: "acct-1", Classification: splAccountClassificationPartner},
					Memberships: []entity.ContactMembership{{SubscriptionID: "sub-1", Type: splMembershipTypeCustomer}},
				}, nil
			},
			getSubscriptionByKeyFn: func(ctx context.Context, subscriptionKey string) (*entity.Subscription, error) {
				return &entity.Subscription{ID: "sub-1", CustomerID: "acct-1"}, nil
			},
		}
		h := NewSplUserScanHandler(sales, neutralCS, []string{"csm-agents"})
		// isPartner=true but membership type is CUSTOMER -> invalid on a partner account.
		r := splScanRequest(t, SplUserScanRequest{Email: "a@b.com", SubscriptionKey: "key-1", IsPartner: true})
		w := httptest.NewRecorder()
		h.ScanUser(w, r)
		resp := decodeJSON[[]SplScanResponse](t, w)
		sf := resp[0]
		if sf.SystemResult[2].State {
			t.Error("membership result should not be success")
		}
		if sf.SystemResult[2].Information != splInfoInvalidMembership {
			t.Errorf("membership information = %+v, want InvalidMembership", sf.SystemResult[2].Information)
		}
	})

	t.Run("partner membership on a non-partner account is invalid", func(t *testing.T) {
		sales := &mockSalesEntityClient{
			getContactByEmailFn: func(ctx context.Context, email string) (*entity.Contact, error) {
				return &entity.Contact{
					ID: "c1",
					Account: struct {
						ID             string `json:"id"`
						Name           string `json:"name"`
						Classification string `json:"classification"`
					}{ID: "acct-1", Classification: "Enterprise"},
					Memberships: []entity.ContactMembership{{SubscriptionID: "sub-1", Type: splMembershipTypePartner}},
				}, nil
			},
			getSubscriptionByKeyFn: func(ctx context.Context, subscriptionKey string) (*entity.Subscription, error) {
				return &entity.Subscription{ID: "sub-1", CustomerID: "acct-1"}, nil
			},
		}
		h := NewSplUserScanHandler(sales, neutralCS, []string{"csm-agents"})
		r := splScanRequest(t, SplUserScanRequest{Email: "a@b.com", SubscriptionKey: "key-1", IsPartner: false})
		w := httptest.NewRecorder()
		h.ScanUser(w, r)
		resp := decodeJSON[[]SplScanResponse](t, w)
		sf := resp[0]
		if sf.SystemResult[2].Information != splInfoInvalidCustomerMembership {
			t.Errorf("membership information = %+v, want InvalidCustomerMembership", sf.SystemResult[2].Information)
		}
	})

	t.Run("subscription and contact in different accounts", func(t *testing.T) {
		sales := &mockSalesEntityClient{
			getContactByEmailFn: func(ctx context.Context, email string) (*entity.Contact, error) {
				return &entity.Contact{
					ID: "c1",
					Account: struct {
						ID             string `json:"id"`
						Name           string `json:"name"`
						Classification string `json:"classification"`
					}{ID: "acct-1"},
					Memberships: []entity.ContactMembership{{SubscriptionID: "sub-1", Type: splMembershipTypeCustomer}},
				}, nil
			},
			getSubscriptionByKeyFn: func(ctx context.Context, subscriptionKey string) (*entity.Subscription, error) {
				return &entity.Subscription{ID: "sub-1", CustomerID: "acct-DIFFERENT"}, nil
			},
		}
		h := NewSplUserScanHandler(sales, neutralCS, []string{"csm-agents"})
		r := splScanRequest(t, SplUserScanRequest{Email: "a@b.com", SubscriptionKey: "key-1", IsPartner: false})
		w := httptest.NewRecorder()
		h.ScanUser(w, r)
		resp := decodeJSON[[]SplScanResponse](t, w)
		sf := resp[0]
		if sf.SystemResult[1].Information != splInfoSubscriptionNotFoundInAccount {
			t.Errorf("subscription information = %+v, want SubscriptionNotFoundInAccount", sf.SystemResult[1].Information)
		}
	})
}

func TestSplScanUser_ServiceNowSide(t *testing.T) {
	neutralSales := &mockSalesEntityClient{}

	t.Run("project not found", func(t *testing.T) {
		cs := &mockCSEntityClient{
			getProjectByProjectKeyFn: func(ctx context.Context, projectKey string) (*entity.Project, error) {
				return nil, nil
			},
		}
		h := NewSplUserScanHandler(neutralSales, cs, []string{"csm-agents"})
		r := splScanRequest(t, SplUserScanRequest{Email: "a@b.com", SubscriptionKey: "key-1"})
		w := httptest.NewRecorder()
		h.ScanUser(w, r)
		resp := decodeJSON[[]SplScanResponse](t, w)
		sn := resp[1]
		if sn.System != "Servicenow" {
			t.Fatalf("system = %q, want Servicenow", sn.System)
		}
		if sn.SystemResult[1].Information != splInfoProjectNotFound {
			t.Errorf("project information = %+v, want ProjectNotFound", sn.SystemResult[1].Information)
		}
		if sn.SystemResult[0].Information != splInfoUserNotFoundInProject {
			t.Errorf("user information = %+v, want UserNotFoundInProject", sn.SystemResult[0].Information)
		}
	})

	t.Run("project not open", func(t *testing.T) {
		cs := &mockCSEntityClient{
			getProjectByProjectKeyFn: func(ctx context.Context, projectKey string) (*entity.Project, error) {
				return &entity.Project{ProjectID: "proj-1", WSO2ClosureState: "Closed"}, nil
			},
			getUserByEmailFn: func(ctx context.Context, email string) (*entity.User, error) {
				return &entity.User{LockedOut: false}, nil
			},
		}
		h := NewSplUserScanHandler(neutralSales, cs, []string{"csm-agents"})
		r := splScanRequest(t, SplUserScanRequest{Email: "a@b.com", SubscriptionKey: "key-1"})
		w := httptest.NewRecorder()
		h.ScanUser(w, r)
		resp := decodeJSON[[]SplScanResponse](t, w)
		sn := resp[1]
		if sn.SystemResult[1].State {
			t.Error("project result should not be success when closure state isn't Open")
		}
		wantIssue := "The project is not in open state. The project is in Closed state. The project should be in the Open state."
		if sn.SystemResult[1].Information.Issue != wantIssue {
			t.Errorf("issue = %q, want %q", sn.SystemResult[1].Information.Issue, wantIssue)
		}
	})

	t.Run("user not found in an existing open project", func(t *testing.T) {
		cs := &mockCSEntityClient{
			getProjectByProjectKeyFn: func(ctx context.Context, projectKey string) (*entity.Project, error) {
				return &entity.Project{ProjectID: "proj-1", WSO2ClosureState: "Open"}, nil
			},
			getUserByEmailFn: func(ctx context.Context, email string) (*entity.User, error) {
				return nil, nil
			},
		}
		h := NewSplUserScanHandler(neutralSales, cs, []string{"csm-agents"})
		r := splScanRequest(t, SplUserScanRequest{Email: "a@b.com", SubscriptionKey: "key-1"})
		w := httptest.NewRecorder()
		h.ScanUser(w, r)
		resp := decodeJSON[[]SplScanResponse](t, w)
		sn := resp[1]
		if sn.SystemResult[1].State != true {
			t.Error("project result should be success (Open)")
		}
		if sn.SystemResult[0].Information != splInfoUserNotFound {
			t.Errorf("user information = %+v, want UserNotFound", sn.SystemResult[0].Information)
		}
	})

	t.Run("active user succeeds", func(t *testing.T) {
		cs := &mockCSEntityClient{
			getProjectByProjectKeyFn: func(ctx context.Context, projectKey string) (*entity.Project, error) {
				return &entity.Project{ProjectID: "proj-1", WSO2ClosureState: "Open"}, nil
			},
			getUserByEmailFn: func(ctx context.Context, email string) (*entity.User, error) {
				return &entity.User{LockedOut: false}, nil
			},
		}
		h := NewSplUserScanHandler(neutralSales, cs, []string{"csm-agents"})
		r := splScanRequest(t, SplUserScanRequest{Email: "a@b.com", SubscriptionKey: "key-1"})
		w := httptest.NewRecorder()
		h.ScanUser(w, r)
		resp := decodeJSON[[]SplScanResponse](t, w)
		if !resp[1].SystemResult[0].State {
			t.Error("user result should be success")
		}
	})

	t.Run("locked-out user with an invitation URL", func(t *testing.T) {
		var capturedProjectID string
		cs := &mockCSEntityClient{
			getProjectByProjectKeyFn: func(ctx context.Context, projectKey string) (*entity.Project, error) {
				return &entity.Project{ProjectID: "proj-1", WSO2ClosureState: "Open"}, nil
			},
			getUserByEmailFn: func(ctx context.Context, email string) (*entity.User, error) {
				return &entity.User{LockedOut: true}, nil
			},
			getProjectContactByEmailFn: func(ctx context.Context, email, projectID string) (*entity.ProjectContact, error) {
				capturedProjectID = projectID
				url := "https://example.com/invite/abc"
				return &entity.ProjectContact{InvitationURL: &url}, nil
			},
		}
		h := NewSplUserScanHandler(neutralSales, cs, []string{"csm-agents"})
		r := splScanRequest(t, SplUserScanRequest{Email: "a@b.com", SubscriptionKey: "key-1"})
		w := httptest.NewRecorder()
		h.ScanUser(w, r)
		resp := decodeJSON[[]SplScanResponse](t, w)
		userResult := resp[1].SystemResult[0]
		if userResult.State {
			t.Error("locked-out user result should not be success")
		}
		if userResult.Information.InvitationURL != "https://example.com/invite/abc" {
			t.Errorf("invitationUrl = %q, want the invitation link", userResult.Information.InvitationURL)
		}
		if userResult.Information.Solution != "You need to inform the user to accept the invitation." {
			t.Errorf("solution = %q, unexpected", userResult.Information.Solution)
		}
		if capturedProjectID != "proj-1" {
			t.Errorf("projectID passed to GetProjectContactByEmail = %q, want proj-1", capturedProjectID)
		}
	})

	t.Run("locked-out user with no project contact found", func(t *testing.T) {
		// Ballerina: projectContact is () -> userInvitationUrl is the literal
		// "No invitation url found for the given email" (not empty), so this
		// takes the non-empty branch and that literal string is surfaced as the
		// invitationUrl — verified against service.bal's scan-user resource.
		cs := &mockCSEntityClient{
			getProjectByProjectKeyFn: func(ctx context.Context, projectKey string) (*entity.Project, error) {
				return &entity.Project{ProjectID: "proj-1", WSO2ClosureState: "Open"}, nil
			},
			getUserByEmailFn: func(ctx context.Context, email string) (*entity.User, error) {
				return &entity.User{LockedOut: true}, nil
			},
			getProjectContactByEmailFn: func(ctx context.Context, email, projectID string) (*entity.ProjectContact, error) {
				return nil, nil
			},
		}
		h := NewSplUserScanHandler(neutralSales, cs, []string{"csm-agents"})
		r := splScanRequest(t, SplUserScanRequest{Email: "a@b.com", SubscriptionKey: "key-1"})
		w := httptest.NewRecorder()
		h.ScanUser(w, r)
		resp := decodeJSON[[]SplScanResponse](t, w)
		userResult := resp[1].SystemResult[0]
		if userResult.Information.Solution != "You need to inform the user to accept the invitation." {
			t.Errorf("solution = %q, unexpected", userResult.Information.Solution)
		}
		if userResult.Information.InvitationURL != "No invitation url found for the given email" {
			t.Errorf("invitationUrl = %q, want the literal not-found message", userResult.Information.InvitationURL)
		}
	})

	t.Run("locked-out user whose project contact has no invitation URL set", func(t *testing.T) {
		// Ballerina: projectContact exists but invitationUrl is nil ->
		// userInvitationUrl is nil, which is NOT equal to "", so this ALSO takes
		// the non-empty branch with an empty invitationUrl value — the quirk
		// documented in spl_user_scan.go's ScanUser. This port deliberately
		// deviates and treats it as the empty case instead.
		cs := &mockCSEntityClient{
			getProjectByProjectKeyFn: func(ctx context.Context, projectKey string) (*entity.Project, error) {
				return &entity.Project{ProjectID: "proj-1", WSO2ClosureState: "Open"}, nil
			},
			getUserByEmailFn: func(ctx context.Context, email string) (*entity.User, error) {
				return &entity.User{LockedOut: true}, nil
			},
			getProjectContactByEmailFn: func(ctx context.Context, email, projectID string) (*entity.ProjectContact, error) {
				return &entity.ProjectContact{InvitationURL: nil}, nil
			},
		}
		h := NewSplUserScanHandler(neutralSales, cs, []string{"csm-agents"})
		r := splScanRequest(t, SplUserScanRequest{Email: "a@b.com", SubscriptionKey: "key-1"})
		w := httptest.NewRecorder()
		h.ScanUser(w, r)
		resp := decodeJSON[[]SplScanResponse](t, w)
		userResult := resp[1].SystemResult[0]
		if userResult.Information.Solution != "The user invitation is empty. You need to resend the invitation." {
			t.Errorf("solution = %q, want the empty-invitation solution", userResult.Information.Solution)
		}
		if userResult.Information.InvitationURL != "" {
			t.Errorf("invitationUrl = %q, want empty", userResult.Information.InvitationURL)
		}
	})
}

func TestSplScanUser_UpstreamFailuresReturn500WithBespokeMessage(t *testing.T) {
	t.Run("contact lookup failure", func(t *testing.T) {
		sales := &mockSalesEntityClient{
			getContactByEmailFn: func(ctx context.Context, email string) (*entity.Contact, error) {
				return nil, context.DeadlineExceeded
			},
		}
		h := NewSplUserScanHandler(sales, &mockCSEntityClient{}, []string{"csm-agents"})
		r := splScanRequest(t, SplUserScanRequest{Email: "a@b.com"})
		w := httptest.NewRecorder()
		h.ScanUser(w, r)
		assertStatus(t, w, http.StatusInternalServerError)
		assertErrorMessage(t, w, "Error occurred when retrieving contact information")
	})
}
