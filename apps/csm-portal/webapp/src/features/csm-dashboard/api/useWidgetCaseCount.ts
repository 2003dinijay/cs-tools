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

import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import { ApiQueryKeys } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import type {
  BeCaseSearchFilters,
  BeCaseSearchPayload,
  BeCaseSearchResponse,
} from "@api/backend/types";

/**
 * One dashboard widget's resolved count, fetched independently of any other
 * widget on the same dashboard: a single `POST /cases/search` with the
 * widget's own filters, `limit: 1`, reading `total` off the response (same
 * count-only pattern as `useCaseCountsMatrix`).
 */
export function useWidgetCaseCount(
  widgetId: string,
  filters: BeCaseSearchFilters,
): UseQueryResult<number, Error> {
  const api = useBackendApi();

  return useQuery<number, Error>({
    queryKey: [ApiQueryKeys.CSM_DASHBOARD_WIDGETS, widgetId, filters],
    queryFn: async (): Promise<number> => {
      const res = await api.post<BeCaseSearchPayload, BeCaseSearchResponse>(
        "/cases/search",
        {
          filters,
          pagination: { offset: 0, limit: 1 },
        },
      );
      return res.total ?? 0;
    },
    staleTime: 60_000,
  });
}
