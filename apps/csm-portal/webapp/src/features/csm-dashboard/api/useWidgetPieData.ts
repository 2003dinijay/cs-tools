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

import { useQueries } from "@tanstack/react-query";
import { ApiQueryKeys } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import type { BeDashboardPieSlice, BeWidgetResourceType } from "@api/backend/types";
import { WIDGET_RESOURCE_CONFIG } from "@features/csm-dashboard/config/widgetResourceConfig";

export interface PieSliceResult extends BeDashboardPieSlice {
  value: number;
}

export interface WidgetPieData {
  slices: PieSliceResult[];
  total: number;
  isLoading: boolean;
  isError: boolean;
}

/**
 * Resolves a `shape: "pie"` widget's per-slice values: one independent
 * `POST {resourceType}/search` per slice — its own `filters` merged under
 * the widget's own base `filters` (slice keys win on conflict) — with
 * `pagination: { limit: 1 }`, reading `total` off each. The exact same
 * mechanism `shape: "count"` (see `useWidgetData`) uses, just fired once
 * per slice instead of once for the whole widget. An empty `slices` array
 * fires no queries at all.
 */
export function useWidgetPieData(
  resourceType: BeWidgetResourceType,
  baseFilters: Record<string, unknown>,
  slices: BeDashboardPieSlice[],
): WidgetPieData {
  const api = useBackendApi();
  const config = WIDGET_RESOURCE_CONFIG[resourceType];

  const queries = useQueries({
    queries: slices.map((slice) => {
      const filters = { ...baseFilters, ...slice.filters };
      return {
        queryKey: [
          ApiQueryKeys.CSM_DASHBOARD_WIDGET_DATA,
          "pie-slice",
          resourceType,
          filters,
        ],
        queryFn: async (): Promise<number> => {
          if (!config) {
            throw new Error(`Unsupported widget resourceType: ${resourceType}`);
          }
          const res = await api.post<
            { filters: Record<string, unknown>; pagination: { offset: number; limit: number } },
            Record<string, unknown>
          >(config.searchEndpoint, {
            filters,
            pagination: { offset: 0, limit: 1 },
          });
          return typeof res.total === "number" ? res.total : 0;
        },
        staleTime: 60_000,
      };
    }),
  });

  const isLoading = queries.some((q) => q.isLoading);
  const isError = queries.some((q) => q.isError);
  const results: PieSliceResult[] = slices.map((slice, i) => ({
    ...slice,
    value: queries[i]?.data ?? 0,
  }));
  const total = results.reduce((sum, s) => sum + s.value, 0);

  return { slices: results, total, isLoading, isError };
}
