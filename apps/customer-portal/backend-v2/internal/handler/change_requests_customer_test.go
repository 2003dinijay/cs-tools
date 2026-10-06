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
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/middleware"
)

const testChangeRequestID = "22222222-2222-2222-2222-222222222222"

// fakeEntityChangeRequestClient records what the handler forwards to
// entity-service. entityChangeRequestClient is embedded (nil) so only the two
// methods under test are implemented.
type fakeEntityChangeRequestClient struct {
	entityChangeRequestClient
	patchCalls  int
	gotPatch    entity.PatchChangeRequestRequest
	patchErr    error
	decideCalls int
	gotDecision string
}

func (f *fakeEntityChangeRequestClient) UpdateChangeRequest(_ context.Context, id string, req entity.PatchChangeRequestRequest) (entity.PatchChangeRequestResponse, error) {
	f.patchCalls++
	f.gotPatch = req
	if f.patchErr != nil {
		return entity.PatchChangeRequestResponse{}, f.patchErr
	}
	var out entity.PatchChangeRequestResponse
	out.ChangeRequest.ID = id
	out.ChangeRequest.UpdatedOn = "2026-10-06T00:00:00Z"
	return out, nil
}

func (f *fakeEntityChangeRequestClient) DecideChangeRequestApproval(_ context.Context, id string, req entity.ChangeRequestApprovalDecisionRequest) (entity.ChangeRequestApprovalDecisionResponse, error) {
	f.decideCalls++
	f.gotDecision = req.Decision
	return entity.ChangeRequestApprovalDecisionResponse{ID: id, State: req.Decision}, nil
}

// patchAs sends PATCH /change-requests/{id} to the handler as a caller the route
// let in at the given level (a zero action is a request that never passed
// through the permission middleware).
func patchAs(t *testing.T, fake *fakeEntityChangeRequestClient, granted middleware.Action, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := authedRequest(http.MethodPatch, "/change-requests/"+testChangeRequestID, body)
	req.SetPathValue("id", testChangeRequestID)
	if granted != "" {
		req = req.WithContext(middleware.WithGrantedAction(req.Context(), granted))
	}
	rec := httptest.NewRecorder()
	NewChangeRequestHandler(fake).PatchChangeRequest(rec, req)
	return rec
}

func wantMessage(t *testing.T, rec *httptest.ResponseRecorder, contains string) {
	t.Helper()
	var body struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not JSON: %v (%s)", err, rec.Body.String())
	}
	if !strings.Contains(body.Message, contains) {
		t.Fatalf("message = %q, want it to contain %q", body.Message, contains)
	}
}

// What a customer is allowed to send, and exactly what reaches entity-service.
func TestPatchChangeRequest_CustomerAllowedBodies(t *testing.T) {
	yes, no := true, false
	start, end := "2026-10-10 10:00:00", "2026-10-10 12:00:00"
	tests := []struct {
		name, body string
		want       entity.PatchChangeRequestRequest
	}{
		{"approve", `{"isCustomerApproved":true}`, entity.PatchChangeRequestRequest{IsCustomerApproved: &yes}},
		{"reject", `{"isCustomerApproved":false}`, entity.PatchChangeRequestRequest{IsCustomerApproved: &no}},
		{"confirm the review", `{"isCustomerReviewed":true}`, entity.PatchChangeRequestRequest{IsCustomerReviewed: &yes}},
		{"fail the review", `{"isCustomerReviewed":false}`, entity.PatchChangeRequestRequest{IsCustomerReviewed: &no}},
		{"propose a start (what the webapp sends)", `{"plannedStartOn":"2026-10-10 10:00:00"}`, entity.PatchChangeRequestRequest{PlannedStartOn: &start}},
		{"propose a window", `{"plannedStartOn":"2026-10-10 10:00:00","plannedEndOn":"2026-10-10 12:00:00"}`, entity.PatchChangeRequestRequest{PlannedStartOn: &start, PlannedEndOn: &end}},
		{"field names are matched case-insensitively like everywhere else", `{"ISCUSTOMERAPPROVED":true}`, entity.PatchChangeRequestRequest{IsCustomerApproved: &yes}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeEntityChangeRequestClient{}
			rec := patchAs(t, fake, middleware.ActionDecide, tt.body)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200. body: %s", rec.Code, rec.Body.String())
			}
			if fake.patchCalls != 1 {
				t.Fatalf("entity-service called %d times, want 1", fake.patchCalls)
			}
			if !reflect.DeepEqual(fake.gotPatch, tt.want) {
				t.Errorf("forwarded %+v, want exactly %+v", fake.gotPatch, tt.want)
			}
		})
	}
}

// Every field of entity-service's PATCH contract other than the four customer
// ones is refused for a customer, with nothing forwarded. Derived from the
// struct, so a field added to entity-service later is covered without anyone
// remembering to list it here: it is refused until a customer is deliberately
// allowed to set it.
func TestPatchChangeRequest_CustomerCannotSendAnyOtherField(t *testing.T) {
	allowed := map[string]bool{"isCustomerApproved": true, "isCustomerReviewed": true, "plannedStartOn": true, "plannedEndOn": true}
	var fields []string
	typ := reflect.TypeOf(entity.PatchChangeRequestRequest{})
	for i := 0; i < typ.NumField(); i++ {
		name := strings.Split(typ.Field(i).Tag.Get("json"), ",")[0]
		if name != "" && !allowed[name] {
			fields = append(fields, name)
		}
	}
	// Fields the portal never had in either struct, and spellings that must not
	// slip through.
	fields = append(fields, "state", "onHold", "onHoldReason", "priority", "category", "customerApprovalRequired",
		"customerReviewRequired", "comment", "workNote", "isPlanningVisibleToCustomers", "deploymentIds",
		"notAField", "title ", " title", "TITLE", "Title", "is_customer_approved", "isCustomerApprovedX", "__proto__")
	if len(fields) < 25 {
		t.Fatalf("only %d forbidden fields enumerated; the struct walk is broken", len(fields))
	}

	for _, field := range fields {
		for _, body := range []string{
			`{"` + field + `":"x"}`,
			`{"` + field + `":null}`,
			`{"` + field + `":true}`,
			`{"isCustomerApproved":true,"` + field + `":"x"}`,
			`{"` + field + `":"x","plannedStartOn":"2026-10-10 10:00:00"}`,
		} {
			fake := &fakeEntityChangeRequestClient{}
			rec := patchAs(t, fake, middleware.ActionDecide, body)
			if rec.Code != http.StatusForbidden {
				t.Errorf("%s: status = %d, want 403", body, rec.Code)
			}
			if fake.patchCalls != 0 {
				t.Errorf("%s: reached entity-service (%+v)", body, fake.gotPatch)
			}
		}
	}

	t.Run("the refusal says what a customer can do", func(t *testing.T) {
		rec := patchAs(t, &fakeEntityChangeRequestClient{}, middleware.ActionDecide, `{"title":"x"}`)
		wantMessage(t, rec, "approve or reject")
	})
}

func TestPatchChangeRequest_CustomerMalformedAndAmbiguousBodies(t *testing.T) {
	tests := []struct {
		name, body, wantMsg string
	}{
		{"empty object", `{}`, "At least one of"},
		{"wrong type for the flag", `{"isCustomerApproved":"yes"}`, "Invalid request payload"},
		{"wrong type for the window", `{"plannedStartOn":12}`, "Invalid request payload"},
		{"both outcomes", `{"isCustomerApproved":true,"isCustomerReviewed":true}`, "not both"},
		{"answer and a proposed time together", `{"isCustomerApproved":true,"plannedStartOn":"2026-10-10 10:00:00"}`, "separate requests"},
		{"review and a proposed time together", `{"isCustomerReviewed":false,"plannedEndOn":"2026-10-10 10:00:00"}`, "separate requests"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeEntityChangeRequestClient{}
			rec := patchAs(t, fake, middleware.ActionDecide, tt.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400. body: %s", rec.Code, rec.Body.String())
			}
			wantMessage(t, rec, tt.wantMsg)
			if fake.patchCalls != 0 {
				t.Errorf("a refused body reached entity-service: %+v", fake.gotPatch)
			}
		})
	}
}

// The restriction must hold when the request never went through the permission
// middleware, or went through it at a level other than Update: only a positive
// match on Update gets the wide field set.
func TestPatchChangeRequest_OnlyUpdateGetsTheFullFieldSet(t *testing.T) {
	for _, granted := range []middleware.Action{"", middleware.ActionDecide, middleware.ActionRead, middleware.ActionDelete} {
		fake := &fakeEntityChangeRequestClient{}
		rec := patchAs(t, fake, granted, `{"title":"new title"}`)
		if rec.Code != http.StatusForbidden || fake.patchCalls != 0 {
			t.Errorf("granted %q: status %d, upstream calls %d; want 403 and none", granted, rec.Code, fake.patchCalls)
		}
	}
}

// Staff (Update) are unchanged: the full customer-safe field set, extra keys
// dropped by the struct as before.
func TestPatchChangeRequest_StaffUnchanged(t *testing.T) {
	fake := &fakeEntityChangeRequestClient{}
	rec := patchAs(t, fake, middleware.ActionUpdate,
		`{"title":"new title","impact":"high","isCustomerApproved":true,"state":"scheduled","assignedTeamId":"x"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200. body: %s", rec.Code, rec.Body.String())
	}
	got := fake.gotPatch
	if got.Title == nil || *got.Title != "new title" || got.Impact == nil || *got.Impact != "high" || got.IsCustomerApproved == nil || !*got.IsCustomerApproved {
		t.Errorf("forwarded %+v, want title, impact and isCustomerApproved", got)
	}
	// state / assignedTeamId are not in dto.ChangeRequestUpdateRequest and stay
	// dropped, as they always were.
	if got.State != nil || got.AssignedTeamID != nil {
		t.Errorf("forwarded state/assignedTeamId (%v / %v): the staff field set must stay the customer-safe subset", got.State, got.AssignedTeamID)
	}

	if rec := patchAs(t, &fakeEntityChangeRequestClient{}, middleware.ActionUpdate, `{}`); rec.Code != http.StatusBadRequest {
		t.Errorf("empty body as staff: status = %d, want 400", rec.Code)
	}
}

// What the customer is told when entity-service refuses: a 409 carries its
// readable message through, a 403 is the generic one (never upstream text), and
// the id and body are validated before anything is sent.
func TestPatchChangeRequest_CustomerUpstreamErrors(t *testing.T) {
	t.Run("409 passes the readable message through", func(t *testing.T) {
		msg := "this approval is no longer pending: the change request is in Scheduled, but the Customer Approval stage can only be decided while it is in Customer Approval"
		fake := &fakeEntityChangeRequestClient{patchErr: &apierror.Error{StatusCode: http.StatusConflict, Body: msg}}
		rec := patchAs(t, fake, middleware.ActionDecide, `{"isCustomerApproved":true}`)
		if rec.Code != http.StatusConflict {
			t.Fatalf("status = %d, want 409", rec.Code)
		}
		wantMessage(t, rec, "no longer pending")
	})
	t.Run("403 from a registered contact of another project is the generic message", func(t *testing.T) {
		fake := &fakeEntityChangeRequestClient{patchErr: &apierror.Error{StatusCode: http.StatusForbidden, Body: "only an internal user or a registered PORTAL_USER contact ..."}}
		rec := patchAs(t, fake, middleware.ActionDecide, `{"isCustomerApproved":true}`)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", rec.Code)
		}
		if strings.Contains(rec.Body.String(), "PORTAL_USER") {
			t.Errorf("upstream 403 text leaked: %s", rec.Body.String())
		}
	})
	t.Run("a bad id is refused before anything is sent", func(t *testing.T) {
		fake := &fakeEntityChangeRequestClient{}
		req := authedRequest(http.MethodPatch, "/change-requests/not-a-uuid", `{"isCustomerApproved":true}`)
		req.SetPathValue("id", "not-a-uuid")
		req = req.WithContext(middleware.WithGrantedAction(req.Context(), middleware.ActionDecide))
		rec := httptest.NewRecorder()
		NewChangeRequestHandler(fake).PatchChangeRequest(rec, req)
		if rec.Code != http.StatusBadRequest || fake.patchCalls != 0 {
			t.Errorf("status %d, upstream calls %d; want 400 and none", rec.Code, fake.patchCalls)
		}
	})
}
