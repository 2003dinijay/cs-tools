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
  normalizePhone,
  platformPhoneState,
  savingRepairsPlatform,
} from "./profilePhone";

describe("normalizePhone", () => {
  it("drops formatting characters only", () => {
    expect(normalizePhone("+1 (555) 000-11.11")).toBe("+15550001111");
    expect(normalizePhone(null)).toBe("");
    expect(normalizePhone(undefined)).toBe("");
  });
});

describe("platformPhoneState", () => {
  it("claims nothing when the platform copy is unknown", () => {
    expect(platformPhoneState("+15550001111", undefined)).toEqual({ kind: "unknown" });
  });

  it("treats the same number, however formatted, as the same", () => {
    expect(platformPhoneState("+15550001111", "+1 555 000 1111")).toEqual({ kind: "same" });
    expect(platformPhoneState(undefined, null)).toEqual({ kind: "same" });
    expect(platformPhoneState("", "")).toEqual({ kind: "same" });
  });

  it("reports two different numbers", () => {
    expect(platformPhoneState("+15550001111", "+15559998888")).toEqual({
      kind: "differs",
      profile: "+15550001111",
      platform: "+15559998888",
    });
  });

  it("reports a platform record with no number", () => {
    expect(platformPhoneState("+15550001111", null)).toEqual({
      kind: "platformMissing",
      profile: "+15550001111",
    });
  });

  it("reports a platform number the profile does not show", () => {
    expect(platformPhoneState(undefined, "+15559998888")).toEqual({
      kind: "profileMissing",
      platform: "+15559998888",
    });
  });
});

describe("savingRepairsPlatform", () => {
  it("is true when the draft differs from the platform copy", () => {
    expect(savingRepairsPlatform("+15550001111", "+15559998888")).toBe(true);
    expect(savingRepairsPlatform("+15550001111", null)).toBe(true);
  });

  it("is false when the platform copy already holds the draft", () => {
    expect(savingRepairsPlatform("+15550001111", "+1 555 000 1111")).toBe(false);
  });

  it("never clears the platform copy and never acts on an unknown one", () => {
    expect(savingRepairsPlatform("  ", "+15559998888")).toBe(false);
    expect(savingRepairsPlatform("+15550001111", undefined)).toBe(false);
  });
});
