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

import { describe, expect, it } from "vitest";
import { QueryClient } from "@tanstack/react-query";
import { ApiQueryKeys } from "@constants/apiConstants";
import { invalidateCallRequestWidgetQueries, invalidateWidgetQueries } from "./invalidateWidgetQueries";

const KEY = ApiQueryKeys.CSM_DASHBOARD_WIDGET_DATA;

function seedQuery(queryClient: QueryClient, queryKey: unknown[]) {
  queryClient.setQueryData(queryKey, { seeded: true });
}

describe("invalidateWidgetQueries", () => {
  it("invalidates every widget-data query-key shape for a matching widget id", async () => {
    const queryClient = new QueryClient();
    const targetId = "widget-1";

    seedQuery(queryClient, [KEY, targetId]); // count/list shape
    seedQuery(queryClient, [KEY, "pie-slice", targetId, "slice-a"]);
    seedQuery(queryClient, [KEY, "group-by", targetId, "field"]);
    seedQuery(queryClient, [KEY, "feedback-trend", targetId, "month"]);
    seedQuery(queryClient, [KEY, "unrelated-widget"]); // not targeted

    await invalidateWidgetQueries(queryClient, new Set([targetId]));

    const states = queryClient.getQueryCache().getAll();
    const isStale = (queryKey: unknown[]) =>
      states.find((q) => JSON.stringify(q.queryKey) === JSON.stringify(queryKey))?.isStale();

    expect(isStale([KEY, targetId])).toBe(true);
    expect(isStale([KEY, "pie-slice", targetId, "slice-a"])).toBe(true);
    expect(isStale([KEY, "group-by", targetId, "field"])).toBe(true);
    expect(isStale([KEY, "feedback-trend", targetId, "month"])).toBe(true);
    expect(isStale([KEY, "unrelated-widget"])).toBe(false);
  });
});

describe("invalidateCallRequestWidgetQueries", () => {
  const isInvalidated = (qc: QueryClient, key: readonly unknown[]): boolean =>
    qc.getQueryState(key)?.isInvalidated === true;

  it("marks only the dashboard widgets that list call requests as stale", async () => {
    const qc = new QueryClient();
    const myCalls = [KEY, "my-calls", "call_request", { a: 1 }, 5, 0, undefined];
    const callsToAttend = [KEY, "calls-to-attend", "call_request", { b: 2 }, 10, 0, undefined];
    const casesWidget = [KEY, "my-cases", "case", { a: 1 }, 5, 0, undefined];
    const differentlyShaped = [KEY, "pie-slice", "w", "call_request"];
    const caseCallRequests = [ApiQueryKeys.CASE_CALL_REQUESTS, "case-1", []];
    for (const key of [myCalls, callsToAttend, casesWidget, differentlyShaped, caseCallRequests]) {
      seedQuery(qc, key);
    }

    await invalidateCallRequestWidgetQueries(qc);

    expect(isInvalidated(qc, myCalls)).toBe(true);
    expect(isInvalidated(qc, callsToAttend)).toBe(true);
    // Nothing else is reloaded: another resource's widget, a key whose third slot is
    // not the resource type, and the case-scoped call request list (which the
    // mutation hook invalidates for itself).
    expect(isInvalidated(qc, casesWidget)).toBe(false);
    expect(isInvalidated(qc, differentlyShaped)).toBe(false);
    expect(isInvalidated(qc, caseCallRequests)).toBe(false);
  });

  it("is a no-op when no dashboard has been opened yet", async () => {
    await expect(invalidateCallRequestWidgetQueries(new QueryClient())).resolves.toBeUndefined();
  });
});
