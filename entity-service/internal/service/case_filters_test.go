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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package service

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// fakeJWTWithEmail builds an unsigned-but-well-formed JWT (3 base64url segments)
// whose payload carries the given email claim, matching what emailFromJWT reads.
func fakeJWTWithEmail(t *testing.T, email string) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	payloadBytes, err := json.Marshal(map[string]string{"email": email})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	return header + "." + payload + ".sig"
}

func TestParseCaseFieldFilters_NamedFieldTranslations(t *testing.T) {
	callerEmail, callerErr := "jane.doe@example.com", error(nil)

	cases := []struct {
		name  string
		in    []domain.CaseFieldFilter
		check func(t *testing.T, p domain.ParsedCaseFilters)
	}{
		{
			name: "type in",
			in:   []domain.CaseFieldFilter{{Field: "type", Op: "in", Values: []string{"case", "engagement"}}},
			check: func(t *testing.T, p domain.ParsedCaseFilters) {
				if len(p.Types) != 2 || p.Types[0] != "case" || p.Types[1] != "engagement" {
					t.Fatalf("Types = %v", p.Types)
				}
			},
		},
		{
			name: "tag in maps to Tags",
			in:   []domain.CaseFieldFilter{{Field: "tag", Op: "in", Values: []string{"patch"}}},
			check: func(t *testing.T, p domain.ParsedCaseFilters) {
				if len(p.Tags) != 1 || p.Tags[0] != "patch" {
					t.Fatalf("Tags = %v", p.Tags)
				}
			},
		},
		{
			name: "tag notIn maps to ExcludeTags",
			in:   []domain.CaseFieldFilter{{Field: "tag", Op: "notIn", Values: []string{"patch"}}},
			check: func(t *testing.T, p domain.ParsedCaseFilters) {
				if len(p.ExcludeTags) != 1 || p.ExcludeTags[0] != "patch" {
					t.Fatalf("ExcludeTags = %v", p.ExcludeTags)
				}
			},
		},
		{
			name: "assignedUserId isEmpty maps to Unassigned",
			in:   []domain.CaseFieldFilter{{Field: "assignedUserId", Op: "isEmpty"}},
			check: func(t *testing.T, p domain.ParsedCaseFilters) {
				if !p.Unassigned {
					t.Fatalf("expected Unassigned = true")
				}
			},
		},
		{
			name: "resolutionNotes isEmpty maps to ResolutionNotesEmpty",
			in:   []domain.CaseFieldFilter{{Field: "resolutionNotes", Op: "isEmpty"}},
			check: func(t *testing.T, p domain.ParsedCaseFilters) {
				if !p.ResolutionNotesEmpty {
					t.Fatalf("expected ResolutionNotesEmpty = true")
				}
			},
		},
		{
			name: "createdBy in maps to literal CreatedBy list",
			in:   []domain.CaseFieldFilter{{Field: "createdBy", Op: "in", Values: []string{"a@example.com", "b@example.com"}}},
			check: func(t *testing.T, p domain.ParsedCaseFilters) {
				if len(p.CreatedBy) != 2 {
					t.Fatalf("CreatedBy = %v", p.CreatedBy)
				}
				if p.CreatedByMe {
					t.Fatalf("expected CreatedByMe = false for a literal email list")
				}
			},
		},
		{
			name: "createdBy eq placeholder maps to CreatedByMe",
			in:   []domain.CaseFieldFilter{{Field: "createdBy", Op: "eq", Values: []string{currentUserFilterPlaceholder}}},
			check: func(t *testing.T, p domain.ParsedCaseFilters) {
				if !p.CreatedByMe {
					t.Fatalf("expected CreatedByMe = true")
				}
				if len(p.CreatedBy) != 0 {
					t.Fatalf("expected CreatedBy left empty (SN forwards CreatedByMe as a flag, not folded in), got %v", p.CreatedBy)
				}
			},
		},
		{
			name: "createdOn gte/lte map to StartCreatedDate/EndCreatedDate",
			in: []domain.CaseFieldFilter{
				{Field: "createdOn", Op: "gte", Values: []string{"2026-01-01"}},
				{Field: "createdOn", Op: "lte", Values: []string{"2026-01-31"}},
			},
			check: func(t *testing.T, p domain.ParsedCaseFilters) {
				if p.StartCreatedDate == nil || p.EndCreatedDate == nil {
					t.Fatalf("expected both StartCreatedDate and EndCreatedDate set")
				}
			},
		},
		{
			name: "projectOnboardingStatus in",
			in:   []domain.CaseFieldFilter{{Field: "projectOnboardingStatus", Op: "in", Values: []string{"Completed"}}},
			check: func(t *testing.T, p domain.ParsedCaseFilters) {
				if len(p.ProjectOnboardingStatuses) != 1 || p.ProjectOnboardingStatuses[0] != "Completed" {
					t.Fatalf("ProjectOnboardingStatuses = %v", p.ProjectOnboardingStatuses)
				}
			},
		},
		{
			name: "parentId eq",
			in:   []domain.CaseFieldFilter{{Field: "parentId", Op: "eq", Values: []string{"00000000-0000-0000-0000-000000000000"}}},
			check: func(t *testing.T, p domain.ParsedCaseFilters) {
				if p.ParentID == nil || *p.ParentID != "00000000-0000-0000-0000-000000000000" {
					t.Fatalf("ParentID = %v", p.ParentID)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, err := ParseCaseFieldFilters(tc.in, callerEmail, callerErr)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tc.check(t, p)
		})
	}
}

func TestParseCaseFieldFilters_Rejections(t *testing.T) {
	cases := []struct {
		name string
		in   []domain.CaseFieldFilter
	}{
		{name: "unsupported field", in: []domain.CaseFieldFilter{{Field: "bogus", Op: "in", Values: []string{"x"}}}},
		{name: "unsupported op", in: []domain.CaseFieldFilter{{Field: "type", Op: "bogus", Values: []string{"x"}}}},
		{name: "bad field/op combo", in: []domain.CaseFieldFilter{{Field: "type", Op: "eq", Values: []string{"case"}}}},
		{name: "in with no values", in: []domain.CaseFieldFilter{{Field: "type", Op: "in"}}},
		{name: "assignedUserId isNotEmpty unsupported", in: []domain.CaseFieldFilter{{Field: "assignedUserId", Op: "isNotEmpty"}}},
		{name: "resolutionNotes isNotEmpty unsupported", in: []domain.CaseFieldFilter{{Field: "resolutionNotes", Op: "isNotEmpty"}}},
		{name: "createdBy eq non-placeholder literal", in: []domain.CaseFieldFilter{{Field: "createdBy", Op: "eq", Values: []string{"someone@example.com"}}}},
		{name: "createdOn bad date format", in: []domain.CaseFieldFilter{{Field: "createdOn", Op: "gte", Values: []string{"not-a-date"}}}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseCaseFieldFilters(tc.in, "caller@example.com", nil)
			if err == nil {
				t.Fatalf("expected an error, got nil")
			}
			var ve *apierror.ValidationError
			if !asValidationError(err, &ve) {
				t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
			}
		})
	}
}

func TestParseCaseFieldFilters_CreatedByCurrentUser_RequiresCallerEmail(t *testing.T) {
	filters := []domain.CaseFieldFilter{{Field: "createdBy", Op: "eq", Values: []string{currentUserFilterPlaceholder}}}

	if _, err := ParseCaseFieldFilters(filters, "", nil); err == nil {
		t.Fatalf("expected an error when no caller email is available")
	} else if _, ok := err.(*apierror.UnauthorizedError); !ok {
		t.Fatalf("expected *apierror.UnauthorizedError, got %T: %v", err, err)
	}

	forwardedErr := &apierror.ValidationError{Msg: "x-user-id-token: malformed"}
	if _, err := ParseCaseFieldFilters(filters, "", forwardedErr); err != forwardedErr {
		t.Fatalf("expected the resolver's own error to be forwarded, got %v", err)
	}
}
