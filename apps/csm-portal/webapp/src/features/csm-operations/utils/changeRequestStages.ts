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

import type { BeChangeRequestApproval, BeChangeRequestState } from "@api/backend/types";
import {
  approvalStageLabel,
  changeRequestStateLabel,
  isChangeRequestOffRampState,
} from "@features/csm-operations/utils/changeRequests";

/**
 * How one stage of the change-request workflow reads for the record in front
 * of the engineer.
 *
 * - `done`: the change passed through this stage.
 * - `current`: the change is in this state right now.
 * - `pending`: still ahead on the path the change is travelling.
 * - `not-taken`: not part of the path this change took. Rollback and Canceled
 *   are exceptions that most changes never take, so they sit here until (and
 *   unless) the change actually ends in them; Closed and the customer review
 *   are `not-taken` on a change that ended in an exception without them.
 * - `unrecorded`: the stage is on the path, but the record cannot say whether
 *   the change passed through it (only ever on a canceled change: see
 *   {@link buildChangeRequestLifecycle}). Distinct from `pending` (which would
 *   claim the stage is still to come) and from `not-taken` (which would claim
 *   it was skipped).
 */
export type ChangeRequestLifecycleStatus =
  | "done"
  | "current"
  | "pending"
  | "not-taken"
  | "unrecorded";

export interface ChangeRequestLifecycleNode {
  key: BeChangeRequestState;
  /** The state's name, e.g. "Customer Approval". */
  label: string;
  /** What reaching the stage means, in the customer portal's own words. */
  caption: string;
  status: ChangeRequestLifecycleStatus;
}

/**
 * The stages in the order the customer portal's "Change Request Workflow"
 * plots them (`CHANGE_REQUEST_STATE_ORDER` there): the two exceptions, Rollback
 * and Canceled, sit among the forward stages -- Rollback before Closed,
 * Canceled last -- rather than apart from them.
 */
export const CHANGE_REQUEST_LIFECYCLE_ORDER: readonly BeChangeRequestState[] = [
  "new",
  "assess",
  "authorize",
  "customer_approval",
  "scheduled",
  "implement",
  "review",
  "customer_review",
  "rollback",
  "closed",
  "canceled",
];

/**
 * The path a change that goes to plan travels: the plotted order without the
 * two exceptions, so `closed` -- plotted after `rollback` -- is the last step.
 */
const HAPPY_PATH: readonly BeChangeRequestState[] = CHANGE_REQUEST_LIFECYCLE_ORDER.filter(
  (s) => s !== "rollback" && s !== "canceled",
);

/** The customer portal's captions, verbatim (`buildChangeRequestWorkflowStages` there). */
export const CHANGE_REQUEST_STAGE_CAPTIONS: Record<BeChangeRequestState, string> = {
  new: "Change request created",
  assess: "Technical assessment completed",
  authorize: "Internal authorization obtained",
  customer_approval: "Customer approval received",
  scheduled: "Maintenance window scheduled",
  implement: "Change implementation",
  review: "Internal review",
  customer_review: "Customer validation",
  rollback: "Change rollback if needed",
  closed: "Change request completed",
  canceled: "Change request canceled",
};

/** True when `state` is one of the eleven states of the workflow. */
export function isChangeRequestLifecycleState(state?: string | null): state is BeChangeRequestState {
  return !!state && (CHANGE_REQUEST_LIFECYCLE_ORDER as readonly string[]).includes(state);
}

/**
 * The state each approval stage belongs to -- the one it is provisioned on
 * entering and can be decided in (the entity service's
 * `approvalStageDecidableState`). Keyed by {@link approvalStageLabel}, so the
 * legacy ServiceNow-style names ("Assess", "Authorize") read the same.
 */
const APPROVAL_STAGE_STATE: Record<string, BeChangeRequestState> = {
  "Peer Approval": "assess",
  "CAB Approval": "authorize",
  "ECAB Approval": "authorize",
  Review: "review",
  "Customer Approval": "customer_approval",
  "Customer Review": "customer_review",
};

type StageEvidence = Pick<BeChangeRequestApproval, "stage" | "status">;

function stageState(row: StageEvidence): BeChangeRequestState | undefined {
  return APPROVAL_STAGE_STATE[approvalStageLabel(row.stage)];
}

/** Whether a Customer Review stage was ever provisioned for the change. */
function customerReviewHappened(approvals?: readonly StageEvidence[]): boolean {
  return !!approvals?.some((row) => stageState(row) === "customer_review");
}

/**
 * How far along {@link HAPPY_PATH} the approval stages PROVE a change got, as
 * an index (-1: nothing is proven). Used only for a canceled change, whose
 * record keeps no history of where it was when it was canceled.
 *
 * Two facts, both guaranteed by the entity service, count as proof:
 *  - a stage row exists for state S: the backend provisions a stage only as
 *    part of moving the change INTO S, so every state before S was passed
 *    (the change may have been canceled while in S itself, so S is not);
 *  - the stage for S is APPROVED: approving it is what moves the change out of
 *    S, so S was passed too.
 * Nothing else is inferred: a stage that is still pending, rejected or
 * cancelled proves only that it was reached, a change with no stage rows at all
 * (a Standard change, a project without registered customer contacts, a change
 * canceled at New) proves nothing, and the absence of a row proves nothing
 * either. A re-scheduled change that is canceled back at Authorize still
 * counts its first pass through Customer Approval as passed -- it was.
 */
function provenPassedIndex(approvals?: readonly StageEvidence[]): number {
  let passed = -1;
  for (const row of approvals ?? []) {
    const s = stageState(row);
    if (!s) continue;
    const at = HAPPY_PATH.indexOf(s);
    passed = Math.max(passed, at - 1);
    if (row.status.trim().toUpperCase() === "APPROVED") passed = Math.max(passed, at);
  }
  return passed;
}

export interface BuildChangeRequestLifecycleInput {
  state?: string | null;
  /** The change's "Customer Approval" checkbox; `undefined` = unknown. */
  customerApprovalRequired?: boolean;
  /** The change's "Customer Review" checkbox; `undefined` = unknown. */
  customerReviewRequired?: boolean;
  /** `GET /change-requests/{id}/approvals`, when loaded. Only read for a rollback or canceled change. */
  approvals?: readonly StageEvidence[];
}

/**
 * The change request's workflow as eleven stages in the customer portal's
 * order and wording, each with a status for THIS change.
 *
 * Which stages are on the line:
 *  - Customer Approval / Customer Review are optional (the change's two
 *    checkboxes). A stage whose checkbox is explicitly `false` is left off
 *    unless the change is in that very state; an unknown (`undefined`) flag
 *    keeps the stage.
 *  - Rollback and Canceled are always on the line.
 *
 * Statuses by the change's state (a stage left off the line has no row):
 *
 * | state                      | forward stages                    | Rollback  | Closed    | Canceled  |
 * |----------------------------|-----------------------------------|-----------|-----------|-----------|
 * | none / unrecognized        | all pending                       | not-taken | pending   | not-taken |
 * | new ... customer_review    | before = done, it = current, after = pending | not-taken | pending | not-taken |
 * | closed                     | all done                          | not-taken | current   | not-taken |
 * | rollback                   | through Review done; Customer Review done only if a Customer Review stage row exists, else not-taken | current | not-taken | not-taken |
 * | canceled                   | done through what the approvals prove (below), else unrecorded | not-taken | not-taken | current |
 *
 * Rollback is only reachable from Review and Customer Review (the backend
 * refuses it anywhere else), so a rolled-back change certainly passed every
 * stage through Review; Customer Review is the one stage that could go either
 * way, and only a Customer Review stage row says it was entered. (The one case
 * this cannot tell apart: a change rolled back from Customer Review in a
 * project with no registered customer contacts leaves no such row, and reads as
 * not taken.) While the approvals are not available (`approvals` is
 * `undefined`: still loading, or the request failed) that stage is `unrecorded`
 * rather than a guess, and so is every stage of a canceled change.
 *
 * Canceled can be reached from every non-terminal state and the record keeps
 * no history of which, so a canceled change marks a stage `done` only when the
 * approvals PROVE it was passed (see {@link provenPassedIndex}) and shows every
 * other stage on the path as `unrecorded`, never as pending or skipped. Closed
 * and Rollback were certainly not taken.
 */
export function buildChangeRequestLifecycle({
  state,
  customerApprovalRequired,
  customerReviewRequired,
  approvals,
}: BuildChangeRequestLifecycleInput): ChangeRequestLifecycleNode[] {
  const onLine = (s: BeChangeRequestState): boolean => {
    if (s === "customer_approval") return customerApprovalRequired !== false || state === s;
    if (s === "customer_review") return customerReviewRequired !== false || state === s;
    return true;
  };

  const statusOf = (s: BeChangeRequestState): ChangeRequestLifecycleStatus => {
    const exception = isChangeRequestOffRampState(s);
    if (state === "rollback" || state === "canceled") {
      if (s === state) return "current";
      if (exception || s === "closed") return "not-taken";
      if (state === "rollback") {
        if (s !== "customer_review") return "done";
        // Approvals not loaded (yet, or the request failed): cannot say.
        if (!approvals) return "unrecorded";
        return customerReviewHappened(approvals) ? "done" : "not-taken";
      }
      return HAPPY_PATH.indexOf(s) <= provenPassedIndex(approvals) ? "done" : "unrecorded";
    }
    if (exception) return "not-taken";
    const at = isChangeRequestLifecycleState(state) ? HAPPY_PATH.indexOf(state) : -1;
    if (at < 0) return "pending";
    const own = HAPPY_PATH.indexOf(s);
    return own < at ? "done" : own === at ? "current" : "pending";
  };

  return CHANGE_REQUEST_LIFECYCLE_ORDER.filter(onLine).map((key) => ({
    key,
    label: changeRequestStateLabel(key),
    caption: CHANGE_REQUEST_STAGE_CAPTIONS[key],
    status: statusOf(key),
  }));
}

/** The status as words, for the node's accessible name (the stepper never conveys it by colour alone). */
export function changeRequestLifecycleStatusText(
  status: ChangeRequestLifecycleStatus,
): string {
  switch (status) {
    case "done":
      return "done";
    case "current":
      return "current";
    case "pending":
      return "upcoming";
    case "not-taken":
      return "not taken";
    case "unrecorded":
      return "history not recorded";
  }
}
