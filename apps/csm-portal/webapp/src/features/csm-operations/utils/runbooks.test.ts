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
import { findRunbookForIncident, parseRunbookImport, runbooksToCsvRows, runbooksToImportJson } from "./runbooks";
import {
  activeRunbookFilterCount,
  cleanAlertNames,
  DEFAULT_RUNBOOK_FILTERS,
  filterRunbooks,
  findRunbookForIncident,
  parseRunbookImport,
  type Runbook,
} from "./runbooks";

describe("parseRunbookImport", () => {
  it("reads an object whose keys are comma-separated alert names, keeping spaces inside a name", () => {
    expect(
      parseRunbookImport('{"Alert 1, Alert 2 ,alert 1": "https://docs.example.com/a", " Alert B ": ""}'),
    ).toEqual([
      { runbookUrl: "https://docs.example.com/a", alerts: ["Alert 1", "Alert 2"] },
      { runbookUrl: null, alerts: ["Alert B"] },
    ]);
  });

  it("reads a list of runbooks with a list or a comma-separated string of alerts, ignoring a name", () => {
    expect(
      parseRunbookImport('[{"name": "Gateway", "runbookUrl": null, "alerts": ["a", "b"]}, {"alerts": "c, d"}]'),
    ).toEqual([
      { runbookUrl: null, alerts: ["a", "b"] },
      { runbookUrl: null, alerts: ["c", "d"] },
    ]);
  });

  it("still reads an older list entry with one alertName", () => {
    expect(parseRunbookImport('[{"alertName": "A", "runbookUrl": null}]')).toEqual([
      { runbookUrl: null, alerts: ["A"] },
    ]);
  });

  it("reads an older alertName with a comma as one alert, never splitting it", () => {
    expect(parseRunbookImport('[{"alertName": "CPU > 90%, prod", "runbookUrl": null}]')).toEqual([
      { runbookUrl: null, alerts: ["CPU > 90%, prod"] },
    ]);
  });

  it("refuses bad JSON, entries with no alert and links that are not http(s)", () => {
    expect(() => parseRunbookImport("{")).toThrow("not valid JSON");
    expect(() => parseRunbookImport('[{"alerts": [" "]}]')).toThrow("Entry 1 needs at least one alert name");
    expect(() => parseRunbookImport('{" , ": "https://x.example"}')).toThrow("at least one alert name");
    expect(() => parseRunbookImport('{"A": "javascript:alert(1)"}')).toThrow("http(s)");
  });
});

describe("cleanAlertNames", () => {
  it("trims and drops blanks and repeats, ignoring case", () => {
    expect(cleanAlertNames([" a ", "", "A", "b"])).toEqual(["a", "b"]);
  });
});

describe("findRunbookForIncident", () => {
  const catalog = [
    { number: "RB0000001", runbookUrl: "https://docs.example.com/gw", alerts: ["gw-5xx", "gw-latency"] },
    { number: "RB0000002", runbookUrl: "https://docs.example.com/long", alerts: ["gw-5xx-prod-001"] },
    { number: "RB0000003", runbookUrl: null, alerts: ["JVM Heap"] },
  ];

  it("picks the runbook holding the longest alert name the subject contains, ignoring case", () => {
    const match = findRunbookForIncident(catalog, "Fired GW-5XX-PROD-001 on log");
    expect(match?.runbook.number).toBe("RB0000002");
    expect(match?.alertName).toBe("gw-5xx-prod-001");
    expect(findRunbookForIncident(catalog, "gw-latency spike")?.runbook.number).toBe("RB0000001");
  });

  it("finds a runbook without a link, and nothing for an unknown subject", () => {
    expect(findRunbookForIncident(catalog, "node / jvm heap")?.runbook.runbookUrl).toBeNull();
    expect(findRunbookForIncident(catalog, "unrelated")).toBeUndefined();
  });
});

describe("export", () => {
  const catalog = [
    { id: "1", alertName: "gw-5xx", runbookUrl: "https://docs.example.com/gw", createdOn: "", createdBy: "", updatedOn: "2026-10-01T00:00:00Z", updatedBy: "a@x" },
    { id: "2", alertName: "JVM Heap", runbookUrl: null, createdOn: "", createdBy: "", updatedOn: "2026-10-02T00:00:00Z", updatedBy: "b@x" },
  ];

  it("writes JSON the importer reads back unchanged", () => {
    expect(parseRunbookImport(runbooksToImportJson(catalog))).toEqual([
      { alertName: "gw-5xx", runbookUrl: "https://docs.example.com/gw" },
      { alertName: "JVM Heap", runbookUrl: null },
    ]);
  });

  it("keeps an alert named __proto__ when exporting", () => {
    const json = runbooksToImportJson([{ alertName: "__proto__", runbookUrl: "https://docs.example.com/p" }]);
    expect(parseRunbookImport(json)).toEqual([{ alertName: "__proto__", runbookUrl: "https://docs.example.com/p" }]);
  });

  it("writes one CSV row per runbook", () => {
    expect(runbooksToCsvRows(catalog)).toEqual([
      ["gw-5xx", "https://docs.example.com/gw", "a@x", "2026-10-01T00:00:00Z"],
      ["JVM Heap", "", "b@x", "2026-10-02T00:00:00Z"],
    ]);
describe("filterRunbooks", () => {
  const now = Date.parse("2026-10-20T00:00:00Z");
  const rb = (id: string, over: Partial<Runbook>): Runbook => ({
    id,
    number: `RB-${id}`,
    runbookUrl: null,
    alerts: ["a"],
    createdOn: "2026-01-01T00:00:00Z",
    createdBy: "x@wso2.com",
    updatedOn: "2026-10-19T00:00:00Z",
    updatedBy: "x@wso2.com",
    ...over,
  });
  const catalog = [
    rb("gw", { runbookUrl: "https://d/gw", alerts: ["gw-5xx", "gw-latency"] }),
    rb("jvm", { alerts: ["JVM Heap"], updatedOn: "2026-09-01T00:00:00Z" }),
    rb("disk", { runbookUrl: "https://d/disk", alerts: ["disk"], updatedOn: "2026-05-01T00:00:00Z" }),
  ];
  const ids = (filters: Partial<typeof DEFAULT_RUNBOOK_FILTERS>, search = "") =>
    filterRunbooks(catalog, search, { ...DEFAULT_RUNBOOK_FILTERS, ...filters }, now).map((r) => r.id);

  it("filters by link, alert count and age", () => {
    expect(ids({})).toEqual(["gw", "jvm", "disk"]);
    expect(ids({ coverage: "linked" })).toEqual(["gw", "disk"]);
    expect(ids({ coverage: "missing" })).toEqual(["jvm"]);
    expect(ids({ alertCount: "single" })).toEqual(["jvm", "disk"]);
    expect(ids({ alertCount: "multiple" })).toEqual(["gw"]);
    expect(ids({ updatedWithin: "7" })).toEqual(["gw"]);
    expect(ids({ updatedWithin: "90" })).toEqual(["gw", "jvm"]);
    expect(ids({ coverage: "linked", alertCount: "single" })).toEqual(["disk"]);
  });

  it("searches number, alert names and link together with the filters", () => {
    expect(ids({}, "rb-jvm")).toEqual(["jvm"]);
    expect(ids({}, "LATENCY")).toEqual(["gw"]);
    expect(ids({}, "d/disk")).toEqual(["disk"]);
    expect(ids({ coverage: "missing" }, "gw")).toEqual([]);
  });

  it("counts the filters that differ from the default", () => {
    expect(activeRunbookFilterCount(DEFAULT_RUNBOOK_FILTERS)).toBe(0);
    expect(activeRunbookFilterCount({ ...DEFAULT_RUNBOOK_FILTERS, alertCount: "multiple", updatedWithin: "7" })).toBe(2);
  });
});
