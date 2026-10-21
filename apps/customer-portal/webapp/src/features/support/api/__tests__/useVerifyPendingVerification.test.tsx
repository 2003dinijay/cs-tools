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
import { useVerifyPendingVerification } from "@features/support/api/useVerifyPendingVerification";
import { ApiQueryKeys } from "@constants/apiConstants";

const mockVerifyResponse = {
  message: "Pending verification marked as verified",
  pendingVerification: {
    id: "pv-1",
    workItemId: "work-item-1",
    caseId: "CS0441241",
    caseTitle: "A case",
    recordType: "Case",
    addedReason: "manual",
    addedAt: "2026-10-01T00:00:00Z",
    addedBy: "someone@test.local",
    verifiedAt: "2026-10-06T00:00:00Z",
    verifiedBy: "verifier@test.local",
  },
};

const mockLogger = { debug: vi.fn(), error: vi.fn() };
vi.mock("@hooks/useLogger", () => ({ useLogger: () => mockLogger }));

vi.mock("@asgardeo/react", () => ({
  useAsgardeo: () => ({ isSignedIn: true, isLoading: false }),
}));

const mockAuthFetch = vi.fn();
vi.mock("@/hooks/useAuthApiClient", () => ({
  useAuthApiClient: () => mockAuthFetch,
}));

describe("useVerifyPendingVerification", () => {
  let queryClient: QueryClient;

  beforeEach(() => {
    queryClient = new QueryClient({ defaultOptions: { mutations: { retry: false } } });
    vi.clearAllMocks();
    mockAuthFetch.mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockVerifyResponse),
    });
    (window as unknown as { config?: { CUSTOMER_PORTAL_BACKEND_BASE_URL?: string } }).config = {
      CUSTOMER_PORTAL_BACKEND_BASE_URL: "https://api.test",
    };
  });

  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );

  it("PATCHes the pending-verification endpoint for the given id", async () => {
    const { result } = renderHook(() => useVerifyPendingVerification("project-1", "work-item-1"), {
      wrapper,
    });

    result.current.mutate("pv-1");

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(mockAuthFetch).toHaveBeenCalledWith("https://api.test/pending-verifications/pv-1", {
      method: "PATCH",
    });
    expect(result.current.data?.pendingVerification.verifiedBy).toBe("verifier@test.local");
  });

  it("refetches and invalidates the project's pending-verification search queries on success", async () => {
    const refetchSpy = vi.spyOn(queryClient, "refetchQueries");
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

    const { result } = renderHook(() => useVerifyPendingVerification("project-1", "work-item-1"), {
      wrapper,
    });

    result.current.mutate("pv-1");

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(refetchSpy).toHaveBeenCalledWith({
      queryKey: [ApiQueryKeys.PENDING_VERIFICATIONS_SEARCH, "project-1"],
      type: "active",
    });
    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: [ApiQueryKeys.PENDING_VERIFICATIONS_SEARCH, "project-1"],
    });
  });

  it("rejects when called with an empty id", async () => {
    const { result } = renderHook(() => useVerifyPendingVerification("project-1", "work-item-1"), {
      wrapper,
    });

    result.current.mutate("");

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.error?.message).toContain("Pending verification id is required");
    expect(mockAuthFetch).not.toHaveBeenCalled();
  });

  it("surfaces the parsed error message on a failed response", async () => {
    mockAuthFetch.mockResolvedValueOnce({
      ok: false,
      status: 409,
      statusText: "Conflict",
      text: () => Promise.resolve(JSON.stringify({ message: "already verified" })),
    });

    const { result } = renderHook(() => useVerifyPendingVerification("project-1", "work-item-1"), {
      wrapper,
    });

    result.current.mutate("pv-1");

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.error?.message).toContain("already verified");
  });
});
