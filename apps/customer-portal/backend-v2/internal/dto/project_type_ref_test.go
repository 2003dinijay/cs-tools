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

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
)

// TestProjectTypeRef_LabelsMatchTheFrontendEnum pins every label byte-for-byte
// against src/types/permission.ts's ProjectType enum.
//
// These are compared for equality in the frontend and drive behaviour, not just
// display: Development Support locks severity to S4, Cloud Support and Cloud
// Evaluation Support auto-pick the primary product, and anything other than
// Managed Cloud Subscription has S0 filtered out of the severity options. A
// wrong label is worse than a missing one — it silently changes which severities
// a customer can choose.
func TestProjectTypeRef_LabelsMatchTheFrontendEnum(t *testing.T) {
	for enum, want := range map[string]string{
		"managed_cloud_subscription": "Managed Cloud Subscription",
		"cloud_support":              "Cloud Support",
		"cloud_evaluation_support":   "Cloud Evaluation Support",
		"evaluation_subscription":    "Evaluation Subscription",
		"subscription":               "Subscription",
		"development_support":        "Development Support",
		"professional_services":      "Professional Services",
	} {
		got := projectTypeRef(enum)
		if got == nil {
			t.Errorf("projectTypeRef(%q) = nil, want a ref", enum)
			continue
		}
		if got.Label != want {
			t.Errorf("projectTypeRef(%q).Label = %q, want %q (must match ProjectType in permission.ts exactly)", enum, got.Label, want)
		}
		if got.ID == "" {
			t.Errorf("projectTypeRef(%q).ID is empty", enum)
		}
	}
}

// TestProjectTypeRef_EdgeCases covers an absent type and an unrecognised one. An
// unknown enum passes through as its own label rather than blanking the field,
// matching the tolerance the case *Ref helpers apply.
func TestProjectTypeRef_EdgeCases(t *testing.T) {
	if got := projectTypeRef(""); got != nil {
		t.Errorf("projectTypeRef(\"\") = %+v, want nil so the field is omitted", got)
	}
	got := projectTypeRef("some_future_type")
	if got == nil || got.Label != "some_future_type" {
		t.Errorf("unknown enum: got %+v, want the value passed through as its own label", got)
	}
}

// TestProjectDetails_EmitsTypeObject is the regression guard for the missing
// Operations nav item. SideBar.tsx computes
// `selectedProject?.type?.label ?? projectDetails?.type?.label` and filters the
// Operations item out when that resolves to nothing, so the object shape — not
// just the value — is what matters.
func TestProjectDetails_EmitsTypeObject(t *testing.T) {
	b, err := json.Marshal(MapProjectDetails(entity.ProjectDetailsView{
		ID:               "6fa0b42d-1bfa-a694-a002-c9d3604bcb77",
		SubscriptionType: "managed_cloud_subscription",
	}))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out struct {
		Type *struct {
			ID    string `json:"id"`
			Label string `json:"label"`
		} `json:"type"`
		SubscriptionType string `json:"subscriptionType"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Type == nil {
		t.Fatal("type is absent; the frontend reads project.type.label and hides the Operations nav without it")
	}
	if out.Type.Label != "Managed Cloud Subscription" {
		t.Errorf("type.label = %q, want %q", out.Type.Label, "Managed Cloud Subscription")
	}
	// The existing key stays, so nothing already reading it breaks.
	if out.SubscriptionType != "managed_cloud_subscription" {
		t.Errorf("subscriptionType = %q, want it preserved alongside type", out.SubscriptionType)
	}
}

// TestSearchProjects_EmitsTypeObject covers the search item, which SideBar reads
// first (selectedProject) and only falls back to the detail if it is absent.
func TestSearchProjects_EmitsTypeObject(t *testing.T) {
	b, err := json.Marshal(MapSearchProjects(entity.SearchProjectsResponse{
		Projects: []entity.ProjectView{
			{ID: "a", SubscriptionType: "development_support"},
			{ID: "b", SubscriptionType: ""},
		},
	}))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out struct {
		Projects []struct {
			Type *struct {
				Label string `json:"label"`
			} `json:"type"`
		} `json:"projects"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out.Projects) != 2 {
		t.Fatalf("got %d projects, want 2", len(out.Projects))
	}
	if out.Projects[0].Type == nil || out.Projects[0].Type.Label != "Development Support" {
		t.Errorf("first project type = %+v, want Development Support", out.Projects[0].Type)
	}
	// An empty upstream type omits the object rather than sending an empty one,
	// so the frontend's ?? fallback to the detail still works.
	if out.Projects[1].Type != nil {
		t.Errorf("second project type = %+v, want omitted for an empty upstream value", out.Projects[1].Type)
	}
}
