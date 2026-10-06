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

package repository

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

func TestIsExternalCaller(t *testing.T) {
	ctxWith := func(scope SearchScope) context.Context { return WithCallerIdentity(context.Background(), scope) }
	for _, tc := range []struct {
		name string
		ctx  context.Context
		want bool
	}{
		{"a customer: resolved, restricted", ctxWith(SearchScope{ViewerEmail: "dave@example.com"}), true},
		{"a customer with project ids", ctxWith(SearchScope{ProjectIDs: []string{"p1"}, ViewerEmail: "dave@example.com"}), true},
		{"internal staff: unrestricted", ctxWith(SearchScope{Unrestricted: true, ViewerEmail: "alice@example.com"}), false},
		{"the system identity", WithSystemIdentity(context.Background()), false},
		{"staff who also hold an external record", ctxWith(SearchScope{ViewerEmail: "alice@example.com", HasInternalAccess: true}), false},
		{"no identity at all", context.Background(), false},
	} {
		if got := isExternalCaller(tc.ctx); got != tc.want {
			t.Errorf("%s: isExternalCaller = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestClassifyExternalPatch(t *testing.T) {
	yes, no := true, false
	start, end := "2030-03-01T09:00:00Z", "2030-03-01T11:00:00Z"
	state := domain.ChangeRequestStateScheduled

	t.Run("the customer's answers", func(t *testing.T) {
		for _, tc := range []struct {
			name     string
			req      domain.PatchChangeRequestRequest
			spec     *customerStageSpec
			approved bool
		}{
			{"approve", domain.PatchChangeRequestRequest{IsCustomerApproved: &yes}, &customerApprovalStageSpec, true},
			{"reject", domain.PatchChangeRequestRequest{IsCustomerApproved: &no}, &customerApprovalStageSpec, false},
			{"confirm the review", domain.PatchChangeRequestRequest{IsCustomerReviewed: &yes}, &customerReviewStageSpec, true},
			{"fail the review", domain.PatchChangeRequestRequest{IsCustomerReviewed: &no}, &customerReviewStageSpec, false},
		} {
			got, err := classifyExternalPatch(tc.req)
			if err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			if got.kind != customerPatchAnswer || got.spec != tc.spec || got.approved != tc.approved {
				t.Errorf("%s: classified as %+v, want an answer for %s approved=%v", tc.name, got, tc.spec.label, tc.approved)
			}
		}
	})

	t.Run("a proposed window", func(t *testing.T) {
		for name, req := range map[string]domain.PatchChangeRequestRequest{
			"a start":        {PlannedStartOn: &start},
			"an end":         {PlannedEndOn: &end},
			"a whole window": {PlannedStartOn: &start, PlannedEndOn: &end},
		} {
			got, err := classifyExternalPatch(req)
			if err != nil || got.kind != customerPatchProposal {
				t.Errorf("%s: classified as %+v (%v), want a proposal", name, got, err)
			}
		}
	})

	t.Run("anything else is forbidden, alone or with an allowed field", func(t *testing.T) {
		impact := domain.ChangeRequestImpact("high")
		extra := map[string]func(*domain.PatchChangeRequestRequest){
			"title":                    func(r *domain.PatchChangeRequestRequest) { r.Title = &start },
			"description":              func(r *domain.PatchChangeRequestRequest) { r.Description = &start },
			"projectId":                func(r *domain.PatchChangeRequestRequest) { r.ProjectID = &start },
			"state":                    func(r *domain.PatchChangeRequestRequest) { r.State = &state },
			"impact":                   func(r *domain.PatchChangeRequestRequest) { r.Impact = &impact },
			"assignedTeamId":           func(r *domain.PatchChangeRequestRequest) { r.AssignedTeamID = &start },
			"requestApproval":          func(r *domain.PatchChangeRequestRequest) { r.RequestApproval = &yes },
			"onHold":                   func(r *domain.PatchChangeRequestRequest) { r.OnHold = &yes },
			"comment":                  func(r *domain.PatchChangeRequestRequest) { r.Comment = &start },
			"workNote":                 func(r *domain.PatchChangeRequestRequest) { r.WorkNote = &start },
			"customerApprovalRequired": func(r *domain.PatchChangeRequestRequest) { r.CustomerApprovalRequired = &no },
			"deploymentIds":            func(r *domain.PatchChangeRequestRequest) { r.DeploymentIDs = &[]string{} },
			"implementationPlan":       func(r *domain.PatchChangeRequestRequest) { r.ImplementationPlan = new(*string) },
		}
		for field, set := range extra {
			for name, base := range map[string]domain.PatchChangeRequestRequest{
				"alone":                {},
				"with the answer":      {IsCustomerApproved: &yes},
				"with the review":      {IsCustomerReviewed: &no},
				"with a proposed time": {PlannedStartOn: &start},
			} {
				req := base
				set(&req)
				_, err := classifyExternalPatch(req)
				var fe *apierror.ForbiddenError
				if !errors.As(err, &fe) {
					t.Errorf("%s %s: err = %v (%T), want *apierror.ForbiddenError", field, name, err, err)
					continue
				}
				if !strings.Contains(fe.Msg, "customers can only record") {
					t.Errorf("%s %s: message %q does not say what a customer can do", field, name, fe.Msg)
				}
			}
		}
		// Every field of the request is covered above or is one of the four: a
		// field added to the contract later must either be added to this list
		// (refused) or be a deliberate decision to let a customer set it.
		allowed := map[string]bool{"IsCustomerApproved": true, "IsCustomerReviewed": true, "PlannedStartOn": true, "PlannedEndOn": true}
		covered := map[string]bool{
			"Title": true, "Description": true, "ProjectID": true, "State": true, "Impact": true, "AssignedTeamID": true,
			"RequestApproval": true, "OnHold": true, "Comment": true, "WorkNote": true, "CustomerApprovalRequired": true,
			"DeploymentIDs": true, "ImplementationPlan": true,
		}
		typ := reflect.TypeOf(domain.PatchChangeRequestRequest{})
		for i := 0; i < typ.NumField(); i++ {
			name := typ.Field(i).Name
			if allowed[name] || covered[name] {
				continue
			}
			// Not enumerated above: it must still be refused. Set it through reflection.
			req := domain.PatchChangeRequestRequest{}
			f := reflect.ValueOf(&req).Elem().Field(i)
			f.Set(reflect.New(f.Type().Elem()))
			if _, err := classifyExternalPatch(req); err == nil {
				t.Errorf("field %s is accepted from an external caller", name)
			}
		}
	})

	t.Run("ambiguous combinations are a validation error", func(t *testing.T) {
		for name, tc := range map[string]struct {
			req  domain.PatchChangeRequestRequest
			want string
		}{
			"both outcomes":          {domain.PatchChangeRequestRequest{IsCustomerApproved: &yes, IsCustomerReviewed: &yes}, "not both"},
			"an answer and a window": {domain.PatchChangeRequestRequest{IsCustomerApproved: &yes, PlannedStartOn: &start}, "separate requests"},
			"a review and a window":  {domain.PatchChangeRequestRequest{IsCustomerReviewed: &no, PlannedEndOn: &end}, "separate requests"},
			"nothing at all":         {domain.PatchChangeRequestRequest{}, "at least one field"},
			"all four at once":       {domain.PatchChangeRequestRequest{IsCustomerApproved: &yes, IsCustomerReviewed: &yes, PlannedStartOn: &start, PlannedEndOn: &end}, "separate requests"},
		} {
			_, err := classifyExternalPatch(tc.req)
			var ve *apierror.ValidationError
			if !errors.As(err, &ve) || !strings.Contains(ve.Msg, tc.want) {
				t.Errorf("%s: err = %v (%T), want a ValidationError containing %q", name, err, err, tc.want)
			}
		}
	})
}

func TestStateForMessage(t *testing.T) {
	for in, want := range map[string]string{"": "NEW", "  ": "NEW", "SCHEDULED": "SCHEDULED", "CUSTOMER_REVIEW": "CUSTOMER_REVIEW"} {
		if got := stateForMessage(in); got != want {
			t.Errorf("stateForMessage(%q) = %q, want %q", in, got, want)
		}
	}
	// A change with no state reads as New in the stale-approval refusal, not as a
	// blank.
	var ce *apierror.ConflictError
	if err := staleApprovalRefusal(stageKindCustomerApproval, stateForMessage("")); !errors.As(err, &ce) || !strings.Contains(ce.Msg, "the change request is in New, but") {
		t.Errorf("refusal for a NULL state = %v, want it to say the change is in New", err)
	}
}
