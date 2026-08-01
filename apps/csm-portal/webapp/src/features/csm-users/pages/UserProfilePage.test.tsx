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

import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import { MemoryRouter } from "react-router";
import type { UseQueryResult } from "@tanstack/react-query";
import type { NormalizedUserDetail } from "@features/csm-users/types/csmUsers";

const navigateMock = vi.fn();
const useGetUserByIdMock = vi.fn();

vi.mock("react-router", async (importOriginal) => {
  const actual = await importOriginal<typeof import("react-router")>();
  return { ...actual, useParams: () => ({ id: "user-1" }) };
});
vi.mock("@hooks/useNavTransition", () => ({
  useNavTransition: () => navigateMock,
}));
// The backend client reads runtime config (`CSM_PORTAL_BACKEND_BASE_URL`) at
// module load, which isn't present under vitest. `QueryErrorState` imports
// `BackendApiError` from it directly, so stub the module with a real class
// (so `instanceof` still works) — same approach as
// CsmChangeRequestDetailPage.test.tsx.
vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {
    status: number;
    constructor(status: number, message: string) {
      super(message);
      this.status = status;
    }
  },
}));
vi.mock("@features/csm-users/api/useGetUserById", () => ({
  useGetUserById: () => useGetUserByIdMock(),
}));

// Imported after the mocks above so the module picks them up.
import UserProfilePage from "@features/csm-users/pages/UserProfilePage";

const INTERNAL_USER: NormalizedUserDetail = {
  id: "user-1",
  userName: "jane.doe",
  name: "Jane Doe",
  email: "jane.doe@example.com",
  timezone: "UTC",
  userType: "internal",
  active: true,
  roles: ["internal", "agent"],
  phone: "+10000000000",
  createdOn: "2025-01-01T00:00:00Z",
  updatedOn: "2025-06-01T00:00:00Z",
  groups: [{ id: "grp-1", name: "Tier 2 support" }],
  teams: [{ id: "team-1", name: "CRE", family: "CRE" }],
};

const BLOCKED_EXTERNAL_USER: NormalizedUserDetail = {
  id: "user-2",
  userName: "john.smith",
  name: "John Smith",
  email: "john.smith@example.com",
  timezone: null,
  userType: "external",
  active: true,
  roles: ["customer"],
  createdOn: "2025-01-01T00:00:00Z",
  updatedOn: "2025-06-01T00:00:00Z",
  projectAccess: [
    {
      projectId: "proj-1",
      projectName: "Payments Platform",
      contactEmail: "john.smith@example.com",
      contactRecordPresent: false,
      emailMatchesLogin: false,
      grantsCaseAccess: false,
    },
    {
      projectId: "proj-2",
      projectName: "Identity Platform",
      contactEmail: "john.smith@example.com",
      contactRecordPresent: true,
      contactRecordEmail: "john.smith@example.com",
      emailMatchesLogin: true,
      registrationState: "registered",
      notificationsEnabled: true,
      roles: ["viewer"],
      grantsCaseAccess: true,
    },
  ],
};

function mockQueryResult(
  overrides: Partial<UseQueryResult<NormalizedUserDetail | null, Error>>,
): void {
  useGetUserByIdMock.mockReturnValue({
    data: null,
    isLoading: false,
    isError: false,
    error: null,
    ...overrides,
  });
}

function renderPage(): ReturnType<typeof render> {
  return render(
    <MemoryRouter>
      <UserProfilePage />
    </MemoryRouter>,
  );
}

describe("UserProfilePage", () => {
  it("renders a loading skeleton while the query is pending", () => {
    mockQueryResult({ isLoading: true });
    const { container } = renderPage();
    expect(container.querySelectorAll(".MuiSkeleton-root").length).toBeGreaterThan(0);
  });

  it("renders an error state when the query fails", () => {
    mockQueryResult({ isError: true, error: new Error("boom") });
    renderPage();
    expect(screen.getByText(/Failed to load user/i)).toBeInTheDocument();
  });

  it("renders a not-found state when the user is null", () => {
    mockQueryResult({ data: null });
    renderPage();
    expect(screen.getByText(/User not found/i)).toBeInTheDocument();
  });

  it("renders an internal user's groups and teams, with roles, phone and timestamps", () => {
    mockQueryResult({ data: INTERNAL_USER });
    renderPage();
    expect(screen.getByText("Jane Doe")).toBeInTheDocument();
    expect(screen.getByText("Internal", { selector: ".MuiChip-label" })).toBeInTheDocument();
    expect(screen.getByText("+10000000000")).toBeInTheDocument();
    expect(screen.getByText("Tier 2 support")).toBeInTheDocument();
    expect(screen.getByText("CRE (CRE)")).toBeInTheDocument();
    expect(screen.getByText("agent", { selector: ".MuiChip-label" })).toBeInTheDocument();
  });

  it("renders 'No team assignments' rather than hiding the card when an internal user has no teams", () => {
    mockQueryResult({ data: { ...INTERNAL_USER, teams: [] } });
    renderPage();
    expect(screen.getByText("No team assignments.")).toBeInTheDocument();
  });

  it("renders the blocking reason for a project that doesn't grant an external user case access", () => {
    mockQueryResult({ data: BLOCKED_EXTERNAL_USER });
    renderPage();

    // The blocked project surfaces its reason...
    expect(screen.getByText("Payments Platform")).toBeInTheDocument();
    expect(screen.getByText("Blocked", { selector: ".MuiChip-label" })).toBeInTheDocument();
    expect(
      screen.getByText(/No contact record is linked to this project/i),
    ).toBeInTheDocument();

    // ...while the granted project shows no reason text at all.
    expect(screen.getByText("Identity Platform")).toBeInTheDocument();
    expect(screen.getByText("Has case access", { selector: ".MuiChip-label" })).toBeInTheDocument();
    expect(screen.queryByText(/doesn't match the login email/i)).not.toBeInTheDocument();

    expect(screen.getByText(/Blocked on 1 of 2 projects/i)).toBeInTheDocument();
  });

  it("renders a mismatched-email reason distinct from a missing contact record", () => {
    mockQueryResult({
      data: {
        ...BLOCKED_EXTERNAL_USER,
        projectAccess: [
          {
            projectId: "proj-3",
            projectName: "Analytics Platform",
            contactEmail: "john.smith@example.com",
            contactRecordPresent: true,
            contactRecordEmail: "j.smith@example.com",
            emailMatchesLogin: false,
            grantsCaseAccess: false,
          },
        ],
      },
    });
    renderPage();
    expect(
      screen.getByText(/Contact record email \(j\.smith@example\.com\) doesn't match the login email/i),
    ).toBeInTheDocument();
  });

  it("renders 'No project access records found' rather than hiding the card for an external user with none", () => {
    mockQueryResult({ data: { ...BLOCKED_EXTERNAL_USER, projectAccess: [] } });
    renderPage();
    expect(
      screen.getByText(/No project access records found for this user/i),
    ).toBeInTheDocument();
  });

  it("calls out an inactive account as blocking access to every project", () => {
    mockQueryResult({ data: { ...BLOCKED_EXTERNAL_USER, active: false } });
    renderPage();
    expect(screen.getByText(/account is inactive/i)).toBeInTheDocument();
  });
});
