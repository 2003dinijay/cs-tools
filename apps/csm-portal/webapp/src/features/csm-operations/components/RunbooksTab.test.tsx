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
import { fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import RunbooksTab from "@features/csm-operations/components/RunbooksTab";

// The import tree reads this at module load; vitest does not load public/config.js.
vi.hoisted(() => {
  window.config = { ...window.config, CSM_PORTAL_BACKEND_BASE_URL: "http://localhost:9090" };
});

const fixtures = vi.hoisted(() => ({
  catalog: [
    { id: "1", alertName: "Gateway Latency High", runbookUrl: "https://docs.example.com/gw", createdOn: "", createdBy: "", updatedOn: "", updatedBy: "" },
    { id: "2", alertName: "JVM Heap", runbookUrl: null, createdOn: "", createdBy: "", updatedOn: "", updatedBy: "" },
  ],
  idleMutation: () => ({ mutate: vi.fn(), isPending: false, isError: false, error: null }),
  access: { canWrite: true },
  saveBlob: vi.fn(),
  downloadCsv: vi.fn(),
}));

vi.mock("@context/current-user/usePortalAccess", () => ({ usePortalAccess: () => fixtures.access }));
vi.mock("@utils/saveBlob", () => ({ saveBlob: fixtures.saveBlob }));
vi.mock("@utils/csvExport", () => ({ downloadCsv: fixtures.downloadCsv }));
vi.mock("@features/csm-operations/api/useRunbooks", () => ({
  useRunbooks: () => ({ data: fixtures.catalog, isLoading: false, isError: false, error: null, refetch: vi.fn() }),
  useCreateRunbook: fixtures.idleMutation,
  useUpdateRunbook: fixtures.idleMutation,
  useDeleteRunbook: fixtures.idleMutation,
  useImportRunbooks: fixtures.idleMutation,
}));

describe("RunbooksTab search runbook with alert name", () => {
  it("finds the runbook for an alert name or an incident subject", () => {
    render(<RunbooksTab />);
    expect(screen.getByText("Search runbook with alert name")).toBeInTheDocument();
    const search = screen.getByLabelText("Search runbook with alert name");

    fireEvent.change(search, { target: { value: "gateway latency high" } });
    expect(screen.getByText("Matched alert: Gateway Latency High")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /docs\.example\.com\/gw/ })).toHaveAttribute("href", "https://docs.example.com/gw");

    fireEvent.change(search, { target: { value: "Icinga Alert: node-07 / JVM Heap" } });
    expect(screen.getByText("Matched alert: JVM Heap")).toBeInTheDocument();
    expect(screen.getByText("No runbook available for this alert.")).toBeInTheDocument();

    fireEvent.change(search, { target: { value: "unknown" } });
    expect(screen.getByText("No runbook covers this alert.")).toBeInTheDocument();
  });
});

describe("RunbooksTab export", () => {
  beforeEach(() => {
    fixtures.access.canWrite = true;
    fixtures.saveBlob.mockReset();
    fixtures.downloadCsv.mockReset();
  });

  it("exports the shown runbooks as importable JSON", async () => {
    render(<RunbooksTab />);
    fireEvent.click(screen.getByRole("button", { name: "Export" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "JSON (can be imported again)" }));
    const [blob, name] = fixtures.saveBlob.mock.calls[0] as [Blob, string];
    expect(name).toMatch(/^runbooks-\d{4}-\d{2}-\d{2}\.json$/);
    const text = await new Promise<string>((resolve) => {
      const reader = new FileReader();
      reader.onload = () => resolve(String(reader.result));
      reader.readAsText(blob);
    });
    expect(JSON.parse(text)).toEqual({ "Gateway Latency High": "https://docs.example.com/gw", "JVM Heap": "" });
  });

  it("exports only the runbooks the search leaves, as CSV", () => {
    render(<RunbooksTab />);
    fireEvent.change(screen.getByLabelText("Search runbooks by alert name or link"), { target: { value: "jvm" } });
    fireEvent.click(screen.getByRole("button", { name: "Export" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "CSV" }));
    expect(fixtures.downloadCsv).toHaveBeenCalledWith(
      ["Alert name", "Runbook link", "Last updated by", "Last updated"],
      [["JVM Heap", "", "", ""]],
      expect.stringMatching(/^runbooks-\d{4}-\d{2}-\d{2}\.csv$/),
    );
  });

  it("offers Export to someone who cannot edit runbooks", () => {
    fixtures.access.canWrite = false;
    render(<RunbooksTab />);
    expect(screen.getByRole("button", { name: "Export" })).toBeEnabled();
    expect(screen.queryByRole("button", { name: "Import JSON" })).not.toBeInTheDocument();
const mocks = vi.hoisted(() => ({ create: vi.fn(), update: vi.fn(), merge: vi.fn(), unmerge: vi.fn() }));

const catalog = [
  {
    id: "1",
    number: "RB0000001",
    runbookUrl: "https://docs.example.com/gw",
    alerts: ["gw-5xx", "Gateway Latency High"],
    createdOn: "2026-03-02T10:00:00Z",
    createdBy: "creator@example.com",
    updatedOn: "2026-09-15T12:00:00Z",
    updatedBy: "editor@example.com",
  },
  { id: "2", number: "RB0000002", runbookUrl: null, alerts: ["JVM Heap"], createdOn: "", createdBy: "", updatedOn: "", updatedBy: "" },
];

const idleMutation = (mutate = vi.fn()) => ({ mutate, isPending: false, isError: false, error: null });

vi.mock("@context/current-user/usePortalAccess", () => ({ usePortalAccess: () => ({ canWrite: true }) }));
vi.mock("@features/csm-operations/api/useRunbooks", () => ({
  useRunbooks: () => ({ data: catalog, isLoading: false, isError: false, error: null, refetch: vi.fn() }),
  useCreateRunbook: () => idleMutation(mocks.create),
  useUpdateRunbook: () => idleMutation(mocks.update),
  useDeleteRunbook: () => idleMutation(),
  useImportRunbooks: () => idleMutation(),
  useMergeRunbooks: () => idleMutation(mocks.merge),
  useUnmergeRunbookAlerts: () => idleMutation(mocks.unmerge),
}));

function type(el: HTMLElement, value: string): void {
  fireEvent.change(el, { target: { value } });
}

function addTag(el: HTMLElement, value: string): void {
  type(el, value);
  fireEvent.keyDown(el, { key: "Enter" });
}

describe("RunbooksTab", () => {
  beforeEach(() => {
    mocks.create.mockReset();
    mocks.update.mockReset();
    mocks.merge.mockReset();
    mocks.unmerge.mockReset();
  });

  it("lists each runbook's number and alerts as tags, with its link, and no expander", () => {
    render(<RunbooksTab />);
    const row = screen.getByRole("checkbox", { name: "Select RB0000001" }).closest("tr") as HTMLElement;
    expect(within(row).getByRole("link", { name: /Open runbook/ })).toHaveAttribute("href", "https://docs.example.com/gw");
    expect(within(row).getByText("RB0000001")).toBeInTheDocument();
    const tags = within(row).getByRole("list", { name: "Alerts of RB0000001" });
    expect(within(tags).getByText("gw-5xx")).toBeInTheDocument();
    expect(within(tags).getByText("Gateway Latency High")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Show the alerts of/ })).not.toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "Number" })).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "Alerts" })).toBeInTheDocument();
    expect(screen.getByText("2 runbooks covering 3 alerts, 1 without a link")).toBeInTheDocument();
  });

  it("adds alerts as tags with Enter and saves them without asking for a name", () => {
    render(<RunbooksTab />);
    fireEvent.click(screen.getByRole("button", { name: "Add runbook" }));
    const dialog = screen.getByRole("dialog");
    expect(within(dialog).queryByLabelText(/^Name/)).not.toBeInTheDocument();
    const alerts = within(dialog).getByLabelText(/^Alerts/);
    addTag(alerts, "Disk Space Low");
    type(alerts, "disk-full");
    expect(within(dialog).getByText("Disk Space Low")).toBeInTheDocument();
    fireEvent.click(within(dialog).getByRole("button", { name: "Add" }));
    expect(mocks.create).toHaveBeenCalledWith(
      { runbookUrl: null, alerts: ["Disk Space Low", "disk-full"] },
      expect.anything(),
    );
  });

  it("refuses an alert another runbook already has", () => {
    render(<RunbooksTab />);
    fireEvent.click(screen.getByRole("button", { name: "Add runbook" }));
    const dialog = screen.getByRole("dialog");
    addTag(within(dialog).getByLabelText(/^Alerts/), "JVM HEAP");
    expect(within(dialog).getByText(/Already used by another runbook: JVM HEAP \(in RB0000002\)/)).toBeInTheDocument();
    expect(within(dialog).getByRole("button", { name: "Add" })).toBeDisabled();
  });

  it("edits a runbook's alerts", () => {
    render(<RunbooksTab />);
    fireEvent.click(screen.getByRole("button", { name: "Edit RB0000001" }));
    const dialog = screen.getByRole("dialog", { name: "Edit RB0000001" });
    addTag(within(dialog).getByLabelText(/^Alerts/), "gw-timeout");
    fireEvent.click(within(dialog).getByRole("button", { name: "Save" }));
    expect(mocks.update).toHaveBeenCalledWith(
      {
        id: "1",
        input: { runbookUrl: "https://docs.example.com/gw", alerts: ["gw-5xx", "Gateway Latency High", "gw-timeout"] },
      },
      expect.anything(),
    );
  });

  it("unmerges an alert only after the confirmation, which names it and links the runbook", () => {
    render(<RunbooksTab />);
    fireEvent.click(screen.getByLabelText("Unmerge gw-5xx"));
    const dialog = screen.getByRole("dialog", { name: "Unmerge alert?" });
    expect(within(dialog).getByText(/will leave the runbook it shares with/)).toBeInTheDocument();
    expect(within(dialog).getByText("Gateway Latency High")).toBeInTheDocument();
    expect(within(dialog).getByRole("link", { name: /docs\.example\.com\/gw/ })).toHaveAttribute("target", "_blank");
    expect(mocks.unmerge).not.toHaveBeenCalled();

    fireEvent.click(within(dialog).getByRole("button", { name: "Cancel" }));
    expect(mocks.unmerge).not.toHaveBeenCalled();

    fireEvent.click(screen.getByLabelText("Unmerge gw-5xx"));
    fireEvent.click(within(screen.getByRole("dialog", { name: "Unmerge alert?" })).getByRole("button", { name: "Unmerge" }));
    expect(mocks.unmerge).toHaveBeenCalledWith({ id: "1", alerts: ["gw-5xx"] }, expect.anything());
  });

  it("shows when each runbook was created and last updated, but not by whom", () => {
    render(<RunbooksTab />);
    expect(screen.getByRole("columnheader", { name: "Created" })).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "Updated" })).toBeInTheDocument();
    const row = screen.getByRole("checkbox", { name: "Select RB0000001" }).closest("tr") as HTMLElement;
    expect(within(row).getByText("Mar 2, 2026")).toBeInTheDocument();
    expect(within(row).getByText("Sep 15, 2026")).toBeInTheDocument();
    expect(within(row).queryByText("creator@example.com")).not.toBeInTheDocument();
    expect(within(row).queryByText("editor@example.com")).not.toBeInTheDocument();
  });

  it("offers no unmerge for a runbook's only alert", () => {
    render(<RunbooksTab />);
    expect(screen.queryByLabelText("Unmerge JVM Heap")).not.toBeInTheDocument();
  });

  it("merges the selected runbooks, previewing the alerts and choosing the link", () => {
    render(<RunbooksTab />);
    fireEvent.click(screen.getByRole("checkbox", { name: "Select RB0000002" }));
    expect(screen.getByRole("button", { name: "Merge" })).toBeDisabled();
    fireEvent.click(screen.getByRole("checkbox", { name: "Select RB0000001" }));
    expect(screen.getByText("2 selected (3 alerts)")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Merge 2 runbooks" }));

    const dialog = screen.getByRole("dialog");
    expect(within(dialog).getByText("Alerts in the merged runbook (3)")).toBeInTheDocument();
    expect(within(dialog).queryByLabelText(/^Name/)).not.toBeInTheDocument();
    expect(within(dialog).getByText(/will share one runbook, RB0000001/)).toBeInTheDocument();
    expect(within(dialog).getByRole("radio", { name: /docs\.example\.com\/gw/ })).toBeChecked();
    const linkOption = within(dialog).getByRole("link", { name: "https://docs.example.com/gw" });
    expect(linkOption).toHaveAttribute("href", "https://docs.example.com/gw");
    expect(linkOption).toHaveAttribute("target", "_blank");
    fireEvent.click(within(dialog).getByRole("button", { name: "Merge" }));
    expect(mocks.merge).toHaveBeenCalledWith(
      { runbookIds: ["1", "2"], runbookUrl: "https://docs.example.com/gw" },
      expect.anything(),
    );
  });

  it("merges with another link typed in the dialog", () => {
    render(<RunbooksTab />);
    fireEvent.click(screen.getByRole("checkbox", { name: "Select RB0000002" }));
    fireEvent.click(screen.getByRole("checkbox", { name: "Select RB0000001" }));
    fireEvent.click(screen.getByRole("button", { name: "Merge 2 runbooks" }));
    const dialog = screen.getByRole("dialog");
    fireEvent.click(within(dialog).getByRole("radio", { name: "Another link" }));
    expect(within(dialog).getByRole("button", { name: "Merge" })).toBeDisabled();
    type(within(dialog).getByLabelText("Another runbook link"), "https://docs.example.com/platform");
    fireEvent.click(within(dialog).getByRole("button", { name: "Merge" }));
    expect(mocks.merge).toHaveBeenCalledWith(
      { runbookIds: ["1", "2"], runbookUrl: "https://docs.example.com/platform" },
      expect.anything(),
    );
  });

  it("shows each filter's default and has no editor filter", () => {
    render(<RunbooksTab />);
    fireEvent.click(screen.getByRole("button", { name: "Filters" }));
    expect(screen.queryByRole("combobox", { name: "Last updated by" })).not.toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: "Last updated" })).toHaveTextContent("Any time");
  });

  it("searches the runbook with an alert name", () => {
    render(<RunbooksTab />);
    const search = screen.getByLabelText("Search runbook with alert name");
    fireEvent.change(search, { target: { value: "gateway latency high" } });
    expect(screen.getByText("RB0000001, matched alert: Gateway Latency High")).toBeInTheDocument();
    fireEvent.change(search, { target: { value: "unknown" } });
    expect(screen.getByText("No runbook covers this alert.")).toBeInTheDocument();
  });
});
