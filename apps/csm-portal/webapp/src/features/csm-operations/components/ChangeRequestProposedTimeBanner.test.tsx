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
import { fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import ChangeRequestProposedTimeBanner from "@features/csm-operations/components/ChangeRequestProposedTimeBanner";
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

// "Not recorded": the backend names nobody.
const UNKNOWN: BeChangeRequestCustomerProposal = { startOn: KNOWN.startOn, endOn: KNOWN.endOn, answer: "pending" };

// Well before every time above, so the proposal is in the future.
const NOW = Date.UTC(2030, 1, 15);

function renderBanner(
  props: Partial<Omit<ComponentProps<typeof ChangeRequestProposedTimeBanner>, "cr">> & { cr?: Partial<BeChangeRequestDetail> } = {},
): { onAccept: ReturnType<typeof vi.fn>; onProposeDifferent: ReturnType<typeof vi.fn> } {
  const onAccept = vi.fn();
  const onProposeDifferent = vi.fn();
  const { cr, ...rest } = props;
  render(
    <ChangeRequestProposedTimeBanner
      cr={{ ...CR, ...cr }}
      proposal={KNOWN}
      isPending={false}
      nowMs={NOW}
      onAccept={onAccept}
      onProposeDifferent={onProposeDifferent}
      {...rest}
    />,
  );
  return { onAccept, onProposeDifferent };
}

const acceptButton = (): HTMLElement => screen.getByRole("button", { name: "Accept proposed time" });
const counterButton = (): HTMLElement => screen.getByRole("button", { name: "Propose a different time" });

describe("ChangeRequestProposedTimeBanner", () => {
  beforeEach(() => setUserPreferredTimeZone("UTC"));
  afterEach(() => vi.restoreAllMocks());
  afterEach(() => clearUserPreferredTimeZone());

  it("is a named region that says the customer proposed a new time", () => {
    renderBanner();
    const region = screen.getByRole("region", { name: "The customer proposed a new time" });
    expect(region).toBeInTheDocument();
    expect(region).toHaveTextContent(/The planned time stays as it is until you answer\./);
  });

  it("shows the planned window beside the proposed one, with the length the customer's proposal keeps", () => {
    renderBanner();
    const region = screen.getByRole("region");
    expect(within(region).getByText("Planned now")).toBeInTheDocument();
    expect(within(region).getByText("Mar 1, 2030, 9:00 AM to Mar 1, 2030, 11:00 AM")).toBeInTheDocument();
    expect(within(region).getByText("Proposed by the customer")).toBeInTheDocument();
    expect(within(region).getByText("Mar 8, 2030, 9:00 AM to Mar 8, 2030, 11:00 AM")).toBeInTheDocument();
    expect(within(region).getByText("Same length as the planned window (2 hours)")).toBeInTheDocument();
  });

  it("derives the proposed end from the planned length when the backend sent none", () => {
    renderBanner({ proposal: { startOn: "2030-03-08T10:00:00Z", answer: "pending", proposedByName: "Mia Member" } });
    expect(screen.getByText("Mar 8, 2030, 10:00 AM to Mar 8, 2030, 12:00 PM")).toBeInTheDocument();
  });

  it("shows the viewer's own time zone", () => {
    setUserPreferredTimeZone("Asia/Colombo");
    renderBanner();
    // 09:00 UTC is 14:30 in Colombo.
    expect(screen.getByText("Mar 8, 2030, 2:30 PM to Mar 8, 2030, 4:30 PM")).toBeInTheDocument();
  });

  describe("the proposer is known", () => {
    it("names who proposed it and when, with Accept as the one primary action", () => {
      renderBanner();
      expect(screen.getByTestId("cr-proposal-proposer")).toHaveTextContent(
        "Proposed by Mia Member (mia.member@example.com) on Feb 1, 2030, 10:00 AM.",
      );
      expect(screen.queryByText(/proposer is not recorded/i)).not.toBeInTheDocument();
      expect(acceptButton().className).toContain("MuiButton-contained");
      expect(counterButton().className).toContain("MuiButton-outlined");
      expect(screen.getAllByRole("button").filter((b) => b.className.includes("MuiButton-contained"))).toHaveLength(1);
    });

    it("names the proposer without a time when the backend sent none", () => {
      renderBanner({ proposal: { ...KNOWN, proposedOn: null } });
      expect(screen.getByTestId("cr-proposal-proposer")).toHaveTextContent("Proposed by Mia Member (mia.member@example.com).");
    });

    it("the two buttons call the caller; neither sends anything itself", () => {
      const { onAccept, onProposeDifferent } = renderBanner();
      fireEvent.click(acceptButton());
      expect(onAccept).toHaveBeenCalledTimes(1);
      expect(onProposeDifferent).not.toHaveBeenCalled();
      fireEvent.click(counterButton());
      expect(onProposeDifferent).toHaveBeenCalledTimes(1);
    });
  });

  describe("the proposer is NOT recorded (a date WSO2 users write too, or one left over from an earlier round)", () => {
    it("says so, and tells the engineer to check the time came from the customer", () => {
      renderBanner({ proposal: UNKNOWN });
      const note = screen.getByTestId("cr-proposal-proposer");
      expect(note).toHaveTextContent("The proposer is not recorded.");
      expect(note).toHaveTextContent(/Check that this time really came from the customer before you accept it/);
      expect(note).not.toHaveTextContent(/Proposed by/);
    });

    it("Accept is no longer the single primary action: both answers are outlined and equally prominent", () => {
      renderBanner({ proposal: UNKNOWN });
      expect(acceptButton().className).toContain("MuiButton-outlined");
      expect(counterButton().className).toContain("MuiButton-outlined");
      expect(screen.getAllByRole("button").filter((b) => b.className.includes("MuiButton-contained"))).toHaveLength(0);
      // Both still work: the confirmation lives in the Accept dialog.
      expect(acceptButton()).toBeEnabled();
      expect(counterButton()).toBeEnabled();
    });

    it("an email alone, or a name alone, is still a proposer on record", () => {
      renderBanner({ proposal: { ...UNKNOWN, proposedByEmail: "mia.member@example.com" } });
      expect(screen.getByTestId("cr-proposal-proposer")).toHaveTextContent("Proposed by mia.member@example.com.");
      expect(acceptButton().className).toContain("MuiButton-contained");
    });
  });

  describe("what Accept cannot do, said up front (the backend refuses each in words as well)", () => {
    it("is disabled once the proposed time has passed, with a focusable reason", () => {
      renderBanner({ nowMs: Date.UTC(2030, 2, 9) });
      expect(acceptButton()).toBeDisabled();
      expect(screen.getByLabelText("Accept proposed time: The proposed time has passed. Propose a different time.")).toHaveAttribute("tabindex", "0");
      expect(counterButton()).toBeEnabled();
    });

    it("is disabled while the change is on hold", () => {
      renderBanner({ cr: { onHold: true } });
      expect(acceptButton()).toBeDisabled();
      expect(screen.getByLabelText("Accept proposed time: This change request is on hold. Take it off hold first.")).toBeInTheDocument();
      expect(counterButton()).toBeEnabled();
    });

    it("is disabled when there is no planned window whose length the proposal could keep", () => {
      renderBanner({ cr: { plannedStartOn: null, plannedEndOn: null } });
      expect(acceptButton()).toBeDisabled();
      expect(screen.getByLabelText(/Accept proposed time: This change request has no planned window/)).toBeInTheDocument();
    });

    it("a click on the disabled Accept does nothing", () => {
      const { onAccept } = renderBanner({ cr: { onHold: true } });
      fireEvent.click(acceptButton());
      expect(onAccept).not.toHaveBeenCalled();
    });
  });

  it("disables both answers while a request is in flight", () => {
    renderBanner({ isPending: true });
    expect(acceptButton()).toBeDisabled();
    expect(counterButton()).toBeDisabled();
  });

  it("offers no Decline of its own: declining is keeping the current time in 'Propose a different time'", () => {
    renderBanner();
    expect(screen.queryByRole("button", { name: /decline|reject/i })).not.toBeInTheDocument();
  });
});
