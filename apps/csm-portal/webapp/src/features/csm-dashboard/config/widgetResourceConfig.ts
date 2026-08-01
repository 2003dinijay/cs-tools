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

import type { BeWidgetResourceType } from "@api/backend/types";
import { humanizeState } from "@features/csm-dashboard/utils/abtDashboard";
import { casesHref } from "@features/csm-cases/utils/casesFiltersUrl";
import type { CasesFilters } from "@features/csm-cases/components/CasesFilterBar";
import type { Severity } from "@features/csm-dashboard/types/abtDashboard";
import {
  DEFAULT_INCIDENT_FILTERS,
  type IncidentFilters,
} from "@features/csm-operations/utils/incidents";
import { writeIncidentFiltersToUrl } from "@features/csm-operations/utils/incidentsFiltersUrl";
import {
  DEFAULT_CR_FILTERS,
  type ChangeRequestFilters,
} from "@features/csm-operations/utils/changeRequests";
import { writeChangeRequestFiltersToUrl } from "@features/csm-operations/utils/changeRequestsFiltersUrl";

/** A resolved search-result row, typed loosely since its real shape depends
 * on `resourceType` — the label extractors below narrow what they read. */
type WidgetItem = Record<string, unknown>;

/**
 * Per-resource-type wiring for a dashboard widget: where to fetch its data,
 * how to read a list-shape row for display, and where a click on the tile
 * navigates.
 */
export interface WidgetResourceConfig {
  /** `POST` endpoint this resource's own search lives at. */
  searchEndpoint: string;
  /** Key the response's item array is nested under. */
  itemsKey: string;
  /** Primary (bold) line for one list-shape row. */
  primaryLabel: (item: WidgetItem) => string;
  /** Optional secondary (muted) line for one list-shape row. */
  secondaryLabel?: (item: WidgetItem) => string | undefined;
  /** Where a click on this widget's tile navigates, given its (opaque,
   * already current-user-resolved) filters. */
  buildHref: (filters: Record<string, unknown>) => string;
}

function asString(v: unknown): string | undefined {
  return typeof v === "string" && v.length > 0 ? v : undefined;
}

function asStringArray(v: unknown): string[] | undefined {
  return Array.isArray(v) && v.every((x) => typeof x === "string")
    ? (v as string[])
    : undefined;
}

// ---------------------------------------------------------------------------
// case — /cases, translating the opaque dashboard filters into CasesFilters.
// ---------------------------------------------------------------------------

/**
 * The dashboard/entity-service case severity values are the lowercase
 * `catastrophic|critical|high|medium|low` enum; the cases list's own filter
 * bar (and its URL encoding) uses the `S0`..`S4` codes instead. No existing
 * mapping between the two lives anywhere else in the app (the app-wide
 * `SEVERITY_LABEL` maps `S0` -> "Catastrophic", a display label, not this
 * enum) — scoped here to dashboard click-through only.
 */
const DASHBOARD_SEVERITY_TO_S_CODE: Record<string, Severity> = {
  catastrophic: "S0",
  critical: "S1",
  high: "S2",
  medium: "S3",
  low: "S4",
};

/**
 * Translate a dashboard widget's opaque case filters into the cases list's
 * own `CasesFilters` shape. `tags` has no equivalent in `CasesFilters` today
 * (the case-list tag filter was pulled out of the filter bar/URL — see the
 * note on `CasesFilters.tags` in `casesFiltersUrl.ts`) and is dropped rather
 * than invented. `assignedUserIds` carries the current user's own UUID
 * (every widget that sets it does so via the current-user placeholder), and
 * `CasesFilters.assignees` is email/`@me`-based with no UUID lookup
 * available here — since these widgets only ever filter "assigned to me",
 * any non-empty `assignedUserIds` maps to the `@me` sentinel rather than an
 * (unresolvable) literal UUID.
 */
function translateCaseDashboardFilters(
  filters: Record<string, unknown>,
): Partial<CasesFilters> {
  const out: Partial<CasesFilters> = {};
  const states = asStringArray(filters.states);
  if (states) out.states = states as CasesFilters["states"];
  const severities = asStringArray(filters.severities);
  if (severities) {
    out.severities = severities
      .map((s) => DASHBOARD_SEVERITY_TO_S_CODE[s])
      .filter((s): s is Severity => Boolean(s));
  }
  const types = asStringArray(filters.types);
  if (types) out.caseTypes = types as CasesFilters["caseTypes"];
  const productNames = asStringArray(filters.productNames);
  if (productNames) out.productNames = productNames;
  const assignedUserIds = asStringArray(filters.assignedUserIds);
  if (assignedUserIds && assignedUserIds.length > 0) out.assignees = ["@me"];
  return out;
}

// ---------------------------------------------------------------------------
// incident / change_request / problem — all live under /operations, switched
// by `?tab=`.
// ---------------------------------------------------------------------------

function operationsHref(tab: string, params?: URLSearchParams): string {
  const out = new URLSearchParams();
  out.set("tab", tab);
  params?.forEach((value, key) => out.set(key, value));
  return `/operations?${out.toString()}`;
}

/** Dashboard incident filters already use the real `BeIncidentPriority`
 * wire values (`CRITICAL`/`HIGH`/...), same as `IncidentFilters.priorities` —
 * no translation table needed, only a type narrowing. */
function translateIncidentDashboardFilters(
  filters: Record<string, unknown>,
): Partial<IncidentFilters> {
  const out: Partial<IncidentFilters> = {};
  const priorities = asStringArray(filters.priorities);
  if (priorities) out.priorities = priorities as IncidentFilters["priorities"];
  return out;
}

/** Dashboard CR filters already use the real `BeChangeRequestState`/`Impact`
 * wire values, same as `ChangeRequestFilters` — no translation needed. */
function translateChangeRequestDashboardFilters(
  filters: Record<string, unknown>,
): Partial<ChangeRequestFilters> {
  const out: Partial<ChangeRequestFilters> = {};
  const states = asStringArray(filters.states);
  if (states) out.states = states as ChangeRequestFilters["states"];
  const impacts = asStringArray(filters.impacts);
  if (impacts) out.impacts = impacts as ChangeRequestFilters["impacts"];
  return out;
}

export const WIDGET_RESOURCE_CONFIG: Record<
  BeWidgetResourceType,
  WidgetResourceConfig
> = {
  case: {
    searchEndpoint: "/cases/search",
    itemsKey: "cases",
    primaryLabel: (item) =>
      [asString(item.number), asString(item.subject)]
        .filter(Boolean)
        .join(" — ") || "—",
    secondaryLabel: (item) => {
      const state = asString(item.state);
      return state ? humanizeState(state) : undefined;
    },
    buildHref: (filters) => casesHref(translateCaseDashboardFilters(filters)),
  },
  incident: {
    searchEndpoint: "/incidents/search",
    itemsKey: "incidents",
    primaryLabel: (item) =>
      [asString(item.number), asString(item.subject)]
        .filter(Boolean)
        .join(" — ") || "—",
    secondaryLabel: (item) => asString(item.priority),
    buildHref: (filters) =>
      operationsHref(
        "incidents",
        writeIncidentFiltersToUrl({
          ...DEFAULT_INCIDENT_FILTERS,
          ...translateIncidentDashboardFilters(filters),
        }),
      ),
  },
  change_request: {
    searchEndpoint: "/change-requests/search",
    itemsKey: "changeRequests",
    primaryLabel: (item) =>
      [asString(item.number), asString(item.subject)]
        .filter(Boolean)
        .join(" — ") || "—",
    secondaryLabel: (item) => {
      const state = asString(item.state);
      return state ? humanizeState(state) : undefined;
    },
    buildHref: (filters) =>
      operationsHref(
        "change_requests",
        writeChangeRequestFiltersToUrl({
          ...DEFAULT_CR_FILTERS,
          ...translateChangeRequestDashboardFilters(filters),
        }),
      ),
  },
  problem: {
    searchEndpoint: "/problems/search",
    itemsKey: "problems",
    primaryLabel: (item) =>
      [asString(item.number), asString(item.subject)]
        .filter(Boolean)
        .join(" — ") || "—",
    secondaryLabel: (item) => {
      const state = asString(item.state);
      return state ? humanizeState(state) : undefined;
    },
    // No dashboard widget filters problems today; the tab has no URL filter
    // scheme of its own yet either, so this is unfiltered.
    buildHref: () => operationsHref("problems"),
  },
  account: {
    searchEndpoint: "/accounts/search",
    itemsKey: "accounts",
    primaryLabel: (item) => asString(item.name) ?? "—",
    secondaryLabel: (item) => asString(item.tier),
    buildHref: () => "/customers/accounts",
  },
  project: {
    searchEndpoint: "/projects/search",
    itemsKey: "projects",
    primaryLabel: (item) => asString(item.name) ?? asString(item.projectKey) ?? "—",
    secondaryLabel: (item) => asString(item.subscriptionType),
    buildHref: () => "/customers/projects",
  },
  user: {
    searchEndpoint: "/users/search",
    itemsKey: "users",
    primaryLabel: (item) => {
      const first = asString(item.firstName);
      const last = asString(item.lastName);
      const full = [first, last].filter(Boolean).join(" ");
      return full || asString(item.userName) || asString(item.email) || "—";
    },
    secondaryLabel: (item) => asString(item.email),
    buildHref: () => "/admin/users",
  },
  time_card: {
    searchEndpoint: "/time-cards/search",
    itemsKey: "timeCards",
    primaryLabel: (item) => {
      const caseNumber = nestedNumber(item.case);
      const workDate = asString(item.workDate);
      return [caseNumber, workDate].filter(Boolean).join(" — ") || "—";
    },
    secondaryLabel: (item) => {
      const state = asString(item.state);
      return state ? humanizeState(state) : undefined;
    },
    buildHref: () => "/time-cards",
  },
  product_vulnerability: {
    searchEndpoint: "/products/vulnerabilities/search",
    itemsKey: "productVulnerabilities",
    primaryLabel: (item) =>
      asString(item.cveId) ?? asString(item.vulnerabilityId) ?? "—",
    secondaryLabel: (item) =>
      asString(item.priority) ?? asString(item.productName),
    buildHref: () => "/security-center",
  },
};

function nestedNumber(v: unknown): string | undefined {
  if (v && typeof v === "object" && "number" in v) {
    return asString((v as { number?: unknown }).number);
  }
  return undefined;
}
