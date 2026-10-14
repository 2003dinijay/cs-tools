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

import type { jsPDF } from "jspdf";
import type { CellHookData } from "jspdf-autotable";
import { hasWso2StaffMarker, toPdfSafeText } from "@utils/pdfText";

/**
 * Names of WSO2 staff end in "ⓦ", which is how a customer tells an engineer from a
 * colleague. jsPDF's built-in fonts cannot draw that character, so the PDF exports keep
 * the name as plain text, reserve a slot after it, and draw a circled "w" there
 * (see {@link drawWso2StaffMarkers}).
 *
 * Four no-break spaces (each 0.278em in Helvetica): one for the gap, three for the circle.
 * They stick to the last word, so the circle can never wrap onto a line of its own.
 */
const STAFF_MARKER_SLOT = "\u00A0".repeat(4);

/** Spaces of the slot taken by the circle itself (the rest is the gap before it). */
const CIRCLE_SLOT_SPACES = 3;

/** Size of the "w" relative to the surrounding text. */
const LETTER_SIZE_FACTOR = 0.65;

/** autotable's own line height factor; text lines are laid out with it (see autoTableText). */
const BASELINE_OFFSET_FACTOR = 2 - 1.15;

const cellKey = (rowIndex: number, columnIndex: number): string =>
  `${rowIndex}:${columnIndex}`;

export type PdfTableBody = {
  /** Table body as autotable should lay it out: safe text, plus a slot where a marker goes. */
  body: string[][];
  /** "row:column" of the body cells that belong to WSO2 staff. */
  staffMarkerCells: ReadonlySet<string>;
};

/**
 * Prepares table rows for a jsPDF autotable: every cell goes through
 * {@link toPdfSafeText}, and a cell that carried the WSO2 staff marker gets room for the
 * circled "w" that {@link drawWso2StaffMarkers} draws after the layout is known.
 *
 * @param rows - Table data rows.
 * @returns {PdfTableBody} The body to hand to autotable and the cells to mark.
 */
export function buildPdfTableBody(rows: string[][]): PdfTableBody {
  const staffMarkerCells = new Set<string>();
  const body = rows.map((row, rowIndex) =>
    row.map((cell, columnIndex) => {
      const text = toPdfSafeText(cell);
      if (!hasWso2StaffMarker(cell) || !text) return text;
      staffMarkerCells.add(cellKey(rowIndex, columnIndex));
      return text + STAFF_MARKER_SLOT;
    }),
  );
  return { body, staffMarkerCells };
}

/**
 * Builds the autotable `didDrawCell` hook that draws a circled "w" (the "ⓦ" of WSO2 staff
 * names) after the last line of each marked body cell.
 *
 * @param staffMarkerCells - Cells to mark, from {@link buildPdfTableBody}.
 * @returns {(data: CellHookData) => void} Hook for autotable's `didDrawCell` option.
 */
export function drawWso2StaffMarkers(
  staffMarkerCells: ReadonlySet<string>,
): (data: CellHookData) => void {
  return (data) => {
    if (data.section !== "body") return;
    if (!staffMarkerCells.has(cellKey(data.row.index, data.column.index))) return;

    const doc = data.doc as jsPDF;
    const { cell } = data;
    const lines = cell.text;
    if (lines.length === 0) return;

    const previous = {
      font: doc.getFont(),
      fontSize: doc.getFontSize(),
      lineWidth: doc.getLineWidth(),
      drawColor: doc.getDrawColor(),
      textColor: doc.getTextColor(),
    };

    // Measure with the cell's own font so the circle follows the text it belongs to.
    doc.setFont(cell.styles.font, cell.styles.fontStyle);
    doc.setFontSize(cell.styles.fontSize);

    const fontSize = doc.getFontSize() / doc.internal.scaleFactor;
    const lineHeight = fontSize * doc.getLineHeightFactor();
    const spaceWidth = doc.getTextWidth("\u00A0");

    // Baseline of the last line, exactly as autotable places it (autoTableText).
    const textPos = cell.getTextPos();
    let firstBaseline = textPos.y + fontSize * BASELINE_OFFSET_FACTOR;
    if (cell.styles.valign === "middle") firstBaseline -= (lines.length / 2) * lineHeight;
    else if (cell.styles.valign === "bottom") firstBaseline -= lines.length * lineHeight;
    const lastBaseline = firstBaseline + (lines.length - 1) * lineHeight;

    // Left edge of the last line (autotable aligns by the full string, slot included).
    const lastLine = lines[lines.length - 1];
    const lastLineWidth = doc.getTextWidth(lastLine);
    let lineLeft = textPos.x;
    if (cell.styles.halign === "right") lineLeft -= lastLineWidth;
    else if (cell.styles.halign === "center") lineLeft -= lastLineWidth / 2;

    const textWidth = doc.getTextWidth(lastLine.replace(/\u00A0+$/, ""));
    const radius = (CIRCLE_SLOT_SPACES * spaceWidth) / 2 - fontSize * 0.04;
    const centerX = lineLeft + textWidth + spaceWidth + (CIRCLE_SLOT_SPACES * spaceWidth) / 2;
    const centerY = lastBaseline - fontSize * 0.33;

    doc.setDrawColor(previous.textColor);
    doc.setLineWidth(fontSize * 0.06);
    doc.circle(centerX, centerY, radius, "S");

    // The "w": bold, sized to sit inside the circle, centred on its x-height (0.52em).
    const letterSize = fontSize * LETTER_SIZE_FACTOR;
    doc.setFont("helvetica", "bold");
    doc.setFontSize(doc.getFontSize() * LETTER_SIZE_FACTOR);
    doc.setTextColor(previous.textColor);
    doc.text("w", centerX, centerY + letterSize * 0.26, { align: "center" });

    doc.setFont(previous.font.fontName, previous.font.fontStyle);
    doc.setFontSize(previous.fontSize);
    doc.setLineWidth(previous.lineWidth);
    doc.setDrawColor(previous.drawColor);
    doc.setTextColor(previous.textColor);
  };
}
