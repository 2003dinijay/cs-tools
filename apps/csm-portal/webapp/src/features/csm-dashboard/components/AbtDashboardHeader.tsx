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
import type { JSX } from "react";
import type { BeDashboardListItem } from "@api/backend/types";
import type { DashboardKey } from "@features/csm-dashboard/types/abtDashboard";

interface AbtDashboardHeaderProps {
  dashboardKey: DashboardKey;
  onDashboardChange: (key: DashboardKey) => void;
  /** Every dashboard in the BE registry (GET /dashboards), for the switcher. */
  dashboardList: BeDashboardListItem[];
}

/**
 * Dashboard header: title plus the dashboard switcher. Dashboards are
 * selected purely by dropdown — there is no other per-dashboard scoping
 * control (the earlier My ABT / All customers toggle was removed; ABT
 * scoping was never implemented and dashboards carry no other special
 * per-dashboard behavior beyond which one is selected).
 */
export default function AbtDashboardHeader({
  dashboardKey,
  onDashboardChange,
  dashboardList,
}: AbtDashboardHeaderProps): JSX.Element {
  const currentOption = dashboardList.find((o) => o.id === dashboardKey);

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
      <FormControl size="small" sx={{ minWidth: 200 }}>
        <Select
          value={dashboardKey}
          onChange={(e) => onDashboardChange(e.target.value as DashboardKey)}
          displayEmpty
        >
          {dashboardList.map((o) => (
            <MenuItem key={o.id} value={o.id}>
              {o.displayName}
            </MenuItem>
          ))}
        </Select>
      </FormControl>
    </Box>
  );
}
