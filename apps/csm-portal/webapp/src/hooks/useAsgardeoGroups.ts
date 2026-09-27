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

// The signed-in user's Asgardeo group memberships, read from the `groups`
// claim of the id_token (the "groups" OIDC scope is already requested in
// AppWithConfig.tsx's AsgardeoProvider, so the claim is present without an
// extra round trip).
//
// DELIBERATE EXCEPTION to this app's own convention: every other
// authorization decision in this codebase comes from the backend's own
// `GET /users/me` (`roles`/`team`), NOT from Asgardeo/IdP claims — see
// UserProfileModal.tsx's comment ("Asgardeo groups are auth-only, not a
// role list") and dashboardBuilderAccess.ts. This hook exists ONLY for the
// SPL section's audience gate (useSplAccess) and its fine-grained action
// permissions (useSplPermissions) — a deliberate, explicitly-chosen
// exception (see the SPL merge plan), not an oversight. Do not reach for
// this hook to gate anything else in this app; follow the backend-roles
// pattern (hasDashboardBuilderAccess, useTimecardRole) instead.
//
// This is presentation only, in the strict sense: it decides which SPL
// controls are worth showing. Every one of them still hits the Go backend's
// /spl/* routes, which re-derive the same groups from the JWT server-side
// (internal/splauth) and reject a caller who doesn't hold them. A tampered
// token doesn't grant anything here — it grants a screen whose buttons all
// fail server-side.
//
// Simpler than one-wso2's version of this hook: this app has no equivalent
// of one-wso2's `@api/authBridge` session-recovery machinery to hook into,
// and this hook's only consumers are presentation-layer nav/button
// visibility, not anything that needs that level of resilience. If a decode
// fails, callers just fail closed (no groups) — the worst case is a hidden
// button, not a broken session.

import { useEffect, useState } from "react";
import { useAsgardeo } from "@asgardeo/react";

export interface AsgardeoGroups {
  /** False until the token has been decoded — hold gated UI until it clears. */
  ready: boolean;
  /** Group names, or an empty array when the claim is absent or decode failed. */
  groups: string[];
}

// Asgardeo emits `groups` as an array, but a user in exactly ONE group can
// come back as a bare string from some IS configurations — same quirk
// one-wso2's useAsgardeoGroups documents and normalizes.
function normalizeGroups(claim: unknown): string[] {
  if (Array.isArray(claim)) {
    return claim.filter((g): g is string => typeof g === "string");
  }
  if (typeof claim === "string" && claim.trim()) return [claim];
  return [];
}

export function useAsgardeoGroups(): AsgardeoGroups {
  const { isSignedIn, getDecodedIdToken } = useAsgardeo();
  const [state, setState] = useState<AsgardeoGroups>({ ready: false, groups: [] });

  useEffect(() => {
    // No setState here for the signed-out case — that's a synchronously
    // derivable fact (see the early return below), not something an effect
    // needs to sync; calling setState unconditionally at the top of an
    // effect body is what react-hooks/set-state-in-effect flags.
    if (!isSignedIn) return;
    let cancelled = false;
    getDecodedIdToken()
      .then((token) => {
        if (cancelled) return;
        const claim = (token as { groups?: unknown } | null)?.groups;
        setState({ ready: true, groups: normalizeGroups(claim) });
      })
      .catch(() => {
        if (cancelled) return;
        // Fail closed: no groups, but still "ready" so a gate doesn't spin
        // forever on a decode that is never going to succeed.
        setState({ ready: true, groups: [] });
      });
    return () => {
      cancelled = true;
    };
  }, [isSignedIn, getDecodedIdToken]);

  if (!isSignedIn) return { ready: false, groups: [] };
  return state;
}

/** True when `groups` contains any of `required`. Empty `required` → false. */
export function hasAnyGroup(groups: readonly string[], required: readonly string[]): boolean {
  return required.some((r) => Boolean(r) && groups.includes(r));
}
