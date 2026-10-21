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

import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import BulkEditChangeRequestsDialog from "@features/csm-operations/components/BulkEditChangeRequestsDialog";
import type { BeChangeRequestDetail } from "@api/backend/types";

// Each selected change request's detail, by id (the dialog reads them for the moves, type, hold and customer steps).
const { apiMock, details, noResults } = vi.hoisted(() => ({
  apiMock: { get: vi.fn(), post: vi.fn(), patch: vi.fn() },
  details: new Map<string, unknown>(),
  noResults: () => ({ data: [], isFetching: false, isError: false }),
}));
vi.mock("@api/backend/client", () => ({ useBackendApi: () => apiMock }));
vi.mock("@api/useSearchUsersByName", () => ({ useSearchInternalUsersByName: noResults }));
vi.mock("@api/useSearchGroups", () => ({ useSearchGroups: noResults }));
vi.mock("@api/useSearchItServices", () => ({ useSearchItServices: noResults }));
vi.mock("@api/useSearchServiceOfferings", () => ({ useSearchServiceOfferings: noResults }));
vi.mock("@features/csm-operations/api/useSearchServiceRequestsForSelect", () => ({
  useSearchServiceRequestsForSelect: noResults,
}));
// The Customer Project picker as a plain input: its value is the project id.
vi.mock("@features/csm-cases/components/AsyncProjectSelect", () => ({
  default: ({
    label,
    value,
    disabled,
    onChange,
  }: {
    label: string;
    value: string;
    disabled?: boolean;
    onChange: (next: string, name?: string) => void;
  }) => (
    <input aria-label={label} disabled={disabled} value={value} onChange={(e) => onChange(e.target.value, "Acme")} />
  ),
}));
vi.mock("@features/csm-operations/api/useChangeRequestScopeLookups", () => ({
  useChangeRequestScopeLookups: () => ({
    deployments: [],
    customerContacts: [],
    contactsReady: true,
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
  }),
}));
// The plan panels' rich-text editor, as a text area whose value is the HTML.
vi.mock("@components/rich-text-editor/Editor", () => ({
  default: ({ onChange, disabled }: { onChange?: (html: string) => void; disabled?: boolean }) => (
    <textarea disabled={disabled} onChange={(e) => onChange?.(e.target.value)} />
  ),
}));

const onApply = vi.fn();

function cr(id: string, fields: Partial<BeChangeRequestDetail> = {}): BeChangeRequestDetail {
  const detail: BeChangeRequestDetail = {
    id,
    number: id,
    state: "new",
    type: "normal",
    legalNextStates: ["assess", "canceled"],
    customerApprovalRequired: false,
    customerReviewRequired: false,
    ...fields,
  };
  details.set(id, detail);
  return detail;
}

function renderDialog(changeRequests: BeChangeRequestDetail[], isApplying = false): HTMLElement {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={queryClient}>
      <BulkEditChangeRequestsDialog
        changeRequests={changeRequests}
        isApplying={isApplying}
        onClose={vi.fn()}
        onApply={onApply}
      />
    </QueryClientProvider>,
  );
  const count = changeRequests.length;
  return screen.getByRole("dialog", { name: `Edit ${count} change request${count === 1 ? "" : "s"}` });
}

/** Waits for every selected change request's detail: the state's helper says so once they are in. */
async function loaded(dialog: HTMLElement): Promise<void> {
  await within(dialog).findByText(
    /Only the moves every selected change request can make|no state they can all move to/,
  );
}

function pick(dialog: HTMLElement, field: string, option: string): void {
  fireEvent.mouseDown(within(dialog).getByRole("combobox", { name: field }));
  fireEvent.click(screen.getByRole("option", { name: option }));
}

function optionsOf(dialog: HTMLElement, field: string): string[] {
  fireEvent.mouseDown(within(dialog).getByRole("combobox", { name: field }));
  const names = screen.getAllByRole("option").map((o) => o.textContent ?? "");
  fireEvent.keyDown(screen.getByRole("listbox"), { key: "Escape" });
  return names;
}

const type = (dialog: HTMLElement, name: string | RegExp, value: string): void => {
  fireEvent.change(within(dialog).getByRole("textbox", { name }), { target: { value } });
};

beforeEach(() => {
  onApply.mockClear();
  details.clear();
  apiMock.get.mockImplementation((path: string) => Promise.resolve(details.get(decodeURIComponent(path.split("/").pop()!))));
});

describe("BulkEditChangeRequestsDialog", () => {
  it("opens wide, so its columns and plan panels never squeeze a field", async () => {
    const dialog = renderDialog([cr("a")]);
    await loaded(dialog);
    expect(dialog).toHaveClass("MuiDialog-paperWidthLg");
  });

  it("sends every field set, and only those, to change requests in New", async () => {
    const dialog = renderDialog([cr("a"), cr("b")]);
    await loaded(dialog);
    const apply = within(dialog).getByRole("button", { name: "Apply to 2 change requests" });
    expect(apply).toBeDisabled();

    type(dialog, "Short description", " Rotate the gateway certificates ");
    pick(dialog, "Type", "Standard");
    pick(dialog, "Category", "Software");
    pick(dialog, "Impact", "Medium");
    pick(dialog, "Priority", "High");
    pick(dialog, "On hold", "Put on hold");
    type(dialog, "On hold reason", "Waiting on the vendor");
    type(dialog, "Rollback duration", "30 mins");
    fireEvent.change(within(within(dialog).getByRole("group", { name: "Description" })).getByRole("textbox"), {
      target: { value: "<p>New certificates</p>" },
    });
    type(dialog, "Service outage / downtime", "None");
    type(dialog, "Communication plan", "Email the customer");
    pick(dialog, "Customer approval", "Required");
    pick(dialog, "Implementation Plan visible to customers", "Not visible");
    fireEvent.change(within(dialog).getByRole("textbox", { name: "Customer Project" }), { target: { value: "proj-a" } });
    type(dialog, "Work note (internal)", "Bulk rotation");
    type(dialog, "Additional comment (customer-visible)", "We will rotate the certificates.");

    expect(apply).toBeEnabled();
    fireEvent.click(apply);
    expect(onApply).toHaveBeenCalledWith({
      title: "Rotate the gateway certificates",
      type: "standard",
      category: "software",
      impact: "medium",
      priority: "high",
      onHold: true,
      onHoldReason: "Waiting on the vendor",
      rollbackDurationText: "30 mins",
      description: "<p>New certificates</p>",
      serviceOutage: "None",
      communicationPlan: "Email the customer",
      customerApprovalRequired: true,
      isPlanningVisibleToCustomers: false,
      projectId: "proj-a",
      deploymentIds: [],
      deploymentProductIds: [],
      workNote: "Bulk rotation",
      comment: "We will rotate the certificates.",
    });
  }, 20_000);

  it("offers the type and the Customer Project only when every selected change request is New", async () => {
    const dialog = renderDialog([cr("a"), cr("b", { state: "assess", legalNextStates: ["canceled"] })]);
    await loaded(dialog);
    expect(within(dialog).getByRole("combobox", { name: "Type" })).toHaveAttribute("aria-disabled", "true");
    expect(within(dialog).getByRole("textbox", { name: "Customer Project" })).toBeDisabled();
    expect(within(dialog).getAllByText(/fixed once approval has been requested/).length).toBeGreaterThan(0);
    // The rest of the fields still apply.
    expect(within(dialog).getByRole("combobox", { name: "Impact" })).not.toHaveAttribute("aria-disabled");
  });

  it("offers a customer step only as far as every selected change request can take it", async () => {
    // After New a step can be added (with a Customer Project) but never removed.
    let dialog = renderDialog([
      cr("a", { state: "assess", project: { id: "p", name: "P" }, legalNextStates: ["canceled"] }),
      cr("b", { state: "assess", customerApprovalRequired: true, legalNextStates: ["canceled"] }),
    ]);
    await loaded(dialog);
    expect(optionsOf(dialog, "Customer approval")).toEqual(["No change", "Required"]);
    expect(within(dialog).getByText("Can be added but not removed once approval has been requested.")).toBeInTheDocument();
    cleanup();

    // Neither can take one: "a2" has no Customer Project to ask, "e" is an Emergency change.

    dialog = renderDialog([
      cr("a2", { state: "assess", legalNextStates: ["canceled"] }),
      cr("e", { state: "new", type: "emergency" }),
    ]);
    await loaded(dialog);
    expect(optionsOf(dialog, "Customer review")).toEqual(["No change", "Not required"]);
  });

  it("turns the customer steps off for an Emergency type, and says so", async () => {
    const dialog = renderDialog([cr("a", { customerApprovalRequired: true })]);
    await loaded(dialog);
    pick(dialog, "Customer review", "Required");
    pick(dialog, "Type", "Emergency");
    expect(within(dialog).getByRole("combobox", { name: "Customer approval" })).toHaveAttribute("aria-disabled", "true");
    expect(within(dialog).getByRole("combobox", { name: "Customer review" })).toHaveAttribute("aria-disabled", "true");
    expect(within(dialog).getByText(/both customer steps are turned off/)).toBeInTheDocument();
    fireEvent.click(within(dialog).getByRole("button", { name: "Apply to 1 change request" }));
    // The run turns both off per change request (bulkChangeRequestPlan); the review just picked is dropped.
    expect(onApply).toHaveBeenCalledWith({ type: "emergency" });
  });

  it("keeps an approved selection to the state, the hold and the notes", async () => {
    const dialog = renderDialog([cr("a", { state: "implement", legalNextStates: ["review", "rollback"] })]);
    await loaded(dialog);
    expect(within(dialog).getByText(/their fields are read-only: only the state, on hold/)).toBeInTheDocument();
    expect(within(dialog).getByRole("textbox", { name: "Short description" })).toBeDisabled();
    expect(within(dialog).getByRole("combobox", { name: "Impact" })).toHaveAttribute("aria-disabled", "true");
    expect(within(dialog).getByRole("combobox", { name: "Service" })).toBeDisabled();
    expect(within(dialog).getByRole("combobox", { name: "State" })).not.toHaveAttribute("aria-disabled");
    expect(within(dialog).getByRole("combobox", { name: "On hold" })).not.toHaveAttribute("aria-disabled");
    expect(within(dialog).getByRole("textbox", { name: "Additional comment (customer-visible)" })).toBeEnabled();
  });

  it("says how many of a mixed selection are approved, and what applies to them", async () => {
    const dialog = renderDialog([cr("a"), cr("b", { state: "scheduled", legalNextStates: ["implement", "canceled"] })]);
    await loaded(dialog);
    expect(
      within(dialog).getByText(/1 of the selected change requests has been approved: only the state, on hold, the work note and the additional comment/),
    ).toBeInTheDocument();
    expect(within(dialog).getByRole("textbox", { name: "Short description" })).toBeEnabled();
  });

  it("takes a hold reason only with Put on hold, and says a state change needs the hold lifted", async () => {
    const dialog = renderDialog([cr("a", { onHold: true }), cr("b")]);
    await loaded(dialog);
    expect(within(dialog).getByRole("textbox", { name: "On hold reason" })).toBeDisabled();
    expect(within(dialog).getByText(/1 of the selected change requests is on hold/)).toBeInTheDocument();
    pick(dialog, "On hold", "Put on hold");
    expect(within(dialog).getByRole("textbox", { name: "On hold reason" })).toBeEnabled();
    type(dialog, "On hold reason", "Vendor");
    pick(dialog, "On hold", "Take off hold");
    fireEvent.click(within(dialog).getByRole("button", { name: "Apply to 2 change requests" }));
    expect(onApply).toHaveBeenCalledWith({ onHold: false });
  });

  it("offers a service offering only once a service is set", async () => {
    const dialog = renderDialog([cr("a")]);
    await loaded(dialog);
    expect(within(dialog).getByRole("combobox", { name: "Service offering" })).toBeDisabled();
    expect(within(dialog).getByText(/Set a Service first/)).toBeInTheDocument();
  });

  it("needs a reason to cancel, taken as the work note", async () => {
    const dialog = renderDialog([cr("a"), cr("b")]);
    await loaded(dialog);
    pick(dialog, "State", "Cancel change (Canceled)");
    const apply = within(dialog).getByRole("button", { name: "Apply to 2 change requests" });
    expect(apply).toBeDisabled();
    expect(within(dialog).getByRole("status")).toHaveTextContent(/A reason is required/);
    type(dialog, /Reason \(internal work note\)/, "Duplicate.");
    fireEvent.click(apply);
    expect(onApply).toHaveBeenCalledWith({ state: "canceled", workNote: "Duplicate." });
  });

  it("keeps everything still while applying", async () => {
    const dialog = renderDialog([cr("a")], true);
    await loaded(dialog);
    expect(within(dialog).getByRole("button", { name: "Applying…" })).toBeDisabled();
    expect(within(dialog).getByRole("button", { name: "Cancel" })).toBeDisabled();
    expect(within(dialog).getByRole("textbox", { name: "Work note (internal)" })).toBeDisabled();
  });
});
