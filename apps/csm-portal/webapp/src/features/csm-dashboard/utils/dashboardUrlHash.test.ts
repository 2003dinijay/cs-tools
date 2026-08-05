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
import { buildDashboardHash, parseDashboardHash } from "./dashboardUrlHash";

describe("parseDashboardHash", () => {
  it("returns both fields undefined for an empty hash", () => {
    expect(parseDashboardHash("")).toEqual({});
    expect(parseDashboardHash("#")).toEqual({});
  });

  it("parses a dashboard-only hash, with or without the leading #", () => {
    expect(parseDashboardHash("#abt")).toEqual({ dashboardId: "abt" });
    expect(parseDashboardHash("abt")).toEqual({ dashboardId: "abt" });
  });

  it("parses a dot-separated dashboard.team hash", () => {
    expect(parseDashboardHash("#abt.atlas")).toEqual({
      dashboardId: "abt",
      teamId: "atlas",
    });
  });

  it("splits on the first dot only", () => {
    expect(parseDashboardHash("#abt.atlas.extra")).toEqual({
      dashboardId: "abt",
      teamId: "atlas.extra",
    });
  });

  it("treats a stray trailing dot as no team id, not an empty string", () => {
    expect(parseDashboardHash("#abt.")).toEqual({ dashboardId: "abt", teamId: undefined });
  });
});

describe("buildDashboardHash", () => {
  it("builds a dashboard-only fragment when no team is given", () => {
    expect(buildDashboardHash("abt")).toBe("#abt");
    expect(buildDashboardHash("abt", undefined)).toBe("#abt");
  });

  it("builds a dot-separated fragment when a team is given", () => {
    expect(buildDashboardHash("abt", "atlas")).toBe("#abt.atlas");
  });

  it("round-trips through parseDashboardHash", () => {
    expect(parseDashboardHash(buildDashboardHash("abt", "atlas"))).toEqual({
      dashboardId: "abt",
      teamId: "atlas",
    });
    expect(parseDashboardHash(buildDashboardHash("abt"))).toEqual({
      dashboardId: "abt",
    });
  });
});
