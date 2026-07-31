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

import type { BackendApi } from "@api/backend/client";
import {
  beStateFromUi,
  priorityFromSeverity,
  severityFromPriority,
  uiStateFromBe,
} from "@api/backend/mappers";
import { ASSIGNEE_ME_TOKEN } from "@features/csm-cases/utils/assignee";
import { BE_MAX_PAGE_LIMIT } from "@constants/apiConstants";
import type {
  BeCaseSearchFilters,
  BeCaseSearchView,
  BeUserSearchResponse,
} from "@api/backend/types";
import type { CasesFilters } from "@features/csm-cases/components/CasesFilterBar";
import type { CsmCaseRow } from "@features/csm-cases/types/csmCases";

/**
 * Builds the `/cases/search` `filters` object for the given UI filter state
 * — shared by `useGetCsmCases` (the listing) and the cases/service-requests
 * "Export CSV" action, so the export is provably searching with the exact
 * same filter payload the listing itself sent, not a hand-rolled
 * approximation that can silently drift from it.
 *
 * `assignedUserIds` must already be resolved (see
 * {@link resolveAssignedUserIds}) — this function does no async work.
 */
export function buildCaseSearchFilters(
  filters: CasesFilters,
  search: string,
  assignedUserIds: string[] | undefined,
): BeCaseSearchFilters {
  return {
    ...(search.length > 0 && { searchQuery: search }),
    ...(filters.severities.length > 0 && {
      severities: filters.severities.map(priorityFromSeverity),
    }),
    ...(filters.states.length > 0 && {
      states: filters.states.map(beStateFromUi),
    }),
    ...(filters.caseTypes.length > 0 && {
      types: filters.caseTypes,
    }),
    ...(filters.workStates.length > 0 && {
      workStates: filters.workStates,
    }),
    ...(filters.engagementTypes.length > 0 && {
      engagementTypes: filters.engagementTypes,
    }),
    ...(filters.projects.length > 0 && {
      projectIds: filters.projects,
    }),
    ...(assignedUserIds && assignedUserIds.length > 0 && {
      assignedUserIds,
    }),
    ...(filters.productNames.length > 0 && {
      productNames: filters.productNames,
    }),
  };
}

/** Sentinel: the assignee filter is active (one or more engineers/`@me`
 * selected) but resolved to no ids at all — the caller must treat this as a
 * definitionally-empty result, never as "no assignee filter" (which would
 * silently broaden the search back to every case). See the resolution note
 * in `useGetCsmCases` for why. */
export const ASSIGNEE_FILTER_RESOLVED_EMPTY = Symbol("assignee-filter-resolved-empty");

/**
 * Resolves the assignee filter (engineer emails + the `@me` sentinel) to the
 * user UUIDs `/cases/search` expects. Shared between `useGetCsmCases` and the
 * export action so both apply the exact same assignee filter to the exact
 * same set of cases.
 */
export async function resolveAssignedUserIds(
  api: BackendApi,
  assignees: string[],
  currentUserId: string | undefined,
): Promise<string[] | undefined | typeof ASSIGNEE_FILTER_RESOLVED_EMPTY> {
  if (assignees.length === 0) return undefined;

  const wantsMe = assignees.includes(ASSIGNEE_ME_TOKEN);
  const emails = assignees.filter((a) => a !== ASSIGNEE_ME_TOKEN);
  let byEmail: BeUserSearchResponse | null = null;
  if (emails.length > 0) {
    byEmail = await api.post<
      { filters: { emails: string[] }; pagination: { limit: number } },
      BeUserSearchResponse
    >("/users/search", {
      filters: { emails },
      pagination: { limit: BE_MAX_PAGE_LIMIT },
    });
  }
  const ids = new Set<string>();
  (byEmail?.users ?? []).forEach((u) => {
    if (u.id) ids.add(u.id);
  });
  if (wantsMe && currentUserId) ids.add(currentUserId);

  if (ids.size === 0) return ASSIGNEE_FILTER_RESOLVED_EMPTY;
  return [...ids];
}

/**
 * Maps one `/cases/search` result item to the UI's `CsmCaseRow` shape —
 * shared by `useGetCsmCases` (the listing) and the export action's own
 * paging, so an exported row is provably the same transform the table itself
 * applies, not a second, driftable copy of it.
 */
export function mapCaseSearchViewToRow(
  c: BeCaseSearchView,
  currentUserEmail: string | undefined,
): CsmCaseRow {
  const projectId = c.project?.id ?? "";
  const assigneeEmail = c.assignedEngineer?.email;
  const assignee = c.assignedEngineer?.name?.trim() || assigneeEmail || "Unassigned";
  const myEmail = currentUserEmail?.toLowerCase();
  const assigneeIsMe =
    !!assigneeEmail && !!myEmail && assigneeEmail.toLowerCase() === myEmail;
  return {
    id: c.id,
    caseNumber: c.number,
    wso2CaseId: c.internalId,
    subject: c.subject ?? "(no subject)",
    customer: "",
    accountId: "",
    projectId,
    projectName: c.project?.name ?? "-",
    product: c.deployedProduct?.name ?? c.product?.name ?? "-",
    severity: severityFromPriority(c.severity),
    state: uiStateFromBe(c.state),
    caseType: c.type,
    workState: c.workState ?? null,
    assignee,
    assigneeIsMe,
    slaClockType: "ack",
    minutesToBreach: 0,
    hasSla: false,
    createdAt: c.createdOn ?? "",
    updatedAt: c.updatedOn ?? c.createdOn ?? "",
    updatedAtIsCreatedFallback: !c.updatedOn && !!c.createdOn,
  };
}
