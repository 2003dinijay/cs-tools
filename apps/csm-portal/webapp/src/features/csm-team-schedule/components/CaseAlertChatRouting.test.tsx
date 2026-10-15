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

import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
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
const noop = { mutate: vi.fn(), isPending: false };

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
  useSaveCaseAlertRoute: () => noop,
  useDeleteCaseAlertRoute: () => noop,
  useSaveCaseAlertValues: () => noop,
  useDeleteCaseAlertValue: () => noop,
}));

const AT = "2026-10-13T10:00:00Z";
const HINT = "spaces/AAAAtest01 · …wxyz";
const HOOK =
  "https://chat.googleapis.com/v1/spaces/AAAAtest01/messages?key=k&token=t";
const NOT_DB =
  "The alert service uses it after its next restart.";
const DB = "The alert service uses it after its next restart.";

function room(over: Partial<CaseAlertChatRoom>): CaseAlertChatRoom {
  return {
    id: "22222222-2222-4222-8222-222222222222",
    ladder: "SRE",
    label: "SRE Case Alerts",
    enabled: true,
    teamless: false,
    hasWebhook: false,
    updatedBy: "x",
    updatedOn: AT,
    ...over,
  };
}

const apollo = room({
  id: "apollo-room",
  teamKey: "apollo",
  label: "Apollo SRE",
  teamless: true,
  hasWebhook: true,
  webhookHint: HINT,
  lastTestStatus: "delivered",
  lastTestAt: AT,
});
const iaas = room({
  id: "iaas-room",
  teamKey: "iaas",
  label: "IaaS SRE",
  channel: "call",
});
const shared = room({ id: "shared-room" });

function make(over: Partial<Settings> = {}): Settings {
  return {
    canManage: {
      view: true,
      roomLadders: ["CRE", "SRE"],
      routeLadders: ["CRE", "SRE"],
    },
    environment: "staging",
    webhooksEnabled: true,
    revision: { saved: 4 },
    rooms: [apollo, iaas],
    routeOptions: {
      groups: [],
      teams: [
        { key: "apollo", name: "Apollo", family: "SRE" },
        { key: "artemis", name: "Artemis", family: "SRE" },
        { key: "iaas", name: "IaaS", family: "SRE" },
        { key: "zeus", name: "Zeus", family: "SRE" },
        { key: "vega", name: "Vega", family: "CRE" },
      ],
      rotas: [],
    },
    routes: [],
    values: [],
    running: {
      reportedAt: AT,
      mode: "shadow",
      ladders: {},
      fileOnly: { SRE: { channel: "chat" } },
    },
    ...over,
  };
}

/** A row of the chat routing table, by its team's name. */
function chatRow(name: string): HTMLElement {
  const table = screen.getByRole("table", { name: "Team chat routing" });
  const tr = within(table).getByText(name).closest("tr");
  if (!tr) throw new Error(`no row for ${name}`);
  return tr;
}

function optionNames(select: HTMLElement): string[] {
  return within(select)
    .getAllByRole("option")
    .map((o) => o.textContent ?? "");
}

function openAdd(): { button: HTMLElement; dialog: HTMLElement } {
  const button = screen.getByRole("button", {
    name: "+ Add team chat routing",
  });
  button.focus();
  fireEvent.click(button);
  return {
    button,
    dialog: screen.getByRole("dialog", { name: "Add team chat routing" }),
  };
}

beforeEach(() => {
  vi.resetAllMocks();
  settings.data = make();
});

describe("Team chat routing", () => {
  it("lists only the routings saved here, with the hint and last test: the space alone", () => {
    render(<CaseAlertSettings chain="SRE_SAAS" />);
    expect(
      screen.getByRole("heading", { name: "Team chat routing" }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "Which Google Chat space each team's alerts go to.",
      ),
    ).toBeInTheDocument();
    // How a team is alerted stays the file's: no column for it.
    expect(
      screen.queryByRole("columnheader", { name: "Alerts by" }),
    ).not.toBeInTheDocument();
    const table = screen.getByRole("table", { name: "Team chat routing" });
    // Artemis has none saved: not listed.
    expect(within(table).queryByText("Artemis")).not.toBeInTheDocument();
    expect(within(table).getAllByRole("row")).toHaveLength(3);

    const a = chatRow("Apollo");
    expect(
      within(a).getByText(/Apollo SRE · also gets incidents with no SRE team/),
    ).toBeInTheDocument();
    expect(within(a).getByText(HINT)).toBeInTheDocument();
    expect(within(a).getByText(/Last test:/)).toHaveTextContent(
      "Last test: delivered ·",
    );
    expect(within(chatRow("IaaS")).queryByText("Calls only")).toBeNull();
    expect(
      within(chatRow("IaaS")).getByText("No URL saved (uses the file's space)"),
    ).toBeInTheDocument();
    // Nothing in the rows saves on change.
    expect(within(table).queryByRole("combobox")).not.toBeInTheDocument();
    expect(within(table).queryByRole("checkbox")).not.toBeInTheDocument();
    expect(document.body.textContent).not.toContain("token=");
  });

  it("lists every SRE routing on both SRE chains, the shared space first", () => {
    settings.data = make({ rooms: [iaas, apollo, shared] });
    for (const chain of ["SRE_SAAS", "SRE_IAAS"] as const) {
      const { unmount } = render(<CaseAlertSettings chain={chain} />);
      const rows = within(
        screen.getByRole("table", { name: "Team chat routing" }),
      ).getAllByRole("row");
      expect(
        rows.map((r) => r.querySelector("b")?.textContent).slice(1),
      ).toEqual(["All SRE teams (shared space)", "Apollo", "IaaS"]);
      unmount();
    }
  });

  it("says when nothing is saved here", () => {
    settings.data = make({ rooms: [] });
    render(<CaseAlertSettings chain="SRE_SAAS" />);
    expect(
      screen.getByText(
        "No team chat routing saved here yet. Alerts go to the chat spaces in the alert service's file.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("table", { name: "Team chat routing" }),
    ).not.toBeInTheDocument();
  });

  it("shows the CRE shared space on the CRE chain, never an SRE team", () => {
    settings.data = make({
      rooms: [
        apollo,
        room({ id: "cre", ladder: "CRE", label: "CRE Case Alerts" }),
      ],
    });
    render(<CaseAlertSettings chain="CRE" />);
    expect(chatRow("All CRE teams (shared space)")).toBeInTheDocument();
    expect(screen.queryByText("Apollo SRE")).not.toBeInTheDocument();
  });

  it("offers only the teams with no routing yet, and the shared space only when it isn't saved", () => {
    render(<CaseAlertSettings chain="SRE_SAAS" />);
    const { dialog } = openAdd();
    expect(optionNames(within(dialog).getByLabelText("Team"))).toEqual([
      "All SRE teams (shared space)",
      "Artemis",
      "Zeus",
    ]);
    fireEvent.click(within(dialog).getByRole("button", { name: "Cancel" }));

    settings.data = make({ rooms: [apollo, iaas, shared] });
    render(<CaseAlertSettings chain="SRE_IAAS" />);
    const second = screen.getAllByRole("button", {
      name: "+ Add team chat routing",
    })[1];
    fireEvent.click(second);
    expect(
      optionNames(within(screen.getByRole("dialog")).getByLabelText("Team")),
    ).toEqual(["Artemis", "Zeus"]);
  });

  it("keeps the keyboard user's place when the last team is added: focus moves to the note", () => {
    settings.data = make({
      rooms: [apollo, iaas, shared, room({ id: "a2", teamKey: "artemis" })],
    });
    const { rerender } = render(<CaseAlertSettings chain="SRE_SAAS" />);
    screen.getByRole("button", { name: "+ Add team chat routing" }).focus();
    // The save's refetch brings the last team back: the button goes away.
    settings.data = make({
      rooms: [
        apollo,
        iaas,
        shared,
        room({ id: "a2", teamKey: "artemis" }),
        room({ id: "z", teamKey: "zeus" }),
      ],
    });
    rerender(<CaseAlertSettings chain="SRE_SAAS" />);
    expect(
      screen.getByText(/Every SRE team already has a chat routing here/),
    ).toHaveFocus();
  });

  it("says so in the page, not in a disabled button, when every team has a routing", () => {
    settings.data = make({
      rooms: [
        apollo,
        iaas,
        shared,
        room({ id: "a2", teamKey: "artemis" }),
        room({ id: "z", teamKey: "zeus" }),
      ],
    });
    render(<CaseAlertSettings chain="SRE_SAAS" />);
    expect(
      screen.queryByRole("button", { name: "+ Add team chat routing" }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByText(/Every SRE team already has a chat routing here/),
    ).toBeInTheDocument();
  });

  it("needs a Google Chat webhook URL to add one", () => {
    render(<CaseAlertSettings chain="SRE_SAAS" />);
    const { dialog } = openAdd();
    expect(
      within(dialog).getByLabelText("Google Chat webhook URL"),
    ).toHaveAttribute("type", "password");
    fireEvent.click(within(dialog).getByRole("button", { name: "Save" }));
    expect(within(dialog).getByRole("alert")).toHaveTextContent(
      "Paste the space's Google Chat webhook URL.",
    );
    fireEvent.change(within(dialog).getByLabelText("Google Chat webhook URL"), {
      target: { value: "https://example.com/hook" },
    });
    fireEvent.click(within(dialog).getByRole("button", { name: "Save" }));
    expect(within(dialog).getByRole("alert")).toHaveTextContent(
      "isn't a Google Chat webhook URL",
    );
    expect(saveRoom.mutateAsync).not.toHaveBeenCalled();
  });

  it("adds a team: the room with every field, then the URL; closes, confirms, and gives focus back", async () => {
    saveRoom.mutateAsync.mockResolvedValue({ ...apollo, id: "new-room" });
    setWebhook.mutateAsync.mockResolvedValue({});
    render(<CaseAlertSettings chain="SRE_SAAS" />);
    const { button, dialog } = openAdd();
    const team = within(dialog).getByLabelText("Team");
    expect(team).toHaveFocus();
    fireEvent.change(team, { target: { value: "artemis" } });
    // Artemis starts as the file has it: it also gets incidents with no SRE team.
    expect(
      within(dialog).getByRole("checkbox", {
        name: "Also gets incidents with no SRE team",
      }),
    ).toBeChecked();
    // Only the space is set here: how the team is alerted stays the file's.
    expect(within(dialog).queryByLabelText("Alerts by")).not.toBeInTheDocument();
    fireEvent.change(within(dialog).getByLabelText("Google Chat webhook URL"), {
      target: { value: HOOK },
    });
    fireEvent.click(within(dialog).getByRole("button", { name: "Save" }));

    await waitFor(() =>
      expect(setWebhook.mutateAsync).toHaveBeenCalledWith({
        roomId: "new-room",
        url: HOOK,
      }),
    );
    expect(saveRoom.mutateAsync).toHaveBeenCalledWith({
      ladder: "SRE",
      teamKey: "artemis",
      label: "Artemis SRE",
      enabled: true,
      teamless: true,
      channel: "",
    });
    await waitFor(() =>
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
    );
    expect(button).toHaveFocus();
    expect(screen.getByText(/^✓ Saved:/)).toHaveTextContent(
      `✓ Saved: Artemis chat routing. ${NOT_DB}`,
    );
  });

  it("names a team the file doesn't know by its name, and adds the shared space without a channel", async () => {
    saveRoom.mutateAsync.mockResolvedValue({ ...shared, id: "s" });
    setWebhook.mutateAsync.mockResolvedValue({});
    render(<CaseAlertSettings chain="SRE_SAAS" />);
    let { dialog } = openAdd();
    fireEvent.change(within(dialog).getByLabelText("Team"), {
      target: { value: "zeus" },
    });
    expect(
      within(dialog).getByRole("checkbox", {
        name: "Also gets incidents with no SRE team",
      }),
    ).not.toBeChecked();
    fireEvent.change(within(dialog).getByLabelText("Google Chat webhook URL"), {
      target: { value: HOOK },
    });
    fireEvent.click(within(dialog).getByRole("button", { name: "Save" }));
    await waitFor(() =>
      expect(saveRoom.mutateAsync).toHaveBeenCalledWith(
        expect.objectContaining({
          teamKey: "zeus",
          label: "Zeus SRE",
          teamless: false,
          channel: "",
        }),
      ),
    );
    await waitFor(() =>
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
    );

    ({ dialog } = openAdd());
    // The shared space follows the ladder: no channel, no teamless box.
    expect(
      within(dialog).queryByLabelText("Alerts by"),
    ).not.toBeInTheDocument();
    expect(within(dialog).queryByRole("checkbox")).not.toBeInTheDocument();
    fireEvent.change(within(dialog).getByLabelText("Google Chat webhook URL"), {
      target: { value: HOOK },
    });
    fireEvent.click(within(dialog).getByRole("button", { name: "Save" }));
    await waitFor(() =>
      expect(saveRoom.mutateAsync).toHaveBeenLastCalledWith({
        ladder: "SRE",
        teamKey: undefined,
        label: "SRE Case Alerts",
        enabled: true,
        teamless: false,
      }),
    );
  });

  it("keeps an error in the dialog", async () => {
    saveRoom.mutateAsync.mockResolvedValue({ ...apollo, id: "new-room" });
    setWebhook.mutateAsync.mockRejectedValue(
      new Error("That webhook URL was not accepted."),
    );
    render(<CaseAlertSettings chain="SRE_SAAS" />);
    const { dialog } = openAdd();
    fireEvent.change(within(dialog).getByLabelText("Google Chat webhook URL"), {
      target: { value: HOOK },
    });
    fireEvent.click(within(dialog).getByRole("button", { name: "Save" }));
    expect(await within(dialog).findByRole("alert")).toHaveTextContent(
      "That webhook URL was not accepted.",
    );
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    expect(screen.queryByText(/^✓ Saved:/)).not.toBeInTheDocument();
  });

  it("edits a team: keeps the saved URL when left empty, and sends every field", async () => {
    saveRoom.mutateAsync.mockResolvedValue(iaas);
    render(<CaseAlertSettings chain="SRE_SAAS" />);
    const edit = within(chatRow("IaaS")).getByRole("button", { name: "Edit" });
    edit.focus();
    fireEvent.click(edit);
    const dialog = screen.getByRole("dialog", {
      name: "Edit IaaS chat routing",
    });
    expect(within(dialog).queryByLabelText("Team")).not.toBeInTheDocument();
    expect(within(dialog).getByText("IaaS")).toBeInTheDocument();
    // A channel saved before stays as it is: not shown, sent back unchanged.
    expect(within(dialog).queryByLabelText("Alerts by")).not.toBeInTheDocument();
    fireEvent.click(
      within(dialog).getByRole("checkbox", {
        name: "Also gets incidents with no SRE team",
      }),
    );
    fireEvent.click(within(dialog).getByRole("button", { name: "Save" }));
    await waitFor(() =>
      expect(saveRoom.mutateAsync).toHaveBeenCalledWith({
        ladder: "SRE",
        teamKey: "iaas",
        label: "IaaS SRE",
        enabled: true,
        teamless: true,
        channel: "call",
      }),
    );
    expect(setWebhook.mutateAsync).not.toHaveBeenCalled();
    await waitFor(() =>
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
    );
    expect(edit).toHaveFocus();
    expect(screen.getByText(/^✓ Saved:/)).toHaveTextContent(
      "✓ Saved: IaaS chat routing.",
    );
  });

  it("shows the saved URL's hint when editing, and sends a new URL when typed", async () => {
    saveRoom.mutateAsync.mockResolvedValue(apollo);
    setWebhook.mutateAsync.mockResolvedValue({});
    render(<CaseAlertSettings chain="SRE_SAAS" />);
    fireEvent.click(
      within(chatRow("Apollo")).getByRole("button", { name: "Edit" }),
    );
    const dialog = screen.getByRole("dialog", {
      name: "Edit Apollo chat routing",
    });
    expect(
      within(dialog).getByText(`Leave empty to keep the saved URL (${HINT}).`),
    ).toBeInTheDocument();
    fireEvent.change(within(dialog).getByLabelText("Google Chat webhook URL"), {
      target: { value: HOOK },
    });
    fireEvent.click(within(dialog).getByRole("button", { name: "Save" }));
    await waitFor(() =>
      expect(setWebhook.mutateAsync).toHaveBeenCalledWith({
        roomId: "apollo-room",
        url: HOOK,
      }),
    );
    expect(saveRoom.mutateAsync).toHaveBeenCalledWith(
      expect.objectContaining({ teamKey: "apollo", channel: "" }),
    );
  });

  it("tests, turns off and removes from the row, confirming each change", () => {
    const succeed = (_v: unknown, opts: { onSuccess?: () => void }) =>
      opts.onSuccess?.();
    saveRoom.mutate.mockImplementation(succeed);
    deleteRoom.mutate.mockImplementation(succeed);
    render(<CaseAlertSettings chain="SRE_SAAS" />);
    const a = chatRow("Apollo");
    // Edit, Test and More: no more than three buttons.
    expect(
      within(a)
        .getAllByRole("button")
        .map((b) => b.textContent),
    ).toEqual(["Edit", "Test", "More ▾"]);
    fireEvent.click(within(a).getByRole("button", { name: "Test" }));
    expect(testRoom.mutate).toHaveBeenCalledWith(
      "apollo-room",
      expect.anything(),
    );

    fireEvent.click(within(a).getByRole("button", { name: "More" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "Turn off" }));
    expect(saveRoom.mutate).not.toHaveBeenCalled();
    fireEvent.click(within(a).getByRole("button", { name: "Turn off" }));
    expect(saveRoom.mutate).toHaveBeenCalledWith(
      {
        ladder: "SRE",
        teamKey: "apollo",
        label: "Apollo SRE",
        enabled: false,
        teamless: true,
        channel: "",
      },
      expect.anything(),
    );
    expect(screen.getByText(/^✓ Saved:/)).toHaveTextContent(
      "✓ Saved: Apollo chat routing turned off.",
    );

    fireEvent.click(within(a).getByRole("button", { name: "More" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "Remove" }));
    expect(deleteRoom.mutate).not.toHaveBeenCalled();
    fireEvent.click(within(a).getByRole("button", { name: "Remove" }));
    expect(deleteRoom.mutate).toHaveBeenCalledWith(
      "apollo-room",
      expect.anything(),
    );
    expect(screen.getByText(/^✓ Saved:/)).toHaveTextContent(
      "✓ Saved: Apollo chat routing removed.",
    );
  });

  it("turns a room that is off back on from More, without asking", () => {
    settings.data = make({ rooms: [{ ...iaas, enabled: false }] });
    render(<CaseAlertSettings chain="SRE_SAAS" />);
    fireEvent.click(
      within(chatRow("IaaS")).getByRole("button", { name: "More" }),
    );
    fireEvent.click(screen.getByRole("menuitem", { name: "Turn on" }));
    expect(saveRoom.mutate).toHaveBeenCalledWith(
      expect.objectContaining({
        teamKey: "iaas",
        enabled: true,
        channel: "call",
      }),
      expect.anything(),
    );
  });

  it("closes the More menu on Escape and on a click outside", () => {
    render(<CaseAlertSettings chain="SRE_SAAS" />);
    const more = within(chatRow("Apollo")).getByRole("button", {
      name: "More",
    });
    expect(more).toHaveAttribute("aria-haspopup", "menu");
    fireEvent.click(more);
    expect(screen.getByRole("menuitem", { name: "Turn off" })).toHaveFocus();
    fireEvent.keyDown(screen.getByRole("menuitem", { name: "Turn off" }), {
      key: "ArrowDown",
    });
    expect(screen.getByRole("menuitem", { name: "Remove" })).toHaveFocus();
    fireEvent.keyDown(screen.getByRole("menuitem", { name: "Remove" }), {
      key: "Escape",
    });
    expect(screen.queryByRole("menu")).not.toBeInTheDocument();
    expect(more).toHaveFocus();
    fireEvent.click(more);
    fireEvent.mouseDown(document.body);
    expect(screen.queryByRole("menu")).not.toBeInTheDocument();
  });

  it("shows nothing of a family whose settings the reader may not change", () => {
    settings.data = make({
      canManage: { view: true, roomLadders: ["CRE"], routeLadders: ["CRE"] },
    });
    const { container } = render(<CaseAlertSettings chain="SRE_SAAS" />);
    expect(container).toBeEmptyDOMElement();
  });

  it("says when URLs can't be saved on this server, and can't add one", () => {
    settings.data = make({ webhooksEnabled: false });
    render(<CaseAlertSettings chain="SRE_SAAS" />);
    expect(
      screen.getByText("Webhook URLs can't be saved on this server."),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "+ Add team chat routing" }),
    ).toBeDisabled();
  });

  it("words the confirmation for db mode, and lets it be dismissed", () => {
    settings.data = make({
      revision: { saved: 5, applied: 4, appliedMode: "db" },
    });
    saveRoom.mutate.mockImplementation(
      (_v: unknown, opts: { onSuccess?: () => void }) => opts.onSuccess?.(),
    );
    render(<CaseAlertSettings chain="SRE_SAAS" />);
    fireEvent.click(
      within(chatRow("Apollo")).getByRole("button", { name: "More" }),
    );
    fireEvent.click(screen.getByRole("menuitem", { name: "Turn off" }));
    fireEvent.click(
      within(chatRow("Apollo")).getByRole("button", { name: "Turn off" }),
    );
    const done = screen.getByText(/^✓ Saved:/);
    expect(done).toHaveTextContent(
      `✓ Saved: Apollo chat routing turned off. ${DB}`,
    );
    expect(done.closest("[aria-live]")).toHaveAttribute("aria-live", "polite");
    fireEvent.click(screen.getByRole("button", { name: "Dismiss" }));
    expect(screen.queryByText(/^✓ Saved:/)).not.toBeInTheDocument();
  });
});

describe("SME team chat spaces", () => {
  const SME_TEAMS = [
    { key: "asgardeo", name: "Asgardeo", family: "SME" as const },
    { key: "u2", name: "U2", family: "SME" as const },
    { key: "apollo", name: "Apollo", family: "SRE" as const },
  ];
  const asgardeo = room({
    id: "asg-room",
    ladder: "SME",
    teamKey: "asgardeo",
    label: "Asgardeo SME",
    hasWebhook: true,
    webhookHint: HINT,
  });

  it("lists the SME rooms only, by the team's name, with no shared space", () => {
    settings.data = make({
      rooms: [apollo, iaas, asgardeo],
      canManage: { view: true, roomLadders: ["SRE", "SME"], routeLadders: ["SRE"] },
    });
    render(<CaseAlertSettings chain="SME" teams={SME_TEAMS} />);
    expect(
      screen.getByRole("heading", { name: "SME settings" }),
    ).toBeInTheDocument();
    const rows = within(
      screen.getByRole("table", { name: "Team chat routing" }),
    ).getAllByRole("row");
    expect(rows.map((r) => r.querySelector("b")?.textContent).slice(1)).toEqual(
      ["Asgardeo"],
    );
    expect(within(chatRow("Asgardeo")).getByText(HINT)).toBeInTheDocument();
    // SME has no redirects, but has ladder settings of its own.
    expect(screen.queryByText(/redirect/i)).not.toBeInTheDocument();
    expect(screen.getByText(/SME ladder settings/)).toBeInTheDocument();
  });

  it("adds an SME team's space: its SME teams only, the URL alone", async () => {
    settings.data = make({
      rooms: [asgardeo],
      canManage: { view: true, roomLadders: ["SME"], routeLadders: [] },
    });
    saveRoom.mutateAsync.mockResolvedValue({ ...asgardeo, id: "u2-room" });
    setWebhook.mutateAsync.mockResolvedValue({});
    render(<CaseAlertSettings chain="SME" teams={SME_TEAMS} />);
    const { dialog } = openAdd();
    expect(optionNames(within(dialog).getByLabelText("Team"))).toEqual(["U2"]);
    expect(within(dialog).queryByLabelText("Alerts by")).not.toBeInTheDocument();
    expect(within(dialog).queryByRole("checkbox")).not.toBeInTheDocument();
    fireEvent.change(within(dialog).getByLabelText("Google Chat webhook URL"), {
      target: { value: HOOK },
    });
    fireEvent.click(within(dialog).getByRole("button", { name: "Save" }));
    await waitFor(() =>
      expect(setWebhook.mutateAsync).toHaveBeenCalledWith({
        roomId: "u2-room",
        url: HOOK,
      }),
    );
    expect(saveRoom.mutateAsync).toHaveBeenCalledWith({
      ladder: "SME",
      teamKey: "u2",
      label: "U2 SME",
      enabled: true,
      teamless: false,
      channel: "",
    });
  });

  it("shows nothing of SME to somebody who may change only the SRE settings", () => {
    settings.data = make({
      rooms: [asgardeo],
      canManage: { view: true, roomLadders: ["SRE"], routeLadders: ["SRE"] },
    });
    const { container } = render(<CaseAlertSettings chain="SME" teams={SME_TEAMS} />);
    expect(container).toBeEmptyDOMElement();
  });
});
