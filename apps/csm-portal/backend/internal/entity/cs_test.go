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

package entity

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestCSClient(t *testing.T, tokenSrv, apiSrv *httptest.Server) *CSEntityClient {
	t.Helper()
	return NewCSEntityClient(CSEntityConfig{
		BaseURL:      apiSrv.URL,
		TokenURL:     tokenSrv.URL,
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
	})
}

func TestGetUserByEmail_ReturnsUser(t *testing.T) {
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"user": map[string]any{"lockedOut": true, "userId": "sys-1"}},
		})
	}))
	defer apiSrv.Close()
	tokenSrv := newGraphQLTokenServer(t)
	defer tokenSrv.Close()

	c := newTestCSClient(t, tokenSrv, apiSrv)

	user, err := c.GetUserByEmail(context.Background(), "user@example.com")
	if err != nil {
		t.Fatalf("GetUserByEmail returned error: %v", err)
	}
	if user == nil || !user.LockedOut || user.UserID != "sys-1" {
		t.Errorf("user = %+v, want {LockedOut:true UserID:sys-1}", user)
	}
}

func TestGetUserByEmail_NoMatchReturnsNilNil(t *testing.T) {
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"user": nil}})
	}))
	defer apiSrv.Close()
	tokenSrv := newGraphQLTokenServer(t)
	defer tokenSrv.Close()

	c := newTestCSClient(t, tokenSrv, apiSrv)

	user, err := c.GetUserByEmail(context.Background(), "nobody@example.com")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if user != nil {
		t.Errorf("expected nil user, got %+v", user)
	}
}

func TestGetProjectByProjectKey_ReturnsProject(t *testing.T) {
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"project": map[string]any{
					"accountId": "acct-1", "projectId": "proj-1",
					"wso2ClosureState": "Open", "endDateClosureState": "Open", "invoiceDueDateClosureState": "Open",
				},
			},
		})
	}))
	defer apiSrv.Close()
	tokenSrv := newGraphQLTokenServer(t)
	defer tokenSrv.Close()

	c := newTestCSClient(t, tokenSrv, apiSrv)

	project, err := c.GetProjectByProjectKey(context.Background(), "proj-key-1")
	if err != nil {
		t.Fatalf("GetProjectByProjectKey returned error: %v", err)
	}
	if project == nil || project.ProjectID != "proj-1" || project.WSO2ClosureState != "Open" {
		t.Errorf("project = %+v, want ProjectID=proj-1 WSO2ClosureState=Open", project)
	}
}

func TestGetProjectContactByEmail_ReturnsInvitationURL(t *testing.T) {
	var capturedBody struct {
		Variables struct {
			Email     string `json:"email"`
			ProjectID string `json:"projectId"`
		} `json:"variables"`
	}
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		url := "https://example.com/invite/abc"
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"projectContact": map[string]any{"invitationUrl": url}},
		})
	}))
	defer apiSrv.Close()
	tokenSrv := newGraphQLTokenServer(t)
	defer tokenSrv.Close()

	c := newTestCSClient(t, tokenSrv, apiSrv)

	pc, err := c.GetProjectContactByEmail(context.Background(), "user@example.com", "proj-1")
	if err != nil {
		t.Fatalf("GetProjectContactByEmail returned error: %v", err)
	}
	if pc == nil || pc.InvitationURL == nil || *pc.InvitationURL != "https://example.com/invite/abc" {
		t.Errorf("projectContact = %+v, want invitationUrl set", pc)
	}
	if capturedBody.Variables.Email != "user@example.com" || capturedBody.Variables.ProjectID != "proj-1" {
		t.Errorf("captured variables = %+v, want email/projectId echoed", capturedBody.Variables)
	}
}
