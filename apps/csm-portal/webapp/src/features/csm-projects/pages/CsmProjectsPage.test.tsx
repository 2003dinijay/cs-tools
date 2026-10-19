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

import type { ReactElement } from "react";
import { fireEvent, render as rtlRender, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter, Route, Routes, useLocation, useNavigate } from "react-router";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import "@testing-library/jest-dom/vitest";
import CsmProjectsPage from "@features/csm-projects/pages/CsmProjectsPage";
import { useSearchProjects } from "@features/csm-projects/api/useSearchProjects";

// The backend client reads runtime config at module load, which isn't
// present under vitest — same approach as the sibling accounts/cases page
// tests.
vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {},
  useBackendApi: () => ({ post: vi.fn() }),
}));

vi.mock("@features/csm-projects/api/useSearchProjects", () => ({
  useSearchProjects: vi.fn(),
}));

vi.mock("@context/current-user/CurrentUserContext", () => ({
  useCurrentUser: () => ({ user: { id: "user-1" }, isLoading: false, isError: false }),
}));

vi.mock("@hooks/useIdTokenClaims", () => ({
  useIdTokenClaims: () => ({ email: "user@example.test" }),
}));

const mockedUseSearch = vi.mocked(useSearchProjects);

function render(ui: ReactElement, initialPath = "/customers/projects"): ReturnType<typeof rtlRender> {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return rtlRender(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[initialPath]}>{ui}</MemoryRouter>
    </QueryClientProvider>,
  );
}

function mockResult(overrides: Partial<ReturnType<typeof useSearchProjects>>): void {
  mockedUseSearch.mockReturnValue({
    data: undefined,
    isLoading: false,
    isFetching: false,
    isError: false,
    error: null,
    refetch: vi.fn(),
    dataUpdatedAt: 0,
    ...overrides,
  } as unknown as ReturnType<typeof useSearchProjects>);
}

beforeEach(() => {
  mockedUseSearch.mockReset();
  window.localStorage.clear();
});

// Regression coverage for a real, reported bug: a dashboard tile like
// "Active Onboarding" (resourceType "project", query
// `{onboardingStatus: ["In-Progress"]}`) used to navigate to this page's
// bare, unfiltered path — the click-through dropped the filter entirely
// (see `widgetResourceConfig.ts`'s own `project.buildHref` fix). This page
// is the other half of that fix: it must actually read the
// `?onboardingStatus=` query param the corrected `buildHref` now sends.
describe("CsmProjectsPage — onboardingStatus filter from the URL", () => {
  it("reads a single onboardingStatus value from the URL and includes it in the search request", async () => {
    mockResult({ data: { projects: [], total: 0, limit: 20, offset: 0, hasMore: false } });

    render(<CsmProjectsPage />, "/customers/projects?onboardingStatus=In-Progress");

    await waitFor(() => {
      expect(mockedUseSearch).toHaveBeenCalledWith(
        expect.objectContaining({ onboardingStatus: ["In-Progress"] }),
      );
    });
  });

  it("reads multiple comma-separated onboardingStatus values from the URL", async () => {
    mockResult({ data: { projects: [], total: 0, limit: 20, offset: 0, hasMore: false } });

    render(<CsmProjectsPage />, "/customers/projects?onboardingStatus=In-Progress,OnHold");

    await waitFor(() => {
      expect(mockedUseSearch).toHaveBeenCalledWith(
        expect.objectContaining({ onboardingStatus: ["In-Progress", "OnHold"] }),
      );
    });
  });

  it("sends no onboardingStatus filter when the URL carries none (the default, unfiltered view)", async () => {
    mockResult({ data: { projects: [], total: 0, limit: 20, offset: 0, hasMore: false } });

    render(<CsmProjectsPage />);

    await waitFor(() => {
      expect(mockedUseSearch).toHaveBeenCalledWith(
        expect.objectContaining({ onboardingStatus: undefined }),
      );
    });
  });

  it("shows the filter control, reflecting the status read from the URL", async () => {
    mockResult({ data: { projects: [], total: 0, limit: 20, offset: 0, hasMore: false } });

    render(<CsmProjectsPage />, "/customers/projects?onboardingStatus=In-Progress");

    expect(await screen.findByText("In Progress")).toBeInTheDocument();
  });

  // The onboardingStatus state used to be seeded only in a useState
  // initializer, which runs once at mount -- so a same-route navigation that
  // doesn't remount the page (e.g. one dashboard tile's click-through
  // followed by another, both landing on /customers/projects) left the
  // filter and the search request stuck on whatever the first mount read,
  // ignoring the URL's new onboardingStatus value. A sibling "navigate"
  // trigger inside the same <Routes> simulates that: the Route element
  // itself never unmounts, only its location/searchParams change.
  it("re-reads onboardingStatus when the URL changes without remounting the page", async () => {
    mockResult({ data: { projects: [], total: 0, limit: 20, offset: 0, hasMore: false } });

    function NavTrigger(): ReactElement {
      const navigate = useNavigate();
      return (
        <button onClick={() => navigate("/customers/projects?onboardingStatus=Completed")}>
          go to Completed
        </button>
      );
    }

    rtlRender(
      <MemoryRouter initialEntries={["/customers/projects?onboardingStatus=In-Progress"]}>
        <Routes>
          <Route
            path="/customers/projects"
            element={
              <>
                <NavTrigger />
                <CsmProjectsPage />
              </>
            }
          />
        </Routes>
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(mockedUseSearch).toHaveBeenCalledWith(
        expect.objectContaining({ onboardingStatus: ["In-Progress"] }),
      );
    });

    fireEvent.click(screen.getByRole("button", { name: "go to Completed" }));

    await waitFor(() => {
      expect(mockedUseSearch).toHaveBeenCalledWith(
        expect.objectContaining({ onboardingStatus: ["Completed"] }),
      );
    });
    expect(await screen.findByText("Completed")).toBeInTheDocument();
  });
});

// Regression coverage: this table used to have a fixed 5-column layout with
// no customisation at all, unlike the Cases table (CasesList.tsx) and the
// Accounts table (CsmAccountsPage.tsx), both of which let a caller show/hide/
// reorder columns via ColumnCustomizerButton + useColumnPreferences. Brought
// in line with that convention, plus two new opt-in columns (onboarding
// status/owner) the table previously had nowhere to show at all.
describe("CsmProjectsPage — column customisation", () => {
  it("shows exactly the original 5 columns by default, with onboarding columns hidden", () => {
    mockResult({ data: { projects: [], total: 0, limit: 20, offset: 0, hasMore: false } });

    render(<CsmProjectsPage />);

    const headerRow = screen.getAllByRole("columnheader").map((h) => h.textContent);
    expect(headerRow).toEqual(["Name", "Project key", "State", "Start", "End"]);
  });

  it("offers a 'Customise projects columns' control", () => {
    mockResult({ data: { projects: [], total: 0, limit: 20, offset: 0, hasMore: false } });

    render(<CsmProjectsPage />);

    expect(
      screen.getByRole("button", { name: "Customise projects columns" }),
    ).toBeInTheDocument();
  });

  it("shows a project's onboarding status and owner once those columns are toggled on", async () => {
    mockResult({
      data: {
        projects: [
          {
            id: "proj-1",
            name: "Acme Project",
            key: "ACME",
            subscriptionType: "subscription",
            startDate: null,
            endDate: null,
            createdOn: "2024-01-01T00:00:00Z",
            closureState: "open",
            endDateClosureState: null,
            invoiceDueDateClosureState: null,
            complianceViolationClosureState: null,
            complianceViolationDate: null,
            suspensionProcessState: null,
            onboardingStatus: "In-Progress",
            onboardingOwner: { id: "user-2", name: "Jane Doe", email: "jane.doe@example.com" },
          },
        ],
        total: 1,
        limit: 20,
        offset: 0,
        hasMore: false,
      },
    });

    render(<CsmProjectsPage />);

    fireEvent.click(screen.getByRole("button", { name: "Customise projects columns" }));
    // "Onboarding status" also names the filter's MultiSelectField label, so
    // scope to the picker's own list (aria-labelled with the same button
    // label) to avoid an ambiguous match.
    const picker = screen.getByRole("list", { name: "Customise projects columns" });
    fireEvent.click(within(picker).getByText("Onboarding status"));
    fireEvent.click(within(picker).getByText("Onboarding owner"));

    expect(screen.getAllByText("In Progress").length).toBeGreaterThan(0);
    expect(await screen.findByText("Jane Doe")).toBeInTheDocument();
  });

  // Regression (CodeRabbit, PR #2907): the owner cell used to render
  // whenever `onboardingOwner` was set, regardless of `onboardingStatus` —
  // but a "Not-Applicable" status means no active onboarding engagement, the
  // same condition the detail page's own owner cell is gated on.
  it("shows a dash for the owner column when onboardingStatus is Not-Applicable, even with an owner set", async () => {
    mockResult({
      data: {
        projects: [
          {
            id: "proj-1",
            name: "Acme Project",
            key: "ACME",
            subscriptionType: "subscription",
            startDate: null,
            endDate: null,
            createdOn: "2024-01-01T00:00:00Z",
            closureState: "open",
            endDateClosureState: null,
            invoiceDueDateClosureState: null,
            complianceViolationClosureState: null,
            complianceViolationDate: null,
            suspensionProcessState: null,
            onboardingStatus: "Not-Applicable",
            onboardingOwner: { id: "user-2", name: "Jane Doe", email: "jane.doe@example.com" },
          },
        ],
        total: 1,
        limit: 20,
        offset: 0,
        hasMore: false,
      },
    });

    render(<CsmProjectsPage />);

    fireEvent.click(screen.getByRole("button", { name: "Customise projects columns" }));
    const picker = screen.getByRole("list", { name: "Customise projects columns" });
    fireEvent.click(within(picker).getByText("Onboarding owner"));

    expect(screen.queryByText("Jane Doe")).not.toBeInTheDocument();
  });

  // Regression (CodeRabbit, PR #2907): the owner cell's UserRefLink had no
  // way to stop its click/keydown from bubbling into the row's own
  // onClick/onKeyDown, so activating the owner's link navigated to the
  // project instead of the owner's own profile page.
  it("clicking the owner link opens the owner's profile, not the project", async () => {
    mockResult({
      data: {
        projects: [
          {
            id: "proj-1",
            name: "Acme Project",
            key: "ACME",
            subscriptionType: "subscription",
            startDate: null,
            endDate: null,
            createdOn: "2024-01-01T00:00:00Z",
            closureState: "open",
            endDateClosureState: null,
            invoiceDueDateClosureState: null,
            complianceViolationClosureState: null,
            complianceViolationDate: null,
            suspensionProcessState: null,
            onboardingStatus: "In-Progress",
            onboardingOwner: { id: "user-2", name: "Jane Doe", email: "jane.doe@example.com" },
          },
        ],
        total: 1,
        limit: 20,
        offset: 0,
        hasMore: false,
      },
    });

    function LocationProbe(): ReactElement {
      const location = useLocation();
      return <div data-testid="location-probe">{location.pathname}</div>;
    }

    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    rtlRender(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter initialEntries={["/customers/projects"]}>
          <Routes>
            <Route
              path="/customers/projects"
              element={
                <>
                  <CsmProjectsPage />
                  <LocationProbe />
                </>
              }
            />
            <Route path="/customers/projects/:id" element={<LocationProbe />} />
            <Route path="/people/:id" element={<LocationProbe />} />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>,
    );

    fireEvent.click(screen.getByRole("button", { name: "Customise projects columns" }));
    const picker = screen.getByRole("list", { name: "Customise projects columns" });
    fireEvent.click(within(picker).getByText("Onboarding owner"));
    // Close the popover — MUI's Modal marks the rest of the page aria-hidden
    // while it's open, which hides the owner link from the accessibility
    // tree and makes the getByRole query below fail to find it.
    fireEvent.keyDown(picker, { key: "Escape" });

    fireEvent.click(await screen.findByRole("link", { name: "Jane Doe" }));

    expect(await screen.findByTestId("location-probe")).toHaveTextContent("/people/user-2");
  });
});
