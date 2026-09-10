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

// TestMapSearchDeployments_EmitsProductCount is the regression guard for the
// Usage & Metrics page rendering blank.
//
// That page filters deployments on `(dep.productCount ?? 0) > 0`. The key is
// productCount — NOT deployedProductCount, which is what entity-service calls
// it upstream. When the key is missing every deployment is filtered out, no
// deployment tab is selected, and every downstream metrics query is disabled by
// its own `enabled` guard — so the page renders empty with no console error and
// no network request at all. The Ballerina backend performs the same rename.
func TestMapSearchDeployments_EmitsProductCount(t *testing.T) {
	got := MapSearchDeployments(entity.SearchDeploymentsResponse{
		Deployments: []entity.DeploymentView{
			{ID: "dep-1", Name: "Primary Production", DeployedProductCount: 3},
			{ID: "dep-2", Name: "Empty", DeployedProductCount: 0},
		},
		Total: 2,
	})

	raw, err := json.Marshal(got.Deployments)
	if err != nil {
		t.Fatalf("marshal returned error: %v", err)
	}
	var items []map[string]any
	if err := json.Unmarshal(raw, &items); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}

	if items[0]["productCount"] != float64(3) {
		t.Errorf(`dep-1 productCount = %v, want 3 — the page filters on this exact key`, items[0]["productCount"])
	}
	// Present-and-zero, not omitted: a deployment with no products is a real
	// answer from the upstream count, not missing data.
	if items[1]["productCount"] != float64(0) {
		t.Errorf("dep-2 productCount = %v, want 0", items[1]["productCount"])
	}
	for i := range items {
		if _, wrong := items[i]["deployedProductCount"]; wrong {
			t.Errorf(`item %d emitted "deployedProductCount"; the frontend reads "productCount"`, i)
		}
	}
}

// TestMapSearchDeployments_EmitsURL covers the other field entity-service was
// dropping: the frontend's ProjectDeploymentItem declares url, and the
// Ballerina entity-service's Deployment record carries it.
func TestMapSearchDeployments_EmitsURL(t *testing.T) {
	url := "https://deployment.example.com"
	got := MapSearchDeployments(entity.SearchDeploymentsResponse{
		Deployments: []entity.DeploymentView{{ID: "dep-1", URL: &url}, {ID: "dep-2"}},
	})

	if got.Deployments[0].URL == nil || *got.Deployments[0].URL != url {
		t.Errorf("dep-1 URL = %v, want %q", got.Deployments[0].URL, url)
	}
	if got.Deployments[1].URL != nil {
		t.Errorf("dep-2 URL = %v, want nil when absent upstream", *got.Deployments[1].URL)
	}
}
