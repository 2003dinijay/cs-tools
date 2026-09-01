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
import { Clock, Plane, Server, ShieldAlert, Users } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import type { BeDashboardWidget } from "@api/backend/types";
import { useDashboard } from "@features/csm-dashboard/api/useDashboard";
import { groupWidgetsBySection, type WidgetGroup } from "@features/csm-dashboard/utils/dashboardWidgetGridLayout";
import WallboardPanel from "@features/csm-dashboard/components/WallboardPanel";
import WallboardCreSection from "@features/csm-dashboard/components/WallboardCreSection";
import WallboardSreSection from "@features/csm-dashboard/components/WallboardSreSection";
import WallboardStatGrid from "@features/csm-dashboard/components/WallboardStatGrid";
import WallboardSecondaryStat from "@features/csm-dashboard/components/WallboardSecondaryStat";
import {
  CS_OVERVIEW_REFETCH_INTERVAL_MS,
  resolveDisplayNameAlias,
  type WallboardSection,
} from "@features/csm-dashboard/utils/wallboardMetricStyle";

export interface WallboardDashboardProps {
  dashboardId: string;
  selectedTeamGroupId?: string | string[];
  selectedTeamLabel?: string;
}

const SECTION_TITLE: Record<WallboardSection, string> = {
  cre: "Customer Reliability Engineering (CRE)",
  sre: "Site Reliability Engineering (SRE)",
  security: "Security Report",
  fde: "Forward Deployed Engineering (FDE)",
};

const SECTION_ICON: Record<WallboardSection, typeof Users> = {
  cre: Users,
  sre: Server,
  security: ShieldAlert,
  fde: Plane,
};

/** Classifies a backend-configured `section` name into one of the four
 * fixed CS Overview families by keyword match, rather than requiring the
 * backend to send an exact literal string — the same tolerant-matching
 * approach `groupWidgetsBySection` already leaves the "section" field free
 * text for. `undefined` for a section that doesn't match any of the four
 * (rendered as a plain fallback grid, never silently dropped). */
function familyFor(sectionName: string | undefined): WallboardSection | undefined {
  if (!sectionName) return undefined;
  const name = sectionName.toLowerCase();
  if (/\bcre\b|customer reliability/.test(name)) return "cre";
  if (/\bsre\b|site reliability/.test(name)) return "sre";
  if (/security/.test(name)) return "security";
  if (/\bfde\b|forward deployed/.test(name)) return "fde";
  return undefined;
}

function LoadingSkeleton(): JSX.Element {
  return (
    <Box sx={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 2 }}>
      {[0, 1, 2, 3].map((i) => (
        <Skeleton key={i} variant="rounded" height={260} sx={{ borderRadius: "16px", bgcolor: "rgba(255,255,255,0.06)" }} />
      ))}
    </Box>
  );
}

function renderFallbackGrid(
  group: WidgetGroup,
  selectedTeamGroupId: string | string[] | undefined,
  selectedTeamLabel: string | undefined,
): JSX.Element {
  return (
    <Box sx={{ display: "grid", gridTemplateColumns: "repeat(3, 1fr)", gap: 0.75 }}>
      {group.widgets.map((widget) => (
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
  );
}

function renderSectionBody(
  family: WallboardSection,
  widgets: BeDashboardWidget[],
  selectedTeamGroupId: string | string[] | undefined,
  selectedTeamLabel: string | undefined,
): JSX.Element {
  switch (family) {
    case "cre":
      return (
        <WallboardCreSection
          widgets={widgets}
          selectedTeamGroupId={selectedTeamGroupId}
          selectedTeamLabel={selectedTeamLabel}
        />
      );
    case "sre":
      return (
        <WallboardSreSection
          widgets={widgets}
          selectedTeamGroupId={selectedTeamGroupId}
          selectedTeamLabel={selectedTeamLabel}
        />
      );
    case "security":
      return (
        <WallboardStatGrid
          widgets={widgets}
          section="security"
          columns={2}
          selectedTeamGroupId={selectedTeamGroupId}
          selectedTeamLabel={selectedTeamLabel}
        />
      );
    case "fde":
      return (
        <WallboardStatGrid
          widgets={widgets}
          section="fde"
          columns={3}
          selectedTeamGroupId={selectedTeamGroupId}
          selectedTeamLabel={selectedTeamLabel}
        />
      );
  }
}

/**
 * The CS Overview dashboard's content — styled to match `digiops-cs`'s
 * `Wallboard.tsx` (see `CS_Dashboard.png`): a 2x2 grid of panels (CRE / SRE
 * / Security Report / FDE), each with its own accent color and internal
 * layout. Data comes from the exact same config-driven `GET
 * /dashboards/{id}` + per-widget `useWidgetData` path every other CSM
 * Portal dashboard already uses — only the rendering is different.
 */
export default function WallboardDashboard({
  dashboardId,
  selectedTeamGroupId,
  selectedTeamLabel,
}: WallboardDashboardProps): JSX.Element {
  const { data, isLoading, isError, dataUpdatedAt } = useDashboard(dashboardId, CS_OVERVIEW_REFETCH_INTERVAL_MS);

  // `dataUpdatedAt` (React Query's own cache timestamp, not component
  // state) — not "track when `data` last changed": React Query's default
  // structural sharing keeps the SAME `data` reference across a refetch
  // that returns identical values, so a `data !== prevData` check would
  // silently skip the timestamp on a real refetch that happened to change
  // nothing. `dataUpdatedAt` updates on every successful fetch regardless,
  // and — living in the shared query cache rather than this component's
  // own state — survives switching away to another dashboard and back
  // (which fully unmounts this component) without going blank in between.
  const lastUpdated = dataUpdatedAt ? new Date(dataUpdatedAt).toLocaleTimeString() : undefined;

  if (isError) {
    return (
      <Typography variant="body2" color="text.secondary">
        Could not load the dashboard.
      </Typography>
    );
  }

  if (isLoading || !data) {
    return <LoadingSkeleton />;
  }

  // Alias resolution runs once here, centrally, before grouping — every
  // downstream component (WallboardCreSection, WallboardSreSection,
  // WallboardStatGrid, the emphasis lookup) then only ever sees the
  // already-canonical displayName, never the raw backend one.
  const aliasedWidgets = data.widgets
    .filter((w) => w.shape === "count")
    .map((w) => ({ ...w, displayName: resolveDisplayNameAlias(w.displayName) }));
  const groups = groupWidgetsBySection(aliasedWidgets);

  return (
    <Box sx={{ bgcolor: "#0f1420", borderRadius: "16px", p: 2 }}>
      <Box sx={{ display: "flex", justifyContent: "flex-end", mb: 1.5 }}>
        <Box
          sx={{
            display: "flex",
            alignItems: "center",
            gap: 0.75,
            fontSize: "0.65rem",
            fontWeight: 600,
            letterSpacing: "0.05em",
            textTransform: "uppercase",
            color: "#cbd5e1",
            bgcolor: "#1f2937",
            px: 1.5,
            py: 0.5,
            borderRadius: "999px",
            border: "1px solid #374151",
          }}
        >
          <Clock size={13} />
          <span>Last updated: {lastUpdated ?? "—"}</span>
          <Box
            sx={{
              width: 6,
              height: 6,
              borderRadius: "50%",
              bgcolor: "#34d399",
              "@keyframes wallboard-live-pulse": {
                "0%, 100%": { opacity: 1 },
                "50%": { opacity: 0.4 },
              },
              animation: "wallboard-live-pulse 1.6s ease-in-out infinite",
            }}
          />
        </Box>
      </Box>
      <Box sx={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 2 }}>
        {(["cre", "sre", "security", "fde"] as const).map((family) => {
          const group = groups.find((g) => familyFor(g.section) === family);
          if (!group) return null;
          return (
            <WallboardPanel key={family} section={family} title={SECTION_TITLE[family]} icon={SECTION_ICON[family]}>
              {renderSectionBody(family, group.widgets, selectedTeamGroupId, selectedTeamLabel)}
            </WallboardPanel>
          );
        })}
        {groups
          .filter((g) => familyFor(g.section) === undefined)
          .map((group, i) => (
            <Box key={group.section ?? `untitled-${i}`} sx={{ gridColumn: "1 / -1" }}>
              {group.section && (
                <Typography sx={{ color: "#cbd5e1", fontSize: "0.8rem", fontWeight: 700, mb: 1 }}>
                  {group.section}
                </Typography>
              )}
              {renderFallbackGrid(group, selectedTeamGroupId, selectedTeamLabel)}
            </Box>
          ))}
      </Box>
    </Box>
  );
}
