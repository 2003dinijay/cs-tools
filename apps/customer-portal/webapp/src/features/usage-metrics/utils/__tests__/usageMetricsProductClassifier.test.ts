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

import { describe, expect, it } from "vitest";
import {
  METRIC_CHART_CONFIG,
  METRIC_CHART_CONFIG_FALLBACK,
  getProductMetricKeys,
} from "@features/usage-metrics/utils/usageMetricsProductClassifier";

const AGENT_KEYS = [
  "AGENT_DIRECT_TOKEN",
  "AGENT_DIRECT_REFRESH_TOKEN",
  "AGENT_OBO_TOKEN",
  "AGENT_OBO_REFRESH_TOKEN",
];

describe("usageMetricsProductClassifier", () => {
  it("gives every Identity Server metric its own title instead of the fallback", () => {
    for (const key of ["MAU", "M2M_TOKEN", ...AGENT_KEYS]) {
      expect(METRIC_CHART_CONFIG[key], key).toBeDefined();
      expect(METRIC_CHART_CONFIG[key].title).not.toBe(METRIC_CHART_CONFIG_FALLBACK.title);
    }
  });

  it("lists MAU and M2M, but no agent metrics, for Identity Server 7.1", () => {
    expect(getProductMetricKeys("WSO2 Identity Server", "7.1.0")).toEqual([
      "TOTAL_B2B_ORGS",
      "TOTAL_ROOT_ORGS",
      "TOTAL_USERS",
      "MAU",
      "M2M_TOKEN",
    ]);
  });

  it("adds the agent metrics from Identity Server 7.2", () => {
    for (const version of ["7.2.0", "7.3.0"]) {
      expect(getProductMetricKeys("WSO2 Identity Server", version)).toEqual([
        "TOTAL_B2B_ORGS",
        "TOTAL_ROOT_ORGS",
        "TOTAL_USERS",
        "MAU",
        "M2M_TOKEN",
        ...AGENT_KEYS,
      ]);
    }
  });

  it("keeps the older Identity Server versions to MAU and M2M without B2B orgs", () => {
    for (const version of ["5.11.0", "6.1.0"]) {
      expect(getProductMetricKeys("WSO2 Identity Server", version)).toEqual([
        "TOTAL_ROOT_ORGS",
        "TOTAL_USERS",
        "MAU",
        "M2M_TOKEN",
      ]);
    }
  });

  it("reads a major-only version as that major", () => {
    expect(getProductMetricKeys("WSO2 Identity Server", "8")).toEqual([
      "TOTAL_B2B_ORGS",
      "TOTAL_ROOT_ORGS",
      "TOTAL_USERS",
      "MAU",
      "M2M_TOKEN",
      ...AGENT_KEYS,
    ]);
  });

  it("reads the version from the label when none is given", () => {
    expect(getProductMetricKeys("WSO2 Identity Server 7.3.0")).toContain("AGENT_OBO_REFRESH_TOKEN");
    expect(getProductMetricKeys("WSO2 Identity Server 7.1.0")).not.toContain("AGENT_OBO_TOKEN");
  });

  it("treats an empty version as absent and reads the label instead", () => {
    expect(getProductMetricKeys("WSO2 Identity Server 7.3.0", "")).toContain("AGENT_OBO_REFRESH_TOKEN");
    expect(getProductMetricKeys("WSO2 Identity Server 7.1.0", "")).not.toContain("AGENT_OBO_TOKEN");
  });

  it("leaves the other products' metrics unchanged", () => {
    expect(getProductMetricKeys("WSO2 API Manager", "4.4.0")).toEqual(["TRANSACTION_COUNT", "API_COUNT"]);
    expect(getProductMetricKeys("WSO2 API Manager", "4.5.0")).not.toContain("MAU");
    expect(getProductMetricKeys("WSO2 Integrator: MI", "4.4.0")).toEqual(["TRANSACTION_COUNT"]);
  });
});
