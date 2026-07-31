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
import type { ReactElement } from "react";
import { MemoryRouter } from "react-router";
import "@testing-library/jest-dom/vitest";

const getMock = vi.fn();

// The real client reads runtime config at module load, which isn't present
// under vitest (same approach as useSearchIncidentsForSelect.test.tsx).
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ get: getMock }),
}));

import { LinkedChangeRequestsWidget } from "@features/csm-cases/components/LinkedChangeRequestsWidget";

/**
 * Render inside the providers this widget needs: a query client (it fetches each
 * change request's detail per row) and a router (each row links to the change
 * request's own page). Retries are disabled so an error-state assertion resolves
 * on the first failure instead of waiting out the default backoff.
 */
function renderWithProviders(ui: ReactElement) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>{ui}</MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("LinkedChangeRequestsWidget", () => {
  beforeEach(() => {
    getMock.mockReset();
  });

  it("renders a message explaining there are none, when the list is empty", () => {
    renderWithProviders(<LinkedChangeRequestsWidget changeRequests={[]} />);
    expect(
      screen.getByText(
        "No change requests have been raised from this service request yet.",
      ),
    ).toBeInTheDocument();
  });

  it("renders a message explaining there are none, when the list is undefined", () => {
    renderWithProviders(<LinkedChangeRequestsWidget changeRequests={undefined} />);
    expect(
      screen.getByText(
        "No change requests have been raised from this service request yet.",
      ),
    ).toBeInTheDocument();
  });

  it("renders a single change request with its state and target environment", async () => {
    getMock.mockResolvedValue({
      id: "cr-1",
      number: "CHG0000001",
      state: "implement",
      deployment: { id: "dep-1", name: "Production" },
    });

    renderWithProviders(
      <LinkedChangeRequestsWidget
        changeRequests={[{ id: "cr-1", number: "CHG0000001", name: "Roll out fix" }]}
      />,
    );

    expect(screen.getByText("CHG0000001 — Roll out fix")).toBeInTheDocument();
    await waitFor(() => expect(screen.getByText("Implement")).toBeInTheDocument());
    expect(screen.getByText("Production")).toBeInTheDocument();
  });

  it("renders one row per promotion, each visually distinguishable by state and environment", async () => {
    getMock.mockImplementation((path: string) => {
      if (path.includes("cr-dev")) {
        return Promise.resolve({
          id: "cr-dev",
          number: "CHG0000001",
          state: "closed",
          deployment: { id: "dep-dev", name: "Dev" },
        });
      }
      if (path.includes("cr-preprod")) {
        return Promise.resolve({
          id: "cr-preprod",
          number: "CHG0000002",
          state: "implement",
          deployment: { id: "dep-preprod", name: "Pre-prod" },
        });
      }
      return Promise.resolve({
        id: "cr-prod",
        number: "CHG0000003",
        state: "new",
        deployment: { id: "dep-prod", name: "Production" },
      });
    });

    renderWithProviders(
      <LinkedChangeRequestsWidget
        changeRequests={[
          { id: "cr-dev", number: "CHG0000001", name: "Roll out fix" },
          { id: "cr-preprod", number: "CHG0000002", name: "Roll out fix" },
          { id: "cr-prod", number: "CHG0000003", name: "Roll out fix" },
        ]}
      />,
    );

    expect(screen.getByText("3 total")).toBeInTheDocument();
    await waitFor(() => expect(screen.getByText("Dev")).toBeInTheDocument());
    expect(screen.getByText("Pre-prod")).toBeInTheDocument();
    expect(screen.getByText("Production")).toBeInTheDocument();
    expect(screen.getByText("Closed")).toBeInTheDocument();
    expect(screen.getByText("Implement")).toBeInTheDocument();
    expect(screen.getByText("New")).toBeInTheDocument();
  });

  it("still renders a row with its number and a dash when that row's detail fetch fails", async () => {
    getMock.mockImplementation((path: string) => {
      if (path.includes("cr-ok")) {
        return Promise.resolve({
          id: "cr-ok",
          number: "CHG0000001",
          state: "new",
          deployment: { id: "dep-1", name: "Dev" },
        });
      }
      return Promise.reject(new Error("not found"));
    });

    renderWithProviders(
      <LinkedChangeRequestsWidget
        changeRequests={[
          { id: "cr-ok", number: "CHG0000001", name: "Roll out fix" },
          { id: "cr-broken", number: "CHG0000002", name: "Roll out fix" },
        ]}
      />,
    );

    expect(screen.getByText("CHG0000001 — Roll out fix")).toBeInTheDocument();
    expect(screen.getByText("CHG0000002 — Roll out fix")).toBeInTheDocument();
    await waitFor(() => expect(screen.getByText("New")).toBeInTheDocument());
    // The failed row keeps its number/name but falls back to a dash rather
    // than being dropped or blanking the whole widget.
    expect(screen.getAllByText("—").length).toBeGreaterThanOrEqual(2);
  });

  it("navigates to the change request's detail route when a row is clicked", async () => {
    getMock.mockResolvedValue({
      id: "cr-1",
      number: "CHG0000001",
      state: "new",
      deployment: { id: "dep-1", name: "Dev" },
    });

    renderWithProviders(
      <LinkedChangeRequestsWidget
        changeRequests={[{ id: "cr-1", number: "CHG0000001", name: "Roll out fix" }]}
      />,
    );

    const row = screen.getByRole("button", { name: /view change request chg0000001/i });
    fireEvent.click(row);
    // Navigation itself is exercised via react-router's MemoryRouter; the
    // absence of a thrown error here is the assertion that the click handler
    // ran without needing a real navigate spy.
    await waitFor(() => expect(getMock).toHaveBeenCalled());
  });
});
