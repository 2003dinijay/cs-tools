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
import type { BeChangeRequestDetail, BePatchChangeRequestPayload } from "@api/backend/types";

// The "Assignment group" picker goes through useSearchGroups, which hits the
// backend client via react-query — stub it out (same approach as
// EditIncidentDialog.test.tsx).
const useSearchGroupsMock = vi.fn(() => ({ data: [], isFetching: false, isError: false }));
vi.mock("@api/useSearchGroups", () => ({
  useSearchGroups: (...args: unknown[]) => useSearchGroupsMock(...(args as [])),
}));

import EditChangeRequestDialog from "@features/csm-operations/components/EditChangeRequestDialog";

const BASE_CR: BeChangeRequestDetail = {
  id: "chg-1",
  number: "CHG0009988",
  subject: "Upgrade the gateway cluster",
  createdOn: "2026-01-01T00:00:00Z",
  state: "assess",
  type: "normal",
  assignedTeam: { id: "team-1", name: "Platform" },
  hasCustomerApproved: false,
  hasCustomerReviewed: false,
};

function renderDialog(
  overrides: Partial<BeChangeRequestDetail> = {},
  onSave = vi.fn<(patch: BePatchChangeRequestPayload) => void>(),
): { onSave: typeof onSave; onClose: ReturnType<typeof vi.fn> } {
  const onClose = vi.fn();
  render(
    <EditChangeRequestDialog
      cr={{ ...BASE_CR, ...overrides }}
      isSaving={false}
      onClose={onClose}
      onSave={onSave}
    />,
  );
  return { onSave, onClose };
}

const approvedSwitch = (): HTMLElement => screen.getByLabelText(/customer approved/i);
const reviewedSwitch = (): HTMLElement => screen.getByLabelText(/customer reviewed/i);
const saveButton = (): HTMLElement => screen.getByRole("button", { name: /save/i });

describe("EditChangeRequestDialog — Customer approved / reviewed mutual exclusion", () => {
  it("allows toggling only 'Customer approved' and saves just that field", () => {
    const { onSave } = renderDialog();
    fireEvent.click(approvedSwitch());
    fireEvent.click(saveButton());
    expect(onSave).toHaveBeenCalledWith({ isCustomerApproved: true });
  });

  it("allows toggling only 'Customer reviewed' and saves just that field", () => {
    const { onSave } = renderDialog();
    fireEvent.click(reviewedSwitch());
    fireEvent.click(saveButton());
    expect(onSave).toHaveBeenCalledWith({ isCustomerReviewed: true });
  });

  it("locks 'Customer reviewed' once 'Customer approved' has been changed, so both can never be sent together", () => {
    renderDialog();
    fireEvent.click(approvedSwitch());
    expect(reviewedSwitch()).toBeDisabled();
    expect(approvedSwitch()).toBeEnabled();
  });

  it("locks 'Customer approved' once 'Customer reviewed' has been changed", () => {
    renderDialog();
    fireEvent.click(reviewedSwitch());
    expect(approvedSwitch()).toBeDisabled();
    expect(reviewedSwitch()).toBeEnabled();
  });

  it("explains why the other switch is locked", () => {
    renderDialog();
    fireEvent.click(approvedSwitch());
    expect(
      screen.getByText(/can't be changed in the same save/i),
    ).toBeInTheDocument();
  });

  it("unlocks the other switch again once the change is undone", () => {
    renderDialog();
    fireEvent.click(approvedSwitch());
    expect(reviewedSwitch()).toBeDisabled();
    fireEvent.click(approvedSwitch());
    expect(reviewedSwitch()).toBeEnabled();
  });

  it("never builds a patch containing both isCustomerApproved and isCustomerReviewed, even via the disabled control", () => {
    const { onSave } = renderDialog();
    fireEvent.click(approvedSwitch());
    // Attempting to click the now-disabled control is a no-op in the DOM;
    // this documents that expectation rather than relying on it silently.
    fireEvent.click(reviewedSwitch());
    fireEvent.click(saveButton());
    expect(onSave).toHaveBeenCalledWith({ isCustomerApproved: true });
    const [patch] = onSave.mock.calls[0];
    expect(patch).not.toHaveProperty("isCustomerReviewed");
  });
});

describe("EditChangeRequestDialog — save error surfacing", () => {
  it("renders no error alert by default", () => {
    renderDialog();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("renders the given saveError as a visible alert inside the dialog", () => {
    const onClose = vi.fn();
    render(
      <EditChangeRequestDialog
        cr={BASE_CR}
        isSaving={false}
        saveError="isCustomerApproved, isCustomerReviewed, and requestApproval are mutually exclusive"
        onClose={onClose}
        onSave={vi.fn()}
      />,
    );
    expect(
      screen.getByRole("alert"),
    ).toHaveTextContent(/mutually exclusive/i);
  });
});
