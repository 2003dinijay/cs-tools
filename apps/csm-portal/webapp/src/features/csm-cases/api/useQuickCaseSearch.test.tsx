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
// under vitest (same approach as useDecideChangeRequestApproval.test.tsx).
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock }),
}));

import { useQuickCaseSearch } from "@features/csm-cases/api/useQuickCaseSearch";
import { ALL_CASE_TYPES } from "@features/csm-cases/utils/caseType";

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

describe("useQuickCaseSearch", () => {
  beforeEach(() => {
    postMock.mockReset();
    postMock.mockResolvedValue({ cases: [] });
  });

  it("requests every known case sub-type, not just the default 'case' type", async () => {
    const { result } = renderHook(() => useQuickCaseSearch("ABC-123"), {
      wrapper,
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(postMock).toHaveBeenCalledWith(
      "/cases/search",
      expect.objectContaining({
        filters: expect.objectContaining({
          searchQuery: "ABC-123",
          types: ALL_CASE_TYPES,
        }),
      }),
    );
    // Guards against the search silently regressing to only `case` hits again
    // (the SRA-missing-from-quick-nav bug) if `ALL_CASE_TYPES` ever shrinks.
    expect(ALL_CASE_TYPES).toEqual(
      expect.arrayContaining([
        "case",
        "service_request",
        "security_report_analysis",
        "announcement",
        "engagement",
      ]),
    );
  });

  it("does not fire a search until the query reaches the minimum length", () => {
    renderHook(() => useQuickCaseSearch("a"), { wrapper });
    expect(postMock).not.toHaveBeenCalled();
  });
});
