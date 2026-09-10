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
	"encoding/json"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
)

func f64(v float64) *float64 { return &v }
func strp(v string) *string  { return &v }

// TestMapProjectDetails_EmitsFrontendKeyNames is the key-name assertion for this
// change. The whole class of bug being closed here is a field that exists on both
// sides but under a name the frontend never reads — productCount vs
// deployedProductCount is what blanked the Usage & Metrics page. Asserting the
// emitted JSON keys is what catches that at build time.
func TestMapProjectDetails_EmitsFrontendKeyNames(t *testing.T) {
	goLive := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	view := entity.ProjectDetailsView{
		ID:                       "6fa0b42d-1bfa-a694-a002-c9d3604bcb77",
		TotalQueryHours:          f64(40),
		ConsumedQueryHours:       f64(12.5),
		RemainingQueryHours:      f64(27.5),
		TotalOnboardingHours:     f64(80),
		ConsumedOnboardingHours:  f64(80),
		RemainingOnboardingHours: f64(0),
		GoLiveDate:               &goLive,
		OnboardingStatus:         strp("Completed"),
		Account: entity.ProjectAccountRef{
			ID:                  "aa11bb22-cc33-dd44-ee55-ff6677889900",
			OwnerEmail:          strp("owner@acme.invalid"),
			TechnicalOwnerEmail: strp("tech@acme.invalid"),
		},
	}

	b, err := json.Marshal(MapProjectDetails(view))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	for key, want := range map[string]float64{
		"totalQueryHours":         40,
		"consumedQueryHours":      12.5,
		"remainingQueryHours":     27.5,
		"totalOnboardingHours":    80,
		"consumedOnboardingHours": 80,
	} {
		got, ok := out[key].(float64)
		if !ok {
			t.Errorf("%s missing from the payload; the frontend reads this exact key", key)
			continue
		}
		if got != want {
			t.Errorf("%s = %v, want %v", key, got, want)
		}
	}
	if out["onboardingStatus"] != "Completed" {
		t.Errorf("onboardingStatus = %v, want Completed", out["onboardingStatus"])
	}
	if out["goLiveDate"] == nil {
		t.Error("goLiveDate missing from the payload")
	}

	// The contacts must land on the nested account, not the project root —
	// that is where the frontend's ProjectDetailsAccount reads them.
	acct, ok := out["account"].(map[string]any)
	if !ok {
		t.Fatal("account object missing")
	}
	if acct["ownerEmail"] != "owner@acme.invalid" {
		t.Errorf("account.ownerEmail = %v", acct["ownerEmail"])
	}
	if acct["technicalOwnerEmail"] != "tech@acme.invalid" {
		t.Errorf("account.technicalOwnerEmail = %v", acct["technicalOwnerEmail"])
	}
	if _, atRoot := out["ownerEmail"]; atRoot {
		t.Error("ownerEmail is at the project root; the frontend reads it on account")
	}
}

// TestMapProjectDetails_ZeroBalanceSurvives pins the reason every hour field is a
// pointer: omitempty omits only nil, so a real zero must still serialise.
// "Tracked, none remaining" and "not tracked at all" are different facts, and
// collapsing them would show a customer 0 hours they do not actually have.
func TestMapProjectDetails_ZeroBalanceSurvives(t *testing.T) {
	b, err := json.Marshal(MapProjectDetails(entity.ProjectDetailsView{
		RemainingQueryHours: f64(0),
	}))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	got, present := out["remainingQueryHours"]
	if !present {
		t.Fatal("a zero remainingQueryHours was omitted; it must be sent as 0")
	}
	if got.(float64) != 0 {
		t.Errorf("remainingQueryHours = %v, want 0", got)
	}
}

// TestMapProjectDetails_UntrackedFieldsAreOmitted is the other half: a project
// with none of this tracked must not invent zeroes.
func TestMapProjectDetails_UntrackedFieldsAreOmitted(t *testing.T) {
	b, err := json.Marshal(MapProjectDetails(entity.ProjectDetailsView{}))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, key := range []string{
		"totalQueryHours", "consumedQueryHours", "remainingQueryHours",
		"totalOnboardingHours", "goLiveDate", "onboardingStatus",
	} {
		if _, present := out[key]; present {
			t.Errorf("%s present for an untracked project; want it omitted", key)
		}
	}
}

// TestMapSearchProjects_ActiveCasesCountAlwaysPresent covers the search item:
// the frontend's ProjectListItem types activeCasesCount as a required number, so
// it must be sent even when zero — hence no omitempty.
func TestMapSearchProjects_ActiveCasesCountAlwaysPresent(t *testing.T) {
	resp := MapSearchProjects(entity.SearchProjectsResponse{
		Projects: []entity.ProjectView{
			{ID: "a", ActiveCasesCount: 7},
			{ID: "b", ActiveCasesCount: 0},
		},
	})
	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out struct {
		Projects []map[string]any `json:"projects"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out.Projects) != 2 {
		t.Fatalf("got %d projects, want 2", len(out.Projects))
	}
	if got := out.Projects[0]["activeCasesCount"]; got != float64(7) {
		t.Errorf("first activeCasesCount = %v, want 7", got)
	}
	got, present := out.Projects[1]["activeCasesCount"]
	if !present {
		t.Fatal("a zero activeCasesCount was omitted; the frontend types it as required")
	}
	if got != float64(0) {
		t.Errorf("second activeCasesCount = %v, want 0", got)
	}
}
