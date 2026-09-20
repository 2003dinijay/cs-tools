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
import { beforeEach, describe, expect, it } from "vitest";
import { navNodeById } from "@config/csmNavItems";
import {
  featureState,
  featureStateForPath,
  firstEnabledDestination,
  navigableNavNodes,
  resetFeatureStatesForTests,
  visibleNavChildren,
  visibleNavSections,
} from "@config/featureFlags";
import { getPortalAccess } from "@context/current-user/portalAccess";

const viewer = getPortalAccess(["viewer"]);
const supportEngineer = getPortalAccess(["support_engineer"]);

describe("feature visibility by portal access", () => {
  beforeEach(() => {
    resetFeatureStatesForTests();
  });

  it("shows Operations to a support engineer and to callers with no access filter", () => {
    expect(visibleNavSections().map((s) => s.id)).toContain("operations");
    expect(visibleNavSections(supportEngineer).map((s) => s.id)).toContain("operations");
  });

  it("hides Operations from a view-only role, and keeps the other sections", () => {
    const ids = visibleNavSections(viewer).map((s) => s.id);
    expect(ids).not.toContain("operations");
    expect(ids.length).toBe(visibleNavSections().length - 1);
  });

  it("hides every Operations tab and route along with the section", () => {
    const operations = navNodeById("operations");
    expect(operations).toBeDefined();
    expect(visibleNavChildren(operations!, viewer)).toEqual([]);
    expect(featureState("operations.incidents", viewer)).toBe("hidden");
    expect(featureStateForPath("/operations/incidents", viewer)).toBe("hidden");
    expect(featureStateForPath("/operations/incidents/INC0001", viewer)).toBe("hidden");
    expect(featureStateForPath("/operations/incidents", supportEngineer)).toBe("enabled");
  });

  it("drops Operations from quick-nav for a viewer and redirects somewhere else", () => {
    expect(navigableNavNodes(viewer).some((n) => n.id.startsWith("operations"))).toBe(false);
    expect(navigableNavNodes(supportEngineer).some((n) => n.id.startsWith("operations"))).toBe(true);
    const fallback = firstEnabledDestination(viewer);
    expect(fallback).toBeDefined();
    expect(fallback).not.toMatch(/^\/operations/);
  });

  it("every role that is not full access is treated as a viewer for Operations", () => {
    for (const role of ["commenter", "escalator", "attachment_downloader", "usage_metrics_viewer"]) {
      expect(featureState("operations", getPortalAccess([role]))).toBe("hidden");
    }
    expect(featureState("operations", getPortalAccess(["admin"]))).toBe("enabled");
  });
});
