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
import { useAuthApiClient } from "@hooks/useAuthApiClient";
import { apiConfig } from "@config/apiConfig";
import { ApiQueryKeys, BE_MAX_PAGE_LIMIT } from "@constants/apiConstants";
import { ApiError, parseApiResponseMessage } from "@utils/ApiError";
import type {
  Project,
  SearchProjectsRequest,
  SearchProjectsResponse,
} from "@features/csm-projects/types/csmProjects";

// `POST /projects/search` has no `accountId` filter (neither the backend nor
// the ServiceNow-backed project-search API it currently proxies supports
// filtering by account — confirmed against the ServiceNow DEV tenant's
// `ProjectsAPI`/`ProjectUtils` scripted API, which only takes
// searchQuery/closureStatus/endDateFrom/endDateTo/sortBy/sortOrder). So this
// scans the project catalogue a page at a time and filters by `accountId`
// client-side, mirroring the existing full-catalogue scan in
// `useProjectOptions` (same MAX_PAGES cap, same ~2,000-project ceiling).
//
// KNOWN GAP: `Project.accountId` is only populated end-to-end on the Postgres
// data source today. The ServiceNow adapter (`sn_project_service.go`) never
// sets it, because the ServiceNow project-search scripted API's list/search
// response omits the (already-loaded, no-extra-query) `account` reference
// field — it is only included in the single-project detail response. Until
// that gap is closed (an additive ServiceNow scripted-API change plus a
// matching `sn_project_service.go` mapping change — both outside this
// frontend's scope), this hook would return an empty list for every account
// on the ServiceNow data source, indistinguishable from a real "no projects"
// result. `isFilterSupported` below exists so callers can tell the two apart
// and avoid rendering a misleadingly authoritative empty state.
//
// NOTE: MAX_PAGES caps the scan at ~2,000 projects (40 pages * BE_MAX_PAGE_LIMIT).
// On an account whose true project belongs to a catalogue larger than that,
// the scan can still miss a real match — this is a pre-existing limitation
// (shared with `useProjectOptions`) and out of scope for this fix; a full
// pagination UI would be needed to close it. `isFilterSupported` is unaffected
// by this cap: it only reflects whether `accountId` was populated on the pages
// we *did* see, not whether the scan was exhaustive, so a cap-truncated scan
// must never be treated as proof the filter is (or isn't) supported.
const MAX_PAGES = 40;

export interface AccountProjectsResult {
  /** Projects matched to this account across the scanned pages. */
  projects: Project[];
  /**
   * Whether the underlying data source actually populates `accountId` on
   * project-search results. `false` means every project seen across the scan
   * omitted `accountId` (the current ServiceNow behavior) — in that case
   * `projects` being empty tells us nothing about whether the account has
   * projects, and callers must not render it as a confirmed "no projects"
   * result. `true` (including the vacuous case where the whole catalogue is
   * empty) means the filter is trustworthy.
   */
  isFilterSupported: boolean;
}

/**
 * All projects belonging to a given account, for the Account detail page's
 * Projects section. Client-side filtered — see the module comment above for
 * why, and its current-data-source limitation.
 */
export function useAccountProjects(
  accountId: string | undefined,
): UseQueryResult<AccountProjectsResult, Error> {
  const authFetch = useAuthApiClient();

  return useQuery<AccountProjectsResult, Error>({
    queryKey: [ApiQueryKeys.CSM_ACCOUNT_PROJECTS, accountId ?? ""],
    queryFn: async (): Promise<AccountProjectsResult> => {
      const matches: Project[] = [];
      let sawAnyProject = false;
      let sawAnyAccountId = false;
      let offset = 0;

      for (let page = 0; page < MAX_PAGES; page += 1) {
        const request: SearchProjectsRequest = {
          pagination: { limit: BE_MAX_PAGE_LIMIT, offset },
        };
        const res = await authFetch(`${apiConfig.backendUrl}/projects/search`, {
          method: "POST",
          body: JSON.stringify(request),
        });
        if (!res.ok) {
          const body = await res.text();
          throw new ApiError(
            res.status,
            res.statusText,
            parseApiResponseMessage(body, res.status, res.statusText),
          );
        }
        const data = (await res.json()) as SearchProjectsResponse;
        const projects = data.projects ?? [];
        if (projects.length > 0) {
          sawAnyProject = true;
          if (projects.some((p) => p.accountId !== undefined)) sawAnyAccountId = true;
        }
        matches.push(...projects.filter((p) => p.accountId === accountId));

        if (!data.hasMore || projects.length === 0) break;
        offset += projects.length;
      }

      // Vacuously "supported" when the catalogue itself is empty — there is
      // nothing to contradict trustworthiness in that case.
      const isFilterSupported = !sawAnyProject || sawAnyAccountId;

      return { projects: matches, isFilterSupported };
    },
    enabled: !!accountId,
    staleTime: 30_000,
  });
}
