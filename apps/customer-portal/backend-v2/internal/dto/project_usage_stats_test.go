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

// TestMapUsageStats_CopiesTheThreeCounters checks the mapping the Usage
// Metrics page depends on. The page fetched nothing at all before this
// endpoint existed in backend-v2 (it was only ever implemented in the
// Ballerina backend).
func TestMapUsageStats_CopiesTheThreeCounters(t *testing.T) {
	got := MapUsageStats(entity.ProjectStatsResponse{
		DeploymentCount:      3,
		DeployedProductCount: 7,
		InstanceCount:        11,
	})

	if got.DeploymentCount != 3 {
		t.Errorf("DeploymentCount = %d, want 3", got.DeploymentCount)
	}
	if got.DeployedProductCount != 7 {
		t.Errorf("DeployedProductCount = %d, want 7", got.DeployedProductCount)
	}
	if got.InstanceCount != 11 {
		t.Errorf("InstanceCount = %d, want 11", got.InstanceCount)
	}
}

// TestMapUsageStats_MatchesFrontendContract pins the wire shape to the
// frontend's UsageStatsResponse type
// (webapp/src/features/project-details/types/usage.ts): exactly three keys,
// named as below. entity-service's ProjectStatsResponse is a superset — hours,
// SLA status and outstanding counts must not leak into this endpoint, which is
// why it maps through a DTO rather than being passed through.
func TestMapUsageStats_MatchesFrontendContract(t *testing.T) {
	raw, err := json.Marshal(MapUsageStats(entity.ProjectStatsResponse{
		TotalHours:       42,
		BillableHours:    21,
		SLAStatus:        "AT_RISK",
		DeploymentCount:  1,
		InstanceCount:    2,
		OutstandingCount: entity.ProjectStatsOutstandingCount{},
	}))
	if err != nil {
		t.Fatalf("marshal returned error: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}

	want := []string{"deploymentCount", "deployedProductCount", "instanceCount"}
	if len(got) != len(want) {
		t.Errorf("got %d keys %v, want exactly %d %v", len(got), got, len(want), want)
	}
	for _, k := range want {
		if _, ok := got[k]; !ok {
			t.Errorf("missing key %q", k)
		}
	}
	for _, leaked := range []string{"totalHours", "billableHours", "slaStatus", "outstandingCount"} {
		if _, ok := got[leaked]; ok {
			t.Errorf("entity-service field %q leaked into the usage-stats response", leaked)
		}
	}
}
