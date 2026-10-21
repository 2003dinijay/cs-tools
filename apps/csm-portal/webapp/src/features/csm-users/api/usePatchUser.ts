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
import { useBackendApi } from "@api/backend/client";
import type { BeUpdateUserPayload } from "@api/backend/types";

/**
 * Updates another user's profile (timezone and/or phone) via
 * `PATCH /users/{id}` — restricted server-side to cs_engineer/admin, unlike
 * `PATCH /users/me` which only ever resolves the caller's own id. Used by the
 * team-lead reassignment flow to set the newly assigned lead's phone number,
 * which entity-service's incident call-escalation ladder dials.
 */
export function usePatchUser(): UseMutationResult<
  unknown,
  Error,
  { id: string; patch: BeUpdateUserPayload }
> {
  const api = useBackendApi();
  const queryClient = useQueryClient();

  return useMutation<unknown, Error, { id: string; patch: BeUpdateUserPayload }>({
    mutationFn: ({ id, patch }) =>
      api.patch<BeUpdateUserPayload, unknown>(
        `/users/${encodeURIComponent(id)}`,
        patch,
      ),
    onSuccess: (_data, { id }) => {
      queryClient.invalidateQueries({ queryKey: ["csm-user-by-id", id] });
    },
  });
}
