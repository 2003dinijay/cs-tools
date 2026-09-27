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

import { useMemo, type ReactNode } from "react";
import { useAsgardeoGroups, hasAnyGroup } from "@hooks/useAsgardeoGroups";
import { SplPermissionContext, type SplPermissions } from "./splPermissionsContext";

function splAddWorknoteGroups(): string[] {
  return window.config?.CSM_PORTAL_SPL_ADD_WORKNOTE_GROUPS ?? [];
}
function splAddEscalationGroups(): string[] {
  return window.config?.CSM_PORTAL_SPL_ADD_ESCALATION_GROUPS ?? [];
}
function splDownloadAttachmentGroups(): string[] {
  return window.config?.CSM_PORTAL_SPL_DOWNLOAD_ATTACHMENT_GROUPS ?? [];
}
function splUsageMetricsGroups(): string[] {
  return window.config?.CSM_PORTAL_SPL_USAGE_METRICS_GROUPS ?? [];
}

// Ported from apps/support-portal-lite/webapp's own SplPermissionProvider
// (itself ported from SupportPortalLite's Ballerina-app Authorize.tsx /
// one-wso2's version) — see useAsgardeoGroups.ts for why this reads
// Asgardeo groups client-side rather than this app's usual backend-`roles`
// pattern. Four independent booleans, each a group-membership check —
// UI guidance only: the Go backend enforces its own copies of these same
// checks server-side (internal/handler/spl_auth.go's requireSPLSubGroups).
export function SplPermissionProvider({ children }: { children: ReactNode }) {
  const { groups } = useAsgardeoGroups();

  const value = useMemo<SplPermissions>(
    () => ({
      canAddWorkNotes: hasAnyGroup(groups, splAddWorknoteGroups()),
      canAddEscalations: hasAnyGroup(groups, splAddEscalationGroups()),
      canDownloadAttachments: hasAnyGroup(groups, splDownloadAttachmentGroups()),
      canViewUsageMetrics: hasAnyGroup(groups, splUsageMetricsGroups()),
    }),
    [groups],
  );

  return <SplPermissionContext.Provider value={value}>{children}</SplPermissionContext.Provider>;
}
