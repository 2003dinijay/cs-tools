/**
 * Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import type { ReactNode } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const api = { post: vi.fn() };
vi.mock("@api/backend/client", () => ({ useBackendApi: () => api }));
vi.mock("@config/apiConfig", () => ({ apiConfig: { backendUrl: "https://example.test" } }));

import { useEngagementAllocations } from "./useEngagementAllocations";

const PATH = "/team-schedule/engagement-allocations/search";

function wrapper() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={qc}>{children}</QueryClientProvider>
  );
}

const ids = (n: number) => Array.from({ length: n }, (_, i) => `00000000-0000-0000-0000-${String(i).padStart(12, "0")}`);
const row = (id: string, userId: string) => ({
  id,
  userId,
  engagementId: "00000000-0000-0000-0000-0000000000e1",
  startDate: "2026-09-21",
  endDate: "2026-09-22",
});

beforeEach(() => {
  api.post.mockReset();
});

describe("useEngagementAllocations", () => {
  it("posts the ids and the window to the search endpoint", async () => {
    api.post.mockResolvedValue({ allocations: [row("a1", ids(1)[0])], count: 1 });
    const { result } = renderHook(() => useEngagementAllocations(ids(2), "2026-09-21", "2026-09-27"), {
      wrapper: wrapper(),
    });
    await waitFor(() => expect(result.current.allocations).toHaveLength(1));
    expect(api.post).toHaveBeenCalledTimes(1);
    expect(api.post).toHaveBeenCalledWith(PATH, { userIds: ids(2), from: "2026-09-21", to: "2026-09-27" });
    expect(result.current.isError).toBe(false);
  });

  it("batches ids at 500 per request", async () => {
    api.post.mockResolvedValue({ allocations: [], count: 0 });
    renderHook(() => useEngagementAllocations(ids(1001), "2026-09-21", "2026-09-27"), { wrapper: wrapper() });
    await waitFor(() => expect(api.post).toHaveBeenCalledTimes(3));
    const sizes = api.post.mock.calls.map((c) => c[1].userIds.length).sort((a, b) => b - a);
    expect(sizes).toEqual([500, 500, 1]);
  });

  it("splits a window over 70 days into requests of at most 70", async () => {
    api.post.mockResolvedValue({ allocations: [], count: 0 });
    renderHook(() => useEngagementAllocations(ids(1), "2026-01-01", "2026-03-12"), { wrapper: wrapper() });
    await waitFor(() => expect(api.post).toHaveBeenCalledTimes(2));
    const windows = api.post.mock.calls.map((c) => [c[1].from, c[1].to]).sort();
    expect(windows).toEqual([
      ["2026-01-01", "2026-03-11"],
      ["2026-03-12", "2026-03-12"],
    ]);
  });

  it("merges the pieces and keeps one copy of an allocation both returned", async () => {
    api.post.mockImplementation(async (_path: string, body: { from: string }) => ({
      allocations: [row("shared", ids(1)[0]), row(`only-${body.from}`, ids(1)[0])],
      count: 2,
    }));
    const { result } = renderHook(() => useEngagementAllocations(ids(1), "2026-01-01", "2026-03-12"), {
      wrapper: wrapper(),
    });
    await waitFor(() => expect(result.current.allocations).toHaveLength(3));
  });

  it("keeps the pieces that loaded when another fails, and says so", async () => {
    api.post.mockImplementation(async (_p: string, body: { from: string }) => {
      if (body.from === "2026-03-12") throw new Error("boom");
      return { allocations: [row("ok", ids(1)[0])], count: 1 };
    });
    const { result } = renderHook(() => useEngagementAllocations(ids(1), "2026-01-01", "2026-03-12"), {
      wrapper: wrapper(),
    });
    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.allocations.map((a) => a.id)).toEqual(["ok"]);
  });

  it("degrades to no bars and no throw when the read fails outright", async () => {
    api.post.mockRejectedValue(new Error("boom"));
    const { result } = renderHook(() => useEngagementAllocations(ids(1), "2026-09-21", "2026-09-27"), {
      wrapper: wrapper(),
    });
    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.allocations).toEqual([]);
  });

  it("asks for nothing with no ids, or when disabled", () => {
    renderHook(() => useEngagementAllocations([], "2026-09-21", "2026-09-27"), { wrapper: wrapper() });
    renderHook(() => useEngagementAllocations(ids(1), "2026-09-21", "2026-09-27", false), { wrapper: wrapper() });
    expect(api.post).not.toHaveBeenCalled();
  });

  it("reads a roster-sized window of 90 days in two pieces, and a one-month window in one", async () => {
    api.post.mockResolvedValue({ allocations: [], count: 0 });
    renderHook(() => useEngagementAllocations(ids(1), "2026-09-01", "2026-09-30"), { wrapper: wrapper() });
    await waitFor(() => expect(api.post).toHaveBeenCalledTimes(1));
    expect(api.post.mock.calls[0][1]).toEqual({ userIds: ids(1), from: "2026-09-01", to: "2026-09-30" });
    api.post.mockClear();
    renderHook(() => useEngagementAllocations(ids(1), "2026-08-07", "2026-11-04"), { wrapper: wrapper() });
    await waitFor(() => expect(api.post).toHaveBeenCalledTimes(2));
    const windows = api.post.mock.calls.map((c) => [c[1].from, c[1].to]).sort();
    expect(windows).toEqual([
      ["2026-08-07", "2026-10-15"],
      ["2026-10-16", "2026-11-04"],
    ]);
  });

  it("does not read again when the view changes to a window it already holds", async () => {
    api.post.mockResolvedValue({ allocations: [row("a1", ids(1)[0])], count: 1 });
    const w = wrapper();
    const { result, rerender } = renderHook(
      (p: { from: string; to: string }) => useEngagementAllocations(ids(1), p.from, p.to),
      { wrapper: w, initialProps: { from: "2026-09-21", to: "2026-09-27" } },
    );
    await waitFor(() => expect(result.current.allocations).toHaveLength(1));
    rerender({ from: "2026-09-01", to: "2026-09-30" });
    await waitFor(() => expect(api.post).toHaveBeenCalledTimes(2));
    rerender({ from: "2026-09-21", to: "2026-09-27" });
    expect(result.current.allocations).toHaveLength(1);
    expect(api.post).toHaveBeenCalledTimes(2);
  });
});
