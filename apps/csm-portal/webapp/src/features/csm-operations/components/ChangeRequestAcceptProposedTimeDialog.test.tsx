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

import type { ComponentProps } from "react";
import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import ChangeRequestAcceptProposedTimeDialog from "@features/csm-operations/components/ChangeRequestAcceptProposedTimeDialog";
import { clearUserPreferredTimeZone, setUserPreferredTimeZone } from "@utils/dateTime";
import type { BeChangeRequestCustomerProposal, BeChangeRequestDetail } from "@api/backend/types";

// Synthetic: shapes only.
const CR: BeChangeRequestDetail = {
  id: "chg-1",
  number: "CHG0001234",
  subject: "Upgrade the gateway cluster",
  createdOn: "2026-01-01T00:00:00Z",
  state: "customer_approval",
  type: "normal",
  plannedStartOn: "2030-03-01 09:00:00",
  plannedEndOn: "2030-03-01 11:00:00",
};

const KNOWN: BeChangeRequestCustomerProposal = {
  startOn: "2030-03-08T09:00:00Z",
  endOn: "2030-03-08T11:00:00Z",
  answer: "pending",
  proposedByName: "Mia Member",
  proposedByEmail: "mia.member@example.com",
  proposedOn: "2030-02-01T10:00:00Z",
};
const UNKNOWN: BeChangeRequestCustomerProposal = { startOn: KNOWN.startOn, endOn: KNOWN.endOn, answer: "pending" };

function renderDialog(
  props: Partial<ComponentProps<typeof ChangeRequestAcceptProposedTimeDialog>> = {},
): { onConfirm: ReturnType<typeof vi.fn>; onClose: ReturnType<typeof vi.fn> } {
  const onConfirm = vi.fn();
  const onClose = vi.fn();
  render(
    <ChangeRequestAcceptProposedTimeDialog
      cr={CR}
      proposal={KNOWN}
      isSubmitting={false}
      onClose={onClose}
      onConfirm={onConfirm}
      {...props}
    />,
  );
  return { onConfirm, onClose };
}

const confirmButton = (): HTMLElement =>
  within(screen.getByRole("dialog")).getByRole("button", { name: "Accept proposed time" });

describe("ChangeRequestAcceptProposedTimeDialog", () => {
  beforeEach(() => setUserPreferredTimeZone("UTC"));
  afterEach(() => clearUserPreferredTimeZone());

  it("asks 'Accept the proposed time?' and says what will be scheduled and what does NOT follow", () => {
    renderDialog();
    expect(screen.getByRole("heading", { name: "Accept the proposed time?" })).toBeInTheDocument();
    expect(screen.getByText(/The change will be scheduled for Mar 8, 2030, 9:00 AM to Mar 8, 2030, 11:00 AM\./)).toBeInTheDocument();
    expect(screen.getByText(/The customer sees that you accepted it and is not asked again\. No CAB approval is needed\./)).toBeInTheDocument();
  });

  it("shows the planned window beside the proposed one", () => {
    renderDialog();
    const dialog = screen.getByRole("dialog");
    expect(within(dialog).getByText("Planned now")).toBeInTheDocument();
    expect(within(dialog).getByText("Mar 1, 2030, 9:00 AM to Mar 1, 2030, 11:00 AM")).toBeInTheDocument();
    expect(within(dialog).getByText("Proposed by the customer")).toBeInTheDocument();
    expect(within(dialog).queryByText("Proposed time")).not.toBeInTheDocument();
    // The window appears once as the sentence's own and once as the proposed block.
    expect(within(dialog).getAllByText(/Mar 8, 2030, 9:00 AM to Mar 8, 2030, 11:00 AM/)).toHaveLength(2);
  });

  describe("the proposer is known", () => {
    it("names them, asks for nothing more, and confirms with one click", () => {
      const { onConfirm } = renderDialog();
      expect(screen.getByText("Proposed by Mia Member (mia.member@example.com) on Feb 1, 2030, 10:00 AM.")).toBeInTheDocument();
      expect(screen.queryByRole("checkbox")).not.toBeInTheDocument();
      expect(screen.queryByText(/proposer is not recorded/i)).not.toBeInTheDocument();
      expect(confirmButton()).toBeEnabled();
      fireEvent.click(confirmButton());
      expect(onConfirm).toHaveBeenCalledTimes(1);
    });
  });

  describe("the proposer is NOT recorded", () => {
    it("labels the window 'Proposed time', not 'Proposed by the customer', however the backend says it", () => {
      for (const proposal of [
        UNKNOWN,
        { ...UNKNOWN, proposerRecorded: false, proposedByName: "Mia Member", proposedByEmail: "mia.member@example.com" },
      ]) {
        const { unmount } = render(
          <ChangeRequestAcceptProposedTimeDialog cr={CR} proposal={proposal} isSubmitting={false} onClose={vi.fn()} onConfirm={vi.fn()} />,
        );
        const dialog = screen.getByRole("dialog");
        expect(within(dialog).getByText("Proposed time")).toBeInTheDocument();
        expect(within(dialog).queryByText("Proposed by the customer")).not.toBeInTheDocument();
        expect(dialog).not.toHaveTextContent(/Proposed by /);
        // What follows from accepting is unchanged, and so is the caveat and the explicit confirmation.
        expect(dialog).toHaveTextContent("The proposer is not recorded.");
        expect(within(dialog).getByRole("checkbox", { name: "I have checked that the customer proposed this time." })).toBeInTheDocument();
        unmount();
      }
    });

    it("says so and holds Accept back until the engineer confirms the customer proposed this time", () => {
      const { onConfirm } = renderDialog({ proposal: UNKNOWN });
      const dialog = screen.getByRole("dialog");
      expect(within(dialog).getByRole("status")).toHaveTextContent("The proposer is not recorded.");
      expect(within(dialog).getByRole("status")).toHaveTextContent(/Check that this time really came from the customer/);
      const check = within(dialog).getByRole("checkbox", { name: "I have checked that the customer proposed this time." });
      expect(check).not.toBeChecked();
      expect(confirmButton()).toBeDisabled();
      fireEvent.click(confirmButton());
      expect(onConfirm).not.toHaveBeenCalled();

      fireEvent.click(check);
      expect(check).toBeChecked();
      expect(confirmButton()).toBeEnabled();
      fireEvent.click(confirmButton());
      expect(onConfirm).toHaveBeenCalledTimes(1);

      // Un-ticking holds it back again.
      fireEvent.click(check);
      expect(confirmButton()).toBeDisabled();
    });
  });

  it("shows the backend's refusal verbatim", () => {
    renderDialog({ error: "the customer's proposed time changed after you opened this change request (it is now 2030-03-09T09:00:00Z); read it again before responding" });
    expect(screen.getByRole("alert")).toHaveTextContent(
      "the customer's proposed time changed after you opened this change request (it is now 2030-03-09T09:00:00Z); read it again before responding",
    );
  });

  it("holds Accept back, and says to close the dialog, when the page moved on behind a refused attempt", () => {
    const { onConfirm, onClose } = renderDialog({ stale: true, error: "the planned implementation time of this change request changed after you opened it" });
    expect(screen.getByRole("alert")).toHaveTextContent("changed after you opened it");
    expect(screen.getByRole("status")).toHaveTextContent(/Close\s+this dialog to see the current state/);
    expect(confirmButton()).toBeDisabled();
    fireEvent.click(confirmButton());
    expect(onConfirm).not.toHaveBeenCalled();
    fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Close" }));
    expect(onClose).toHaveBeenCalled();
  });

  it("closes via Close, and neither action works while submitting", () => {
    const { onClose } = renderDialog();
    fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Close" }));
    expect(onClose).toHaveBeenCalled();
    cleanup();
    renderDialog({ isSubmitting: true });
    expect(within(screen.getByRole("dialog")).getByRole("button", { name: "Close" })).toBeDisabled();
    expect(confirmButton()).toBeDisabled();
  });
});
