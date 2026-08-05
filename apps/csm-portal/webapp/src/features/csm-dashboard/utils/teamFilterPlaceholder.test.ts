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
import {
  CURRENT_TEAM_PLACEHOLDER,
  resolveTeamPlaceholder,
} from "./teamFilterPlaceholder";

describe("resolveTeamPlaceholder", () => {
  it("substitutes the placeholder with the selected team's groupId when one is available", () => {
    const filters = {
      filters: [
        { field: "state", op: "in", values: ["open"] },
        { field: "integrationCsTeam", op: "in", values: [CURRENT_TEAM_PLACEHOLDER] },
      ],
    };

    const resolved = resolveTeamPlaceholder(filters, "22222222-2222-2222-2222-222222222222");

    expect(resolved).toEqual({
      filters: [
        { field: "state", op: "in", values: ["open"] },
        {
          field: "integrationCsTeam",
          op: "in",
          values: ["22222222-2222-2222-2222-222222222222"],
        },
      ],
    });
  });

  it("drops the integrationCsTeam entry entirely when no groupId is available", () => {
    const filters = {
      filters: [
        { field: "state", op: "in", values: ["open"] },
        { field: "integrationCsTeam", op: "in", values: [CURRENT_TEAM_PLACEHOLDER] },
      ],
    };

    const resolved = resolveTeamPlaceholder(filters, undefined);

    expect(resolved).toEqual({
      filters: [{ field: "state", op: "in", values: ["open"] }],
    });
  });

  it("only substitutes the placeholder entry within a values array, leaving other literal values alone", () => {
    const filters = {
      filters: [
        {
          field: "integrationCsTeam",
          op: "in",
          values: ["some-literal-group-id", CURRENT_TEAM_PLACEHOLDER],
        },
      ],
    };

    const resolved = resolveTeamPlaceholder(filters, "team-group-id");

    expect(resolved).toEqual({
      filters: [
        {
          field: "integrationCsTeam",
          op: "in",
          values: ["some-literal-group-id", "team-group-id"],
        },
      ],
    });
  });

  it("returns filters unchanged when there's no integrationCsTeam entry at all", () => {
    const filters = { filters: [{ field: "state", op: "in", values: ["open"] }] };

    expect(resolveTeamPlaceholder(filters, "team-group-id")).toEqual(filters);
    expect(resolveTeamPlaceholder(filters, undefined)).toEqual(filters);
  });

  it("returns non-case-filter-shaped filters (other resourceTypes) unchanged", () => {
    const filters = { states: ["open"], severities: ["critical"] };

    expect(resolveTeamPlaceholder(filters, "team-group-id")).toBe(filters);
  });

  it("passes through an empty filters array unchanged", () => {
    const filters = { filters: [] };

    expect(resolveTeamPlaceholder(filters, "team-group-id")).toBe(filters);
  });

  it("splices every groupId in an array into the placeholder's spot ('All ABTs')", () => {
    const filters = {
      filters: [
        { field: "state", op: "in", values: ["open"] },
        { field: "integrationCsTeam", op: "in", values: [CURRENT_TEAM_PLACEHOLDER] },
      ],
    };

    const resolved = resolveTeamPlaceholder(filters, ["group-a", "group-b", "group-c"]);

    expect(resolved).toEqual({
      filters: [
        { field: "state", op: "in", values: ["open"] },
        {
          field: "integrationCsTeam",
          op: "in",
          values: ["group-a", "group-b", "group-c"],
        },
      ],
    });
  });

  it("keeps a literal value alongside a spliced-in array, in position", () => {
    const filters = {
      filters: [
        {
          field: "integrationCsTeam",
          op: "in",
          values: ["some-literal-group-id", CURRENT_TEAM_PLACEHOLDER],
        },
      ],
    };

    const resolved = resolveTeamPlaceholder(filters, ["group-a", "group-b"]);

    expect(resolved).toEqual({
      filters: [
        {
          field: "integrationCsTeam",
          op: "in",
          values: ["some-literal-group-id", "group-a", "group-b"],
        },
      ],
    });
  });

  it("drops the integrationCsTeam entry entirely when given an empty array", () => {
    const filters = {
      filters: [
        { field: "state", op: "in", values: ["open"] },
        { field: "integrationCsTeam", op: "in", values: [CURRENT_TEAM_PLACEHOLDER] },
      ],
    };

    const resolved = resolveTeamPlaceholder(filters, []);

    expect(resolved).toEqual({
      filters: [{ field: "state", op: "in", values: ["open"] }],
    });
  });

  it("still substitutes a single string 1:1, same as before array support existed", () => {
    const filters = {
      filters: [
        { field: "integrationCsTeam", op: "in", values: [CURRENT_TEAM_PLACEHOLDER] },
      ],
    };

    const resolved = resolveTeamPlaceholder(filters, "single-group-id");

    expect(resolved).toEqual({
      filters: [{ field: "integrationCsTeam", op: "in", values: ["single-group-id"] }],
    });
  });
});
