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

import { render, screen, fireEvent } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { ReactElement } from "react";
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";
import PendingVerificationsList from "@features/support/components/pending-verifications/PendingVerificationsList";
import type { PendingVerificationView } from "@features/support/types/pendingVerification";

const theme = createTheme();
function renderList(ui: ReactElement) {
  return render(<ThemeProvider theme={theme}>{ui}</ThemeProvider>);
}

const records: PendingVerificationView[] = [
  {
    id: "pv-1",
    workItemId: "work-item-1",
    caseId: "CS0441241",
    caseTitle: "First case",
    recordType: "Case",
    addedReason: "manual",
    addedAt: "2026-10-01T00:00:00Z",
    addedBy: "someone@test.local",
  },
  {
    id: "pv-2",
    workItemId: "work-item-2",
    caseId: "CS0441242",
    caseTitle: "Second case",
    recordType: "Case",
    addedReason: "auto-closed",
    previousStatus: "Solution Proposed",
    addedAt: "2026-10-02T00:00:00Z",
    addedBy: "system@entity-service",
  },
];

describe("PendingVerificationsList", () => {
  it("shows a loading skeleton while loading, and nothing else", () => {
    renderList(<PendingVerificationsList records={[]} isLoading />);

    expect(screen.queryByText("First case")).not.toBeInTheDocument();
    expect(screen.queryByText("No records pending verification.")).not.toBeInTheDocument();
  });

  it("shows an error state", () => {
    renderList(<PendingVerificationsList records={[]} isLoading={false} isError />);

    expect(screen.getByText(/Failed to load pending verifications/)).toBeInTheDocument();
  });

  it("shows the generic empty state with no search/filter applied", () => {
    renderList(<PendingVerificationsList records={[]} isLoading={false} />);

    expect(screen.getByText("No records pending verification.")).toBeInTheDocument();
  });

  it("shows the no-results empty state when a search/filter narrowed the list to nothing", () => {
    renderList(<PendingVerificationsList records={[]} isLoading={false} hasListRefinement />);

    expect(screen.getByText(/No records found/)).toBeInTheDocument();
  });

  it("uses a caller-provided emptyMessage instead of the generic one", () => {
    renderList(
      <PendingVerificationsList records={[]} isLoading={false} emptyMessage="No verified records yet." />,
    );

    expect(screen.getByText("No verified records yet.")).toBeInTheDocument();
    expect(screen.queryByText("No records pending verification.")).not.toBeInTheDocument();
  });

  it("renders one card per record", () => {
    renderList(<PendingVerificationsList records={records} isLoading={false} />);

    expect(screen.getByText("First case")).toBeInTheDocument();
    expect(screen.getByText("Second case")).toBeInTheDocument();
    expect(screen.getByText("From Solution Proposed")).toBeInTheDocument();
  });

  it("forwards onViewCase/onMarkVerified to each card", () => {
    const onViewCase = vi.fn();
    const onMarkVerified = vi.fn();
    renderList(
      <PendingVerificationsList
        records={records}
        isLoading={false}
        onViewCase={onViewCase}
        onMarkVerified={onMarkVerified}
      />,
    );

    fireEvent.click(screen.getByText("First case"));
    expect(onViewCase).toHaveBeenCalledWith(records[0]);

    fireEvent.click(screen.getAllByText("Mark Verified")[1]);
    expect(onMarkVerified).toHaveBeenCalledWith(records[1]);
  });
});
