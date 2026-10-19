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
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// TestPendingVerificationSeverityFromEnum locks in the CodeRabbit finding:
// pending_verification's severity must agree with GetCaseByID/SearchCases'
// own Severity field (lowercase domain words, via caseSeverityFromEnum),
// not the raw case_severity_enum label ('S0'..'S4') the column actually
// stores.
func TestPendingVerificationSeverityFromEnum(t *testing.T) {
	tests := []struct {
		name string
		raw  *string
		want *string
	}{
		{"nil stays nil", nil, nil},
		{"S0 maps to catastrophic", strPtr("S0"), strPtr("catastrophic")},
		{"S1 maps to critical", strPtr("S1"), strPtr("critical")},
		{"S2 maps to high", strPtr("S2"), strPtr("high")},
		{"S3 maps to medium", strPtr("S3"), strPtr("medium")},
		{"S4 maps to low", strPtr("S4"), strPtr("low")},
		{"an unrecognized label maps to nil, not passed through raw", strPtr("S9"), nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pendingVerificationSeverityFromEnum(tt.raw)
			if (got == nil) != (tt.want == nil) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			if got != nil && *got != *tt.want {
				t.Fatalf("got %q, want %q", *got, *tt.want)
			}
		})
	}
}

// TestDedupeAddedReasonClause locks in the CodeRabbit finding: when
// VerifiedOnly is set, addedReason must be applied AFTER Search's own
// DISTINCT ON dedup (as an outer filter), never as a pre-dedup inner WHERE
// clause, or dedup can pick a stale, already-verified cycle over a case's
// true latest (different-reason, unverified) cycle.
func TestDedupeAddedReasonClause(t *testing.T) {
	autoClosed := domain.PendingVerificationAddedReasonAutoClosed

	tests := []struct {
		name       string
		filters    domain.PendingVerificationSearchFilters
		argOffset  int
		wantClause string
		wantArgs   []any
	}{
		{
			name:       "nil AddedReason produces no clause and no args",
			filters:    domain.PendingVerificationSearchFilters{},
			argOffset:  0,
			wantClause: "",
			wantArgs:   nil,
		},
		{
			name:       "AddedReason set with argOffset 0 numbers its placeholder $1 and compares as text",
			filters:    domain.PendingVerificationSearchFilters{AddedReason: &autoClosed},
			argOffset:  0,
			wantClause: " AND added_reason::TEXT = $1",
			wantArgs:   []any{string(autoClosed)},
		},
		{
			name:       "AddedReason set with a non-zero argOffset numbers its placeholder accordingly",
			filters:    domain.PendingVerificationSearchFilters{AddedReason: &autoClosed},
			argOffset:  3,
			wantClause: " AND added_reason::TEXT = $4",
			wantArgs:   []any{string(autoClosed)},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotClause, gotArgs := dedupeAddedReasonClause(tt.filters, tt.argOffset)
			if gotClause != tt.wantClause {
				t.Fatalf("clause: got %q, want %q", gotClause, tt.wantClause)
			}
			if len(gotArgs) != len(tt.wantArgs) {
				t.Fatalf("args: got %v, want %v", gotArgs, tt.wantArgs)
			}
			for i := range gotArgs {
				if gotArgs[i] != tt.wantArgs[i] {
					t.Fatalf("args[%d]: got %v, want %v", i, gotArgs[i], tt.wantArgs[i])
				}
			}
		})
	}
}
