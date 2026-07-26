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

//
// The time-card feature is wired to the real csm-portal-backend — there is no
// seeded mock data, and no delete endpoint, so anything a spec creates via
// `POST /time-cards` becomes a permanent record in staging. Every card an E2E
// spec creates MUST carry E2E_TAG in its work-log comment so it's easy to
// find/identify later, and specs must never assume any pre-existing card.
//

/** Prefix every E2E-created work-log comment with this, followed by a
 * timestamp, so entries are identifiable and never collide across runs. */
export const E2E_TAG = "[E2E]";

/** The dedicated, data-complete test project (deployment + deployed product
 * + catalog all configured) that create/provision flows should target by
 * default, instead of relying on the tenant's first-page project (most
 * other DEV-SN projects are corrupted or missing deployment/product data).
 * Env-overridable for a different environment's equivalent project. */
export const E2E_PROJECT =
  process.env.E2E_PROJECT ?? "Customer Portal Project";

export function e2eWorkLogComment(label: string): string {
  return `${E2E_TAG} ${label} — ${new Date().toISOString()}`;
}

/** Page-level accessible names used across the time-cards UI. */
export const TIMECARDS = {
  path: "/time-cards",
  heading: "Time cards",
  tabs: { mine: "My time sheets", approvals: "Approvals" },
} as const;

//
// Change requests are wired to the real csm-portal-backend too — POST
// /change-requests has no delete endpoint, so anything a spec creates
// becomes a permanent ServiceNow record. Same tagging rule as time cards.
//

/** Same E2E_TAG + label + timestamp format as {@link e2eWorkLogComment} —
 * kept as its own named export since it tags a different kind of record,
 * but delegates to avoid duplicating the format itself. */
export function e2eChangeRequestSubject(label: string): string {
  return e2eWorkLogComment(label);
}

export const CHANGE_REQUEST_CREATE = {
  path: "/operations/change-requests/new",
  heading: "New change request",
} as const;

//
// Incidents are wired to the real csm-portal-backend too — POST /incidents
// has no delete endpoint (only search/create/get), so anything a spec
// creates becomes a permanent ServiceNow record. Same tagging rule as time
// cards and change requests.
//

/** Same E2E_TAG + label + timestamp format as {@link e2eWorkLogComment} —
 * kept as its own named export since it tags a different kind of record,
 * but delegates to avoid duplicating the format itself. */
export function e2eIncidentSubject(label: string): string {
  return e2eWorkLogComment(label);
}

export const INCIDENT_CREATE = {
  path: "/operations/incidents/new",
  heading: "New incident",
} as const;

//
// Cases (support cases, service requests, problems, security reports) are all
// wired to the real csm-portal-backend — POST /cases (and /problems) has no
// delete endpoint, so anything a spec creates becomes a permanent record.
// Same tagging rule as time cards, change requests, and incidents.
//

/** Same E2E_TAG + label + timestamp format as {@link e2eWorkLogComment} —
 * kept as its own named export since it tags a different kind of record,
 * but delegates to avoid duplicating the format itself. */
export function e2eCaseSubject(label: string): string {
  return e2eWorkLogComment(label);
}

/** Same format as {@link e2eCaseSubject}, for service requests (created via
 * `POST /cases` with `type: "service_request"` — the create form has no
 * free-text subject field of its own; the subject is used to tag the
 * work-log/comment text this suite posts against a created SR instead). */
export function e2eServiceRequestSubject(label: string): string {
  return e2eWorkLogComment(label);
}

/** Same format as {@link e2eCaseSubject}, for problems (`POST /problems`,
 * whose only required field IS a free-text Subject). */
export function e2eProblemSubject(label: string): string {
  return e2eWorkLogComment(label);
}

/** Same format as {@link e2eCaseSubject}, for security reports (`POST /cases`
 * with `type: "security_report_analysis"` — Subject is auto-generated but
 * editable; specs should overwrite it with this tagged value). */
export function e2eSecurityReportSubject(label: string): string {
  return e2eWorkLogComment(label);
}

/**
 * Cases list (`/cases`) — the all-cases view. Also reused, parameterized by
 * base path, for the Service Requests tab list (`/operations` with
 * `?tab=service_requests`, detail base `/operations/service-requests`) and
 * the Engagements list (`/engagements`) — both render the same
 * `CsmIssuesView` component, just with different `lockedFilters` /
 * `detailBasePath` props (see `CsmIssuesView.tsx`).
 *
 * `detailTabs` are the case-detail page's tab labels (`CsmCaseDetailPage.tsx`
 * `TAB_DEFS`) — note the tab's accessible name gets a trailing " (N)" count
 * appended once its widget has data (e.g. "Attachments (3)"), so specs should
 * match these as a prefix/regex, not an exact string.
 *
 * There is no `tasks` entry: the Tasks tab is currently `hidden: true` in
 * `TAB_DEFS` (review follow-up) — unreachable via tab navigation, though the
 * underlying feature (data, hooks, widget, "Create task…" action) is still
 * live. Don't add it back here without first confirming the tab bar itself
 * has it again.
 */
export const CASES = {
  path: "/cases",
  heading: "Cases",
  create: { path: "/cases/new", heading: "New case" },
  detailTabs: {
    activities: "Activities",
    details: "Details",
    related: "Related",
    sla: "SLAs",
    attachments: "Attachments",
    time: "Time tracking",
    callRequests: "Call requests",
  },
} as const;

export const SERVICE_REQUEST_CREATE = {
  path: "/operations/service-requests/new",
  heading: "New service request",
} as const;

export const PROBLEM_CREATE = {
  path: "/operations/problems/new",
  heading: "New problem",
} as const;

/** Engagements list — `CsmIssuesView` locked to `type: engagement`, detail
 * links point at `/engagements/:id` (still `CsmCaseDetailPage` underneath). */
export const ENGAGEMENTS = {
  path: "/engagements",
  heading: "Engagements",
} as const;

export const ANNOUNCEMENTS = {
  path: "/announcements",
  heading: "Announcements",
} as const;

export const UPDATES = {
  path: "/updates",
  heading: "Updates",
} as const;

/** Security Center — tabbed landing (`Security reports` / `Vulnerabilities`).
 * The Security reports tab is a `CsmIssuesView` with the default
 * `detailBasePath` ("/cases") — NOT "/security-center/..." — so a report row
 * opens the ordinary case-detail page at `/cases/:id`. */
export const SECURITY_CENTER = {
  path: "/security-center",
  heading: "Security Center",
  tabs: { reports: "Security reports", vulnerabilities: "Vulnerabilities" },
} as const;

export const SECURITY_REPORT_CREATE = {
  path: "/security-center/reports/new",
  heading: "New security report",
} as const;
