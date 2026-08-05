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

import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import { ApiQueryKeys, BE_MAX_PAGE_LIMIT } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import { auditEntryFromBeActivity } from "@features/csm-cases/api/useCsmCaseActivities";
import type {
  BeCaseActivitiesSearchPayload,
  BeCaseActivitiesSearchResponse,
} from "@api/backend/types";
import type { CaseAuditEntry } from "@features/csm-cases/types/csmCases";

/** Page size used when loading the field-change lane. Capped by the BE; see BE_MAX_PAGE_LIMIT. */
const ACTIVITIES_PAGE_LIMIT = BE_MAX_PAGE_LIMIT;

/**
 * Load the audited field/state-change lane for an incident, calling
 * `POST /incidents/{id}/activities/search` — a distinct endpoint from the
 * case one, confirmed by the team to exist as its own resource on the
 * ServiceNow side (both it and the case activities endpoint are built on
 * the same underlying activity-search mechanism, but they are separate
 * endpoints, not one shared URL). Reuses {@link auditEntryFromBeActivity}
 * from the case activities hook rather than duplicating the mapping logic,
 * since an activity entry's shape is identical either way.
 *
 * Not yet exercised against a live response in this session (no ServiceNow
 * credentials available here) — the request/response shape mirrors the
 * case endpoint exactly on the assumption it matches, per the team's
 * confirmation that both endpoints share the same underlying mechanism.
 */
export function useGetCsmIncidentActivities(
  incidentId: string | undefined,
): UseQueryResult<CaseAuditEntry[], Error> {
  const api = useBackendApi();

  return useQuery<CaseAuditEntry[], Error>({
    queryKey: [ApiQueryKeys.INCIDENT_ACTIVITIES, incidentId ?? ""],
    queryFn: async (): Promise<CaseAuditEntry[]> => {
      if (!incidentId) return [];

      const payload: BeCaseActivitiesSearchPayload = {
        pagination: { offset: 0, limit: ACTIVITIES_PAGE_LIMIT },
        includeFieldChanges: true,
      };
      const response = await api.post<
        BeCaseActivitiesSearchPayload,
        BeCaseActivitiesSearchResponse
      >(`/incidents/${encodeURIComponent(incidentId)}/activities/search`, payload);
      return (response.activity ?? [])
        .filter((a) => a.type === "field_change")
        .map(auditEntryFromBeActivity);
    },
    enabled: !!incidentId,
    staleTime: 10_000,
  });
}
