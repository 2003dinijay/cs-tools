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
// under vitest (same approach as useSearchGroups.test.tsx).
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock }),
}));

import { useSearchIncidentsExcludingSelf } from "@features/csm-operations/api/useSearchIncidentsForSelect";

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

describe("useSearchIncidentsExcludingSelf", () => {
  beforeEach(() => {
    postMock.mockReset();
    postMock.mockResolvedValue({
      incidents: [
        { id: "inc-1", number: "INC0000001", subject: "Self" },
        { id: "inc-2", number: "INC0000002", subject: "Other incident" },
      ],
    });
  });

  it("drops the incident being edited (excludeId) from the results", async () => {
    const { result } = renderHook(
      () => useSearchIncidentsExcludingSelf("", true, "inc-1"),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.data?.map((i) => i.id)).toEqual(["inc-2"]);
  });

  it("returns every result unfiltered when no excludeId is given", async () => {
    const { result } = renderHook(
      () => useSearchIncidentsExcludingSelf("", true, undefined),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.data?.map((i) => i.id)).toEqual(["inc-1", "inc-2"]);
  });
});
