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
import type { BeTeamMember } from "@api/backend/types";

/**
 * A team's real CRE/SRE roster via `GET /teams/{id}/members` — `id` must be
 * the real backing UUID (an account's `creTeam.id`/`sreTeam.id`, or the
 * `creGroupId`/`sreGroupId` a registry-matched `BeTeam` row carries), never
 * the registry key `CsmTeamsPage` otherwise addresses a team by. Resolves to
 * `[]` when the id is unknown or has no members (the backend's 404) so
 * callers render an empty state rather than an error.
 */
export function useGetTeamMembers(
  teamId: string | undefined,
): UseQueryResult<BeTeamMember[], Error> {
  const api = useBackendApi();

  return useQuery<BeTeamMember[], Error>({
    queryKey: [ApiQueryKeys.CSM_ADMIN_TEAM_MEMBERS, teamId ?? ""],
    queryFn: async (): Promise<BeTeamMember[]> =>
      (await api.get<BeTeamMember[]>(
        `/teams/${encodeURIComponent(teamId as string)}/members`,
      )) ?? [],
    enabled: !!teamId,
    staleTime: 60_000,
  });
}
