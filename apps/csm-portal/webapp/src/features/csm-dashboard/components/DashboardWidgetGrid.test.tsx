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

import { fireEvent, render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import { MemoryRouter } from "react-router";
import type { ReactNode } from "react";
import type { BeDashboardWidget } from "@api/backend/types";
import type { PieSliceResult } from "@features/csm-dashboard/api/useWidgetPieData";

// `DashboardWidgetGrid` now imports `WidgetInlineDrilldownPanel` directly
// (unmocked, its own real module graph reaches `widgetListConfig.tsx`,
// which pulls in `useTimeSheets.ts` — time_card's mapper — which reads
// `window.config` at load via `@config/apiConfig`, unavailable under
// vitest). See `DashboardWidgetTile.test.tsx`'s identical mock/comment.
vi.mock("@config/apiConfig", () => ({
  apiConfig: { backendUrl: "https://example.test" },
}));

// Stubs the real tile out entirely — this test is only about
// `DashboardWidgetGrid`'s own wiring of `hideRefreshButton` alongside
// `renderWidgetAction`, not about anything the real tile fetches/renders.
// Exposes both as plain text so the assertions below can read them straight
// out of the DOM rather than needing a spy. Also exposes `expandedSlice`/
// `onExpandChange` as a plain button so this file's own tests can assert on
// the LIFTED expand/collapse behavior `DashboardWidgetGrid` now owns,
// without depending on the real chart/slice-click machinery
// `DashboardWidgetTile.test.tsx` already covers in full.
vi.mock("@features/csm-dashboard/components/DashboardWidgetTile", () => ({
  default: ({
    widgetId,
    hideRefreshButton,
    expandedSlice,
    onExpandChange,
  }: {
    widgetId: string;
    hideRefreshButton?: boolean;
    expandedSlice?: PieSliceResult | null;
    onExpandChange?: (slice: PieSliceResult | null) => void;
  }) => (
    <div data-testid={`tile-${widgetId}`}>
      {!hideRefreshButton && (
        <button type="button" aria-label={`Refresh ${widgetId}`}>
          refresh
        </button>
      )}
      {onExpandChange && (
        <button
          type="button"
          aria-label={`Expand a slice on ${widgetId}`}
          onClick={() =>
            onExpandChange(
              expandedSlice
                ? null
                : { label: `${widgetId}-slice`, value: 1, query: {} },
            )
          }
        >
          toggle slice
        </button>
      )}
    </div>
  ),
}));

// Stubs the real panel out too — this file only asserts on WHERE/WHEN it
// renders (full-width sibling, singular across the grid), not on its own
// data-fetching/rendering, which `WidgetInlineDrilldownPanel.test.tsx`
// covers directly.
vi.mock("@features/csm-dashboard/components/WidgetInlineDrilldownPanel", () => ({
  default: ({
    widgetId,
    slice,
    onClose,
  }: {
    widgetId: string;
    slice: PieSliceResult;
    onClose: () => void;
  }) => (
    <div data-testid={`panel-${widgetId}`}>
      <span>{slice.label}</span>
      <button type="button" aria-label={`Close panel for ${widgetId}`} onClick={onClose}>
        close
      </button>
    </div>
  ),
}));

import DashboardWidgetGrid from "@features/csm-dashboard/components/DashboardWidgetGrid";

function renderGrid(
  widgets: BeDashboardWidget[],
  renderWidgetAction?: (widget: BeDashboardWidget) => ReactNode,
) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <DashboardWidgetGrid widgets={widgets} renderWidgetAction={renderWidgetAction} />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

function makeWidget(overrides: Partial<BeDashboardWidget> = {}): BeDashboardWidget {
  return {
    widgetId: "my_patches",
    displayName: "My Patches",
    resourceType: "case",
    shape: "count",
    gridWidth: 3,
    query: {},
    ...overrides,
  } as BeDashboardWidget;
}

describe("DashboardWidgetGrid", () => {
  it("renders every tile's own refresh button as before when no renderWidgetAction is passed (live dashboard, unaffected)", () => {
    renderGrid([makeWidget()]);

    expect(screen.getByRole("button", { name: "Refresh my_patches" })).toBeInTheDocument();
  });

  it("suppresses a widget's own refresh button (and renders the builder action instead) when renderWidgetAction returns a non-null action for it", () => {
    renderGrid([makeWidget()], (widget) => (
      <div>
        <button type="button" aria-label={`Edit ${widget.widgetId}`}>
          edit
        </button>
        <button type="button" aria-label={`Remove ${widget.widgetId}`}>
          remove
        </button>
      </div>
    ));

    // The builder's own Edit/Remove actions render...
    expect(screen.getByRole("button", { name: "Edit my_patches" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Remove my_patches" })).toBeInTheDocument();
    // ...and the tile's own refresh button is suppressed, so there's no
    // longer any overlap for the two to fight over in the same corner.
    expect(
      screen.queryByRole("button", { name: "Refresh my_patches" }),
    ).not.toBeInTheDocument();
  });

  it("keeps a widget's own refresh button when renderWidgetAction returns nothing for that specific widget", () => {
    renderGrid(
      [makeWidget({ widgetId: "no_action_widget" })],
      () => null,
    );

    expect(
      screen.getByRole("button", { name: "Refresh no_action_widget" }),
    ).toBeInTheDocument();
  });

  it("renders a section's widgets in the config's own array order, not grouped/reordered by shape", () => {
    // Deliberately lists a bar-shape widget ahead of a list-shape widget
    // within the same (default/untitled) section — the old implementation
    // hardcoded every list/count tile ahead of every pie/bar tile
    // regardless of array position, so reordering this config array had no
    // visible effect. This asserts the fix: DOM order follows array order.
    renderGrid([
      makeWidget({ widgetId: "trend_widget", shape: "bar", groupBy: { field: "status" } }),
      makeWidget({ widgetId: "list_widget", shape: "list" }),
      makeWidget({ widgetId: "count_widget", shape: "count" }),
    ]);

    const tileIds = screen
      .getAllByTestId(/^tile-/)
      .map((el) => el.getAttribute("data-testid"));

    expect(tileIds).toEqual(["tile-trend_widget", "tile-list_widget", "tile-count_widget"]);
  });

  it("expands a widget's slice into a full-width panel rendered as a sibling of that widget's own tile, not nested inside it", () => {
    renderGrid([makeWidget({ widgetId: "cases_by_severity", shape: "pie" })]);

    expect(screen.queryByTestId("panel-cases_by_severity")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Expand a slice on cases_by_severity" }));

    const panel = screen.getByTestId("panel-cases_by_severity");
    expect(panel).toBeInTheDocument();
    expect(screen.getByText("cases_by_severity-slice")).toBeInTheDocument();
    // A sibling of the tile's own wrapper, not a descendant of it.
    const tile = screen.getByTestId("tile-cases_by_severity");
    expect(tile.contains(panel)).toBe(false);
  });

  it("collapses the panel when the tile reports null (e.g. clicking the same slice again, or the panel's own close control)", () => {
    renderGrid([makeWidget({ widgetId: "cases_by_severity", shape: "pie" })]);

    const toggle = screen.getByRole("button", { name: "Expand a slice on cases_by_severity" });
    fireEvent.click(toggle);
    expect(screen.getByTestId("panel-cases_by_severity")).toBeInTheDocument();

    fireEvent.click(toggle);
    expect(screen.queryByTestId("panel-cases_by_severity")).not.toBeInTheDocument();
  });

  it("closing the panel via its own onClose collapses it", () => {
    renderGrid([makeWidget({ widgetId: "cases_by_severity", shape: "pie" })]);

    fireEvent.click(screen.getByRole("button", { name: "Expand a slice on cases_by_severity" }));
    expect(screen.getByTestId("panel-cases_by_severity")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Close panel for cases_by_severity" }));
    expect(screen.queryByTestId("panel-cases_by_severity")).not.toBeInTheDocument();
  });

  it("expanding a different widget's slice replaces the previously expanded panel — only one panel is ever open at a time", () => {
    renderGrid([
      makeWidget({ widgetId: "widget_a", shape: "pie" }),
      makeWidget({ widgetId: "widget_b", shape: "pie" }),
    ]);

    fireEvent.click(screen.getByRole("button", { name: "Expand a slice on widget_a" }));
    expect(screen.getByTestId("panel-widget_a")).toBeInTheDocument();
    expect(screen.queryByTestId("panel-widget_b")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Expand a slice on widget_b" }));
    expect(screen.queryByTestId("panel-widget_a")).not.toBeInTheDocument();
    expect(screen.getByTestId("panel-widget_b")).toBeInTheDocument();
  });
});
