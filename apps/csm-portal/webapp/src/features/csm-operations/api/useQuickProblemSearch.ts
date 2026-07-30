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
  BeProblemSearchPayload,
  BeProblemSearchResponse,
} from "@api/backend/types";

/** Don't fire a search until the user has typed something searchable. */
export const QUICK_PROBLEM_MIN_QUERY_LEN = 2;

/** A small result page — the palette only shows the top few hits. */
const QUICK_PROBLEM_LIMIT = 5;

/** One hit from the global-search problem lookup, enough for the palette's result row. */
export interface QuickProblemHit {
  id: string;
  number?: string;
  subject: string;
  state?: string;
  assigneeName?: string;
}

/**
 * Free-text problem lookup for the global quick-nav palette. Calls
 * `POST /problems/search` with the typed text as `searchQuery` (ServiceNow
 * data source only) — same endpoint `useSearchProblems` uses for the
 * Operations tab's listing, just capped to a handful of hits and shaped for
 * the palette instead of a paginated table.
 *
 * Disabled until the trimmed text reaches {@link QUICK_PROBLEM_MIN_QUERY_LEN},
 * so opening the palette costs no network.
 */
export function useQuickProblemSearch(
  query: string,
): UseQueryResult<QuickProblemHit[], Error> {
  const api = useBackendApi();
  const q = query.trim();

  return useQuery<QuickProblemHit[], Error>({
    queryKey: [ApiQueryKeys.PROBLEMS, "quick-search", q],
    queryFn: async (): Promise<QuickProblemHit[]> => {
      const res = await api.post<BeProblemSearchPayload, BeProblemSearchResponse>(
        "/problems/search",
        {
          pagination: { offset: 0, limit: QUICK_PROBLEM_LIMIT },
          filters: { searchQuery: q },
        },
      );
      return (res.problems ?? []).map((p) => ({
        id: p.id,
        number: p.number,
        subject: p.subject ?? "(no subject)",
        state: p.state,
        assigneeName: p.assignedTo?.name,
      }));
    },
    enabled: q.length >= QUICK_PROBLEM_MIN_QUERY_LEN,
    staleTime: 15_000,
  });
}
