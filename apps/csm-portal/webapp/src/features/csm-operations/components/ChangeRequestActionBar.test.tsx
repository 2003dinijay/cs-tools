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

import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi, type Mock } from "vitest";
import "@testing-library/jest-dom/vitest";
import ChangeRequestActionBar from "@features/csm-operations/components/ChangeRequestActionBar";
import type { PendingCustomerRequest } from "@features/csm-operations/utils/changeRequests";
import type { BeChangeRequestDetail } from "@api/backend/types";

const BASE_CR: BeChangeRequestDetail = {
  id: "chg-1",
  number: "CHG0009988",
  subject: "Upgrade the gateway cluster",
  createdOn: "2026-01-01T00:00:00Z",
  state: "new",
  type: "normal",
  assignedTeam: { id: "team-1", name: "Platform" },
};

function renderBar(
  overrides: Partial<BeChangeRequestDetail>,
  {
    isPending = false,
    onAction = vi.fn<(target: string) => void>(),
    pendingCustomerRequest,
  }: {
    isPending?: boolean;
    onAction?: Mock<(target: string) => void>;
    pendingCustomerRequest?: PendingCustomerRequest | null;
  } = {},
): { onAction: Mock<(target: string) => void>; container: HTMLElement } {
  const { container } = render(
    <ChangeRequestActionBar
      cr={{ ...BASE_CR, ...overrides }}
      isPending={isPending}
      pendingCustomerRequest={pendingCustomerRequest}
      onAction={onAction}
    />,
  );
  return { onAction, container };
}

/** Open the overflow menu, which must exist for this to succeed. */
function openMenu(): void {
  fireEvent.click(screen.getByRole("button", { name: /change state/i }));
}

describe("ChangeRequestActionBar — driven only by legalNextStates", () => {
  it("renders nothing when legalNextStates is absent", () => {
    const { container } = renderBar({ legalNextStates: undefined });
    expect(container).toBeEmptyDOMElement();
  });

  it("renders nothing when legalNextStates is empty", () => {
    const { container } = renderBar({ legalNextStates: [] });
    expect(container).toBeEmptyDOMElement();
  });

  it("renders nothing when the only entry is the CR's own current state", () => {
    const { container } = renderBar({ state: "assess", legalNextStates: ["assess"] });
    expect(container).toBeEmptyDOMElement();
  });

  it("offers only the states present in legalNextStates, not the whole lifecycle", () => {
    renderBar({ state: "scheduled", legalNextStates: ["implement", "canceled"] });
    // "Start implementation" is the forward move -> primary button.
    expect(
      screen.getByRole("button", { name: /start implementation/i }),
    ).toBeInTheDocument();
    openMenu();
    expect(screen.getByRole("menuitem", { name: /cancel change/i })).toBeInTheDocument();
    // Never offered: legal elsewhere in the lifecycle, but not in this array.
    expect(screen.queryByRole("menuitem", { name: /^close$/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("menuitem", { name: /roll back/i })).not.toBeInTheDocument();
  });

  it("renders a state it has no curated config for, via the generic fallback", () => {
    renderBar({ state: "review", legalNextStates: ["closed", "awaiting_vendor"] });
    openMenu();
    // Sentence-cased from the raw value — no frontend change was needed for it.
    expect(
      screen.getByRole("menuitem", { name: /^awaiting vendor$/i }),
    ).toBeInTheDocument();
  });

  it("dispatches an uncurated state verbatim, not a normalised guess at it", () => {
    const { onAction } = renderBar({
      state: "review",
      legalNextStates: ["closed", "awaiting_vendor"],
    });
    openMenu();
    fireEvent.click(screen.getByRole("menuitem", { name: /^awaiting vendor$/i }));
    expect(onAction).toHaveBeenCalledWith("awaiting_vendor");
  });
});

describe("ChangeRequestActionBar — exactly one primary button", () => {
  it("promotes only the first forward move, even with six legal targets", () => {
    renderBar({
      state: "new",
      legalNextStates: [
        "closed",
        "customer_review",
        "review",
        "implement",
        "scheduled",
        "assess",
        "rollback",
        "canceled",
      ],
    });
    const contained = screen
      .getAllByRole("button")
      .filter((b) => b.className.includes("MuiButton-contained"));
    expect(contained).toHaveLength(1);
    expect(contained[0]).toHaveTextContent(/request approval/i);
  });

  it("puts every non-promoted target behind the Change state menu", () => {
    renderBar({ state: "new", legalNextStates: ["assess", "implement", "canceled"] });
    expect(screen.getByRole("button", { name: /request approval/i })).toBeInTheDocument();
    openMenu();
    expect(screen.getByRole("menuitem", { name: /start implementation/i })).toBeInTheDocument();
    expect(screen.getByRole("menuitem", { name: /cancel change/i })).toBeInTheDocument();
  });

  it("renders no overflow menu at all when the single legal target is the primary one", () => {
    renderBar({ state: "scheduled", legalNextStates: ["implement"] });
    expect(screen.getByRole("button", { name: /start implementation/i })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /change state/i })).not.toBeInTheDocument();
  });

  it("never promotes a destructive target: with only cancel legal, there is no primary button", () => {
    renderBar({ state: "implement", legalNextStates: ["canceled"] });
    expect(screen.queryByRole("button", { name: /cancel change/i })).not.toBeInTheDocument();
    openMenu();
    expect(screen.getByRole("menuitem", { name: /cancel change/i })).toBeInTheDocument();
  });

  it("never promotes an uncurated target, even when it is the only legal one", () => {
    renderBar({ state: "review", legalNextStates: ["awaiting_vendor"] });
    expect(
      screen.queryByRole("button", { name: /^awaiting vendor$/i }),
    ).not.toBeInTheDocument();
    openMenu();
    expect(screen.getByRole("menuitem", { name: /^awaiting vendor$/i })).toBeInTheDocument();
  });
});

describe("ChangeRequestActionBar — labels are the action, not the destination", () => {
  it.each([
    ["assess", /^request approval$/i],
    ["implement", /start implementation/i],
    ["review", /mark implemented/i],
    ["customer_review", /send for customer review/i],
    ["closed", /^close$/i],
  ])("labels the %s transition as the action taken", (target, label) => {
    renderBar({ state: "new", legalNextStates: [target] });
    expect(screen.getByRole("button", { name: label })).toBeInTheDocument();
  });
});

describe("ChangeRequestActionBar — dispatch", () => {
  it("calls onAction with the target when the primary button is clicked", () => {
    const { onAction } = renderBar({ state: "new", legalNextStates: ["assess"] });
    fireEvent.click(screen.getByRole("button", { name: /request approval/i }));
    expect(onAction).toHaveBeenCalledWith("assess");
  });

  it("calls onAction with the target when a menu item is clicked, and closes the menu", () => {
    const { onAction } = renderBar({
      state: "implement",
      legalNextStates: ["review", "canceled"],
    });
    openMenu();
    fireEvent.click(screen.getByRole("menuitem", { name: /cancel change/i }));
    expect(onAction).toHaveBeenCalledWith("canceled");
  });
});

/**
 * Neither state is human-enterable in the backing system, so the bar must not
 * offer them however they arrive in `legalNextStates`. See
 * `NEVER_OFFERED_TARGETS` for why the filter exists — these tests are what
 * stops it being removed as dead code.
 */
describe("ChangeRequestActionBar — states the bar never offers", () => {
  it("renders neither rollback (outside the review states) nor customer approval, as a button or a menu item", () => {
    renderBar({
      state: "implement",
      legalNextStates: ["review", "rollback", "customer_approval", "canceled"],
    });
    expect(screen.queryByRole("button", { name: /roll back/i })).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /customer approval/i }),
    ).not.toBeInTheDocument();
    openMenu();
    expect(screen.queryByRole("menuitem", { name: /roll back/i })).not.toBeInTheDocument();
    expect(
      screen.queryByRole("menuitem", { name: /customer approval/i }),
    ).not.toBeInTheDocument();
  });

  it("still renders the other legal targets normally alongside them", () => {
    renderBar({
      state: "implement",
      legalNextStates: ["review", "rollback", "customer_approval", "canceled"],
    });
    expect(screen.getByRole("button", { name: /mark implemented/i })).toBeInTheDocument();
    openMenu();
    expect(screen.getByRole("menuitem", { name: /cancel change/i })).toBeInTheDocument();
  });

  it("excludes them even when they would otherwise render through the generic fallback", () => {
    // `customer_approval` has no curated action label, so without the
    // exclusion it would still be renderable via `DEFAULT_TARGET_CONFIG`.
    renderBar({ state: "assess", legalNextStates: ["customer_approval", "awaiting_vendor"] });
    openMenu();
    expect(screen.getByRole("menuitem", { name: /^awaiting vendor$/i })).toBeInTheDocument();
    expect(
      screen.queryByRole("menuitem", { name: /customer approval/i }),
    ).not.toBeInTheDocument();
  });

  it("renders no bar at all when every legal target is excluded", () => {
    const { container } = renderBar({
      state: "implement",
      legalNextStates: ["rollback", "customer_approval"],
    });
    expect(container).toBeEmptyDOMElement();
  });

  it("never offers rollback from any state but review and customer_review, even if the backend listed it", () => {
    for (const state of [
      "new", "assess", "authorize", "customer_approval", "scheduled", "implement",
      "closed", "canceled", "rollback",
    ]) {
      cleanup();
      const { container } = renderBar({ state, legalNextStates: ["rollback"] });
      expect(container, state).toBeEmptyDOMElement();
    }
  });

  /**
   * Authorize must only ever be reached as the automatic side effect of an
   * approver approving in the Approvers section (`ChangeRequestApprovals.tsx`)
   * — never via a direct click here, even from Assess, where it would
   * otherwise be the obvious next forward move. Same exclusion mechanism as
   * rollback/customer_approval above, for a different reason (a human
   * decision made elsewhere in the UI, not automation).
   */
  it("never offers authorize from Assess, as a button or a menu item", () => {
    renderBar({
      state: "assess",
      legalNextStates: ["authorize", "canceled"],
    });
    expect(screen.queryByRole("button", { name: /^authorize$/i })).not.toBeInTheDocument();
    openMenu();
    expect(screen.queryByRole("menuitem", { name: /^authorize$/i })).not.toBeInTheDocument();
  });

  it("excludes authorize even when it would otherwise render through the generic fallback", () => {
    // `authorize` has no curated action label, so without the exclusion it
    // would still be renderable via `DEFAULT_TARGET_CONFIG`.
    renderBar({ state: "assess", legalNextStates: ["authorize", "awaiting_vendor"] });
    openMenu();
    expect(screen.getByRole("menuitem", { name: /^awaiting vendor$/i })).toBeInTheDocument();
    expect(screen.queryByRole("menuitem", { name: /^authorize$/i })).not.toBeInTheDocument();
  });

  it("renders no bar at all from Assess when authorize is the only legal target", () => {
    const { container } = renderBar({
      state: "assess",
      legalNextStates: ["authorize"],
    });
    expect(container).toBeEmptyDOMElement();
  });
});

describe("ChangeRequestActionBar — pending state", () => {
  it("disables the primary button while a transition is in flight", () => {
    renderBar({ state: "new", legalNextStates: ["assess", "canceled"] }, { isPending: true });
    expect(screen.getByRole("button", { name: /request approval/i })).toBeDisabled();
  });

  it("disables the Change state menu trigger while a transition is in flight", () => {
    renderBar({ state: "new", legalNextStates: ["assess", "canceled"] }, { isPending: true });
    expect(screen.getByRole("button", { name: /change state/i })).toBeDisabled();
  });
});

/**
 * "Request Approval" (New -> Assess) requires an assigned team: its members
 * are who the Peer Approval stage is provisioned for.
 */
describe("ChangeRequestActionBar — per-target blocked reasons", () => {
  it("disables the assess transition when the CR has no assigned team", () => {
    const { onAction } = renderBar({
      state: "new",
      legalNextStates: ["assess"],
      assignedTeam: null,
    });
    const button = screen.getByRole("button", { name: /request approval/i });
    expect(button).toBeDisabled();
    fireEvent.click(button);
    expect(onAction).not.toHaveBeenCalled();
  });

  it("exposes the blocked reason to keyboard users via a focusable, labelled wrapper", () => {
    renderBar({ state: "new", legalNextStates: ["assess"], assignedTeam: null });
    const focusTarget = screen
      .getByRole("button", { name: /request approval/i })
      .closest('[tabindex="0"]');
    expect(focusTarget).not.toBeNull();
    expect(focusTarget).toHaveAttribute(
      "aria-label",
      "Request Approval: Set an assigned team before requesting approval",
    );
  });

  it("leaves the transition enabled once the prerequisite is met", () => {
    renderBar({ state: "new", legalNextStates: ["assess"] });
    expect(screen.getByRole("button", { name: /request approval/i })).toBeEnabled();
  });

  it("blocks only the target with the unmet prerequisite, leaving the others clickable", () => {
    // `assess` is blocked *and* is first in FORWARD_ORDER, so it stays the
    // promoted (disabled) primary while `canceled` stays usable behind the
    // menu — a blocked target must not take the rest of the bar down with it.
    const { onAction } = renderBar({
      state: "new",
      legalNextStates: ["canceled", "assess"],
      assignedTeam: null,
    });
    expect(screen.getByRole("button", { name: /request approval/i })).toBeDisabled();
    openMenu();
    fireEvent.click(screen.getByRole("menuitem", { name: /cancel change/i }));
    expect(onAction).toHaveBeenCalledWith("canceled");
  });
});

/**
 * CAB (or ECAB) approval moves a CR to Scheduled automatically, and a Standard
 * change goes straight there from Request Approval -- there is no manual
 * "Schedule" button. The backend no longer lists `scheduled` in
 * `legalNextStates`; the bar also filters it defensively.
 */
describe("ChangeRequestActionBar — Request Approval flow, no manual Schedule", () => {
  it("shows 'Request Approval' and never 'Move to Assess' for a new CR", () => {
    renderBar({ state: "new", legalNextStates: ["assess", "canceled"] });
    expect(screen.getByRole("button", { name: "Request Approval" })).toBeInTheDocument();
    expect(screen.queryByText(/move to assess/i)).not.toBeInTheDocument();
  });

  it("never offers Schedule, as a button or menu item, even if the backend lists scheduled", () => {
    renderBar({ state: "authorize", legalNextStates: ["scheduled", "canceled"] });
    expect(screen.queryByRole("button", { name: /schedule/i })).not.toBeInTheDocument();
    openMenu();
    expect(screen.queryByRole("menuitem", { name: /schedule/i })).not.toBeInTheDocument();
    expect(screen.getByRole("menuitem", { name: /cancel change/i })).toBeInTheDocument();
  });

  it("renders no bar at all when scheduled is the only legal target", () => {
    const { container } = renderBar({ state: "authorize", legalNextStates: ["scheduled"] });
    expect(container).toBeEmptyDOMElement();
  });

  it("offers no Schedule for an Assess-stage CR (approval pending), only Cancel", () => {
    renderBar({ state: "assess", legalNextStates: ["authorize", "scheduled", "canceled"] });
    openMenu();
    expect(screen.queryByRole("menuitem", { name: /schedule|authorize/i })).not.toBeInTheDocument();
    expect(screen.getByRole("menuitem", { name: /cancel change/i })).toBeInTheDocument();
  });

  it("from Scheduled, the forward move is Start implementation (the CR got there automatically)", () => {
    renderBar({ state: "scheduled", legalNextStates: ["implement", "canceled"] });
    expect(screen.getByRole("button", { name: /start implementation/i })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /schedule/i })).not.toBeInTheDocument();
  });
});

/**
 * Customer Approval / Customer Review gates. `scheduled` is a manual target in
 * exactly one place: leaving `customer_approval`, where it is the "Bypass
 * customer approval" action (an engineer answering for the customer); `closed`
 * out of `customer_review` is "Bypass customer review". Both are menu-only.
 * `legalNextStates` stays the single source of truth for which of Close / Send
 * for customer review the Review state offers.
 */
describe("ChangeRequestActionBar — customer approval and customer review gates", () => {
  it("from customer_approval offers 'Bypass customer approval' and Cancel in the menu only, no primary", () => {
    const { onAction } = renderBar({
      state: "customer_approval",
      customerApprovalRequired: true,
      legalNextStates: ["scheduled", "canceled"],
    });
    // Never a button of its own, and never the retired wording.
    expect(screen.queryByRole("button", { name: /bypass customer approval/i })).not.toBeInTheDocument();
    expect(screen.queryByText(/record customer approval/i)).not.toBeInTheDocument();
    // "Change state" is the only button, so it is the contained one.
    const buttons = screen.getAllByRole("button");
    expect(buttons.map((b) => b.textContent)).toEqual(["Change state"]);
    expect(buttons[0].className).toContain("MuiButton-contained");
    openMenu();
    expect(screen.getAllByRole("menuitem").map((i) => i.textContent)).toEqual([
      "Bypass customer approval",
      "Cancel change",
    ]);
    // No Schedule wording.
    expect(screen.queryByText(/^schedule/i)).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("menuitem", { name: "Bypass customer approval" }));
    // Sent as a plain PATCH {state:"scheduled"} by the caller.
    expect(onAction).toHaveBeenCalledWith("scheduled");
  });

  it("never offers the customer_approval state itself as an action, even when listed", () => {
    renderBar({
      state: "authorize",
      legalNextStates: ["customer_approval", "canceled"],
    });
    expect(screen.queryByRole("button", { name: /customer approval/i })).not.toBeInTheDocument();
    openMenu();
    expect(screen.queryByRole("menuitem", { name: /customer approval/i })).not.toBeInTheDocument();
  });

  it("offers no bypass outside customer_approval, even if scheduled is listed", () => {
    for (const state of ["new", "assess", "authorize", "scheduled", "implement", "review", "customer_review"]) {
      const { container } = renderBar({
        state,
        legalNextStates: ["scheduled"],
      });
      expect(container).toBeEmptyDOMElement();
      expect(screen.queryByText(/bypass customer/i)).not.toBeInTheDocument();
      cleanup();
    }
  });

  it("offers no button or menu item labelled Schedule/Scheduled in any state", () => {
    for (const state of ["new", "assess", "authorize", "customer_approval", "scheduled", "implement", "review", "customer_review"]) {
      renderBar({
        state,
        legalNextStates: ["assess", "scheduled", "implement", "review", "customer_review", "closed", "canceled"],
      });
      expect(screen.queryByRole("button", { name: /schedul/i })).not.toBeInTheDocument();
      const trigger = screen.queryByRole("button", { name: /change state/i });
      if (trigger) {
        fireEvent.click(trigger);
        expect(screen.queryByRole("menuitem", { name: /schedul/i })).not.toBeInTheDocument();
      }
      cleanup();
    }
  });

  it("Review with customer review NOT required offers Close (primary, not a bypass) and Cancel, and no customer review", () => {
    const { onAction } = renderBar({
      state: "review",
      customerReviewRequired: false,
      legalNextStates: ["closed", "canceled"],
    });
    const close = screen.getByRole("button", { name: "Close" });
    expect(close.className).toContain("MuiButton-contained");
    expect(screen.queryByText(/send for customer review/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/bypass/i)).not.toBeInTheDocument();
    openMenu();
    expect(screen.queryByRole("menuitem", { name: /customer review/i })).not.toBeInTheDocument();
    expect(screen.getByRole("menuitem", { name: /cancel change/i })).toBeInTheDocument();
    fireEvent.keyDown(screen.getByRole("menu"), { key: "Escape" });
    fireEvent.click(close);
    expect(onAction).toHaveBeenCalledWith("closed");
  });

  it("Review with customer review required offers Send for customer review and Cancel, and no Close", () => {
    renderBar({
      state: "review",
      customerReviewRequired: true,
      legalNextStates: ["customer_review", "canceled"],
    });
    expect(screen.getByRole("button", { name: "Send for customer review" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Close" })).not.toBeInTheDocument();
    openMenu();
    expect(screen.queryByRole("menuitem", { name: /^close$/i })).not.toBeInTheDocument();
    expect(screen.getByRole("menuitem", { name: /cancel change/i })).toBeInTheDocument();
  });

  it("customer_review offers 'Bypass customer review' and Cancel in the menu only, never a Close button", () => {
    const { onAction } = renderBar({
      state: "customer_review",
      customerReviewRequired: true,
      legalNextStates: ["closed", "canceled"],
    });
    expect(screen.queryByRole("button", { name: /^close$/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /bypass customer review/i })).not.toBeInTheDocument();
    expect(screen.queryByText(/send for customer review/i)).not.toBeInTheDocument();
    expect(screen.getAllByRole("button").map((b) => b.textContent)).toEqual(["Change state"]);
    openMenu();
    expect(screen.queryByRole("menuitem", { name: /^close$/i })).not.toBeInTheDocument();
    expect(screen.getAllByRole("menuitem").map((i) => i.textContent)).toEqual([
      "Bypass customer review",
      "Cancel change",
    ]);
    fireEvent.click(screen.getByRole("menuitem", { name: "Bypass customer review" }));
    expect(onAction).toHaveBeenCalledWith("closed");
  });
});

/**
 * With a live Customer Approval / Customer Review stage the backend offers only
 * `canceled`; with no customer group (no stage) it keeps offering the manual
 * bypass. The bar enables a bypass exactly when `legalNextStates` offers it,
 * and shows it disabled -- with who the customer request is waiting on -- when
 * the caller says a customer request is pending.
 */
describe("ChangeRequestActionBar — customer gates with and without a live customer stage", () => {
  const PENDING_APPROVAL: PendingCustomerRequest = {
    kind: "approval",
    contactNames: ["Mira Santos", "Noel Prasad"],
  };
  const PENDING_REVIEW: PendingCustomerRequest = { kind: "review", contactNames: ["Mira Santos"] };

  it("customer_approval with legalNextStates=[canceled] and nothing known to be pending offers only Cancel", () => {
    renderBar({ state: "customer_approval", customerApprovalRequired: true, legalNextStates: ["canceled"] });
    expect(screen.queryByRole("button", { name: /bypass customer approval/i })).not.toBeInTheDocument();
    // Cancel is destructive: menu-only, so it is the sole item behind "Change state".
    openMenu();
    expect(screen.getAllByRole("menuitem")).toHaveLength(1);
    expect(screen.getByRole("menuitem", { name: /cancel change/i })).toBeInTheDocument();
  });

  it("customer_approval fallback legalNextStates=[scheduled, canceled] enables the bypass, no pending request", () => {
    const { onAction } = renderBar({
      state: "customer_approval",
      customerApprovalRequired: true,
      legalNextStates: ["scheduled", "canceled"],
      customerContacts: [],
    });
    openMenu();
    const item = screen.getByRole("menuitem", { name: "Bypass customer approval" });
    expect(item).not.toHaveAttribute("aria-disabled", "true");
    // A bypass with nothing pending shows no explanation.
    expect(screen.queryByText(/pending/i)).not.toBeInTheDocument();
    fireEvent.click(item);
    expect(onAction).toHaveBeenCalledWith("scheduled");
  });

  it("customer_review with legalNextStates=[canceled] and nothing known to be pending offers only Cancel", () => {
    renderBar({ state: "customer_review", customerReviewRequired: true, legalNextStates: ["canceled"] });
    expect(screen.queryByRole("button", { name: /^close$/i })).not.toBeInTheDocument();
    openMenu();
    expect(screen.getAllByRole("menuitem")).toHaveLength(1);
    expect(screen.getByRole("menuitem", { name: /cancel change/i })).toBeInTheDocument();
  });

  it("customer_review fallback legalNextStates=[closed, canceled] enables 'Bypass customer review'", () => {
    renderBar({ state: "customer_review", customerReviewRequired: true, legalNextStates: ["closed", "canceled"] });
    openMenu();
    expect(screen.getByRole("menuitem", { name: "Bypass customer review" })).not.toHaveAttribute(
      "aria-disabled",
      "true",
    );
  });

  it("customer_approval with a pending request shows the bypass DISABLED, with who the customer is and where they answer", () => {
    const { onAction } = renderBar(
      { state: "customer_approval", customerApprovalRequired: true, legalNextStates: ["authorize", "canceled"] },
      { pendingCustomerRequest: PENDING_APPROVAL },
    );
    // Re-schedule stays an outlined button; the bypass is in the menu, ahead of Cancel change.
    expect(screen.getByRole("button", { name: "Re-schedule" }).className).toContain("MuiButton-outlined");
    openMenu();
    expect(screen.getAllByRole("menuitem").map((i) => i.textContent)).toEqual([
      "Bypass customer approvalCustomer approval is pending from Mira Santos, Noel Prasad. They answer in the Customer Portal, so it can't be bypassed from here.",
      "Cancel change",
    ]);
    const item = screen.getByRole("menuitem", { name: /^Bypass customer approval: / });
    expect(item).toHaveAttribute("aria-disabled", "true");
    expect(item).toHaveAttribute(
      "aria-label",
      "Bypass customer approval: Customer approval is pending from Mira Santos, Noel Prasad. They answer in the Customer Portal, so it can't be bypassed from here.",
    );
    fireEvent.click(item);
    expect(onAction).not.toHaveBeenCalled();
    // The rest of the menu still works.
    fireEvent.click(screen.getByRole("menuitem", { name: /cancel change/i }));
    expect(onAction).toHaveBeenCalledWith("canceled");
  });

  it("keeps the disabled bypass reachable by keyboard so its reason can be read", () => {
    renderBar(
      { state: "customer_approval", legalNextStates: ["authorize", "canceled"] },
      { pendingCustomerRequest: PENDING_APPROVAL },
    );
    openMenu();
    const item = screen.getByRole("menuitem", { name: /^Bypass customer approval: / });
    act(() => item.focus());
    expect(item).toHaveFocus();
    expect(item).not.toHaveAttribute("disabled");
  });

  it("customer_review with a pending request shows 'Bypass customer review' and Roll back DISABLED next to Cancel", () => {
    const { onAction } = renderBar(
      { state: "customer_review", customerReviewRequired: true, legalNextStates: ["canceled"] },
      { pendingCustomerRequest: PENDING_REVIEW },
    );
    expect(screen.getAllByRole("button").map((b) => b.textContent)).toEqual(["Change state"]);
    openMenu();
    expect(screen.getAllByRole("menuitem").map((i) => i.textContent)).toEqual([
      "Bypass customer reviewCustomer review is pending from Mira Santos. They answer in the Customer Portal, so it can't be bypassed from here.",
      "Roll backCustomer review is pending from Mira Santos. A failed review is theirs to give in the Customer Portal, so the change can't be rolled back from here.",
      "Cancel change",
    ]);
    const bypass = screen.getByRole("menuitem", { name: /^Bypass customer review: / });
    const rollBack = screen.getByRole("menuitem", { name: /^Roll back: / });
    expect(bypass).toHaveAttribute("aria-disabled", "true");
    expect(rollBack).toHaveAttribute("aria-disabled", "true");
    fireEvent.click(bypass);
    fireEvent.click(rollBack);
    expect(onAction).not.toHaveBeenCalled();
    // Cancel is the one way out an engineer still has.
    expect(screen.getByRole("menuitem", { name: /cancel change/i })).not.toHaveAttribute("aria-disabled", "true");
  });

  it("keeps Roll back disabled at Customer Review even if an older backend still lists it, and enables it with nothing pending", () => {
    const { onAction } = renderBar(
      { state: "customer_review", legalNextStates: ["closed", "rollback", "canceled"] },
      { pendingCustomerRequest: PENDING_REVIEW },
    );
    openMenu();
    expect(screen.getAllByRole("menuitem")).toHaveLength(3);
    const blocked = screen.getByRole("menuitem", { name: /^Roll back: / });
    expect(blocked).toHaveAttribute("aria-disabled", "true");
    fireEvent.click(blocked);
    expect(onAction).not.toHaveBeenCalled();
    cleanup();
    const second = renderBar(
      { state: "customer_review", legalNextStates: ["closed", "rollback", "canceled"] },
      { pendingCustomerRequest: null },
    );
    openMenu();
    const free = screen.getByRole("menuitem", { name: "Roll back" });
    expect(free).not.toHaveAttribute("aria-disabled", "true");
    fireEvent.click(free);
    expect(second.onAction).toHaveBeenCalledWith("rollback");
  });

  it("never blocks Roll back out of Review, whatever customer request is passed as pending", () => {
    renderBar(
      { state: "review", customerReviewRequired: true, legalNextStates: ["customer_review", "rollback", "canceled"] },
      { pendingCustomerRequest: PENDING_REVIEW },
    );
    openMenu();
    expect(screen.getAllByRole("menuitem").map((i) => i.textContent)).toEqual(["Roll back", "Cancel change"]);
    expect(screen.getByRole("menuitem", { name: "Roll back" })).not.toHaveAttribute("aria-disabled", "true");
  });

  it("adds no Roll back to Customer Approval, where a pending request withholds only the bypass", () => {
    renderBar(
      { state: "customer_approval", legalNextStates: ["authorize", "canceled"] },
      { pendingCustomerRequest: PENDING_APPROVAL },
    );
    openMenu();
    expect(screen.queryByRole("menuitem", { name: /roll back/i })).not.toBeInTheDocument();
  });

  it("uses a generic sentence when the pending request names nobody", () => {
    renderBar(
      { state: "customer_approval", legalNextStates: ["authorize", "canceled"] },
      { pendingCustomerRequest: { kind: "approval", contactNames: [] } },
    );
    openMenu();
    expect(screen.getByRole("menuitem", { name: /^Bypass customer approval: / })).toHaveTextContent(
      "Customer approval is pending. The customer answers in the Customer Portal, so it can't be bypassed from here.",
    );
  });

  it("disables the bypass even when an older backend still lists it while a customer request is pending", () => {
    const { onAction } = renderBar(
      { state: "customer_approval", legalNextStates: ["scheduled", "authorize", "canceled"] },
      { pendingCustomerRequest: PENDING_APPROVAL },
    );
    openMenu();
    // Listed by the backend and flagged pending: one entry, disabled.
    const items = screen.getAllByRole("menuitem");
    expect(items).toHaveLength(2);
    const item = screen.getByRole("menuitem", { name: /^Bypass customer approval: / });
    expect(item).toHaveAttribute("aria-disabled", "true");
    fireEvent.click(item);
    expect(onAction).not.toHaveBeenCalled();
  });

  it("adds the disabled bypass only alongside targets the backend offered", () => {
    // No legal targets (a record the caller may not transition): nothing to render.
    const { container } = renderBar(
      { state: "customer_approval", legalNextStates: [] },
      { pendingCustomerRequest: PENDING_APPROVAL },
    );
    expect(container).toBeEmptyDOMElement();
  });

  it("never turns a pending customer request into a block on an ordinary Close out of Review", () => {
    const { onAction } = renderBar(
      { state: "review", customerReviewRequired: false, legalNextStates: ["closed", "rollback", "canceled"] },
      { pendingCustomerRequest: PENDING_REVIEW },
    );
    const close = screen.getByRole("button", { name: "Close" });
    expect(close).toBeEnabled();
    openMenu();
    expect(screen.getAllByRole("menuitem").map((i) => i.textContent)).toEqual(["Roll back", "Cancel change"]);
    fireEvent.keyDown(screen.getByRole("menu"), { key: "Escape" });
    fireEvent.click(close);
    expect(onAction).toHaveBeenCalledWith("closed");
  });

  it("adds no bypass entry to states that are not customer gates, whatever is passed as pending", () => {
    for (const state of ["new", "assess", "authorize", "scheduled", "implement", "review"]) {
      cleanup();
      renderBar({ state, legalNextStates: ["canceled"] }, { pendingCustomerRequest: PENDING_APPROVAL });
      openMenu();
      expect(screen.getAllByRole("menuitem").map((i) => i.textContent), state).toEqual(["Cancel change"]);
    }
  });

  it("enables the bypass once the pending request is gone (e.g. the contact was removed from the project)", () => {
    const { onAction } = renderBar(
      { state: "customer_approval", legalNextStates: ["scheduled", "authorize", "canceled"] },
      { pendingCustomerRequest: null },
    );
    openMenu();
    const item = screen.getByRole("menuitem", { name: "Bypass customer approval" });
    expect(item).not.toHaveAttribute("aria-disabled", "true");
    fireEvent.click(item);
    expect(onAction).toHaveBeenCalledWith("scheduled");
  });

  it("disables an enabled bypass while a transition is in flight", () => {
    renderBar(
      { state: "customer_approval", legalNextStates: ["scheduled", "authorize", "canceled"] },
      { isPending: true },
    );
    // The whole menu trigger is disabled, as for every other target.
    expect(screen.getByRole("button", { name: /change state/i })).toBeDisabled();
  });
});

/**
 * How the bypass looks: menu-only, warning-coloured (not the error colour of
 * Roll back / Cancel change), and between the forward moves and the
 * destructive off-ramps.
 */
describe("ChangeRequestActionBar — customer bypass presentation", () => {
  it("is a menu item with a warning skip icon and the default label colour, not the error colour of Cancel change", () => {
    renderBar({ state: "customer_approval", legalNextStates: ["scheduled", "authorize", "canceled"] });
    openMenu();
    const bypass = screen.getByRole("menuitem", { name: "Bypass customer approval" });
    const cancel = screen.getByRole("menuitem", { name: /cancel change/i });
    // Label: the default text colour (the warning orange is 3.75:1 on the light menu, and the
    // skip icon and the word "Bypass" carry the emphasis), never the error red of Cancel change.
    expect(bypass.querySelector("span")).not.toHaveStyle({ color: "rgb(230, 81, 0)" });
    expect(bypass.querySelector("span")).not.toHaveStyle({ color: "rgb(211, 47, 47)" });
    expect(cancel.querySelector("span")).toHaveStyle({ color: "rgb(211, 47, 47)" });
    // Icon: warning, and a skip icon (lucide's skip-forward), distinct from Cancel's ban icon.
    const bypassIcon = bypass.querySelector("svg");
    expect(bypassIcon).toHaveClass("lucide-skip-forward");
    expect(bypassIcon?.parentElement).toHaveStyle({ color: "rgb(237, 108, 2)" });
    expect(cancel.querySelector("svg")).not.toHaveClass("lucide-skip-forward");
  });

  it("sits after the forward moves and before Roll back and Cancel change", () => {
    renderBar({
      state: "customer_review",
      legalNextStates: ["closed", "rollback", "canceled"],
    });
    openMenu();
    expect(screen.getAllByRole("menuitem").map((i) => i.textContent)).toEqual([
      "Bypass customer review",
      "Roll back",
      "Cancel change",
    ]);
  });

  it("is never the contained or the outlined button, in either gate, whatever else is legal", () => {
    for (const [state, legal] of [
      ["customer_approval", ["scheduled", "authorize", "canceled"]],
      ["customer_approval", ["scheduled", "canceled"]],
      ["customer_review", ["closed", "rollback", "canceled"]],
      ["customer_review", ["closed", "canceled"]],
    ] as const) {
      cleanup();
      renderBar({ state, legalNextStates: [...legal] });
      for (const b of screen.getAllByRole("button")) {
        expect(b.textContent, `${state} ${legal.join(",")}`).not.toMatch(/bypass/i);
      }
    }
  });
});

/**
 * The whole bar, state by state: which button is primary, which is the outlined
 * secondary, and what sits behind "Change state" (in order). Each row is the
 * legalNextStates the backend sends for that state with nothing blocking it.
 */
describe("ChangeRequestActionBar — what each state shows", () => {
  const TABLE: Array<{
    name: string;
    cr: Partial<BeChangeRequestDetail>;
    primary: string | null;
    secondary: string[];
    menu: string[];
  }> = [
    { name: "new", cr: { state: "new", legalNextStates: ["assess", "canceled"] }, primary: "Request Approval", secondary: [], menu: ["Cancel change"] },
    { name: "assess", cr: { state: "assess", legalNextStates: ["authorize", "canceled"] }, primary: null, secondary: [], menu: ["Cancel change"] },
    { name: "authorize", cr: { state: "authorize", legalNextStates: ["canceled"] }, primary: null, secondary: [], menu: ["Cancel change"] },
    {
      name: "customer_approval (no customer request live)",
      cr: { state: "customer_approval", legalNextStates: ["scheduled", "authorize", "canceled"] },
      primary: null,
      secondary: ["Re-schedule"],
      menu: ["Bypass customer approval", "Cancel change"],
    },
    { name: "scheduled", cr: { state: "scheduled", legalNextStates: ["implement", "canceled"] }, primary: "Start implementation", secondary: [], menu: ["Cancel change"] },
    { name: "implement", cr: { state: "implement", legalNextStates: ["review", "canceled"] }, primary: "Mark implemented", secondary: [], menu: ["Cancel change"] },
    {
      name: "review (customer review required)",
      cr: { state: "review", customerReviewRequired: true, legalNextStates: ["customer_review", "rollback", "canceled"] },
      primary: "Send for customer review",
      secondary: [],
      menu: ["Roll back", "Cancel change"],
    },
    {
      name: "review (no customer review)",
      cr: { state: "review", customerReviewRequired: false, legalNextStates: ["closed", "rollback", "canceled"] },
      primary: "Close",
      secondary: [],
      menu: ["Roll back", "Cancel change"],
    },
    {
      name: "customer_review (no customer request live)",
      cr: { state: "customer_review", legalNextStates: ["closed", "rollback", "canceled"] },
      primary: null,
      secondary: [],
      menu: ["Bypass customer review", "Roll back", "Cancel change"],
    },
  ];

  it.each(TABLE)("$name", ({ cr, primary, secondary, menu }) => {
    renderBar(cr);
    const contained = screen.queryAllByRole("button").filter((b) => b.className.includes("MuiButton-contained"));
    const outlined = screen
      .getAllByRole("button")
      .filter((b) => b.className.includes("MuiButton-outlined") && b.textContent !== "Change state");
    // Exactly one contained button: the primary move, or "Change state" when there is none.
    expect(contained.map((b) => b.textContent)).toEqual([primary ?? "Change state"]);
    expect(outlined.map((b) => b.textContent)).toEqual(secondary);
    openMenu();
    expect(screen.getAllByRole("menuitem").map((i) => i.textContent)).toEqual(menu);
  });

  it.each(["rollback", "closed", "canceled"])("%s is terminal and renders nothing", (state) => {
    const { container } = renderBar({ state, legalNextStates: [] });
    expect(container).toBeEmptyDOMElement();
  });
});

/** "Roll back": the failed-review off-ramp, offered from the two review states only. */
describe("ChangeRequestActionBar — Roll back", () => {
  it.each([
    ["review", ["closed", "rollback", "canceled"], /^close$/i, ["Roll back", "Cancel change"]],
    ["review", ["customer_review", "rollback", "canceled"], /send for customer review/i, ["Roll back", "Cancel change"]],
    // Out of Customer Review the forward move is the customer bypass: menu-only, so no primary at all.
    ["customer_review", ["closed", "rollback", "canceled"], null, ["Bypass customer review", "Roll back", "Cancel change"]],
  ])("from %s (%j) offers Roll back as a destructive menu item next to the forward move", (state, legal, forward, expected) => {
    const { onAction } = renderBar({ state, legalNextStates: legal });
    // At most one primary button: the forward move, never Roll back.
    if (forward) expect(screen.getByRole("button", { name: forward })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /roll back/i })).not.toBeInTheDocument();
    openMenu();
    const items = screen.getAllByRole("menuitem").map((i) => i.textContent);
    expect(items).toEqual(expected);
    const item = screen.getByRole("menuitem", { name: /roll back/i });
    // Error colour, same as Cancel change.
    expect(item.querySelector("span")).toHaveStyle({ color: "rgb(211, 47, 47)" });
    fireEvent.click(item);
    expect(onAction).toHaveBeenCalledWith("rollback");
  });

  it("is not offered while only Cancel is legal (a live customer stage), nor from a terminal state", () => {
    renderBar({ state: "customer_review", legalNextStates: ["canceled"] });
    openMenu();
    expect(screen.getByRole("menuitem", { name: /cancel change/i })).toBeInTheDocument();
    expect(screen.queryByText(/roll back/i)).not.toBeInTheDocument();
    cleanup();
    const { container } = renderBar({ state: "rollback", legalNextStates: [] });
    expect(container).toBeEmptyDOMElement();
  });
});

/**
 * "Re-schedule": `authorize` from Customer Approval -- the planned time changed,
 * so the change goes back through internal approval. A secondary (outlined)
 * button next to the primary move; offered from `customer_approval` only.
 */
describe("ChangeRequestActionBar — Re-schedule", () => {
  it("is an outlined button beside 'Change state', which holds the bypass and Cancel", () => {
    const { onAction } = renderBar({
      state: "customer_approval",
      legalNextStates: ["scheduled", "authorize", "canceled"],
    });
    const contained = screen.getAllByRole("button").filter((b) => b.className.includes("MuiButton-contained"));
    expect(contained).toHaveLength(1);
    expect(contained[0]).toHaveTextContent("Change state");
    const reschedule = screen.getByRole("button", { name: "Re-schedule" });
    expect(reschedule.className).toContain("MuiButton-outlined");
    openMenu();
    expect(screen.getAllByRole("menuitem").map((i) => i.textContent)).toEqual([
      "Bypass customer approval",
      "Cancel change",
    ]);
    fireEvent.keyDown(screen.getByRole("menu"), { key: "Escape" });
    fireEvent.click(reschedule);
    expect(onAction).toHaveBeenCalledWith("authorize");
  });

  it("stays on offer while a customer group's approval is pending (Cancel is the only other action)", () => {
    renderBar({ state: "customer_approval", legalNextStates: ["authorize", "canceled"] });
    expect(screen.getByRole("button", { name: "Re-schedule" })).toBeInTheDocument();
    expect(screen.queryByText(/bypass customer approval/i)).not.toBeInTheDocument();
    openMenu();
    expect(screen.getAllByRole("menuitem").map((i) => i.textContent)).toEqual(["Cancel change"]);
  });

  it("is disabled while a transition is in flight", () => {
    renderBar({ state: "customer_approval", legalNextStates: ["authorize", "canceled"] }, { isPending: true });
    expect(screen.getByRole("button", { name: "Re-schedule" })).toBeDisabled();
  });

  it("is never offered from any other state, even if the backend listed authorize", () => {
    for (const state of [
      "new", "assess", "authorize", "scheduled", "implement", "review", "customer_review",
      "closed", "canceled", "rollback",
    ]) {
      cleanup();
      renderBar({ state, legalNextStates: ["authorize"] });
      expect(screen.queryByRole("button", { name: /re-schedule/i }), state).not.toBeInTheDocument();
      expect(screen.queryByRole("button", { name: /authorize/i }), state).not.toBeInTheDocument();
    }
  });
});

describe("ChangeRequestActionBar — the Change state button", () => {
  it("says whether its menu is open, and which element the menu is", () => {
    renderBar({ state: "customer_approval", legalNextStates: ["scheduled", "authorize", "canceled"] });
    const button = screen.getByRole("button", { name: /change state/i });
    expect(button).toHaveAttribute("aria-haspopup", "menu");
    expect(button).toHaveAttribute("aria-expanded", "false");
    expect(button).not.toHaveAttribute("aria-controls");
    fireEvent.click(button);
    expect(button).toHaveAttribute("aria-expanded", "true");
    const menuId = button.getAttribute("aria-controls");
    expect(menuId).toBeTruthy();
    expect(document.getElementById(menuId!)).toBe(screen.getByRole("menu"));
    fireEvent.keyDown(screen.getByRole("menu"), { key: "Escape" });
  });
});
