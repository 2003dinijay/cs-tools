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
import { getDeactivationState } from "./csmAccounts";

const NOW = new Date("2026-07-31T00:00:00Z");

describe("getDeactivationState", () => {
  it("returns 'none' when the date is absent", () => {
    expect(getDeactivationState(undefined, NOW)).toBe("none");
    expect(getDeactivationState(null, NOW)).toBe("none");
    expect(getDeactivationState("", NOW)).toBe("none");
  });

  it("returns 'none' for an unparseable date string", () => {
    expect(getDeactivationState("not-a-date", NOW)).toBe("none");
  });

  it("returns 'past' for a date strictly before now", () => {
    expect(getDeactivationState("2026-01-01T00:00:00Z", NOW)).toBe("past");
  });

  it("returns 'future' for a date at or after now", () => {
    expect(getDeactivationState("2027-01-01T00:00:00Z", NOW)).toBe("future");
  });

  it("treats the exact boundary instant as 'future', not 'past'", () => {
    expect(getDeactivationState(NOW.toISOString(), NOW)).toBe("future");
  });
});
