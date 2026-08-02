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

import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { describe, expect, it, vi, beforeEach } from "vitest";
import "@testing-library/jest-dom/vitest";
import { Children, cloneElement, isValidElement, type ReactElement, type ReactNode } from "react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router";

const postMock = vi.fn();

vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock }),
}));
// A `shape: "list"` tile now renders through widgetListConfig.tsx, which
// pulls in useTimeSheets.ts (time_card's mapper) — that module reads
// `window.config` at load via `@config/apiConfig`, unavailable under vitest.
vi.mock("@config/apiConfig", () => ({
  apiConfig: { backendUrl: "https://example.test" },
}));
vi.mock("@context/current-user/CurrentUserContext", () => ({
  useCurrentUser: () => ({
    user: { id: "11111111-aaaa-bbbb-cccc-000000000001" },
    isLoading: false,
    isError: false,
  }),
}));
// Recharts' ResponsiveContainer measures a real layout size, which jsdom
// always reports as 0 — nothing would render. Stubbed to a plain list of
// slice buttons (label + value), enough to assert on data/clicks without
// depending on actual SVG geometry (same approach the customer-portal app's
// own chart tests use for this same package).
vi.mock("@wso2/oxygen-ui-charts-react", () => ({
  ResponsiveContainer: ({ children }: { children: ReactNode }) => <div>{children}</div>,
  PieChart: ({ children }: { children: ReactNode }) => <div>{children}</div>,
  Pie: ({
    data,
    onClick,
  }: {
    data: { name: string; value: number }[];
    onClick?: (item: unknown, index: number) => void;
  }) => (
    <div>
      {data.map((item, i) => (
        <button key={item.name} type="button" onClick={() => onClick?.(item, i)}>
          slice:{item.name}:{item.value}
        </button>
      ))}
    </div>
  ),
  Cell: () => null,
  // `data` is a BarChart-level prop (not Bar's own) in the real package —
  // clone it onto the Bar child so the mock below can read it.
  BarChart: ({
    children,
    data,
  }: {
    children: ReactNode;
    data?: { name: string; value: number }[];
  }) => (
    <div>
      {Children.map(children, (child) =>
        isValidElement(child)
          ? cloneElement(child as ReactElement<{ data?: typeof data }>, { data })
          : child,
      )}
    </div>
  ),
  Bar: ({
    data,
    onClick,
  }: {
    data?: { name: string; value: number }[];
    onClick?: (item: unknown, index: number) => void;
  }) => (
    <div>
      {(data ?? []).map((item, i) => (
        <button key={item.name} type="button" onClick={() => onClick?.(item, i)}>
          bar:{item.name}:{item.value}
        </button>
      ))}
    </div>
  ),
}));

import DashboardWidgetTile from "@features/csm-dashboard/components/DashboardWidgetTile";

function renderWithClient(ui: ReactNode) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>{ui}</MemoryRouter>
    </QueryClientProvider>,
  );
}

function LocationProbe() {
  const location = useLocation();
  return <div data-testid="location-probe">{location.pathname + location.search}</div>;
}

/** For tests that need to observe where a click actually navigated to —
 * `renderWithClient` has no destination route to land on. */
function renderWithRoutes(ui: ReactNode, destinationPath: string) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={["/"]}>
        <Routes>
          <Route path="/" element={ui} />
          <Route path={destinationPath} element={<LocationProbe />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("DashboardWidgetTile", () => {
  beforeEach(() => {
    postMock.mockReset();
  });

  it("renders a skeleton while its own count is in flight", () => {
    postMock.mockReturnValue(new Promise(() => {}));
    const { container } = renderWithClient(
      <DashboardWidgetTile
        widgetId="my_patches"
        displayName="My Patches"
        resourceType="case"
        shape="count"
        filters={{}}
      />,
    );
    expect(container.querySelectorAll(".MuiSkeleton-root").length).toBe(1);
  });

  it("renders the resolved count once its own /cases/search call succeeds", async () => {
    postMock.mockResolvedValue({ total: 3, cases: [], limit: 1, offset: 0, hasMore: false });

    renderWithClient(
      <DashboardWidgetTile
        widgetId="my_patches"
        displayName="My Patches"
        resourceType="case"
        shape="count"
        filters={{}}
      />,
    );

    await waitFor(() => expect(screen.getByText("3")).toBeInTheDocument());
    expect(screen.getByText("My Patches")).toBeInTheDocument();
    expect(postMock).toHaveBeenCalledWith("/cases/search", {
      filters: {},
      pagination: { offset: 0, limit: 1 },
    });
  });

  it("renders its own error state when its /cases/search call fails", async () => {
    postMock.mockRejectedValue(new Error("boom"));

    renderWithClient(
      <DashboardWidgetTile
        widgetId="my_patches"
        displayName="My Patches"
        resourceType="case"
        shape="count"
        filters={{}}
      />,
    );

    await waitFor(() =>
      expect(screen.getByText("Could not load this widget.")).toBeInTheDocument(),
    );
  });

  it("renders the same table the Cases tab uses for shape: list, capped at listLimit", async () => {
    postMock.mockResolvedValue({
      total: 2,
      cases: [
        { id: "11111111-1111-1111-1111-111111111111", number: "CS-1", subject: "Disk full", state: "open" },
        {
          id: "22222222-2222-2222-2222-222222222222",
          number: "CS-2",
          subject: "Auth failing",
          state: "work_in_progress",
        },
      ],
      limit: 5,
      offset: 0,
      hasMore: false,
    });

    renderWithClient(
      <DashboardWidgetTile
        widgetId="my_critical_open"
        displayName="My Critical & High Cases"
        resourceType="case"
        shape="list"
        filters={{}}
        listLimit={5}
      />,
    );

    await waitFor(() => expect(screen.getByText("CS-1")).toBeInTheDocument());
    expect(screen.getByText("Disk full")).toBeInTheDocument();
    expect(screen.getByText("CS-2")).toBeInTheDocument();
    expect(screen.getByText("Auth failing")).toBeInTheDocument();
    expect(postMock).toHaveBeenCalledWith("/cases/search", {
      filters: {},
      pagination: { offset: 0, limit: 5 },
    });
  });

  it("shows a 'View more' link through to the full tab only when more records exist than shown", async () => {
    postMock.mockResolvedValue({
      total: 1,
      cases: [{ id: "11111111-1111-1111-1111-111111111111", number: "CS-1", subject: "Disk full", state: "open" }],
      limit: 5,
      offset: 0,
      hasMore: false,
    });

    renderWithClient(
      <DashboardWidgetTile
        widgetId="my_critical_open"
        displayName="My Critical & High Cases"
        resourceType="case"
        shape="list"
        filters={{}}
        listLimit={5}
      />,
    );

    await waitFor(() => expect(screen.getByText("CS-1")).toBeInTheDocument());
    expect(screen.queryByRole("link", { name: /view more/i })).not.toBeInTheDocument();

    postMock.mockResolvedValue({
      total: 6,
      cases: [{ id: "11111111-1111-1111-1111-111111111111", number: "CS-1", subject: "Disk full", state: "open" }],
      limit: 5,
      offset: 0,
      hasMore: true,
    });

    renderWithClient(
      <DashboardWidgetTile
        widgetId="my_critical_open_2"
        displayName="My Critical & High Cases"
        resourceType="case"
        shape="list"
        filters={{}}
        listLimit={5}
      />,
    );

    const viewMoreLink = await screen.findByRole("link", { name: /view more/i });
    const href = viewMoreLink.getAttribute("href") ?? "";
    // Goes to the widget's own preview page (real, bookmarkable URL — see
    // widgetPreviewUrl.ts), not straight to the resource's own tab.
    expect(href.startsWith("/dashboard/cases?")).toBe(true);
    const params = new URLSearchParams(href.split("?")[1]);
    expect(params.get("w")).toBe("my_critical_open_2");
    expect(params.get("n")).toBe("My Critical & High Cases");
    expect(params.get("f")).toBeNull();
  });

  it("masks the signed-in user's own id in the 'View more' link's filter query params", async () => {
    postMock.mockResolvedValue({
      total: 6,
      cases: [{ id: "11111111-1111-1111-1111-111111111111", number: "CS-1", subject: "Disk full", state: "open" }],
      limit: 5,
      offset: 0,
      hasMore: true,
    });

    renderWithClient(
      <DashboardWidgetTile
        widgetId="my_cases"
        displayName="My Cases"
        resourceType="case"
        shape="list"
        // "11111111-aaaa-bbbb-cccc-000000000001" is the mocked signed-in
        // user's own id (see the CurrentUserContext mock above) — it must
        // never appear verbatim in the resulting URL. Matches the real
        // DASHBOARDS_CONFIG shape: the widget's opaque case filters are the
        // generic field/op/values DSL nested under `filters.filters`.
        filters={{
          filters: [
            {
              field: "assignedUserId",
              op: "in",
              values: ["11111111-aaaa-bbbb-cccc-000000000001"],
            },
          ],
        }}
        listLimit={5}
      />,
    );

    const viewMoreLink = await screen.findByRole("link", { name: /view more/i });
    const href = viewMoreLink.getAttribute("href") ?? "";
    expect(href).not.toContain("11111111-aaaa-bbbb-cccc-000000000001");
    const params = new URLSearchParams(href.split("?")[1]);
    expect(params.get("assignedUserId")).toBe("@me");
  });

  it("navigates to /cases with translated filters when a case-resource tile is clicked", async () => {
    postMock.mockResolvedValue({ total: 3, cases: [], limit: 1, offset: 0, hasMore: false });

    renderWithClient(
      <DashboardWidgetTile
        widgetId="my_patches"
        displayName="My Patches"
        resourceType="case"
        shape="count"
        filters={{
          filters: [
            { field: "severity", op: "in", values: ["critical"] },
            { field: "state", op: "in", values: ["open"] },
          ],
        }}
      />,
    );

    await waitFor(() => expect(screen.getByText("3")).toBeInTheDocument());

    const link = screen.getByRole("link");
    const href = link.getAttribute("href") ?? "";
    expect(href.startsWith("/cases?")).toBe(true);
    const params = new URLSearchParams(href.split("?")[1]);
    expect(params.get("severities")).toBe("S1");
    expect(params.get("states")).toBe("open");
  });

  it("shape bar: issues one search per slice and renders a bar per slice, clickable the same way as a pie slice", async () => {
    postMock.mockImplementation(
      (_path: string, body: { filters: Record<string, unknown> }) => {
        const severities = body.filters.severities as string[] | undefined;
        if (severities?.includes("critical")) return Promise.resolve({ total: 1 });
        if (severities?.includes("high")) return Promise.resolve({ total: 3 });
        return Promise.resolve({ total: 0 });
      },
    );

    renderWithRoutes(
      <DashboardWidgetTile
        widgetId="cases_by_severity"
        displayName="Open Cases by Severity"
        resourceType="case"
        shape="bar"
        filters={{ states: ["open"] }}
        slices={[
          { label: "S1 · Critical", color: "error", filters: { severities: ["critical"] } },
          { label: "S2 · High", color: "warning", filters: { severities: ["high"] } },
        ]}
      />,
      "/cases",
    );

    await waitFor(() => expect(screen.getByText("bar:S1 · Critical:1")).toBeInTheDocument());
    expect(screen.getByText("bar:S2 · High:3")).toBeInTheDocument();
    expect(postMock).toHaveBeenCalledWith("/cases/search", {
      filters: { states: ["open"], severities: ["critical"] },
      pagination: { offset: 0, limit: 1 },
    });

    fireEvent.click(screen.getByText("bar:S1 · Critical:1"));
    await waitFor(() => expect(screen.getByTestId("location-probe")).toBeInTheDocument());
    const probeText = screen.getByTestId("location-probe").textContent ?? "";
    const params = new URLSearchParams(probeText.split("?")[1]);
    expect(params.get("severities")).toBe("S1");
    expect(params.get("states")).toBe("open");
  });

  it("shape pie: issues one search per slice (own filters merged under the widget's base filters) and renders values + percentages", async () => {
    postMock.mockImplementation(
      (_path: string, body: { filters: Record<string, unknown> }) => {
        const severities = body.filters.severities as string[] | undefined;
        if (severities?.includes("critical")) return Promise.resolve({ total: 1 });
        if (severities?.includes("high")) return Promise.resolve({ total: 3 });
        return Promise.resolve({ total: 0 });
      },
    );

    renderWithClient(
      <DashboardWidgetTile
        widgetId="cases-by-severity"
        displayName="Cases by severity"
        description="Share of active cases at each severity level."
        resourceType="case"
        shape="pie"
        filters={{ states: ["open"] }}
        slices={[
          { label: "S1 · Critical", color: "error", filters: { severities: ["critical"] } },
          { label: "S2 · High", color: "warning", filters: { severities: ["high"] } },
        ]}
      />,
    );

    expect(screen.getByText("Share of active cases at each severity level.")).toBeInTheDocument();
    await waitFor(() => expect(screen.getByText("slice:S1 · Critical:1")).toBeInTheDocument());
    expect(screen.getByText("slice:S2 · High:3")).toBeInTheDocument();
    expect(screen.getByText("1 (25%)")).toBeInTheDocument();
    expect(screen.getByText("3 (75%)")).toBeInTheDocument();
    expect(postMock).toHaveBeenCalledWith("/cases/search", {
      filters: { states: ["open"], severities: ["critical"] },
      pagination: { offset: 0, limit: 1 },
    });
    expect(postMock).toHaveBeenCalledWith("/cases/search", {
      filters: { states: ["open"], severities: ["high"] },
      pagination: { offset: 0, limit: 1 },
    });
  });

  it("shape pie: clicking a slice navigates to /cases with the widget's base filters merged under that slice's own filters", async () => {
    postMock.mockResolvedValue({ total: 2 });

    renderWithRoutes(
      <DashboardWidgetTile
        widgetId="cases-by-severity"
        displayName="Cases by severity"
        resourceType="case"
        shape="pie"
        filters={{ states: ["open"] }}
        slices={[{ label: "Critical", filters: { severities: ["critical"] } }]}
      />,
      "/cases",
    );

    await waitFor(() => expect(screen.getByText("slice:Critical:2")).toBeInTheDocument());
    fireEvent.click(screen.getByText("slice:Critical:2"));

    await waitFor(() => expect(screen.getByTestId("location-probe")).toBeInTheDocument());
    const probeText = screen.getByTestId("location-probe").textContent ?? "";
    expect(probeText.startsWith("/cases?")).toBe(true);
    const params = new URLSearchParams(probeText.split("?")[1]);
    expect(params.get("severities")).toBe("S1");
    expect(params.get("states")).toBe("open");
  });

  it("shape pie: clicking a legend row navigates the same way as clicking the slice", async () => {
    postMock.mockResolvedValue({ total: 2 });

    renderWithRoutes(
      <DashboardWidgetTile
        widgetId="cases-by-severity"
        displayName="Cases by severity"
        resourceType="case"
        shape="pie"
        filters={{}}
        slices={[{ label: "Critical", filters: { severities: ["critical"] } }]}
      />,
      "/cases",
    );

    await waitFor(() => expect(screen.getByText("Critical")).toBeInTheDocument());
    fireEvent.click(screen.getByText("Critical"));

    await waitFor(() => expect(screen.getByTestId("location-probe")).toBeInTheDocument());
    const probeText = screen.getByTestId("location-probe").textContent ?? "";
    expect(new URLSearchParams(probeText.split("?")[1]).get("severities")).toBe("S1");
  });

  it("shape pie: renders an empty state (no slices, zero total) rather than crashing when a widget has no slices configured yet", async () => {
    renderWithClient(
      <DashboardWidgetTile
        widgetId="cases-by-severity"
        displayName="Cases by severity"
        resourceType="case"
        shape="pie"
        filters={{}}
      />,
    );

    expect(screen.getByText("Cases by severity")).toBeInTheDocument();
    expect(screen.getByText("Nothing to show here right now")).toBeInTheDocument();
    expect(postMock).not.toHaveBeenCalled();
  });

  it("renders an unsupported-widget message instead of crashing for an unrecognized resourceType", () => {
    renderWithClient(
      <DashboardWidgetTile
        widgetId="mystery_widget"
        displayName="Mystery Widget"
        // Simulates a resourceType the backend registry knows about (now
        // runtime JSON config, not compile-time checked) but this frontend
        // build doesn't yet have an entry for in WIDGET_RESOURCE_CONFIG.
        resourceType={"future_resource" as unknown as never}
        shape="count"
        filters={{}}
      />,
    );

    expect(screen.getByText("Mystery Widget")).toBeInTheDocument();
    expect(screen.getByText("Unsupported widget type.")).toBeInTheDocument();
    expect(postMock).not.toHaveBeenCalled();
  });
});
