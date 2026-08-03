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
  BeItServiceSearchPayload,
  BeItServiceSearchResponse,
} from "@api/backend/types";

const PAGE_LIMIT = BE_MAX_PAGE_LIMIT;

/**
 * Distinct service names for the incidents "Product" filter.
 *
 * Incidents have no product dimension of their own — `productNames` matches
 * against the name of the *service* the incident relates to (see
 * `BeIncidentSearchPayload`), which is only ~43% populated and mixes real
 * products with customer names and service categories. There is no endpoint
 * that enumerates just the names actually used on incidents, so this reuses
 * the same `/services/search` catalogue already backing the create-incident
 * form's "Service" picker ({@link useSearchItServices}) and fetches it in full
 * (bounded, same approach as the cases list's `useProductNameOptions`) rather
 * than requiring the user to already know a name to type-ahead against.
 */
export function useIncidentProductNameOptions(): UseQueryResult<string[], Error> {
  const api = useBackendApi();

  return useQuery<string[], Error>({
    queryKey: [ApiQueryKeys.IT_SERVICE_NAMES],
    queryFn: async (): Promise<string[]> => {
      const names = new Set<string>();
      for (let offset = 0; ; offset += PAGE_LIMIT) {
        const res = await api.post<BeItServiceSearchPayload, BeItServiceSearchResponse>(
          "/services/search",
          { pagination: { offset, limit: PAGE_LIMIT } },
        );
        const page = res.services ?? [];
        for (const s of page) {
          const name = s.name?.trim();
          if (name) names.add(name);
        }
        if (page.length < PAGE_LIMIT) break;
      }
      return [...names].sort((a, b) => a.localeCompare(b));
    },
    staleTime: 5 * 60_000,
  });
}
