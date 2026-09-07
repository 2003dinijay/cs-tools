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

// Package dto's instance-related types and mappers back the 15 portal
// endpoints fanned out from entity-service's 5 instance endpoints: each of
// searchInstances/searchInstanceMetrics/searchInstanceUsage/
// searchInstanceMetricsStats/searchInstanceUsageStats is exposed as three
// portal routes (project-scoped, deployment-scoped, deployed-product-scoped)
// that each force exactly one ID filter from the URL path. The mapping logic
// itself is identical across all three scopes — only the handler layer
// differs in which path param feeds which entity filter field.
package dto

import "github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"

func toOptionalRef(r *entity.ReferenceTableItem) *ReferenceItem {
	if r == nil {
		return nil
	}
	return &ReferenceItem{ID: r.ID, Label: r.Name}
}

// InstanceSearchDateFilters holds the optional date-range filter nested
// under InstanceSearchRequest.Filters, matching the frontend's own
// InstanceSearchFilters type (apps/customer-portal/webapp/src/features/
// project-details/types/usage.ts: filters?: { startDate?, endDate? }) —
// the frontend sends these nested, not top-level.
type InstanceSearchDateFilters struct {
	StartDate *string `json:"startDate,omitempty"`
	EndDate   *string `json:"endDate,omitempty"`
}

// InstanceSearchRequest is the portal's request body for the three
// instances/search routes. Exactly one path param (project/deployment/
// deployed-product ID) is injected server-side into the corresponding
// entity filter field by the calling handler.
type InstanceSearchRequest struct {
	Filters    *InstanceSearchDateFilters `json:"filters,omitempty"`
	Pagination entity.Pagination          `json:"pagination"`
}

// InstanceMetadata is the portal's own copy of entity.InstanceMetadata.
type InstanceMetadata struct {
	ID                 string         `json:"id"`
	CoreCount          *int           `json:"coreCount"`
	Updates            *int           `json:"updates"`
	JDKVersion         *string        `json:"jdkVersion"`
	DeploymentMetadata map[string]any `json:"deploymentMetadata"`
	CreatedOn          string         `json:"createdOn"`
	UpdatedOn          string         `json:"updatedOn"`
	CustomCreatedOn    *string        `json:"customCreatedOn"`
	CustomUpdatedOn    *string        `json:"customUpdatedOn"`
}

func toOptionalInstanceMetadata(m *entity.InstanceMetadata) *InstanceMetadata {
	if m == nil {
		return nil
	}
	return &InstanceMetadata{
		ID:                 m.ID,
		CoreCount:          m.CoreCount,
		Updates:            m.Updates,
		JDKVersion:         m.JDKVersion,
		DeploymentMetadata: m.DeploymentMetadata,
		CreatedOn:          m.CreatedOn,
		UpdatedOn:          m.UpdatedOn,
		CustomCreatedOn:    m.CustomCreatedOn,
		CustomUpdatedOn:    m.CustomUpdatedOn,
	}
}

// Instance is a single instance row.
type Instance struct {
	ID              string            `json:"id"`
	Key             string            `json:"key"`
	Project         *ReferenceItem    `json:"project"`
	Deployment      *ReferenceItem    `json:"deployment"`
	Product         *ReferenceItem    `json:"product"`
	DeployedProduct *ReferenceItem    `json:"deployedProduct"`
	CreatedOn       string            `json:"createdOn"`
	UpdatedOn       string            `json:"updatedOn"`
	Metadata        *InstanceMetadata `json:"metadata"`
}

// InstanceSearchResponse is the portal's response for the three
// instances/search routes. TotalRecords (not Total) to match the frontend's
// shared pagination envelope (InstancesResponse extends PaginationResponse).
type InstanceSearchResponse struct {
	Instances    []Instance `json:"instances"`
	TotalRecords int        `json:"totalRecords"`
	Offset       int        `json:"offset"`
	Limit        int        `json:"limit"`
}

// MapInstanceSearchResponse builds the portal response from entity-service's
// SearchInstancesResponse.
func MapInstanceSearchResponse(r entity.SearchInstancesResponse) InstanceSearchResponse {
	items := make([]Instance, 0, len(r.Instances))
	for _, i := range r.Instances {
		items = append(items, Instance{
			ID:              i.ID,
			Key:             i.Key,
			Project:         toOptionalRef(i.Project),
			Deployment:      toOptionalRef(i.Deployment),
			Product:         toOptionalRef(i.Product),
			DeployedProduct: toOptionalRef(i.DeployedProduct),
			CreatedOn:       i.CreatedOn,
			UpdatedOn:       i.UpdatedOn,
			Metadata:        toOptionalInstanceMetadata(i.Metadata),
		})
	}
	return InstanceSearchResponse{Instances: items, TotalRecords: r.Total, Offset: r.Offset, Limit: r.Limit}
}

// InstanceDateRangeRequest is the portal's request body for the
// instances/metrics/search and instances/usages/search route families.
// StartDate/EndDate are required; the path-scoped ID filter is injected
// server-side by the calling handler.
type InstanceDateRangeRequest struct {
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
}

// InstanceDataPoint is the portal's own copy of entity.InstanceDataPoint.
type InstanceDataPoint struct {
	Date               string         `json:"date"`
	CreatedOn          string         `json:"createdOn"`
	CoreCount          *int           `json:"coreCount"`
	JDKVersion         *string        `json:"jdkVersion"`
	Updates            *int           `json:"updates"`
	DeploymentMetadata map[string]any `json:"deploymentMetadata"`
}

func mapInstanceDataPoints(points []entity.InstanceDataPoint) []InstanceDataPoint {
	out := make([]InstanceDataPoint, 0, len(points))
	for _, p := range points {
		out = append(out, InstanceDataPoint{
			Date:               p.Date,
			CreatedOn:          p.CreatedOn,
			CoreCount:          p.CoreCount,
			JDKVersion:         p.JDKVersion,
			Updates:            p.Updates,
			DeploymentMetadata: p.DeploymentMetadata,
		})
	}
	return out
}

// InstanceMetric is one instance's metric time series, ordered newest to oldest.
type InstanceMetric struct {
	InstanceID      string              `json:"instanceId"`
	InstanceKey     string              `json:"instanceKey"`
	Project         *ReferenceItem      `json:"project"`
	Deployment      *ReferenceItem      `json:"deployment"`
	Product         *ReferenceItem      `json:"product"`
	DeployedProduct *ReferenceItem      `json:"deployedProduct"`
	DataPoints      []InstanceDataPoint `json:"dataPoints"`
}

// InstanceMetricsResponse is the portal's response for the instances/metrics/search routes.
type InstanceMetricsResponse struct {
	Metrics        []InstanceMetric `json:"metrics"`
	TotalInstances int              `json:"totalInstances"`
	StartDate      string           `json:"startDate"`
	EndDate        string           `json:"endDate"`
}

// MapInstanceMetricsResponse builds the portal response from entity-service's
// InstanceMetricsResponse.
func MapInstanceMetricsResponse(r entity.InstanceMetricsResponse) InstanceMetricsResponse {
	metrics := make([]InstanceMetric, 0, len(r.Metrics))
	for _, m := range r.Metrics {
		metrics = append(metrics, InstanceMetric{
			InstanceID:      m.InstanceID,
			InstanceKey:     m.InstanceKey,
			Project:         toOptionalRef(m.Project),
			Deployment:      toOptionalRef(m.Deployment),
			Product:         toOptionalRef(m.Product),
			DeployedProduct: toOptionalRef(m.DeployedProduct),
			DataPoints:      mapInstanceDataPoints(m.DataPoints),
		})
	}
	return InstanceMetricsResponse{
		Metrics:        metrics,
		TotalInstances: r.TotalInstances,
		StartDate:      r.StartDate,
		EndDate:        r.EndDate,
	}
}

// InstanceUsageSummary is the portal's own copy of entity.InstanceSummary —
// one period's usage counts for an instance.
type InstanceUsageSummary struct {
	Period string         `json:"period"`
	Counts map[string]int `json:"counts"`
}

func mapInstanceUsageSummaries(summaries []entity.InstanceSummary) []InstanceUsageSummary {
	out := make([]InstanceUsageSummary, 0, len(summaries))
	for _, s := range summaries {
		out = append(out, InstanceUsageSummary{Period: s.Period, Counts: s.Counts})
	}
	return out
}

// InstanceUsageEntry is one instance's usage time series.
type InstanceUsageEntry struct {
	InstanceID      string                 `json:"instanceId"`
	InstanceKey     string                 `json:"instanceKey"`
	Project         *ReferenceItem         `json:"project"`
	Deployment      *ReferenceItem         `json:"deployment"`
	Product         *ReferenceItem         `json:"product"`
	DeployedProduct *ReferenceItem         `json:"deployedProduct"`
	PeriodSummaries []InstanceUsageSummary `json:"periodSummaries"`
}

// InstanceUsageResponse is the portal's response for the instances/usages/search routes.
type InstanceUsageResponse struct {
	Usages         []InstanceUsageEntry `json:"usages"`
	TotalInstances int                  `json:"totalInstances"`
	StartDate      string               `json:"startDate"`
	EndDate        string               `json:"endDate"`
}

// MapInstanceUsageResponse builds the portal response from entity-service's
// InstanceUsageResponse.
func MapInstanceUsageResponse(r entity.InstanceUsageResponse) InstanceUsageResponse {
	usages := make([]InstanceUsageEntry, 0, len(r.Usages))
	for _, u := range r.Usages {
		usages = append(usages, InstanceUsageEntry{
			InstanceID:      u.InstanceID,
			InstanceKey:     u.InstanceKey,
			Project:         toOptionalRef(u.Project),
			Deployment:      toOptionalRef(u.Deployment),
			Product:         toOptionalRef(u.Product),
			DeployedProduct: toOptionalRef(u.DeployedProduct),
			PeriodSummaries: mapInstanceUsageSummaries(u.PeriodSummaries),
		})
	}
	return InstanceUsageResponse{
		Usages:         usages,
		TotalInstances: r.TotalInstances,
		StartDate:      r.StartDate,
		EndDate:        r.EndDate,
	}
}

// InstanceStatsRequest is the portal's request body for the
// instances/stats/metrics/search and instances/stats/usages/search route
// families. StartDate/EndDate are required; DataSource is optional (1 = API
// Call, 2 = File Upload). Note: the project-scoped metrics-stats variant does
// NOT forward DataSource to entity-service (a real asymmetry, by design, not
// a bug) — see InstanceMetricsStatsRequestFilters's doc comment on the
// handler side for how this is preserved.
type InstanceStatsRequest struct {
	StartDate  string `json:"startDate"`
	EndDate    string `json:"endDate"`
	DataSource *int   `json:"dataSource,omitempty"`
}

// InstanceMetricSummary is the current/min/max/avg summary for a metrics-stats query.
type InstanceMetricSummary struct {
	Current float64 `json:"current"`
	Min     float64 `json:"min"`
	Max     float64 `json:"max"`
	Avg     float64 `json:"avg"`
}

// InstanceMetricsStatsResponse is the portal's response for the
// instances/stats/metrics/search routes. TotalRecords (not Total) for
// consistency with InstanceUsageStatsResponse, which the frontend's own
// type confirms reads totalRecords.
type InstanceMetricsStatsResponse struct {
	Stats        map[string]map[string]int `json:"stats"`
	Summary      InstanceMetricSummary     `json:"summary"`
	TotalRecords int                       `json:"totalRecords"`
	StartDate    string                    `json:"startDate"`
	EndDate      string                    `json:"endDate"`
}

// MapInstanceMetricsStatsResponse builds the portal response from
// entity-service's InstanceMetricsStatsResponse.
func MapInstanceMetricsStatsResponse(r entity.InstanceMetricsStatsResponse) InstanceMetricsStatsResponse {
	return InstanceMetricsStatsResponse{
		Stats: r.Stats,
		Summary: InstanceMetricSummary{
			Current: r.Summary.Current,
			Min:     r.Summary.Min,
			Max:     r.Summary.Max,
			Avg:     r.Summary.Avg,
		},
		TotalRecords: r.Total,
		StartDate:    r.StartDate,
		EndDate:      r.EndDate,
	}
}

// InstanceUsageStatsResponse is the portal's response for the
// instances/stats/usages/search routes. Unlike InstanceMetricsStatsResponse,
// there is no summary block, by design. TotalRecords (not Total) to match
// the frontend's own InstanceUsageStatsResponse type.
type InstanceUsageStatsResponse struct {
	Stats        map[string]map[string]int `json:"stats"`
	TotalRecords int                       `json:"totalRecords"`
	StartDate    string                    `json:"startDate"`
	EndDate      string                    `json:"endDate"`
}

// MapInstanceUsageStatsResponse builds the portal response from
// entity-service's InstanceUsageStatsResponse.
func MapInstanceUsageStatsResponse(r entity.InstanceUsageStatsResponse) InstanceUsageStatsResponse {
	return InstanceUsageStatsResponse{
		Stats:        r.Stats,
		TotalRecords: r.Total,
		StartDate:    r.StartDate,
		EndDate:      r.EndDate,
	}
}
