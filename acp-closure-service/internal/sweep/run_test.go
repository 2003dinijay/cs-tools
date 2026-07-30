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

package sweep

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestRun_SinglePageEvaluatesAllProjects(t *testing.T) {
	reader := &mockEntityReader{
		searchProjectsFn: func(ctx context.Context, body []byte) ([]byte, error) {
			return []byte(`{
				"projects": [
					{"id": "p1", "endDate": null},
					{"id": "p2", "endDate": null}
				],
				"total": 2, "limit": 100, "offset": 0, "hasMore": false
			}`), nil
		},
	}
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{}

	result, err := Run(context.Background(), reader, updater, ntf, time.Now(), "")
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if result.ProjectsEvaluated != 2 {
		t.Errorf("ProjectsEvaluated = %d, want 2", result.ProjectsEvaluated)
	}
	if len(result.Failures) != 0 {
		t.Errorf("Failures = %d, want 0", len(result.Failures))
	}
	if len(reader.searchProjectsCalls) != 1 {
		t.Errorf("SearchProjects calls = %d, want 1", len(reader.searchProjectsCalls))
	}
}

// TestRun_MultiPagePaginatesUntilHasMoreFalse verifies offset increments by
// the page size across pages, and stops once hasMore is false.
func TestRun_MultiPagePaginatesUntilHasMoreFalse(t *testing.T) {
	var gotOffsets []int
	reader := &mockEntityReader{
		searchProjectsFn: func(ctx context.Context, body []byte) ([]byte, error) {
			var req searchProjectsRequest
			if err := json.Unmarshal(body, &req); err != nil {
				t.Fatalf("parse search request: %v", err)
			}
			gotOffsets = append(gotOffsets, req.Pagination.Offset)

			if req.Pagination.Offset == 0 {
				return []byte(`{
					"projects": [{"id": "p1", "endDate": null}],
					"total": 2, "limit": 100, "offset": 0, "hasMore": true
				}`), nil
			}
			return []byte(`{
				"projects": [{"id": "p2", "endDate": null}],
				"total": 2, "limit": 100, "offset": 100, "hasMore": false
			}`), nil
		},
	}
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{}

	result, err := Run(context.Background(), reader, updater, ntf, time.Now(), "")
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if result.ProjectsEvaluated != 2 {
		t.Errorf("ProjectsEvaluated = %d, want 2", result.ProjectsEvaluated)
	}
	if len(gotOffsets) != 2 || gotOffsets[0] != 0 || gotOffsets[1] != 100 {
		t.Errorf("offsets = %v, want [0 100]", gotOffsets)
	}
}

// TestRun_OneProjectFailureDoesNotBlockTheRest verifies the two-tier failure
// design: a single project's processProject failure (malformed
// suspensionProcessState, here) is recorded in Result.Failures and the
// sweep continues to the remaining projects in the same page — it must not
// abort the whole run.
func TestRun_OneProjectFailureDoesNotBlockTheRest(t *testing.T) {
	now := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	firingEndDate := now.AddDate(0, 0, 89).Format(time.RFC3339) // fires the 90-day window

	reader := &mockEntityReader{
		searchProjectsFn: func(ctx context.Context, body []byte) ([]byte, error) {
			return []byte(`{
				"projects": [
					{"id": "p1", "endDate": null},
					{"id": "p2", "endDate": "` + firingEndDate + `", "suspensionProcessState": "not-an-object"},
					{"id": "p3", "endDate": null}
				],
				"total": 3, "limit": 100, "offset": 0, "hasMore": false
			}`), nil
		},
	}
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{}

	result, err := Run(context.Background(), reader, updater, ntf, now, "")
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if result.ProjectsEvaluated != 3 {
		t.Errorf("ProjectsEvaluated = %d, want 3", result.ProjectsEvaluated)
	}
	if len(result.Failures) != 1 {
		t.Fatalf("Failures = %d, want 1", len(result.Failures))
	}
	if result.Failures[0].ProjectID != "p2" {
		t.Errorf("failed ProjectID = %q, want %q", result.Failures[0].ProjectID, "p2")
	}
}

// TestRun_PageFetchFailureIsFatal verifies the other tier: a failure
// fetching a page itself stops the run and returns a non-nil error, rather
// than being folded into Result.Failures like a per-project failure.
func TestRun_PageFetchFailureIsFatal(t *testing.T) {
	reader := &mockEntityReader{
		searchProjectsFn: func(ctx context.Context, body []byte) ([]byte, error) {
			return nil, errors.New("connection refused")
		},
	}
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{}

	_, err := Run(context.Background(), reader, updater, ntf, time.Now(), "")
	if err == nil {
		t.Fatal("Run() error = nil, want non-nil")
	}
}

// TestRun_ScopedToProjectIDFetchesOnlyThatProject verifies the
// TEST_PROJECT_ID scoping: when a non-empty projectID is passed, Run fetches
// that one project directly via GetProject and never calls the broad
// SearchProjects sweep at all — proving a scoped run can't accidentally
// touch every open project in the environment.
func TestRun_ScopedToProjectIDFetchesOnlyThatProject(t *testing.T) {
	const testProjectID = "e3e87599-1bc7-6650-182c-0dc5604bcb68"

	var gotID string
	reader := &mockEntityReader{
		getProjectFn: func(ctx context.Context, id string) ([]byte, error) {
			gotID = id
			return []byte(`{"id": "` + testProjectID + `", "account": {"id": "f213fdd1-1b4b-a650-a002-c9d3604bcbac"}, "endDate": null}`), nil
		},
		searchProjectsFn: func(ctx context.Context, body []byte) ([]byte, error) {
			t.Fatal("SearchProjects should not be called when scoped to a single project")
			return nil, nil
		},
	}
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{}

	result, err := Run(context.Background(), reader, updater, ntf, time.Now(), testProjectID)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if gotID != testProjectID {
		t.Errorf("GetProject called with id = %q, want %q", gotID, testProjectID)
	}
	if result.ProjectsEvaluated != 1 {
		t.Errorf("ProjectsEvaluated = %d, want 1", result.ProjectsEvaluated)
	}
	if len(reader.searchProjectsCalls) != 0 {
		t.Errorf("SearchProjects calls = %d, want 0", len(reader.searchProjectsCalls))
	}
}

// TestRun_ScopedProjectFetchFailureIsFatal mirrors
// TestRun_PageFetchFailureIsFatal for the scoped path: if GetProject itself
// fails, that's fatal for the run, not a per-project soft failure — there's
// nothing else to fall back to when the one requested project can't be
// fetched at all.
func TestRun_ScopedProjectFetchFailureIsFatal(t *testing.T) {
	reader := &mockEntityReader{
		getProjectFn: func(ctx context.Context, id string) ([]byte, error) {
			return nil, errors.New("not found")
		},
	}
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{}

	_, err := Run(context.Background(), reader, updater, ntf, time.Now(), "e3e87599-1bc7-6650-182c-0dc5604bcb68")
	if err == nil {
		t.Fatal("Run() error = nil, want non-nil")
	}
}
