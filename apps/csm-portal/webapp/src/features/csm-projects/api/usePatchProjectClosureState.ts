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

/** The five real values of `wso2_closure_state_enum` (entity-service). */
export const PROJECT_CLOSURE_STATES = [
  "OPEN",
  "READ_ONLY",
  "CLOSED",
  "RESTRICTED",
  "SUSPENDED",
] as const;

export type ProjectClosureState = (typeof PROJECT_CLOSURE_STATES)[number];

interface ProjectClosureStatePatch {
  closureState: ProjectClosureState;
}

interface ProjectClosureStatePatchResponse {
  message: string;
  project: { id: string; closureState: string | null };
}

/**
 * Directly override a project's overall closure state via
 * `PATCH /projects/{id}` `{closureState}` — admin only (see
 * `apps/csm-portal/backend`'s own CLAUDE.md, "PATCH /projects/{id}:
 * closureState requires PermAdmin"; `usePortalAccess().canUpdateProjectClosureState`
 * is this app's own mirror of that gate). Bypasses the usual derivation from
 * the project's three closure sub-state fields — a later edit that touches
 * any of those recomputes over this value again, with nothing recording that
 * it was ever manually set (see entity-service's own CLAUDE.md for the full
 * reasoning). Mirrors `usePatchAccountTeams`'s shape: bound to a single id up
 * front, invalidates the detail query on success so it refetches the
 * authoritative state.
 */
export function usePatchProjectClosureState(
  projectId: string | undefined,
): UseMutationResult<ProjectClosureStatePatchResponse, Error, ProjectClosureState> {
  const api = useBackendApi();
  const queryClient = useQueryClient();

  return useMutation<ProjectClosureStatePatchResponse, Error, ProjectClosureState>({
    mutationFn: async (closureState): Promise<ProjectClosureStatePatchResponse> => {
      if (!projectId) {
        throw new Error("Cannot update a project without an id.");
      }
      return api.patch<ProjectClosureStatePatch, ProjectClosureStatePatchResponse>(
        `/projects/${encodeURIComponent(projectId)}`,
        { closureState },
      );
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.CSM_PROJECT_DETAIL, projectId ?? ""],
      });
    },
  });
}
