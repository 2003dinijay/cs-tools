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

import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ScopeDeployment } from "@features/csm-operations/api/useChangeRequestScopeLookups";

// The lookup is the only thing mocked: the cascade under test is the hook's own.
// `lookupDeployments` is what the "backend" currently returns; a test can change
// it between renders to model data arriving late.
let lookupDeployments: Record<string, ScopeDeployment[]> = {};
vi.mock("@features/csm-operations/api/useChangeRequestScopeLookups", () => ({
  useChangeRequestScopeLookups: (projectId: string | undefined) => ({
    deployments: projectId ? (lookupDeployments[projectId] ?? []) : [],
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
  }),
}));

import { useChangeRequestScope } from "@features/csm-operations/hooks/useChangeRequestScope";

const dep = (
  id: string,
  envs: Array<[string, string]> | undefined,
  products: Array<[string, string]> | undefined,
): ScopeDeployment => ({
  id,
  label: `Deployment ${id}`,
  environments: envs?.map(([eid, label]) => ({ id: eid, label })),
  products: products?.map(([pid, label]) => ({ id: pid, label })),
});

beforeEach(() => {
  lookupDeployments = {
    p1: [
      dep("d1", [["e-prod", "Production"]], [["x1", "APIM"], ["x2", "IS"]]),
      dep("d2", [["e-stg", "Staging"]], [["x3", "APIM staging"]]),
      dep("d3", [["e-prod", "Production"]], [["x4", "Choreo"]]),
    ],
    p2: [dep("d9", [["e-dev", "Development"]], [["x9", "Other"]])],
  };
});

describe("useChangeRequestScope", () => {
  it("starts empty with nothing selectable below the project", () => {
    const { result } = renderHook(() => useChangeRequestScope());
    expect(result.current).toMatchObject({
      projectId: "",
      deploymentIds: [],
      environmentIds: [],
      deploymentProductIds: [],
      deploymentOptions: [],
      environmentOptions: [],
      productsReady: true,
    });
  });

  it("offers the picked project's deployments", () => {
    const { result } = renderHook(() => useChangeRequestScope());
    act(() => result.current.setProject("p1", "Project One"));
    expect(result.current.projectLabel).toBe("Project One");
    expect(result.current.deploymentOptions.map((d) => d.id)).toEqual(["d1", "d2", "d3"]);
  });

  it("preselects the environments of newly chosen deployments and derives their products", () => {
    const { result } = renderHook(() => useChangeRequestScope());
    act(() => result.current.setProject("p1"));
    act(() => result.current.setDeployments(["d1"]));
    expect(result.current.environmentIds).toEqual(["e-prod"]);
    expect(result.current.deploymentProductIds).toEqual(["x1", "x2"]);

    act(() => result.current.setDeployments(["d1", "d2"]));
    expect(result.current.environmentIds).toEqual(["e-prod", "e-stg"]);
    expect(result.current.deploymentProductIds).toEqual(["x1", "x2", "x3"]);
    expect(result.current.environmentOptions.map((e) => e.id)).toEqual(["e-prod", "e-stg"]);
  });

  it("offers an environment shared by two deployments once", () => {
    const { result } = renderHook(() => useChangeRequestScope());
    act(() => result.current.setProject("p1"));
    act(() => result.current.setDeployments(["d1", "d3"]));
    expect(result.current.environmentOptions.map((e) => e.id)).toEqual(["e-prod"]);
    expect(result.current.environmentIds).toEqual(["e-prod"]);
  });

  it("keeps a shared environment when only one of the deployments providing it is removed", () => {
    const { result } = renderHook(() => useChangeRequestScope());
    act(() => result.current.setProject("p1"));
    act(() => result.current.setDeployments(["d1", "d3"]));
    act(() => result.current.setDeployments(["d3"]));
    expect(result.current.environmentIds).toEqual(["e-prod"]);
    expect(result.current.deploymentProductIds).toEqual(["x4"]);
  });

  it("drops an environment no remaining deployment provides, and its products", () => {
    const { result } = renderHook(() => useChangeRequestScope());
    act(() => result.current.setProject("p1"));
    act(() => result.current.setDeployments(["d1", "d2"]));
    act(() => result.current.setDeployments(["d2"]));
    expect(result.current.environmentIds).toEqual(["e-stg"]);
    expect(result.current.deploymentProductIds).toEqual(["x3"]);
  });

  it("does not bring a deselected environment back when an unrelated deployment is added", () => {
    const { result } = renderHook(() => useChangeRequestScope());
    act(() => result.current.setProject("p1"));
    act(() => result.current.setDeployments(["d1", "d2"]));
    act(() => result.current.setEnvironments(["e-prod"])); // deselect Staging
    act(() => result.current.setDeployments(["d1", "d2", "d3"]));
    expect(result.current.environmentIds).toEqual(["e-prod"]);
  });

  it("clears deployments, environments and products when the project changes", () => {
    const { result } = renderHook(() => useChangeRequestScope());
    act(() => result.current.setProject("p1"));
    act(() => result.current.setDeployments(["d1"]));
    act(() => result.current.setProject("p2", "Project Two"));
    expect(result.current).toMatchObject({
      projectId: "p2",
      projectLabel: "Project Two",
      deploymentIds: [],
      environmentIds: [],
      deploymentProductIds: [],
    });
    expect(result.current.deploymentOptions.map((d) => d.id)).toEqual(["d9"]);
  });

  it("clears everything when the project is cleared", () => {
    const { result } = renderHook(() => useChangeRequestScope());
    act(() => result.current.setProject("p1", "Project One"));
    act(() => result.current.setDeployments(["d1"]));
    act(() => result.current.setProject(""));
    expect(result.current).toMatchObject({ projectId: "", projectLabel: "", deploymentIds: [], environmentIds: [] });
  });

  it("leaves everything alone when the same project is picked again", () => {
    const { result } = renderHook(() => useChangeRequestScope());
    act(() => result.current.setProject("p1"));
    act(() => result.current.setDeployments(["d1"]));
    act(() => result.current.setProject("p1"));
    expect(result.current.deploymentIds).toEqual(["d1"]);
  });

  describe("seeded (draft / edit dialog)", () => {
    const seed = {
      projectId: "p1",
      projectLabel: "Project One",
      deployments: [{ id: "d1", label: "Deployment d1" }, { id: "d2", label: "Deployment d2" }],
      environments: [{ id: "e-prod", label: "Production" }],
      deploymentProducts: [{ id: "x1", label: "APIM" }],
    };

    it("keeps a stored subset of environments instead of re-selecting all of them", () => {
      const { result } = renderHook(() => useChangeRequestScope(seed));
      expect(result.current.environmentIds).toEqual(["e-prod"]);
      expect(result.current.deploymentIds).toEqual(["d1", "d2"]);
    });

    it("shows the seeded names for ids the lookup has not resolved", () => {
      lookupDeployments = {};
      const { result } = renderHook(() => useChangeRequestScope(seed));
      expect(result.current.deploymentLabels).toMatchObject({ d1: "Deployment d1", d2: "Deployment d2" });
      expect(result.current.environmentLabels).toMatchObject({ "e-prod": "Production" });
    });

    it("stands the seeded products in until the lookup has settled, and does not claim they are ready", () => {
      // Products unknown (still loading) for the chosen deployments.
      lookupDeployments = {
        p1: [dep("d1", [["e-prod", "Production"]], undefined), dep("d2", [["e-stg", "Staging"]], undefined)],
      };
      const { result } = renderHook(() => useChangeRequestScope(seed));
      expect(result.current.deploymentProductIds).toEqual(["x1"]);
      expect(result.current.productsReady).toBe(false);
    });

    it("switches to the freshly derived products once they settle", () => {
      lookupDeployments = {
        p1: [dep("d1", [["e-prod", "Production"]], undefined), dep("d2", [["e-stg", "Staging"]], undefined)],
      };
      const { result, rerender } = renderHook(() => useChangeRequestScope(seed));
      lookupDeployments = {
        p1: [
          dep("d1", [["e-prod", "Production"]], [["x1", "APIM"], ["x2", "IS"]]),
          dep("d2", [["e-stg", "Staging"]], [["x3", "APIM staging"]]),
        ],
      };
      rerender();
      expect(result.current.deploymentProductIds).toEqual(["x1", "x2", "x3"]);
      expect(result.current.productsReady).toBe(true);
    });

    it("adds an environment for a newly chosen deployment on top of the stored subset", () => {
      const { result } = renderHook(() => useChangeRequestScope(seed));
      act(() => result.current.setDeployments(["d1", "d2", "d3"]));
      // d3's environment (e-prod) is already stored; d2's Staging stays unselected as stored.
      expect(result.current.environmentIds).toEqual(["e-prod"]);
    });
  });

  describe("environments that arrive after the deployment was chosen", () => {
    it("folds them in once known, without dropping anything in the meantime", () => {
      lookupDeployments = { p1: [dep("d1", undefined, undefined), dep("d2", [["e-stg", "Staging"]], [["x3", "APIM staging"]])] };
      const { result, rerender } = renderHook(() => useChangeRequestScope());
      act(() => result.current.setProject("p1"));
      act(() => result.current.setDeployments(["d2", "d1"]));
      // d2's environment is known, d1's is not: keep what we have, drop nothing.
      expect(result.current.environmentIds).toEqual(["e-stg"]);

      lookupDeployments = {
        p1: [dep("d1", [["e-prod", "Production"]], [["x1", "APIM"]]), dep("d2", [["e-stg", "Staging"]], [["x3", "APIM staging"]])],
      };
      rerender();
      expect(result.current.environmentIds).toEqual(["e-stg", "e-prod"]);
    });
  });
});
