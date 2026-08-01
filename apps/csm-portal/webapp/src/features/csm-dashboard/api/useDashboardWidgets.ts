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
import type { BeDashboardWidget } from "@api/backend/types";

/** Dashboard id for the config-driven widget pilot (3 widgets, all `single_score`). */
export const AGENTS_PILOT_DASHBOARD_ID = "agents_pilot";

/**
 * Resolved widgets for the "agents_pilot" dashboard.
 *
 * All widgets on a dashboard resolve in one backend call (not one request per
 * widget) so the pattern scales to dozens of widgets without request
 * fan-out — see `GET /dashboards/{dashboardId}/widgets`. Callers render one
 * tile per entry in the returned array; every tile shares this single query's
 * loading/error state.
 */
export function useDashboardWidgets(): UseQueryResult<
  BeDashboardWidget[],
  Error
> {
  const api = useBackendApi();

  return useQuery<BeDashboardWidget[], Error>({
    queryKey: [ApiQueryKeys.CSM_DASHBOARD_WIDGETS, AGENTS_PILOT_DASHBOARD_ID],
    queryFn: async (): Promise<BeDashboardWidget[]> => {
      const res = await api.get<BeDashboardWidget[]>(
        `/dashboards/${AGENTS_PILOT_DASHBOARD_ID}/widgets`,
      );
      return res ?? [];
    },
    staleTime: 30_000,
  });
}
