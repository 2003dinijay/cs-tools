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

// One html2canvas of a whole report is taller than the browser will draw.
// Callers hand this one section at a time. Tables and repeated blocks break
// between rows so a page edge does not slice a case in half.
import html2canvas from "html2canvas";
import type { jsPDF } from "jspdf";

const SCALE = 2;
/** Stay under the usual 16384px canvas side. */
const MAX_CANVAS_SIDE = 12000;

export type PdfCaptureOptions = {
  width?: number;
  windowWidth?: number;
  /** Content height, in CSS pixels, that should land on one PDF page. */
  pageCssHeight: number;
};

function maxCaptureCssHeight(): number {
  return Math.floor(MAX_CANVAS_SIDE / SCALE);
}

function elementChildren(element: HTMLElement): HTMLElement[] {
  return Array.from(element.children).filter((node): node is HTMLElement => node instanceof HTMLElement);
}

function loosen(element: HTMLElement): void {
  element.style.minHeight = "0";
  element.style.height = "auto";
  element.style.display = "";
}

async function shoot(element: HTMLElement, opts: PdfCaptureOptions, crop?: { y: number; height: number }): Promise<HTMLCanvasElement> {
  return html2canvas(element, {
    scale: SCALE,
    backgroundColor: "#ffffff",
    logging: false,
    imageTimeout: 2000,
    width: opts.width,
    windowWidth: opts.windowWidth ?? opts.width,
    y: crop?.y,
    height: crop?.height,
  });
}

function paint(pdf: jsPDF, canvas: HTMLCanvasElement, started: boolean, pageCssHeight: number, elementCssHeight: number): boolean {
  const pdfW = pdf.internal.pageSize.getWidth();
  const pdfH = pdf.internal.pageSize.getHeight();
  const slicePx = Math.max(1, Math.round(pageCssHeight * (canvas.height / Math.max(elementCssHeight, 1))));
  let y = 0;
  do {
    const h = Math.min(slicePx, canvas.height - y);
    if (h <= 1) break;
    const slice = document.createElement("canvas");
    slice.width = canvas.width;
    slice.height = h;
    const ctx = slice.getContext("2d");
    if (!ctx) break;
    ctx.fillStyle = "#ffffff";
    ctx.fillRect(0, 0, slice.width, slice.height);
    ctx.drawImage(canvas, 0, y, canvas.width, h, 0, 0, canvas.width, h);
    if (started) pdf.addPage();
    started = true;
    const drawH = (h / canvas.width) * pdfW;
    pdf.addImage(slice.toDataURL("image/jpeg", 0.92), "JPEG", 0, 0, pdfW, Math.min(drawH, pdfH));
    y += h;
  } while (y < canvas.height - 2);
  return started;
}

function splitTable(element: HTMLElement, limit: number, parent: HTMLElement): HTMLElement[] | null {
  const table = element.querySelector("table");
  const tbody = table?.tBodies[0];
  if (!table || !tbody || tbody.rows.length <= 1) return null;
  const rows = Array.from(tbody.rows).map((row) => row.cloneNode(true) as HTMLTableRowElement);
  const template = element.cloneNode(true) as HTMLElement;
  loosen(template);
  const templateBody = template.querySelector("tbody");
  if (!templateBody) return null;
  templateBody.replaceChildren();
  const pieces: HTMLElement[] = [];
  let current = template.cloneNode(true) as HTMLElement;
  let body = current.querySelector("tbody");
  if (!body) return null;
  parent.appendChild(current);
  for (const row of rows) {
    body.appendChild(row);
    if (body.rows.length > 1 && current.scrollHeight > limit) {
      body.removeChild(row);
      parent.removeChild(current);
      pieces.push(current);
      current = template.cloneNode(true) as HTMLElement;
      body = current.querySelector("tbody");
      if (!body) return pieces;
      body.replaceChildren(row);
      parent.appendChild(current);
    }
  }
  parent.removeChild(current);
  if (body.rows.length > 0) pieces.push(current);
  return pieces.length > 0 ? pieces : null;
}

function splitBlocks(element: HTMLElement, blocks: HTMLElement[], limit: number, parent: HTMLElement): HTMLElement[] | null {
  if (blocks.length <= 1) return null;
  const pieces: HTMLElement[] = [];
  let current = element.cloneNode(false) as HTMLElement;
  loosen(current);
  parent.appendChild(current);
  for (const block of blocks) {
    const copy = block.cloneNode(true) as HTMLElement;
    current.appendChild(copy);
    if (current.childElementCount > 1 && current.scrollHeight > limit) {
      current.removeChild(copy);
      parent.removeChild(current);
      pieces.push(current);
      current = element.cloneNode(false) as HTMLElement;
      loosen(current);
      current.appendChild(copy);
      parent.appendChild(current);
    }
  }
  parent.removeChild(current);
  if (current.childElementCount > 0) pieces.push(current);
  return pieces.length > 0 ? pieces : null;
}

function splitDeep(element: HTMLElement, limit: number, parent: HTMLElement): HTMLElement[] | null {
  const tablePieces = splitTable(element, limit, parent);
  if (tablePieces) return tablePieces;
  const blocks = elementChildren(element);
  const blockPieces = splitBlocks(element, blocks, limit, parent);
  if (blockPieces) return blockPieces;
  if (blocks.length !== 1) return null;
  const inner = splitDeep(blocks[0], limit, parent);
  if (!inner) return null;
  return inner.map((piece) => {
    const outer = element.cloneNode(false) as HTMLElement;
    loosen(outer);
    outer.appendChild(piece);
    return outer;
  });
}

async function addPiece(pdf: jsPDF, piece: HTMLElement, started: boolean, opts: PdfCaptureOptions): Promise<boolean> {
  const cap = Math.min(opts.pageCssHeight, maxCaptureCssHeight());
  const total = Math.ceil(piece.scrollHeight);
  if (total <= cap) {
    const canvas = await shoot(piece, opts);
    return paint(pdf, canvas, started, opts.pageCssHeight, Math.max(piece.scrollHeight, 1));
  }
  let y = 0;
  while (y < total) {
    const h = Math.min(cap, total - y);
    const canvas = await shoot(piece, opts, { y, height: h });
    started = paint(pdf, canvas, started, opts.pageCssHeight, h);
    y += h;
  }
  return started;
}

/** Draws `element` into `pdf`, one page-sized capture at a time. */
export async function addElementPages(pdf: jsPDF, element: HTMLElement, started: boolean, opts: PdfCaptureOptions): Promise<boolean> {
  const parent = element.parentElement;
  const limit = Math.min(opts.pageCssHeight, maxCaptureCssHeight());
  if (!parent || element.scrollHeight <= limit) {
    const canvas = await shoot(element, opts);
    return paint(pdf, canvas, started, opts.pageCssHeight, Math.max(element.scrollHeight, 1));
  }

  const hidden = element.style.display;
  element.style.display = "none";
  let pieces: HTMLElement[] | null = null;
  try {
    pieces = splitDeep(element, limit, parent);
  } finally {
    element.style.display = hidden;
  }
  if (!pieces) {
    return addPiece(pdf, element, started, opts);
  }

  const marker = document.createComment("pdf-pages");
  parent.replaceChild(marker, element);
  try {
    for (const piece of pieces) {
      parent.insertBefore(piece, marker);
      started = await addPiece(pdf, piece, started, opts);
      piece.remove();
    }
  } finally {
    marker.replaceWith(element);
  }
  return started;
}

/** A cloned canvas is blank. Copy the bitmap so a chart survives the clone. */
export function copyCanvasPixels(source: ParentNode, clone: ParentNode): void {
  const from = source.querySelectorAll("canvas");
  const to = clone.querySelectorAll("canvas");
  from.forEach((src, index) => {
    const dest = to[index];
    if (!(src instanceof HTMLCanvasElement) || !(dest instanceof HTMLCanvasElement)) return;
    dest.width = src.width;
    dest.height = src.height;
    dest.getContext("2d")?.drawImage(src, 0, 0);
  });
}
