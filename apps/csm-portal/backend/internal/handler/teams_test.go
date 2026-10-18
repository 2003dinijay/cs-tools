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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

type mockEntityTeamsClient struct {
	fn                    func(ctx context.Context, teamID string) ([]byte, error)
	getTeamFn             func(ctx context.Context, teamID string) ([]byte, error)
	updateTeamFn          func(ctx context.Context, teamID string, body []byte) ([]byte, error)
	assignTeamLeadFn      func(ctx context.Context, teamID string, body []byte) ([]byte, error)
	gotUpdateTeamCall     bool
	gotAssignTeamLeadCall bool
}

func (m *mockEntityTeamsClient) GetTeamMembers(ctx context.Context, teamID string) ([]byte, error) {
	return m.fn(ctx, teamID)
}

func (m *mockEntityTeamsClient) GetTeam(ctx context.Context, teamID string) ([]byte, error) {
	if m.getTeamFn != nil {
		return m.getTeamFn(ctx, teamID)
	}
	return []byte(`{"id":"` + teamID + `","name":"Apollo","type":"cre","chatWebhookUrl":null}`), nil
}

func (m *mockEntityTeamsClient) UpdateTeam(ctx context.Context, teamID string, body []byte) ([]byte, error) {
	m.gotUpdateTeamCall = true
	if m.updateTeamFn != nil {
		return m.updateTeamFn(ctx, teamID, body)
	}
	return []byte(`{"id":"` + teamID + `","name":"Apollo","type":"cre","chatWebhookUrl":null}`), nil
}

func (m *mockEntityTeamsClient) AssignTeamLead(ctx context.Context, teamID string, body []byte) ([]byte, error) {
	m.gotAssignTeamLeadCall = true
	if m.assignTeamLeadFn != nil {
		return m.assignTeamLeadFn(ctx, teamID, body)
	}
	return nil, nil
}

const testTeamUUID = "11111111-1111-1111-1111-111111111111"

func TestGetTeamMembers(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewTeamHandler(&mockEntityTeamsClient{})
		r := httptest.NewRequest(http.MethodGet, "/teams/"+testTeamUUID+"/members", nil)
		r.SetPathValue("id", testTeamUUID)
		w := httptest.NewRecorder()
		h.GetTeamMembers(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
	})

	t.Run("rejects a non-UUID id", func(t *testing.T) {
		h := NewTeamHandler(&mockEntityTeamsClient{})
		r := withUser(httptest.NewRequest(http.MethodGet, "/teams/not-a-uuid/members", nil))
		r.SetPathValue("id", "not-a-uuid")
		w := httptest.NewRecorder()
		h.GetTeamMembers(w, r)
		assertStatus(t, w, http.StatusBadRequest)
	})

	t.Run("returns 404 when no members found", func(t *testing.T) {
		entity := &mockEntityTeamsClient{
			fn: func(ctx context.Context, teamID string) ([]byte, error) {
				return []byte(`{"members":[]}`), nil
			},
		}
		h := NewTeamHandler(entity)
		r := withUser(httptest.NewRequest(http.MethodGet, "/teams/"+testTeamUUID+"/members", nil))
		r.SetPathValue("id", testTeamUUID)
		w := httptest.NewRecorder()
		h.GetTeamMembers(w, r)
		assertStatus(t, w, http.StatusNotFound)
	})

	t.Run("returns member name, email, and role from entity-service", func(t *testing.T) {
		entity := &mockEntityTeamsClient{
			fn: func(ctx context.Context, teamID string) ([]byte, error) {
				if teamID != testTeamUUID {
					t.Errorf("teamID = %q, want %q", teamID, testTeamUUID)
				}
				return []byte(`{"members":[{"id":"m1","name":"Jane Doe","email":"jane@example.com","role":"lead"}]}`), nil
			},
		}
		h := NewTeamHandler(entity)
		r := withUser(httptest.NewRequest(http.MethodGet, "/teams/"+testTeamUUID+"/members", nil))
		r.SetPathValue("id", testTeamUUID)
		w := httptest.NewRecorder()
		h.GetTeamMembers(w, r)
		assertStatus(t, w, http.StatusOK)
		views := decodeJSON[[]TeamMemberView](t, w)
		if len(views) != 1 {
			t.Fatalf("got %d members, want 1", len(views))
		}
		v := views[0]
		if v.Name != "Jane Doe" || v.Email != "jane@example.com" || v.Role != "lead" {
			t.Errorf("view = %+v, unexpected", v)
		}
	})

	t.Run("a member with no role still comes back with a blank role rather than an error", func(t *testing.T) {
		entity := &mockEntityTeamsClient{
			fn: func(ctx context.Context, teamID string) ([]byte, error) {
				return []byte(`{"members":[{"id":"m1","name":"Jane Doe","email":"jane@example.com"}]}`), nil
			},
		}
		h := NewTeamHandler(entity)
		r := withUser(httptest.NewRequest(http.MethodGet, "/teams/"+testTeamUUID+"/members", nil))
		r.SetPathValue("id", testTeamUUID)
		w := httptest.NewRecorder()
		h.GetTeamMembers(w, r)
		assertStatus(t, w, http.StatusOK)
		views := decodeJSON[[]TeamMemberView](t, w)
		if len(views) != 1 || views[0].Role != "" {
			t.Errorf("view = %+v, want a blank role rather than an error", views)
		}
	})
}

var testTeamLeadUser = &middleware.UserInfo{
	Email:  "lead@example.com",
	UserID: "33333333-3333-3333-3333-333333333333",
	Roles:  []string{"test-viewer"},
}

var testAdminUser = &middleware.UserInfo{
	Email:  "admin@example.com",
	UserID: "44444444-4444-4444-4444-444444444444",
	Roles:  []string{"test-admin"},
}

func rosterWithLead(leadEmail string) func(ctx context.Context, teamID string) ([]byte, error) {
	return func(ctx context.Context, teamID string) ([]byte, error) {
		return []byte(`{"members":[{"id":"m1","name":"Team Lead","email":"` + leadEmail + `","role":"lead"}]}`), nil
	}
}

func TestGetTeam(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewTeamHandler(&mockEntityTeamsClient{}).WithAccessGuard(viewerAccessGuard)
		r := httptest.NewRequest(http.MethodGet, "/teams/"+testTeamUUID, nil)
		r.SetPathValue("id", testTeamUUID)
		w := httptest.NewRecorder()
		h.GetTeam(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
	})

	t.Run("rejects a non-UUID id", func(t *testing.T) {
		h := NewTeamHandler(&mockEntityTeamsClient{}).WithAccessGuard(viewerAccessGuard)
		r := withUser(httptest.NewRequest(http.MethodGet, "/teams/not-a-uuid", nil))
		r.SetPathValue("id", "not-a-uuid")
		w := httptest.NewRecorder()
		h.GetTeam(w, r)
		assertStatus(t, w, http.StatusBadRequest)
	})

	t.Run("forbidden for a caller who is neither admin nor this team's lead", func(t *testing.T) {
		entity := &mockEntityTeamsClient{fn: rosterWithLead(testTeamLeadUser.Email)}
		h := NewTeamHandler(entity).WithAccessGuard(viewerAccessGuard)
		r := withUser(httptest.NewRequest(http.MethodGet, "/teams/"+testTeamUUID, nil))
		r.SetPathValue("id", testTeamUUID)
		w := httptest.NewRecorder()
		h.GetTeam(w, r)
		assertStatus(t, w, http.StatusForbidden)
	})

	t.Run("allowed for an admin, regardless of team membership", func(t *testing.T) {
		url := "https://chat.googleapis.com/v1/spaces/apollo"
		entity := &mockEntityTeamsClient{
			fn: rosterWithLead(testTeamLeadUser.Email),
			getTeamFn: func(ctx context.Context, teamID string) ([]byte, error) {
				return []byte(`{"id":"` + teamID + `","name":"Apollo","type":"cre","chatWebhookUrl":"` + url + `"}`), nil
			},
		}
		h := NewTeamHandler(entity).WithAccessGuard(viewerAccessGuard)
		r := httptest.NewRequest(http.MethodGet, "/teams/"+testTeamUUID, nil)
		r.SetPathValue("id", testTeamUUID)
		r = r.WithContext(middleware.WithUserInfo(r.Context(), testAdminUser))
		w := httptest.NewRecorder()
		h.GetTeam(w, r)
		assertStatus(t, w, http.StatusOK)
		view := decodeJSON[TeamDetailView](t, w)
		if view.ChatWebhookURL == nil || *view.ChatWebhookURL != url {
			t.Errorf("ChatWebhookURL = %v, want %q", view.ChatWebhookURL, url)
		}
	})

	t.Run("allowed for this team's own lead", func(t *testing.T) {
		entity := &mockEntityTeamsClient{fn: rosterWithLead(testTeamLeadUser.Email)}
		h := NewTeamHandler(entity).WithAccessGuard(viewerAccessGuard)
		r := httptest.NewRequest(http.MethodGet, "/teams/"+testTeamUUID, nil)
		r.SetPathValue("id", testTeamUUID)
		r = r.WithContext(middleware.WithUserInfo(r.Context(), testTeamLeadUser))
		w := httptest.NewRecorder()
		h.GetTeam(w, r)
		assertStatus(t, w, http.StatusOK)
	})
}

func TestUpdateTeam(t *testing.T) {
	t.Run("forbidden for a caller who is neither admin nor this team's lead -- entity is never called", func(t *testing.T) {
		entity := &mockEntityTeamsClient{fn: rosterWithLead(testTeamLeadUser.Email)}
		h := NewTeamHandler(entity).WithAccessGuard(viewerAccessGuard)
		body := strings.NewReader(`{"chatWebhookUrl":"https://chat.googleapis.com/v1/spaces/apollo"}`)
		r := withUser(httptest.NewRequest(http.MethodPatch, "/teams/"+testTeamUUID, body))
		r.SetPathValue("id", testTeamUUID)
		w := httptest.NewRecorder()
		h.UpdateTeam(w, r)
		assertStatus(t, w, http.StatusForbidden)
		if entity.gotUpdateTeamCall {
			t.Error("entity.UpdateTeam should not have been called")
		}
	})

	t.Run("rejects a body with no chatWebhookUrl field, for a caller allowed to update", func(t *testing.T) {
		entity := &mockEntityTeamsClient{fn: rosterWithLead(testTeamLeadUser.Email)}
		h := NewTeamHandler(entity).WithAccessGuard(viewerAccessGuard)
		r := httptest.NewRequest(http.MethodPatch, "/teams/"+testTeamUUID, strings.NewReader(`{}`))
		r.SetPathValue("id", testTeamUUID)
		r = r.WithContext(middleware.WithUserInfo(r.Context(), testTeamLeadUser))
		w := httptest.NewRecorder()
		h.UpdateTeam(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		if entity.gotUpdateTeamCall {
			t.Error("entity.UpdateTeam should not have been called")
		}
	})

	t.Run("this team's own lead may update it", func(t *testing.T) {
		url := "https://chat.googleapis.com/v1/spaces/apollo"
		entity := &mockEntityTeamsClient{
			fn: rosterWithLead(testTeamLeadUser.Email),
			updateTeamFn: func(ctx context.Context, teamID string, body []byte) ([]byte, error) {
				if !strings.Contains(string(body), url) {
					t.Errorf("body = %s, want it to carry %q", body, url)
				}
				return []byte(`{"id":"` + teamID + `","name":"Apollo","type":"cre","chatWebhookUrl":"` + url + `"}`), nil
			},
		}
		h := NewTeamHandler(entity).WithAccessGuard(viewerAccessGuard)
		body := strings.NewReader(`{"chatWebhookUrl":"` + url + `"}`)
		r := httptest.NewRequest(http.MethodPatch, "/teams/"+testTeamUUID, body)
		r.SetPathValue("id", testTeamUUID)
		r = r.WithContext(middleware.WithUserInfo(r.Context(), testTeamLeadUser))
		w := httptest.NewRecorder()
		h.UpdateTeam(w, r)
		assertStatus(t, w, http.StatusOK)
		if !entity.gotUpdateTeamCall {
			t.Error("entity.UpdateTeam should have been called")
		}
	})
}

func TestAssignLead(t *testing.T) {
	const testUserUUID = "22222222-2222-2222-2222-222222222222"

	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewTeamHandler(&mockEntityTeamsClient{})
		r := httptest.NewRequest(http.MethodPost, "/teams/"+testTeamUUID+"/lead", strings.NewReader(`{"userId":"`+testUserUUID+`"}`))
		r.SetPathValue("id", testTeamUUID)
		w := httptest.NewRecorder()
		h.AssignLead(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
	})

	t.Run("rejects a non-UUID team id", func(t *testing.T) {
		h := NewTeamHandler(&mockEntityTeamsClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/teams/not-a-uuid/lead", strings.NewReader(`{"userId":"`+testUserUUID+`"}`)))
		r.SetPathValue("id", "not-a-uuid")
		w := httptest.NewRecorder()
		h.AssignLead(w, r)
		assertStatus(t, w, http.StatusBadRequest)
	})

	t.Run("rejects a non-UUID userId", func(t *testing.T) {
		entity := &mockEntityTeamsClient{}
		h := NewTeamHandler(entity)
		r := withUser(httptest.NewRequest(http.MethodPost, "/teams/"+testTeamUUID+"/lead", strings.NewReader(`{"userId":"not-a-uuid"}`)))
		r.SetPathValue("id", testTeamUUID)
		w := httptest.NewRecorder()
		h.AssignLead(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		if entity.gotAssignTeamLeadCall {
			t.Error("entity.AssignTeamLead should not have been called")
		}
	})

	t.Run("calls entity and returns 204 -- route-level PermAdmin is this handler's only gate", func(t *testing.T) {
		entity := &mockEntityTeamsClient{
			assignTeamLeadFn: func(ctx context.Context, teamID string, body []byte) ([]byte, error) {
				if teamID != testTeamUUID {
					t.Errorf("teamID = %q, want %q", teamID, testTeamUUID)
				}
				if !strings.Contains(string(body), testUserUUID) {
					t.Errorf("body = %s, want it to carry %q", body, testUserUUID)
				}
				return nil, nil
			},
		}
		h := NewTeamHandler(entity)
		r := withUser(httptest.NewRequest(http.MethodPost, "/teams/"+testTeamUUID+"/lead", strings.NewReader(`{"userId":"`+testUserUUID+`"}`)))
		r.SetPathValue("id", testTeamUUID)
		w := httptest.NewRecorder()
		h.AssignLead(w, r)
		assertStatus(t, w, http.StatusNoContent)
		if !entity.gotAssignTeamLeadCall {
			t.Error("entity.AssignTeamLead should have been called")
		}
	})
}
