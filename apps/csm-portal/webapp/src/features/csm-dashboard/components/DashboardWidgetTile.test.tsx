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

const postMock = vi.fn();

vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock }),
}));

import DashboardWidgetTile from "@features/csm-dashboard/components/DashboardWidgetTile";

function renderWithClient(ui: ReactNode) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>,
  );
}

describe("DashboardWidgetTile", () => {
  beforeEach(() => {
    postMock.mockReset();
  });

  it("renders a skeleton while its own count is in flight", () => {
    postMock.mockReturnValue(new Promise(() => {}));
    const { container } = renderWithClient(
      <DashboardWidgetTile widgetId="my_patches" displayName="My Patches" filters={{}} />,
    );
    expect(container.querySelectorAll(".MuiSkeleton-root").length).toBe(1);
  });

  it("renders the resolved count once its own /cases/search call succeeds", async () => {
    postMock.mockResolvedValue({ total: 3, cases: [], limit: 1, offset: 0, hasMore: false });

    renderWithClient(
      <DashboardWidgetTile widgetId="my_patches" displayName="My Patches" filters={{}} />,
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
      <DashboardWidgetTile widgetId="my_patches" displayName="My Patches" filters={{}} />,
    );

    await waitFor(() =>
      expect(screen.getByText("Could not load this widget.")).toBeInTheDocument(),
    );
  });
});
