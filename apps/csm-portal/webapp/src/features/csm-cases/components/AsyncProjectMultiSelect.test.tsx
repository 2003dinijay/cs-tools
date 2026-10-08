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
import { afterEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import AsyncProjectMultiSelect from "@features/csm-cases/components/AsyncProjectMultiSelect";
import { useInfiniteProjectSearch } from "@features/csm-cases/api/useProjectSearch";

vi.mock("@features/csm-cases/api/useProjectSearch", () => ({
  useInfiniteProjectSearch: vi.fn(),
}));

const mockedUseInfiniteProjectSearch = vi.mocked(useInfiniteProjectSearch);

afterEach(() => {
  mockedUseInfiniteProjectSearch.mockReset();
});

const PROJECTS = [
  { id: "p1", name: "Customer 3 Project - Managed Cloud Subscription" },
  { id: "p2", name: "CP Ph2 Test Project - Managed Cloud Subscription" },
];

function mockResults(): void {
  mockedUseInfiniteProjectSearch.mockReturnValue({
    projects: PROJECTS,
    isFetching: false,
    isFetchingNextPage: false,
    hasNextPage: false,
    isError: false,
    fetchNextPage: vi.fn(),
  });
}

describe("AsyncProjectMultiSelect — search text vs. selected-names summary", () => {
  it("does not render the selected-names summary while the dropdown is open, so it can't collide with the typed search text", () => {
    mockResults();
    const onChange = vi.fn();
    render(<AsyncProjectMultiSelect values={["p1"]} onChange={onChange} />);

    const input = screen.getByRole("combobox");
    fireEvent.mouseDown(input);
    fireEvent.change(input, { target: { value: "managed" } });

    expect(screen.getByDisplayValue("managed")).toBeInTheDocument();
    // The project's own name still legitimately appears once, as the open
    // dropdown's own list option (with its checkbox ticked) — the bug this
    // guards against is a *second* copy appearing in the tags/summary area,
    // concatenated right next to the typed "managed" in the same input row.
    expect(
      screen.getAllByText("Customer 3 Project - Managed Cloud Subscription"),
    ).toHaveLength(1);
  });

  it("clears the typed search text and shows the selected-names summary once the dropdown closes", () => {
    mockResults();
    const onChange = vi.fn();
    render(<AsyncProjectMultiSelect values={["p1"]} onChange={onChange} />);

    const input = screen.getByRole("combobox");
    fireEvent.mouseDown(input);
    fireEvent.change(input, { target: { value: "managed" } });
    fireEvent.keyDown(input, { key: "Escape" });

    expect(screen.queryByDisplayValue("managed")).not.toBeInTheDocument();
    expect(
      screen.getByText("Customer 3 Project - Managed Cloud Subscription"),
    ).toBeInTheDocument();
  });
});
