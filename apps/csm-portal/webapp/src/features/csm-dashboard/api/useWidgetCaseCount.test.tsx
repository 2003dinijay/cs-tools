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

const postMock = vi.fn();

vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock }),
}));

import { useWidgetCaseCount } from "@features/csm-dashboard/api/useWidgetCaseCount";

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

describe("useWidgetCaseCount", () => {
  beforeEach(() => {
    postMock.mockReset();
  });

  it("resolves the widget's count from its own /cases/search call", async () => {
    postMock.mockResolvedValue({ total: 3, cases: [], limit: 1, offset: 0, hasMore: false });

    const { result } = renderHook(
      () =>
        useWidgetCaseCount("my_patches", {
          assignedUserIds: ["user-1"],
          tags: ["patch"],
        }),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(postMock).toHaveBeenCalledTimes(1);
    expect(postMock).toHaveBeenCalledWith("/cases/search", {
      filters: { assignedUserIds: ["user-1"], tags: ["patch"] },
      pagination: { offset: 0, limit: 1 },
    });
    expect(result.current.data).toBe(3);
  });

  it("surfaces a query error when the call fails", async () => {
    postMock.mockRejectedValue(new Error("boom"));

    const { result } = renderHook(
      () => useWidgetCaseCount("my_reminders", { states: ["awaiting_info"] }),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.error?.message).toBe("boom");
  });
});
