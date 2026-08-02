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
 * If `selectedTeamGroupId` is undefined — no team selected yet, or the
 * selected team has no group configured in the deployment's team registry —
 * the `integrationCsTeam` entry is DROPPED from the filter array entirely,
 * rather than either (a) sent with the literal placeholder string, which the
 * entity-service would either reject with a 400 (not a valid UUID) or,
 * worse, silently treat as a value that matches nothing, or (b) sent with an
 * empty `values` array, which the entity-service also rejects for a
 * non-`isEmpty`/`isNotEmpty` op. Dropping the condition instead just widens
 * the query back to "every team" — the same result as if this filter had
 * never been applied — which is the safer failure mode for a dashboard
 * tile: a count/list that's too broad is visibly wrong (an obviously large
 * number, or rows from other teams) and gets noticed, where a query that
 * silently matches zero rows reads as "there's nothing to see here" and
 * doesn't.
 *
 * Every other filter entry, and every other resourceType's filters shape
 * (this only touches the case-search generic field/op/values DSL), passes
 * through unchanged.
 */
export function resolveTeamPlaceholder(
  filters: Record<string, unknown>,
  selectedTeamGroupId: string | undefined,
): Record<string, unknown> {
  const fieldFilters = filters.filters;
  if (!isCaseFieldFilterArray(fieldFilters)) return filters;

  let changed = false;
  const resolved: WidgetCaseFieldFilterLike[] = [];
  for (const entry of fieldFilters) {
    const values = entry.values;
    if (entry.field !== TEAM_FILTER_FIELD || !values?.includes(CURRENT_TEAM_PLACEHOLDER)) {
      resolved.push(entry);
      continue;
    }
    changed = true;
    if (!selectedTeamGroupId) {
      // Drop the entry entirely — see the doc comment above.
      continue;
    }
    resolved.push({
      ...entry,
      values: values.map((v) => (v === CURRENT_TEAM_PLACEHOLDER ? selectedTeamGroupId : v)),
    });
  }

  if (!changed) return filters;
  return { ...filters, filters: resolved };
}
