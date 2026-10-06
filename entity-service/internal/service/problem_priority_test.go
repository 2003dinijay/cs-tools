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
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// ServiceNow returns a new problem's priority as its display value; every
// new problem there is "5 - Planning" (discovery script 63), which is also
// the fallback for a missing or unrecognised value.
func TestProblemPriorityFromServiceNow(t *testing.T) {
	sp := func(s string) *string { return &s }
	for in, want := range map[*string]string{
		sp("1 - Critical"): "CRITICAL",
		sp("2 - High"):     "HIGH",
		sp("3 - Moderate"): "MODERATE",
		sp("4 - Low"):      "LOW",
		sp("5 - Planning"): "PLANNING",
		sp("3"):            "MODERATE",
		sp(" 2 - High "):   "HIGH",
		sp(""):             "PLANNING",
		sp("Urgent"):       "PLANNING",
		nil:                "PLANNING",
	} {
		label := "<nil>"
		if in != nil {
			label = *in
		}
		if got := problemPriorityFromServiceNow(in); got != want {
			t.Errorf("%q -> %q, want %q", label, got, want)
		}
	}
}

// Dual-write: the Postgres copy gets the priority ServiceNow gave the
// problem, not a blank.
func TestCreateProblem_DualWriteStoresServiceNowsPriority(t *testing.T) {
	id, number, prio := "11111111-2222-4333-8444-555555555555", "PRB0040215", "5 - Planning"
	mirror := &stubMirrorProblemService{
		createProblem: func(context.Context, domain.CreateProblemRequest) (domain.ProblemDetail, error) {
			return domain.ProblemDetail{ID: &id, Number: &number, Priority: &prio}, nil
		},
	}
	repo := &stubProblemRepo{
		createProblemFromServiceNow: func(_ context.Context, _ domain.CreateProblemRequest, id, number, _ string, _ *string) (domain.ProblemDetail, error) {
			return domain.ProblemDetail{ID: &id, Number: &number}, nil
		},
	}
	svc := NewProblemServiceWithSNMirror(repo, mirror, nil)
	if _, err := svc.CreateProblem(userCtxProblem(t), validCreateProblemRequest()); err != nil {
		t.Fatalf("CreateProblem: %v", err)
	}
	if repo.lastCreatePriority != "PLANNING" {
		t.Errorf("Postgres got priority %q, want PLANNING", repo.lastCreatePriority)
	}
}
