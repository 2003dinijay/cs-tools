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
