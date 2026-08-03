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

import { useTeams } from "@features/csm-dashboard/api/useTeams";

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

describe("useTeams", () => {
  beforeEach(() => {
    postMock.mockReset();
  });

  it("fetches teams via POST /teams/search when enabled", async () => {
    postMock.mockResolvedValue({
      teams: [{ id: "cs_team_leads", name: "CS Team Leads" }],
    });

    const { result } = renderHook(() => useTeams(true), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(postMock).toHaveBeenCalledTimes(1);
    expect(postMock).toHaveBeenCalledWith("/teams/search", {
      pagination: { offset: 0, limit: 100 },
    });
    expect(result.current.data).toEqual([
      { id: "cs_team_leads", name: "CS Team Leads" },
    ]);
  });

  it("does not fetch when disabled", () => {
    renderHook(() => useTeams(false), { wrapper });
    expect(postMock).not.toHaveBeenCalled();
  });
});
