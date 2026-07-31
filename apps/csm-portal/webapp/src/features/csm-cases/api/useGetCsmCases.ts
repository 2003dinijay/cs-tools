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
import { useLogger } from "@hooks/useLogger";
import { useIdTokenClaims } from "@hooks/useIdTokenClaims";
import { ApiQueryKeys } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import {
  ASSIGNEE_FILTER_RESOLVED_EMPTY,
  buildCaseSearchFilters,
  mapCaseSearchViewToRow,
  resolveAssignedUserIds,
} from "@features/csm-cases/utils/caseSearchPayload";
import { ASSIGNEE_ME_TOKEN } from "@features/csm-cases/utils/assignee";
import { useCurrentUser } from "@context/current-user/CurrentUserContext";
import type { BeCaseSearchPayload, BeCaseSearchResponse } from "@api/backend/types";
import type { CasesFilters } from "@features/csm-cases/components/CasesFilterBar";
import type {
  CsmCaseRow,
  CsmCasesListResponse,
} from "@features/csm-cases/types/csmCases";
import {
  DEFAULT_CASES_SORT,
  type CasesSortOrder,
} from "@features/csm-cases/utils/casesSort";

/**
 * Cross-project CSM cases list.
 *
 * Does a single `POST /cases/search` (the flat, cross-project search)
 * and maps each rich `CaseSearchView` — which embeds project / deployment /
 * deployed-product — to the UI `CsmCaseRow`. The list has no Customer column,
 * so the customer (account) name is deliberately not resolved: doing so used to
 * page the entire account directory (`/accounts/search` has no ID filter, and
 * the case search view doesn't embed the account) to fill a field nothing
 * renders. If a Customer column is added, resolve the name from the search view
 * once it carries the account, not by scanning the directory.
 *
 * Search and the severity / state / case-type / project filters are pushed
 * into the search payload (searchQuery / severities / states / types /
 * projectIds) and the BE paginates the result (`pagination` → `total` /
 * `limit` / `offset` / `hasMore`).
 *
 * `page` is zero-based (matching MUI `TablePagination`); `pageSize` is the row
 * limit (≤ the backend's page-size cap, `BE_MAX_PAGE_LIMIT`). Cases are always sorted by `updatedOn`;
 * `sortOrder` (default `"desc"`) controls direction, so the cases page loads
 * the most recently updated cases on arrival by default but can be flipped
 * to oldest-updated-first. `enabled` is an optional escape hatch to suspend
 * the fetch.
 */
export function useGetCsmCases(
  filters: CasesFilters,
  page: number,
  pageSize: number,
  enabled = true,
  sortOrder: CasesSortOrder = DEFAULT_CASES_SORT.order,
): UseQueryResult<CsmCasesListResponse, Error> {
  const logger = useLogger();
  const api = useBackendApi();
  // Signed-in email, to resolve `assigneeIsMe` per row against the assigned
  // engineer's email. In the key so a late-arriving claim recomputes — this
  // applies to every row regardless of the assignee filter, so it stays
  // unconditional.
  const currentUserEmail = useIdTokenClaims()?.email;
  // The caller's platform UUID, fetched once app-wide (CurrentUserProvider)
  // and used only to resolve an `@me` assignee filter (see
  // `resolveAssignedUserIds`) — nothing else reads it. Only fold it into the
  // key when `@me` is actually selected: `/users/me` is a real network call
  // that resolves after the id starts as `undefined`, and keying on it
  // unconditionally meant every page load re-fetched the exact same
  // unfiltered search a second time the moment it arrived, for every user,
  // regardless of whether any assignee filter was active at all.
  const currentUserId = useCurrentUser().user?.id;
  const wantsMe = filters.assignees.includes(ASSIGNEE_ME_TOKEN);

  const offset = page * pageSize;
  const search = filters.search.trim();

  return useQuery<CsmCasesListResponse, Error>({
    // Sort the array filters so selection order doesn't fragment the cache
    // (["S1","S2"] and ["S2","S1"] are the same query). `assignees` holds
    // engineer emails (+ the `@me` sentinel); it's resolved to UUIDs in the
    // queryFn, but keying on the raw selection is enough since resolution is
    // deterministic. `currentUserEmail` is already in the key, covering `@me`.
    queryKey: [
      ApiQueryKeys.CSM_CASES,
      search,
      [...filters.severities].sort(),
      [...filters.states].sort(),
      [...filters.caseTypes].sort(),
      [...filters.workStates].sort(),
      [...filters.assignees].sort(),
      [...filters.projects].sort(),
      [...filters.engagementTypes].sort(),
      [...filters.productNames].sort(),
      currentUserEmail ?? "",
      wantsMe ? (currentUserId ?? "") : "",
      page,
      pageSize,
      sortOrder,
    ],
    queryFn: async (): Promise<CsmCasesListResponse> => {
      // Resolve the assignee filter (engineer emails + the `@me` sentinel) to
      // the UUIDs `/cases/search` expects — shared with the export action via
      // `resolveAssignedUserIds` so both apply the identical assignee filter.
      // A transport failure of the lookup is NOT swallowed — it throws so the
      // query errors (the list shows an error) instead of silently
      // broadening to all cases.
      let assignedUserIds: string[] | undefined;
      if (filters.assignees.length > 0) {
        let resolved: Awaited<ReturnType<typeof resolveAssignedUserIds>>;
        try {
          resolved = await resolveAssignedUserIds(api, filters.assignees, currentUserId);
        } catch (err) {
          logger.warn(
            `[useGetCsmCases] assignee lookup failed: ${(err as Error).message}`,
          );
          throw new Error("Failed to resolve the assignee filter");
        }
        // Active assignee filter that resolved to nothing → empty result, not
        // a broadened (filter-less) search. See `resolveAssignedUserIds`.
        if (resolved === ASSIGNEE_FILTER_RESOLVED_EMPTY) {
          return { cases: [], total: 0, limit: pageSize, offset, hasMore: false };
        }
        assignedUserIds = resolved;
      }

      // One cross-project case search. No account/project directory scan: the
      // list has no Customer column, and the old scan paged the entire account
      // directory to resolve a name nothing renders.
      const casesResponse = await api.post<
        BeCaseSearchPayload,
        BeCaseSearchResponse
      >("/cases/search", {
          pagination: { offset, limit: pageSize },
          sortBy: { field: "updatedOn", order: sortOrder },
          filters: buildCaseSearchFilters(filters, search, assignedUserIds),
      });

      const cases: CsmCaseRow[] = (casesResponse.cases ?? []).map((c) =>
        mapCaseSearchViewToRow(c, currentUserEmail),
      );

      return {
        cases,
        total: casesResponse.total ?? cases.length,
        limit: casesResponse.limit ?? pageSize,
        offset: casesResponse.offset ?? offset,
        hasMore: casesResponse.hasMore ?? false,
      };
    },
    enabled,
    staleTime: 30_000,
  });
}
