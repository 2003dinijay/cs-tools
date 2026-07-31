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
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import type * as React from "react";
import { DEFAULT_INCIDENT_FILTERS } from "@features/csm-operations/utils/incidents";
import IncidentsFilterBar from "@features/csm-operations/components/IncidentsFilterBar";

// The product multi-select goes through a bulk catalogue fetch — stub it out
// so this test doesn't need a QueryClientProvider.
const useIncidentProductNameOptionsMock = vi.fn(() => ({
  data: ["Choreo", "Asgardeo"],
  isFetching: false,
  isError: false,
}));
vi.mock("@features/csm-operations/api/useIncidentProductNameOptions", () => ({
  useIncidentProductNameOptions: () => useIncidentProductNameOptionsMock(),
}));

/**
 * Render the filter bar over `DEFAULT_INCIDENT_FILTERS` with the given prop
 * overrides, returning the `onChange`/`onReset`/`onFiltersToggle` spies. The bar
 * is controlled, so a test asserts on the filter object handed to `onChange`
 * rather than on the input's own value.
 */
function renderFilterBar(
  overrides: Partial<React.ComponentProps<typeof IncidentsFilterBar>> = {},
) {
  const onChange = vi.fn();
  const onReset = vi.fn();
  const onFiltersToggle = vi.fn();
  render(
    <IncidentsFilterBar
      filters={DEFAULT_INCIDENT_FILTERS}
      onChange={onChange}
      onReset={onReset}
      isFiltersOpen
      onFiltersToggle={onFiltersToggle}
      {...overrides}
    />,
  );
  return { onChange, onReset, onFiltersToggle };
}

describe("IncidentsFilterBar", () => {
  it("labels both created-date bounds as UTC, so a picked day isn't mistaken for local time", () => {
    renderFilterBar();
    expect(screen.getAllByText(/created from \(utc\)/i).length).toBeGreaterThan(0);
    expect(screen.getAllByText(/created to \(utc\)/i).length).toBeGreaterThan(0);
  });

  it("renders the SLA-violated checkbox unchecked by default and toggles it on", () => {
    const { onChange } = renderFilterBar();
    const checkbox = screen.getByRole("checkbox", { name: /sla violated/i });
    expect(checkbox).not.toBeChecked();

    fireEvent.click(checkbox);

    expect(onChange).toHaveBeenCalledWith({
      ...DEFAULT_INCIDENT_FILTERS,
      slaViolated: true,
    });
  });

  it("reflects an already-active SLA-violated filter as checked", () => {
    renderFilterBar({ filters: { ...DEFAULT_INCIDENT_FILTERS, slaViolated: true } });
    expect(screen.getByRole("checkbox", { name: /sla violated/i })).toBeChecked();
  });

  it("surfaces the product filter's coverage caveat as visible helper text", () => {
    renderFilterBar();
    expect(
      screen.getByText(/only incidents with a recorded service can match/i),
    ).toBeInTheDocument();
  });

  it("shows the active-filter count and a Clear filters action once a filter is set", () => {
    renderFilterBar({ filters: { ...DEFAULT_INCIDENT_FILTERS, slaViolated: true } });
    expect(screen.getByRole("button", { name: /filters \(1\)/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /clear filters/i })).toBeInTheDocument();
  });

  it("shows no active-filter badge and no Clear filters action for the defaults", () => {
    renderFilterBar();
    expect(screen.getByRole("button", { name: /^filters$/i })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /clear filters/i })).not.toBeInTheDocument();
  });

  it("calls onReset when Clear filters is clicked", () => {
    const { onReset } = renderFilterBar({
      filters: { ...DEFAULT_INCIDENT_FILTERS, products: ["Choreo"] },
    });
    fireEvent.click(screen.getByRole("button", { name: /clear filters/i }));
    expect(onReset).toHaveBeenCalledTimes(1);
  });
});
