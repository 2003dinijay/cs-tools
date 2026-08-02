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

import { keepPreviousData, useQuery, type UseQueryResult } from "@tanstack/react-query";
import { ApiQueryKeys } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import type {
  BeCaseSearchPayload,
  BeCaseSearchResponse,
  BeCaseSearchView,
} from "@api/backend/types";

/** A single page of matches is plenty for a type-ahead picker. */
const SERVICE_REQUEST_SEARCH_LIMIT = 20;

/**
 * Type-ahead service-request search (`POST /cases/search`,
 * `filters.searchQuery`) for the change-request create form's "Originating
 * service request" picker. Matches the `(query, enabled) => {data,
 * isFetching, isError}` shape `AsyncEntitySelect` expects — same template as
 * `useSearchCasesForSelect`. Pinned to `types: ["service_request"]` since the
 * picker links a change request back to the service request it was raised
 * from, never a plain case.
 */
export function useSearchServiceRequestsForSelect(
  query: string,
  enabled: boolean,
): UseQueryResult<BeCaseSearchView[], Error> {
  const api = useBackendApi();
  const q = query.trim();

  return useQuery<BeCaseSearchView[], Error>({
    queryKey: [ApiQueryKeys.SERVICE_REQUESTS_SEARCH_FOR_SELECT, q],
    queryFn: async (): Promise<BeCaseSearchView[]> => {
      const res = await api.post<BeCaseSearchPayload, BeCaseSearchResponse>(
        "/cases/search",
        {
          filters: {
            searchQuery: q,
            filters: [{ field: "type", op: "in", values: ["service_request"] }],
          },
          pagination: { offset: 0, limit: SERVICE_REQUEST_SEARCH_LIMIT },
        },
      );
      return res.cases ?? [];
    },
    enabled,
    placeholderData: keepPreviousData,
    staleTime: 60_000,
  });
}
