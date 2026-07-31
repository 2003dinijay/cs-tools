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

import type {
  BeChangeRequestDetail,
  BeChangeRequestImpact,
  BeChangeRequestState,
  BeChangeRequestType,
} from "@api/backend/types";
import { isBlankHtml, sanitizeRichTextHtml } from "@utils/sanitizeHtml";

type ChipColor = "default" | "info" | "warning" | "success" | "error";

const STATE_LABEL: Record<BeChangeRequestState, string> = {
  new: "New",
  assess: "Assess",
  authorize: "Authorize",
  customer_approval: "Customer Approval",
  scheduled: "Scheduled",
  implement: "Implement",
  review: "Review",
  customer_review: "Customer Review",
  rollback: "Rollback",
  closed: "Closed",
  canceled: "Canceled",
};

// State chip colour: approvals/reviews are in-flight (info), implement is active
// (warning), rollback/cancel are problem states (error), closed is terminal-good.
const STATE_COLOR: Record<BeChangeRequestState, ChipColor> = {
  new: "default",
  assess: "info",
  authorize: "info",
  customer_approval: "info",
  scheduled: "info",
  implement: "warning",
  review: "info",
  customer_review: "info",
  rollback: "error",
  closed: "success",
  canceled: "error",
};

const IMPACT_LABEL: Record<BeChangeRequestImpact, string> = {
  high: "High",
  medium: "Medium",
  low: "Low",
};

const IMPACT_COLOR: Record<BeChangeRequestImpact, ChipColor> = {
  high: "error",
  medium: "warning",
  low: "default",
};

/** All CR states, for a filter control. */
export const CHANGE_REQUEST_STATES = Object.keys(STATE_LABEL) as BeChangeRequestState[];

/** All CR impact levels, for a filter control. */
export const CHANGE_REQUEST_IMPACTS = Object.keys(IMPACT_LABEL) as BeChangeRequestImpact[];

function humanize(value: string): string {
  return value.replace(/_/g, " ");
}

export function changeRequestStateLabel(state?: string | null): string {
  if (!state) return "—";
  return STATE_LABEL[state as BeChangeRequestState] ?? humanize(state);
}

export function changeRequestStateColor(state?: string | null): ChipColor {
  return STATE_COLOR[state as BeChangeRequestState] ?? "default";
}

/**
 * Human-readable reason a comment cannot be posted on this change request right
 * now, or `null` when it can. Change requests don't share the case work-state
 * model — the only gate is terminal state.
 */
export function changeRequestCommentGateReason(
  state?: string | null,
): string | null {
  if (state === "closed" || state === "canceled") {
    return "Comments are disabled on a closed or canceled change request.";
  }
  return null;
}

export function changeRequestImpactLabel(impact?: string | null): string {
  if (!impact) return "—";
  return IMPACT_LABEL[impact as BeChangeRequestImpact] ?? humanize(impact);
}

export function changeRequestImpactColor(impact?: string | null): ChipColor {
  return IMPACT_COLOR[impact as BeChangeRequestImpact] ?? "default";
}

// Approval-stage / approver status labels and colours. The backend passes
// these through from the data source without validating them (see
// `BeChangeRequestApprover.status`), so both maps are deliberately partial —
// unrecognized values fall back to a humanized version of the raw string
// rather than crashing or rendering nothing.
const APPROVAL_STATUS_LABEL: Record<string, string> = {
  APPROVED: "Approved",
  REJECTED: "Rejected",
  PENDING: "Pending",
  REQUESTED: "Requested",
  NOT_REQUIRED: "Not required",
  CANCELLED: "Cancelled",
  NO_CONSENSUS: "No consensus",
};

const APPROVAL_STATUS_COLOR: Record<string, ChipColor> = {
  APPROVED: "success",
  REJECTED: "error",
  PENDING: "warning",
  REQUESTED: "warning",
  NOT_REQUIRED: "default",
  CANCELLED: "default",
  NO_CONSENSUS: "error",
};

export function approvalStatusLabel(status?: string | null): string {
  if (!status) return "—";
  return APPROVAL_STATUS_LABEL[status.toUpperCase()] ?? humanize(status.toLowerCase());
}

export function approvalStatusColor(status?: string | null): ChipColor {
  if (!status) return "default";
  return APPROVAL_STATUS_COLOR[status.toUpperCase()] ?? "default";
}

export interface ChangeRequestFilters {
  search: string;
  states: BeChangeRequestState[];
  impacts: BeChangeRequestImpact[];
  /** YYYY-MM-DD local date string, or empty. */
  closedStartDate: string;
  /** YYYY-MM-DD local date string, or empty. */
  closedEndDate: string;
}

export const DEFAULT_CR_FILTERS: ChangeRequestFilters = {
  search: "",
  states: [],
  impacts: [],
  closedStartDate: "",
  closedEndDate: "",
};

/** Count non-search active filters (used for the badge on the Filters button). */
export function countActiveCRFilters(filters: ChangeRequestFilters): number {
  return (
    (filters.states.length > 0 ? 1 : 0) +
    (filters.impacts.length > 0 ? 1 : 0) +
    (filters.closedStartDate ? 1 : 0) +
    (filters.closedEndDate ? 1 : 0)
  );
}

// ---------------------------------------------------------------------------
// Clone ("create similar") support: pre-fills the create form from an
// existing change request via router state, so promoting the same change
// through another environment doesn't mean re-typing every field.
//
// This is a *partial* clone by necessity, not by choice: `GET
// /change-requests/{id}` (BeChangeRequestDetail) and `POST /change-requests`
// (BeCreateChangeRequestPayload) are asymmetric on the backend today —
// several fields the create form can set are never returned by the read,
// and several fields the detail page can show have no create-time
// equivalent at all. Concretely (verified against the entity service's own
// request/response structs, not just the two frontend types):
//   - `category`, `priority`, and `risk` are write-only — accepted by create,
//     never present on the read response — so there is no source value to
//     copy from, ever, regardless of how the form is wired.
//   - `implementationPlan` and `riskImpactAnalysis` are write-only for the
//     same reason.
//   - `impactDescription`, `serviceOutage`, `communicationPlan`, and
//     `rollbackPlan` are read-only today — the create payload has no field
//     for any of them.
//   - `project`, `case`, `deployment`, `deployedProduct`, and `product` are
//     read-only refs with no create-time field to set them from at all.
//   - `serviceId`, `serviceOfferingId`, and `configurationItemId` are the
//     mirror image: create accepts all three, and the read response carries no
//     equivalent, so a clone always leaves them blank.
//   - `assignedTeam` is read-only; create's nearest-sounding field
//     (`groupId`, "Assignment group") is a *different* underlying reference
//     with no confirmed equivalence to `assignedTeam` — mapping one into the
//     other would be a guess, not a verified carry-over, so it's left alone.
// None of the above can be safely carried over without either fabricating
// data or guessing at an unconfirmed field mapping, so this only clones the
// fields that are genuinely the same field on both sides: `subject`,
// `description`, `justification`, `testPlan`, `type`, `impact`, and
// `assignedEngineer`. Everything else resets to the create form's own
// defaults, same as a from-scratch change request — see
// `CLONE_SOURCE_GAP_MESSAGE` for the user-facing disclosure of this gap.
export interface CloneChangeRequestNavState {
  /** For the banner shown on the create form — never sent to the backend. */
  sourceNumber?: string;
  subject?: string;
  description?: string;
  justification?: string;
  testPlan?: string;
  type?: BeChangeRequestType;
  impact?: BeChangeRequestImpact;
  assignedEngineerId?: string;
  /** Display label for `assignedEngineerId` until a fresh search resolves it. */
  assignedEngineerLabel?: string;
}

/** Rich-text field carried into the clone form only when it has real content. */
function cloneableHtml(html?: string | null): string | undefined {
  if (!html || isBlankHtml(html)) return undefined;
  return sanitizeRichTextHtml(html);
}

/**
 * Builds the router-state payload for a change request's "Clone" action.
 * Deliberately omits: environment/deployment, state, approval fields
 * (`hasCustomerApproved`/`hasCustomerReviewed`/`approvedBy`/`approvedOn`),
 * planned start/end, and every auto-numbered/timestamp/created-by field —
 * per this feature's requirement that promoting a change to a new
 * environment must never silently carry an approval or a stale schedule
 * across. Comments and attachments are never part of this payload; they
 * belong to the original record only.
 */
export function buildCloneChangeRequestNavState(
  cr: BeChangeRequestDetail,
): CloneChangeRequestNavState {
  return {
    sourceNumber: cr.number,
    subject: cr.subject ?? undefined,
    description: cloneableHtml(cr.description),
    justification: cloneableHtml(cr.justification),
    testPlan: cloneableHtml(cr.testPlan),
    type: (cr.type as BeChangeRequestType) ?? undefined,
    impact: (cr.impact as BeChangeRequestImpact) ?? undefined,
    assignedEngineerId: cr.assignedEngineer?.id || undefined,
    assignedEngineerLabel: cr.assignedEngineer?.name || undefined,
  };
}

/**
 * User-facing disclosure shown on the create form when it was opened via
 * Clone, so the field gap documented above is visible rather than silently
 * dropped. Kept as a single shared string so the detail page (if it ever
 * wants a preview) and the create page stay in sync.
 */
export const CLONE_SOURCE_GAP_MESSAGE =
  "Copied the subject, description, justification, test plan, type, impact, and assigned engineer. " +
  "Category, priority, risk, implementation plan, risk/impact analysis, backout plan, assignment group, " +
  "linked project/case, affected product, service, service offering, and configuration item aren't " +
  "available to copy and need to be re-entered. " +
  "Deployment, schedule, and approval fields are intentionally left blank for you to set for the new environment.";
