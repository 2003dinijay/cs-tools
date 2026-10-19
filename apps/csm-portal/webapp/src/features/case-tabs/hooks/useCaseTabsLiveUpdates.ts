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

import { useCallback } from "react";
import { useLocation } from "react-router";
import { matchCaseLocation } from "@context/case-tabs/caseRoutePatterns";
import { useCaseTabsController } from "@context/case-tabs/CaseTabsContext";
import type { CaseRouteKind, CaseTabState } from "@context/case-tabs/caseTabsTypes";
import {
  MAX_STREAM_CASE_IDS,
  useMultiplexedCaseStream,
} from "@features/csm-cases/api/useMultiplexedCaseStream";

/** Kinds whose id is a case id the activity stream serves. Incidents and
 * change requests have their own pages and no case-activity events. */
const STREAMABLE_KINDS: ReadonlySet<CaseRouteKind> = new Set<CaseRouteKind>([
  "case",
  "service_request",
  "engagement",
  "announcement",
  "security_report_analysis",
]);

// The backend rejects the whole stream with a 400 if any id is malformed, so
// anything that is not a canonical UUID (a case number typed into the URL,
// a dashless id awaiting repair) is left out rather than risked.
const UUID_PATTERN = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

function isStreamable(kind: CaseRouteKind, caseId: string): boolean {
  return STREAMABLE_KINDS.has(kind) && UUID_PATTERN.test(caseId);
}

/**
 * The ids to subscribe to: the case on screen first, then every open tab
 * (most recently opened first, the closest proxy for "most recently used"
 * the tab state carries), de-duplicated and cut to `max`. Open tabs are
 * capped at `MAX_OPEN_CASE_TABS` (10), below the backend's 20, so the cut only
 * matters if either limit moves.
 */
export function selectStreamCaseIds(
  tabs: readonly CaseTabState[],
  routeCaseId: string | undefined,
  activeCaseId: string | undefined,
  max: number = MAX_STREAM_CASE_IDS,
): string[] {
  const ordered = [
    ...(routeCaseId ? [routeCaseId] : []),
    ...(activeCaseId ? [activeCaseId] : []),
    ...[...tabs].reverse().map((t) => t.caseId),
  ];
  const seen = new Set<string>();
  const result: string[] = [];
  for (const id of ordered) {
    const key = id.toLowerCase();
    if (seen.has(key)) continue;
    seen.add(key);
    result.push(id);
    if (result.length === max) break;
  }
  return result;
}

/**
 * Mounted once at the case-tabs workspace. Holds the single multiplexed
 * activity stream for every open case tab (see `useMultiplexedCaseStream`),
 * plus the case on screen when it is not tab-managed (cap-blocked, or the
 * tab mechanism is off), so a case page always keeps its live updates. On an
 * update for a tab that is not on screen, flags that tab so `CaseTabStrip`
 * blinks it until it is activated.
 */
export function useCaseTabsLiveUpdates(): void {
  const { tabs, activeTab, markTabUpdated } = useCaseTabsController();
  const { pathname } = useLocation();

  const routeMatch = matchCaseLocation(pathname);
  const routeCaseId =
    routeMatch && isStreamable(routeMatch.kind, routeMatch.caseId) ? routeMatch.caseId : undefined;
  const streamableTabs = tabs.filter((t) => isStreamable(t.kind, t.caseId));
  const activeCaseId =
    activeTab && isStreamable(activeTab.kind, activeTab.caseId) ? activeTab.caseId : undefined;

  const caseIds = selectStreamCaseIds(streamableTabs, routeCaseId, activeCaseId);

  // Whether the active tab is what the user is looking at: the current route
  // is that tab's own case. Off it (e.g. on the dashboard, the pinned tab is
  // the live view) the active tab is as unseen as any other.
  const activeTabVisible = !!routeMatch && !!activeTab && routeMatch.caseId === activeTab.caseId;

  const onCaseUpdated = useCallback(
    (caseId: string) => markTabUpdated(caseId, activeTabVisible),
    [markTabUpdated, activeTabVisible],
  );

  useMultiplexedCaseStream(caseIds, onCaseUpdated);
}
