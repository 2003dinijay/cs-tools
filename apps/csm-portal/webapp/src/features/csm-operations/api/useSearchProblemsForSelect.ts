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
  BeProblemSearchPayload,
  BeProblemSearchResponse,
  BeProblemSearchView,
} from "@api/backend/types";

/** A single page of matches is plenty for a type-ahead picker. */
const PROBLEM_SEARCH_LIMIT = 20;

/**
 * Type-ahead problem search (`POST /problems/search`, `filters.searchQuery`)
 * for the incident edit dialog's "Problem" linking picker. Matches the
 * `(query, enabled) => {data, isFetching, isError}` shape `AsyncEntitySelect`
 * expects — same template as `useSearchGroups`/`useSearchUsersByName`. Fires
 * as soon as the dropdown opens, even with an empty query, so the picker
 * shows a default page instead of looking broken until the caller types
 * something.
 */
export function useSearchProblemsForSelect(
  query: string,
  enabled: boolean,
): UseQueryResult<BeProblemSearchView[], Error> {
  const api = useBackendApi();
  const q = query.trim();

  return useQuery<BeProblemSearchView[], Error>({
    queryKey: [ApiQueryKeys.PROBLEMS_SEARCH_FOR_SELECT, q],
    queryFn: async (): Promise<BeProblemSearchView[]> => {
      const res = await api.post<BeProblemSearchPayload, BeProblemSearchResponse>(
        "/problems/search",
        { filters: { searchQuery: q }, pagination: { offset: 0, limit: PROBLEM_SEARCH_LIMIT } },
      );
      return res.problems ?? [];
    },
    enabled,
    placeholderData: keepPreviousData,
    staleTime: 60_000,
  });
}
