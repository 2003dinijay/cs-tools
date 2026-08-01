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

import { Box, Card, Skeleton, Typography } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import { useDashboardWidgets } from "@features/csm-dashboard/api/useDashboardWidgets";
import DashboardWidgetTile from "@features/csm-dashboard/components/DashboardWidgetTile";
import SectionCard from "@features/csm-dashboard/components/SectionCard";
import RefreshButton from "@features/csm-dashboard/components/RefreshButton";

/** Placeholder tile count while the widget template list is in flight. */
const PILOT_TILE_COUNT = 3;

/**
 * Pilot section for the config-driven dashboard widget system (the
 * "agents_pilot" dashboard: 3 `single_score` widgets). The widget template
 * list — display metadata plus each widget's filter criteria — is fetched
 * once via {@link useDashboardWidgets}; each rendered tile then resolves its
 * own data independently. Kept as a clearly separate, labeled add-on below
 * the existing dashboard sections, not a redesign.
 */
export default function AgentsLandingPagePilot(): JSX.Element {
  const { data, isLoading, isError, isFetching, refetch } =
    useDashboardWidgets();

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
      {isError ? (
        <Typography variant="body2" color="text.secondary">
          Could not load the widget pilot.
        </Typography>
      ) : (
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
          {isLoading
            ? Array.from({ length: PILOT_TILE_COUNT }, (_, i) => (
                <Card key={i} variant="outlined" sx={{ p: 1.75 }}>
                  <Skeleton variant="rounded" height={48} />
                </Card>
              ))
            : (data ?? []).map((widget) => (
                <DashboardWidgetTile
                  key={widget.widgetId}
                  widgetId={widget.widgetId}
                  displayName={widget.displayName}
                  filters={widget.filters}
                />
              ))}
        </Box>
      )}
    </SectionCard>
  );
}
