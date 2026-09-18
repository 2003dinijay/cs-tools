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
	"errors"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
)

// ServiceNow's conversation-state choice list carries "Open" as id 1, which this
// backend has never mapped. Skipping it left States empty, and an empty States
// is "no state filter" to entity-service rather than "no matches" — so filtering
// by Open returned every conversation in the project (1072 of them on staging)
// while looking like a working filter.

// TestBuildEntitySearchConversationsRequest_RejectsUnmappedState is the
// regression: id 1 must be refused, never dropped.
func TestBuildEntitySearchConversationsRequest_RejectsUnmappedState(t *testing.T) {
	_, err := BuildEntitySearchConversationsRequest("p-1", ConversationSearchRequest{
		Filters: ConversationSearchFilters{StateKeys: []int{1}},
	})
	if err == nil {
		t.Fatal("expected an error for the unmapped Open state, got nil")
	}
	if !errors.Is(err, ErrUnsupportedConversationState) {
		t.Fatalf("error = %v, want ErrUnsupportedConversationState", err)
	}
}

// A state the search cannot apply must not widen the result set, whatever its
// value — the Open id is the one users hit, but any unknown id did the same.
func TestBuildEntitySearchConversationsRequest_RejectsArbitraryUnknownState(t *testing.T) {
	_, err := BuildEntitySearchConversationsRequest("p-1", ConversationSearchRequest{
		Filters: ConversationSearchFilters{StateKeys: []int{99}},
	})
	if !errors.Is(err, ErrUnsupportedConversationState) {
		t.Fatalf("error = %v, want ErrUnsupportedConversationState", err)
	}
}

// One bad id among good ones must still fail: a partial translation would
// silently broaden the filter to the states it happened to understand.
func TestBuildEntitySearchConversationsRequest_RejectsPartiallyMappedStates(t *testing.T) {
	_, err := BuildEntitySearchConversationsRequest("p-1", ConversationSearchRequest{
		Filters: ConversationSearchFilters{StateKeys: []int{2, 1}},
	})
	if !errors.Is(err, ErrUnsupportedConversationState) {
		t.Fatalf("error = %v, want ErrUnsupportedConversationState", err)
	}
}

// The mapped states still work, and still translate to entity-service's enum
// vocabulary rather than the numeric ids the frontend sends.
func TestBuildEntitySearchConversationsRequest_MapsSupportedStates(t *testing.T) {
	got, err := BuildEntitySearchConversationsRequest("p-1", ConversationSearchRequest{
		Filters: ConversationSearchFilters{StateKeys: []int{2, 3}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Filters.States) != 2 || got.Filters.States[0] != "ACTIVE" || got.Filters.States[1] != "RESOLVED" {
		t.Fatalf("states = %v, want [ACTIVE RESOLVED]", got.Filters.States)
	}
	if len(got.Filters.ProjectIDs) != 1 || got.Filters.ProjectIDs[0] != "p-1" {
		t.Errorf("project scope must come from the path: %v", got.Filters.ProjectIDs)
	}
}

// No state filter at all is a legitimate request — "show me everything" is fine
// when the caller actually asked for everything.
func TestBuildEntitySearchConversationsRequest_NoStateFilterIsAllowed(t *testing.T) {
	got, err := BuildEntitySearchConversationsRequest("p-1", ConversationSearchRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Filters.States) != 0 {
		t.Fatalf("states = %v, want empty", got.Filters.States)
	}
}

// TestMapSupportedConversationStates_DropsUnsupported keeps the dropdown honest:
// a state the search cannot apply must not be offered as an option.
func TestMapSupportedConversationStates_DropsUnsupported(t *testing.T) {
	// Exactly what /projects/{id}/filters returns upstream, Open included.
	in := []entity.ChoiceListItem{
		{ID: "6", Label: "Close"},
		{ID: "5", Label: "Abandoned"},
		{ID: "4", Label: "Converted"},
		{ID: "1", Label: "Open"},
		{ID: "3", Label: "Resolved"},
		{ID: "2", Label: "Active"},
	}

	got := mapSupportedConversationStates(in)

	if len(got) != 5 {
		t.Fatalf("got %d states, want 5 (Open dropped): %+v", len(got), got)
	}
	for _, s := range got {
		if s.ID == "1" {
			t.Fatalf("Open (id 1) must not be offered — the search cannot apply it: %+v", got)
		}
	}
	// And the supported ones survive untouched, in order.
	wantIDs := []string{"6", "5", "4", "3", "2"}
	for i, want := range wantIDs {
		if got[i].ID != want {
			t.Errorf("state[%d].ID = %q, want %q", i, got[i].ID, want)
		}
	}
}

// Every state the dropdown offers must be one the search accepts. This is the
// invariant that ties the two halves of the fix together: if a future choice
// list gains a state, this fails rather than quietly returning everything.
func TestMapSupportedConversationStates_EveryOfferedStateIsFilterable(t *testing.T) {
	in := []entity.ChoiceListItem{
		{ID: "1", Label: "Open"},
		{ID: "2", Label: "Active"},
		{ID: "3", Label: "Resolved"},
		{ID: "4", Label: "Converted"},
		{ID: "5", Label: "Abandoned"},
		{ID: "6", Label: "Close"},
		{ID: "77", Label: "Some Future State"},
	}

	for _, s := range mapSupportedConversationStates(in) {
		id := 0
		for _, c := range s.ID {
			id = id*10 + int(c-'0')
		}
		if _, err := BuildEntitySearchConversationsRequest("p-1", ConversationSearchRequest{
			Filters: ConversationSearchFilters{StateKeys: []int{id}},
		}); err != nil {
			t.Errorf("state %q (%s) is offered but the search rejects it: %v", s.ID, s.Label, err)
		}
	}
}
