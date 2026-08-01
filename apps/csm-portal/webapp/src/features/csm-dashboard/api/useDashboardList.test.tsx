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

import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { describe, expect, it, vi, beforeEach } from "vitest";
import type { ReactNode } from "react";

const getMock = vi.fn();

// The real client reads runtime config at module load, which isn't present
// under vitest (same approach as useSearchGroups.test.tsx).
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ get: getMock }),
}));

import { useDashboardList } from "@features/csm-dashboard/api/useDashboardList";

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

describe("useDashboardList", () => {
  beforeEach(() => {
    getMock.mockReset();
  });

  it("fetches the dashboard registry from a single call", async () => {
    getMock.mockResolvedValue([
      { id: "agents_pilot", displayName: "Engineer overview", isDefault: true },
      { id: "operations", displayName: "Operations", isDefault: false },
    ]);

    const { result } = renderHook(() => useDashboardList(), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(getMock).toHaveBeenCalledTimes(1);
    expect(getMock).toHaveBeenCalledWith("/dashboards");
    expect(result.current.data).toEqual([
      { id: "agents_pilot", displayName: "Engineer overview", isDefault: true },
      { id: "operations", displayName: "Operations", isDefault: false },
    ]);
  });

  it("surfaces a query error when the call fails", async () => {
    getMock.mockRejectedValue(new Error("boom"));

    const { result } = renderHook(() => useDashboardList(), { wrapper });

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.error?.message).toBe("boom");
  });

  it("surfaces a query error rather than an empty list when the endpoint 404s", async () => {
    // api.get resolves 404 to null; GET /dashboards has no path param and
    // always returns 200 in practice, so a null here means the endpoint
    // itself is missing (routing/deployment problem) — must not be
    // silently treated as "zero dashboards configured".
    getMock.mockResolvedValue(null);

    const { result } = renderHook(() => useDashboardList(), { wrapper });

    await waitFor(() => expect(result.current.isError).toBe(true));
  });
});
