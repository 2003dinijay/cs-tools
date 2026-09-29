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
import DiscardDraftDialog from "@features/csm-admin/dashboards/components/DiscardDraftDialog";

function renderDialog(hasDeployedVersion: boolean) {
  const onCancel = vi.fn();
  const onConfirm = vi.fn();
  render(
    <DiscardDraftDialog
      open
      dashboardName="Engineer overview"
      hasDeployedVersion={hasDeployedVersion}
      onCancel={onCancel}
      onConfirm={onConfirm}
    />,
  );
  return { onCancel, onConfirm };
}

describe("DiscardDraftDialog", () => {
  it("names the dashboard and says the reset happens on next open when a deployed version exists", () => {
    renderDialog(true);
    expect(screen.getByText("Discard local draft?")).toBeInTheDocument();
    expect(screen.getByText(/"Engineer overview"/)).toBeInTheDocument();
    expect(screen.getByText(/cannot be undone/i)).toBeInTheDocument();
    expect(screen.getByText(/reset to the deployed version the next time it is opened/i)).toBeInTheDocument();
    expect(screen.queryByText(/deletes it entirely/i)).not.toBeInTheDocument();
  });

  it("says the dashboard is deleted entirely when it was never deployed", () => {
    renderDialog(false);
    expect(screen.getByText(/exists only in this browser/i)).toBeInTheDocument();
    expect(screen.getByText(/deletes it entirely/i)).toBeInTheDocument();
    expect(screen.queryByText(/deployed version/i)).not.toBeInTheDocument();
  });

  it("wires Cancel and Discard to their own callbacks", () => {
    const { onCancel, onConfirm } = renderDialog(true);
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(onCancel).toHaveBeenCalledTimes(1);
    expect(onConfirm).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Discard" }));
    expect(onConfirm).toHaveBeenCalledTimes(1);
  });
});
