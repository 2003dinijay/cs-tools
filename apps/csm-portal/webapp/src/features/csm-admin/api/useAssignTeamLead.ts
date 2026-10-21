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

import {
  useMutation,
  useQueryClient,
  type UseMutationResult,
} from "@tanstack/react-query";
import { ApiQueryKeys } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import type { BeAssignTeamLeadPayload } from "@api/backend/types";

/**
 * Reassigns a team's one `lead` via `POST /teams/{id}/lead` — admin-only
 * server-side. This writes the real, paging-connected `team_member.role`
 * column: entity-service's incident call-escalation ladder reads it to
 * decide who gets paged for this team, so this changes who is called during
 * an incident, immediately. Always pair this with setting the new lead's
 * phone number (`usePatchUser`) if they have none on file.
 */
export function useAssignTeamLead(
  teamId: string | undefined,
): UseMutationResult<void, Error, string> {
  const api = useBackendApi();
  const queryClient = useQueryClient();

  return useMutation<void, Error, string>({
    mutationFn: async (userId): Promise<void> => {
      if (!teamId) {
        throw new Error("Cannot assign a team lead without a team id.");
      }
      await api.post<BeAssignTeamLeadPayload, void>(
        `/teams/${encodeURIComponent(teamId)}/lead`,
        { userId },
      );
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.CSM_ADMIN_TEAM_MEMBERS, teamId ?? ""],
      });
    },
  });
}
