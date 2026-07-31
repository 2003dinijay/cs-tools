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
import { DEFAULT_INCIDENT_FILTERS } from "@features/csm-operations/utils/incidents";
import {
  readIncidentFiltersFromUrl,
  writeIncidentFiltersToUrl,
} from "./incidentsFiltersUrl";

describe("readIncidentFiltersFromUrl", () => {
  it("returns the defaults for an empty query string", () => {
    expect(readIncidentFiltersFromUrl(new URLSearchParams())).toEqual(
      DEFAULT_INCIDENT_FILTERS,
    );
  });

  it("parses a fully-populated query string", () => {
    const params = new URLSearchParams("incQ=timeout&incPriorities=HIGH,LOW");
    expect(readIncidentFiltersFromUrl(params)).toEqual({
      search: "timeout",
      priorities: ["HIGH", "LOW"],
    });
  });

  it("drops values outside the allowed priority enum", () => {
    const params = new URLSearchParams("incPriorities=HIGH,BOGUS");
    expect(readIncidentFiltersFromUrl(params).priorities).toEqual(["HIGH"]);
  });

  it("does not read the change-requests tab's own `cr...` params", () => {
    const params = new URLSearchParams("crQ=foo&crStates=implement");
    expect(readIncidentFiltersFromUrl(params)).toEqual(
      DEFAULT_INCIDENT_FILTERS,
    );
  });
});

describe("writeIncidentFiltersToUrl", () => {
  it("omits default-valued fields to keep the URL clean", () => {
    expect(
      writeIncidentFiltersToUrl(DEFAULT_INCIDENT_FILTERS).toString(),
    ).toBe("");
  });

  it("round-trips a non-default filter set", () => {
    const filters = { search: "timeout", priorities: ["HIGH" as const] };
    const round = readIncidentFiltersFromUrl(writeIncidentFiltersToUrl(filters));
    expect(round).toEqual(filters);
  });
});
