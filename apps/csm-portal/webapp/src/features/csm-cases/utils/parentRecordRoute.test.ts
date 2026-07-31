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
import { parentRecordPath } from "@features/csm-cases/utils/parentRecordRoute";

const PARENT_ID = "11111111-1111-1111-1111-111111111111";

describe("parentRecordPath", () => {
  it("routes a case parent to the case detail page", () => {
    expect(parentRecordPath({ id: PARENT_ID, type: "case" })).toBe(
      `/cases/${PARENT_ID}`,
    );
  });

  it("routes an incident parent to the incident detail page", () => {
    expect(parentRecordPath({ id: PARENT_ID, type: "incident" })).toBe(
      `/operations/incidents/${PARENT_ID}`,
    );
  });

  it("routes a change-request parent to the change-request detail page", () => {
    expect(parentRecordPath({ id: PARENT_ID, type: "change_request" })).toBe(
      `/operations/change-requests/${PARENT_ID}`,
    );
  });

  it("routes a problem parent to the problem detail page", () => {
    expect(parentRecordPath({ id: PARENT_ID, type: "problem" })).toBe(
      `/operations/problems/${PARENT_ID}`,
    );
  });

  // The cases below are the whole point of the null return: a parent whose kind
  // the backend could not resolve must NOT be guessed at as a case, because
  // linking to /cases/{id} for an incident or change request lands on a 404.
  it("returns null when the parent's type is absent", () => {
    expect(parentRecordPath({ id: PARENT_ID })).toBeNull();
  });

  it("returns null when the parent's type is null", () => {
    expect(parentRecordPath({ id: PARENT_ID, type: null })).toBeNull();
  });

  it("returns null for a record kind the portal has no page for", () => {
    expect(parentRecordPath({ id: PARENT_ID, type: "service_task" })).toBeNull();
  });

  it("returns null when there is no parent at all", () => {
    expect(parentRecordPath(undefined)).toBeNull();
  });
});
