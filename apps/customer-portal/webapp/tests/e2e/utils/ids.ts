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

//
// Record ids appear in two interchangeable spellings, and a URL assertion has to
// accept either.
//
// ServiceNow sys_ids — which is what projects, cases and deployments are keyed
// by — are 32 hex characters with no separators, and that is the form the
// fixtures in config/testData.ts store and the API returns. The portal, however,
// canonicalises the id in the address bar to the UUID-hyphenated spelling of the
// same 32 characters: navigating to
//
//   /projects/641058e63b5a87103e1e088aa4e45a13/dashboard
//
// lands on
//
//   /projects/641058e6-3b5a-8710-3e1e-088aa4e45a13/dashboard
//
// (verified live). Both address the same record and both are accepted on the way
// in, so this is a display convention rather than a redirect to somewhere else —
// but a `toHaveURL` built by interpolating a fixture id fails against it, and
// fails in a way that reads like a routing bug rather than a formatting one.
//

/**
 * Escapes a string for literal use inside a regular expression.
 *
 * @param value - Raw string.
 * @returns The string with regex metacharacters escaped.
 */
function escapeRegExp(value: string): string {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

/**
 * Builds a regex fragment matching an id in either spelling.
 *
 * Ids that are not 32-hex are returned escaped and unchanged — service request
 * numbers and the like have no second form, and silently widening them would
 * make an assertion looser than it looks.
 *
 * @param id - Record id, in either spelling.
 * @returns A non-capturing regex fragment, for interpolation into a URL pattern.
 */
export function idPattern(id: string): string {
  const plain = id.replace(/-/g, "");
  if (!/^[0-9a-f]{32}$/i.test(plain)) return escapeRegExp(id);

  const uuid = [
    plain.slice(0, 8),
    plain.slice(8, 12),
    plain.slice(12, 16),
    plain.slice(16, 20),
    plain.slice(20),
  ].join("-");

  return `(?:${plain}|${uuid})`;
}

/**
 * Builds a pattern for a project-scoped path, tolerant of both id spellings.
 *
 * @param projectId - Project id, in either spelling.
 * @param suffix - Path after the project id, e.g. `dashboard` or
 *   `support/cases/abc`. Interpolate ids in it via {@link idPattern}.
 * @returns A RegExp matching that path under either spelling.
 */
export function projectPathPattern(
  projectId: string,
  suffix: string,
): RegExp {
  return new RegExp(`/projects/${idPattern(projectId)}/${suffix}`);
}
