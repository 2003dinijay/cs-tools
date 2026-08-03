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
import { Bar, BarChart, Cell } from "@wso2/oxygen-ui-charts-react";
import { useState, type JSX } from "react";
import type { BeWidgetPaletteColor } from "@api/backend/types";
import type { PieSliceResult } from "@features/csm-dashboard/api/useWidgetPieData";
import { useDarkMode } from "@utils/useDarkMode";

const CHART_HEIGHT_PX = 220;

/** Used in order when a slice's own config omits `color`. */
const DEFAULT_COLOR_ROTATION: BeWidgetPaletteColor[] = [
  "primary",
  "warning",
  "secondary",
  "info",
  "success",
  "error",
];

interface DashboardBarChartProps {
  slices: PieSliceResult[];
  total: number;
  isLoading: boolean;
  isError: boolean;
  onSliceClick: (slice: PieSliceResult) => void;
}

/**
 * Bar chart for a `shape: "bar"` dashboard widget — one bar per slice
 * (`useWidgetPieData` is shape-agnostic: it resolves N labeled slice totals
 * the same way regardless of whether they render as wedges or bars), each
 * clickable through to that slice's own filtered case list, same as
 * `DashboardPieChart`'s wedges/legend rows. A total of 0 renders an empty
 * state instead of an all-grey chart, same treatment as `DashboardPieChart`.
 */
export default function DashboardBarChart({
  slices,
  total,
  isLoading,
  isError,
  onSliceClick,
}: DashboardBarChartProps): JSX.Element {
  const theme = useTheme();
  const isDarkMode = useDarkMode();
  const [activeIndex, setActiveIndex] = useState<number | undefined>(undefined);

  const colorFor = (slice: PieSliceResult, i: number): string => {
    const key = slice.color ?? DEFAULT_COLOR_ROTATION[i % DEFAULT_COLOR_ROTATION.length];
    return theme.palette[key].main;
  };

  if (isLoading) {
    return <Skeleton variant="rounded" height={CHART_HEIGHT_PX} />;
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
          minHeight: CHART_HEIGHT_PX,
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
    <Box sx={{ width: "100%", height: CHART_HEIGHT_PX, "& .recharts-bar-rectangle": { cursor: "pointer" } }}>
      <BarChart
        data={chartData}
        xAxisDataKey="name"
        height="100%"
        width="100%"
        legend={{ show: false }}
        yAxis={{ show: false }}
        grid={{ show: false }}
        // Library default top margin (12px) is too tight once each bar has
        // its own always-visible value label sitting above it.
        margin={{ top: 24, right: 8, bottom: 8, left: 0 }}
        // cursor: false drops the library's default full-height highlight
        // box behind the hovered bar — the bar's own hover color/border is
        // enough of a highlight on its own.
        tooltip={{ show: true, wrapperStyle: { zIndex: 1000 }, cursor: false }}
      >
        <Bar
          dataKey="value"
          radius={[4, 4, 0, 0]}
          isAnimationActive={false}
          label={{
            position: "top",
            fill: isDarkMode ? theme.palette.common.white : theme.palette.text.primary,
            fontSize: 13,
            fontWeight: 600,
          }}
          onMouseEnter={(_data: unknown, i: number) => setActiveIndex(i)}
          onMouseLeave={() => setActiveIndex(undefined)}
          onClick={(_data: unknown, i: number) => {
            const slice = slices[i];
            if (slice) onSliceClick(slice);
          }}
        >
          {slices.map((slice, i) => (
            <Cell
              key={slice.label}
              fill={colorFor(slice, i)}
              stroke={activeIndex === i ? colorFor(slice, i) : "none"}
              strokeWidth={activeIndex === i ? 2 : 0}
            />
          ))}
        </Bar>
      </BarChart>
    </Box>
  );
}
