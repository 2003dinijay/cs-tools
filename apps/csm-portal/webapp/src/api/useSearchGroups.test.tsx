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

// The real client reads runtime config at module load, which isn't present
// under vitest (same approach as useQuickCaseSearch.test.tsx).
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock }),
}));

import {
  resetAssignableGroupsSupport,
  useSearchAssignableGroups,
  useSearchGroups,
} from "@api/useSearchGroups";

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

/** Shaped like `BackendApiError`: the hook matches on `status`, not on the class. */
class StatusError extends Error {
  constructor(
    readonly status: number,
    message: string,
  ) {
    super(message);
  }
}

describe("useSearchGroups", () => {
  beforeEach(() => {
    postMock.mockReset();
    postMock.mockResolvedValue({ groups: [] });
    resetAssignableGroupsSupport();
  });

  it("fires with an empty query as soon as the caller enables it (dropdown opened, nothing typed)", async () => {
    const { result } = renderHook(() => useSearchGroups("", true), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(postMock).toHaveBeenCalledWith(
      "/groups/search",
      expect.objectContaining({ filters: { searchQuery: "" } }),
    );
  });

  it("does not fire while the caller keeps it disabled (dropdown closed)", () => {
    renderHook(() => useSearchGroups("", false), { wrapper });
    expect(postMock).not.toHaveBeenCalled();
  });

  it("lists every team: the plain picker never asks for assignable groups only", async () => {
    const { result } = renderHook(() => useSearchGroups("ap", true), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    const body = postMock.mock.calls[0][1] as { filters: Record<string, unknown> };
    expect(body.filters).toEqual({ searchQuery: "ap" });
  });
});

describe("useSearchAssignableGroups", () => {
  beforeEach(() => {
    postMock.mockReset();
    postMock.mockResolvedValue({ groups: [{ id: "g1", name: "Atlas", active: true }] });
    resetAssignableGroupsSupport();
  });

  it("asks only for the groups a change can be assigned to, and returns them", async () => {
    const { result } = renderHook(() => useSearchAssignableGroups(" at ", true), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(postMock).toHaveBeenCalledTimes(1);
    expect(postMock).toHaveBeenCalledWith(
      "/groups/search",
      expect.objectContaining({ filters: { searchQuery: "at", assignableOnly: true } }),
    );
    expect(result.current.data).toEqual([{ id: "g1", name: "Atlas", active: true }]);
  });

  it("does not fire while the dropdown is closed", () => {
    renderHook(() => useSearchAssignableGroups("", false), { wrapper });
    expect(postMock).not.toHaveBeenCalled();
  });

  it("falls back to the plain search when the entity service predates the flag, and stops sending it", async () => {
    postMock
      .mockRejectedValueOnce(new StatusError(400, "unknown field assignableOnly"))
      .mockResolvedValue({ groups: [{ id: "g2", name: "Apollo", active: true }] });

    const first = renderHook(() => useSearchAssignableGroups("", true), { wrapper });
    await waitFor(() => expect(first.result.current.isSuccess).toBe(true));
    expect(first.result.current.data).toEqual([{ id: "g2", name: "Apollo", active: true }]);
    expect((postMock.mock.calls[1][1] as { filters: Record<string, unknown> }).filters).toEqual({ searchQuery: "" });

    // The next search goes straight to the plain request.
    postMock.mockClear();
    const second = renderHook(() => useSearchAssignableGroups("ap", true), { wrapper });
    await waitFor(() => expect(second.result.current.isSuccess).toBe(true));
    expect(postMock).toHaveBeenCalledTimes(1);
    expect((postMock.mock.calls[0][1] as { filters: Record<string, unknown> }).filters).toEqual({ searchQuery: "ap" });
  });

  it("reports a failure that is not a 400 instead of retrying", async () => {
    postMock.mockRejectedValue(new StatusError(503, "unavailable"));
    const { result } = renderHook(() => useSearchAssignableGroups("", true), { wrapper });
    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(postMock).toHaveBeenCalledTimes(1);
  });

  it("does not stop sending the flag because a request was simply invalid", async () => {
    // Rejected with the flag AND without it: the request is at fault, not the flag.
    postMock.mockRejectedValue(new StatusError(400, "searchQuery too long"));
    const first = renderHook(() => useSearchAssignableGroups("x", true), { wrapper });
    await waitFor(() => expect(first.result.current.isError).toBe(true));

    postMock.mockReset();
    postMock.mockResolvedValue({ groups: [] });
    const second = renderHook(() => useSearchAssignableGroups("y", true), { wrapper });
    await waitFor(() => expect(second.result.current.isSuccess).toBe(true));
    expect((postMock.mock.calls[0][1] as { filters: Record<string, unknown> }).filters).toEqual({
      searchQuery: "y",
      assignableOnly: true,
    });
  });
});
