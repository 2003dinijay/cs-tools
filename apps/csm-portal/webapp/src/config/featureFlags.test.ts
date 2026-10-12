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

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  enabledNavChildren,
  featureState,
  featureStateForPath,
  firstEnabledDestination,
  isFeatureEnabled,
  isFeatureVisible,
  listPreviewableFeatures,
  navigableNavNodes,
  resetFeatureStatesForTests,
  visibleNavChildren,
  visibleNavSections,
} from "@config/featureFlags";
import { CSM_NAV_ITEMS, navNodeById } from "@config/csmNavItems";
import { getPortalAccess } from "@context/current-user/portalAccess";

function setOverrides(value: unknown): void {
  window.config = {
    ...window.config,
    CSM_PORTAL_FEATURE_OVERRIDES: value,
  } as Window["config"];
  resetFeatureStatesForTests();
}

beforeEach(() => {
  vi.spyOn(console, "warn").mockImplementation(() => undefined);
  setOverrides(undefined);
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("featureState", () => {
  it("treats anything absent from the config as a working feature", () => {
    expect(featureState("operations")).toBe("enabled");
    expect(featureState("operations.incidents")).toBe("enabled");
    expect(featureState("admin.user-management.roles")).toBe("enabled");
  });

  it("treats an id that isn't in the nav tree as enabled", () => {
    expect(featureState("does.not.exist")).toBe("enabled");
  });

  it("applies an explicit override", () => {
    setOverrides({ updates: "hidden", engagements: "wip" });
    expect(featureState("updates")).toBe("hidden");
    expect(featureState("engagements")).toBe("wip");
    expect(featureState("dashboard")).toBe("enabled");
  });

  it("cascades a WIP section onto its tabs", () => {
    setOverrides({ "security-center": "wip" });
    expect(featureState("security-center.reports")).toBe("wip");
    expect(featureState("security-center.vulnerabilities")).toBe("wip");
  });

  it("lets a tab opt out of its section's WIP state", () => {
    setOverrides({ operations: "wip", "operations.incidents": "enabled" });
    expect(featureState("operations.incidents")).toBe("enabled");
    expect(featureState("operations.problems")).toBe("wip");
  });

  it("promotes a WIP section that has a usable tab, so the tab is reachable", () => {
    setOverrides({ operations: "wip", "operations.incidents": "enabled" });
    expect(featureState("operations")).toBe("enabled");
  });

  it("leaves a WIP section disabled when every tab is restricted", () => {
    setOverrides({ operations: "wip" });
    expect(featureState("operations")).toBe("wip");
  });

  it("hides a section's whole subtree, overriding any tab that opts out", () => {
    setOverrides({ operations: "hidden", "operations.incidents": "enabled" });
    expect(featureState("operations")).toBe("hidden");
    expect(featureState("operations.incidents")).toBe("hidden");
  });
});

describe("override parsing", () => {
  it("accepts the map as a JSON string, for string-only config injection", () => {
    setOverrides(JSON.stringify({ "admin.user-management.roles": "hidden" }));
    expect(featureState("admin.user-management.roles")).toBe("hidden");
  });

  it("ignores a malformed JSON string and warns", () => {
    setOverrides("{not json");
    expect(featureState("admin.user-management.roles")).toBe("enabled");
    expect(console.warn).toHaveBeenCalled();
  });

  it("ignores an unknown page id and warns", () => {
    setOverrides({ "operations.typo": "hidden" });
    expect(featureState("operations.typo")).toBe("enabled");
    expect(console.warn).toHaveBeenCalledWith(
      expect.stringContaining("operations.typo"),
    );
  });

  it("ignores an unknown state and warns", () => {
    setOverrides({ updates: "disabled" });
    expect(featureState("updates")).toBe("enabled");
    expect(console.warn).toHaveBeenCalledWith(expect.stringContaining("updates"));
  });

  it("ignores a non-object override value", () => {
    setOverrides(["updates"]);
    expect(featureState("updates")).toBe("enabled");
  });
});

describe("featureStateForPath", () => {
  it("resolves a tab's deep link to the tab, not to its section", () => {
    setOverrides({ operations: "wip", "operations.incidents": "enabled" });
    expect(featureStateForPath("/operations/incidents")).toBe("enabled");
    expect(featureStateForPath("/operations/incidents/INC0001")).toBe("enabled");
    expect(featureStateForPath("/operations/problems/PRB0001")).toBe("wip");
  });

  it("resolves a section's own landing route to the section", () => {
    setOverrides({ operations: "wip" });
    expect(featureStateForPath("/operations")).toBe("wip");
  });

  it("resolves a route-backed tab and its detail pages", () => {
    setOverrides({ "customers.projects": "hidden" });
    expect(featureStateForPath("/customers/projects")).toBe("hidden");
    expect(featureStateForPath("/customers/projects/abc")).toBe("hidden");
    expect(featureStateForPath("/customers/accounts")).toBe("enabled");
  });

  it("leaves an unrecognised route alone for the 404 handler", () => {
    expect(featureStateForPath("/nothing-here")).toBe("enabled");
  });
});

describe("navigation helpers", () => {
  it("keeps WIP sections in the sidebar but drops hidden ones", () => {
    setOverrides({ updates: "hidden", engagements: "wip" });
    const ids = visibleNavSections().map((section) => section.id);
    expect(ids).not.toContain("updates");
    expect(ids).toContain("engagements");
  });

  it("separates visible tabs from usable ones", () => {
    setOverrides({
      "admin.user-management.roles": "wip",
      "admin.user-management.groups": "hidden",
    });
    const userManagement = navNodeById("admin.user-management");
    expect(userManagement).toBeDefined();
    const visible = visibleNavChildren(userManagement!).map((child) => child.id);
    const enabled = enabledNavChildren(userManagement!).map((child) => child.id);
    expect(visible).toContain("admin.user-management.roles");
    expect(visible).not.toContain("admin.user-management.groups");
    expect(enabled).not.toContain("admin.user-management.roles");
    expect(enabled).toContain("admin.user-management.users");
  });

  it("offers only usable destinations to the quick-nav palette", () => {
    setOverrides({ operations: "wip", "operations.incidents": "enabled" });
    const ids = navigableNavNodes().map((node) => node.id);
    expect(ids).toContain("operations");
    expect(ids).toContain("operations.incidents");
    expect(ids).not.toContain("operations.problems");
  });

  it("labels a tab destination with its owning section", () => {
    const incidents = navigableNavNodes().find(
      (node) => node.id === "operations.incidents",
    );
    expect(incidents?.label).toBe("Incidents");
    expect(incidents?.sublabel).toBe("Operations");
    expect(incidents?.href).toBe("/operations?tab=incidents");
  });

  it("flattens a grandchild tab too, labelled by its immediate parent", () => {
    const users = navigableNavNodes().find(
      (node) => node.id === "admin.user-management.users",
    );
    expect(users?.label).toBe("Users");
    expect(users?.sublabel).toBe("User management");
    expect(users?.href).toBe("/admin/user-management/users");

    const userManagement = navigableNavNodes().find(
      (node) => node.id === "admin.user-management",
    );
    expect(userManagement?.sublabel).toBe("Settings");
  });
});

describe("firstEnabledDestination", () => {
  it("defaults to the first section in the tree", () => {
    expect(firstEnabledDestination()).toBe("/dashboard");
  });

  it("skips restricted sections so a hidden page never redirects into one", () => {
    setOverrides({ dashboard: "hidden", support: "wip" });
    expect(firstEnabledDestination()).toBe("/operations");
  });

  it("returns undefined when the config leaves nothing reachable", () => {
    const everything = Object.fromEntries(
      CSM_NAV_ITEMS.map((section) => [section.id, "hidden"]),
    );
    setOverrides(everything);
    expect(firstEnabledDestination()).toBeUndefined();
  });
});

describe("isFeatureVisible / isFeatureEnabled", () => {
  it("distinguishes hidden, WIP and enabled", () => {
    setOverrides({ updates: "hidden", engagements: "wip" });
    expect(isFeatureVisible("updates")).toBe(false);
    expect(isFeatureVisible("engagements")).toBe(true);
    expect(isFeatureEnabled("engagements")).toBe(false);
    expect(isFeatureEnabled("dashboard")).toBe(true);
  });
});

describe("featureState preview overrides", () => {
  const admin = getPortalAccess(["admin"]);
  const csEngineer = getPortalAccess(["cs_engineer"]);

  it("promotes a deployment-hidden node to enabled for an admin with the override on", () => {
    setOverrides({ updates: "hidden" });
    expect(featureState("updates", admin)).toBe("hidden");
    expect(featureState("updates", admin, { updates: true })).toBe("enabled");
  });

  it("promotes a deployment-wip node to enabled for an admin with the override on", () => {
    setOverrides({ engagements: "wip" });
    expect(featureState("engagements", admin)).toBe("wip");
    expect(featureState("engagements", admin, { engagements: true })).toBe("enabled");
  });

  it("does not promote for a non-admin, even with the override on", () => {
    setOverrides({ updates: "hidden" });
    expect(featureState("updates", csEngineer, { updates: true })).toBe("hidden");
  });

  it("does not promote when the override for that id is off or absent", () => {
    setOverrides({ updates: "hidden" });
    expect(featureState("updates", admin, { updates: false })).toBe("hidden");
    expect(featureState("updates", admin, {})).toBe("hidden");
    expect(featureState("updates", admin, { engagements: true })).toBe("hidden");
  });

  it("never bypasses a genuine capability failure, even for an admin with the override on", () => {
    // operations.incidents requires canUseOperations; a plain viewer lacks it
    // regardless of what the deployment config or any override says.
    const viewerAdminish = { ...getPortalAccess(["viewer"]), isAdmin: true };
    expect(featureState("operations.incidents", viewerAdminish, { "operations.incidents": true })).toBe(
      "hidden",
    );
  });

  it("never demotes an already-enabled node", () => {
    expect(featureState("dashboard", admin, { dashboard: false })).toBe("enabled");
  });

  it("leaves every other caller (no third argument) completely unaffected", () => {
    setOverrides({ updates: "hidden", engagements: "wip" });
    expect(featureState("updates", admin)).toBe("hidden");
    expect(featureState("engagements", admin)).toBe("wip");
    expect(featureState("dashboard")).toBe("enabled");
  });
});

describe("listPreviewableFeatures", () => {
  it("returns only nodes that are hidden or WIP for this deployment, without applying any override", () => {
    setOverrides({ updates: "hidden", engagements: "wip" });
    const ids = listPreviewableFeatures().map((n) => n.id);
    expect(ids).toContain("updates");
    expect(ids).toContain("engagements");
    expect(ids).not.toContain("dashboard");
  });

  it("includes a config-hidden node for a caller whose capability would allow it", () => {
    setOverrides({ "operations.incidents": "hidden" });
    const csEngineerIds = listPreviewableFeatures(getPortalAccess(["cs_engineer"])).map(
      (n) => n.id,
    );
    expect(csEngineerIds).toContain("operations.incidents");
  });

  it("returns an empty array when nothing is hidden or WIP", () => {
    expect(listPreviewableFeatures()).toEqual([]);
  });
});
