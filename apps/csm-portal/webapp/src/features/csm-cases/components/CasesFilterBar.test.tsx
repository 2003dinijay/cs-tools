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

import { fireEvent, render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import type { ReactNode } from "react";
import CasesFilterBar, {
  type CasesFilters,
} from "@features/csm-cases/components/CasesFilterBar";
import { DEFAULT_CASES_FILTERS } from "@features/csm-cases/utils/casesFiltersUrl";

const postMock = vi.fn();

vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock, get: vi.fn() }),
}));

function renderBar(
  filters: CasesFilters,
  onChange = vi.fn(),
  extraProps: Partial<Parameters<typeof CasesFilterBar>[0]> = {},
): { onChange: ReturnType<typeof vi.fn> } {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  render(
    <QueryClientProvider client={queryClient}>
      <CasesFilterBar
        filters={filters}
        onChange={onChange}
        onReset={() => {}}
        isFiltersOpen
        onFiltersToggle={() => {}}
        availableAssigneeUsers={[]}
        availableProjects={[]}
        {...extraProps}
      />
    </QueryClientProvider> as ReactNode,
  );
  return { onChange };
}

describe("CasesFilterBar — active-filter chips for URL-only fields", () => {
  beforeEach(() => {
    postMock.mockReset();
    postMock.mockResolvedValue({ teams: [] });
  });

  it("renders no chips when nothing is active", () => {
    renderBar({ ...DEFAULT_CASES_FILTERS });
    expect(screen.queryByText(/SLA/)).not.toBeInTheDocument();
    expect(screen.queryByText(/Escalat/)).not.toBeInTheDocument();
    expect(screen.queryByText(/CS team:/)).not.toBeInTheDocument();
    expect(screen.queryByText(/^Tag:/)).not.toBeInTheDocument();
  });

  it("renders one chip per URL-only filter and each is independently removable", () => {
    const { onChange } = renderBar({
      ...DEFAULT_CASES_FILTERS,
      slaElapsedPctGte: 80,
      hasEscalation: true,
      onboardingStatuses: ["in_progress"],
      createdOnGte: "2026-07-27",
    });

    expect(screen.getByText("SLA ≥ 80%")).toBeInTheDocument();
    expect(screen.getByText("Escalated")).toBeInTheDocument();
    expect(screen.getByText("Onboarding: In progress")).toBeInTheDocument();
    expect(screen.getByText(/Created after/)).toBeInTheDocument();

    // Removing the SLA chip clears only slaElapsedPctGte, nothing else.
    const slaChip = screen.getByText("SLA ≥ 80%");
    const deleteIcon = slaChip.parentElement?.querySelector(
      '[data-testid="CancelIcon"], svg',
    );
    fireEvent.click(deleteIcon ?? slaChip);

    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({
        slaElapsedPctGte: null,
        hasEscalation: true,
        onboardingStatuses: ["in_progress"],
        createdOnGte: "2026-07-27",
      }),
    );
  });

  it("clearing one onboarding-status chip removes only that value, keeping siblings", () => {
    const { onChange } = renderBar({
      ...DEFAULT_CASES_FILTERS,
      onboardingStatuses: ["in_progress", "OnHold"],
    });

    expect(screen.getByText("Onboarding: In progress")).toBeInTheDocument();
    expect(screen.getByText("Onboarding: On hold")).toBeInTheDocument();

    const chip = screen.getByText("Onboarding: In progress");
    const deleteIcon = chip.parentElement?.querySelector("svg");
    fireEvent.click(deleteIcon ?? chip);

    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ onboardingStatuses: ["OnHold"] }),
    );
  });

  it("both SLA bounds render and clear independently", () => {
    const { onChange } = renderBar({
      ...DEFAULT_CASES_FILTERS,
      slaElapsedPctGte: 80,
      slaElapsedPctLte: 100,
    });

    expect(screen.getByText("SLA ≥ 80%")).toBeInTheDocument();
    expect(screen.getByText("SLA ≤ 100%")).toBeInTheDocument();

    const chip = screen.getByText("SLA ≤ 100%");
    const deleteIcon = chip.parentElement?.querySelector("svg");
    fireEvent.click(deleteIcon ?? chip);

    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ slaElapsedPctGte: 80, slaElapsedPctLte: null }),
    );
  });
});


describe("CasesFilterBar — removed bar controls fall back to chips", () => {
  beforeEach(() => {
    postMock.mockReset();
  });


  /**
   * The CS-team and tag bar controls were removed as clutter, so a chip is now
   * the ONLY way these filters are visible or clearable after a dashboard
   * click-through. If these break, a user lands on a filtered list with no way
   * to see or undo why.
   */
  it("renders chips for csTeams/tags/excludeTags now that their bar controls are gone", () => {
    const { onChange } = renderBar({
      ...DEFAULT_CASES_FILTERS,
      csTeams: ["g1"],
      tags: ["micro-gw"],
      excludeTags: ["s_dip"],
    });

    expect(screen.getByText("Tag: micro-gw")).toBeInTheDocument();
    expect(screen.getByText("Excluding tag: s_dip")).toBeInTheDocument();
    // Team name is unresolved here (no teams fetched), so it falls back to the
    // id rather than hiding the chip.
    expect(screen.getByText(/CS team: /)).toBeInTheDocument();

    fireEvent.click(screen.getByText("Excluding tag: s_dip").closest(".MuiChip-root")!.querySelector("svg")!);
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ excludeTags: [], tags: ["micro-gw"], csTeams: ["g1"] }),
    );
  });

  it("no longer renders the removed CS team / Tags / Exclude tags bar controls", () => {
    renderBar({ ...DEFAULT_CASES_FILTERS });
    expect(screen.queryByLabelText(/^CS team$/)).not.toBeInTheDocument();
    expect(screen.queryByLabelText(/^Tags$/)).not.toBeInTheDocument();
    expect(screen.queryByLabelText(/^Exclude tags$/)).not.toBeInTheDocument();
  });
});
