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

import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@asgardeo/react", () => ({
  useAsgardeo: () => ({ isSignedIn: true }),
}));

vi.mock("@features/csm-recent/hooks/useRecentViews", () => ({
  useRecentViews: () => [],
}));

vi.mock("@config/featureFlags", () => ({
  navigableNavNodes: () => [],
}));

const quickCaseSearchMock = vi.fn();
vi.mock("@features/csm-cases/api/useQuickCaseSearch", () => ({
  QUICK_CASE_MIN_QUERY_LEN: 2,
  useQuickCaseSearch: (q: string) => quickCaseSearchMock(q),
}));

const quickIncidentSearchMock = vi.fn();
vi.mock("@features/csm-operations/api/useQuickIncidentSearch", () => ({
  QUICK_INCIDENT_MIN_QUERY_LEN: 2,
  useQuickIncidentSearch: (q: string) => quickIncidentSearchMock(q),
}));

const quickChangeRequestSearchMock = vi.fn();
vi.mock("@features/csm-operations/api/useQuickChangeRequestSearch", () => ({
  QUICK_CHANGE_REQUEST_MIN_QUERY_LEN: 2,
  useQuickChangeRequestSearch: (q: string) => quickChangeRequestSearchMock(q),
}));

const quickProblemSearchMock = vi.fn();
vi.mock("@features/csm-operations/api/useQuickProblemSearch", () => ({
  QUICK_PROBLEM_MIN_QUERY_LEN: 2,
  useQuickProblemSearch: (q: string) => quickProblemSearchMock(q),
}));

const navigateMock = vi.fn();
vi.mock("@hooks/useNavTransition", () => ({
  useNavTransition: () => navigateMock,
}));

const { default: QuickNav } = await import("./QuickNav");

const idleResult = { data: undefined, isFetching: false };

function renderQuickNav(initialEntries?: string[]) {
  return render(
    <MemoryRouter initialEntries={initialEntries}>
      <QuickNav />
    </MemoryRouter>,
  );
}

async function openAndType(query: string) {
  renderQuickNav();
  fireEvent.click(screen.getByLabelText("Search or jump to (open quick nav)"));
  const input = await screen.findByLabelText("Quick nav search");
  fireEvent.change(input, { target: { value: query } });
  // Wait past the 180ms debounce for the search hooks to be called with the
  // settled query.
  await waitFor(() =>
    expect(quickCaseSearchMock).toHaveBeenLastCalledWith(query),
  );
}

describe("QuickNav", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    quickCaseSearchMock.mockReturnValue(idleResult);
    quickIncidentSearchMock.mockReturnValue(idleResult);
    quickChangeRequestSearchMock.mockReturnValue(idleResult);
    quickProblemSearchMock.mockReturnValue(idleResult);
  });

  it("renders an 'Incidents' section for a live incident hit and links to the incident route", async () => {
    quickIncidentSearchMock.mockReturnValue({
      data: [
        {
          id: "inc-1",
          number: "INC0001234",
          subject: "Prod cluster down",
          state: "IN_PROGRESS",
          assigneeName: "Jane Doe",
        },
      ],
      isFetching: false,
    });

    await openAndType("cluster");

    expect(await screen.findByText("Incidents")).toBeInTheDocument();
    expect(screen.getByText("INC0001234")).toBeInTheDocument();
    expect(screen.getByText("Prod cluster down")).toBeInTheDocument();

    fireEvent.click(screen.getByText("Prod cluster down"));
    expect(navigateMock).toHaveBeenCalledWith("/operations/incidents/inc-1");
  });

  it("renders a 'Change Requests' section for a live CR hit and links to the change-request route", async () => {
    quickChangeRequestSearchMock.mockReturnValue({
      data: [
        {
          id: "cr-1",
          number: "CHG0005",
          subject: "Upgrade the API gateway",
          state: "assess",
          assigneeName: "John Smith",
        },
      ],
      isFetching: false,
    });

    await openAndType("upgrade");

    expect(await screen.findByText("Change Requests")).toBeInTheDocument();
    expect(screen.getByText("CHG0005")).toBeInTheDocument();

    fireEvent.click(screen.getByText("Upgrade the API gateway"));
    expect(navigateMock).toHaveBeenCalledWith("/operations/change-requests/cr-1");
  });

  it("shows no entity sections beyond Pages while nothing has matched", async () => {
    await openAndType("zz");
    expect(screen.queryByText("Incidents")).not.toBeInTheDocument();
    expect(screen.queryByText("Change Requests")).not.toBeInTheDocument();
    expect(screen.queryByText("Problems")).not.toBeInTheDocument();
    expect(screen.getByText("No matches.")).toBeInTheDocument();
  });

  it("opens pre-filled and searching from a `?q=` link", async () => {
    renderQuickNav(["/?q=CS0440883"]);

    const input = await screen.findByLabelText("Quick nav search");
    expect(input).toHaveValue("CS0440883");
    await waitFor(() =>
      expect(quickCaseSearchMock).toHaveBeenLastCalledWith("CS0440883"),
    );
  });

  it("auto-navigates to the single record a `?goto=` link resolves to", async () => {
    quickIncidentSearchMock.mockReturnValue({
      data: [
        {
          id: "inc-1",
          number: "INC0001234",
          subject: "Prod cluster down",
          state: "IN_PROGRESS",
        },
      ],
      isFetching: false,
    });

    renderQuickNav(["/?goto=INC0001234"]);

    await waitFor(() =>
      expect(navigateMock).toHaveBeenCalledWith("/operations/incidents/inc-1"),
    );
  });

  it("leaves the palette open when `?goto=` matches nothing", async () => {
    renderQuickNav(["/?goto=NOPE0000"]);

    const input = await screen.findByLabelText("Quick nav search");
    await waitFor(() =>
      expect(quickCaseSearchMock).toHaveBeenLastCalledWith("NOPE0000"),
    );
    await waitFor(() => expect(screen.getByText("No matches.")).toBeInTheDocument());
    expect(input).toBeInTheDocument();
    expect(navigateMock).not.toHaveBeenCalled();
  });
});
