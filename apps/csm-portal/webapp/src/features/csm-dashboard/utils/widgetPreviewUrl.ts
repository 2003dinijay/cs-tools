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

/** Marker param set only when the filters object being encoded/decoded uses
 * the case-search generic field/op/values DSL (see
 * `isCaseFieldFilterArray`) — so `parseWidgetPreviewFilters` knows to
 * reconstruct `{ filters: [...] }` rather than a flat key→values record. */
const CASE_FILTER_MARKER = "_cf";

const RESERVED_PARAMS = new Set(["w", "n", CASE_FILTER_MARKER]);

/** Placeholder swapped in for the signed-in user's own id wherever a
 * widget's (opaque, backend-resolved) filters carry it — e.g. "My Cases"
 * resolves to `assignedUserIds: ["<real uuid>"]` — so a bookmarked/shared
 * preview URL never carries a bare internal user id. */
const CURRENT_USER_SENTINEL = "@me";

function isStringArray(value: unknown): value is string[] {
  return Array.isArray(value) && value.every((v) => typeof v === "string");
}

/**
 * One entry of the case-search generic filter DSL (`BeCaseFieldFilter`),
 * structurally typed here (not imported from `types.ts`) since this file
 * works with every resourceType's opaque `Record<string, unknown>` filters,
 * not just case's.
 */
export interface WidgetCaseFieldFilterLike {
  field: string;
  op: string;
  values?: string[];
}

/**
 * True when `value` is the `filters` array of a case widget's filters object
 * (`{ filters: BeCaseFieldFilter[] }` — see `BeCaseSearchFilters`), detected
 * structurally so this file never needs to know the resourceType. Every
 * dashboard case-filter widget today uses `op: "in"` only (see
 * `.env.example`'s `DASHBOARDS_CONFIG`); a non-`"in"` op still round-trips
 * through the URL (its `values` are preserved), but always decodes back with
 * `op: "in"` — a lossy simplification acceptable only because op isn't
 * configurable from this preview UI yet.
 */
export function isCaseFieldFilterArray(value: unknown): value is WidgetCaseFieldFilterLike[] {
  return (
    Array.isArray(value) &&
    value.length > 0 &&
    value.every(
      (e) =>
        e !== null &&
        typeof e === "object" &&
        typeof (e as Record<string, unknown>).field === "string" &&
        typeof (e as Record<string, unknown>).op === "string",
    )
  );
}

/**
 * Builds the URL a dashboard widget tile's "View more" link points at — a
 * real, bookmarkable/shareable/refresh-safe URL (no router state): the
 * resource type is the path segment (`previewSlug`, from
 * `WIDGET_RESOURCE_CONFIG`), the widget's own id/display name are `w`/`n`
 * query params, and each filter field is its own readable query param
 * (e.g. `severities=critical`) rather than one opaque JSON blob — and the
 * signed-in user's own id, wherever it appears, is masked to `@me` (see
 * `CURRENT_USER_SENTINEL`). Read back by `parseWidgetPreviewFilters` /
 * `resolveCurrentUserSentinels` in `DashboardWidgetPreviewPage`.
 */
export function buildWidgetPreviewHref(params: {
  previewSlug: string;
  widgetId: string;
  displayName: string;
  filters: Record<string, unknown>;
  /** The signed-in user's own id, so it can be masked rather than embedded
   * verbatim in the URL. Omit if not yet known — the filter value(s) are
   * then left as-is rather than masked. */
  currentUserId?: string;
}): string {
  const q = new URLSearchParams();
  q.set("w", params.widgetId);
  q.set("n", params.displayName);
  let usesCaseFieldFilterShape = false;
  for (const [key, value] of Object.entries(params.filters)) {
    if (RESERVED_PARAMS.has(key)) continue;
    if (key === "filters" && isCaseFieldFilterArray(value)) {
      // Case widgets carry the generic field/op/values DSL nested under
      // `filters.filters` (see `BeCaseSearchFilters`/`isCaseFieldFilterArray`)
      // — flatten each entry to its own readable `field=values` query param
      // (e.g. `severity=critical,high`), matching the flat encoding below,
      // instead of surfacing one opaque JSON blob.
      usesCaseFieldFilterShape = true;
      for (const entry of value) {
        const values = entry.values ?? [];
        if (values.length === 0) continue;
        const masked = values.map((v) =>
          v === params.currentUserId ? CURRENT_USER_SENTINEL : v,
        );
        q.set(entry.field, masked.join(","));
      }
      continue;
    }
    if (isStringArray(value)) {
      if (value.length === 0) continue;
      const masked = value.map((v) =>
        v === params.currentUserId ? CURRENT_USER_SENTINEL : v,
      );
      q.set(key, masked.join(","));
    } else if (typeof value === "string") {
      q.set(key, value === params.currentUserId ? CURRENT_USER_SENTINEL : value);
    }
  }
  if (usesCaseFieldFilterShape) q.set(CASE_FILTER_MARKER, "1");
  return `/dashboard/${params.previewSlug}?${q.toString()}`;
}

export interface ParsedWidgetPreviewFilters {
  filters: Record<string, unknown>;
  /** True if a filter value still carries the `@me` sentinel and needs
   * `resolveCurrentUserSentinels` before it's safe to query with. */
  needsCurrentUser: boolean;
}

/** Parses every non-reserved (`w`/`n`) query param back into the widget's
 * filters object — the inverse of `buildWidgetPreviewHref`. Every value is
 * decoded as a comma-split string array (matching how every current dashboard
 * widget filter field is shaped — see `widgetResourceConfig.ts`'s
 * translators), so this never throws. */
export function parseWidgetPreviewFilters(
  searchParams: URLSearchParams,
): ParsedWidgetPreviewFilters {
  let needsCurrentUser = false;

  if (searchParams.get(CASE_FILTER_MARKER) === "1") {
    const fieldFilters: WidgetCaseFieldFilterLike[] = [];
    for (const [key, raw] of searchParams.entries()) {
      if (RESERVED_PARAMS.has(key)) continue;
      const values = raw.split(",");
      if (values.includes(CURRENT_USER_SENTINEL)) needsCurrentUser = true;
      // Every dashboard case-filter widget today uses `op: "in"` only — see
      // `isCaseFieldFilterArray`'s doc comment.
      fieldFilters.push({ field: key, op: "in", values });
    }
    return { filters: { filters: fieldFilters }, needsCurrentUser };
  }

  const filters: Record<string, unknown> = {};
  for (const [key, raw] of searchParams.entries()) {
    if (RESERVED_PARAMS.has(key)) continue;

    const values = raw.split(",");
    if (values.includes(CURRENT_USER_SENTINEL)) needsCurrentUser = true;
    filters[key] = values;
  }

  return { filters, needsCurrentUser };
}

/** Substitutes the `@me` sentinel back to the signed-in user's own id —
 * see `buildWidgetPreviewHref`'s masking of that same id. Returns `filters`
 * unchanged if `currentUserId` isn't known yet (caller should hold off
 * querying in that case — see `needsCurrentUser`). */
export function resolveCurrentUserSentinels(
  filters: Record<string, unknown>,
  currentUserId: string | undefined,
): Record<string, unknown> {
  if (!currentUserId) return filters;
  const resolved: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(filters)) {
    if (key === "filters" && isCaseFieldFilterArray(value)) {
      resolved[key] = value.map((entry) => ({
        ...entry,
        values: entry.values?.map((v) =>
          v === CURRENT_USER_SENTINEL ? currentUserId : v,
        ),
      }));
      continue;
    }
    resolved[key] = Array.isArray(value)
      ? value.map((v) => (v === CURRENT_USER_SENTINEL ? currentUserId : v))
      : value === CURRENT_USER_SENTINEL
        ? currentUserId
        : value;
  }
  return resolved;
}
