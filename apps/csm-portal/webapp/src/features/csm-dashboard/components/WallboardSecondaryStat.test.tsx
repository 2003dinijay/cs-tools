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

import { render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { describe, expect, it, vi, beforeEach } from "vitest";
import "@testing-library/jest-dom/vitest";
import type { ReactNode } from "react";
import { MemoryRouter } from "react-router";

const postMock = vi.fn();

vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock }),
}));
vi.mock("@config/apiConfig", () => ({
  apiConfig: { backendUrl: "https://example.test" },
}));
vi.mock("@context/current-user/CurrentUserContext", () => ({
  useCurrentUser: () => ({ user: { id: "u1" }, isLoading: false, isError: false }),
}));

import WallboardSecondaryStat from "@features/csm-dashboard/components/WallboardSecondaryStat";
import { __resetWidgetFetchConcurrencyForTests } from "@features/csm-dashboard/utils/widgetFetchConcurrency";

function renderWithClient(ui: ReactNode) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>{ui}</MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("WallboardSecondaryStat", () => {
  beforeEach(() => {
    postMock.mockReset();
    __resetWidgetFetchConcurrencyForTests();
  });

  it("renders a skeleton while its count is in flight", () => {
    postMock.mockReturnValue(new Promise(() => {}));
    const { container } = renderWithClient(
      <WallboardSecondaryStat widgetId="w1" displayName="Being Fixed" resourceType="incident" filters={{}} />,
    );
    expect(container.querySelectorAll(".MuiSkeleton-root").length).toBe(1);
  });

  it("renders the resolved count and label, with no data-alert attribute at all", async () => {
    postMock.mockResolvedValue({ total: 99, incidents: [], limit: 1, offset: 0, hasMore: false });
    const { container } = renderWithClient(
      <WallboardSecondaryStat widgetId="w1" displayName="Being Fixed" resourceType="incident" filters={{}} />,
    );
    expect(await screen.findByText("99")).toBeInTheDocument();
    expect(screen.getByText("Being Fixed")).toBeInTheDocument();
    // Unlike WallboardStatTile, this tier is never glow-capable at all.
    expect(container.querySelector("[data-alert]")).toBeNull();
  });
});
