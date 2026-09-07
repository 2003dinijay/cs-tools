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
	"strconv"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
)

// DateRangePayload is the portal's request body for both deployed-product
// metrics endpoints — deploymentId is never client-supplied, always injected
// from the path by the calling handler.
type DateRangePayload struct {
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
}

// IsInvalidDateRange reports whether startDate is after endDate.
func IsInvalidDateRange(startDate, endDate string) bool {
	return startDate != "" && endDate != "" && startDate > endDate
}

// IsWithinOneYear reports whether the span between startDate and endDate
// (both YYYY-MM-DD) is at most one year. ok is false if either date fails to parse.
func IsWithinOneYear(startDate, endDate string) (withinOneYear, ok bool) {
	if len(startDate) != 10 || len(endDate) != 10 {
		return false, false
	}
	startYear, errSY := strconv.Atoi(startDate[0:4])
	startMonth, errSM := strconv.Atoi(startDate[5:7])
	startDay, errSD := strconv.Atoi(startDate[8:10])
	endYear, errEY := strconv.Atoi(endDate[0:4])
	endMonth, errEM := strconv.Atoi(endDate[5:7])
	endDay, errED := strconv.Atoi(endDate[8:10])
	if errSY != nil || errSM != nil || errSD != nil || errEY != nil || errEM != nil || errED != nil {
		return false, false
	}

	yearDiff := endYear - startYear
	if yearDiff > 1 {
		return false, true
	}
	if yearDiff == 1 {
		return endMonth < startMonth || (endMonth == startMonth && endDay <= startDay), true
	}
	return true, true
}

// DeployedProductRef is the deployed-product reference embedded in both
// deployed-product metrics responses — id/name only, dropping entity-service's
// number/internalId/count.
type DeployedProductRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// DeployedProductMetricsInstance is a single instance's contribution to a
// metrics chart entry.
type DeployedProductMetricsInstance struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Cores int    `json:"cores"`
}

// DeployedProductMetricsChartEntry is one date's worth of core-count data.
type DeployedProductMetricsChartEntry struct {
	Date          string                           `json:"date"`
	InstanceCount int                              `json:"instanceCount"`
	TotalCores    int                              `json:"totalCores"`
	MinCores      int                              `json:"minCores"`
	MaxCores      int                              `json:"maxCores"`
	AvgCores      float64                          `json:"avgCores"`
	Instances     []DeployedProductMetricsInstance `json:"instances"`
}

// DeployedProductMetricsDateRange is the queried date range echoed back in a summary.
type DeployedProductMetricsDateRange struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// DeployedProductMetricsSummary is the summary statistics for a metrics query.
type DeployedProductMetricsSummary struct {
	DateRange      DeployedProductMetricsDateRange `json:"dateRange"`
	TotalInstances int                             `json:"totalInstances"`
	MinCores       *int                            `json:"minCores"`
	MaxCores       *int                            `json:"maxCores"`
	AvgCores       *float64                        `json:"avgCores"`
}

// DeployedProductMetricsResponse is the portal's response for
// POST /deployments/{deploymentId}/products/{productId}/metrics/search.
type DeployedProductMetricsResponse struct {
	Product   DeployedProductRef                 `json:"product"`
	Summary   DeployedProductMetricsSummary      `json:"summary"`
	ChartData []DeployedProductMetricsChartEntry `json:"chartData"`
}

// MapDeployedProductMetrics builds the portal response from entity-service's
// DeployedProductMetricsResponse.
func MapDeployedProductMetrics(r entity.DeployedProductMetricsResponse) DeployedProductMetricsResponse {
	chartData := make([]DeployedProductMetricsChartEntry, 0, len(r.ChartData))
	for _, e := range r.ChartData {
		instances := make([]DeployedProductMetricsInstance, 0, len(e.Instances))
		for _, i := range e.Instances {
			instances = append(instances, DeployedProductMetricsInstance{ID: i.ID, Name: i.Name, Cores: i.Cores})
		}
		chartData = append(chartData, DeployedProductMetricsChartEntry{
			Date:          e.Date,
			InstanceCount: e.InstanceCount,
			TotalCores:    e.TotalCores,
			MinCores:      e.MinCores,
			MaxCores:      e.MaxCores,
			AvgCores:      e.AvgCores,
			Instances:     instances,
		})
	}
	return DeployedProductMetricsResponse{
		Product: DeployedProductRef{ID: r.DeployedProduct.ID, Name: r.DeployedProduct.Name},
		Summary: DeployedProductMetricsSummary{
			DateRange:      DeployedProductMetricsDateRange{Start: r.Summary.DateRange.Start, End: r.Summary.DateRange.End},
			TotalInstances: r.Summary.TotalInstances,
			MinCores:       r.Summary.MinCores,
			MaxCores:       r.Summary.MaxCores,
			AvgCores:       r.Summary.AvgCores,
		},
		ChartData: chartData,
	}
}

// UsageCountInstance is a single instance's contribution to a usage-count entry.
type UsageCountInstance struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}

// UsageCountEntry is one count type's aggregated value on a single date.
type UsageCountEntry struct {
	Value       float64              `json:"value"`
	Aggregation string               `json:"aggregation"`
	Instances   []UsageCountInstance `json:"instances"`
}

// DeployedProductUsageCountsChartEntry is one date's worth of usage-count data.
type DeployedProductUsageCountsChartEntry struct {
	Date   string                     `json:"date"`
	Counts map[string]UsageCountEntry `json:"counts"`
}

// CountTypeAggregation is the summary statistics for a single count type over
// the queried date range.
type CountTypeAggregation struct {
	Aggregation string  `json:"aggregation"`
	Min         float64 `json:"min"`
	Max         float64 `json:"max"`
	Avg         float64 `json:"avg"`
}

// DeployedProductUsageCountsSummary is the summary statistics for a usage-counts query.
type DeployedProductUsageCountsSummary struct {
	DateRange  DeployedProductMetricsDateRange `json:"dateRange"`
	CountTypes map[string]CountTypeAggregation `json:"countTypes"`
}

// DeployedProductUsageCountsResponse is the portal's response for
// POST /deployments/{deploymentId}/products/{productId}/metrics/usage-counts/search.
type DeployedProductUsageCountsResponse struct {
	Product   DeployedProductRef                     `json:"product"`
	Summary   DeployedProductUsageCountsSummary      `json:"summary"`
	ChartData []DeployedProductUsageCountsChartEntry `json:"chartData"`
}

// MapDeployedProductUsageCounts builds the portal response from entity-service's
// DeployedProductUsageCountsResponse.
func MapDeployedProductUsageCounts(r entity.DeployedProductUsageCountsResponse) DeployedProductUsageCountsResponse {
	countTypes := make(map[string]CountTypeAggregation, len(r.Summary.CountTypes))
	for k, v := range r.Summary.CountTypes {
		countTypes[k] = CountTypeAggregation{Aggregation: v.Aggregation, Min: v.Min, Max: v.Max, Avg: v.Avg}
	}

	chartData := make([]DeployedProductUsageCountsChartEntry, 0, len(r.ChartData))
	for _, e := range r.ChartData {
		counts := make(map[string]UsageCountEntry, len(e.Counts))
		for k, v := range e.Counts {
			instances := make([]UsageCountInstance, 0, len(v.Instances))
			for _, i := range v.Instances {
				instances = append(instances, UsageCountInstance{ID: i.ID, Name: i.Name, Value: i.Value})
			}
			counts[k] = UsageCountEntry{Value: v.Value, Aggregation: v.Aggregation, Instances: instances}
		}
		chartData = append(chartData, DeployedProductUsageCountsChartEntry{Date: e.Date, Counts: counts})
	}

	return DeployedProductUsageCountsResponse{
		Product: DeployedProductRef{ID: r.DeployedProduct.ID, Name: r.DeployedProduct.Name},
		Summary: DeployedProductUsageCountsSummary{
			DateRange:  DeployedProductMetricsDateRange{Start: r.Summary.DateRange.Start, End: r.Summary.DateRange.End},
			CountTypes: countTypes,
		},
		ChartData: chartData,
	}
}
