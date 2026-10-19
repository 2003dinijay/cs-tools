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
import { beforeEach, describe, expect, it, vi } from "vitest";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { usePendingVerificationsListSearch } from "@features/support/api/usePendingVerificationsListSearch";

const mockSearchResponse = {
  pendingVerifications: [],
  totalRecords: 3,
  autoClosedCount: 2,
  manualCount: 1,
  typeCounts: [],
  limit: 50,
  offset: 0,
  hasMore: false,
};

vi.mock("@asgardeo/react", () => ({
  useAsgardeo: () => ({ isSignedIn: true, isLoading: false }),
}));

const mockAuthFetch = vi.fn().mockResolvedValue({
  ok: true,
  json: () => Promise.resolve(mockSearchResponse),
});
vi.mock("@/hooks/useAuthApiClient", () => ({
  useAuthApiClient: () => mockAuthFetch,
}));

describe("usePendingVerificationsListSearch", () => {
  let queryClient: QueryClient;

  beforeEach(() => {
    queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    vi.clearAllMocks();
    mockAuthFetch.mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockSearchResponse),
    });
    (window as unknown as { config?: { CUSTOMER_PORTAL_BACKEND_BASE_URL?: string } }).config = {
      CUSTOMER_PORTAL_BACKEND_BASE_URL: "https://api.test",
    };
  });

  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );

  it("returns the search response on success", async () => {
    const { result } = renderHook(
      () =>
        usePendingVerificationsListSearch(
          "project-1",
          { includeVerified: false },
          { limit: 50, offset: 0 },
        ),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.data?.totalRecords).toBe(3);
    expect(result.current.data?.autoClosedCount).toBe(2);
    expect(result.current.data?.manualCount).toBe(1);
  });

  it("posts the project id, filters, and pagination in the request body", async () => {
    renderHook(
      () =>
        usePendingVerificationsListSearch(
          "project-1",
          { addedReason: "auto-closed", includeVerified: false, searchQuery: "CS001" },
          { limit: 25, offset: 50 },
        ),
      { wrapper },
    );

    await waitFor(() => expect(mockAuthFetch).toHaveBeenCalled());

    const [url, init] = mockAuthFetch.mock.calls[0];
    expect(url).toBe("https://api.test/pending-verifications/search");
    expect(init.method).toBe("POST");
    expect(JSON.parse(init.body)).toEqual({
      projectId: "project-1",
      pagination: { limit: 25, offset: 50 },
      filters: {
        recordTypes: undefined,
        addedReason: "auto-closed",
        searchQuery: "CS001",
        includeVerified: false,
        verifiedOnly: false,
      },
    });
  });

  it("defaults verifiedOnly and includeVerified to false when omitted", async () => {
    renderHook(
      () => usePendingVerificationsListSearch("project-1", {}, { limit: 50, offset: 0 }),
      { wrapper },
    );

    await waitFor(() => expect(mockAuthFetch).toHaveBeenCalled());

    const [, init] = mockAuthFetch.mock.calls[0];
    const body = JSON.parse(init.body);
    expect(body.filters.includeVerified).toBe(false);
    expect(body.filters.verifiedOnly).toBe(false);
  });

  it("does not fetch when projectId is empty", () => {
    const { result } = renderHook(
      () => usePendingVerificationsListSearch("", {}, { limit: 50, offset: 0 }),
      { wrapper },
    );

    expect(result.current.fetchStatus).toBe("idle");
    expect(mockAuthFetch).not.toHaveBeenCalled();
  });

  it("does not fetch when enabled is explicitly false", () => {
    const { result } = renderHook(
      () => usePendingVerificationsListSearch("project-1", {}, { limit: 50, offset: 0 }, false),
      { wrapper },
    );

    expect(result.current.fetchStatus).toBe("idle");
    expect(mockAuthFetch).not.toHaveBeenCalled();
  });

  it("surfaces a fetch error", async () => {
    mockAuthFetch.mockResolvedValueOnce({ ok: false, statusText: "Internal Server Error" });

    const { result } = renderHook(
      () => usePendingVerificationsListSearch("project-1", {}, { limit: 50, offset: 0 }),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.error?.message).toContain("Failed to fetch pending verifications");
  });
});
