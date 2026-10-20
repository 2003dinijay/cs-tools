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

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import RunbooksTab from "@features/csm-operations/components/RunbooksTab";

// The import tree reads this at module load; vitest does not load public/config.js.
vi.hoisted(() => {
  window.config = { ...window.config, CSM_PORTAL_BACKEND_BASE_URL: "http://localhost:9090" };
});

const fixtures = vi.hoisted(() => ({
  catalog: [
    { id: "1", alertName: "Gateway Latency High", runbookUrl: "https://docs.example.com/gw", createdOn: "", createdBy: "", updatedOn: "", updatedBy: "" },
    { id: "2", alertName: "JVM Heap", runbookUrl: null, createdOn: "", createdBy: "", updatedOn: "", updatedBy: "" },
  ],
  idleMutation: () => ({ mutate: vi.fn(), isPending: false, isError: false, error: null }),
}));

vi.mock("@context/current-user/usePortalAccess", () => ({ usePortalAccess: () => ({ canWrite: true }) }));
vi.mock("@features/csm-operations/api/useRunbooks", () => ({
  useRunbooks: () => ({ data: fixtures.catalog, isLoading: false, isError: false, error: null, refetch: vi.fn() }),
  useCreateRunbook: fixtures.idleMutation,
  useUpdateRunbook: fixtures.idleMutation,
  useDeleteRunbook: fixtures.idleMutation,
  useImportRunbooks: fixtures.idleMutation,
}));

describe("RunbooksTab search runbook with alert name", () => {
  it("finds the runbook for an alert name or an incident subject", () => {
    render(<RunbooksTab />);
    expect(screen.getByText("Search runbook with alert name")).toBeInTheDocument();
    const search = screen.getByLabelText("Search runbook with alert name");

    fireEvent.change(search, { target: { value: "gateway latency high" } });
    expect(screen.getByText("Matched alert: Gateway Latency High")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /docs\.example\.com\/gw/ })).toHaveAttribute("href", "https://docs.example.com/gw");

    fireEvent.change(search, { target: { value: "Icinga Alert: node-07 / JVM Heap" } });
    expect(screen.getByText("Matched alert: JVM Heap")).toBeInTheDocument();
    expect(screen.getByText("No runbook available for this alert.")).toBeInTheDocument();

    fireEvent.change(search, { target: { value: "unknown" } });
    expect(screen.getByText("No runbook covers this alert.")).toBeInTheDocument();
  });
});
