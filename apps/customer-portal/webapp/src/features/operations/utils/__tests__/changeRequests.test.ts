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

import { describe, expect, it } from "vitest";
import {
  changeRequestToApiDatetime,
  changeRequestToDatetimeLocal,
  formatChangeRequestDuration,
  formatDuration,
  getChangeRequestDecisionMode,
  describeChangeRequestActionError,
  getCustomerDecisionLabels,
  getCustomerDecisionMessages,
  getCustomerRejectConfirmCopy,
  buildChangeRequestWorkflowStages,
  mapChangeRequestStats,
  resolveCustomerDecisionMode,
  stripChangeRequestCustomTags,
  sumChangeRequestStateCount,
  AWAITING_YOUR_ACTION_STATE_IDS,
  AWAITING_LABELS,
  CHANGE_REQUEST_ANSWER_STALE_MESSAGE,
  CHANGE_REQUEST_NOT_A_CONTACT_MESSAGE,
  CHANGE_REQUEST_ON_HOLD_MESSAGE,
} from "@features/operations/utils/changeRequests";
import { ChangeRequestDecisionMode } from "@features/operations/types/changeRequests";
import { ApiError } from "@utils/ApiError";

describe("changeRequests utils", () => {
  it("sumChangeRequestStateCount sums by id and label", () => {
    const total = sumChangeRequestStateCount(
      [
        { id: "5", label: "Customer Approval", count: 2 },
        { id: "", label: "Customer Review", count: 1 },
        { id: "9", label: "Other", count: 99 },
      ],
      AWAITING_YOUR_ACTION_STATE_IDS,
      AWAITING_LABELS,
    );
    expect(total).toBe(3);
  });

  it("mapChangeRequestStats maps API payload to card stats", () => {
    const stats = mapChangeRequestStats({
      stateCount: [{ id: "5", label: "Customer Approval", count: 4 }],
      totalCount: 10,
      resolvedCount: { total: 0, currentMonth: 0, pastThirtyDays: 0 },
    });
    expect(stats.awaitingYourAction).toBe(4);
    expect(stats.totalRequests).toBe(10);
  });

  it("formatChangeRequestDuration formats minutes", () => {
    expect(formatChangeRequestDuration(90)).toBe("1 hour 30 minutes");
    expect(formatChangeRequestDuration(45)).toBe("45 minutes");
  });

  it("formatDuration handles hour and minute segments", () => {
    expect(formatDuration(90)).toBe("1h 30m");
  });

  it("changeRequest datetime helpers round-trip wall time", () => {
    const api = changeRequestToApiDatetime("2026-06-01T10:30");
    expect(api).toMatch(/2026-06-01 10:30:00/);
    expect(changeRequestToDatetimeLocal(api)).toBe("2026-06-01T10:30");
  });

  it("stripChangeRequestCustomTags removes custom tags", () => {
    expect(stripChangeRequestCustomTags("[code]hello[/code]")).toBe("hello");
  });

  it("getChangeRequestDecisionMode detects customer approval state", () => {
    expect(
      getChangeRequestDecisionMode({
        state: { id: "5", label: "Customer Approval" },
      } as never),
    ).toBe(ChangeRequestDecisionMode.CUSTOMER_APPROVAL);
  });
});

describe("resolveCustomerDecisionMode", () => {
  const { CUSTOMER_APPROVAL, CUSTOMER_REVIEW, NONE } = ChangeRequestDecisionMode;
  const approval = { id: "5", label: "Customer Approval" };
  const review = { id: "1", label: "Customer Review" };

  // state x customerCanAnswer x hasCustomerApproved -> what the customer is offered
  const table: Array<{
    name: string;
    state: { id: string; label: string };
    customerCanAnswer: boolean | undefined;
    hasCustomerApproved: boolean | undefined;
    expected: ChangeRequestDecisionMode;
  }> = [
    { name: "approval, can answer, stamp unset", state: approval, customerCanAnswer: true, hasCustomerApproved: false, expected: CUSTOMER_APPROVAL },
    { name: "approval, can answer, stamp set", state: approval, customerCanAnswer: true, hasCustomerApproved: true, expected: CUSTOMER_APPROVAL },
    { name: "approval, cannot answer, stamp set", state: approval, customerCanAnswer: false, hasCustomerApproved: true, expected: NONE },
    { name: "approval, cannot answer, stamp unset", state: approval, customerCanAnswer: false, hasCustomerApproved: false, expected: NONE },
    { name: "approval, unknown, stamp set (legacy gate)", state: approval, customerCanAnswer: undefined, hasCustomerApproved: true, expected: CUSTOMER_APPROVAL },
    { name: "approval, unknown, stamp unset (legacy gate)", state: approval, customerCanAnswer: undefined, hasCustomerApproved: false, expected: NONE },
    { name: "approval, unknown, stamp missing", state: approval, customerCanAnswer: undefined, hasCustomerApproved: undefined, expected: NONE },
    { name: "review, can answer", state: review, customerCanAnswer: true, hasCustomerApproved: false, expected: CUSTOMER_REVIEW },
    { name: "review, unknown (no gate)", state: review, customerCanAnswer: undefined, hasCustomerApproved: false, expected: CUSTOMER_REVIEW },
    { name: "review, cannot answer", state: review, customerCanAnswer: false, hasCustomerApproved: true, expected: NONE },
    { name: "scheduled, can answer", state: { id: "-2", label: "Scheduled" }, customerCanAnswer: true, hasCustomerApproved: true, expected: NONE },
    { name: "authorize, can answer", state: { id: "-3", label: "Authorize" }, customerCanAnswer: true, hasCustomerApproved: true, expected: NONE },
    { name: "closed, unknown", state: { id: "3", label: "Closed" }, customerCanAnswer: undefined, hasCustomerApproved: true, expected: NONE },
  ];

  it.each(table)("$name -> $expected", ({ state, customerCanAnswer, hasCustomerApproved, expected }) => {
    expect(
      resolveCustomerDecisionMode({
        state,
        customerCanAnswer,
        hasCustomerApproved,
      } as never),
    ).toBe(expected);
  });

  it("recognises the state by label when the id is missing", () => {
    expect(
      resolveCustomerDecisionMode({
        state: { label: "Customer Approval" },
        customerCanAnswer: true,
      } as never),
    ).toBe(CUSTOMER_APPROVAL);
  });

  it("treats a null customerCanAnswer (not a boolean) as unknown", () => {
    expect(
      resolveCustomerDecisionMode({
        state: approval,
        customerCanAnswer: null,
        hasCustomerApproved: true,
      } as never),
    ).toBe(CUSTOMER_APPROVAL);
  });

  it("offers nothing without a change request or a state", () => {
    expect(resolveCustomerDecisionMode(null)).toBe(NONE);
    expect(resolveCustomerDecisionMode(undefined)).toBe(NONE);
    expect(resolveCustomerDecisionMode({ customerCanAnswer: true } as never)).toBe(NONE);
  });
});

describe("customer decision copy", () => {
  it("labels the buttons for each stage", () => {
    expect(getCustomerDecisionLabels(ChangeRequestDecisionMode.CUSTOMER_APPROVAL)).toEqual({
      approve: "Approve",
      reject: "Reject",
    });
    expect(getCustomerDecisionLabels(ChangeRequestDecisionMode.CUSTOMER_REVIEW)).toEqual({
      approve: "Successful",
      reject: "Unsuccessful",
    });
  });

  it("states the consequence of rejecting or failing a review", () => {
    expect(getCustomerRejectConfirmCopy(ChangeRequestDecisionMode.CUSTOMER_APPROVAL).message).toBe(
      "Rejecting cancels this change request.",
    );
    expect(getCustomerRejectConfirmCopy(ChangeRequestDecisionMode.CUSTOMER_REVIEW).message).toBe(
      "Marking it unsuccessful sends the change into rollback.",
    );
  });

  it("says what happened to the change request after an answer", () => {
    const approval = ChangeRequestDecisionMode.CUSTOMER_APPROVAL;
    const review = ChangeRequestDecisionMode.CUSTOMER_REVIEW;
    expect(getCustomerDecisionMessages(approval, true).success).toBe(
      "Change request approved. It is now scheduled.",
    );
    expect(getCustomerDecisionMessages(approval, false).success).toBe(
      "Change request rejected. It has been canceled.",
    );
    expect(getCustomerDecisionMessages(review, true).success).toBe(
      "Change request marked as successful. It is now closed.",
    );
    expect(getCustomerDecisionMessages(review, false).success).toBe(
      "Change request marked as unsuccessful. It is now in rollback.",
    );
  });
});

describe("describeChangeRequestActionError", () => {
  const fallback = "Could not do it. Please try again.";

  it("explains a conflict as an answer that is no longer wanted", () => {
    expect(
      describeChangeRequestActionError(new ApiError(409, "Conflict", "stale approval: whatever"), fallback),
    ).toEqual({ message: CHANGE_REQUEST_ANSWER_STALE_MESSAGE, terminal: true });
    expect(CHANGE_REQUEST_ANSWER_STALE_MESSAGE).toBe(
      "This request was already answered or is no longer waiting for your answer.",
    );
  });

  it("says so when a proposal is refused because the change is on hold, and keeps the customer where they are", () => {
    expect(
      describeChangeRequestActionError(
        new ApiError(409, "Conflict", "this change request is on hold, so a new implementation time cannot be proposed now"),
        fallback,
      ),
    ).toEqual({ message: CHANGE_REQUEST_ON_HOLD_MESSAGE, terminal: false });
  });

  it("explains a refusal as not being a contact who can answer", () => {
    expect(
      describeChangeRequestActionError(new ApiError(403, "Forbidden", "You do not have permission."), fallback),
    ).toEqual({ message: CHANGE_REQUEST_NOT_A_CONTACT_MESSAGE, terminal: true });
    expect(CHANGE_REQUEST_NOT_A_CONTACT_MESSAGE).toBe(
      "You are not one of the contacts who can answer this change request.",
    );
  });

  it("shows the backend's message for a bad request, in plainer words for the window ones", () => {
    expect(
      describeChangeRequestActionError(new ApiError(400, "Bad Request", "plannedStartOn must follow the format: YYYY-MM-DD HH:mm:ss"), fallback),
    ).toEqual({
      message: "plannedStartOn must follow the format: YYYY-MM-DD HH:mm:ss",
      terminal: false,
    });
    expect(
      describeChangeRequestActionError(new ApiError(400, "Bad Request", "the planned start must not be after the planned end"), fallback).message,
    ).toBe("The proposed end must be after the proposed start.");
    expect(
      describeChangeRequestActionError(
        new ApiError(400, "Bad Request", "re-scheduling requires a changed planned start or end: send plannedStartOn and/or plannedEndOn with a value different from the stored one"),
        fallback,
      ).message,
    ).toMatch(/same as the current schedule/);
  });

  it("explains an answer that was already locked in", () => {
    expect(
      describeChangeRequestActionError(
        new ApiError(400, "Bad Request", "isCustomerApproved is locked once set to true and cannot be reverted to false"),
        fallback,
      ).message,
    ).toBe("This answer has already been given and cannot be changed.");
  });

  it("shows the backend's message for other failures, or the fallback when it has none", () => {
    expect(
      describeChangeRequestActionError(new ApiError(500, "Internal Server Error", "Failed to update change request."), fallback),
    ).toEqual({ message: "Failed to update change request.", terminal: false });
    expect(
      describeChangeRequestActionError(new ApiError(500, "Internal Server Error", "Internal Server Error"), fallback).message,
    ).toBe(fallback);
    expect(
      describeChangeRequestActionError(new ApiError(502, "", "HTTP 502"), fallback).message,
    ).toBe(fallback);
  });

  it("uses the fallback for anything that is not an API error", () => {
    expect(describeChangeRequestActionError(new Error("Failed to fetch"), fallback)).toEqual({
      message: fallback,
      terminal: false,
    });
    expect(describeChangeRequestActionError("boom", fallback).message).toBe(fallback);
  });
});

describe("buildChangeRequestWorkflowStages", () => {
  it("shows Authorize as current for the raw state a customer's proposal leaves the change in", () => {
    const { workflowStages, currentStateIndex } = buildChangeRequestWorkflowStages({
      state: { label: "authorize" },
      hasCustomerApproved: false,
      hasCustomerReviewed: false,
    } as never);
    expect(workflowStages.find((s) => s.current)?.name).toBe("Authorize");
    expect(currentStateIndex).toBe(2);
    expect(workflowStages.filter((s) => s.current)).toHaveLength(1);
    expect(workflowStages[0]).toMatchObject({ name: "New", completed: true, current: false });
  });

  it("still marks Customer Approval current, with the old gate's stamp untouched", () => {
    const { workflowStages } = buildChangeRequestWorkflowStages({
      state: { id: "5", label: "Customer Approval" },
      hasCustomerApproved: false,
      hasCustomerReviewed: false,
    } as never);
    expect(workflowStages.find((s) => s.current)?.name).toBe("Customer Approval");
  });
});
