/**
 * Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { keepPreviousData, useQueries } from "@tanstack/react-query";
import { useBackendApi } from "@api/backend/client";
import type {
  EngagementAllocation,
  EngagementAllocationsResponse,
  SearchEngagementAllocationsPayload,
} from "../types";
import { chunk, splitWindow } from "../utils/engagementAllocations";

/** A read of who is at a customer; it changes when somebody confirms one. */
const ALLOCATIONS_STALE_MS = 60_000;

/** What the schedule reads of the allocations: the bars, and whether some
 *  piece of the read failed. A failure never takes the bars that did load. */
export interface EngagementAllocationsRead {
  allocations: EngagementAllocation[];
  isLoading: boolean;
  isError: boolean;
}

/**
 * Confirmed customer engagement allocations for a set of engineers over a
 * date window, read-only.
 *
 * The read takes at most 500 user ids and 70 days, so the ids are cut into
 * batches and a longer window into pieces, one request per batch per piece.
 * Each is cached under its own key (ids and window), so paging the window only
 * fetches the piece that came into view. Pieces that fail leave out their own
 * bars and nothing else: this read is decoration on the schedule, and the
 * schedule must not blank or break because it failed.
 *
 * `userIds` should be the distinct ids of the people on screen (see
 * distinctUserIds). With none there is nothing to ask and no request is made.
 */
export function useEngagementAllocations(
  userIds: readonly string[],
  from: string,
  to: string,
  enabled = true,
): EngagementAllocationsRead {
  const api = useBackendApi();
  const batches = chunk(userIds);
  const windows = splitWindow(from, to);
  const payloads: SearchEngagementAllocationsPayload[] = batches.flatMap((ids) =>
    windows.map((w) => ({ userIds: ids, from: w.from, to: w.to })),
  );
  return useQueries({
    queries: payloads.map((p) => ({
      queryKey: ["team-schedule", "engagement-allocations", p] as const,
      queryFn: () =>
        api.post<SearchEngagementAllocationsPayload, EngagementAllocationsResponse>(
          "/team-schedule/engagement-allocations/search",
          p,
        ),
      enabled,
      placeholderData: keepPreviousData,
      staleTime: ALLOCATIONS_STALE_MS,
    })),
    combine: (results): EngagementAllocationsRead => {
      const byId = new Map<string, EngagementAllocation>();
      for (const r of results) for (const a of r.data?.allocations ?? []) byId.set(a.id, a);
      return {
        allocations: [...byId.values()],
        isLoading: results.some((r) => r.isLoading),
        isError: results.some((r) => r.isError),
      };
    },
  });
}
