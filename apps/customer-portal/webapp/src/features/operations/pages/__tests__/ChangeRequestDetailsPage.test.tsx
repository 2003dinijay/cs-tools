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

import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import ChangeRequestDetailsPage from "@features/operations/pages/ChangeRequestDetailsPage";
import {
  CHANGE_REQUEST_ANSWER_STALE_MESSAGE,
  CHANGE_REQUEST_NOT_A_CONTACT_MESSAGE,
} from "@features/operations/utils/changeRequests";
import { ApiError } from "@utils/ApiError";

const mocks = vi.hoisted(() => ({
  changeRequest: { value: null as Record<string, unknown> | null },
  mutateAsync: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
  isPending: { value: false },
}));

vi.mock("@features/operations/api/useGetChangeRequestDetails", () => ({
  default: () => ({
    data: mocks.changeRequest.value,
    isLoading: false,
    isFetching: false,
    isError: false,
  }),
}));

vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: mocks.showError }),
}));

vi.mock("@context/success-banner/SuccessBannerContext", () => ({
  useSuccessBanner: () => ({ showSuccess: mocks.showSuccess }),
}));

vi.mock("@features/operations/api/usePatchChangeRequest", () => ({
  usePatchChangeRequest: () => ({
    mutateAsync: mocks.mutateAsync,
    isPending: mocks.isPending.value,
  }),
}));

const STATES = {
  approval: { id: "5", label: "Customer Approval" },
  review: { id: "1", label: "Customer Review" },
  scheduled: { id: "-2", label: "Scheduled" },
  authorize: { id: "-3", label: "Authorize" },
};

function makeChangeRequest(overrides: Record<string, unknown> = {}) {
  return {
    id: "cr-1",
    number: "CHG001",
    title: "Deploy patch",
    state: STATES.scheduled,
    type: { id: "normal", label: "Normal" },
    project: { id: "p1", label: "Proj", number: null },
    case: null,
    deployment: null,
    deployedProduct: null,
    product: null,
    assignedEngineer: null,
    assignedTeam: null,
    startDate: "2026-06-10 04:30:00",
    endDate: "2026-06-10 06:30:00",
    createdOn: "2026-01-01",
    updatedOn: "2026-01-02",
    hasCustomerApproved: false,
    hasCustomerReviewed: false,
    ...overrides,
  };
}

function renderPage() {
  return render(
    <MemoryRouter initialEntries={["/projects/p1/operations/change-requests/cr-1"]}>
      <Routes>
        <Route
          path="/projects/:projectId/operations/change-requests/:changeRequestId"
          element={<ChangeRequestDetailsPage />}
        />
      </Routes>
    </MemoryRouter>,
  );
}

const button = (name: string) => screen.queryByRole("button", { name });

describe("ChangeRequestDetailsPage", () => {
  beforeEach(() => {
    mocks.changeRequest.value = makeChangeRequest();
    mocks.mutateAsync.mockReset();
    mocks.mutateAsync.mockResolvedValue({ id: "cr-1" });
    mocks.showError.mockReset();
    mocks.showSuccess.mockReset();
    mocks.isPending.value = false;
  });

  it("renders change request number when loaded", () => {
    renderPage();
    expect(screen.getByText("CHG001")).toBeInTheDocument();
  });

  describe("which buttons the customer gets", () => {
    type Case = {
      name: string;
      state: { id: string; label: string };
      customerCanAnswer?: boolean;
      hasCustomerApproved?: boolean;
      buttons: string[];
    };
    const approvalButtons = ["Propose New Time", "Approve", "Reject"];
    const reviewButtons = ["Successful", "Unsuccessful"];
    const cases: Case[] = [
      { name: "Customer Approval, customerCanAnswer true, stamp unset", state: STATES.approval, customerCanAnswer: true, hasCustomerApproved: false, buttons: approvalButtons },
      { name: "Customer Approval, customerCanAnswer false, stamp set", state: STATES.approval, customerCanAnswer: false, hasCustomerApproved: true, buttons: [] },
      { name: "Customer Approval, customerCanAnswer absent, stamp set (legacy gate)", state: STATES.approval, hasCustomerApproved: true, buttons: approvalButtons },
      { name: "Customer Approval, customerCanAnswer absent, stamp unset", state: STATES.approval, hasCustomerApproved: false, buttons: [] },
      { name: "Customer Review, customerCanAnswer true", state: STATES.review, customerCanAnswer: true, buttons: reviewButtons },
      { name: "Customer Review, customerCanAnswer absent", state: STATES.review, buttons: reviewButtons },
      { name: "Customer Review, customerCanAnswer false", state: STATES.review, customerCanAnswer: false, buttons: [] },
      { name: "Scheduled, customerCanAnswer true", state: STATES.scheduled, customerCanAnswer: true, hasCustomerApproved: true, buttons: [] },
      { name: "Authorize (after a proposal), customerCanAnswer true", state: STATES.authorize, customerCanAnswer: true, buttons: [] },
    ];
    const all = [...approvalButtons, ...reviewButtons];

    it.each(cases)("$name", ({ state, customerCanAnswer, hasCustomerApproved, buttons }) => {
      mocks.changeRequest.value = makeChangeRequest({
        state,
        ...(customerCanAnswer === undefined ? {} : { customerCanAnswer }),
        ...(hasCustomerApproved === undefined ? {} : { hasCustomerApproved }),
      });
      renderPage();
      for (const name of all) {
        if (buttons.includes(name)) expect(button(name), name).toBeInTheDocument();
        else expect(button(name), name).not.toBeInTheDocument();
      }
    });
  });

  describe("approving", () => {
    beforeEach(() => {
      mocks.changeRequest.value = makeChangeRequest({ state: STATES.approval, customerCanAnswer: true });
    });

    it("approves in one click, without a confirmation, and says what happened", async () => {
      renderPage();
      fireEvent.click(screen.getByRole("button", { name: "Approve" }));
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
      await waitFor(() => expect(mocks.showSuccess).toHaveBeenCalledTimes(1));
      expect(mocks.mutateAsync).toHaveBeenCalledWith({ isCustomerApproved: true });
      expect(mocks.showSuccess).toHaveBeenCalledWith("Change request approved. It is now scheduled.");
      expect(mocks.showError).not.toHaveBeenCalled();
    });

    it("sends only one request however often it is clicked while it is in flight", async () => {
      let resolve: (v: unknown) => void = () => {};
      mocks.mutateAsync.mockReturnValueOnce(new Promise((r) => { resolve = r; }));
      renderPage();
      const approve = screen.getByRole("button", { name: "Approve" });
      fireEvent.click(approve);
      fireEvent.click(approve);
      fireEvent.click(approve);
      expect(mocks.mutateAsync).toHaveBeenCalledTimes(1);
      await act(async () => { resolve({ id: "cr-1" }); });
      await waitFor(() => expect(mocks.showSuccess).toHaveBeenCalledTimes(1));
    });

    it("disables every answer button while a request is pending", () => {
      mocks.isPending.value = true;
      renderPage();
      for (const name of ["Propose New Time", "Approve", "Reject"]) {
        expect(screen.getByRole("button", { name })).toBeDisabled();
      }
    });

    it.each([
      [new ApiError(409, "Conflict", "stale"), CHANGE_REQUEST_ANSWER_STALE_MESSAGE],
      [new ApiError(403, "Forbidden", "nope"), CHANGE_REQUEST_NOT_A_CONTACT_MESSAGE],
      [new ApiError(500, "Internal Server Error", "Failed to update change request."), "Failed to update change request."],
      [new Error("Failed to fetch"), "Could not approve the change request. Please try again."],
    ])("shows a readable message when approving fails (%#)", async (error, message) => {
      mocks.mutateAsync.mockRejectedValueOnce(error);
      renderPage();
      fireEvent.click(screen.getByRole("button", { name: "Approve" }));
      await waitFor(() => expect(mocks.showError).toHaveBeenCalledWith(message));
      expect(mocks.showSuccess).not.toHaveBeenCalled();
    });
  });

  describe("rejecting", () => {
    beforeEach(() => {
      mocks.changeRequest.value = makeChangeRequest({ state: STATES.approval, customerCanAnswer: true });
    });

    it("asks first, in plain words, and sends nothing until confirmed", () => {
      renderPage();
      fireEvent.click(screen.getByRole("button", { name: "Reject" }));
      const dialog = screen.getByRole("dialog", { name: "Reject this change request?" });
      expect(within(dialog).getByText("Rejecting cancels this change request.")).toBeInTheDocument();
      expect(mocks.mutateAsync).not.toHaveBeenCalled();
    });

    it("sends nothing when the customer goes back", async () => {
      renderPage();
      fireEvent.click(screen.getByRole("button", { name: "Reject" }));
      fireEvent.click(screen.getByRole("button", { name: "Go back" }));
      await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
      expect(mocks.mutateAsync).not.toHaveBeenCalled();
    });

    it("sends nothing when the customer presses Escape", async () => {
      renderPage();
      fireEvent.click(screen.getByRole("button", { name: "Reject" }));
      fireEvent.keyDown(screen.getByRole("dialog"), { key: "Escape" });
      await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
      expect(mocks.mutateAsync).not.toHaveBeenCalled();
    });

    it("rejects once confirmed, closes the dialog and says it was canceled", async () => {
      renderPage();
      fireEvent.click(screen.getByRole("button", { name: "Reject" }));
      fireEvent.click(screen.getByRole("button", { name: "Reject change request" }));
      await waitFor(() => expect(mocks.showSuccess).toHaveBeenCalledTimes(1));
      expect(mocks.mutateAsync).toHaveBeenCalledTimes(1);
      expect(mocks.mutateAsync).toHaveBeenCalledWith({ isCustomerApproved: false });
      expect(mocks.showSuccess).toHaveBeenCalledWith("Change request rejected. It has been canceled.");
      await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    });

    it("closes the dialog and shows why when the rejection is refused", async () => {
      mocks.mutateAsync.mockRejectedValueOnce(new ApiError(409, "Conflict", "stale"));
      renderPage();
      fireEvent.click(screen.getByRole("button", { name: "Reject" }));
      fireEvent.click(screen.getByRole("button", { name: "Reject change request" }));
      await waitFor(() => expect(mocks.showError).toHaveBeenCalledWith(CHANGE_REQUEST_ANSWER_STALE_MESSAGE));
      await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
      expect(mocks.showSuccess).not.toHaveBeenCalled();
    });
  });

  describe("the review", () => {
    beforeEach(() => {
      mocks.changeRequest.value = makeChangeRequest({ state: STATES.review });
    });

    it("confirms the change in one click", async () => {
      renderPage();
      fireEvent.click(screen.getByRole("button", { name: "Successful" }));
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
      await waitFor(() => expect(mocks.showSuccess).toHaveBeenCalledTimes(1));
      expect(mocks.mutateAsync).toHaveBeenCalledWith({ isCustomerReviewed: true });
      expect(mocks.showSuccess).toHaveBeenCalledWith("Change request marked as successful. It is now closed.");
    });

    it("asks before marking it unsuccessful, because that sends it into rollback", async () => {
      renderPage();
      fireEvent.click(screen.getByRole("button", { name: "Unsuccessful" }));
      const dialog = screen.getByRole("dialog", { name: "Mark this change as unsuccessful?" });
      expect(
        within(dialog).getByText("Marking it unsuccessful sends the change into rollback."),
      ).toBeInTheDocument();
      expect(mocks.mutateAsync).not.toHaveBeenCalled();

      fireEvent.click(screen.getByRole("button", { name: "Mark unsuccessful" }));
      await waitFor(() => expect(mocks.showSuccess).toHaveBeenCalledTimes(1));
      expect(mocks.mutateAsync).toHaveBeenCalledWith({ isCustomerReviewed: false });
      expect(mocks.showSuccess).toHaveBeenCalledWith("Change request marked as unsuccessful. It is now in rollback.");
    });
  });

  describe("proposing a new time", () => {
    it("opens the dialog from the header button", () => {
      mocks.changeRequest.value = makeChangeRequest({ state: STATES.approval, customerCanAnswer: true });
      renderPage();
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
      fireEvent.click(screen.getByRole("button", { name: "Propose New Time" }));
      expect(screen.getByRole("dialog", { name: "Propose New Implementation Time" })).toBeInTheDocument();
    });

    it("closes the dialog by itself once the change request no longer waits on the customer", () => {
      mocks.changeRequest.value = makeChangeRequest({ state: STATES.approval, customerCanAnswer: true });
      const view = renderPage();
      fireEvent.click(screen.getByRole("button", { name: "Propose New Time" }));
      expect(screen.getByRole("dialog")).toBeInTheDocument();

      // What the refetch after a proposal brings back: Authorize.
      mocks.changeRequest.value = makeChangeRequest({ state: STATES.authorize, customerCanAnswer: false });
      view.rerender(
        <MemoryRouter initialEntries={["/projects/p1/operations/change-requests/cr-1"]}>
          <Routes>
            <Route
              path="/projects/:projectId/operations/change-requests/:changeRequestId"
              element={<ChangeRequestDetailsPage />}
            />
          </Routes>
        </MemoryRouter>,
      );
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
      expect(button("Propose New Time")).not.toBeInTheDocument();
      expect(button("Approve")).not.toBeInTheDocument();
    });
  });
});
