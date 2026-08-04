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
import type { JSX, ReactElement } from "react";
import { useLocation, MemoryRouter, Route, Routes } from "react-router";
import "@testing-library/jest-dom/vitest";

// The real client reads runtime config at module load, which isn't present
// under vitest (same approach as CaseActivitiesFeed.test.tsx). CasesList
// renders a CasePreviewDrawer alongside every row (closed by default here,
// since no test opens it), which calls useGetCsmCaseComments — the mock just
// keeps that hook's useBackendApi() call from throwing on missing runtime
// config; its query stays disabled (no caseId) so it's never actually invoked.
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: vi.fn().mockResolvedValue({ comments: [] }) }),
}));

import CasesList from "@features/csm-cases/components/CasesList";
import type { CsmCaseRow } from "@features/csm-cases/types/csmCases";

function renderWithProviders(ui: ReactElement, initialEntries: string[]): ReturnType<typeof render> {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={initialEntries}>{ui}</MemoryRouter>
    </QueryClientProvider>,
  );
}

const CASE: CsmCaseRow = {
  id: "case-1",
  caseNumber: "CS-1007",
  subject: "Cluster fails to start",
  customer: "Acme Corp",
  accountId: "acct-1",
  projectId: "proj-1",
  projectName: "Acme Project",
  product: "WSO2 Identity Server",
  severity: "S2",
  state: "work_in_progress",
  assignee: "Jane Doe",
  assigneeIsMe: false,
  slaClockType: "first_response",
  minutesToBreach: 120,
  createdAt: "2026-01-01T00:00:00Z",
  updatedAt: "2026-01-02T00:00:00Z",
};

// Stand-in for the detail page: reads back whatever `state` the row link
// carried, so the test can assert the filtered list URL survived the
// navigation without depending on the real detail page's implementation.
function DetailStub(): JSX.Element {
  const location = useLocation();
  const from = (location.state as { from?: string } | undefined)?.from;
  return <div data-testid="from-state">{from ?? "(none)"}</div>;
}

describe("CasesList row navigation", () => {
  it("carries the current (filtered) list URL forward as router state", () => {
    renderWithProviders(
      <Routes>
        <Route
          path="/cases"
          element={<CasesList cases={[CASE]} isLoading={false} />}
        />
        <Route path="/cases/:id" element={<DetailStub />} />
      </Routes>,
      ["/cases?state=work_in_progress&severity=S2"],
    );

    fireEvent.click(screen.getByText("Cluster fails to start"));

    expect(screen.getByTestId("from-state")).toHaveTextContent(
      "/cases?state=work_in_progress&severity=S2",
    );
  });

  it("carries a bare list URL forward when no filters are active", () => {
    renderWithProviders(
      <Routes>
        <Route
          path="/cases"
          element={<CasesList cases={[CASE]} isLoading={false} />}
        />
        <Route path="/cases/:id" element={<DetailStub />} />
      </Routes>,
      ["/cases"],
    );

    fireEvent.click(screen.getByText("Cluster fails to start"));

    expect(screen.getByTestId("from-state")).toHaveTextContent("/cases");
  });
});

describe("CasesList quick preview", () => {
  it("opens the preview drawer instead of navigating when the preview action is clicked", () => {
    renderWithProviders(
      <Routes>
        <Route
          path="/cases"
          element={<CasesList cases={[CASE]} isLoading={false} />}
        />
        <Route path="/cases/:id" element={<DetailStub />} />
      </Routes>,
      ["/cases"],
    );

    fireEvent.click(screen.getByRole("button", { name: "Quick preview CS-1007" }));

    expect(screen.getByText("View full details")).toBeInTheDocument();
    expect(screen.queryByTestId("from-state")).not.toBeInTheDocument();
  });
});
