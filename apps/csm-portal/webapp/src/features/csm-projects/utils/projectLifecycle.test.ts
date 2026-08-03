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
  closureStatePresentation,
  endDateLabel,
  startDateLabel,
} from "./projectLifecycle";

describe("startDateLabel", () => {
  it("returns 'Started on' when createdOn and startDate are the same calendar day", () => {
    expect(
      startDateLabel("2021-09-15T10:23:00Z", "2021-09-15T00:00:00Z"),
    ).toBe("Started on");
  });

  it("returns 'Renewed on' when startDate is a later calendar day than createdOn", () => {
    expect(
      startDateLabel("2021-09-15T10:23:00Z", "2023-01-01T00:00:00Z"),
    ).toBe("Renewed on");
  });

  it("returns 'Started on' when startDate is earlier than createdOn (bad data)", () => {
    expect(
      startDateLabel("2023-01-01T10:23:00Z", "2021-09-15T00:00:00Z"),
    ).toBe("Started on");
  });

  it("returns 'Started on' when createdOn is missing", () => {
    expect(startDateLabel(null, "2021-09-15T00:00:00Z")).toBe("Started on");
  });

  it("returns 'Started on' when startDate is missing", () => {
    expect(startDateLabel("2021-09-15T10:23:00Z", null)).toBe("Started on");
  });

  it("returns 'Started on' when either value is unparseable", () => {
    expect(startDateLabel("not-a-date", "2021-09-15T00:00:00Z")).toBe(
      "Started on",
    );
    expect(startDateLabel("2021-09-15T10:23:00Z", "not-a-date")).toBe(
      "Started on",
    );
  });
});

describe("endDateLabel", () => {
  const now = new Date("2026-07-31T12:00:00Z");

  it("returns 'Ends on' for a future endDate", () => {
    expect(endDateLabel("2027-01-01T00:00:00Z", now)).toBe("Ends on");
  });

  it("returns 'Ended on' for a past endDate", () => {
    expect(endDateLabel("2020-01-01T00:00:00Z", now)).toBe("Ended on");
  });

  it("returns 'Ends on' when endDate is missing", () => {
    expect(endDateLabel(null, now)).toBe("Ends on");
    expect(endDateLabel(undefined, now)).toBe("Ends on");
  });

  it("returns 'Ends on' when endDate is unparseable", () => {
    expect(endDateLabel("not-a-date", now)).toBe("Ends on");
  });

  it("defaults `now` to the real clock when not supplied", () => {
    const farFuture = new Date(Date.now() + 1000 * 60 * 60 * 24 * 365 * 50).toISOString();
    expect(endDateLabel(farFuture)).toBe("Ends on");
  });
});

describe("closureStatePresentation", () => {
  it("returns null for a null closureState", () => {
    expect(closureStatePresentation(null)).toBeNull();
  });

  it("returns null for an empty/whitespace closureState", () => {
    expect(closureStatePresentation("")).toBeNull();
    expect(closureStatePresentation("   ")).toBeNull();
  });

  it("matches a known closure state regardless of casing", () => {
    expect(closureStatePresentation("Open")).toEqual({
      label: "Open",
      severity: "success",
    });
    expect(closureStatePresentation("READ_ONLY")).toEqual({
      label: "Read only",
      severity: "warning",
    });
  });

  it("maps every known closure state to its label and severity", () => {
    expect(closureStatePresentation("open")).toEqual({
      label: "Open",
      severity: "success",
    });
    expect(closureStatePresentation("notify")).toEqual({
      label: "Notify",
      severity: "warning",
    });
    expect(closureStatePresentation("read_only")).toEqual({
      label: "Read only",
      severity: "warning",
    });
    expect(closureStatePresentation("restricted")).toEqual({
      label: "Restricted",
      severity: "warning",
    });
    expect(closureStatePresentation("suspended")).toEqual({
      label: "Suspended",
      severity: "error",
    });
    expect(closureStatePresentation("closed")).toEqual({
      label: "Closed",
      severity: "error",
    });
  });

  it("falls back to a neutral chip for an unknown closureState", () => {
    expect(closureStatePresentation("pending_review")).toEqual({
      label: "Pending review",
      severity: "default",
    });
  });
});
