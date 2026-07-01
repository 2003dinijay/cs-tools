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

import type { SemanticRole } from "@components/SemanticChip";
import type {
  ActivityBreakdown,
  ActivityKey,
  IssueComplexity,
  TaskType,
  TimeCardActivityAction,
  TimeCardApprover,
  TimeCardCategory,
  TimeCardState,
  TimeSheetState,
} from "@features/csm-timecards/types/timeCards";

/** Human labels for the card activity/audit actions. */
export const TIME_CARD_ACTIVITY_LABEL: Record<TimeCardActivityAction, string> = {
  created: "Created",
  edited: "Edited",
  submitted: "Submitted",
  approved: "Approved",
  rejected: "Rejected",
  recalled: "Recalled",
  processed: "Processed",
};

/** Selectable task types (ServiceNow time cards attach to the Task table). */
export const TASK_TYPES: { key: TaskType; label: string }[] = [
  { key: "case", label: "Case" },
  { key: "project", label: "Project" },
  { key: "change_request", label: "Change request" },
  { key: "incident", label: "Incident" },
];

export const TASK_TYPE_LABEL: Record<TaskType, string> = {
  case: "Case",
  project: "Project",
  change_request: "Change request",
  incident: "Incident",
};

/**
 * Mock "assigned tasks" for the auto-generate flow, standing in for the user's
 * open work across the Task table until the backend provides it.
 */
export const MOCK_ASSIGNED_TASKS: {
  taskType: TaskType;
  reference: string;
  label: string;
  category: TimeCardCategory;
}[] = [
  { taskType: "case", reference: "CS0353310", label: "Gateway 5xx spike", category: "Task work" },
  { taskType: "project", reference: "PRJ0010234", label: "APIM upgrade", category: "Task work" },
  { taskType: "change_request", reference: "CHG0044120", label: "TLS cert rotation", category: "Task work" },
  { taskType: "incident", reference: "INC0098871", label: "Cluster node down", category: "Investigation" },
];

/**
 * Fallback approver list for FE-first / offline use, shown when the live user
 * search (`/users/search`) returns nothing. Remove once the backend search is
 * the sole source of approvers.
 */
export const MOCK_APPROVERS: TimeCardApprover[] = [
  { id: "lead-chathura", name: "Chathura Wijesinghe" },
  { id: "lead-dilani", name: "Dilani Jayasuriya" },
  { id: "lead-ruwan", name: "Ruwan Gunawardena" },
];

/** A labelled activity row in the time-breakdown editor. */
export interface ActivityBucket {
  key: ActivityKey;
  label: string;
}

/**
 * The five fixed activity categories, in display order — the activity types from
 * the Time Management requirement (ISSU-009).
 */
export const ACTIVITY_BUCKETS: ActivityBucket[] = [
  { key: "analysisDebugging", label: "Analysis and debugging" },
  { key: "reproduce", label: "Reproduce" },
  { key: "settingUp", label: "Setting up" },
  { key: "providingSolution", label: "Providing solution" },
  { key: "answering", label: "Answering" },
];

/** Work categories (the ServiceNow "Category" field). */
export const TIME_CARD_CATEGORIES: readonly TimeCardCategory[] = [
  "Task work",
  "Investigation",
  "Customer call",
  "Documentation",
];
export const DEFAULT_TIME_CARD_CATEGORY: TimeCardCategory = "Task work";

/** Issue-complexity options (the ServiceNow "Issue Complexity" field). */
export const ISSUE_COMPLEXITY_OPTIONS: readonly IssueComplexity[] = [
  "N/A",
  "Low",
  "Medium",
  "High",
];
export const DEFAULT_ISSUE_COMPLEXITY: IssueComplexity = "N/A";

/**
 * Whether logged time is billable to the customer by default (ISSU-009). Most
 * support work is billable; the engineer can flip it per card.
 */
export const DEFAULT_BILLABLE = true;

/** Short label for a card's billable classification. */
export function billableLabel(billable: boolean): string {
  return billable ? "Billable" : "Non-billable";
}

/** Character caps mirroring the ServiceNow form. */
export const WORK_LOG_MAX = 4000;
export const LEAD_COMMENT_MAX = 500;

/** Label + semantic colour for each card state (drives the status chip). */
export const TIME_CARD_STATE_META: Record<
  TimeCardState,
  { label: string; role: SemanticRole }
> = {
  pending: { label: "Pending", role: "default" },
  submitted: { label: "Submitted", role: "info" },
  approved: { label: "Approved", role: "success" },
  rejected: { label: "Rejected", role: "error" },
  recalled: { label: "Recalled", role: "warning" },
  processed: { label: "Processed", role: "success" },
};

/** Label + semantic colour for each rolled-up time-sheet state. */
export const TIME_SHEET_STATE_META: Record<
  TimeSheetState,
  { label: string; role: SemanticRole }
> = {
  open: { label: "Open", role: "default" },
  submitted: { label: "Submitted", role: "info" },
  approved: { label: "Approved", role: "success" },
  rejected: { label: "Rejected", role: "error" },
  recalled: { label: "Recalled", role: "warning" },
};

/**
 * Group whose members may accept/reject time cards. Configurable at runtime via
 * `window.config.CSM_PORTAL_TEAM_LEAD_GROUP`; falls back to this default when
 * unset so the gate is closed-by-default rather than open.
 */
export const DEFAULT_TEAM_LEAD_GROUP = "csm-leads";

/** Resolve the configured team-lead (approver) group name, or the default. */
export function teamLeadGroup(): string {
  const configured = window.config?.CSM_PORTAL_TEAM_LEAD_GROUP?.trim();
  return configured || DEFAULT_TEAM_LEAD_GROUP;
}

/**
 * Group whose members are time-card admins (edit any user's Pending/Rejected
 * cards, approve by exception). Configurable via
 * `window.config.CSM_PORTAL_TIMECARD_ADMIN_GROUP`.
 */
export const DEFAULT_TIMECARD_ADMIN_GROUP = "csm-timecard-admins";

/** Resolve the configured time-card admin group name, or the default. */
export function timecardAdminGroup(): string {
  const configured = window.config?.CSM_PORTAL_TIMECARD_ADMIN_GROUP?.trim();
  return configured || DEFAULT_TIMECARD_ADMIN_GROUP;
}

/**
 * One-line summary of a breakdown's non-zero activities, e.g.
 * "Analyzing 1h · Patching 1.5h". Returns a placeholder when nothing is logged.
 */
export function breakdownSummary(breakdown: ActivityBreakdown): string {
  const parts = ACTIVITY_BUCKETS.filter((b) => (breakdown[b.key] || 0) > 0).map(
    (b) => `${b.label} ${breakdown[b.key]}h`,
  );
  return parts.length > 0 ? parts.join(" · ") : "No time logged";
}
