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

/** A case's parent reference, as far as routing is concerned. */
export interface ParentRecordRefLike {
  id: string;
  /** Kind of record the parent is, when the backend could resolve it. */
  type?: string | null;
}

/**
 * Route for a case's parent-record chip. A case's parent can be any of several
 * record kinds (case, incident, change request, or problem), so this cannot
 * assume `/cases/{id}`.
 *
 * Returns null when the parent's kind is absent or unrecognised. That is not a
 * "probably a case" signal: the backend leaves `type` null precisely when it
 * could not resolve the referenced record's kind, and routing such a parent to
 * a case detail page lands on a 404. Callers render a non-clickable chip
 * instead, so the link is still visible but never dead.
 */
export function parentRecordPath(
  parentCase: ParentRecordRefLike | undefined | null,
): string | null {
  if (!parentCase) return null;
  switch (parentCase.type) {
    case "case":
      return `/cases/${parentCase.id}`;
    case "incident":
      return `/operations/incidents/${parentCase.id}`;
    case "change_request":
      return `/operations/change-requests/${parentCase.id}`;
    case "problem":
      return `/operations/problems/${parentCase.id}`;
    default:
      return null;
  }
}
