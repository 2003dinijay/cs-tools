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

import type { BeTeamMember } from "@api/backend/types";

/** The team's current lead (role "lead", case-insensitive), or `undefined`
 * when the roster has none. A team has at most one, enforced server-side. */
export function findTeamLead(
  members: BeTeamMember[] | undefined,
): BeTeamMember | undefined {
  return members?.find((m) => m.role.toLowerCase() === "lead");
}

/** True when `callerEmail` matches the team's current lead, case-insensitively
 * — mirrors the backend's own `TeamHandler.callerIsTeamLead` check, so the UI
 * hides what the server would 403 anyway (see `apps/csm-portal/backend`'s own
 * CLAUDE.md). This is a display convenience only: the backend enforces it
 * regardless of what this returns. */
export function callerIsTeamLead(
  members: BeTeamMember[] | undefined,
  callerEmail: string | undefined,
): boolean {
  if (!callerEmail) return false;
  const lead = findTeamLead(members);
  return !!lead && lead.email.toLowerCase() === callerEmail.toLowerCase();
}
