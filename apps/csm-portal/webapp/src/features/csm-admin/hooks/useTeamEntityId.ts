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

import { useSearchTeams } from "@features/csm-admin/api/useSearchTeams";

const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

export interface TeamEntityIdResult {
  /** The real backing UUID `GET /teams/{id}/members`,
   * `GET/PATCH /teams/{id}/chat-webhook` and `POST /teams/{id}/lead` need —
   * `undefined` while still resolving, or when this team has no such id to
   * resolve to (a registry-matched team with neither a configured CRE nor
   * SRE group). */
  teamEntityId: string | undefined;
  isLoading: boolean;
}

/**
 * `/admin/teams/:id` normally carries a registry team KEY (e.g. "alpha"), not
 * the real entity-service UUID the team-roster/chat-webhook/lead routes need
 * — `CsmTeamsPage`'s own `DirectoryEntityTable` link only ever puts the
 * registry key in the URL (the users-by-team filter on this same page needs
 * that exact key, so the link can't be changed to carry the UUID instead).
 * `DirectoryEntityChip` (a case/account/user's own team chip), on the other
 * hand, already links here with a real group UUID directly — not every
 * `:id` is a registry key in practice.
 *
 * Resolves which case this is: if `id` already looks like a UUID, it's used
 * as-is (no lookup); otherwise this searches the team registry for the
 * matching `BeTeam` row (by its exact registry key) and reads its own
 * `creGroupId`/`sreGroupId` — the only ids these routes actually accept for
 * a registry-curated team. A team with neither configured resolves to
 * `undefined`: the chat-webhook/lead-reassignment features simply have
 * nothing to act on for it.
 */
export function useTeamEntityId(
  id: string | undefined,
  name: string | undefined,
): TeamEntityIdResult {
  const alreadyUuid = !!id && UUID_RE.test(id);
  const searchQuery = name ?? id ?? "";

  const { data, isLoading } = useSearchTeams(
    { filters: { searchQuery }, pagination: { limit: 25, offset: 0 } },
    !alreadyUuid && !!searchQuery,
  );

  if (alreadyUuid) {
    return { teamEntityId: id, isLoading: false };
  }

  const match = data?.teams.find((t) => t.id === id);
  return { teamEntityId: match?.creGroupId ?? match?.sreGroupId, isLoading };
}
