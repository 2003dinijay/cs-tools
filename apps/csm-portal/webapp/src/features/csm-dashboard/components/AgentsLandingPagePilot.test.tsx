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

import { render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { describe, expect, it, vi, beforeEach } from "vitest";
import "@testing-library/jest-dom/vitest";
import type { ReactNode } from "react";

const getMock = vi.fn();

vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ get: getMock }),
}));

import AgentsLandingPagePilot from "@features/csm-dashboard/components/AgentsLandingPagePilot";

function renderWithClient(ui: ReactNode) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>,
  );
}

describe("AgentsLandingPagePilot", () => {
  beforeEach(() => {
    getMock.mockReset();
  });

  it("renders skeleton tiles while the shared query is in flight", () => {
    getMock.mockReturnValue(new Promise(() => {}));
    const { container } = renderWithClient(<AgentsLandingPagePilot />);
    expect(container.querySelectorAll(".MuiSkeleton-root").length).toBe(3);
  });

  it("renders one tile per resolved widget once the query succeeds", async () => {
    getMock.mockResolvedValue([
      { widgetId: "my_patches", displayName: "My Patches", displayType: "single_score", count: 3 },
      { widgetId: "my_reminders", displayName: "My Reminders", displayType: "single_score", count: 5 },
      { widgetId: "open_incident_team", displayName: "Open Incidents (Team)", displayType: "single_score", count: 12 },
    ]);

    renderWithClient(<AgentsLandingPagePilot />);

    await waitFor(() => expect(screen.getByText("My Patches")).toBeInTheDocument());
    expect(screen.getByText("3")).toBeInTheDocument();
    expect(screen.getByText("My Reminders")).toBeInTheDocument();
    expect(screen.getByText("5")).toBeInTheDocument();
    expect(screen.getByText("Open Incidents (Team)")).toBeInTheDocument();
    expect(screen.getByText("12")).toBeInTheDocument();
  });

  it("renders an inline error state on each tile when the shared query fails", async () => {
    getMock.mockRejectedValue(new Error("boom"));

    renderWithClient(<AgentsLandingPagePilot />);

    await waitFor(() =>
      expect(screen.getAllByText("Could not load this widget.").length).toBe(3),
    );
  });
});
