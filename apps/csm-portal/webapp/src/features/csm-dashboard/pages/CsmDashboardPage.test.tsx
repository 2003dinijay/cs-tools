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

import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import "@testing-library/jest-dom/vitest";
import CsmDashboardPage from "@features/csm-dashboard/pages/CsmDashboardPage";
import { useDashboardList } from "@features/csm-dashboard/api/useDashboardList";

vi.mock("@features/csm-dashboard/api/useDashboardList", () => ({
  useDashboardList: vi.fn(),
}));

// None of these dashboards are team-based, so the header's team selector
// never renders/fetches here, but useTeams still needs mocking since
// AbtDashboardHeader calls it unconditionally (the fetch itself is disabled
// via its `enabled` param) — without this, the real hook reaches the real
// API client, which throws under vitest (no runtime config).
vi.mock("@features/csm-dashboard/api/useTeams", () => ({
  useTeams: vi.fn(() => ({ data: undefined })),
}));

// Keeps this test focused on dashboard selection + the header; the widget
// grid itself has its own tests (AgentsLandingPagePilot.test.tsx).
vi.mock("@features/csm-dashboard/components/AgentsLandingPagePilot", () => ({
  default: ({ dashboardId }: { dashboardId: string }) => (
    <div data-testid="agents-landing-pilot">{dashboardId}</div>
  ),
}));

const mockedUseDashboardList = vi.mocked(useDashboardList);

const DASHBOARD_LIST = [
  { id: "operations", displayName: "Operations", isDefault: false, isTeamBased: false },
  { id: "agents_pilot", displayName: "Engineer overview", isDefault: true, isTeamBased: false },
  { id: "iam", displayName: "IAM CS", isDefault: false, isTeamBased: false },
];

function mockListResult(
  overrides: Partial<ReturnType<typeof useDashboardList>>,
): void {
  mockedUseDashboardList.mockReturnValue({
    data: undefined,
    isLoading: true,
    isError: false,
    ...overrides,
  } as unknown as ReturnType<typeof useDashboardList>);
}

beforeEach(() => {
  mockedUseDashboardList.mockReset();
});

describe("CsmDashboardPage", () => {
  it("shows a loading skeleton before the dashboard list resolves", () => {
    mockListResult({ data: undefined, isLoading: true });

    const { container } = render(<CsmDashboardPage />);

    expect(container.querySelectorAll(".MuiSkeleton-root").length).toBeGreaterThan(0);
    expect(screen.queryByTestId("agents-landing-pilot")).not.toBeInTheDocument();
  });

  it("selects the isDefault dashboard once the list loads and renders the enabled, populated switcher", () => {
    mockListResult({ data: DASHBOARD_LIST, isLoading: false });

    render(<CsmDashboardPage />);

    // The isDefault entry ("agents_pilot") is selected on load.
    expect(screen.getByTestId("agents-landing-pilot")).toHaveTextContent(
      "agents_pilot",
    );

    // The switcher is populated from the BE list and enabled (no
    // disabled-state tooltip gate any more): open it and check every
    // dashboard from the list appears as an option.
    const select = screen.getByRole("combobox");
    expect(select).not.toHaveAttribute("aria-disabled", "true");

    fireEvent.mouseDown(select);
    const listbox = screen.getByRole("listbox");
    expect(within(listbox).getByText("Operations")).toBeInTheDocument();
    expect(within(listbox).getByText("IAM CS")).toBeInTheDocument();
    expect(within(listbox).getByText("Engineer overview")).toBeInTheDocument();
  });

  it("renders the real widget grid for every dashboard, not only agents_pilot", () => {
    // "operations" is the default entry here — every dashboard now has real
    // widgets, so the grid renders regardless of which one is selected.
    mockListResult({
      data: [
        { id: "agents_pilot", displayName: "Engineer overview", isDefault: false, isTeamBased: false },
        { id: "operations", displayName: "Operations", isDefault: true, isTeamBased: false },
      ],
      isLoading: false,
    });

    render(<CsmDashboardPage />);

    expect(screen.getByTestId("agents-landing-pilot")).toHaveTextContent(
      "operations",
    );
  });
});
