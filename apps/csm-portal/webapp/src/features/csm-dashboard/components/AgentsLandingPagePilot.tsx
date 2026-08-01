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
import { useDashboardWidgets } from "@features/csm-dashboard/api/useDashboardWidgets";
import DashboardWidgetTile from "@features/csm-dashboard/components/DashboardWidgetTile";
import SectionCard from "@features/csm-dashboard/components/SectionCard";
import RefreshButton from "@features/csm-dashboard/components/RefreshButton";

/** Placeholder count while the pilot's single shared query is in flight. */
const PILOT_TILE_COUNT = 3;

/**
 * Pilot section for the config-driven dashboard widget system (the
 * "agents_pilot" dashboard: 3 `single_score` widgets resolved by one backend
 * call — see {@link useDashboardWidgets}). Kept as a clearly separate,
 * labeled add-on below the existing dashboard sections, not a redesign.
 */
export default function AgentsLandingPagePilot(): JSX.Element {
  const { data, isLoading, isError, isFetching, refetch } =
    useDashboardWidgets();

  const tiles = data ?? new Array<undefined>(PILOT_TILE_COUNT).fill(undefined);

  return (
    <SectionCard
      title="Widget pilot"
      subtitle="Config-driven dashboard widgets (preview)"
      action={
        <RefreshButton
          onRefresh={() => void refetch()}
          isFetching={isFetching}
          label="Refresh widget pilot"
        />
      }
    >
      <Box
        sx={{
          display: "grid",
          gap: 1.5,
          gridTemplateColumns: {
            xs: "repeat(1, minmax(0, 1fr))",
            sm: "repeat(3, minmax(0, 1fr))",
          },
        }}
      >
        {tiles.map((widget, i) => (
          <DashboardWidgetTile
            key={widget?.widgetId ?? i}
            widget={widget}
            isLoading={isLoading}
            isError={isError}
          />
        ))}
      </Box>
    </SectionCard>
  );
}
