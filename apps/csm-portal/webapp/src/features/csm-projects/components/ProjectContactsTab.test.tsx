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
import { QueryClient, QueryClientProvider, type UseQueryResult } from "@tanstack/react-query";
import type { BeProjectContact } from "@api/backend/types";

const useSearchProjectContactsMock = vi.fn();
const postMock = vi.fn();

vi.mock("@features/csm-projects/api/useSearchProjectContacts", () => ({
  useSearchProjectContacts: () => useSearchProjectContactsMock(),
}));
// The backend client reads runtime config (`CSM_PORTAL_BACKEND_BASE_URL`) at
// module load, which isn't present under vitest. `QueryErrorState` imports
// `BackendApiError` from it directly, so stub the module with a real class
// (so `instanceof` still works) — same approach as
// CsmChangeRequestDetailPage.test.tsx. UserRefLink resolves an unknown id
// through `POST /users/search` via `useBackendApi`.
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

// Imported after the mocks above so the module picks them up.
import ProjectContactsTab from "@features/csm-projects/components/ProjectContactsTab";

function mockQueryResult(
  overrides: Partial<UseQueryResult<BeProjectContact[], Error>>,
): void {
  useSearchProjectContactsMock.mockReturnValue({
    data: [],
    isLoading: false,
    isError: false,
    error: null,
    ...overrides,
  });
}

function renderTab(): ReturnType<typeof render> {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <ProjectContactsTab projectId="proj-1" />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

const LINKED_CONTACT: BeProjectContact = {
  id: "00000000-0000-0000-0000-000000000000",
  name: "Jane Doe",
  email: "jane.doe@example.com",
  registrationState: "registered",
  notificationsEnabled: true,
  roles: ["viewer"],
};

const ORPHANED_CONTACT: BeProjectContact = {
  // No `id` — this row has no linked contact record.
  name: "John Smith",
  email: "john.smith@example.com",
  registrationState: "pending",
};

// The more common real-world shape: no `id`, no `name`, *and* no `email` —
// nothing to key the row on at all.
const FULLY_BLANK_ORPHANED_CONTACT: BeProjectContact = {
  registrationState: "pending",
};

describe("ProjectContactsTab", () => {
  it("renders a loading skeleton while the query is pending", () => {
    mockQueryResult({ isLoading: true });
    const { container } = renderTab();
    expect(container.querySelectorAll(".MuiSkeleton-root").length).toBeGreaterThan(0);
  });

  it("renders an error state when the query fails", () => {
    mockQueryResult({ isError: true, error: new Error("boom") });
    renderTab();
    expect(screen.getByText(/Failed to load project contacts/i)).toBeInTheDocument();
  });

  it("renders an empty state rather than an empty table when the project has no contacts", () => {
    mockQueryResult({ data: [] });
    renderTab();
    expect(screen.getByText(/No contacts found for this project/i)).toBeInTheDocument();
  });

  it("renders a linked contact as a clickable profile link", () => {
    postMock.mockResolvedValue({ users: [] });
    mockQueryResult({ data: [LINKED_CONTACT] });
    renderTab();
    const link = screen.getByRole("link", { name: "Jane Doe" });
    expect(link).toHaveAttribute("href", `/people/${LINKED_CONTACT.id}`);
    expect(screen.getByText("registered")).toBeInTheDocument();
    expect(screen.getByText("viewer", { selector: ".MuiChip-label" })).toBeInTheDocument();
  });

  it("renders an orphaned contact (no id) without a link, flagged, and without crashing", () => {
    postMock.mockResolvedValue({ users: [] });
    mockQueryResult({ data: [ORPHANED_CONTACT] });
    renderTab();
    expect(screen.getByText("John Smith")).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "John Smith" })).not.toBeInTheDocument();
    expect(screen.getByText("Orphaned", { selector: ".MuiChip-label" })).toBeInTheDocument();
  });

  it("renders a fully-blank orphaned row (no id, name, or email) with a clearly-marked, always-visible reason rather than a blank row", () => {
    postMock.mockResolvedValue({ users: [] });
    mockQueryResult({ data: [FULLY_BLANK_ORPHANED_CONTACT] });
    renderTab();

    // Never a bare blank cell — the row states plainly that it has no
    // linked contact record...
    expect(screen.getByText("No linked contact record")).toBeInTheDocument();
    // ...and the operationally important consequence is visible inline
    // (not hidden behind a hover-only tooltip).
    expect(
      screen.getByText(/can't see this project's cases/i),
    ).toBeInTheDocument();
    expect(screen.getByText("Orphaned", { selector: ".MuiChip-label" })).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: "No linked contact record" }),
    ).not.toBeInTheDocument();
  });

  it("renders both a linked and an orphaned row together without crashing", () => {
    postMock.mockResolvedValue({ users: [] });
    mockQueryResult({ data: [LINKED_CONTACT, ORPHANED_CONTACT] });
    renderTab();
    expect(screen.getByRole("link", { name: "Jane Doe" })).toBeInTheDocument();
    expect(screen.getByText("John Smith")).toBeInTheDocument();
    expect(screen.getByText("Orphaned", { selector: ".MuiChip-label" })).toBeInTheDocument();
  });
});
