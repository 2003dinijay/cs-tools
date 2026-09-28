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
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
)

// fakeWatchListEntityClient records what CreateCase/UpdateCase were actually
// called with, and answers SearchUsers from a fixed email->id fixture -- the
// same shape every other fake in this package uses (embed the interface,
// override only what the test needs).
type fakeWatchListEntityClient struct {
	entityCaseClient
	usersByEmail   map[string]string
	searchUsersErr error

	gotCreateCaseReq entity.CreateCaseRequest
	gotUpdateCaseReq entity.UpdateCaseRequest
}

func (f *fakeWatchListEntityClient) SearchUsers(ctx context.Context, req entity.SearchUsersRequest) (entity.SearchUsersResponse, error) {
	if f.searchUsersErr != nil {
		return entity.SearchUsersResponse{}, f.searchUsersErr
	}
	var users []entity.UserSummary
	for _, email := range req.Filters.Emails {
		if id, ok := f.usersByEmail[email]; ok {
			users = append(users, entity.UserSummary{ID: id, Email: email})
		}
	}
	return entity.SearchUsersResponse{Users: users}, nil
}

func (f *fakeWatchListEntityClient) GetProject(ctx context.Context, id string) (entity.ProjectDetailsView, error) {
	return entity.ProjectDetailsView{ID: id}, nil
}

func (f *fakeWatchListEntityClient) CreateCase(ctx context.Context, req entity.CreateCaseRequest) (entity.CreateCaseResponse, error) {
	f.gotCreateCaseReq = req
	return entity.CreateCaseResponse{}, nil
}

func (f *fakeWatchListEntityClient) UpdateCase(ctx context.Context, id string, req entity.UpdateCaseRequest) (entity.UpdateCaseResponse, error) {
	f.gotUpdateCaseReq = req
	return entity.UpdateCaseResponse{}, nil
}

func TestResolveWatchListUserIDs_ResolvesKnownEmailsAndDropsUnknown(t *testing.T) {
	fake := &fakeWatchListEntityClient{
		usersByEmail: map[string]string{
			"alice@example.com": "11111111-1111-1111-1111-111111111111",
			"bob@example.com":   "22222222-2222-2222-2222-222222222222",
		},
	}
	h := NewCaseHandler(fake)

	got := h.resolveWatchListUserIDs(context.Background(), []string{
		"alice@example.com", "unknown@example.com", "bob@example.com",
	})

	want := []string{"11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222"}
	if len(got) != len(want) {
		t.Fatalf("resolveWatchListUserIDs() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("resolveWatchListUserIDs()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestResolveWatchListUserIDs_EmptyInputSkipsUpstreamCall(t *testing.T) {
	fake := &fakeWatchListEntityClient{
		searchUsersErr: errors.New("SearchUsers must not be called for an empty watch list"),
	}
	h := NewCaseHandler(fake)

	if got := h.resolveWatchListUserIDs(context.Background(), nil); got != nil {
		t.Fatalf("resolveWatchListUserIDs(nil) = %v, want nil", got)
	}
}

func TestResolveWatchListUserIDs_UpstreamErrorReturnsNilNotPartial(t *testing.T) {
	fake := &fakeWatchListEntityClient{searchUsersErr: errors.New("boom")}
	h := NewCaseHandler(fake)

	got := h.resolveWatchListUserIDs(context.Background(), []string{"alice@example.com"})
	if got != nil {
		t.Fatalf("resolveWatchListUserIDs() = %v, want nil on upstream error", got)
	}
}

// TestCreateCase_WatchList_ResolvesEmailsToUserIDs is the regression test for
// the real bug this fix addresses: the frontend sends project-contact email
// addresses in watchList, but entity-service's CreateCase requires UUIDs.
// Before this fix, CreateCase forwarded the emails verbatim.
func TestCreateCase_WatchList_ResolvesEmailsToUserIDs(t *testing.T) {
	fake := &fakeWatchListEntityClient{
		usersByEmail: map[string]string{
			"alice@example.com": "11111111-1111-1111-1111-111111111111",
		},
	}
	h := NewCaseHandler(fake)

	reqBody := `{"projectId":"33333333-3333-3333-3333-333333333333","title":"Test Case","description":"Details","watchList":["alice@example.com"]}`
	req := authedRequest(http.MethodPost, "/cases", reqBody)
	rec := httptest.NewRecorder()

	h.CreateCase(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", rec.Code, rec.Body.String())
	}
	want := []string{"11111111-1111-1111-1111-111111111111"}
	if len(fake.gotCreateCaseReq.WatchList) != 1 || fake.gotCreateCaseReq.WatchList[0] != want[0] {
		t.Fatalf("entity CreateCaseRequest.WatchList = %v, want %v (resolved UUID, not the raw email)", fake.gotCreateCaseReq.WatchList, want)
	}
}

// TestPatchCase_WatchList_ResolvesEmailsToUserIDs is the same regression,
// for the PATCH /cases/{id} path — this is the one a real user hit live
// ("watchList contains invalid UUID") when editing an existing case's watch
// list from the case details page.
func TestPatchCase_WatchList_ResolvesEmailsToUserIDs(t *testing.T) {
	fake := &fakeWatchListEntityClient{
		usersByEmail: map[string]string{
			"alice@example.com": "11111111-1111-1111-1111-111111111111",
			"bob@example.com":   "22222222-2222-2222-2222-222222222222",
		},
	}
	h := NewCaseHandler(fake)

	mux := http.NewServeMux()
	mux.HandleFunc("PATCH /cases/{id}", h.PatchCase)

	reqBody := `{"watchList":["alice@example.com","bob@example.com"]}`
	req := authedRequest(http.MethodPatch, "/cases/33333333-3333-3333-3333-333333333333", reqBody)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}
	want := []string{"11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222"}
	got := fake.gotUpdateCaseReq.WatchList
	if len(got) != len(want) {
		t.Fatalf("entity UpdateCaseRequest.WatchList = %v, want %v (resolved UUIDs, not raw emails)", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("entity UpdateCaseRequest.WatchList[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
