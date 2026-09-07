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

// TestDateOnly_RendersBareCalendarDates covers the helper directly, including the
// zero-time case: a fabricated "0001-01-01" is worse than an absent field.
func TestDateOnly_RendersBareCalendarDates(t *testing.T) {
	ts := time.Date(2026, 6, 16, 13, 45, 7, 0, time.UTC)
	if got := DateOnly(&ts); got == nil || *got != "2026-06-16" {
		t.Errorf("DateOnly = %v, want 2026-06-16 (time component dropped)", got)
	}
	if got := DateOnly(nil); got != nil {
		t.Errorf("DateOnly(nil) = %v, want nil", got)
	}
	var zero time.Time
	if got := DateOnly(&zero); got != nil {
		t.Errorf("DateOnly(zero) = %v, want nil rather than a fabricated date", got)
	}
	if got := DateOnlyValue(zero); got != nil {
		t.Errorf("DateOnlyValue(zero) = %v, want nil", got)
	}
}

// TestProjectDates_AreDateOnlyNotTimestamps is the regression guard for the
// time-tracking filter failure.
//
// The time-tracking page defaults its startDate filter to project.startDate
// verbatim (ProjectTimeTracking.tsx: setStartDate(project.startDate)). While this
// backend emitted an RFC3339 timestamp, that produced
// startDate=2026-06-16T00:00:00Z, which the upstream rejects on its own date
// pattern — while the locally-formatted endDate beside it was accepted. Ballerina
// types every one of these as DateString, so date-only is the contract.
func TestProjectDates_AreDateOnlyNotTimestamps(t *testing.T) {
	d := func(y int, m time.Month, day int) *time.Time {
		v := time.Date(y, m, day, 0, 0, 0, 0, time.UTC)
		return &v
	}

	b, err := json.Marshal(MapProjectDetails(entity.ProjectDetailsView{
		ID:        "6fa0b42d-1bfa-a694-a002-c9d3604bcb77",
		StartDate: *d(2026, 6, 16),
		EndDate:   *d(2027, 6, 15),
	}))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for key, want := range map[string]string{
		"startDate": "2026-06-16",
		"endDate":   "2027-06-15",
	} {
		got, _ := out[key].(string)
		if got != want {
			t.Errorf("%s = %q, want %q — a timestamp here breaks the time-tracking filter", key, got, want)
		}
	}
}

// TestSearchProjects_DatesAreDateOnly covers the search item, which the page's
// project lookup also reads.
func TestSearchProjects_DatesAreDateOnly(t *testing.T) {
	start := time.Date(2026, 6, 16, 9, 30, 0, 0, time.UTC)
	end := time.Date(2027, 1, 5, 0, 0, 0, 0, time.UTC)
	b, err := json.Marshal(MapSearchProjects(entity.SearchProjectsResponse{
		Projects: []entity.ProjectView{{ID: "a", StartDate: &start, EndDate: &end}},
	}))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out struct {
		Projects []map[string]any `json:"projects"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got, _ := out.Projects[0]["startDate"].(string); got != "2026-06-16" {
		t.Errorf("startDate = %q, want 2026-06-16", got)
	}
	if got, _ := out.Projects[0]["endDate"].(string); got != "2027-01-05" {
		t.Errorf("endDate = %q, want 2027-01-05", got)
	}
}

// TestProjectDates_RoundTripThroughTheDateFilterValidator ties the two halves
// together: whatever this backend emits for a project date must be accepted by
// its own date-filter validation, because the frontend feeds one straight into
// the other.
func TestProjectDates_RoundTripThroughTheDateFilterValidator(t *testing.T) {
	ts := time.Date(2026, 6, 16, 23, 59, 59, 0, time.UTC)
	emitted := DateOnly(&ts)
	if emitted == nil {
		t.Fatal("DateOnly returned nil for a valid timestamp")
	}
	if !IsValidDateOnly(*emitted) {
		t.Errorf("this backend emits %q for a project date, but its own filter validation rejects it", *emitted)
	}
}
