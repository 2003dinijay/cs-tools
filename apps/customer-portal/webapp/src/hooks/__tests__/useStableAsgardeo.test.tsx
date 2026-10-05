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

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import useGetUserDetails from "@features/settings/api/useGetUserDetails";
import { useAsgardeo } from "@hooks/useStableAsgardeo";

const { authFetchMock, sdk } = vi.hoisted(() => ({
  authFetchMock: vi.fn(),
  sdk: {
    state: { isSignedIn: true, isLoading: false } as {
      isSignedIn: boolean;
      isLoading: boolean;
    },
  },
}));

vi.mock("@asgardeo/react", () => ({
  useAsgardeo: () => ({ ...sdk.state, getIdToken: vi.fn() }),
}));

vi.mock("@/hooks/useAuthApiClient", () => ({
  useAuthApiClient: () => authFetchMock,
}));

vi.mock("@hooks/useLogger", () => ({
  useLogger: () => ({ debug: vi.fn(), error: vi.fn(), warn: vi.fn() }),
}));

function createWrapper() {
  // same shape as AppWithConfig's client for the fields that matter here:
  // staleTime stays at the library default (0) and the hook sets its own 0
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, refetchOnMount: true } },
  });
  return ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

const settle = (ms = 25) => new Promise((resolve) => setTimeout(resolve, ms));

describe("useAsgardeo (stable loading flag)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    sdk.state = { isSignedIn: true, isLoading: false };
    (
      window as unknown as { config?: { CUSTOMER_PORTAL_BACKEND_BASE_URL?: string } }
    ).config = { CUSTOMER_PORTAL_BACKEND_BASE_URL: "https://api.test" };
    authFetchMock.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ id: "u-1" }),
    });
  });

  describe("on a real data hook (useGetUserDetails)", () => {
    it("fetches once, then ignores the SDK's loading flag flipping while signed in", async () => {
      const { result, rerender } = renderHook(() => useGetUserDetails(), {
        wrapper: createWrapper(),
      });
      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(authFetchMock).toHaveBeenCalledTimes(1);

      // the SDK reports "loading" while it touches the token, over and over
      for (let i = 0; i < 4; i++) {
        sdk.state.isLoading = true;
        rerender();
        await settle();
        sdk.state.isLoading = false;
        rerender();
        await settle();
      }

      expect(authFetchMock).toHaveBeenCalledTimes(1);
    });

    it("still waits for the first ready state before fetching", async () => {
      sdk.state = { isSignedIn: true, isLoading: true };
      const { result, rerender } = renderHook(() => useGetUserDetails(), {
        wrapper: createWrapper(),
      });
      await settle();
      expect(authFetchMock).not.toHaveBeenCalled();

      sdk.state.isLoading = false;
      rerender();
      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(authFetchMock).toHaveBeenCalledTimes(1);
    });

    it("does not fetch while signed out", async () => {
      sdk.state = { isSignedIn: false, isLoading: false };
      renderHook(() => useGetUserDetails(), { wrapper: createWrapper() });
      await settle();
      expect(authFetchMock).not.toHaveBeenCalled();
    });
  });

  describe("the hook itself", () => {
    it("reports loading until the session has been ready once, then stays settled", () => {
      sdk.state = { isSignedIn: true, isLoading: true };
      const { result, rerender } = renderHook(() => useAsgardeo());
      expect(result.current.isLoading).toBe(true);

      sdk.state.isLoading = false;
      rerender();
      expect(result.current.isLoading).toBe(false);

      sdk.state.isLoading = true;
      rerender();
      expect(result.current.isLoading).toBe(false);
    });

    it("passes loading through again after a sign-out", () => {
      const { result, rerender } = renderHook(() => useAsgardeo());
      expect(result.current.isLoading).toBe(false);

      sdk.state = { isSignedIn: false, isLoading: false };
      rerender();
      expect(result.current.isSignedIn).toBe(false);

      // signing back in, with the SDK still busy: not settled yet
      sdk.state = { isSignedIn: true, isLoading: true };
      rerender();
      expect(result.current.isLoading).toBe(true);
    });

    it("never reports a signed-out session as ready", () => {
      sdk.state = { isSignedIn: false, isLoading: true };
      const { result } = renderHook(() => useAsgardeo());
      expect(result.current.isSignedIn).toBe(false);
      expect(result.current.isLoading).toBe(true);
    });

    it("passes the rest of the SDK object through untouched", () => {
      const { result } = renderHook(() => useAsgardeo());
      expect(typeof result.current.getIdToken).toBe("function");
    });
  });
});
