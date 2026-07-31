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

import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import TimeCardsTable from "@features/csm-timecards/components/TimeCardsTable";
import type { CsmTimeCard } from "@features/csm-timecards/types/timeCards";

const CARD: CsmTimeCard = {
  id: "tc-1",
  caseId: "case-1",
  caseNumber: "CS0352584",
  projectId: "proj-1",
  projectName: "Acme Project",
  workDate: "2026-07-01",
  userId: "user-1",
  userName: "Jane Doe",
  state: "submitted",
  billable: true,
  totalMinutes: 30,
};

const ROLE_CTX = { isOwner: false, isApprover: false, isAdmin: false };

describe("TimeCardsTable column visibility", () => {
  it("shows the Case column but not the Engineer column on the personal view (My time sheets)", () => {
    render(
      <TimeCardsTable
        cards={[CARD]}
        isLoading={false}
        emptyText="No cards"
        groupBy="case"
        roleFor={() => ROLE_CTX}
        onCardAction={vi.fn()}
      />,
    );

    expect(screen.getByRole("columnheader", { name: "Case" })).toBeInTheDocument();
    expect(
      screen.queryByRole("columnheader", { name: "Engineer" }),
    ).not.toBeInTheDocument();

    expect(screen.getByText("CS0352584")).toBeInTheDocument();
    expect(screen.queryByText("Jane Doe")).not.toBeInTheDocument();
  });

  it("shows both the Case and Engineer columns when showEngineerColumn is set (All / Approvals)", () => {
    render(
      <TimeCardsTable
        cards={[CARD]}
        isLoading={false}
        emptyText="No cards"
        groupBy="case"
        showEngineerColumn
        roleFor={() => ROLE_CTX}
        onCardAction={vi.fn()}
      />,
    );

    expect(screen.getByRole("columnheader", { name: "Case" })).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "Engineer" })).toBeInTheDocument();

    expect(screen.getByText("CS0352584")).toBeInTheDocument();
    expect(screen.getByText("Jane Doe")).toBeInTheDocument();
  });
});
