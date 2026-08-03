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
 * Regression coverage for the empty-array filter guard: a DSL entry with
 * `values: []` must leave the corresponding `CasesFilters` field unset
 * rather than setting an explicit empty filter (which the cases list would
 * treat as "match nothing" instead of "no constraint") — see the
 * CodeRabbit finding this closes.
 */

import { describe, expect, it } from "vitest";
import { WIDGET_RESOURCE_CONFIG } from "@features/csm-dashboard/config/widgetResourceConfig";

function hrefParams(href: string): URLSearchParams {
  const [, qs] = href.split("?");
  return new URLSearchParams(qs ?? "");
}

describe("WIDGET_RESOURCE_CONFIG.case.buildHref", () => {
  it("omits states/severities/types/products from the href when the DSL entry's values are empty", () => {
    const href = WIDGET_RESOURCE_CONFIG.case.buildHref({
      filters: [
        { field: "state", op: "in", values: [] },
        { field: "severity", op: "in", values: [] },
        { field: "type", op: "in", values: [] },
        { field: "product", op: "in", values: [] },
      ],
    });

    const params = hrefParams(href);
    expect(params.has("states")).toBe(false);
    expect(params.has("severities")).toBe(false);
    expect(params.has("types")).toBe(false);
    expect(params.has("products")).toBe(false);
  });

  it("still sets each field when the DSL entry carries real values", () => {
    const href = WIDGET_RESOURCE_CONFIG.case.buildHref({
      filters: [
        { field: "state", op: "in", values: ["open"] },
        { field: "severity", op: "in", values: ["critical"] },
        { field: "type", op: "in", values: ["case"] },
        { field: "product", op: "in", values: ["API Manager"] },
      ],
    });

    const params = hrefParams(href);
    expect(params.get("states")).toBe("open");
    expect(params.get("types")).toBe("case");
    expect(params.get("products")).toBe("API Manager");
    // Severity is remapped from the dashboard label to the case-list's own
    // S-code, so just assert it was set at all (severity-mapping specifics
    // aren't this fix's concern).
    expect(params.has("severities")).toBe(true);
  });

  it("carries engagementType and workState through to the cases list (previously dropped)", () => {
    // Regression: a case widget filtering by engagementType (e.g. "Engagements
    // In Progress") clicked through to an unfiltered cases list, because this
    // mapping didn't exist at all -- not a translation bug, a missing one.
    const href = WIDGET_RESOURCE_CONFIG.case.buildHref({
      filters: [
        { field: "engagementType", op: "in", values: ["migration", "onboarding"] },
        { field: "workState", op: "in", values: ["paused"] },
      ],
    });

    const params = hrefParams(href);
    expect(params.get("engagementTypes")).toBe("migration,onboarding");
    expect(params.get("workStates")).toBe("paused");
  });

  it("omits engagementTypes/workStates when the DSL entry's values are empty", () => {
    const href = WIDGET_RESOURCE_CONFIG.case.buildHref({
      filters: [
        { field: "engagementType", op: "in", values: [] },
        { field: "workState", op: "in", values: [] },
      ],
    });

    const params = hrefParams(href);
    expect(params.has("engagementTypes")).toBe(false);
    expect(params.has("workStates")).toBe(false);
  });
});
