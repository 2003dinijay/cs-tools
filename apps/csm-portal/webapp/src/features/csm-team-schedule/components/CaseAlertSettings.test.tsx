/**
 * Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import type {
  CaseAlertChatRoom,
  CaseAlertSettings as Settings,
} from "../types";
// vi.mock below is hoisted above this import, so the section gets the mocks.
import CaseAlertSettings from "./CaseAlertSettings";

const settings: { data?: Settings } = {};
const saveRoom = { mutate: vi.fn(), mutateAsync: vi.fn(), isPending: false };
const setWebhook = { mutate: vi.fn(), mutateAsync: vi.fn(), isPending: false };
const testRoom = { mutate: vi.fn(), isPending: false };
const deleteRoom = { mutate: vi.fn(), isPending: false };
const saveRoute = { mutate: vi.fn(), isPending: false };
const deleteRoute = { mutate: vi.fn(), isPending: false };
const saveValues = { mutate: vi.fn(), isPending: false };
const deleteValue = { mutate: vi.fn(), isPending: false };

vi.mock("../api/useCaseAlertSettings", () => ({
  useCaseAlertSettings: () => ({
    data: settings.data,
    isLoading: false,
    isError: false,
  }),
  useSaveCaseAlertRoom: () => saveRoom,
  useSetCaseAlertWebhook: () => setWebhook,
  useTestCaseAlertRoom: () => testRoom,
  useDeleteCaseAlertRoom: () => deleteRoom,
  useSaveCaseAlertRoute: () => saveRoute,
  useDeleteCaseAlertRoute: () => deleteRoute,
  useSaveCaseAlertValues: () => saveValues,
  useDeleteCaseAlertValue: () => deleteValue,
}));

const APOLLO_ID = "22222222-2222-4222-8222-222222222222";
const HINT = "spaces/AAAAtest01 · …wxyz";

const apolloRoom: CaseAlertChatRoom = {
  id: APOLLO_ID,
  ladder: "SRE",
  teamKey: "apollo",
  label: "Apollo SRE",
  enabled: true,
  teamless: true,
  hasWebhook: true,
  webhookHint: HINT,
  webhookSetBy: "sre.rota@example.test",
  lastTestStatus: "delivered",
  updatedBy: "sre.rota@example.test",
  updatedOn: "2026-10-13T10:00:00Z",
};

function make(over: Partial<Settings> = {}): Settings {
  return {
    canManage: { view: true, roomLadders: ["SRE"], routeLadders: ["SRE"] },
    environment: "staging",
    webhooksEnabled: true,
    revision: { saved: 4 },
    rooms: [apolloRoom],
    routeOptions: {
      groups: [
        { key: "vega", name: "Vega", family: "CRE" },
        { key: "apollo", name: "Apollo", family: "SRE" },
        { key: "asgardeo", name: "Asgardeo", family: "SME" },
      ],
      teams: [
        { key: "vega", name: "Vega", family: "CRE" },
        { key: "apollo", name: "Apollo", family: "SRE" },
      ],
      rotas: [{ code: "SRE_SAAS", label: "SaaS SRE" }],
    },
    routes: [
      {
        id: "55555555-5555-4555-8555-555555555555",
        ladder: "SRE",
        groupName: "default sre group",
        rotaCode: "SRE_SAAS",
        enabled: true,
        updatedBy: "x",
        updatedOn: "2026-10-13T10:00:00Z",
      },
    ],
    values: [],
    ...over,
  };
}

/** What the alert service reported, with the SRE ladder on channel. */
function running(channel: string): Settings["running"] {
  return {
    reportedAt: "2026-10-13T10:00:00Z",
    mode: "shadow",
    ladders: {},
    fileOnly: { SRE: { channel } },
  };
}

beforeEach(() => {
  vi.clearAllMocks();
  // The redirect sections start closed; these tests work inside them, so
  // they open as a person who opened them before.
  window.localStorage.setItem(
    "csm-ts.alert-settings.open",
    JSON.stringify(["SRE.redirects", "CRE.redirects"]),
  );
  settings.data = make();
});

describe("CaseAlertSettings", () => {
  it("shows nothing to somebody who may not see the settings", () => {
    settings.data = make({
      canManage: { view: false, roomLadders: [], routeLadders: [] },
    });
    const { container } = render(<CaseAlertSettings chain="SRE_SAAS" />);
    expect(container).toBeEmptyDOMElement();
  });

  it("shows nothing for a chain with no settings here (SME)", () => {
    const { container } = render(<CaseAlertSettings chain="SME" />);
    expect(container).toBeEmptyDOMElement();
  });

  it("says when saved changes wait for a restart, and when they are in use", () => {
    settings.data = make({
      revision: { saved: 5, applied: 4, appliedMode: "db" },
    });
    const { unmount } = render(<CaseAlertSettings chain="SRE_SAAS" />);
    expect(
      screen.getByText(/apply the next time the alert service restarts/),
    ).toBeInTheDocument();
    unmount();
    settings.data = make({
      revision: {
        saved: 5,
        applied: 5,
        appliedMode: "db",
        appliedAt: "2026-10-13T10:00:00Z",
      },
    });
    render(<CaseAlertSettings chain="SRE_SAAS" />);
    expect(
      screen.getByText(
        /In use by the alert service \(as the alert service reported at/,
      ),
    ).toBeInTheDocument();
  });

  it("confirms a redirect once it is added, and once it is removed", () => {
    saveRoute.mutate.mockImplementationOnce(
      (_v: unknown, opts: { onSuccess?: () => void }) => opts.onSuccess?.(),
    );
    deleteRoute.mutate.mockImplementationOnce(
      (_v: unknown, opts: { onSuccess?: () => void }) => opts.onSuccess?.(),
    );
    render(<CaseAlertSettings chain="SRE_SAAS" />);
    fireEvent.change(screen.getByLabelText("When an incident is assigned to"), {
      target: { value: "Asgardeo" },
    });
    fireEvent.change(screen.getByLabelText("send its alerts to"), {
      target: { value: "team:apollo" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Add" }));
    expect(screen.getByText(/^✓ Saved:/)).toHaveTextContent(
      "✓ Saved: redirect from Asgardeo to Apollo. The alert service uses the saved settings once it is switched to them (db mode).",
    );
    const redirect = screen
      .getByText("Every team on the SaaS SRE rota")
      .closest("tr") as HTMLElement;
    fireEvent.click(within(redirect).getByRole("button", { name: "Remove" }));
    fireEvent.click(within(redirect).getByRole("button", { name: "Remove" }));
    expect(screen.getByText(/^✓ Saved:/)).toHaveTextContent(
      "✓ Saved: redirect from default sre group removed.",
    );
  });

  it("sends an SME team's incidents to a whole rota, picked from lists", () => {
    render(<CaseAlertSettings chain="SRE_SAAS" />);
    expect(
      screen.getByText("Every team on the SaaS SRE rota"),
    ).toBeInTheDocument();
    const from = screen.getByLabelText("When an incident is assigned to");
    // SRE and SME teams for the SRE ladder; never a CRE team.
    expect(
      within(from).getByRole("option", { name: "Apollo" }),
    ).toBeInTheDocument();
    expect(
      within(from).getByRole("option", { name: "Asgardeo" }),
    ).toBeInTheDocument();
    expect(
      within(from).queryByRole("option", { name: "Vega" }),
    ).not.toBeInTheDocument();
    fireEvent.change(from, { target: { value: "Asgardeo" } });
    fireEvent.change(screen.getByLabelText("send its alerts to"), {
      target: { value: "rota:SRE_SAAS" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Add" }));
    expect(saveRoute.mutate).toHaveBeenCalledWith(
      { ladder: "SRE", groupName: "Asgardeo", rotaCode: "SRE_SAAS" },
      expect.anything(),
    );
  });

  it("offers the ladder's own teams, then whole rotas, as where alerts go", () => {
    render(<CaseAlertSettings chain="SRE_SAAS" />);
    const to = screen.getByLabelText("send its alerts to");
    expect(
      within(to).getByRole("option", { name: "Apollo" }),
    ).toBeInTheDocument();
    expect(
      within(to).getByRole("option", { name: "Every team on SaaS SRE" }),
    ).toBeInTheDocument();
    expect(
      within(to).queryByRole("option", { name: "Vega" }),
    ).not.toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("When an incident is assigned to"), {
      target: { value: "Asgardeo" },
    });
    fireEvent.change(to, { target: { value: "team:apollo" } });
    fireEvent.click(screen.getByRole("button", { name: "Add" }));
    expect(saveRoute.mutate).toHaveBeenCalledWith(
      { ladder: "SRE", groupName: "Asgardeo", teamKey: "apollo" },
      expect.anything(),
    );
  });

  it("says where a redirected incident's alerts end up: the team's space and channel", () => {
    settings.data = make({
      running: running("both"),
      routes: [
        {
          id: "77777777-7777-4777-8777-777777777777",
          ladder: "SRE",
          groupName: "asgardeo",
          teamKey: "apollo",
          enabled: true,
          updatedBy: "x",
          updatedOn: "2026-10-13T10:00:00Z",
        },
      ],
    });
    render(<CaseAlertSettings chain="SRE_SAAS" />);
    const redirect = screen
      .getByRole("cell", { name: "Asgardeo" })
      .closest("tr") as HTMLElement;
    expect(within(redirect).getByText("Apollo")).toBeInTheDocument();
    expect(
      within(redirect).getByText("Calls and chat · in the Apollo SRE space"),
    ).toBeInTheDocument();
  });

  it("keeps a redirect from a team whose name is not its key: the alert service needs it", () => {
    settings.data = make({
      routes: [
        {
          id: "77777777-7777-4777-8777-777777777777",
          ladder: "SRE",
          groupName: "apollo sre group",
          teamKey: "apollo",
          enabled: true,
          updatedBy: "x",
          updatedOn: "2026-10-13T10:00:00Z",
        },
      ],
    });
    render(<CaseAlertSettings chain="SRE_SAAS" />);
    expect(
      screen.queryByText(
        "Same team, not needed: it is alerted as itself anyway",
      ),
    ).not.toBeInTheDocument();
  });

  it("marks a redirect from a team to itself as not needed", () => {
    settings.data = make({
      routes: [
        {
          id: "77777777-7777-4777-8777-777777777777",
          ladder: "SRE",
          groupName: "apollo",
          teamKey: "apollo",
          enabled: true,
          updatedBy: "x",
          updatedOn: "2026-10-13T10:00:00Z",
        },
      ],
    });
    render(<CaseAlertSettings chain="SRE_SAAS" />);
    expect(
      screen.getByText("Same team, not needed: it is alerted as itself anyway"),
    ).toBeInTheDocument();
  });

  it("names only the channel for a team that gets no chat cards", () => {
    settings.data = make({
      running: running("both"),
      rooms: [{ ...apolloRoom, channel: "call" }],
      routes: [
        {
          id: "77777777-7777-4777-8777-777777777777",
          ladder: "SRE",
          groupName: "asgardeo",
          teamKey: "apollo",
          enabled: true,
          updatedBy: "x",
          updatedOn: "2026-10-13T10:00:00Z",
        },
      ],
    });
    render(<CaseAlertSettings chain="SRE_SAAS" />);
    const redirect = screen
      .getByRole("cell", { name: "Asgardeo" })
      .closest("tr") as HTMLElement;
    expect(within(redirect).getByText("Calls only")).toBeInTheDocument();
  });

  it("offers CRE teams only on the CRE chain, and no rotas", () => {
    settings.data = make({
      canManage: { view: true, roomLadders: ["CRE"], routeLadders: ["CRE"] },
    });
    render(<CaseAlertSettings chain="CRE" />);
    const from = screen.getByLabelText("When a case is assigned to");
    expect(
      within(from).getByRole("option", { name: "Vega" }),
    ).toBeInTheDocument();
    expect(
      within(from).queryByRole("option", { name: "Asgardeo" }),
    ).not.toBeInTheDocument();
    expect(
      within(screen.getByLabelText("send its alerts to")).queryByRole("group", {
        name: "A whole rota",
      }),
    ).not.toBeInTheDocument();
  });

  it("names the parts: Team chat routing, and where an incident's or a case's alerts go", () => {
    const { unmount } = render(<CaseAlertSettings chain="SRE_SAAS" />);
    expect(
      screen.getByRole("heading", { name: "Team chat routing" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("region", {
        name: "Send an incident's alerts to another SRE team",
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("columnheader", { name: "Alerts by" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("columnheader", { name: "Assigned to in CSM Portal" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("columnheader", { name: "Alerts go to" }),
    ).toBeInTheDocument();
    unmount();
    settings.data = make({
      canManage: { view: true, roomLadders: ["CRE"], routeLadders: ["CRE"] },
      routes: [],
    });
    render(<CaseAlertSettings chain="CRE" />);
    expect(
      screen.getByRole("region", {
        name: "Send a case's alerts to another CRE team",
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "No redirects. Each case is alerted to the CRE team it's assigned to.",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText("Other teams")).not.toBeInTheDocument();
  });

  it("tells somebody who may only view who can change the settings", () => {
    settings.data = make({
      canManage: { view: true, roomLadders: [], routeLadders: [] },
    });
    render(<CaseAlertSettings chain="SRE_IAAS" />);
    expect(
      screen.getByRole("heading", { name: "IaaS SRE settings" }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "You can view these. CSM admins and the SRE rota admin can change them.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "More" }),
    ).not.toBeInTheDocument();
  });
});
