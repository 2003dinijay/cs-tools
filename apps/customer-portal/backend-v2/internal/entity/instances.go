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

package entity

import "context"

// SearchInstances calls POST /instances/search.
func (c *Client) SearchInstances(ctx context.Context, req SearchInstancesRequest) (SearchInstancesResponse, error) {
	var out SearchInstancesResponse
	err := c.postJSON(ctx, "/instances/search", req, &out)
	return out, err
}

// SearchInstanceMetrics calls POST /instances/metrics/search.
func (c *Client) SearchInstanceMetrics(ctx context.Context, req InstanceMetricsRequest) (InstanceMetricsResponse, error) {
	var out InstanceMetricsResponse
	err := c.postJSON(ctx, "/instances/metrics/search", req, &out)
	return out, err
}

// SearchInstanceUsage calls POST /instances/usages/search.
func (c *Client) SearchInstanceUsage(ctx context.Context, req InstanceUsageRequest) (InstanceUsageResponse, error) {
	var out InstanceUsageResponse
	err := c.postJSON(ctx, "/instances/usages/search", req, &out)
	return out, err
}

// SearchInstanceMetricsStats calls POST /instances/metrics/stats/search.
func (c *Client) SearchInstanceMetricsStats(ctx context.Context, req InstanceMetricsStatsRequest) (InstanceMetricsStatsResponse, error) {
	var out InstanceMetricsStatsResponse
	err := c.postJSON(ctx, "/instances/metrics/stats/search", req, &out)
	return out, err
}

// SearchInstanceUsageStats calls POST /instances/usages/stats/search.
func (c *Client) SearchInstanceUsageStats(ctx context.Context, req InstanceUsageStatsRequest) (InstanceUsageStatsResponse, error) {
	var out InstanceUsageStatsResponse
	err := c.postJSON(ctx, "/instances/usages/stats/search", req, &out)
	return out, err
}
