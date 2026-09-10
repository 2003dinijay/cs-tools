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
	"encoding/json"
	"fmt"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	integrationservice "github.com/wso2-open-operations/cs-tools/entity-service/internal/servicenow-integration-service"
)

// validateExclusiveIDFilters mirrors the Ballerina reference's shared
// validateIdFilterMutualExclusivity: at most one of projectIDs, deploymentIDs,
// and deployedProductIDs may be non-empty.
func validateExclusiveIDFilters(projectIDs, deploymentIDs, deployedProductIDs []string) error {
	filled := 0
	if len(projectIDs) > 0 {
		filled++
	}
	if len(deploymentIDs) > 0 {
		filled++
	}
	if len(deployedProductIDs) > 0 {
		filled++
	}
	if filled > 1 {
		return &apierror.ValidationError{Msg: "only one of projectIds, deploymentIds, or deployedProductIds can be provided at a time"}
	}
	return nil
}

// snInstanceMetadata mirrors the Choreo InstanceMetadata shape.
type snInstanceMetadata struct {
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

func (m *snInstanceMetadata) toDomain() *domain.InstanceMetadata {
	if m == nil {
		return nil
	}
	return &domain.InstanceMetadata{
		ID:                 sysidToUUID(m.ID),
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

// toDomainOptionalRef converts an optional snReferenceTableItem to a
// domain.ReferenceTableItem pointer, used throughout this file for the
// project/deployment/product/deployedProduct reference fields, all of which
// are nullable in the Choreo response.
func toDomainOptionalRef(r *snReferenceTableItem) *domain.ReferenceTableItem {
	if r == nil {
		return nil
	}
	v := r.toDomain()
	return &v
}

// snInstance mirrors the Choreo Instance shape.
type snInstance struct {
	ID              string                `json:"id"`
	Key             string                `json:"key"`
	Project         *snReferenceTableItem `json:"project"`
	Deployment      *snReferenceTableItem `json:"deployment"`
	Product         *snReferenceTableItem `json:"product"`
	DeployedProduct *snReferenceTableItem `json:"deployedProduct"`
	CreatedOn       string                `json:"createdOn"`
	UpdatedOn       string                `json:"updatedOn"`
	Metadata        *snInstanceMetadata   `json:"metadata"`
}

func (i snInstance) toDomain() domain.Instance {
	return domain.Instance{
		ID:              sysidToUUID(i.ID),
		Key:             i.Key,
		Project:         toDomainOptionalRef(i.Project),
		Deployment:      toDomainOptionalRef(i.Deployment),
		Product:         toDomainOptionalRef(i.Product),
		DeployedProduct: toDomainOptionalRef(i.DeployedProduct),
		CreatedOn:       i.CreatedOn,
		UpdatedOn:       i.UpdatedOn,
		Metadata:        i.Metadata.toDomain(),
	}
}

// snInstanceSearchFilters mirrors the Choreo InstanceSearchPayload.filters shape.
type snInstanceSearchFilters struct {
	StartDate          *string  `json:"startDate,omitempty"`
	EndDate            *string  `json:"endDate,omitempty"`
	ProjectIDs         []string `json:"projectIds,omitempty"`
	DeploymentIDs      []string `json:"deploymentIds,omitempty"`
	DeployedProductIDs []string `json:"deployedProductIds,omitempty"`
}

// snInstanceSearchPayload mirrors the Choreo POST /instances/search request body.
type snInstanceSearchPayload struct {
	Filters    *snInstanceSearchFilters `json:"filters,omitempty"`
	Pagination snProjectPagination      `json:"pagination"`
}

// snInstancesResponse mirrors the Choreo POST /instances/search response.
type snInstancesResponse struct {
	Instances    []snInstance `json:"instances"`
	TotalRecords int          `json:"totalRecords"`
	Limit        int          `json:"limit"`
	Offset       int          `json:"offset"`
}

type snInstanceService struct {
	client *integrationservice.Client
}

// NewServiceNowInstanceService constructs an InstanceService backed by the Choreo API.
func NewServiceNowInstanceService(client *integrationservice.Client) InstanceService {
	return &snInstanceService{client: client}
}

func (s *snInstanceService) SearchInstances(ctx context.Context, req domain.SearchInstancesRequest) (domain.SearchInstancesResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchInstancesResponse{}, err
	}

	payload := snInstanceSearchPayload{
		Pagination: snProjectPagination{Limit: req.Pagination.Limit, Offset: req.Pagination.Offset},
	}

	if req.Filters != nil {
		if err := validateUUIDs("projectIds", req.Filters.ProjectIDs); err != nil {
			return domain.SearchInstancesResponse{}, err
		}
		if err := validateUUIDs("deploymentIds", req.Filters.DeploymentIDs); err != nil {
			return domain.SearchInstancesResponse{}, err
		}
		if err := validateUUIDs("deployedProductIds", req.Filters.DeployedProductIDs); err != nil {
			return domain.SearchInstancesResponse{}, err
		}
		if err := validateExclusiveIDFilters(req.Filters.ProjectIDs, req.Filters.DeploymentIDs, req.Filters.DeployedProductIDs); err != nil {
			return domain.SearchInstancesResponse{}, err
		}

		payload.Filters = &snInstanceSearchFilters{
			StartDate:          req.Filters.StartDate,
			EndDate:            req.Filters.EndDate,
			ProjectIDs:         uuidsToSysids(req.Filters.ProjectIDs),
			DeploymentIDs:      uuidsToSysids(req.Filters.DeploymentIDs),
			DeployedProductIDs: uuidsToSysids(req.Filters.DeployedProductIDs),
		}
	}

	token := middleware.UserIDTokenFromContext(ctx)

	raw, err := s.client.Post(ctx, "/instances/search", token, payload)
	if err != nil {
		return domain.SearchInstancesResponse{}, err
	}

	var snResp snInstancesResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.SearchInstancesResponse{}, fmt.Errorf("sn search instances: parse response: %w", err)
	}

	instances := make([]domain.Instance, 0, len(snResp.Instances))
	for _, i := range snResp.Instances {
		instances = append(instances, i.toDomain())
	}

	return domain.SearchInstancesResponse{
		Instances: instances,
		Total:     snResp.TotalRecords,
		Limit:     snResp.Limit,
		Offset:    snResp.Offset,
	}, nil
}

// snInstanceDateRangeFilters mirrors the shared filters shape used by
// InstanceMetricsPayload and InstanceUsagePayload.
type snInstanceDateRangeFilters struct {
	StartDate          string   `json:"startDate"`
	EndDate            string   `json:"endDate"`
	ProjectIDs         []string `json:"projectIds,omitempty"`
	DeploymentIDs      []string `json:"deploymentIds,omitempty"`
	DeployedProductIDs []string `json:"deployedProductIds,omitempty"`
}

func validateInstanceDateRangeFilters(f domain.InstanceDateRangeFilters) (snInstanceDateRangeFilters, error) {
	if err := validateUUIDs("projectIds", f.ProjectIDs); err != nil {
		return snInstanceDateRangeFilters{}, err
	}
	if err := validateUUIDs("deploymentIds", f.DeploymentIDs); err != nil {
		return snInstanceDateRangeFilters{}, err
	}
	if err := validateUUIDs("deployedProductIds", f.DeployedProductIDs); err != nil {
		return snInstanceDateRangeFilters{}, err
	}
	if err := validateExclusiveIDFilters(f.ProjectIDs, f.DeploymentIDs, f.DeployedProductIDs); err != nil {
		return snInstanceDateRangeFilters{}, err
	}
	return snInstanceDateRangeFilters{
		StartDate:          f.StartDate,
		EndDate:            f.EndDate,
		ProjectIDs:         uuidsToSysids(f.ProjectIDs),
		DeploymentIDs:      uuidsToSysids(f.DeploymentIDs),
		DeployedProductIDs: uuidsToSysids(f.DeployedProductIDs),
	}, nil
}

// snInstanceMetricsPayload mirrors the Choreo POST /instances/metrics/search request body.
type snInstanceMetricsPayload struct {
	Filters snInstanceDateRangeFilters `json:"filters"`
}

// snInstanceDataPoint mirrors the Choreo InstanceDataPoint shape.
type snInstanceDataPoint struct {
	Date               string         `json:"date"`
	CreatedOn          string         `json:"createdOn"`
	CoreCount          *int           `json:"coreCount"`
	JDKVersion         *string        `json:"jdkVersion"`
	Updates            *int           `json:"updates"`
	DeploymentMetadata map[string]any `json:"deploymentMetadata"`
}

// snInstanceMetric mirrors the Choreo InstanceMetric shape.
type snInstanceMetric struct {
	InstanceID      string                `json:"instanceId"`
	InstanceKey     string                `json:"instanceKey"`
	Project         *snReferenceTableItem `json:"project"`
	Deployment      *snReferenceTableItem `json:"deployment"`
	Product         *snReferenceTableItem `json:"product"`
	DeployedProduct *snReferenceTableItem `json:"deployedProduct"`
	DataPoints      []snInstanceDataPoint `json:"dataPoints"`
}

// snInstanceMetricsResponse mirrors the Choreo POST /instances/metrics/search response.
type snInstanceMetricsResponse struct {
	Metrics        []snInstanceMetric `json:"metrics"`
	TotalInstances int                `json:"totalInstances"`
	StartDate      string             `json:"startDate"`
	EndDate        string             `json:"endDate"`
}

func (s *snInstanceService) SearchInstanceMetrics(ctx context.Context, req domain.InstanceMetricsRequest) (domain.InstanceMetricsResponse, error) {
	filters, err := validateInstanceDateRangeFilters(req.Filters)
	if err != nil {
		return domain.InstanceMetricsResponse{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)

	payload := snInstanceMetricsPayload{Filters: filters}

	raw, err := s.client.Post(ctx, "/instances/metrics/search", token, payload)
	if err != nil {
		return domain.InstanceMetricsResponse{}, err
	}

	var snResp snInstanceMetricsResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.InstanceMetricsResponse{}, fmt.Errorf("sn instance metrics: parse response: %w", err)
	}

	metrics := make([]domain.InstanceMetric, 0, len(snResp.Metrics))
	for _, m := range snResp.Metrics {
		dataPoints := make([]domain.InstanceDataPoint, 0, len(m.DataPoints))
		for _, dp := range m.DataPoints {
			dataPoints = append(dataPoints, domain.InstanceDataPoint{
				Date:               dp.Date,
				CreatedOn:          dp.CreatedOn,
				CoreCount:          dp.CoreCount,
				JDKVersion:         dp.JDKVersion,
				Updates:            dp.Updates,
				DeploymentMetadata: dp.DeploymentMetadata,
			})
		}
		metrics = append(metrics, domain.InstanceMetric{
			InstanceID:      sysidToUUID(m.InstanceID),
			InstanceKey:     m.InstanceKey,
			Project:         toDomainOptionalRef(m.Project),
			Deployment:      toDomainOptionalRef(m.Deployment),
			Product:         toDomainOptionalRef(m.Product),
			DeployedProduct: toDomainOptionalRef(m.DeployedProduct),
			DataPoints:      dataPoints,
		})
	}

	return domain.InstanceMetricsResponse{
		Metrics:        metrics,
		TotalInstances: snResp.TotalInstances,
		StartDate:      snResp.StartDate,
		EndDate:        snResp.EndDate,
	}, nil
}

// snInstanceUsagePayload mirrors the Choreo POST /instances/usages/search request body.
type snInstanceUsagePayload struct {
	Filters snInstanceDateRangeFilters `json:"filters"`
}

// snInstanceSummary mirrors the Choreo InstanceSummary shape.
type snInstanceSummary struct {
	Period string         `json:"period"`
	Counts map[string]int `json:"counts"`
}

// snInstanceUsageEntry mirrors the Choreo InstanceUsageEntry shape.
type snInstanceUsageEntry struct {
	InstanceID      string                `json:"instanceId"`
	InstanceKey     string                `json:"instanceKey"`
	Project         *snReferenceTableItem `json:"project"`
	Deployment      *snReferenceTableItem `json:"deployment"`
	Product         *snReferenceTableItem `json:"product"`
	DeployedProduct *snReferenceTableItem `json:"deployedProduct"`
	PeriodSummaries []snInstanceSummary   `json:"periodSummaries"`
}

// snInstanceUsageResponse mirrors the Choreo POST /instances/usages/search response.
type snInstanceUsageResponse struct {
	Usages         []snInstanceUsageEntry `json:"usages"`
	TotalInstances int                    `json:"totalInstances"`
	StartDate      string                 `json:"startDate"`
	EndDate        string                 `json:"endDate"`
}

func (s *snInstanceService) SearchInstanceUsage(ctx context.Context, req domain.InstanceUsageRequest) (domain.InstanceUsageResponse, error) {
	filters, err := validateInstanceDateRangeFilters(req.Filters)
	if err != nil {
		return domain.InstanceUsageResponse{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)

	payload := snInstanceUsagePayload{Filters: filters}

	raw, err := s.client.Post(ctx, "/instances/usages/search", token, payload)
	if err != nil {
		return domain.InstanceUsageResponse{}, err
	}

	var snResp snInstanceUsageResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.InstanceUsageResponse{}, fmt.Errorf("sn instance usage: parse response: %w", err)
	}

	usages := make([]domain.InstanceUsageEntry, 0, len(snResp.Usages))
	for _, u := range snResp.Usages {
		summaries := make([]domain.InstanceSummary, 0, len(u.PeriodSummaries))
		for _, sm := range u.PeriodSummaries {
			summaries = append(summaries, domain.InstanceSummary{Period: sm.Period, Counts: sm.Counts})
		}
		usages = append(usages, domain.InstanceUsageEntry{
			InstanceID:      sysidToUUID(u.InstanceID),
			InstanceKey:     u.InstanceKey,
			Project:         toDomainOptionalRef(u.Project),
			Deployment:      toDomainOptionalRef(u.Deployment),
			Product:         toDomainOptionalRef(u.Product),
			DeployedProduct: toDomainOptionalRef(u.DeployedProduct),
			PeriodSummaries: summaries,
		})
	}

	return domain.InstanceUsageResponse{
		Usages:         usages,
		TotalInstances: snResp.TotalInstances,
		StartDate:      snResp.StartDate,
		EndDate:        snResp.EndDate,
	}, nil
}

// snInstanceStatsFilters mirrors the shared filters shape used by
// InstanceMetricsStatsPayload and InstanceUsageStatsPayload — adds the
// optional dataSource discriminator on top of snInstanceDateRangeFilters.
type snInstanceStatsFilters struct {
	StartDate          string   `json:"startDate"`
	EndDate            string   `json:"endDate"`
	ProjectIDs         []string `json:"projectIds,omitempty"`
	DeploymentIDs      []string `json:"deploymentIds,omitempty"`
	DeployedProductIDs []string `json:"deployedProductIds,omitempty"`
	DataSource         *int     `json:"dataSource,omitempty"`
}

func validateInstanceStatsFilters(f domain.InstanceStatsFilters) (snInstanceStatsFilters, error) {
	base, err := validateInstanceDateRangeFilters(f.InstanceDateRangeFilters)
	if err != nil {
		return snInstanceStatsFilters{}, err
	}
	return snInstanceStatsFilters{
		StartDate:          base.StartDate,
		EndDate:            base.EndDate,
		ProjectIDs:         base.ProjectIDs,
		DeploymentIDs:      base.DeploymentIDs,
		DeployedProductIDs: base.DeployedProductIDs,
		DataSource:         f.DataSource,
	}, nil
}

// snInstanceMetricsStatsPayload mirrors the Choreo POST /instances/metrics/stats request body.
type snInstanceMetricsStatsPayload struct {
	Filters snInstanceStatsFilters `json:"filters"`
}

// snInstanceMetricSummary mirrors the Choreo MetricSummary shape.
type snInstanceMetricSummary struct {
	Current float64 `json:"curr"`
	Min     float64 `json:"min"`
	Max     float64 `json:"max"`
	Avg     float64 `json:"avg"`
}

// snInstanceMetricsStatsResponse mirrors the Choreo POST /instances/metrics/stats response.
type snInstanceMetricsStatsResponse struct {
	Stats        map[string]map[string]int `json:"stats"`
	Summary      snInstanceMetricSummary   `json:"summary"`
	TotalRecords int                       `json:"totalRecords"`
	StartDate    string                    `json:"startDate"`
	EndDate      string                    `json:"endDate"`
}

// SearchInstanceMetricsStats calls the Choreo /instances/metrics/stats
// endpoint — note this SN path has no trailing /search, unlike this method's
// own exposed route, mirroring the Ballerina reference's own asymmetry
// between its resource path and the downstream servicenow module call.
func (s *snInstanceService) SearchInstanceMetricsStats(ctx context.Context, req domain.InstanceMetricsStatsRequest) (domain.InstanceMetricsStatsResponse, error) {
	filters, err := validateInstanceStatsFilters(req.Filters)
	if err != nil {
		return domain.InstanceMetricsStatsResponse{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)

	payload := snInstanceMetricsStatsPayload{Filters: filters}

	raw, err := s.client.Post(ctx, "/instances/metrics/stats", token, payload)
	if err != nil {
		return domain.InstanceMetricsStatsResponse{}, err
	}

	var snResp snInstanceMetricsStatsResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.InstanceMetricsStatsResponse{}, fmt.Errorf("sn instance metrics stats: parse response: %w", err)
	}

	return domain.InstanceMetricsStatsResponse{
		Stats: snResp.Stats,
		Summary: domain.InstanceMetricSummary{
			Current: snResp.Summary.Current,
			Min:     snResp.Summary.Min,
			Max:     snResp.Summary.Max,
			Avg:     snResp.Summary.Avg,
		},
		Total:     snResp.TotalRecords,
		StartDate: snResp.StartDate,
		EndDate:   snResp.EndDate,
	}, nil
}

// snInstanceUsageStatsPayload mirrors the Choreo POST /instances/usages/stats request body.
type snInstanceUsageStatsPayload struct {
	Filters snInstanceStatsFilters `json:"filters"`
}

// snInstanceUsageStatsResponse mirrors the Choreo POST /instances/usages/stats response.
type snInstanceUsageStatsResponse struct {
	Stats        map[string]map[string]int `json:"stats"`
	TotalRecords int                       `json:"totalRecords"`
	StartDate    string                    `json:"startDate"`
	EndDate      string                    `json:"endDate"`
}

// SearchInstanceUsageStats calls the Choreo /instances/usages/stats endpoint
// — same no-trailing-/search asymmetry as SearchInstanceMetricsStats.
func (s *snInstanceService) SearchInstanceUsageStats(ctx context.Context, req domain.InstanceUsageStatsRequest) (domain.InstanceUsageStatsResponse, error) {
	filters, err := validateInstanceStatsFilters(req.Filters)
	if err != nil {
		return domain.InstanceUsageStatsResponse{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)

	payload := snInstanceUsageStatsPayload{Filters: filters}

	raw, err := s.client.Post(ctx, "/instances/usages/stats", token, payload)
	if err != nil {
		return domain.InstanceUsageStatsResponse{}, err
	}

	var snResp snInstanceUsageStatsResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.InstanceUsageStatsResponse{}, fmt.Errorf("sn instance usage stats: parse response: %w", err)
	}

	return domain.InstanceUsageStatsResponse{
		Stats:     snResp.Stats,
		Total:     snResp.TotalRecords,
		StartDate: snResp.StartDate,
		EndDate:   snResp.EndDate,
	}, nil
}
