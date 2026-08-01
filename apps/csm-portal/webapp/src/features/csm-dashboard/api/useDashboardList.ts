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
import type { BeDashboardListItem } from "@api/backend/types";

/**
 * Every dashboard registered in the config-driven pilot: id, display name,
 * and whether it is the default. A small static registry, not
 * user-configurable — drives the dashboard switcher dropdown
 * (AbtDashboardHeader) and the initial dashboard selection (the `isDefault`
 * entry, see CsmDashboardPage).
 */
export function useDashboardList(): UseQueryResult<
  BeDashboardListItem[],
  Error
> {
  const api = useBackendApi();

  return useQuery<BeDashboardListItem[], Error>({
    queryKey: [ApiQueryKeys.CSM_DASHBOARD_LIST],
    queryFn: async (): Promise<BeDashboardListItem[]> => {
      const res = await api.get<BeDashboardListItem[]>("/dashboards");
      return res ?? [];
    },
    staleTime: 30_000,
  });
}
