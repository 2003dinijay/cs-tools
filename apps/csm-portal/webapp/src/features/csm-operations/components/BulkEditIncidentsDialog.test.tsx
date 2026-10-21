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
import { fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import BulkEditIncidentsDialog from "@features/csm-operations/components/BulkEditIncidentsDialog";
import type { BeIncident } from "@api/backend/types";

vi.mock("@api/useSearchUsersByName", () => ({
  useSearchInternalUsersByName: () => ({ data: [], isFetching: false, isError: false }),
}));
vi.mock("@api/useSearchGroups", () => ({
  useSearchGroups: () => ({ data: [], isFetching: false, isError: false }),
}));
vi.mock("@api/useSearchItServices", () => ({
  useSearchItServices: () => ({ data: [], isFetching: false, isError: false }),
}));

const onApply = vi.fn();
const incident = (id: string, state: BeIncident["state"]): BeIncident => ({ id, number: id, state }) as BeIncident;

function pick(dialog: HTMLElement, field: string, option: string): void {
  fireEvent.mouseDown(within(dialog).getByRole("combobox", { name: field }));
  fireEvent.click(screen.getByRole("option", { name: option }));
}

beforeEach(() => onApply.mockClear());

describe("BulkEditIncidentsDialog", () => {
  it("offers only the states every selected incident can move to", () => {
    render(
      <BulkEditIncidentsDialog
        incidents={[incident("a", "NEW"), incident("b", "ON_HOLD")]}
        isApplying={false}
        onClose={vi.fn()}
        onApply={onApply}
      />,
    );
    const dialog = screen.getByRole("dialog", { name: "Edit 2 incidents" });
    fireEvent.mouseDown(within(dialog).getByRole("combobox", { name: "State" }));
    expect(screen.getAllByRole("option").map((o) => o.textContent)).toEqual(["No change", "In Progress", "Cancelled"]);
  });

  it("can't apply with nothing changed", () => {
    render(
      <BulkEditIncidentsDialog incidents={[incident("a", "NEW")]} isApplying={false} onClose={vi.fn()} onApply={onApply} />,
    );
    expect(screen.getByRole("button", { name: "Apply to 1 incident" })).toBeDisabled();
  });

  it("needs a resolution code and notes to resolve, and sends them with the state", () => {
    render(
      <BulkEditIncidentsDialog
        incidents={[incident("a", "IN_PROGRESS"), incident("b", "IN_PROGRESS")]}
        isApplying={false}
        onClose={vi.fn()}
        onApply={onApply}
      />,
    );
    const dialog = screen.getByRole("dialog", { name: "Edit 2 incidents" });
    pick(dialog, "State", "Resolved");
    const apply = within(dialog).getByRole("button", { name: "Apply to 2 incidents" });
    expect(apply).toBeDisabled();
    expect(within(dialog).getByRole("status")).toHaveTextContent("Resolved needs a resolution code and notes.");

    pick(dialog, "Resolution code", "Solved (Permanently)");
    expect(apply).toBeDisabled();
    fireEvent.change(within(dialog).getByRole("textbox", { name: /Resolution notes/ }), {
      target: { value: " Patched the node pool. " },
    });
    expect(apply).toBeEnabled();
    fireEvent.click(apply);
    expect(onApply).toHaveBeenCalledWith({
      state: "RESOLVED",
      resolutionCode: "SOLVED_PERMANENTLY",
      resolutionNotes: "Patched the node pool.",
    });
  });

  it("drops the resolution again when the state moves away from Resolved", () => {
    render(
      <BulkEditIncidentsDialog
        incidents={[incident("a", "IN_PROGRESS")]}
        isApplying={false}
        onClose={vi.fn()}
        onApply={onApply}
      />,
    );
    const dialog = screen.getByRole("dialog", { name: "Edit 1 incident" });
    pick(dialog, "State", "Resolved");
    pick(dialog, "Resolution code", "False Alarm");
    pick(dialog, "State", "On Hold");
    expect(within(dialog).queryByRole("combobox", { name: "Resolution code" })).not.toBeInTheDocument();
    fireEvent.click(within(dialog).getByRole("button", { name: "Apply to 1 incident" }));
    expect(onApply).toHaveBeenCalledWith({ state: "ON_HOLD" });
  });

  it("keeps everything still while applying", () => {
    render(
      <BulkEditIncidentsDialog incidents={[incident("a", "NEW")]} isApplying onClose={vi.fn()} onApply={onApply} />,
    );
    expect(screen.getByRole("button", { name: "Applying…" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Cancel" })).toBeDisabled();
  });
});
