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
import type { BeTeam } from "@api/backend/types";

interface TeamsSearchPayload {
  pagination: { offset: number; limit: number };
}

interface TeamsSearchResponse {
  teams: BeTeam[];
}

/**
 * Every team from `POST /teams/search`, for the team selector a team-based
 * dashboard shows alongside the dashboard switcher (see
 * `AbtDashboardHeader`). Selecting a team is UI state only today — it does
 * not yet scope any widget's data (see `Dashboard.isTeamBased` on the
 * backend).
 */
export function useTeams(enabled: boolean): UseQueryResult<BeTeam[], Error> {
  const api = useBackendApi();

  return useQuery<BeTeam[], Error>({
    queryKey: [ApiQueryKeys.CSM_TEAMS],
    queryFn: async (): Promise<BeTeam[]> => {
      const res = await api.post<TeamsSearchPayload, TeamsSearchResponse>(
        "/teams/search",
        { pagination: { offset: 0, limit: 100 } },
      );
      return res.teams ?? [];
    },
    enabled,
    staleTime: 5 * 60_000,
  });
}
