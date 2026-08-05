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

import { Box, Skeleton, Typography, alpha, useTheme } from "@wso2/oxygen-ui";
import { Inbox } from "@wso2/oxygen-ui-icons-react";
import { Cell, Pie, PieChart } from "@wso2/oxygen-ui-charts-react";
import { useState, type JSX, type KeyboardEvent, type SyntheticEvent } from "react";
import type { BeWidgetPaletteColor } from "@api/backend/types";
import type { PieSliceResult } from "@features/csm-dashboard/api/useWidgetPieData";
import { useDarkMode } from "@utils/useDarkMode";

const CHART_SIZE_PX = 180;
const INNER_RADIUS_PX = 62;
const OUTER_RADIUS_PX = 88;

/** Used in order when a slice's own config omits `color`. */
const DEFAULT_COLOR_ROTATION: BeWidgetPaletteColor[] = [
  "primary",
  "warning",
  "secondary",
  "info",
  "success",
  "error",
];

interface DashboardPieChartProps {
  slices: PieSliceResult[];
  total: number;
  isLoading: boolean;
  isError: boolean;
  onSliceClick: (slice: PieSliceResult) => void;
}

/**
 * Donut chart + legend for a `shape: "pie"` dashboard widget — a fixed
 * center total, one wedge (and one legend row) per slice, both clickable
 * through to that slice's own filtered case list (see
 * `DashboardWidgetTile`'s pie branch for how the destination href is
 * built). A total of 0 renders an empty state (same treatment as the
 * customer-portal app's own dashboard charts) instead of an all-grey ring.
 */
export default function DashboardPieChart({
  slices,
  total,
  isLoading,
  isError,
  onSliceClick,
}: DashboardPieChartProps): JSX.Element {
  const theme = useTheme();
  const isDarkMode = useDarkMode();
  const [activeIndex, setActiveIndex] = useState<number | undefined>(undefined);

  const colorFor = (slice: PieSliceResult, i: number): string => {
    const key = slice.color ?? DEFAULT_COLOR_ROTATION[i % DEFAULT_COLOR_ROTATION.length];
    return theme.palette[key].main;
  };

  if (isLoading) {
    return (
      <Box sx={{ display: "flex", alignItems: "center", gap: 3 }}>
        <Skeleton variant="circular" width={CHART_SIZE_PX} height={CHART_SIZE_PX} sx={{ flexShrink: 0 }} />
        <Box sx={{ display: "flex", flexDirection: "column", gap: 1, flex: 1 }}>
          {slices.map((slice) => (
            <Skeleton key={slice.label} variant="rounded" height={20} />
          ))}
        </Box>
      </Box>
    );
  }

  if (isError) {
    return (
      <Typography variant="body2" color="text.secondary">
        Could not load this widget.
      </Typography>
    );
  }

  if (total === 0) {
    return (
      <Box
        sx={{
          minHeight: CHART_SIZE_PX,
          width: "100%",
          display: "flex",
          flexDirection: "column",
          alignItems: "center",
          justifyContent: "center",
          gap: 1.5,
        }}
      >
        <Box
          sx={{
            width: 52,
            height: 52,
            borderRadius: "50%",
            bgcolor: alpha(theme.palette.grey[500], 0.08),
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
          }}
        >
          {/* text.disabled/text.secondary render too close to the
              background's own color in this app's dark theme (same issue
              fixed for the bar chart's value labels) — grey[400] is legible
              against a dark background without looking harsh in light mode. */}
          <Inbox size={24} color={isDarkMode ? theme.palette.grey[400] : theme.palette.grey[500]} />
        </Box>
        <Typography variant="body2" sx={{ color: isDarkMode ? theme.palette.grey[400] : theme.palette.text.disabled }}>
          Nothing to show here right now
        </Typography>
      </Box>
    );
  }

  const chartData = slices.map((slice) => ({ name: slice.label, value: slice.value }));

  return (
    <Box sx={{ display: "flex", alignItems: "center", gap: 3, flexWrap: "wrap" }}>
      <Box
        sx={{
          position: "relative",
          width: CHART_SIZE_PX,
          height: CHART_SIZE_PX,
          flexShrink: 0,
          "& .recharts-pie-sector": { cursor: "pointer" },
        }}
      >
        {/* wrapperStyle's zIndex matches the same fix the customer-portal
            app's own dashboard charts use (see OutstandingIncidentsChart.tsx)
            — without it the tooltip can render underneath/get clipped by
            this tile's own header, looking cut off / see-through.
            margin: PieChart's own default is {top:12,right:24,left:24,
            bottom:40} — the large bottom value reserves room for a legend,
            which is disabled here. Left uncleared, that asymmetric margin
            shifts the plot's (and so the ring's) center downward within the
            180x180 box, making the top of the donut look pushed toward /
            cut off by this tile's own header above it. */}
        <PieChart
          // Explicit pixel size, not "100%": the parent Box just above is
          // already a fixed CHART_SIZE_PX square, not a responsive/percentage
          // container, so there's nothing for a percentage width/height to
          // measure against except an internal ResizeObserver callback --
          // that callback fires one tick after first paint, so the chart's
          // very first render sees width/height as -1 (a real, harmless but
          // noisy console warning: "The width(-1) and height(-1) of chart
          // should be greater than 0"). Passing the known constant directly
          // sizes it synchronously on the first render, no observer race.
          width={CHART_SIZE_PX}
          height={CHART_SIZE_PX}
          legend={{ show: false }}
          margin={{ top: 0, right: 0, bottom: 0, left: 0 }}
          tooltip={{ show: true, wrapperStyle: { zIndex: 1000 } }}
        >
          <Pie
            data={chartData}
            cx="50%"
            cy="50%"
            innerRadius={INNER_RADIUS_PX}
            outerRadius={OUTER_RADIUS_PX}
            paddingAngle={0}
            dataKey="value"
            nameKey="name"
            startAngle={90}
            endAngle={-270}
            label={false}
            labelLine={false}
            onMouseEnter={(_data: unknown, i: number) => setActiveIndex(i)}
            onMouseLeave={() => setActiveIndex(undefined)}
            // Stops the click from also bubbling up to the tile-level
            // click-through `DashboardWidgetTile` attaches to the whole
            // card for `shape: "pie"`/`"bar"` — without this, clicking a
            // wedge would navigate to the slice's own filtered list AND
            // then (via bubbling) immediately re-navigate to the tile's
            // base-filtered list.
            onClick={(_data: unknown, i: number, event?: SyntheticEvent) => {
              event?.stopPropagation();
              const slice = slices[i];
              if (slice) onSliceClick(slice);
            }}
          >
            {slices.map((slice, i) => (
              <Cell
                key={slice.label}
                fill={colorFor(slice, i)}
                stroke={activeIndex === i ? colorFor(slice, i) : "none"}
                strokeWidth={activeIndex === i ? 3 : 0}
              />
            ))}
          </Pie>
        </PieChart>
        <Box
          sx={{
            position: "absolute",
            inset: 0,
            display: "flex",
            flexDirection: "column",
            alignItems: "center",
            justifyContent: "center",
            pointerEvents: "none",
          }}
        >
          <Typography variant="h5">{total}</Typography>
          <Typography variant="caption" color="text.secondary">
            Total
          </Typography>
        </Box>
      </Box>

      <Box sx={{ display: "flex", flexDirection: "column", gap: 0.75, flex: 1, minWidth: 180 }}>
        {slices.map((slice, i) => {
          const pct = total > 0 ? Math.round((slice.value / total) * 100) : 0;
          return (
            <Box
              key={slice.label}
              role="button"
              tabIndex={0}
              aria-label={`${slice.label}: ${slice.value} cases (${pct}%)`}
              // stopPropagation for the same reason as the wedge's own
              // onClick above — this row sits inside the tile-level
              // click-through `DashboardWidgetTile` attaches for
              // shape "pie"/"bar".
              onClick={(event) => {
                event.stopPropagation();
                onSliceClick(slice);
              }}
              onKeyDown={(event: KeyboardEvent) => {
                if (event.key === "Enter" || event.key === " ") {
                  event.preventDefault();
                  event.stopPropagation();
                  onSliceClick(slice);
                }
              }}
              sx={{
                display: "flex",
                alignItems: "center",
                gap: 1,
                px: 0.5,
                py: 0.25,
                borderRadius: 1,
                cursor: "pointer",
                "&:hover": { bgcolor: "action.hover" },
                "&:focus-visible": {
                  outline: `2px solid ${theme.palette.primary.main}`,
                  outlineOffset: -2,
                },
              }}
            >
              <Box
                sx={{
                  width: 10,
                  height: 10,
                  borderRadius: "50%",
                  bgcolor: colorFor(slice, i),
                  flexShrink: 0,
                }}
              />
              <Typography variant="body2" sx={{ flex: 1 }} noWrap>
                {slice.label}
              </Typography>
              <Typography variant="body2" sx={{ fontWeight: 600 }}>
                {slice.value} ({pct}%)
              </Typography>
            </Box>
          );
        })}
      </Box>
    </Box>
  );
}
