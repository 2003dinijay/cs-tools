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

// The Profile dialog shows `phoneNumber` (the identity provider's copy), but
// Case Alerts calls dial `platformPhone` (the platform user record). The data
// sync writes the platform copy too, so the two can drift apart, and the
// dialog only sent a phone when the shown number was edited: a user whose
// platform copy was wrong could not fix it by saving. These helpers compare
// the two so the dialog can say so and repair it.

// E.164 phone validator: leading + required, then country code digit + 7-14 more digits.
export const E164 = /^\+[1-9]\d{7,14}$/;

/** Formatting characters a stored number may carry; never part of the number. */
const PHONE_FORMATTING = /[\s\-().]/g;

/** The number with spaces, dashes, dots and brackets removed, for comparing. */
export function normalizePhone(value: string | null | undefined): string {
  return (value ?? "").replace(PHONE_FORMATTING, "");
}

/**
 * How the number Case Alerts dial compares with the one the dialog shows.
 *
 * - `unknown`: the backend did not say (`platformPhone` absent: an older
 *   backend, or the platform record could not be read). Nothing is claimed.
 * - `same`: both hold the same number, or neither holds one.
 * - `differs`: both hold a number, and they are different.
 * - `platformMissing`: the profile has a number, the platform record has none.
 * - `profileMissing`: the platform record has a number the profile does not show.
 */
export type PlatformPhoneState =
  | { kind: "unknown" }
  | { kind: "same" }
  | { kind: "differs"; platform: string; profile: string }
  | { kind: "platformMissing"; profile: string }
  | { kind: "profileMissing"; platform: string };

export function platformPhoneState(
  profilePhone: string | null | undefined,
  platformPhone: string | null | undefined,
): PlatformPhoneState {
  if (platformPhone === undefined) return { kind: "unknown" };
  const profile = (profilePhone ?? "").trim();
  const platform = (platformPhone ?? "").trim();
  if (normalizePhone(profile) === normalizePhone(platform)) {
    return { kind: "same" };
  }
  if (!platform) return { kind: "platformMissing", profile };
  if (!profile) return { kind: "profileMissing", platform };
  return { kind: "differs", platform, profile };
}

/**
 * Whether saving `nextPhone` must send it even though it equals the shown
 * number: it would correct the platform copy. Never true for an empty draft
 * (that would clear the platform number, which nobody asked for) or while the
 * platform copy is unknown.
 */
export function savingRepairsPlatform(
  nextPhone: string,
  platformPhone: string | null | undefined,
): boolean {
  if (platformPhone === undefined || nextPhone.trim() === "") return false;
  return normalizePhone(nextPhone) !== normalizePhone(platformPhone);
}
