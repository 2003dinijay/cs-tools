/**
 * Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {
  keepPreviousData,
  useMutation,
  useQuery,
  useQueryClient,
  type UseMutationResult,
  type UseQueryResult,
} from "@tanstack/react-query";
import { useBackendApi } from "@api/backend/client";
import type {
  ScheduleAbsencesResponse,
  ScheduleAssignment,
  ScheduleAssignmentsResponse,
  ScheduleCatalogue,
  SearchScheduleAbsencesPayload,
  SearchScheduleAssignmentsPayload,
} from "../types";

/**
 * The catalogue changes when a lead adds a window, which is roughly never
 * within a session -- so it is cached for the session rather than refetched
 * alongside every rota read.
 */
const CATALOGUE_STALE_MS = 30 * 60_000;

/** A rota read is cheap and the roster does change during a shift handover. */
const ROTA_STALE_MS = 60_000;

const QK = {
  catalogue: ["team-schedule", "catalogue"] as const,
  assignments: (p: SearchScheduleAssignmentsPayload) =>
    ["team-schedule", "assignments", p] as const,
  absences: (p: SearchScheduleAbsencesPayload) => ["team-schedule", "absences", p] as const,
  leadTeams: ["team-schedule", "my-lead-teams"] as const,
};

/**
 * The zones, windows and absence kinds every view needs before it can draw
 * anything. Served as one payload, so one query.
 */
export function useScheduleCatalogue(): UseQueryResult<ScheduleCatalogue, Error> {
  const api = useBackendApi();
  return useQuery<ScheduleCatalogue, Error>({
    queryKey: QK.catalogue,
    queryFn: async () => (await api.get<ScheduleCatalogue>("/team-schedule/catalogue")) ?? {
      zones: [],
      shifts: [],
      absenceKinds: [],
    },
    staleTime: CATALOGUE_STALE_MS,
  });
}

/** Who is working over a date window. */
export function useScheduleAssignments(
  payload: SearchScheduleAssignmentsPayload,
  enabled = true,
): UseQueryResult<ScheduleAssignmentsResponse, Error> {
  const api = useBackendApi();
  return useQuery<ScheduleAssignmentsResponse, Error>({
    queryKey: QK.assignments(payload),
    queryFn: () =>
      api.post<SearchScheduleAssignmentsPayload, ScheduleAssignmentsResponse>(
        "/team-schedule/assignments/search",
        payload,
      ),
    enabled,
    placeholderData: keepPreviousData,
    staleTime: ROTA_STALE_MS,
  });
}

/** Who is out of the rota over a date window. */
export function useScheduleAbsences(
  payload: SearchScheduleAbsencesPayload,
  enabled = true,
): UseQueryResult<ScheduleAbsencesResponse, Error> {
  const api = useBackendApi();
  return useQuery<ScheduleAbsencesResponse, Error>({
    queryKey: QK.absences(payload),
    queryFn: () =>
      api.post<SearchScheduleAbsencesPayload, ScheduleAbsencesResponse>(
        "/team-schedule/absences/search",
        payload,
      ),
    enabled,
    placeholderData: keepPreviousData,
    staleTime: ROTA_STALE_MS,
  });
}

/**
 * Which teams this reader may edit.
 *
 * Asked once and cached: it changes when somebody is made a lead, not while
 * they are looking at a rota. The page uses it to decide whether to offer an
 * edit control at all -- entity-service refuses the write either way, but a
 * control that always fails is worse than no control.
 */
export function useMyLeadTeams(): UseQueryResult<string[], Error> {
  const api = useBackendApi();
  return useQuery<string[], Error>({
    queryKey: QK.leadTeams,
    queryFn: async () => {
      const r = await api.get<{ teamKeys?: string[] }>("/team-schedule/my-lead-teams");
      return r?.teamKeys ?? [];
    },
    staleTime: 5 * 60 * 1000,
  });
}

/** Everything a write touches, so the views redraw from the server rather than
 *  from an optimistic guess about what the server did. */
function invalidateRota(qc: ReturnType<typeof useQueryClient>): void {
  void qc.invalidateQueries({ queryKey: ["team-schedule", "assignments"] });
  void qc.invalidateQueries({ queryKey: ["team-schedule", "absences"] });
}

export interface CreateAssignmentPayload {
  userId: string;
  teamKey: string;
  shiftCode: string;
  rotaDate: string;
  tier?: string | null;
  isOnCall?: boolean | null;
  note?: string | null;
}

/** Put somebody on a window. Lead only; the server decides. */
export function useCreateAssignment(): UseMutationResult<
  ScheduleAssignment,
  Error,
  CreateAssignmentPayload
> {
  const api = useBackendApi();
  const qc = useQueryClient();
  return useMutation<ScheduleAssignment, Error, CreateAssignmentPayload>({
    mutationFn: (payload) =>
      api.post<CreateAssignmentPayload, ScheduleAssignment>("/team-schedule/assignments", payload),
    onSuccess: () => invalidateRota(qc),
  });
}

export interface UpdateAssignmentPayload {
  id: string;
  userId?: string;
  tier?: string;
  isOnCall?: boolean;
  note?: string;
}

/** Change who holds a slot. */
export function useUpdateAssignment(): UseMutationResult<
  ScheduleAssignment,
  Error,
  UpdateAssignmentPayload
> {
  const api = useBackendApi();
  const qc = useQueryClient();
  return useMutation<ScheduleAssignment, Error, UpdateAssignmentPayload>({
    mutationFn: ({ id, ...body }) =>
      api.patch<Omit<UpdateAssignmentPayload, "id">, ScheduleAssignment>(
        `/team-schedule/assignments/${encodeURIComponent(id)}`,
        body,
      ),
    onSuccess: () => invalidateRota(qc),
  });
}

/** Take somebody off a slot. */
export function useDeleteAssignment(): UseMutationResult<
  unknown,
  Error,
  { id: string; note?: string }
> {
  const api = useBackendApi();
  const qc = useQueryClient();
  return useMutation<unknown, Error, { id: string; note?: string }>({
    mutationFn: ({ id, note }) =>
      api.del(
        `/team-schedule/assignments/${encodeURIComponent(id)}` +
          (note ? `?note=${encodeURIComponent(note)}` : ""),
      ),
    onSuccess: () => invalidateRota(qc),
  });
}
