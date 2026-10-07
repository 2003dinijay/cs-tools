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
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// The customer's proposed time (customer_updated_on) and WSO2's answer
// (customer_updated_date_confirmation) are PostgreSQL-only: ServiceNow's change request API has
// no field for either. What is mirrored of the three acts of that conversation is read from what
// PostgreSQL COMMITTED, and every other PATCH mirrors exactly what it always did.

func sPtr(s string) *string { return &s }

func statePtr(s domain.ChangeRequestState) *domain.ChangeRequestState { return &s }

// runMirror sends req through the dual-write service whose repository answers with committed, and
// returns what the ServiceNow mirror was asked to PATCH (nil when it was not called at all).
func runMirror(t *testing.T, req domain.PatchChangeRequestRequest, committed domain.ChangeRequest) *domain.PatchChangeRequestRequest {
	t.Helper()
	called := make(chan domain.PatchChangeRequestRequest, 1)
	mirror := &stubMirrorChangeRequestService{
		patchChangeRequest: func(_ context.Context, _ string, r domain.PatchChangeRequestRequest) (domain.PatchChangeRequestResponse, error) {
			called <- r
			return domain.PatchChangeRequestResponse{}, nil
		},
	}
	repo := &stubChangeRequestRepo{
		patchChangeRequest: func(_ context.Context, id string, _ domain.PatchChangeRequestRequest, _ string) (domain.ChangeRequest, error) {
			committed.ID = id
			return committed, nil
		},
	}
	failures := &recordingSNWritebackFailures{}
	svc := NewChangeRequestServiceWithSNWriteback(repo, stubUserRepo{}, mirror, NewSNWritebackDispatcher(failures))
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	if _, err := svc.PatchChangeRequest(ctx, testUUID, req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	select {
	case got := <-called:
		return &got
	case <-time.After(400 * time.Millisecond):
		if n := failures.count(); n != 0 {
			t.Fatalf("nothing was dispatched but %d sn_writeback_failures were recorded", n)
		}
		return nil
	}
}

func committedAt(state, start, end string) domain.ChangeRequest {
	cr := domain.ChangeRequest{}
	cr.State = sPtr(state)
	if start != "" {
		cr.PlannedStartOn = sPtr(start)
	}
	if end != "" {
		cr.PlannedEndOn = sPtr(end)
	}
	return cr
}

func TestChangeRequestService_PatchChangeRequest_TheTimeConversationMirror(t *testing.T) {
	const planStart, planEnd = "2030-03-01T09:00:00Z", "2030-03-01T11:00:00Z"
	withProposal := func(cr domain.ChangeRequest, start string) domain.ChangeRequest {
		cr.CustomerProposal = &domain.ChangeRequestCustomerProposal{StartOn: start, Answer: "pending"}
		return cr
	}

	t.Run("a customer's proposal mirrors nothing", func(t *testing.T) {
		for name, req := range map[string]domain.PatchChangeRequestRequest{
			"the start alone":                   {PlannedStartOn: sPtr("2030-03-08 09:00:00")},
			"the start and the derived end":     {PlannedStartOn: sPtr("2030-03-08 09:00:00"), PlannedEndOn: sPtr("2030-03-08 11:00:00")},
			"the start in RFC 3339 with offset": {PlannedStartOn: sPtr("2030-03-08T14:30:00+05:30"), PlannedEndOn: sPtr("2030-03-08T16:30:00+05:30")},
		} {
			got := runMirror(t, req, withProposal(committedAt("customer_approval", planStart, planEnd), "2030-03-08T09:00:00Z"))
			if got != nil {
				t.Fatalf("%s: the mirror was asked to PATCH %+v: the plan did not move, the proposal has no ServiceNow field", name, *got)
			}
		}
	})

	t.Run("Accept mirrors Scheduled and the committed window, in ServiceNow's layout", func(t *testing.T) {
		req := domain.PatchChangeRequestRequest{ConfirmCustomerUpdatedDate: sPtr("agree"), ExpectedCustomerUpdatedOn: sPtr("2030-03-08T09:00:00Z"),
			ExpectedPlannedStartOn: sPtr(planStart), ExpectedPlannedEndOn: sPtr(planEnd)}
		got := runMirror(t, req, committedAt("scheduled", "2030-03-08T09:00:00Z", "2030-03-08T11:00:00Z"))
		if got == nil {
			t.Fatal("Accept was not mirrored")
		}
		want := domain.PatchChangeRequestRequest{State: statePtr(domain.ChangeRequestStateScheduled), PlannedStartOn: sPtr("2030-03-08 09:00:00"), PlannedEndOn: sPtr("2030-03-08 11:00:00")}
		if !reflect.DeepEqual(*got, want) {
			t.Fatalf("Accept mirrored %+v, want exactly {state: scheduled, plannedStartOn, plannedEndOn} %+v", *got, want)
		}
	})

	t.Run("a Re-schedule or a counter-proposal mirrors the window and not the state", func(t *testing.T) {
		for name, req := range map[string]domain.PatchChangeRequestRequest{
			"a plain Re-schedule": {State: statePtr(domain.ChangeRequestStateAuthorize), PlannedStartOn: sPtr("2030-03-08T09:00:00Z"), PlannedEndOn: sPtr("2030-03-08T11:00:00Z")},
			"a different time with the proposal and the window it saw": {State: statePtr(domain.ChangeRequestStateAuthorize), PlannedStartOn: sPtr("2030-03-08 09:00:00"), PlannedEndOn: sPtr("2030-03-08 11:00:00"),
				ExpectedCustomerUpdatedOn: sPtr("2030-03-05T09:00:00Z"), ExpectedPlannedStartOn: sPtr(planStart), ExpectedPlannedEndOn: sPtr(planEnd)},
		} {
			got := runMirror(t, req, committedAt("customer_approval", "2030-03-08T09:00:00Z", "2030-03-08T11:00:00Z"))
			if got == nil {
				t.Fatalf("%s: nothing mirrored", name)
			}
			want := domain.PatchChangeRequestRequest{PlannedStartOn: sPtr("2030-03-08 09:00:00"), PlannedEndOn: sPtr("2030-03-08 11:00:00")}
			if !reflect.DeepEqual(*got, want) {
				t.Fatalf("%s: mirrored %+v, want the window only %+v (ServiceNow stays where PostgreSQL stays)", name, *got, want)
			}
		}
	})

	t.Run("a decline mirrors nothing", func(t *testing.T) {
		req := domain.PatchChangeRequestRequest{State: statePtr(domain.ChangeRequestStateAuthorize), ExpectedCustomerUpdatedOn: sPtr("2030-03-08T09:00:00Z"),
			ExpectedPlannedStartOn: sPtr(planStart), ExpectedPlannedEndOn: sPtr(planEnd)}
		if got := runMirror(t, req, committedAt("customer_approval", planStart, planEnd)); got != nil {
			t.Fatalf("a decline mirrored %+v", *got)
		}
	})

	t.Run("an Accept-only request passes the at-least-one-field check", func(t *testing.T) {
		repo := &stubChangeRequestRepo{patchChangeRequest: func(_ context.Context, id string, _ domain.PatchChangeRequestRequest, _ string) (domain.ChangeRequest, error) {
			return committedAt("scheduled", planStart, planEnd), nil
		}}
		svc := NewChangeRequestService(repo, stubUserRepo{})
		ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
		if _, err := svc.PatchChangeRequest(ctx, testUUID, domain.PatchChangeRequestRequest{ConfirmCustomerUpdatedDate: sPtr("agree")}); err != nil {
			t.Fatalf("an Accept-only request: %v", err)
		}
		// ...and a request with no field at all is still refused.
		_, err := svc.PatchChangeRequest(ctx, testUUID, domain.PatchChangeRequestRequest{})
		var ve *apierror.ValidationError
		if !asValidationError(err, &ve) || ve.Msg != "at least one field must be provided" {
			t.Fatalf("an empty request: %v", err)
		}
	})
}

// Every other PATCH is mirrored exactly as it was before the conversation existed: the same
// fields, the same values, the state included -- the three acts above are the only exceptions,
// and they are recognised by what PostgreSQL committed, never by the shape of the request alone.
func TestChangeRequestService_PatchChangeRequest_EveryOtherPatchMirrorsAsBefore(t *testing.T) {
	const planStart, planEnd = "2030-03-01T09:00:00Z", "2030-03-01T11:00:00Z"
	title := "renamed"
	for _, tc := range []struct {
		name      string
		req       domain.PatchChangeRequestRequest
		committed domain.ChangeRequest
		want      domain.PatchChangeRequestRequest
	}{
		{"a title", domain.PatchChangeRequestRequest{Title: &title}, committedAt("new", "", ""), domain.PatchChangeRequestRequest{Title: &title}},
		{"Request Approval", domain.PatchChangeRequestRequest{State: statePtr(domain.ChangeRequestStateAssess)}, committedAt("assess", "", ""),
			domain.PatchChangeRequestRequest{State: statePtr(domain.ChangeRequestStateAssess)}},
		{"a resend of authorize on a change that is in Authorize (state kept: the change IS in Authorize)", domain.PatchChangeRequestRequest{State: statePtr(domain.ChangeRequestStateAuthorize)},
			committedAt("authorize", "", ""), domain.PatchChangeRequestRequest{State: statePtr(domain.ChangeRequestStateAuthorize)}},
		{"Cancel", domain.PatchChangeRequestRequest{State: statePtr(domain.ChangeRequestStateCanceled)}, committedAt("canceled", "", ""),
			domain.PatchChangeRequestRequest{State: statePtr(domain.ChangeRequestStateCanceled)}},
		{"a staff edit of the window", domain.PatchChangeRequestRequest{PlannedStartOn: sPtr("2030-03-08T14:30:00+05:30"), PlannedEndOn: sPtr("2030-03-08 11:00:00")},
			committedAt("customer_approval", "2030-03-08T09:00:00Z", "2030-03-08T11:00:00Z"),
			domain.PatchChangeRequestRequest{PlannedStartOn: sPtr("2030-03-08 09:00:00"), PlannedEndOn: sPtr("2030-03-08 11:00:00")}},
		{"a staff edit of the window to exactly the time the customer proposed (it IS applied: mirrored)", domain.PatchChangeRequestRequest{PlannedStartOn: sPtr("2030-03-08 09:00:00")},
			func() domain.ChangeRequest {
				cr := committedAt("customer_approval", "2030-03-08T09:00:00Z", "2030-03-08T11:00:00Z")
				cr.CustomerProposal = &domain.ChangeRequestCustomerProposal{StartOn: "2030-03-08T09:00:00Z", Answer: "unanswered"}
				return cr
			}(), domain.PatchChangeRequestRequest{PlannedStartOn: sPtr("2030-03-08 09:00:00")}},
		{"a window edit while a proposal waits that moves to another time", domain.PatchChangeRequestRequest{PlannedStartOn: sPtr("2030-03-20 09:00:00")},
			func() domain.ChangeRequest {
				cr := committedAt("customer_approval", "2030-03-20T09:00:00Z", "2030-03-20T11:00:00Z")
				cr.CustomerProposal = &domain.ChangeRequestCustomerProposal{StartOn: "2030-03-08T09:00:00Z", Answer: "pending"}
				return cr
			}(), domain.PatchChangeRequestRequest{PlannedStartOn: sPtr("2030-03-20 09:00:00")}},
		{"a window edit with a state", domain.PatchChangeRequestRequest{State: statePtr(domain.ChangeRequestStateCanceled), PlannedStartOn: sPtr("2030-03-08 09:00:00")},
			committedAt("canceled", "2030-03-08T09:00:00Z", "2030-03-08T11:00:00Z"),
			domain.PatchChangeRequestRequest{State: statePtr(domain.ChangeRequestStateCanceled), PlannedStartOn: sPtr("2030-03-08 09:00:00")}},
		{"a comment", domain.PatchChangeRequestRequest{Comment: sPtr("hello")}, committedAt("customer_approval", planStart, planEnd), domain.PatchChangeRequestRequest{Comment: sPtr("hello")}},
	} {
		got := runMirror(t, tc.req, tc.committed)
		if got == nil {
			t.Fatalf("%s: nothing was mirrored, want %+v", tc.name, tc.want)
		}
		if !reflect.DeepEqual(*got, tc.want) {
			t.Fatalf("%s: mirrored %+v, want exactly what it always was: %+v", tc.name, *got, tc.want)
		}
	}
}

// The pure ServiceNow data source has no PostgreSQL columns to hold the answer: ServiceNow is the
// authority there and answers the customer's proposed date itself. A request that carries the
// answer is refused up front and nothing is sent; proposals and Re-schedules are forwarded as ever.
func TestSNChangeRequestService_PatchChangeRequest_RefusesTheAnswerToAProposedTime(t *testing.T) {
	svc := NewServiceNowChangeRequestService(nil)
	for name, req := range map[string]domain.PatchChangeRequestRequest{
		"Accept":              {ConfirmCustomerUpdatedDate: sPtr("agree")},
		"Accept with a title": {ConfirmCustomerUpdatedDate: sPtr("agree"), Title: sPtr("x")},
		"the version alone":   {ExpectedCustomerUpdatedOn: sPtr("2030-03-08T09:00:00Z"), Title: sPtr("x")},
	} {
		_, err := svc.PatchChangeRequest(contextWithUserIDToken("token"), testCaseUUID, req)
		var ve *apierror.ValidationError
		if !asValidationError(err, &ve) || ve.Msg != "confirmCustomerUpdatedDate is not supported on the ServiceNow data source: answer the customer's proposed date in ServiceNow" {
			t.Fatalf("%s: err = %v, want the refusal naming where to answer", name, err)
		}
	}
}
