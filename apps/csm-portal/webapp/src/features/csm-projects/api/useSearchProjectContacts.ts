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
import { ApiQueryKeys, BE_MAX_PAGE_LIMIT } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import type {
  BeProjectContact,
  BeProjectContactSearchPayload,
  BeProjectContactSearchResponse,
} from "@api/backend/types";

const PAGE_LIMIT = BE_MAX_PAGE_LIMIT;

/**
 * A project's contacts, via `POST /projects/{id}/contacts/search`. Pages
 * through the full list — the response has no `hasMore` flag, so `offset +
 * contacts.length < total` is the continuation check. Disabled until a
 * project id is provided.
 */
export function useSearchProjectContacts(
  projectId: string | undefined,
): UseQueryResult<BeProjectContact[], Error> {
  const api = useBackendApi();

  return useQuery<BeProjectContact[], Error>({
    queryKey: [ApiQueryKeys.PROJECT_CONTACTS, projectId ?? ""],
    queryFn: async (): Promise<BeProjectContact[]> => {
      const all: BeProjectContact[] = [];
      let total = Infinity;
      for (let offset = 0; all.length < total; offset += PAGE_LIMIT) {
        const res = await api.post<
          BeProjectContactSearchPayload,
          BeProjectContactSearchResponse
        >(`/projects/${encodeURIComponent(projectId ?? "")}/contacts/search`, {
          pagination: { offset, limit: PAGE_LIMIT },
        });
        const page = res.contacts ?? [];
        all.push(...page);
        total = res.total ?? all.length;
        if (page.length === 0) break;
      }
      return all;
    },
    enabled: !!projectId,
    staleTime: 60_000,
  });
}
