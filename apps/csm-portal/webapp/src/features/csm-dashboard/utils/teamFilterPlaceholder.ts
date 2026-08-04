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

import { isCaseFieldFilterArray, type WidgetCaseFieldFilterLike } from "./widgetPreviewUrl";

/**
 * Placeholder value an `integrationCsTeam` filter entry's `values` array may
 * carry — mirrors how `assignedUserId` widgets carry the signed-in user's
 * own placeholder and how the entity-service resolves its own
 * `__current_user_email__` for `createdBy` server-side. Unlike both of
 * those, this one is resolved entirely CLIENT-SIDE: it stands for "the
 * currently selected team in this dashboard's own UI state", not anything
 * about the signed-in user's identity, so only this frontend (never the
 * entity-service) ever sees it.
 */
export const CURRENT_TEAM_PLACEHOLDER = "__current_team__";

/**
 * Sentinel `selectedTeamId`/`selectedTeamGroupId` value for the team
 * picker's "All ABTs" option (see `AbtDashboardHeader`) — every team in the
 * current dashboard's own family (`abtFamilyForDashboardType`), OR'd
 * together, rather than exactly one team's `groupId`. `__`-prefixed so it
 * can never collide with a real `BeTeam.id` (registry team keys like
 * `"castor"` never use that convention). This is deliberately narrower than
 * the "My ABT / All customers" toggle removed 2026-08-02 (see
 * `AbtDashboardHeader`'s own doc comment) — that toggle spanned every team
 * in the whole registry; this one never leaves the current dashboard's
 * family.
 */
export const ALL_TEAMS_SENTINEL = "__all__";

const TEAM_FILTER_FIELD = "integrationCsTeam";

/**
 * Substitutes {@link CURRENT_TEAM_PLACEHOLDER} wherever it appears in an
 * `integrationCsTeam` filter entry's `values` (in the same
 * `{ filters: BeCaseFieldFilter[] }` DSL shape `mergeWidgetFilters` and
 * `resolveCurrentUserSentinels` already walk) with the selected team's own
 * `groupId` — the backing data source's assignment-group id reformatted as
 * this platform's UUID (`BeTeam.groupId`), never the team registry key
 * (`BeTeam.id`) that `integrationCsTeam` values are NOT keyed by.
 *
 * `selectedTeamGroupId` may also be an array of `groupId`s — the "All ABTs"
 * case (see {@link ALL_TEAMS_SENTINEL}): every team in the current
 * dashboard's family, OR'd together. The single placeholder occurrence in
 * `values` is then spliced out and replaced with all of them (`in` already
 * supports multiple values — no backend change needed), rather than the
 * 1:1 substitution a single string gets. An empty array is treated the same
 * as `undefined` (see below) — a family with no teams configured is no
 * different from no team selected at all.
 *
 * If `selectedTeamGroupId` is undefined (or an empty array) — no team
 * selected yet, or the selected team has no group configured in the
 * deployment's team registry — the `integrationCsTeam` entry is DROPPED
 * from the filter array entirely, rather than either (a) sent with the
 * literal placeholder string, which the entity-service would either reject
 * with a 400 (not a valid UUID) or, worse, silently treat as a value that
 * matches nothing, or (b) sent with an empty `values` array, which the
 * entity-service also rejects for a non-`isEmpty`/`isNotEmpty` op. Dropping
 * the condition instead just widens the query back to "every team" — the
 * same result as if this filter had never been applied — which is the
 * safer failure mode for a dashboard tile: a count/list that's too broad is
 * visibly wrong (an obviously large number, or rows from other teams) and
 * gets noticed, where a query that silently matches zero rows reads as
 * "there's nothing to see here" and doesn't.
 *
 * Every other filter entry, and every other resourceType's filters shape
 * (this only touches the case-search generic field/op/values DSL), passes
 * through unchanged.
 */
export function resolveTeamPlaceholder(
  filters: Record<string, unknown>,
  selectedTeamGroupId: string | string[] | undefined,
): Record<string, unknown> {
  const fieldFilters = filters.filters;
  if (!isCaseFieldFilterArray(fieldFilters)) return filters;

  const hasReplacement =
    selectedTeamGroupId !== undefined &&
    (!Array.isArray(selectedTeamGroupId) || selectedTeamGroupId.length > 0);

  let changed = false;
  const resolved: WidgetCaseFieldFilterLike[] = [];
  for (const entry of fieldFilters) {
    const values = entry.values;
    if (entry.field !== TEAM_FILTER_FIELD || !values?.includes(CURRENT_TEAM_PLACEHOLDER)) {
      resolved.push(entry);
      continue;
    }
    changed = true;
    if (!hasReplacement) {
      // Drop the entry entirely — see the doc comment above.
      continue;
    }
    resolved.push({
      ...entry,
      values: values.flatMap((v) =>
        v === CURRENT_TEAM_PLACEHOLDER
          ? Array.isArray(selectedTeamGroupId)
            ? selectedTeamGroupId
            : [selectedTeamGroupId as string]
          : [v],
      ),
    });
  }

  if (!changed) return filters;
  return { ...filters, filters: resolved };
}
