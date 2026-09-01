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

import { Box } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import type { BeDashboardWidget } from "@api/backend/types";
import WallboardStatGrid from "@features/csm-dashboard/components/WallboardStatGrid";
import WallboardSecondaryStat from "@features/csm-dashboard/components/WallboardSecondaryStat";
import { CRE_PRIMARY_ORDER, CRE_SECONDARY_ORDER, sortByFixedOrder } from "@features/csm-dashboard/utils/wallboardMetricStyle";

export interface WallboardCreSectionProps {
  widgets: BeDashboardWidget[];
  selectedTeamGroupId?: string | string[];
  selectedTeamLabel?: string;
}

/**
 * CRE's own two-tier layout: a primary 2x2 grid of glow-capable tiles
 * (Open / In-Progress / SLA Violations / Escalations, in that fixed
 * order), then every other CRE widget as a smaller, plain (never
 * glow-capable) tile in a 3-column grid below, in `CRE_SECONDARY_ORDER`'s
 * own fixed order — matching the original's `StatCard` 2x2 grid +
 * `SecondaryStat` 3-column grid.
 */
export default function WallboardCreSection({
  widgets,
  selectedTeamGroupId,
  selectedTeamLabel,
}: WallboardCreSectionProps): JSX.Element {
  const byName = new Map(widgets.map((w) => [w.displayName, w] as const));
  const primary = CRE_PRIMARY_ORDER.map((name) => byName.get(name)).filter(
    (w): w is BeDashboardWidget => w !== undefined,
  );
  const primaryIds = new Set(primary.map((w) => w.widgetId));
  const secondary = sortByFixedOrder(
    widgets.filter((w) => !primaryIds.has(w.widgetId)),
    CRE_SECONDARY_ORDER,
  );

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 1, flex: 1, minHeight: 0 }}>
      <WallboardStatGrid
        widgets={primary}
        section="cre"
        columns={2}
        selectedTeamGroupId={selectedTeamGroupId}
        selectedTeamLabel={selectedTeamLabel}
      />
      <Box sx={{ display: "grid", gridTemplateColumns: "repeat(3, 1fr)", gap: 0.75, flex: 1, minHeight: 0 }}>
        {secondary.map((widget) => (
          <WallboardSecondaryStat
            key={widget.widgetId}
            widgetId={widget.widgetId}
            displayName={widget.displayName}
            resourceType={widget.resourceType}
            filters={widget.query}
            selectedTeamGroupId={selectedTeamGroupId}
            selectedTeamLabel={selectedTeamLabel}
          />
        ))}
      </Box>
    </Box>
  );
}
