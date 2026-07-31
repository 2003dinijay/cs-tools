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
  INCIDENT_PRIORITIES,
  type IncidentFilters,
} from "@features/csm-operations/utils/incidents";

// URL params owned by the incident filter state. Prefixed (`inc...`) so they
// can't collide with the same-named params the shared cases view and the
// change-requests tab keep in the same `?tab=`-switched URL.
export const INCIDENT_FILTER_PARAM_KEYS = ["incQ", "incPriorities"] as const;

function parseCsv<T extends string>(raw: string | null, allowed: T[]): T[] {
  if (!raw) return [];
  return raw
    .split(",")
    .map((s) => s.trim())
    .filter((s): s is T => (allowed as string[]).includes(s));
}

/**
 * Read incident filters from the URL. An unknown/malformed value (a
 * hand-edited or stale query string) is dropped rather than passed through,
 * so it falls back to the default (unfiltered) behaviour instead of being
 * silently sent to the backend.
 */
export function readIncidentFiltersFromUrl(
  params: URLSearchParams,
): IncidentFilters {
  return {
    search: params.get("incQ") ?? "",
    priorities: parseCsv(params.get("incPriorities"), INCIDENT_PRIORITIES),
  };
}

/**
 * Build the search-params representing these filters. Default values are
 * omitted so the URL stays clean.
 */
export function writeIncidentFiltersToUrl(f: IncidentFilters): URLSearchParams {
  const out = new URLSearchParams();
  if (f.search) out.set("incQ", f.search);
  if (f.priorities.length) out.set("incPriorities", f.priorities.join(","));
  return out;
}
