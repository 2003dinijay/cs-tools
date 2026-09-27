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
// DELIBERATE EXCEPTION, see useAsgardeoGroups.ts's own doc comment: reads
// Asgardeo groups client-side rather than this app's usual backend-`roles`
// pattern. Chosen explicitly for the SPL merge rather than extending
// GET /users/me — see the merge plan for the trade-off.
//
// Real enforcement is server-side: every /spl/* route on the Go backend
// re-checks SPL_ALLOWED_GROUPS from the JWT (internal/splauth). A caller
// who reaches an SPL screen without the group sees a 403 from every call
// it makes, same as any other tampered/stale-claim scenario in this app.

import { useAsgardeoGroups, hasAnyGroup } from "@hooks/useAsgardeoGroups";
import { devBypassAccessCheck } from "@config/devFlags";

export interface SplAccess {
  /** False until group membership has been resolved — hold gated UI until it clears. */
  ready: boolean;
  hasAccess: boolean;
}

function splAudienceGroups(): string[] {
  return window.config?.CSM_PORTAL_SPL_AUDIENCE_GROUPS ?? [];
}

export function useSplAccess(): SplAccess {
  const identity = useAsgardeoGroups();
  // TEMPORARY / LOCAL DEV ONLY — see authConfig.ts's devBypassAccessCheck.
  // Short-circuits SPL's own audience gate so the section shows up even
  // when the real id_token carries no (or the wrong) Asgardeo groups.
  if (devBypassAccessCheck) return { ready: true, hasAccess: true };
  if (!identity.ready) return { ready: false, hasAccess: false };
  return { ready: true, hasAccess: hasAnyGroup(identity.groups, splAudienceGroups()) };
}
