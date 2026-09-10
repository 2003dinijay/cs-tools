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

package dto

import (
	"reflect"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
)

// TestBuildEntityUpdateCallRequestRequest_StateKeyTranslatesToEnum guards
// against reintroducing the bug where this backend expected a "state" string
// enum field the frontend never sends — the frontend (built against the old
// Ballerina backend) sends stateKey as a ServiceNow numeric choice-list key.
func TestBuildEntityUpdateCallRequestRequest_StateKeyTranslatesToEnum(t *testing.T) {
	got := BuildEntityUpdateCallRequestRequest(CallRequestUpdateRequest{StateKey: 6})

	if got.State != "canceled" {
		t.Fatalf("State = %q, want %q", got.State, "canceled")
	}
}

// TestBuildEntityUpdateCallRequestRequest_UnrecognizedStateKeyProducesEmptyState
// verifies an unrecognized (or absent, zero-value) stateKey translates to an
// empty State rather than panicking or forwarding the raw number —
// entity-service's own validCallRequestStates check then rejects an empty
// state with 400.
func TestBuildEntityUpdateCallRequestRequest_UnrecognizedStateKeyProducesEmptyState(t *testing.T) {
	got := BuildEntityUpdateCallRequestRequest(CallRequestUpdateRequest{StateKey: 999})

	if got.State != "" {
		t.Fatalf("State = %q, want empty string for an unrecognized stateKey", got.State)
	}
}

// TestBuildEntitySearchCallRequestsRequest_CaseIDFromPathAndStateKeysTranslated
// verifies caseID always comes from the path parameter (never the body,
// which the frontend never sends one in) and that filters.stateKeys
// translates to entity-service's own states string enum.
func TestBuildEntitySearchCallRequestsRequest_CaseIDFromPathAndStateKeysTranslated(t *testing.T) {
	req := CallRequestSearchRequest{
		Filters:    CallRequestSearchFilters{StateKeys: []int{3, 8, 999}},
		Pagination: entity.Pagination{Limit: 10, Offset: 0},
	}

	got := BuildEntitySearchCallRequestsRequest("case-1", req)

	if got.CaseID != "case-1" {
		t.Fatalf("CaseID = %q, want %q", got.CaseID, "case-1")
	}
	if got.Filters == nil {
		t.Fatal("expected non-nil Filters")
	}
	want := []string{"scheduled", "concluded"}
	if !reflect.DeepEqual(got.Filters.States, want) {
		t.Fatalf("States = %+v, want %+v (999 has no mapping and must be dropped)", got.Filters.States, want)
	}
	if got.Pagination != req.Pagination {
		t.Fatalf("Pagination = %+v, want %+v", got.Pagination, req.Pagination)
	}
}

// TestBuildEntitySearchCallRequestsRequest_NoStateKeysLeavesFiltersNil
// verifies an empty/absent stateKeys filter produces a nil Filters, matching
// entity.SearchCallRequestsRequest.Filters's omitempty/optional contract,
// rather than an empty-but-present filters object.
func TestBuildEntitySearchCallRequestsRequest_NoStateKeysLeavesFiltersNil(t *testing.T) {
	got := BuildEntitySearchCallRequestsRequest("case-1", CallRequestSearchRequest{})

	if got.Filters != nil {
		t.Fatalf("Filters = %+v, want nil", got.Filters)
	}
}
