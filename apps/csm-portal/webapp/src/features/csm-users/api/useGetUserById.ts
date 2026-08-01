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
import { useBackendApi } from "@api/backend/client";
import {
  normalizeUser,
  type NormalizedUser,
  type SnUser,
} from "@features/csm-users/types/csmUsers";

/**
 * `GET /users/{id}` response. The endpoint is ServiceNow-data-source only, so
 * the row is always the `SnUser` shape; `groups`/`teams`/`projectAccess` are
 * additive detail the profile page doesn't render yet (see
 * `UserProfilePage`'s "not available yet" placeholders — that wiring is a
 * separate, larger piece of work).
 */
type SnUserDetailResponse = SnUser;

/**
 * Look up a single user by id, for the person-profile page. Returns `null`
 * (not an error) for an unknown id, mirroring `useGetCsmCaseDetail`'s
 * not-found handling, so the page can render its own not-found state.
 */
export function useGetUserById(
  id: string | undefined,
): UseQueryResult<NormalizedUser | null, Error> {
  const api = useBackendApi();

  return useQuery<NormalizedUser | null, Error>({
    queryKey: ["csm-user-by-id", id ?? ""],
    queryFn: async (): Promise<NormalizedUser | null> => {
      if (!id) return null;
      const user = await api.get<SnUserDetailResponse>(
        `/users/${encodeURIComponent(id)}`,
      );
      return user ? normalizeUser(user) : null;
    },
    enabled: !!id,
    staleTime: 30_000,
  });
}
