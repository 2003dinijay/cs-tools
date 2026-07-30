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

import { afterEach, describe, expect, it, vi } from "vitest";
import { isPastDateOnly, isPastDateTime } from "./dateTime";

describe("isPastDateTime", () => {
  it("is true for an instant strictly before now", () => {
    expect(isPastDateTime(new Date(Date.now() - 60_000))).toBe(true);
  });

  it("is false for an instant in the future", () => {
    expect(isPastDateTime(new Date(Date.now() + 60_000))).toBe(false);
  });

  it("is false for null (an empty/unset field isn't 'in the past')", () => {
    expect(isPastDateTime(null)).toBe(false);
  });

  it("is false for an invalid Date", () => {
    expect(isPastDateTime(new Date("not-a-date"))).toBe(false);
  });
});

describe("isPastDateOnly", () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it("is false for today's local-midnight date, regardless of current time", () => {
    // Freeze the clock so `today` (built here) and the "now" that
    // isPastDateOnly constructs internally can't disagree across an actual
    // local-midnight boundary crossed between this line and the call below.
    vi.useFakeTimers();
    vi.setSystemTime(new Date(2026, 0, 15, 12, 0, 0));
    const today = new Date();
    today.setHours(0, 0, 0, 0);
    expect(isPastDateOnly(today)).toBe(false);
  });

  it("is true for yesterday", () => {
    const yesterday = new Date();
    yesterday.setHours(0, 0, 0, 0);
    yesterday.setDate(yesterday.getDate() - 1);
    expect(isPastDateOnly(yesterday)).toBe(true);
  });

  it("is false for tomorrow", () => {
    const tomorrow = new Date();
    tomorrow.setHours(0, 0, 0, 0);
    tomorrow.setDate(tomorrow.getDate() + 1);
    expect(isPastDateOnly(tomorrow)).toBe(false);
  });

  it("is false for null/invalid", () => {
    expect(isPastDateOnly(null)).toBe(false);
    expect(isPastDateOnly(new Date("not-a-date"))).toBe(false);
  });
});
