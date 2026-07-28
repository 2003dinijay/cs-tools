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

/**
 * Projects belonging to a given account, for the Account detail page's
 * Projects section. `POST /projects/search` supports a server-side
 * `accountId` filter, so this is a single filtered request rather than a
 * client-side scan-and-filter of the whole catalogue.
 *
 * Only the first `BE_MAX_PAGE_LIMIT` projects are returned — this section has
 * no pager of its own today; an account with more projects than that would
 * need one added.
 */
export function useAccountProjects(
  accountId: string | undefined,
): UseQueryResult<{ projects: Project[] }, Error> {
  const authFetch = useAuthApiClient();

  return useQuery<{ projects: Project[] }, Error>({
    queryKey: [ApiQueryKeys.CSM_ACCOUNT_PROJECTS, accountId ?? ""],
    queryFn: async (): Promise<{ projects: Project[] }> => {
      const request: SearchProjectsRequest = {
        accountId,
        pagination: { limit: BE_MAX_PAGE_LIMIT, offset: 0 },
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
      return { projects: data.projects ?? [] };
    },
    enabled: !!accountId,
    staleTime: 30_000,
  });
}
