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

import { Box, Skeleton } from "@wso2/oxygen-ui";
import { useState, type JSX } from "react";
import AbtDashboardHeader from "@features/csm-dashboard/components/AbtDashboardHeader";
import AgentsLandingPagePilot from "@features/csm-dashboard/components/AgentsLandingPagePilot";
import { useDashboardList } from "@features/csm-dashboard/api/useDashboardList";
import type {
  DashboardKey,
  DashboardScope,
} from "@features/csm-dashboard/types/abtDashboard";

/**
 * Top-level CSM dashboard. The dashboard list and the default selection are
 * BE-driven: `GET /dashboards` populates the switcher in the header (always
 * enabled, see AbtDashboardHeader), and the `isDefault` entry is selected on
 * load. Every dashboard in the registry now has at least one real
 * (config-driven) widget, so this always renders the real widget grid — the
 * earlier mock `DashboardPlaceholder` (pinned KPI numbers per dashboard) is
 * gone.
 */
export default function CsmDashboardPage(): JSX.Element {
  // ABT scoping is not implemented yet, so default to (and stay on)
  // all-customers; the My ABT / All customers toggle is disabled in the header.
  const [scope, setScope] = useState<DashboardScope>("all_customers");
  // Undefined until the switcher is used; until then the selection derives
  // from the loaded list's isDefault entry (see `dashboardKey` below), so
  // there is nothing to synchronize via an effect.
  const [manualDashboardKey, setManualDashboardKey] = useState<
    DashboardKey | undefined
  >(undefined);

  const dashboardList = useDashboardList();
  const list = dashboardList.data;
  const defaultEntry =
    list && list.length > 0 ? (list.find((d) => d.isDefault) ?? list[0]) : undefined;
  const dashboardKey = manualDashboardKey ?? defaultEntry?.id;

  // Only the engineer-overview dashboard is a personal queue (my patches, my
  // reminders, ...); every other dashboard is team/org-wide and has no
  // scope-relevant My ABT / All customers toggle. Not worth a BE field for
  // this single-dashboard UI nuance.
  const scopeBased = dashboardKey === "agents_pilot";

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
        scope={scope}
        onScopeChange={setScope}
        dashboardKey={dashboardKey}
        onDashboardChange={setManualDashboardKey}
        dashboardList={dashboardList.data ?? []}
        scopeBased={scopeBased}
      />
      <AgentsLandingPagePilot dashboardId={dashboardKey} />
    </Box>
  );
}
