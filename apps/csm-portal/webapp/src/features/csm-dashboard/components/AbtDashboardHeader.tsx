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

import { Box, FormControl, MenuItem, Select, Typography } from "@wso2/oxygen-ui";
import { useState, type JSX } from "react";
import type { BeDashboardListItem } from "@api/backend/types";
import type { DashboardKey } from "@features/csm-dashboard/types/abtDashboard";
import { useTeams } from "@features/csm-dashboard/api/useTeams";

interface AbtDashboardHeaderProps {
  dashboardKey: DashboardKey;
  onDashboardChange: (key: DashboardKey) => void;
  /** Every dashboard in the BE registry (GET /dashboards), for the switcher. */
  dashboardList: BeDashboardListItem[];
}

/**
 * Dashboard header: title, the dashboard switcher, and (for a dashboard
 * flagged `isTeamBased`) a team selector sourced from `POST /teams/search`.
 * Team selection is UI state only today — it does not yet scope any
 * widget's data (see `Dashboard.isTeamBased` on the backend); wiring a
 * selected team into widget filters is a later increment. The earlier My
 * ABT / All customers toggle was removed entirely — ABT scoping was never
 * implemented and dashboards carry no other special behavior beyond which
 * one (and, for team-based ones, which team) is selected.
 */
export default function AbtDashboardHeader({
  dashboardKey,
  onDashboardChange,
  dashboardList,
}: AbtDashboardHeaderProps): JSX.Element {
  const currentOption = dashboardList.find((o) => o.id === dashboardKey);
  const isTeamBased = currentOption?.isTeamBased ?? false;

  const [selectedTeamId, setSelectedTeamId] = useState<string | undefined>(
    undefined,
  );
  const teams = useTeams(isTeamBased);

  return (
    <Box
      sx={{
        display: "flex",
        alignItems: "center",
        justifyContent: "space-between",
        gap: 2,
        flexWrap: "wrap",
      }}
    >
      <Box>
        <Typography variant="h5">Dashboard</Typography>
        <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5 }}>
          {currentOption?.displayName ?? ""}
        </Typography>
      </Box>
      <Box sx={{ display: "flex", gap: 1, alignItems: "center", flexWrap: "wrap" }}>
        {isTeamBased && (
          <FormControl size="small" sx={{ minWidth: 180 }}>
            <Select
              value={selectedTeamId ?? ""}
              onChange={(e) => setSelectedTeamId(e.target.value || undefined)}
              displayEmpty
              aria-label="Select team"
            >
              <MenuItem value="">
                <em>All teams</em>
              </MenuItem>
              {(teams.data ?? []).map((t) => (
                <MenuItem key={t.id} value={t.id}>
                  {t.name}
                </MenuItem>
              ))}
            </Select>
          </FormControl>
        )}
        <FormControl size="small" sx={{ minWidth: 200 }}>
          <Select
            value={dashboardKey}
            onChange={(e) => onDashboardChange(e.target.value as DashboardKey)}
            displayEmpty
            aria-label="Select dashboard"
          >
            {dashboardList.map((o) => (
              <MenuItem key={o.id} value={o.id}>
                {o.displayName}
              </MenuItem>
            ))}
          </Select>
        </FormControl>
      </Box>
    </Box>
  );
}
