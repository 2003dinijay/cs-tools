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

/**
 * Text-interpolation token a widget config author may put inside a
 * `displayName`/`description` string (e.g.
 * `"Open Incidents — {{currentTeam}}"` in a dashboard config JSON), resolved
 * client-side by `resolveWidgetText` below. Deliberately a distinct, more
 * visually obvious shape (`{{double-curly}}`) than the `__snake__`-style
 * filter-value placeholders in `teamFilterPlaceholder.ts`
 * (`CURRENT_TEAM_PLACEHOLDER`, `ALL_TEAMS_SENTINEL`) — those live inside a
 * filter entry's `values` array and are resolved into the case-search
 * filter DSL; this one is resolved into plain text a user reads, so a
 * config author should be able to tell the two mechanisms apart at a
 * glance.
 */
export const CURRENT_TEAM_TEXT_TOKEN = "{{currentTeam}}";

/**
 * Substitutes every occurrence of {@link CURRENT_TEAM_TEXT_TOKEN} in a
 * widget's `displayName`/`description` with `teamLabel` — the selected
 * team's own display `name` (e.g. `"Castor"`), or the literal string
 * `"All ABTs"` when the "All ABTs" option is selected (see
 * `ALL_TEAMS_SENTINEL` in `teamFilterPlaceholder.ts`). `teamLabel` is a
 * human-readable label, never a `groupId` (an opaque UUID, useless for
 * display) or the team registry key.
 *
 * When `teamLabel` is `undefined` — not a team-based dashboard, or the team
 * list/user profile is still loading — the token is stripped to an empty
 * string rather than left literally visible to the user, and a resulting
 * trailing separator (` — `, ` | `, ` - `) plus any trailing whitespace is
 * trimmed off, so `"Cases — {{currentTeam}}"` unresolved renders as
 * `"Cases"` rather than `"Cases — "`. This is a best-effort cosmetic trim
 * (a single trailing separator), not a general template engine — it does
 * not handle the token appearing mid-string with trailing text after it.
 *
 * Returns `text` itself unchanged (same reference) when it doesn't contain
 * the token at all, and `undefined` when `text` itself is `undefined` (the
 * `description` prop is optional).
 */
export function resolveWidgetText(
  text: string | undefined,
  teamLabel: string | undefined,
): string | undefined {
  if (text === undefined || !text.includes(CURRENT_TEAM_TEXT_TOKEN)) return text;

  if (teamLabel !== undefined) {
    return text.split(CURRENT_TEAM_TEXT_TOKEN).join(teamLabel);
  }

  const stripped = text.split(CURRENT_TEAM_TEXT_TOKEN).join("");
  return stripped.replace(/\s*[—|-]\s*$/, "").trimEnd();
}
