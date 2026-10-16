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

import { renderHook } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import type { CaseTabState } from "@context/case-tabs/caseTabsTypes";

const streamMock = vi.fn();
vi.mock("@features/csm-cases/api/useMultiplexedCaseStream", () => ({
  MAX_STREAM_CASE_IDS: 20,
  useMultiplexedCaseStream: (...args: unknown[]) => streamMock(...args),
}));

const controllerState: {
  tabs: CaseTabState[];
  activeTab?: CaseTabState;
  markTabUpdated: ReturnType<typeof vi.fn>;
} = { tabs: [], markTabUpdated: vi.fn() };
vi.mock("@context/case-tabs/CaseTabsContext", () => ({
  useCaseTabsController: () => controllerState,
}));

const { useCaseTabsLiveUpdates, selectStreamCaseIds } = await import("./useCaseTabsLiveUpdates");

const uuid = (n: number) => `00000000-0000-0000-0000-${String(n).padStart(12, "0")}`;

function tab(n: number, kind: CaseTabState["kind"] = "case"): CaseTabState {
  return {
    id: `t${n}`,
    caseId: uuid(n),
    kind,
    path: `/cases/${uuid(n)}`,
    hasDraft: false,
  };
}

function renderAt(pathname: string) {
  return renderHook(() => useCaseTabsLiveUpdates(), {
    wrapper: ({ children }: { children: ReactNode }) => (
      <MemoryRouter initialEntries={[pathname]}>{children}</MemoryRouter>
    ),
  });
}

const lastIds = () => streamMock.mock.calls[streamMock.mock.calls.length - 1][0] as string[];
const lastCallback = () =>
  streamMock.mock.calls[streamMock.mock.calls.length - 1][1] as (id: string) => void;

describe("selectStreamCaseIds", () => {
  it("puts the on-screen case first, then tabs newest-first, de-duplicated", () => {
    const tabs = [tab(1), tab(2), tab(3)];
    expect(selectStreamCaseIds(tabs, uuid(2), uuid(2))).toEqual([uuid(2), uuid(3), uuid(1)]);
  });

  it("caps the list, keeping the on-screen and most recent ones", () => {
    const tabs = Array.from({ length: 25 }, (_, i) => tab(i + 1));
    const ids = selectStreamCaseIds(tabs, undefined, uuid(1));
    expect(ids).toHaveLength(20);
    expect(ids[0]).toBe(uuid(1));
    expect(ids).toContain(uuid(25));
    expect(ids).not.toContain(uuid(2));
  });

  it("treats ids that differ only by case as one", () => {
    expect(selectStreamCaseIds([tab(1)], uuid(1).toUpperCase(), undefined)).toHaveLength(1);
  });
});

describe("useCaseTabsLiveUpdates", () => {
  beforeEach(() => {
    streamMock.mockReset();
    controllerState.markTabUpdated = vi.fn();
    controllerState.tabs = [];
    controllerState.activeTab = undefined;
  });

  it("subscribes to every open case tab", () => {
    controllerState.tabs = [tab(1), tab(2)];
    controllerState.activeTab = controllerState.tabs[1];
    renderAt("/dashboard");
    expect([...lastIds()].sort()).toEqual([uuid(1), uuid(2)]);
  });

  it("leaves out incident and change-request tabs and ids that are not UUIDs", () => {
    controllerState.tabs = [
      tab(1),
      tab(2, "incident"),
      tab(3, "change_request"),
      { ...tab(4), caseId: "CS0004" },
    ];
    renderAt("/dashboard");
    expect(lastIds()).toEqual([uuid(1)]);
  });

  it("also covers the case on screen when it has no tab of its own", () => {
    controllerState.tabs = [tab(1)];
    renderAt(`/cases/${uuid(9)}`);
    expect([...lastIds()].sort()).toEqual([uuid(1), uuid(9)]);
  });

  it("covers the case on screen when no tabs are open at all (tabs disabled)", () => {
    renderAt(`/cases/${uuid(9)}`);
    expect(lastIds()).toEqual([uuid(9)]);
  });

  it("flags the updated tab, telling the reducer the active tab is on screen when its case is routed", () => {
    controllerState.tabs = [tab(1), tab(2)];
    controllerState.activeTab = controllerState.tabs[1];
    renderAt(`/cases/${uuid(2)}`);
    lastCallback()(uuid(1));
    expect(controllerState.markTabUpdated).toHaveBeenCalledWith(uuid(1), true);
  });

  it("reports the active tab as not on screen while another page is showing", () => {
    controllerState.tabs = [tab(1), tab(2)];
    controllerState.activeTab = controllerState.tabs[1];
    renderAt("/dashboard");
    lastCallback()(uuid(2));
    expect(controllerState.markTabUpdated).toHaveBeenCalledWith(uuid(2), false);
  });
});
