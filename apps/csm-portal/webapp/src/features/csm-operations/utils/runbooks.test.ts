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

describe("parseRunbookImport", () => {
  it("reads an object of alert name to link, blank meaning no runbook", () => {
    expect(parseRunbookImport('{"Alert A": "https://docs.example.com/a", " Alert B ": ""}')).toEqual([
      { alertName: "Alert A", runbookUrl: "https://docs.example.com/a" },
      { alertName: "Alert B", runbookUrl: null },
    ]);
  });

  it("reads a list of entries", () => {
    expect(parseRunbookImport('[{"alertName": "A", "runbookUrl": null}]')).toEqual([{ alertName: "A", runbookUrl: null }]);
  });

  it("refuses bad JSON, blank names and links that are not http(s)", () => {
    expect(() => parseRunbookImport("{")).toThrow("not valid JSON");
    expect(() => parseRunbookImport('[{"alertName": " "}]')).toThrow("alert name");
    expect(() => parseRunbookImport('{"A": "javascript:alert(1)"}')).toThrow("http(s)");
  });
});

describe("findRunbookForIncident", () => {
  const catalog = [
    { alertName: "gw-5xx", runbookUrl: "https://docs.example.com/short" },
    { alertName: "gw-5xx-prod-001", runbookUrl: "https://docs.example.com/long" },
    { alertName: "JVM Heap", runbookUrl: null },
  ];

  it("picks the longest alert name the subject contains, ignoring case", () => {
    expect(findRunbookForIncident(catalog, "Fired GW-5XX-PROD-001 on log")?.runbookUrl).toBe("https://docs.example.com/long");
  });

  it("finds a known alert without a link, and nothing for an unknown subject", () => {
    expect(findRunbookForIncident(catalog, "node / jvm heap")).toEqual({ alertName: "JVM Heap", runbookUrl: null });
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

  it("writes one CSV row per runbook", () => {
    expect(runbooksToCsvRows(catalog)).toEqual([
      ["gw-5xx", "https://docs.example.com/gw", "a@x", "2026-10-01T00:00:00Z"],
      ["JVM Heap", "", "b@x", "2026-10-02T00:00:00Z"],
    ]);
  });
});
