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
  BeChangeRequestSearchPayload,
  BeChangeRequestSearchResponse,
  BeChangeRequestSearchView,
} from "@api/backend/types";

/** A single page of matches is plenty for a type-ahead picker. */
const CHANGE_REQUEST_SEARCH_LIMIT = 20;

/**
 * Type-ahead change-request search (`POST /change-requests/search`,
 * `filters.searchQuery`) for the incident edit dialog's "Change request"
 * linking picker. Matches the `(query, enabled) => {data, isFetching,
 * isError}` shape `AsyncEntitySelect` expects — same template as
 * `useSearchGroups`/`useSearchUsersByName`. Fires as soon as the dropdown
 * opens, even with an empty query, so the picker shows a default page
 * instead of looking broken until the caller types something.
 */
export function useSearchChangeRequestsForSelect(
  query: string,
  enabled: boolean,
): UseQueryResult<BeChangeRequestSearchView[], Error> {
  const api = useBackendApi();
  const q = query.trim();

  return useQuery<BeChangeRequestSearchView[], Error>({
    queryKey: [ApiQueryKeys.CHANGE_REQUESTS_SEARCH_FOR_SELECT, q],
    queryFn: async (): Promise<BeChangeRequestSearchView[]> => {
      const res = await api.post<BeChangeRequestSearchPayload, BeChangeRequestSearchResponse>(
        "/change-requests/search",
        { filters: { searchQuery: q }, pagination: { offset: 0, limit: CHANGE_REQUEST_SEARCH_LIMIT } },
      );
      return res.changeRequests ?? [];
    },
    enabled,
    placeholderData: keepPreviousData,
    staleTime: 60_000,
  });
}
