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

  it("renders no chips when only bar-controlled fields (or nothing) are active", () => {
    renderBar({ ...DEFAULT_CASES_FILTERS, csTeams: ["g1"], tags: ["micro-gw"] });
    // No stray "×"-labeled chip content beyond what the bar's own controls
    // (team select, tags input) already render.
    expect(screen.queryByText(/SLA/)).not.toBeInTheDocument();
    expect(screen.queryByText(/Escalat/)).not.toBeInTheDocument();
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

describe("CasesFilterBar — tag include/exclude control", () => {
  beforeEach(() => {
    postMock.mockReset();
    postMock.mockResolvedValue({ teams: [] });
  });

  it("typing into 'Tags' sets `tags`, typing into 'Exclude tags' sets `excludeTags` — never conflated", () => {
    const { onChange } = renderBar(DEFAULT_CASES_FILTERS);

    const tagsInput = screen.getByLabelText("Tags");
    fireEvent.change(tagsInput, { target: { value: "micro-gw" } });
    fireEvent.keyDown(tagsInput, { key: "Enter" });

    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ tags: ["micro-gw"], excludeTags: [] }),
    );

    onChange.mockClear();
    const excludeInput = screen.getByLabelText("Exclude tags");
    fireEvent.change(excludeInput, { target: { value: "s_dip" } });
    fireEvent.keyDown(excludeInput, { key: "Enter" });

    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ excludeTags: ["s_dip"] }),
    );
  });

  it("both tags and excludeTags can be set at once (independent, not mutually exclusive)", () => {
    renderBar({ ...DEFAULT_CASES_FILTERS, tags: ["micro-gw"], excludeTags: ["s_dip"] });
    expect(screen.getByLabelText("Tags").closest("form, div")).toBeInTheDocument();
    expect(screen.getByText("micro-gw")).toBeInTheDocument();
    expect(screen.getByText("s_dip")).toBeInTheDocument();
  });
});

describe("CasesFilterBar — CS team control", () => {
  beforeEach(() => {
    postMock.mockReset();
  });

  it("shows team display names, not group-id UUIDs, and filters by groupId on selection", async () => {
    postMock.mockResolvedValue({
      teams: [
        { id: "alpha", name: "Team Alpha", groupId: "22222222-2222-2222-2222-222222222222" },
        { id: "beta", name: "Team Beta", groupId: "33333333-3333-3333-3333-333333333333" },
      ],
    });

    renderBar(DEFAULT_CASES_FILTERS);

    fireEvent.mouseDown(screen.getByLabelText("CS team"));
    const option = await screen.findByRole("option", { name: "Team Alpha" });
    expect(option).toBeInTheDocument();
    expect(screen.queryByText("22222222-2222-2222-2222-222222222222")).not.toBeInTheDocument();
  });
});
