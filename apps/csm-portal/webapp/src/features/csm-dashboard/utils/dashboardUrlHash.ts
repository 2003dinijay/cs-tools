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
 * The selected dashboard (and, for a team-based dashboard, the selected
 * team) is kept in the URL's fragment rather than a query param —
 * `#<dashboardId>` with no team, `#<dashboardId>.<teamId>` when one is
 * selected — so a link to a specific dashboard/team view is still
 * shareable/bookmarkable/refresh-safe (same intent the old `?dashboard=` /
 * `&team=` version had) without occupying the query string. See
 * `CsmDashboardPage.tsx`.
 */
export interface ParsedDashboardHash {
  dashboardId?: string;
  teamId?: string;
}

/**
 * Parses `location.hash` (with or without its leading `#`) back into a
 * dashboard id and an optional team id, split on the FIRST `.` only — a
 * dashboard id itself never contains a `.` in the current registry, but
 * splitting on the first occurrence rather than requiring none keeps this
 * from breaking if that ever changes. Never throws: an empty hash returns
 * both fields `undefined`, and a stray trailing `.` with nothing after it
 * yields `teamId: undefined` rather than an empty string. Whether a parsed
 * `teamId` is actually applicable is a decision the caller makes once it
 * knows the resolved dashboard's `isTeamBased` flag — this function has no
 * opinion on that (a hand-edited/stale URL's `.teamId` suffix on a
 * non-team-based dashboard must not crash here, or anywhere downstream).
 */
export function parseDashboardHash(hash: string): ParsedDashboardHash {
  const raw = hash.startsWith("#") ? hash.slice(1) : hash;
  if (!raw) return {};
  const dotIndex = raw.indexOf(".");
  if (dotIndex === -1) return { dashboardId: raw };
  const dashboardId = raw.slice(0, dotIndex);
  const teamId = raw.slice(dotIndex + 1);
  return { dashboardId, teamId: teamId.length > 0 ? teamId : undefined };
}

/**
 * Inverse of {@link parseDashboardHash}: builds the `#...` fragment for a
 * given dashboard/team selection, including the leading `#`. Omits the team
 * suffix entirely when `teamId` is absent — same behavior the old
 * `?dashboard=&team=` version had when clearing a stale `team` param on a
 * non-team-based dashboard.
 */
export function buildDashboardHash(dashboardId: string, teamId?: string): string {
  return teamId ? `#${dashboardId}.${teamId}` : `#${dashboardId}`;
}
