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
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
)

// The dashboard's outstanding-engagements chart matches engagement type
// buckets on the LABEL, against "Consultancy"/"Onboarding"/"Migration"/
// "Follow Up"/"New Feature Improvement" (features/dashboard/constants/dashboard.ts,
// OUTSTANDING_ENGAGEMENTS_CATEGORY_CHART_DATA). The Postgres data source
// returns the raw enum label (UPPER_SNAKE) as both id and label -- without
// normalization every bucket misses that match and silently drops out of
// both the chart and its total, the same class of bug
// TestNormalizeCaseSeverityChoices_PostgresEnumLabels guards for severity.
func TestMapProjectCaseStats_NormalizesEngagementTypeChoices(t *testing.T) {
	resp := entity.ProjectCaseStatsResponse{
		EngagementTypeCount: []entity.ChoiceListItem{
			{ID: "NEW_FEATURE_IMPROVEMENT", Label: "NEW_FEATURE_IMPROVEMENT", Count: intPtr(2)},
		},
		OutstandingEngagementTypeCount: []entity.ChoiceListItem{
			{ID: "FOLLOW_UP", Label: "FOLLOW_UP", Count: intPtr(3)},
		},
	}

	got := MapProjectCaseStats(resp)

	if len(got.EngagementTypeCount) != 1 || got.EngagementTypeCount[0].Label != "New Feature Improvement" {
		t.Fatalf("EngagementTypeCount not normalized: %+v", got.EngagementTypeCount)
	}
	if len(got.OutstandingEngagementTypeCount) != 1 || got.OutstandingEngagementTypeCount[0].Label != "Follow Up" {
		t.Fatalf("OutstandingEngagementTypeCount not normalized: %+v", got.OutstandingEngagementTypeCount)
	}
}

// GET /projects/{id}/filters' changeRequestStates/changeRequestImpacts fed
// ChangeRequestsPage.tsx's State/Impact filter dropdowns directly. On the
// Postgres data source these carried the raw enum label as id
// (e.g. {"id":"ROLLBACK"}), never run through the normalizer every sibling
// field on this same response already uses -- so filters.stateIds?.map(Number)
// converted every selection to NaN, which reached the search request as
// null instead of a real state key. Also verifies New and Assess are excluded
// from the list (no change request a customer can see is ever in either) while
// Authorize is offered: a change request the customer proposed a new time for
// waits there, so the filter has to be able to name it.
func TestMapProjectFilterOptions_NormalizesChangeRequestChoicesAndOffersAuthorize(t *testing.T) {
	resp := entity.ProjectMetadataResponse{
		ChangeRequestStates: []entity.ChoiceListItem{
			{ID: "NEW", Label: "NEW"},
			{ID: "ASSESS", Label: "ASSESS"},
			{ID: "AUTHORIZE", Label: "AUTHORIZE"},
			{ID: "ROLLBACK", Label: "ROLLBACK"},
			{ID: "CLOSED", Label: "CLOSED"},
		},
		ChangeRequestImpacts: []entity.ChoiceListItem{
			{ID: "HIGH", Label: "HIGH"},
		},
	}

	got := MapProjectFilterOptions(resp)

	want := []struct{ id, label string }{{"-3", "Authorize"}, {"2", "Rollback"}, {"3", "Closed"}}
	if len(got.ChangeRequestStates) != len(want) {
		t.Fatalf("ChangeRequestStates = %+v, want Authorize, Rollback and Closed (New/Assess excluded)", got.ChangeRequestStates)
	}
	for i, w := range want {
		if got.ChangeRequestStates[i].ID != w.id || got.ChangeRequestStates[i].Label != w.label {
			t.Errorf("ChangeRequestStates[%d] = %+v, want {ID: %q, Label: %q}", i, got.ChangeRequestStates[i], w.id, w.label)
		}
	}
	if len(got.ChangeRequestImpacts) != 1 || got.ChangeRequestImpacts[0].ID != "1" || got.ChangeRequestImpacts[0].Label != "High" {
		t.Errorf("ChangeRequestImpacts = %+v, want [{ID: \"1\", Label: \"High\"}]", got.ChangeRequestImpacts)
	}
}

// GET /projects/{id}/filters' conversationStates fed AllConversationsPage.tsx's
// State filter dropdown directly. On the Postgres data source these carried
// the raw enum label as id (e.g. {"id":"ACTIVE"}, {"id":"CLOSE"} -- the real
// Postgres label for the closed state has no D), never run through the
// normalizer every sibling choice list on this same response already uses --
// so filters.stateId ? [Number(filters.stateId)] : undefined converted every
// selection to NaN, which conversationIDsToEnums then refused as an unmapped
// state id. The page never recovered: its own loading flag only clears once
// the search query has a successful response, so every selection left it
// stuck showing its loader (digiops-cs#3273).
func TestMapProjectFilterOptions_NormalizesConversationStateChoices(t *testing.T) {
	resp := entity.ProjectMetadataResponse{
		ConversationStates: []entity.ChoiceListItem{
			{ID: "OPEN", Label: "OPEN"},
			{ID: "ACTIVE", Label: "ACTIVE"},
			{ID: "RESOLVED", Label: "RESOLVED"},
			{ID: "CONVERTED", Label: "CONVERTED"},
			{ID: "ABANDONED", Label: "ABANDONED"},
			{ID: "CLOSE", Label: "CLOSE"},
		},
	}

	got := MapProjectFilterOptions(resp)

	want := []ReferenceItem{
		{ID: "1", Label: "Open"},
		{ID: "2", Label: "Active"},
		{ID: "3", Label: "Resolved"},
		{ID: "4", Label: "Converted"},
		{ID: "5", Label: "Abandoned"},
		{ID: "6", Label: "Closed"},
	}
	if len(got.ConversationStates) != len(want) {
		t.Fatalf("ConversationStates = %+v, want %+v", got.ConversationStates, want)
	}
	for i, w := range want {
		if got.ConversationStates[i].ID != w.ID || got.ConversationStates[i].Label != w.Label {
			t.Errorf("ConversationStates[%d] = %+v, want %+v", i, got.ConversationStates[i], w)
		}
	}
}

// An id this normalizer does not recognise (ServiceNow's own numeric choice
// key, already what the frontend wants) must pass through unchanged rather
// than being dropped or rewritten.
func TestMapProjectFilterOptions_ConversationStateUnrecognisedIDPassesThrough(t *testing.T) {
	resp := entity.ProjectMetadataResponse{
		ConversationStates: []entity.ChoiceListItem{
			{ID: "7", Label: "Some Future State"},
		},
	}

	got := MapProjectFilterOptions(resp)

	if len(got.ConversationStates) != 1 || got.ConversationStates[0].ID != "7" || got.ConversationStates[0].Label != "Some Future State" {
		t.Errorf("ConversationStates = %+v, want [{ID: \"7\", Label: \"Some Future State\"}] unchanged", got.ConversationStates)
	}
}

// The exclusion must catch New and Assess under either data source's own shape:
// ServiceNow's real numeric ids ("-5", "-4") with a display-cased label, and
// Postgres's raw UPPER_SNAKE label with no matching id. Missing either would let
// that one data source's state leak into the customer-facing filter panel. And
// Authorize survives under both shapes ("-3" and "AUTHORIZE").
func TestMapProjectFilterOptions_ExcludesNewAndAssessByIDOrLabelAndKeepsAuthorize(t *testing.T) {
	resp := entity.ProjectMetadataResponse{
		ChangeRequestStates: []entity.ChoiceListItem{
			{ID: "-5", Label: "New"},              // ServiceNow: real numeric id, display-cased label
			{ID: "-4", Label: "Assess"},           // ServiceNow
			{ID: "-3", Label: "Authorize"},        // ServiceNow: kept
			{ID: "NEW", Label: "NEW"},             // Postgres: no id, UPPER_SNAKE label
			{ID: "ASSESS", Label: "ASSESS"},       // Postgres
			{ID: "AUTHORIZE", Label: "AUTHORIZE"}, // Postgres: kept
			{ID: "CLOSED", Label: "CLOSED"},
		},
	}

	got := MapProjectFilterOptions(resp)

	var labels []string
	for _, s := range got.ChangeRequestStates {
		labels = append(labels, s.Label)
	}
	if strings.Join(labels, ",") != "Authorize,Authorize,Closed" {
		t.Fatalf("ChangeRequestStates labels = %v, want [Authorize Authorize Closed] (one Authorize per data source's shape)", labels)
	}
}

// TestMapProjectFilterOptions_ExposesResolutionCodesAndCauses is the
// regression test for a real, reported bug: closing a case requires
// resolutionCode/cause/closeNotes, but the webapp had no choice lists to
// build a close dialog from at all -- GET /projects/{id}/filters simply
// never carried either field. Confirms both now pass through unchanged.
func TestMapProjectFilterOptions_ExposesResolutionCodesAndCauses(t *testing.T) {
	resp := entity.ProjectMetadataResponse{
		ResolutionCodes: []entity.ChoiceListItem{{ID: "SOLVED_WORKAROUND_PROVIDED", Label: "Solved Workaround Provided"}},
		Causes:          []entity.ChoiceListItem{{ID: "PRODUCT_BUG", Label: "Product Bug"}},
	}

	got := MapProjectFilterOptions(resp)

	if len(got.ResolutionCodes) != 1 || got.ResolutionCodes[0].ID != "SOLVED_WORKAROUND_PROVIDED" {
		t.Fatalf("ResolutionCodes = %+v", got.ResolutionCodes)
	}
	if len(got.Causes) != 1 || got.Causes[0].ID != "PRODUCT_BUG" {
		t.Fatalf("Causes = %+v", got.Causes)
	}
}

// GET /projects/{id}/stats/change-requests feeds the Operations page's
// Upcoming Changes / Action Required Changes cards, which find their counts by
// display label ("Scheduled", "Customer Approval", "Customer Review"). The
// Postgres data source returns the raw enum as both id and label, so without
// normalization "SCHEDULED" never matched and the card showed "--" while the
// change-request list beside it showed four Scheduled changes.
func TestMapProjectChangeRequestStats_NormalizesStateCountLabels(t *testing.T) {
	four, zero := 4, 0
	resp := entity.ProjectChangeRequestStatsResponse{
		TotalCount: 134,
		StateCount: []entity.ChoiceListItem{
			{ID: "CUSTOMER_APPROVAL", Label: "CUSTOMER_APPROVAL", Count: &zero},
			{ID: "SCHEDULED", Label: "SCHEDULED", Count: &four},
			{ID: "CUSTOMER_REVIEW", Label: "CUSTOMER_REVIEW", Count: &zero},
		},
	}

	got := MapProjectChangeRequestStats(resp)

	want := []struct{ id, label string }{
		{"5", "Customer Approval"},
		{"-2", "Scheduled"},
		{"1", "Customer Review"},
	}
	if len(got.StateCount) != len(want) {
		t.Fatalf("StateCount = %+v, want %d entries", got.StateCount, len(want))
	}
	for i, w := range want {
		if got.StateCount[i].ID != w.id || got.StateCount[i].Label != w.label {
			t.Errorf("StateCount[%d] = {%q, %q}, want {%q, %q}",
				i, got.StateCount[i].ID, got.StateCount[i].Label, w.id, w.label)
		}
	}
	if got.StateCount[1].Count == nil || *got.StateCount[1].Count != 4 {
		t.Errorf("Scheduled count = %v, want 4 (counts must survive normalization)", got.StateCount[1].Count)
	}
}

// A change request the customer proposed a new time for waits in Authorize, so the
// stats must be able to count it under a state the webapp recognises:
// {id: "-3", label: "Authorize"} (the webapp files that id under "Ongoing"). New
// and Assess are two rows of 0 under raw ids no screen names -- no change request a
// customer can see is ever in either -- so they are not sent. The totals pass
// through as entity-service computed them.
func TestMapProjectChangeRequestStats_CountsAuthorizeAndDropsNewAndAssess(t *testing.T) {
	one, zero := 1, 0
	resp := entity.ProjectChangeRequestStatsResponse{
		TotalCount:       3,
		OutstandingCount: 2,
		StateCount: []entity.ChoiceListItem{
			{ID: "NEW", Label: "NEW", Count: &zero},
			{ID: "ASSESS", Label: "ASSESS", Count: &zero},
			{ID: "AUTHORIZE", Label: "AUTHORIZE", Count: &one},
			{ID: "CUSTOMER_APPROVAL", Label: "CUSTOMER_APPROVAL", Count: &one},
		},
	}

	got := MapProjectChangeRequestStats(resp)

	var rows []string
	for _, s := range got.StateCount {
		c := -1
		if s.Count != nil {
			c = *s.Count
		}
		rows = append(rows, s.ID+"|"+s.Label+"|"+strings.Repeat("#", c))
	}
	if strings.Join(rows, ",") != "-3|Authorize|#,5|Customer Approval|#" {
		t.Errorf("StateCount rows = %v, want Authorize and Customer Approval only", rows)
	}
	if got.TotalCount != 3 || got.OutstandingCount != 2 {
		t.Errorf("totals = %d / %d, want them passed through (3 / 2)", got.TotalCount, got.OutstandingCount)
	}
}
