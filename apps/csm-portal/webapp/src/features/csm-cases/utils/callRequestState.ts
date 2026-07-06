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

import type { BeCallRequestStateKey } from "@api/backend/types";

/** Human-readable label for each call request state key. */
export const CALL_REQUEST_STATE_LABEL: Record<BeCallRequestStateKey, string> = {
  pending_on_customer: "Pending on customer",
  pending_on_wso2: "Pending on WSO2",
  scheduled: "Scheduled",
  customer_rejected: "Rejected by customer",
  wso2_rejected: "Rejected by WSO2",
  canceled: "Canceled",
  notes_pending: "Notes pending",
  concluded: "Concluded",
};

/**
 * MUI Chip `color` for each state. Maps to the Oxygen UI palette.
 * Terminal states (rejected, canceled, concluded) use muted colours;
 * active/pending states use the action palette.
 */
export const CALL_REQUEST_STATE_COLOR: Record<
  BeCallRequestStateKey,
  "default" | "primary" | "secondary" | "error" | "info" | "success" | "warning"
> = {
  pending_on_customer: "warning",
  pending_on_wso2: "info",
  scheduled: "primary",
  customer_rejected: "error",
  wso2_rejected: "error",
  canceled: "default",
  notes_pending: "warning",
  concluded: "success",
};

/**
 * Resolve the display label for a call request state returned by the backend.
 * The backend may return `state.label` directly; fall back to our own label map
 * using the id cast to a string key when `label` is absent or when the id is a
 * known enum key.
 */
export function callRequestStateLabel(state: {
  id: number | string;
  label?: string;
} | undefined): string {
  if (!state) return "Unknown";
  if (state.label) return state.label;
  // id may be the string key or an opaque integer — map what we can.
  const key = String(state.id) as BeCallRequestStateKey;
  return CALL_REQUEST_STATE_LABEL[key] ?? String(state.id);
}

/**
 * Resolve the MUI chip color for a call request state returned by the backend.
 * Same resolution logic as `callRequestStateLabel`.
 */
export function callRequestStateColor(
  state: { id: number | string; label?: string } | undefined,
): "default" | "primary" | "secondary" | "error" | "info" | "success" | "warning" {
  if (!state) return "default";
  const key = String(state.id) as BeCallRequestStateKey;
  return CALL_REQUEST_STATE_COLOR[key] ?? "default";
}

/** All 8 state keys, used to drive filter dropdowns. */
export const ALL_CALL_REQUEST_STATES: BeCallRequestStateKey[] = [
  "pending_on_customer",
  "pending_on_wso2",
  "scheduled",
  "customer_rejected",
  "wso2_rejected",
  "canceled",
  "notes_pending",
  "concluded",
];

/**
 * Agent-facing actions available on a call request, keyed by its current
 * state. This mirrors exactly what the backend can fulfil today (see the
 * cross-layer contract) -- do not add an action here unless there is a
 * corresponding backend endpoint, otherwise the UI offers a transition that
 * fails on submit.
 *
 * - `pending_on_wso2`: agent can schedule the call or reject it.
 * - `scheduled`: agent can reschedule or cancel. Sending notes from this
 *   state is normally gated by the backend's post-due automation moving the
 *   request to `notes_pending` first; it is not offered here.
 * - `notes_pending`: agent can send call notes (concludes the request).
 * - `pending_on_customer`: the customer owns scheduling/rejecting; the agent
 *   can only cancel on their behalf.
 * - Terminal states (`customer_rejected`, `wso2_rejected`, `canceled`,
 *   `concluded`): no actions.
 */
export type CallRequestAgentAction = "schedule" | "reschedule" | "reject" | "sendNotes" | "cancel";

export const CALL_REQUEST_AGENT_ACTIONS: Record<
  BeCallRequestStateKey,
  CallRequestAgentAction[]
> = {
  pending_on_customer: ["cancel"],
  pending_on_wso2: ["schedule", "reject"],
  scheduled: ["reschedule", "cancel"],
  customer_rejected: [],
  wso2_rejected: [],
  canceled: [],
  notes_pending: ["sendNotes"],
  concluded: [],
};
