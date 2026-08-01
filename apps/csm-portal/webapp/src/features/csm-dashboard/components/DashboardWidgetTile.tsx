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
import { Link as RouterLink } from "react-router";
import type { BeWidgetResourceType, BeWidgetShape } from "@api/backend/types";
import { useWidgetData } from "@features/csm-dashboard/api/useWidgetData";
import { WIDGET_RESOURCE_CONFIG } from "@features/csm-dashboard/config/widgetResourceConfig";

interface DashboardWidgetTileProps {
  widgetId: string;
  displayName: string;
  resourceType: BeWidgetResourceType;
  shape: BeWidgetShape;
  filters: Record<string, unknown>;
  /** Only meaningful for shape "list"; how many rows to render. */
  listLimit?: number;
}

/**
 * Single dashboard widget tile: fetches and renders its own data
 * independently of any sibling tile, so one widget's loading/error state
 * never affects another's. Renders a big number for `shape: "count"`, a
 * compact row list for `shape: "list"`, and is clickable through to the
 * resource's own list page with the widget's filters translated into that
 * page's own URL filter scheme (see `widgetResourceConfig.ts`).
 */
export default function DashboardWidgetTile({
  widgetId,
  displayName,
  resourceType,
  shape,
  filters,
  listLimit,
}: DashboardWidgetTileProps): JSX.Element {
  const { data, isLoading, isError } = useWidgetData(
    widgetId,
    resourceType,
    filters,
    shape,
    listLimit,
  );
  const config = WIDGET_RESOURCE_CONFIG[resourceType];

  if (!config) {
    // resourceType came from a runtime-configurable backend registry (not a
    // compile-time-checked Go literal) — an unrecognized value must not
    // crash this tile's render (config.buildHref below would throw).
    return (
      <Card variant="outlined" sx={{ p: 1.75 }}>
        <Typography variant="caption" color="text.secondary">
          {displayName}
        </Typography>
        <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5 }}>
          Unsupported widget type.
        </Typography>
      </Card>
    );
  }

  const href = config.buildHref(filters);

  return (
    <Card
      variant="outlined"
      component={RouterLink}
      to={href}
      sx={{
        p: 1.75,
        display: "block",
        height: "100%",
        cursor: "pointer",
        color: "inherit",
        textDecoration: "none",
        transition: "background-color 0.15s ease",
        "&:hover": { bgcolor: "action.hover" },
        "&:focus-visible": { outline: "2px solid", outlineColor: "primary.main", outlineOffset: -2 },
      }}
    >
      {isLoading ? (
        <Skeleton variant="rounded" height={48} />
      ) : isError ? (
        <Typography variant="body2" color="text.secondary">
          Could not load this widget.
        </Typography>
      ) : (
        <>
          <Typography variant="caption" color="text.secondary">
            {displayName}
          </Typography>
          {shape === "list" ? (
            <WidgetListBody
              items={data?.items ?? []}
              limit={listLimit ?? 5}
              resourceType={resourceType}
            />
          ) : shape === "count" ? (
            <Typography variant="h5" sx={{ mt: 0.5 }}>
              {data?.total ?? 0}
            </Typography>
          ) : (
            // pie/bar: no aggregate endpoint exists anywhere in the stack
            // today, so there is nothing to resolve or render yet — see
            // `BeWidgetShape`.
            <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5 }}>
              Not yet supported.
            </Typography>
          )}
        </>
      )}
    </Card>
  );
}

interface WidgetListBodyProps {
  items: Record<string, unknown>[];
  limit: number;
  resourceType: BeWidgetResourceType;
}

function WidgetListBody({ items, limit, resourceType }: WidgetListBodyProps): JSX.Element {
  const config = WIDGET_RESOURCE_CONFIG[resourceType];
  const rows = items.slice(0, limit);

  if (rows.length === 0) {
    return (
      <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5 }}>
        No records.
      </Typography>
    );
  }

  return (
    <Box sx={{ mt: 0.5, display: "flex", flexDirection: "column", gap: 0.5 }}>
      {rows.map((item, i) => {
        const secondary = config.secondaryLabel?.(item);
        // Rows have no stable id in this loosely-typed shape; the primary
        // label (usually a record number) is unique enough in practice for a
        // short, non-reorderable list, with the index as a tiebreaker.
        const key = `${config.primaryLabel(item)}-${i}`;
        return (
          <Box key={key} sx={{ minWidth: 0 }}>
            <Typography variant="body2" noWrap>
              {config.primaryLabel(item)}
            </Typography>
            {secondary && (
              <Typography variant="caption" color="text.secondary">
                {secondary}
              </Typography>
            )}
          </Box>
        );
      })}
    </Box>
  );
}
