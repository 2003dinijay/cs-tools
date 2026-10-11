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
	"strconv"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/middleware"
)

// fakeStateTransitionEntityClient records what UpdateCase was actually called
// with, and answers GetCase with a fixed current state -- the same
// embed-and-override shape every other fake in this package uses.
type fakeStateTransitionEntityClient struct {
	entityCaseClient
	currentState string
	getCaseErr   error

	gotUpdateCaseReq entity.UpdateCaseRequest
	updateCaseCalls  int
}

func (f *fakeStateTransitionEntityClient) GetCase(ctx context.Context, id string) (entity.CaseView, error) {
	if f.getCaseErr != nil {
		return entity.CaseView{}, f.getCaseErr
	}
	return entity.CaseView{ID: id, State: f.currentState}, nil
}

func (f *fakeStateTransitionEntityClient) UpdateCase(ctx context.Context, id string, req entity.UpdateCaseRequest) (entity.UpdateCaseResponse, error) {
	f.updateCaseCalls++
	f.gotUpdateCaseReq = req
	return entity.UpdateCaseResponse{Case: entity.UpdatedCase{ID: id, State: *req.State}}, nil
}

func patchCaseState(t *testing.T, fake *fakeStateTransitionEntityClient, stateKey int) *httptest.ResponseRecorder {
	t.Helper()
	h := NewCaseHandler(fake)

	mux := http.NewServeMux()
	mux.HandleFunc("PATCH /cases/{id}", h.PatchCase)

	reqBody := `{"stateKey":` + strconv.Itoa(stateKey) + `}`
	req := authedRequest(http.MethodPatch, "/cases/33333333-3333-3333-3333-333333333333", reqBody)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

// patchCaseStateAsRole mirrors patchCaseState, but also attaches resolved
// roles to the request context the way RequirePermission/RequirePermissionOneOf
// do in production -- simulating a request that already passed through the
// RBAC middleware as the given role(s).
func patchCaseStateAsRole(t *testing.T, fake *fakeStateTransitionEntityClient, stateKey int, roles ...middleware.CanonicalRole) *httptest.ResponseRecorder {
	t.Helper()
	h := NewCaseHandler(fake)

	mux := http.NewServeMux()
	mux.HandleFunc("PATCH /cases/{id}", h.PatchCase)

	reqBody := `{"stateKey":` + strconv.Itoa(stateKey) + `}`
	req := authedRequest(http.MethodPatch, "/cases/33333333-3333-3333-3333-333333333333", reqBody)
	req = req.WithContext(middleware.WithResolvedRoles(req.Context(), roles))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

// TestPatchCase_StateTransition_CustomerCanCloseFromMostStates pins the
// legacy ServiceNow scoped-app's ALLOWED_STATE_TRANSITIONS rule: a customer
// may always move open/work_in_progress/waiting_on_wso2/reopened to closed.
func TestPatchCase_StateTransition_CustomerCanCloseFromMostStates(t *testing.T) {
	for _, from := range []string{"open", "work_in_progress", "waiting_on_wso2", "reopened"} {
		t.Run(from, func(t *testing.T) {
			fake := &fakeStateTransitionEntityClient{currentState: from}
			rec := patchCaseState(t, fake, 3) // closed

			if rec.Code != http.StatusOK {
				t.Fatalf("PATCH from %q to closed: got %d, want 200: %s", from, rec.Code, rec.Body.String())
			}
			if fake.updateCaseCalls != 1 {
				t.Fatalf("expected exactly one UpdateCase call, got %d", fake.updateCaseCalls)
			}
			if fake.gotUpdateCaseReq.State == nil || *fake.gotUpdateCaseReq.State != "closed" {
				t.Fatalf("UpdateCase.State = %v, want \"closed\"", fake.gotUpdateCaseReq.State)
			}
		})
	}
}

// TestPatchCase_StateTransition_CustomerCannotStartWork pins the other half
// of the legacy rule: a customer may never move a case into
// work_in_progress, awaiting_info, or solution_proposed themselves (those
// are agent-only transitions, SUPERUSER_STATE_TRANSITIONS in the legacy
// rule set) -- this backend had no such guard before this fix, so any
// stateKey a customer sent was forwarded to entity-service unchecked.
func TestPatchCase_StateTransition_CustomerCannotStartWork(t *testing.T) {
	fake := &fakeStateTransitionEntityClient{currentState: "open"}
	rec := patchCaseState(t, fake, 10) // work_in_progress

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("PATCH open->work_in_progress: got %d, want 400: %s", rec.Code, rec.Body.String())
	}
	if fake.updateCaseCalls != 0 {
		t.Fatalf("UpdateCase must not be called for an illegal transition, got %d calls", fake.updateCaseCalls)
	}
}

// TestPatchCase_StateTransition_SolutionProposedAllowsReopenAndWaiting pins
// the one state with extra customer options in the legacy table: from
// solution_proposed a customer may also reopen the case or send it back to
// waiting_on_wso2, not just close it.
func TestPatchCase_StateTransition_SolutionProposedAllowsReopenAndWaiting(t *testing.T) {
	for stateKey, want := range map[int]string{1006: "reopened", 1003: "waiting_on_wso2"} {
		fake := &fakeStateTransitionEntityClient{currentState: "solution_proposed"}
		rec := patchCaseState(t, fake, stateKey)

		if rec.Code != http.StatusOK {
			t.Fatalf("PATCH solution_proposed->%s: got %d, want 200: %s", want, rec.Code, rec.Body.String())
		}
		if fake.gotUpdateCaseReq.State == nil || *fake.gotUpdateCaseReq.State != want {
			t.Fatalf("UpdateCase.State = %v, want %q", fake.gotUpdateCaseReq.State, want)
		}
	}
}

// TestPatchCase_StateTransition_AwaitingInfoRejectsSolutionProposed: the
// legacy table allows awaiting_info -> {closed, waiting_on_wso2} only, not
// solution_proposed -- that transition is agent-only.
func TestPatchCase_StateTransition_AwaitingInfoRejectsSolutionProposed(t *testing.T) {
	fake := &fakeStateTransitionEntityClient{currentState: "awaiting_info"}
	rec := patchCaseState(t, fake, 6) // solution_proposed

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("PATCH awaiting_info->solution_proposed: got %d, want 400: %s", rec.Code, rec.Body.String())
	}
	if fake.updateCaseCalls != 0 {
		t.Fatalf("UpdateCase must not be called for an illegal transition, got %d calls", fake.updateCaseCalls)
	}
}

// TestPatchCase_StateTransition_AcceptsServiceNowDisplayLabel confirms the
// guard normalizes entity.CaseView.State the same way the rest of this
// package does (dto.NormalizedCaseStateEnum) -- a ServiceNow-backed GetCase
// can return "Work In Progress", not the domain enum, and the transition
// check must still recognize it.
func TestPatchCase_StateTransition_AcceptsServiceNowDisplayLabel(t *testing.T) {
	fake := &fakeStateTransitionEntityClient{currentState: "Work In Progress"}
	rec := patchCaseState(t, fake, 3) // closed

	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH \"Work In Progress\"->closed: got %d, want 200: %s", rec.Code, rec.Body.String())
	}
}

// TestPatchCase_StateTransition_GetCaseErrorIsPropagated: a failure fetching
// the current state must not be swallowed into either a false-accept or a
// generic 500 -- it goes through the normal mapUpstreamError path.
func TestPatchCase_StateTransition_GetCaseErrorIsPropagated(t *testing.T) {
	fake := &fakeStateTransitionEntityClient{getCaseErr: errors.New("boom")}
	rec := patchCaseState(t, fake, 3)

	if rec.Code == http.StatusOK {
		t.Fatalf("expected a non-200 response when GetCase fails, got 200: %s", rec.Body.String())
	}
	if fake.updateCaseCalls != 0 {
		t.Fatalf("UpdateCase must not be called when GetCase fails, got %d calls", fake.updateCaseCalls)
	}
}
