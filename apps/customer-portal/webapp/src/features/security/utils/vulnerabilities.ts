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
  VulnerabilitySeverityToken,
  VulnerabilityStatusToken,
  type ProductVulnerability,
} from "@features/security/types/security";

/**
 * Returns the Oxygen UI color path for vulnerability severity.
 *
 * @param severity - Severity label (Critical, High, Medium, Low).
 * @returns The Oxygen UI color path.
 */
export const getVulnerabilitySeverityColor = (severity?: string): string => {
  const normalized = severity?.toLowerCase().trim() || "";
  switch (normalized) {
    case VulnerabilitySeverityToken.CRITICAL:
      return "error.main";
    case VulnerabilitySeverityToken.HIGH:
      return "warning.main";
    case VulnerabilitySeverityToken.MEDIUM:
      return "text.disabled";
    case VulnerabilitySeverityToken.LOW:
      return "info.main";
    default:
      return "text.secondary";
  }
};

/**
 * Returns the Oxygen UI color path for vulnerability status.
 * Uses exact matches so "unresolved" does not match "resolved".
 *
 * @param status - Status label (In Progress, Open, Resolved).
 * @returns The Oxygen UI color path.
 */
export const getVulnerabilityStatusColor = (status?: string): string => {
  const normalized = status?.toLowerCase().trim() || "";
  switch (normalized) {
    case VulnerabilityStatusToken.IN_PROGRESS:
      return "warning.main";
    case VulnerabilityStatusToken.OPEN:
      return "info.main";
    case VulnerabilityStatusToken.RESOLVED:
      return "success.main";
    default:
      return "text.secondary";
  }
};

const normalizeKey = (value?: string | null): string =>
  value?.trim().toLowerCase() ?? "";

const compareText = (a?: string | null, b?: string | null): number =>
  (a ?? "").localeCompare(b ?? "", undefined, { numeric: true, sensitivity: "base" });

/**
 * Returns every record that describes the same vulnerability as `current`.
 *
 * The backend stores one row per product, product version and component, so a
 * single CVE appears as many rows. Rows are matched on the CVE id, falling back
 * to the vulnerability id when the CVE is missing. `current` is always part of
 * the result, even when the supplied list is stale or does not contain it, and
 * the rows are ordered by product, product version, component, component version.
 *
 * @param all - Candidate records (typically the full product vulnerability list).
 * @param current - The vulnerability the details page is showing.
 * @returns The affected product records, including `current`.
 */
export const getAffectedProducts = (
  all: ProductVulnerability[],
  current: ProductVulnerability | undefined,
): ProductVulnerability[] => {
  if (!current) return [];

  const cveKey = normalizeKey(current.cveId);
  const vulnerabilityKey = normalizeKey(current.vulnerabilityId);

  const matches = all.filter((item) => {
    if (item.id === current.id) return true;
    if (cveKey) return normalizeKey(item.cveId) === cveKey;
    if (vulnerabilityKey) {
      return normalizeKey(item.vulnerabilityId) === vulnerabilityKey;
    }
    return false;
  });

  const rows = matches.some((item) => item.id === current.id)
    ? matches
    : [current, ...matches];

  return [...rows].sort(
    (a, b) =>
      compareText(a.productName, b.productName) ||
      compareText(a.productVersion, b.productVersion) ||
      compareText(a.componentName, b.componentName) ||
      compareText(a.version, b.version),
  );
};
