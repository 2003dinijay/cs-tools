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
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// usageRouteRepo records which InstanceRepository methods were reached.
// Unlisted methods panic via the nil embedded interface.
type usageRouteRepo struct {
	repository.InstanceRepository
	calls []string
}

func (r *usageRouteRepo) SearchInstances(context.Context, domain.SearchInstancesRequest) ([]domain.Instance, int, error) {
	r.calls = append(r.calls, "SearchInstances")
	return nil, 0, nil
}
func (r *usageRouteRepo) SearchInstanceMetrics(context.Context, domain.InstanceDateRangeFilters) ([]domain.InstanceMetric, int, error) {
	r.calls = append(r.calls, "SearchInstanceMetrics")
	return nil, 0, nil
}
func (r *usageRouteRepo) SearchInstanceUsage(context.Context, domain.InstanceDateRangeFilters) ([]domain.InstanceUsageEntry, int, error) {
	r.calls = append(r.calls, "SearchInstanceUsage")
	return nil, 0, nil
}
func (r *usageRouteRepo) SearchInstanceMetricsStats(context.Context, domain.InstanceDateRangeFilters) (domain.InstanceMetricsStatsResponse, error) {
	r.calls = append(r.calls, "SearchInstanceMetricsStats")
	return domain.InstanceMetricsStatsResponse{}, nil
}
func (r *usageRouteRepo) SearchInstanceUsageStats(context.Context, domain.InstanceDateRangeFilters, *string) (domain.InstanceUsageStatsResponse, error) {
	r.calls = append(r.calls, "SearchInstanceUsageStats")
	return domain.InstanceUsageStatsResponse{}, nil
}

// usageRouteMirror records which InstanceService methods were reached on the
// mirror (the previous read path).
type usageRouteMirror struct{ calls []string }

func (m *usageRouteMirror) SearchInstances(context.Context, domain.SearchInstancesRequest) (domain.SearchInstancesResponse, error) {
	m.calls = append(m.calls, "SearchInstances")
	return domain.SearchInstancesResponse{}, nil
}
func (m *usageRouteMirror) SearchInstanceMetrics(context.Context, domain.InstanceMetricsRequest) (domain.InstanceMetricsResponse, error) {
	m.calls = append(m.calls, "SearchInstanceMetrics")
	return domain.InstanceMetricsResponse{}, nil
}
func (m *usageRouteMirror) SearchInstanceUsage(context.Context, domain.InstanceUsageRequest) (domain.InstanceUsageResponse, error) {
	m.calls = append(m.calls, "SearchInstanceUsage")
	return domain.InstanceUsageResponse{}, nil
}
func (m *usageRouteMirror) SearchInstanceMetricsStats(context.Context, domain.InstanceMetricsStatsRequest) (domain.InstanceMetricsStatsResponse, error) {
	m.calls = append(m.calls, "SearchInstanceMetricsStats")
	return domain.InstanceMetricsStatsResponse{}, nil
}
func (m *usageRouteMirror) SearchInstanceUsageStats(context.Context, domain.InstanceUsageStatsRequest) (domain.InstanceUsageStatsResponse, error) {
	m.calls = append(m.calls, "SearchInstanceUsageStats")
	return domain.InstanceUsageStatsResponse{}, nil
}

var instanceUsageMethods = []string{
	"SearchInstances", "SearchInstanceMetrics", "SearchInstanceUsage",
	"SearchInstanceMetricsStats", "SearchInstanceUsageStats",
}

// callAllInstanceMethods invokes the five methods with a request valid for
// the Postgres path (the mirror path needs no validation at all).
func callAllInstanceMethods(t *testing.T, svc InstanceService) {
	t.Helper()
	ctx := context.Background()
	f := domain.InstanceDateRangeFilters{StartDate: "2026-01-01", EndDate: "2026-01-31"}
	if _, err := svc.SearchInstances(ctx, domain.SearchInstancesRequest{Pagination: domain.Pagination{Limit: 10}}); err != nil {
		t.Fatalf("SearchInstances: %v", err)
	}
	if _, err := svc.SearchInstanceMetrics(ctx, domain.InstanceMetricsRequest{Filters: f}); err != nil {
		t.Fatalf("SearchInstanceMetrics: %v", err)
	}
	if _, err := svc.SearchInstanceUsage(ctx, domain.InstanceUsageRequest{Filters: f}); err != nil {
		t.Fatalf("SearchInstanceUsage: %v", err)
	}
	if _, err := svc.SearchInstanceMetricsStats(ctx, domain.InstanceMetricsStatsRequest{Filters: domain.InstanceStatsFilters{InstanceDateRangeFilters: f}}); err != nil {
		t.Fatalf("SearchInstanceMetricsStats: %v", err)
	}
	if _, err := svc.SearchInstanceUsageStats(ctx, domain.InstanceUsageStatsRequest{Filters: domain.InstanceStatsFilters{InstanceDateRangeFilters: f}}); err != nil {
		t.Fatalf("SearchInstanceUsageStats: %v", err)
	}
}

func equalCalls(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Default (PRODUCT_CONSUMPTION_DATA_READ_MODE=postgres): NewInstanceService, all five reads hit
// the repository and none reaches a mirror.
func TestInstanceService_PostgresRoute_AllFiveReadsHitRepo(t *testing.T) {
	repo := &usageRouteRepo{}
	callAllInstanceMethods(t, NewInstanceService(repo))
	if !equalCalls(repo.calls, instanceUsageMethods) {
		t.Errorf("repo calls = %v, want %v", repo.calls, instanceUsageMethods)
	}
}

// PRODUCT_CONSUMPTION_DATA_READ_MODE=legacy under dual-write: NewInstanceServiceWithSNFallback,
// all five reads go to the mirror and the repository is never touched.
func TestInstanceService_PreviousPathFallback_AllFiveReadsHitMirror(t *testing.T) {
	repo := &usageRouteRepo{}
	mirror := &usageRouteMirror{}
	callAllInstanceMethods(t, NewInstanceServiceWithSNFallback(repo, mirror))
	if !equalCalls(mirror.calls, instanceUsageMethods) {
		t.Errorf("mirror calls = %v, want %v", mirror.calls, instanceUsageMethods)
	}
	if len(repo.calls) != 0 {
		t.Errorf("repo calls = %v, want none", repo.calls)
	}
}

// usageRouteDPMirror is a DeployedProductService whose only live methods are
// the two usage/metrics reads, which it records.
type usageRouteDPMirror struct {
	DeployedProductService
	calls []string
}

func (m *usageRouteDPMirror) SearchDeployedProductMetrics(context.Context, string, domain.DeployedProductMetricsRequest) (domain.DeployedProductMetricsResponse, error) {
	m.calls = append(m.calls, "SearchDeployedProductMetrics")
	return domain.DeployedProductMetricsResponse{}, nil
}
func (m *usageRouteDPMirror) SearchDeployedProductUsageCounts(context.Context, string, domain.DeployedProductUsageCountsRequest) (domain.DeployedProductUsageCountsResponse, error) {
	m.calls = append(m.calls, "SearchDeployedProductUsageCounts")
	return domain.DeployedProductUsageCountsResponse{}, nil
}

// usageRouteDPRepo records the two usage/metrics repository reads.
type usageRouteDPRepo struct {
	repository.DeployedProductRepository
	calls []string
}

func (r *usageRouteDPRepo) SearchDeployedProductMetrics(context.Context, string, string, string, string) (domain.DeployedProductMetricsResponse, error) {
	r.calls = append(r.calls, "SearchDeployedProductMetrics")
	return domain.DeployedProductMetricsResponse{}, nil
}
func (r *usageRouteDPRepo) SearchDeployedProductUsageCounts(context.Context, string, string, string, string) (domain.DeployedProductUsageCountsResponse, error) {
	r.calls = append(r.calls, "SearchDeployedProductUsageCounts")
	return domain.DeployedProductUsageCountsResponse{}, nil
}

func callDeployedProductUsageReads(t *testing.T, svc DeployedProductService) {
	t.Helper()
	ctx := context.Background()
	id, dep := "11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222"
	if _, err := svc.SearchDeployedProductMetrics(ctx, id, domain.DeployedProductMetricsRequest{DeploymentID: dep, StartDate: "2026-01-01", EndDate: "2026-01-31"}); err != nil {
		t.Fatalf("SearchDeployedProductMetrics: %v", err)
	}
	if _, err := svc.SearchDeployedProductUsageCounts(ctx, id, domain.DeployedProductUsageCountsRequest{DeployedProductMetricsRequest: domain.DeployedProductMetricsRequest{DeploymentID: dep, StartDate: "2026-01-01", EndDate: "2026-01-31"}}); err != nil {
		t.Fatalf("SearchDeployedProductUsageCounts: %v", err)
	}
}

var deployedProductUsageMethods = []string{"SearchDeployedProductMetrics", "SearchDeployedProductUsageCounts"}

// Default: the dual-write service (mirror present for writes) still reads
// usage/metrics from Postgres.
func TestDeployedProductService_DualWriteDefault_UsageReadsHitRepo(t *testing.T) {
	repo, mirror := &usageRouteDPRepo{}, &usageRouteDPMirror{}
	callDeployedProductUsageReads(t, NewDeployedProductServiceWithSNWriteback(repo, nil, mirror))
	if !equalCalls(repo.calls, deployedProductUsageMethods) || len(mirror.calls) != 0 {
		t.Errorf("repo=%v mirror=%v, want repo=%v mirror=[]", repo.calls, mirror.calls, deployedProductUsageMethods)
	}
}

// Legacy under dual-write: both usage/metrics reads go to the mirror.
func TestDeployedProductService_LegacyUsageReads_HitMirror(t *testing.T) {
	repo, mirror := &usageRouteDPRepo{}, &usageRouteDPMirror{}
	callDeployedProductUsageReads(t, WithLegacyUsageReads(NewDeployedProductServiceWithSNWriteback(repo, nil, mirror)))
	if !equalCalls(mirror.calls, deployedProductUsageMethods) || len(repo.calls) != 0 {
		t.Errorf("repo=%v mirror=%v, want repo=[] mirror=%v", repo.calls, mirror.calls, deployedProductUsageMethods)
	}
}

// WithLegacyUsageReads is a no-op for a plain Postgres service (no mirror).
func TestDeployedProductService_LegacyUsageReads_NoMirrorIsNoOp(t *testing.T) {
	repo := &usageRouteDPRepo{}
	callDeployedProductUsageReads(t, WithLegacyUsageReads(NewDeployedProductService(repo)))
	if !equalCalls(repo.calls, deployedProductUsageMethods) {
		t.Errorf("repo calls = %v, want %v", repo.calls, deployedProductUsageMethods)
	}
}
