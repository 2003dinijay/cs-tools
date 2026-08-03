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
import type { CasesFilters } from "@features/csm-cases/components/CasesFilterBar";
import { DEFAULT_CASES_FILTERS } from "@features/csm-cases/utils/casesFiltersUrl";
import { buildCaseSearchFilters } from "./caseSearchPayload";

function filterOf(filters: CasesFilters, field: string) {
  return (buildCaseSearchFilters(filters, "", undefined).filters ?? []).filter(
    (f) => f.field === field,
  );
}

describe("buildCaseSearchFilters — new advanced-filter fields", () => {
  it("emits integrationCsTeam op:in for csTeams", () => {
    const filters: CasesFilters = { ...DEFAULT_CASES_FILTERS, csTeams: ["team-a"] };
    expect(filterOf(filters, "integrationCsTeam")).toEqual([
      { field: "integrationCsTeam", op: "in", values: ["team-a"] },
    ]);
  });

  it("emits tag op:in and tag op:notIn as two independent entries", () => {
    const filters: CasesFilters = {
      ...DEFAULT_CASES_FILTERS,
      tags: ["patch"],
      excludeTags: ["s_dip"],
    };
    const tagEntries = filterOf(filters, "tag");
    expect(tagEntries).toEqual([
      { field: "tag", op: "in", values: ["patch"] },
      { field: "tag", op: "notIn", values: ["s_dip"] },
    ]);
  });

  it("does NOT invert excludeTags into an `in` entry", () => {
    // Regression: the equivalent bug in widgetPreviewUrl.ts inverted `tag
    // notIn` into `tag in` because it dropped the op. This asserts the
    // payload builder never produces an `in` entry when only excludeTags is
    // set.
    const filters: CasesFilters = { ...DEFAULT_CASES_FILTERS, excludeTags: ["s_dip"] };
    const tagEntries = filterOf(filters, "tag");
    expect(tagEntries).toEqual([{ field: "tag", op: "notIn", values: ["s_dip"] }]);
    expect(tagEntries?.some((e) => e.op === "in")).toBe(false);
  });

  it("emits projectOnboardingStatus op:in", () => {
    const filters: CasesFilters = {
      ...DEFAULT_CASES_FILTERS,
      onboardingStatuses: ["in_progress"],
    };
    expect(filterOf(filters, "projectOnboardingStatus")).toEqual([
      { field: "projectOnboardingStatus", op: "in", values: ["in_progress"] },
    ]);
  });

  it("emits taskSLABusinessElapsedPercent gte and lte as separate entries", () => {
    const filters: CasesFilters = {
      ...DEFAULT_CASES_FILTERS,
      slaElapsedPctGte: 50,
      slaElapsedPctLte: 100,
    };
    expect(filterOf(filters, "taskSLABusinessElapsedPercent")).toEqual([
      { field: "taskSLABusinessElapsedPercent", op: "gte", values: ["50"] },
      { field: "taskSLABusinessElapsedPercent", op: "lte", values: ["100"] },
    ]);
  });

  it("emits escalation isNotEmpty with no `values` when hasEscalation is true", () => {
    const filters: CasesFilters = { ...DEFAULT_CASES_FILTERS, hasEscalation: true };
    expect(filterOf(filters, "escalation")).toEqual([
      { field: "escalation", op: "isNotEmpty" },
    ]);
  });

  it("emits escalation isEmpty with no `values` when hasEscalation is false", () => {
    const filters: CasesFilters = { ...DEFAULT_CASES_FILTERS, hasEscalation: false };
    expect(filterOf(filters, "escalation")).toEqual([{ field: "escalation", op: "isEmpty" }]);
  });

  it("omits escalation entirely when hasEscalation is null (unfiltered)", () => {
    const filters: CasesFilters = { ...DEFAULT_CASES_FILTERS, hasEscalation: null };
    expect(filterOf(filters, "escalation")).toEqual([]);
  });

  it("emits escalationLevel and projectType op:in", () => {
    const filters: CasesFilters = {
      ...DEFAULT_CASES_FILTERS,
      escalationLevels: ["L1"],
      projectTypes: ["enterprise"],
    };
    expect(filterOf(filters, "escalationLevel")).toEqual([
      { field: "escalationLevel", op: "in", values: ["L1"] },
    ]);
    expect(filterOf(filters, "projectType")).toEqual([
      { field: "projectType", op: "in", values: ["enterprise"] },
    ]);
  });

  it("emits createdOn/updatedOn/closedOn gte+lte as independent entries per field", () => {
    const filters: CasesFilters = {
      ...DEFAULT_CASES_FILTERS,
      createdOnGte: "2026-01-01",
      createdOnLte: "2026-03-31",
      updatedOnGte: "2026-02-01",
      closedOnLte: "2026-06-30",
    };
    expect(filterOf(filters, "createdOn")).toEqual([
      { field: "createdOn", op: "gte", values: ["2026-01-01"] },
      { field: "createdOn", op: "lte", values: ["2026-03-31"] },
    ]);
    expect(filterOf(filters, "updatedOn")).toEqual([
      { field: "updatedOn", op: "gte", values: ["2026-02-01"] },
    ]);
    expect(filterOf(filters, "closedOn")).toEqual([
      { field: "closedOn", op: "lte", values: ["2026-06-30"] },
    ]);
  });

  it("emits nothing beyond the default filters when all new fields are unset", () => {
    const result = buildCaseSearchFilters(DEFAULT_CASES_FILTERS, "", undefined);
    expect(result.filters).toBeUndefined();
  });
});
