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

import { beforeEach, describe, expect, it, vi } from "vitest";
import { downloadPdfFile } from "@utils/pdf";
import { buildPdfTableBody } from "@utils/pdfStaffMarker";

// Real jsPDF, except save(): it is replaced by a capture of the generated PDF so the
// tests can read what actually went into the file.
const captured = vi.hoisted(() => ({ pdfs: [] as string[] }));

vi.mock("jspdf", async (importOriginal) => {
  const actual = await importOriginal<typeof import("jspdf")>();
  const RealJsPdf = actual.default;
  function CapturingJsPdf(...args: ConstructorParameters<typeof RealJsPdf>) {
    const doc = new RealJsPdf(...args);
    doc.save = (() => {
      captured.pdfs.push(doc.output());
      return doc;
    }) as typeof doc.save;
    return doc;
  }
  return { ...actual, default: CapturingJsPdf };
});

const SLOT = "\u00A0".repeat(4);

describe("buildPdfTableBody", () => {
  it("keeps the name as plain text and reserves a slot for the marker", () => {
    const { body, staffMarkerCells } = buildPdfTableBody([
      ["CHG1", "Danidu Wijendra ⓦ"],
      ["CHG2", "Jane Smith"],
    ]);

    expect(body).toEqual([
      ["CHG1", `Danidu Wijendra${SLOT}`],
      ["CHG2", "Jane Smith"],
    ]);
    expect([...staffMarkerCells]).toEqual(["0:1"]);
  });

  it("identifies marked cells by row and column", () => {
    const { staffMarkerCells } = buildPdfTableBody([
      ["a", "b", "c"],
      ["d", "e ⓦ", "f"],
      ["g ⓦ", "h", "i ⓦ"],
    ]);

    expect([...staffMarkerCells].sort()).toEqual(["1:1", "2:0", "2:2"]);
  });

  it("leaves a cell with nothing but the marker empty and unmarked", () => {
    const { body, staffMarkerCells } = buildPdfTableBody([["ⓦ"]]);

    expect(body).toEqual([[""]]);
    expect(staffMarkerCells.size).toBe(0);
  });
});

describe("downloadPdfFile with WSO2 staff names", () => {
  beforeEach(() => {
    captured.pdfs.length = 0;
  });

  const rows = [
    ["CHG1", "Vimukthi Perera ⓦ"],
    ["CHG2", "Jane Smith"],
    ["CHG3", "Osura Weerasinghe ⓦ"],
  ];

  it("writes each name as plain text instead of letter-spaced UCS-2", () => {
    downloadPdfFile("change-requests.pdf", "change requests — Project", ["Number", "Assigned to"], rows);

    const pdf = captured.pdfs[0];
    expect(pdf).toMatch(/\(Vimukthi Perera\u00A0{4}\) Tj/);
    expect(pdf).toMatch(/\(Osura Weerasinghe\u00A0{4}\) Tj/);
    expect(pdf).toContain("(Jane Smith) Tj");
    expect(pdf).not.toContain("\u0000");
  });

  it("draws the circled w once per WSO2 staff name, and nowhere else", () => {
    downloadPdfFile("change-requests.pdf", "change requests — Project", ["Number", "Assigned to"], rows);

    const pdf = captured.pdfs[0];
    expect(pdf.match(/\(w\) Tj/g)).toHaveLength(2);
    // Four Bézier curves per circle.
    expect(pdf.match(/ c\n/g)).toHaveLength(2 * 4);
  });

  it("draws no marker when nobody on the page is WSO2 staff", () => {
    downloadPdfFile("change-requests.pdf", "title", ["Number", "Assigned to"], [["CHG1", "Jane Smith"]]);

    expect(captured.pdfs[0]).not.toContain("(w) Tj");
  });

  it("documents the failure it prevents: jsPDF garbles the whole string around an unencodable character", async () => {
    const { default: JsPdf } = await import("jspdf");
    const doc = new JsPdf();
    doc.text("Vimukthi Perera ⓦ", 10, 10);

    expect(doc.output()).toContain("\u0000");
  });
});
