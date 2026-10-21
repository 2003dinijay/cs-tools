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
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";
import PendingVerificationCard, {
  type PendingVerificationCardProps,
} from "@features/support/components/pending-verifications/PendingVerificationCard";
import type { PendingVerificationView } from "@features/support/types/pendingVerification";

const baseRecord: PendingVerificationView = {
  id: "pv-1",
  workItemId: "work-item-1",
  caseId: "CS0441241",
  caseTitle: "Customer cannot log in",
  recordType: "Case",
  addedReason: "manual",
  addedAt: "2026-10-01T00:00:00Z",
  addedBy: "someone@test.local",
};

const theme = createTheme();
function renderCard(props: Partial<PendingVerificationCardProps> = {}) {
  return render(
    <ThemeProvider theme={theme}>
      <PendingVerificationCard record={baseRecord} {...props} />
    </ThemeProvider>,
  );
}

describe("PendingVerificationCard", () => {
  it("renders the case number and title", () => {
    renderCard();

    expect(screen.getByText("CS0441241")).toBeInTheDocument();
    expect(screen.getByText("Customer cannot log in")).toBeInTheDocument();
  });

  it("shows a Manually Added chip for a manual entry, with no previous-status chip", () => {
    renderCard();

    expect(screen.getByText("Manually Added")).toBeInTheDocument();
    expect(screen.queryByText(/^From /)).not.toBeInTheDocument();
  });

  it("shows an Auto-closed chip and the previous-status chip for an auto-closed entry", () => {
    renderCard({
      record: {
        ...baseRecord,
        addedReason: "auto-closed",
        previousStatus: "Awaiting Info",
      },
    });

    expect(screen.getByText("Auto-closed")).toBeInTheDocument();
    expect(screen.getByText("From Awaiting Info")).toBeInTheDocument();
  });

  it("does not show a previous-status chip for an auto-closed entry with none recorded", () => {
    renderCard({ record: { ...baseRecord, addedReason: "auto-closed" } });

    expect(screen.queryByText(/^From /)).not.toBeInTheDocument();
  });

  // entity-service sends the lowercase domain word (matching every other
  // case severity field in this API), not the raw "S1" DB code -- the
  // display label is still reconstructed as "S1 — Critical".
  it("shows a severity chip when severity is present", () => {
    renderCard({ record: { ...baseRecord, severity: "critical" } });

    expect(screen.getByText(/^S1/)).toBeInTheDocument();
  });

  it("shows a note when present, and omits it otherwise", () => {
    const { rerender } = renderCard({ record: { ...baseRecord, note: "Customer confirmed fix" } });
    expect(screen.getByText("Customer confirmed fix")).toBeInTheDocument();

    rerender(
      <ThemeProvider theme={theme}>
        <PendingVerificationCard record={baseRecord} />
      </ThemeProvider>,
    );
    expect(screen.queryByText("Customer confirmed fix")).not.toBeInTheDocument();
  });

  it("shows Mark Verified for an unverified entry and calls onMarkVerified without triggering onViewCase", () => {
    const onViewCase = vi.fn();
    const onMarkVerified = vi.fn();
    renderCard({ onViewCase, onMarkVerified });

    fireEvent.click(screen.getByText("Mark Verified"));

    expect(onMarkVerified).toHaveBeenCalledWith(baseRecord);
    expect(onViewCase).not.toHaveBeenCalled();
  });

  it("shows a Verified chip and hides Mark Verified for a verified entry", () => {
    renderCard({
      record: { ...baseRecord, verifiedAt: "2026-10-05T00:00:00Z", verifiedBy: "verifier@test.local" },
    });

    expect(screen.getByText("Verified")).toBeInTheDocument();
    expect(screen.queryByText("Mark Verified")).not.toBeInTheDocument();
  });

  it("calls onViewCase when the card itself is clicked", () => {
    const onViewCase = vi.fn();
    renderCard({ onViewCase });

    fireEvent.click(screen.getByText("Customer cannot log in"));

    expect(onViewCase).toHaveBeenCalledWith(baseRecord);
  });
});
