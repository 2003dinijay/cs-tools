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
import { render as rtlRender, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter } from "react-router";
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

const mockedUseSearch = vi.mocked(useSearchProjects);

function render(ui: ReactElement, initialPath = "/customers/projects"): ReturnType<typeof rtlRender> {
  return rtlRender(<MemoryRouter initialEntries={[initialPath]}>{ui}</MemoryRouter>);
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
});
