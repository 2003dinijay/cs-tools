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
import type { BeGroup, BeGroupSearchPayload, BeGroupSearchResponse } from "@api/backend/types";

/** A single page of matches is plenty for a type-ahead picker. */
const GROUP_SEARCH_LIMIT = 20;

/**
 * Whether the entity service behind this backend still accepts
 * `assignableOnly`. It rejects request fields it does not declare (a 400), so a
 * portal deployed before the entity service it talks to must not leave the
 * assignment-group picker empty. Cleared the first time such a rejection is
 * seen, then the flag is simply not sent for the rest of the session (the
 * picker then lists every team again, as it did before the flag existed).
 */
let assignableOnlyAccepted = true;

/** For tests: forget what an earlier request learned about the entity service. */
export function resetAssignableGroupsSupport(): void {
  assignableOnlyAccepted = true;
}

/** A `BackendApiError` carries the HTTP status; matched by shape, as `postSkippingTotal` does. */
function isBadRequest(err: unknown): boolean {
  return err instanceof Error && (err as { status?: unknown }).status === 400;
}

/**
 * The one group search both pickers below share. `assignableOnly` keeps only
 * the groups a record can be assigned to: the team registry this search lists
 * also holds teams added by hand (an approval team, say) that ServiceNow has no
 * group for, and choosing one made creating the change request fail.
 */
function useGroupSearch(
  query: string,
  enabled: boolean,
  assignableOnly: boolean,
): UseQueryResult<BeGroup[], Error> {
  const api = useBackendApi();
  const q = query.trim();

  return useQuery<BeGroup[], Error>({
    queryKey: [ApiQueryKeys.GROUPS_SEARCH, assignableOnly ? "assignable" : "all", q],
    queryFn: async (): Promise<BeGroup[]> => {
      const body = (assignable: boolean): BeGroupSearchPayload => ({
        filters: assignable ? { searchQuery: q, assignableOnly: true } : { searchQuery: q },
        pagination: { offset: 0, limit: GROUP_SEARCH_LIMIT },
      });
      const post = (assignable: boolean) =>
        api.post<BeGroupSearchPayload, BeGroupSearchResponse>("/groups/search", body(assignable));

      if (!assignableOnly || !assignableOnlyAccepted) {
        return (await post(false)).groups ?? [];
      }
      try {
        return (await post(true)).groups ?? [];
      } catch (err) {
        if (!isBadRequest(err)) throw err;
        // Only counts as "unsupported" if the plain retry succeeds, so a request
        // that is simply invalid is reported as the error it is.
        const res = await post(false);
        assignableOnlyAccepted = false;
        return res.groups ?? [];
      }
    },
    enabled,
    placeholderData: keepPreviousData,
    staleTime: 60_000,
  });
}

/**
 * Type-ahead group search (`POST /groups/search`) listing every team. Fires as
 * soon as the dropdown opens, even with an empty query, so the picker shows a
 * default page of groups instead of looking broken until the caller types
 * something. For picking a group to assign a record to, use
 * `useSearchAssignableGroups`.
 */
export function useSearchGroups(
  query: string,
  enabled: boolean,
): UseQueryResult<BeGroup[], Error> {
  return useGroupSearch(query, enabled, false);
}

/**
 * Type-ahead group search for the "Assignment group" picker of the change
 * request create form and edit dialog: only groups a change can be assigned to.
 */
export function useSearchAssignableGroups(
  query: string,
  enabled: boolean,
): UseQueryResult<BeGroup[], Error> {
  return useGroupSearch(query, enabled, true);
}
