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

import { render, waitFor } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import "@testing-library/jest-dom/vitest";
import { MemoryRouter } from "react-router";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";

const authFetchMock = vi.fn();
const postMock = vi.fn();

// useSearchUsers (csm-users/api) is on the older useAuthApiClient + apiConfig
// convention; useSearchRoles / useSearchTeams / the group picker's
// useSearchGroups are all on useBackendApi. Both read runtime config at
// module load, which isn't present under vitest, so stub both (same approach
// as useAccountProjects.test.tsx / useQuickCaseSearch.test.tsx).
vi.mock("@config/apiConfig", () => ({
  apiConfig: { backendUrl: "https://example.test" },
}));
vi.mock("@hooks/useAuthApiClient", () => ({
  useAuthApiClient: () => authFetchMock,
}));
vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {
    status: number;
    constructor(status: number, message: string) {
      super(message);
      this.status = status;
    }
  },
  useBackendApi: () => ({ post: postMock }),
}));

import CsmUsersPage from "@features/csm-users/pages/CsmUsersPage";

function jsonResponse(body: unknown): Response {
  return {
    ok: true,
    status: 200,
    statusText: "OK",
    json: async () => body,
    text: async () => JSON.stringify(body),
  } as unknown as Response;
}

function renderPage(initialPath: string): ReturnType<typeof render> {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[initialPath]}>
        <CsmUsersPage />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("CsmUsersPage", () => {
  beforeEach(() => {
    authFetchMock.mockReset();
    postMock.mockReset();
    postMock.mockImplementation((path: string) => {
      if (path === "/roles/search") {
        return Promise.resolve({
          roles: [{ id: "agent", name: "Agent" }],
          total: 1,
          limit: 50,
          offset: 0,
        });
      }
      if (path === "/teams/search") {
        return Promise.resolve({
          teams: [{ id: "alpha", name: "Alpha" }],
          total: 1,
          limit: 50,
          offset: 0,
        });
      }
      return Promise.resolve({ groups: [], total: 0, limit: 20, offset: 0 });
    });
    authFetchMock.mockResolvedValue(
      jsonResponse({ users: [], total: 0, limit: 20, offset: 0, hasMore: false }),
    );
  });

  it("combines name/email search, role, group, team and status into one request with every key set", async () => {
    renderPage(
      "/admin/users?q=jane&roles=agent&groups=11111111-1111-1111-1111-111111111111&teams=alpha&active=active",
    );

    await waitFor(() => expect(authFetchMock).toHaveBeenCalled());

    // All filters land on a single /users/search call, combined (AND'd)
    // server-side — not split across separate requests per filter.
    expect(authFetchMock).toHaveBeenCalledTimes(1);
    const [url, requestInit] = authFetchMock.mock.calls[0];
    expect(url).toContain("/users/search");
    const body = JSON.parse(requestInit.body as string);
    expect(body.filters).toEqual({
      searchQuery: "jane",
      roleIds: ["agent"],
      groupIds: ["11111111-1111-1111-1111-111111111111"],
      teamIds: ["alpha"],
      active: true,
    });
  });

  it("omits every filter key when nothing is selected", async () => {
    renderPage("/admin/users");

    await waitFor(() => expect(authFetchMock).toHaveBeenCalled());
    const [, requestInit] = authFetchMock.mock.calls[0];
    const body = JSON.parse(requestInit.body as string);
    expect(body.filters).toEqual({});
  });
});
