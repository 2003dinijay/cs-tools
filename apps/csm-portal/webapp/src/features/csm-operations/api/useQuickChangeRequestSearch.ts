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
  BeChangeRequestSearchPayload,
  BeChangeRequestSearchResponse,
} from "@api/backend/types";

/** Don't fire a search until the user has typed something searchable. */
export const QUICK_CHANGE_REQUEST_MIN_QUERY_LEN = 2;

/** A small result page — the palette only shows the top few hits. */
const QUICK_CHANGE_REQUEST_LIMIT = 5;

/** One hit from the global-search change-request lookup, enough for the palette's result row. */
export interface QuickChangeRequestHit {
  id: string;
  number?: string;
  subject: string;
  state?: string | null;
  impact?: string | null;
  assigneeName?: string;
  updatedOn?: string;
}

/**
 * Free-text change-request lookup for the global quick-nav palette. Calls
 * `POST /change-requests/search` with the typed text as `searchQuery`
 * (ServiceNow data source only) — same endpoint `useSearchChangeRequests`
 * uses for the Operations tab's listing, just capped to a handful of hits and
 * shaped for the palette instead of a paginated table.
 *
 * Disabled until the trimmed text reaches
 * {@link QUICK_CHANGE_REQUEST_MIN_QUERY_LEN}, so opening the palette costs no
 * network.
 */
export function useQuickChangeRequestSearch(
  query: string,
): UseQueryResult<QuickChangeRequestHit[], Error> {
  const api = useBackendApi();
  const q = query.trim();

  return useQuery<QuickChangeRequestHit[], Error>({
    queryKey: [ApiQueryKeys.CHANGE_REQUESTS, "quick-search", q],
    queryFn: async (): Promise<QuickChangeRequestHit[]> => {
      const res = await api.post<
        BeChangeRequestSearchPayload,
        BeChangeRequestSearchResponse
      >("/change-requests/search", {
        pagination: { offset: 0, limit: QUICK_CHANGE_REQUEST_LIMIT },
        filters: { searchQuery: q },
      });
      return (res.changeRequests ?? []).map((cr) => ({
        id: cr.id,
        number: cr.number,
        subject: cr.subject ?? "(no subject)",
        state: cr.state,
        impact: cr.impact,
        assigneeName: cr.assignedEngineer?.name,
        updatedOn: cr.updatedOn,
      }));
    },
    enabled: q.length >= QUICK_CHANGE_REQUEST_MIN_QUERY_LEN,
    staleTime: 15_000,
  });
}
