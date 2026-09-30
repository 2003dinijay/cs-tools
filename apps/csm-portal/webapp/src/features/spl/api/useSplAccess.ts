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

// Whether the signed-in user should see the SPL (Support Portal Lite)
// section at all — the "is this Sales/SA staff" audience gate, distinct
// from useSplPermissions.ts's fine-grained action gates.
//
// Reads a portal role off `GET /users/me` (the same server-authoritative
// `roles` array usePortalView reads it from — see
// `internal/handler/access.go`'s `AccessConfig`), not a client-side
// Asgardeo-groups decode.
//
// TEMPORARY: checks plain "viewer", not "sales_solutions", per explicit
// request — Sales/SA staff are provisioned with Viewer today, and
// sales_solutions isn't reliably assigned yet. This is NOT the long-term
// answer: CS engineers are slated to also hold Viewer once cs_engineer
// itself is retired in favor of composable roles, at which point they'd
// pass this gate too. Needs a real SPL-vs-CS-Portal signal before that
// happens — see usePortalView.ts and internal/handler/access.go's
// PermSPLAccess, which carry the identical, byte-for-byte-in-sync check
// and the same TEMPORARY flag; keep all three in lockstep.
//
// Real enforcement is server-side: every /spl/* route on the Go backend
// re-checks PermSPLAccess (internal/handler/access.go), currently granted
// by the same Viewer role this hook checks. A caller who reaches an SPL
// screen without the role sees a 403 from every call it makes, same as
// any other tampered/stale-claim scenario in this app.

import { useMemo } from "react";
import { useCurrentUser } from "@context/current-user/CurrentUserContext";
import { devBypassAccessCheck } from "@config/devFlags";

export interface SplAccess {
  /** False until the signed-in user's profile has been resolved — hold gated UI until it clears. */
  ready: boolean;
  hasAccess: boolean;
}

// Must stay byte-for-byte in sync with the backend's AccessGuard portalRoles
// key (apps/csm-portal/backend/internal/handler/access.go) and the identical
// literal usePortalView.ts check to pick the SPL nav. TEMPORARY -- see this
// file's own top-of-file comment.
const SPL_AUDIENCE_ROLE = "viewer";

export function useSplAccess(): SplAccess {
  let roles: string[] | undefined;
  let isLoading = false;
  try {
    // useCurrentUser always runs its useContext before it can throw, so the
    // hook order is identical on every render — same pattern as
    // usePortalAccess/usePortalView, which this mirrors.
    const ctx = useCurrentUser();
    roles = ctx.user?.roles;
    isLoading = ctx.isLoading;
  } catch {
    roles = undefined;
  }

  return useMemo<SplAccess>(() => {
    // TEMPORARY / LOCAL DEV ONLY — see authConfig.ts's devBypassAccessCheck.
    // Short-circuits SPL's own audience gate so the section shows up even
    // when the signed-in account has no portal roles provisioned yet.
    if (devBypassAccessCheck) return { ready: true, hasAccess: true };
    if (isLoading) return { ready: false, hasAccess: false };
    return { ready: true, hasAccess: (roles ?? []).includes(SPL_AUDIENCE_ROLE) };
  }, [roles, isLoading]);
}
