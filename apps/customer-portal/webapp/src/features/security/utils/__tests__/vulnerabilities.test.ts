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

import { describe, expect, it } from "vitest";
import {
  getAffectedProducts,
  getVulnerabilitySeverityColor,
  getVulnerabilityStatusColor,
} from "@features/security/utils/vulnerabilities";
import type { ProductVulnerability } from "@features/security/types/security";

describe("vulnerabilities utils", () => {
  it("maps severity labels to theme colors", () => {
    expect(getVulnerabilitySeverityColor(" Critical ")).toBe("error.main");
    expect(getVulnerabilitySeverityColor("HIGH")).toBe("warning.main");
    expect(getVulnerabilitySeverityColor("unknown")).toBe("text.secondary");
  });

  it("maps status labels to theme colors with strict matching", () => {
    expect(getVulnerabilityStatusColor("in progress")).toBe("warning.main");
    expect(getVulnerabilityStatusColor("resolved")).toBe("success.main");
    expect(getVulnerabilityStatusColor("unresolved")).toBe("text.secondary");
  });

  describe("getAffectedProducts", () => {
    const make = (
      overrides: Partial<ProductVulnerability>,
    ): ProductVulnerability => ({
      id: "id",
      cveId: "CVE-2026-1",
      vulnerabilityId: "VULN-1",
      severity: { id: "1", label: "High" },
      productName: "Product A",
      productVersion: "1.0.0",
      componentName: "lib-a",
      version: "1.0",
      type: "library",
      useCase: null,
      justification: null,
      resolution: null,
      ...overrides,
    });

    it("returns every record that shares the CVE, ordered by product and version", () => {
      const current = make({ id: "a2", productName: "Product A", productVersion: "10.0.0" });
      const all = [
        current,
        make({ id: "b1", productName: "Product B", productVersion: "2.0.0" }),
        make({ id: "a1", productName: "Product A", productVersion: "2.0.0" }),
        make({ id: "other", cveId: "CVE-2026-2", productName: "Product A" }),
      ];

      expect(getAffectedProducts(all, current).map((v) => v.id)).toEqual([
        "a1",
        "a2",
        "b1",
      ]);
    });

    it("matches the CVE case-insensitively and ignores surrounding whitespace", () => {
      const current = make({ id: "a", cveId: "CVE-2026-1" });
      const all = [current, make({ id: "b", cveId: " cve-2026-1 " })];

      expect(getAffectedProducts(all, current)).toHaveLength(2);
    });

    it("always includes the current record, even when the list does not contain it", () => {
      const current = make({ id: "missing" });
      const all = [make({ id: "x" })];

      expect(getAffectedProducts(all, current).map((v) => v.id).sort()).toEqual([
        "missing",
        "x",
      ]);
    });

    it("falls back to the vulnerability id when the CVE is missing", () => {
      const current = make({ id: "a", cveId: "", vulnerabilityId: "VULN-9" });
      const all = [
        current,
        make({ id: "b", cveId: "", vulnerabilityId: "VULN-9" }),
        make({ id: "c", cveId: "", vulnerabilityId: "VULN-10" }),
      ];

      expect(getAffectedProducts(all, current).map((v) => v.id)).toEqual(["a", "b"]);
    });

    it("does not group unrelated records when neither identifier is present", () => {
      const current = make({ id: "a", cveId: "", vulnerabilityId: "" });
      const all = [current, make({ id: "b", cveId: "", vulnerabilityId: "" })];

      expect(getAffectedProducts(all, current).map((v) => v.id)).toEqual(["a"]);
    });

    it("returns an empty list when there is no current record", () => {
      expect(getAffectedProducts([make({ id: "x" })], undefined)).toEqual([]);
    });
  });
});
