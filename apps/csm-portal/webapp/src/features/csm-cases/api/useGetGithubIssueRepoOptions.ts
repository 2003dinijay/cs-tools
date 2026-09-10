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
import type { BeGithubIssueRepoOptionsResponse } from "@api/backend/types";

/**
 * The config-driven "repository" catalogue offered by
 * `CreateGithubIssueDialog`'s repo `Select` (cloud cases only). Replaces what
 * used to be a hardcoded frontend array — see
 * `apps/csm-portal/backend/internal/githubissue/options.go` for why: the
 * hardcoded array carried a wrong Asgardeo owner/repo mapping that misrouted
 * a real filed issue, undetectable from the dropdown label alone.
 *
 * An unconfigured deployment returns an empty array, not an error — the
 * dialog simply has nothing to offer under `showRepoField`, same contract as
 * `useDashboardList`.
 */
export function useGetGithubIssueRepoOptions(): UseQueryResult<
  BeGithubIssueRepoOptionsResponse,
  Error
> {
  const api = useBackendApi();

  return useQuery<BeGithubIssueRepoOptionsResponse, Error>({
    queryKey: [ApiQueryKeys.CSM_GITHUB_ISSUE_REPO_OPTIONS],
    queryFn: async (): Promise<BeGithubIssueRepoOptionsResponse> => {
      const res = await api.get<BeGithubIssueRepoOptionsResponse>(
        "/github-issue-repo-options",
      );
      // Always 200 (empty array when unconfigured) — `null` here means the
      // endpoint itself 404'd (a routing/deployment problem), not "no
      // options configured". Throw so the query enters its error state
      // instead of silently rendering an empty dropdown.
      if (res === null) {
        throw new Error("GET /github-issue-repo-options returned 404");
      }
      return res;
    },
    staleTime: 30_000,
  });
}
