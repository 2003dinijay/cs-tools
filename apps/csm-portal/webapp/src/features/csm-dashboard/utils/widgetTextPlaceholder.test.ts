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
import { resolveWidgetText } from "./widgetTextPlaceholder";

describe("resolveWidgetText", () => {
  it("resolves the token to a real team's display name", () => {
    expect(resolveWidgetText("Open Incidents — {{currentTeam}}", "Castor")).toBe(
      "Open Incidents — Castor",
    );
  });

  it("resolves the token to the literal 'All ABTs' string", () => {
    expect(resolveWidgetText("Open Incidents — {{currentTeam}}", "All ABTs")).toBe(
      "Open Incidents — All ABTs",
    );
  });

  it("strips the token to empty and trims a trailing em-dash separator when unresolved", () => {
    expect(resolveWidgetText("Cases — {{currentTeam}}", undefined)).toBe("Cases");
  });

  it("strips the token to empty and trims a trailing hyphen separator when unresolved", () => {
    expect(resolveWidgetText("Cases - {{currentTeam}}", undefined)).toBe("Cases");
  });

  it("strips the token to empty and trims a trailing pipe separator when unresolved", () => {
    expect(resolveWidgetText("Cases | {{currentTeam}}", undefined)).toBe("Cases");
  });

  it("returns text unchanged when it carries no token at all", () => {
    expect(resolveWidgetText("Open Incidents", "Castor")).toBe("Open Incidents");
    expect(resolveWidgetText("Open Incidents", undefined)).toBe("Open Incidents");
  });

  it("returns undefined when text itself is undefined", () => {
    expect(resolveWidgetText(undefined, "Castor")).toBeUndefined();
    expect(resolveWidgetText(undefined, undefined)).toBeUndefined();
  });

  it("substitutes every occurrence of the token when resolved", () => {
    expect(resolveWidgetText("{{currentTeam}} cases for {{currentTeam}}", "Castor")).toBe(
      "Castor cases for Castor",
    );
  });
});
