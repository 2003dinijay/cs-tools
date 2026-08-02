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

import { Box, Skeleton, Typography } from "@wso2/oxygen-ui";
import { useCallback, useMemo, type JSX } from "react";
import { useSearchParams } from "react-router";
import AbtDashboardHeader from "@features/csm-dashboard/components/AbtDashboardHeader";
import AgentsLandingPagePilot from "@features/csm-dashboard/components/AgentsLandingPagePilot";
import { useDashboardList } from "@features/csm-dashboard/api/useDashboardList";
import type { DashboardKey } from "@features/csm-dashboard/types/abtDashboard";

/** Query param holding the selected dashboard id — kept in the URL so a
 * link to a specific dashboard (and, for team-based ones, a specific team;
 * see `TEAM_PARAM`) is shareable/bookmarkable/refresh-safe, matching the
 * convention in `casesFiltersUrl.ts`. */
const DASHBOARD_PARAM = "dashboard";
/** Query param holding the selected team id, only meaningful (and only
 * ever set) while the current dashboard is `isTeamBased`. */
const TEAM_PARAM = "team";

/**
 * Top-level CSM dashboard. The dashboard list and the default selection are
 * BE-driven: `GET /dashboards` populates the switcher in the header, and the
 * `isDefault` entry is selected on load unless the URL already names a
 * (valid) dashboard. Dashboards are selected purely by dropdown — there is
 * no other per-dashboard scoping control. Every dashboard in the registry
 * has at least one real (config-driven) widget, so this always renders the
 * real widget grid.
 */
export default function CsmDashboardPage(): JSX.Element {
  const [searchParams, setSearchParams] = useSearchParams();

  const dashboardList = useDashboardList();
  const list = dashboardList.data;
  const defaultEntry =
    list && list.length > 0 ? (list.find((d) => d.isDefault) ?? list[0]) : undefined;

  const urlDashboardKey = searchParams.get(DASHBOARD_PARAM) as DashboardKey | null;
  const urlEntry = list?.find((d) => d.id === urlDashboardKey);
  // Fall back to the BE default whenever the URL doesn't name a dashboard,
  // or names one that isn't in the loaded list (stale/hand-edited link) —
  // never crash on an unknown id.
  const dashboardKey = urlEntry ? urlEntry.id : defaultEntry?.id;
  const currentEntry = urlEntry ?? defaultEntry;

  const rawTeamId = searchParams.get(TEAM_PARAM) ?? undefined;
  // Only apply a `team` param when the CURRENT dashboard is team-based — a
  // stale param left over from a previously selected team-based dashboard
  // must not leak into a non-team-based one.
  const selectedTeamId = currentEntry?.isTeamBased ? rawTeamId : undefined;

  const handleDashboardChange = useCallback(
    (key: DashboardKey) => {
      const next = new URLSearchParams(searchParams);
      next.set(DASHBOARD_PARAM, key);
      const nextEntry = list?.find((d) => d.id === key);
      // Switching to a dashboard that isn't team-based: clear any stale
      // `team` param rather than leaving an inapplicable one sitting in
      // the URL.
      if (!nextEntry?.isTeamBased) next.delete(TEAM_PARAM);
      setSearchParams(next, { replace: true });
    },
    [searchParams, setSearchParams, list],
  );

  const handleTeamChange = useCallback(
    (teamId: string | undefined) => {
      const next = new URLSearchParams(searchParams);
      if (teamId) next.set(TEAM_PARAM, teamId);
      else next.delete(TEAM_PARAM);
      setSearchParams(next, { replace: true });
    },
    [searchParams, setSearchParams],
  );

  const dashboardListData = useMemo(() => dashboardList.data ?? [], [dashboardList.data]);

  if (dashboardList.isError) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
        <Typography variant="h5">Dashboard</Typography>
        <Typography variant="body2" color="text.secondary">
          Could not load the dashboard list.
        </Typography>
      </Box>
    );
  }

  if (dashboardKey === undefined) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
        <Skeleton variant="rounded" height={32} width={240} />
        <Skeleton variant="rounded" height={200} />
      </Box>
    );
  }

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <AbtDashboardHeader
        dashboardKey={dashboardKey}
        onDashboardChange={handleDashboardChange}
        dashboardList={dashboardListData}
        selectedTeamId={selectedTeamId}
        onTeamChange={handleTeamChange}
      />
      <AgentsLandingPagePilot dashboardId={dashboardKey} />
    </Box>
  );
}
