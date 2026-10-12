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
  useMutation,
  useQuery,
  useQueryClient,
  type UseMutationResult,
  type UseQueryResult,
} from "@tanstack/react-query";
import { useBackendApi } from "@api/backend/client";

/** The one rota that can be generated. */
export const GENERATED_ROTA = "SRE_SAAS";

export interface GenerateRotaPayload {
  /** YYYY-MM. */
  month: string;
  dryRun?: boolean;
  regenerate?: boolean;
  /** YYYY-MM-DD, the first day written. */
  from?: string;
  /** Team key -> the people chosen to work that team's TZ3 for the month.
   *  A team left out keeps the usual nights. */
  nightCrew?: Record<string, string[]>;
}

export interface GeneratedRotaTurn {
  userId: string;
  name: string;
  teamKey: string;
  shiftCode: string;
  tier: string;
  rotaDate: string;
}

export interface GenerateRotaResult {
  rotaCode: string;
  month: string;
  from: string;
  dryRun: boolean;
  alreadyGenerated: boolean;
  summary: {
    planned: number;
    keptByHand: number;
    written: number;
    replaced: number;
    lieuPlanned: number;
    lieuWritten: number;
  };
  turns: GeneratedRotaTurn[];
  lieuLeave: { userId: string; name: string; teamKey: string; startsOn: string; endsOn: string }[];
  warnings: { date?: string; message: string }[];
}

/**
 * Work out (dryRun) or write a month of the SaaS rota from the availability
 * marked on the roster. Who may is the server's decision: a lead of Apollo or
 * Artemis, or an SRE rota admin.
 */
export function useGenerateRota(): UseMutationResult<GenerateRotaResult, Error, GenerateRotaPayload> {
  const api = useBackendApi();
  const qc = useQueryClient();
  return useMutation<GenerateRotaResult, Error, GenerateRotaPayload>({
    mutationFn: (payload) =>
      api.post<GenerateRotaPayload, GenerateRotaResult>(
        `/team-schedule/rotas/${GENERATED_ROTA}/generate`,
        payload,
      ),
    onSuccess: (result) => {
      if (result.dryRun) return;
      // The written month, and its lieu leave, are on the roster now.
      void qc.invalidateQueries({ queryKey: ["team-schedule", "assignments"] });
      void qc.invalidateQueries({ queryKey: ["team-schedule", "absences"] });
      // And the month's TZ3 people, which the generation recorded.
      void qc.invalidateQueries({ queryKey: NIGHT_CREW_KEY });
    },
  });
}

/** One person chosen for a month's TZ3 night crew, under the team chosen for. */
export interface NightCrewMember {
  userId: string;
  teamKey: string;
}

/** The TZ3 people each month was generated with; a month with nobody chosen
 *  has an empty crew. */
export interface RotaNightCrew {
  rotaCode: string;
  months: { month: string; crew: NightCrewMember[] }[];
}

const NIGHT_CREW_KEY = ["team-schedule", "night-crew"] as const;

/**
 * The TZ3 people each month fromMonth..toMonth (YYYY-MM) of the SaaS rota was
 * generated with -- who the Month roster lists under the night cover instead
 * of under their team. Recorded by a real "Generate month" run. Resolves to
 * null when the server has no such endpoint (an entity service from before
 * it answers 404): the roster then reads the night cover from the turns.
 */
export function useRotaNightCrew(
  fromMonth: string,
  toMonth: string,
  enabled = true,
): UseQueryResult<RotaNightCrew | null, Error> {
  const api = useBackendApi();
  return useQuery<RotaNightCrew | null, Error>({
    queryKey: [...NIGHT_CREW_KEY, fromMonth, toMonth],
    // A GET that 404s resolves to null: kept as null (not "nobody chosen"),
    // so the roster knows the server cannot say.
    queryFn: () =>
      api.get<RotaNightCrew>(
        `/team-schedule/rotas/${GENERATED_ROTA}/night-crew?from=${encodeURIComponent(fromMonth)}&to=${encodeURIComponent(toMonth)}`,
      ),
    enabled: enabled && /^\d{4}-\d{2}$/.test(fromMonth) && /^\d{4}-\d{2}$/.test(toMonth),
    staleTime: 60_000,
    retry: false,
  });
}
