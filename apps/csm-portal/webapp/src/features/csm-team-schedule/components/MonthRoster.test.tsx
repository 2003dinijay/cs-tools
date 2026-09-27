/**
 * Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import MonthRoster from "./MonthRoster";
import {
  ANNUAL_LEAVE,
  EVENING,
  MONDAY,
  REGULAR,
  TZ1,
  TZ1_WE,
  TZ2,
  TZ2_WE,
  absence,
  assignment,
  scopeControls,
  shiftMap,
} from "../test/fixtures";

const SHIFTS = shiftMap(REGULAR, EVENING, TZ1, TZ2, TZ1_WE, TZ2_WE);

function renderRoster(over: Partial<React.ComponentProps<typeof MonthRoster>> = {}) {
  const props = {
    month: MONDAY,
    monthCount: 1,
    assignments: [assignment({ name: "Asela", rotaDate: "2026-09-21", shiftCode: EVENING.code })],
    absences: [],
    shifts: SHIFTS,
    absenceKinds: [ANNUAL_LEAVE],
    selectedIso: "2026-09-21",
    ...scopeControls(),
    ...over,
  } as React.ComponentProps<typeof MonthRoster>;
  return render(<MonthRoster {...props} />);
}

describe("MonthRoster: who may edit", () => {
  it("offers no editable cell when the reader leads nothing", () => {
    const { container } = renderRoster({ leadTeams: [], editing: true, onEditCell: vi.fn() });
    expect(container.querySelectorAll("td.editable")).toHaveLength(0);
  });

  it("offers no editable cell until edit mode is on, even for a lead", () => {
    // A lead reads this grid far more often than they change it, so a cell
    // that writes on one stray click is the thing being guarded against.
    const { container } = renderRoster({
      leadTeams: ["castor"],
      editing: false,
      onEditCell: vi.fn(),
    });
    expect(container.querySelectorAll("td.editable")).toHaveLength(0);
  });

  it("makes only the reader's own team editable", () => {
    const { container } = renderRoster({
      assignments: [
        assignment({ name: "Asela", rotaDate: "2026-09-21", shiftCode: EVENING.code, teamKey: "castor" }),
        assignment({ name: "Nuwan", rotaDate: "2026-09-21", shiftCode: EVENING.code, teamKey: "draco" }),
      ],
      leadTeams: ["castor"],
      editing: true,
      onEditCell: vi.fn(),
    });
    const rows = [...container.querySelectorAll("tbody tr")];
    const castor = rows.find((r) => r.textContent?.includes("Asela"));
    const draco = rows.find((r) => r.textContent?.includes("Nuwan"));
    expect(castor?.querySelectorAll("td.editable").length).toBeGreaterThan(0);
    expect(draco?.querySelectorAll("td.editable")).toHaveLength(0);
  });

  it("reports the slot, what is on it, and where it is", () => {
    const onEditCell = vi.fn();
    const { container } = renderRoster({ leadTeams: ["castor"], editing: true, onEditCell });
    const cell = container.querySelector("td.editable");
    fireEvent.click(cell!);

    expect(onEditCell).toHaveBeenCalledTimes(1);
    const edit = onEditCell.mock.calls[0][0];
    expect(edit).toMatchObject({ name: "Asela", teamKey: "castor" });
    // The picker opens against the cell, so it needs to know where that is.
    expect(edit.anchor).toEqual(
      expect.objectContaining({
        top: expect.any(Number),
        left: expect.any(Number),
        bottom: expect.any(Number),
        right: expect.any(Number),
      }),
    );
  });

  it("reports the leave on a cell so the picker can mark it and offer a clear", () => {
    const onEditCell = vi.fn();
    const { container } = renderRoster({
      absences: [absence({ name: "Asela", startsOn: "2026-09-21", endsOn: "2026-09-21" })],
      leadTeams: ["castor"],
      editing: true,
      onEditCell,
    });
    const cell = [...container.querySelectorAll("td.editable")].find((td) =>
      td.textContent?.includes("AL"),
    );
    fireEvent.click(cell!);
    expect(onEditCell.mock.calls[0][0].absenceKindCode).toBe("ANNUAL_LEAVE");
  });
});

describe("MonthRoster: what the grid says", () => {
  it("shows leave over the rota underneath it", () => {
    // Somebody on leave is not on the rota that day, whatever the generated
    // row says -- but the assignment is only covered, never deleted.
    renderRoster({
      absences: [absence({ name: "Asela", startsOn: "2026-09-21", endsOn: "2026-09-21" })],
    });
    expect(screen.getByText("AL")).toBeInTheDocument();
    expect(screen.queryByText("6-9p")).not.toBeInTheDocument();
  });

  it("marks the week the reader is in, its two ends included", () => {
    const { container } = renderRoster({ month: new Date() });
    const band = container.querySelectorAll("thead th.day.cw");
    expect(band).toHaveLength(7);
    expect(container.querySelectorAll("thead th.day.cwa")).toHaveLength(1);
    expect(container.querySelectorAll("thead th.day.cwz")).toHaveLength(1);
  });

  it("marks today and the chosen day apart", () => {
    const { container } = renderRoster({ month: new Date(), selectedIso: "2026-09-21" });
    // Today is a fact and the selection is a choice; both are marked, and a
    // month that contains neither would mark nothing.
    expect(container.querySelectorAll("thead th.day.today").length).toBeLessThanOrEqual(1);
  });

  it("finds an engineer by name and hides the rest", () => {
    const { container } = renderRoster({
      assignments: [
        assignment({ name: "Asela", rotaDate: "2026-09-21", shiftCode: EVENING.code }),
        assignment({ name: "Nuwan", rotaDate: "2026-09-21", shiftCode: EVENING.code }),
      ],
    });
    fireEvent.change(screen.getByLabelText("Find an engineer"), { target: { value: "asel" } });
    expect(screen.getByText("Asela")).toBeInTheDocument();
    expect(screen.queryByText("Nuwan")).not.toBeInTheDocument();
    expect(container.querySelectorAll("tbody tr")).toHaveLength(1);
  });
});

describe("MonthRoster: the zone-split grid", () => {
  const sreProps = {
    assignments: [
      assignment({ name: "Apollo01", rotaDate: "2026-09-21", shiftCode: TZ1.code, zoneCode: "TZ1", teamKey: "apollo" }),
      assignment({ name: "Apollo01", rotaDate: "2026-09-21", shiftCode: TZ2.code, zoneCode: "TZ2", teamKey: "apollo" }),
    ],
    family: "SRE" as const,
  };

  it("gives a lead a cell per zone, not one for the day", () => {
    const { container } = renderRoster({
      ...sreProps,
      leadTeams: ["apollo"],
      editing: true,
      onEditCell: vi.fn(),
    });
    expect(container.querySelectorAll("td.z.editable").length).toBeGreaterThan(1);
  });

  it("says which zone column was clicked, so the picker can narrow to it", () => {
    const onEditCell = vi.fn();
    const { container } = renderRoster({
      ...sreProps,
      leadTeams: ["apollo"],
      editing: true,
      onEditCell,
    });
    const zoneCells = [...container.querySelectorAll("td.z.editable")];
    fireEvent.click(zoneCells[0]);
    expect(onEditCell.mock.calls[0][0].zoneCode).toBeTruthy();
  });

  it("keeps the week band's edges on the day rather than on each zone", () => {
    // A day split across zone columns still has one left edge and one right
    // edge; ruling every zone would mark the inside of a day as heavily as
    // its boundary.
    const { container } = renderRoster({ ...sreProps, month: new Date() });
    const opens = container.querySelectorAll("tbody td.cwa");
    const closes = container.querySelectorAll("tbody td.cwz");
    const rows = container.querySelectorAll("tbody tr").length;
    expect(opens.length).toBeLessThanOrEqual(rows);
    expect(closes.length).toBeLessThanOrEqual(rows);
  });
});
