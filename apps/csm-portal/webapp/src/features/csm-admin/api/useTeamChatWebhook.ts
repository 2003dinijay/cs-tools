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
  useQuery,
  useQueryClient,
  type UseMutationResult,
  type UseQueryResult,
} from "@tanstack/react-query";
import { ApiQueryKeys } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import type { BeTeamDetail, BeUpdateTeamPayload } from "@api/backend/types";

/**
 * A team's own id/name/type and configured Google Chat space link via
 * `GET /teams/{id}`. Restricted server-side to an Asgardeo admin or this
 * specific team's own lead (403 for everyone else) — `id` is the same real
 * backing UUID `useGetTeamMembers` takes, never the registry key. A single
 * generic team resource (mirroring `GET /users/{id}`) rather than a
 * dedicated `/teams/{id}/chat-webhook` endpoint.
 */
export function useGetTeam(
  teamId: string | undefined,
  options: { enabled?: boolean } = {},
): UseQueryResult<BeTeamDetail | null, Error> {
  const api = useBackendApi();

  return useQuery<BeTeamDetail | null, Error>({
    queryKey: [ApiQueryKeys.CSM_ADMIN_TEAM_DETAIL, teamId ?? ""],
    queryFn: (): Promise<BeTeamDetail | null> =>
      api.get<BeTeamDetail>(`/teams/${encodeURIComponent(teamId as string)}`),
    enabled: !!teamId && (options.enabled ?? true),
    staleTime: 30_000,
  });
}

/**
 * Sets or clears a team's own Chat space link via `PATCH /teams/{id}` —
 * `chatWebhookUrl` is the only settable field today. An empty value clears
 * it; a non-empty value must be a well-formed
 * `https://chat.googleapis.com/...` URL (the backend's own refusal reason is
 * echoed back through `error`).
 */
export function useUpdateTeam(
  teamId: string | undefined,
): UseMutationResult<BeTeamDetail, Error, string> {
  const api = useBackendApi();
  const queryClient = useQueryClient();

  return useMutation<BeTeamDetail, Error, string>({
    mutationFn: async (chatWebhookUrl): Promise<BeTeamDetail> => {
      if (!teamId) {
        throw new Error("Cannot update a team's Chat space without a team id.");
      }
      return api.patch<BeUpdateTeamPayload, BeTeamDetail>(
        `/teams/${encodeURIComponent(teamId)}`,
        { chatWebhookUrl },
      );
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.CSM_ADMIN_TEAM_DETAIL, teamId ?? ""],
      });
    },
  });
}
