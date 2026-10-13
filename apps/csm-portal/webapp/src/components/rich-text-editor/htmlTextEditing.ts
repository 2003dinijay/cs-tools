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

/**
 * Edit the wording of HTML without re-serialising it.
 *
 * The announcement preview lets someone fix a word or a space directly on the
 * rendered, styled content. The edit must reach the HTML source as exactly that
 * edit and nothing else: the preview is a sanitised copy (it has lost, for
 * example, `target` on links), and a browser serialising it back would also
 * normalise quoting and tags, so writing the preview's HTML back would change
 * parts nobody touched.
 *
 * So this works on the source string. It finds the text between tags in the
 * source, pairs each piece with the matching text node in the preview, and when
 * a text node changes it splices only that piece of the source. Every other
 * character of the source is kept as it was.
 *
 * Pairing is checked, not assumed: if the source and the preview do not
 * contain the same text in the same order (a sanitiser removed an element with
 * text in it, say), there is no mapping and editing is not offered.
 */

/** Elements whose content is not rendered text, so never a text piece (script, style, ...). */
const RAW_TEXT_ELEMENTS = new Set(["script", "style", "textarea", "title"]);

/** HTML's own definition of whitespace, which excludes the non-breaking space. */
const NON_WHITESPACE = /[^ \t\n\r\f]/;

const collapseWhitespace = (s: string): string => s.replace(/[ \t\n\r\f]+/g, " ").trim();

/** A run of text between tags in the source: `source.slice(start, end)`. */
export interface SourceTextRun {
  start: number;
  end: number;
}

/**
 * The text between tags in an HTML source string, in order. Comments,
 * declarations, tags (including `>` inside quoted attribute values) and the
 * content of script/style-like elements are skipped. A `<` that does not start a
 * tag ("a < b") stays part of the text, as the HTML parser treats it.
 */
export function findTextRuns(html: string): SourceTextRun[] {
  const runs: SourceTextRun[] = [];
  const n = html.length;
  let i = 0;
  let textStart = 0;
  const flush = (end: number): void => {
    if (end > textStart) runs.push({ start: textStart, end });
  };

  while (i < n) {
    const lt = html.indexOf("<", i);
    if (lt === -1) break;
    const next = html.charAt(lt + 1);
    if (!/[A-Za-z/!?]/.test(next)) {
      i = lt + 1;
      continue;
    }
    flush(lt);

    if (html.startsWith("<!--", lt)) {
      const close = html.indexOf("-->", lt + 4);
      i = close === -1 ? n : close + 3;
      textStart = i;
      continue;
    }

    // Find the end of the tag, ignoring a ">" inside a quoted attribute value.
    let j = lt + 1;
    let quote: string | null = null;
    const isMarkup = next === "!" || next === "?";
    while (j < n) {
      const ch = html.charAt(j);
      if (!isMarkup && (ch === '"' || ch === "'")) {
        if (quote === null) quote = ch;
        else if (quote === ch) quote = null;
      } else if (ch === ">" && quote === null) {
        break;
      }
      j++;
    }
    const tagEnd = Math.min(j, n - 1);
    i = tagEnd + 1;
    textStart = i;

    const open = /^<([A-Za-z][A-Za-z0-9-]*)/.exec(html.slice(lt, tagEnd + 1));
    if (open && RAW_TEXT_ELEMENTS.has(open[1].toLowerCase()) && html.charAt(tagEnd - 1) !== "/") {
      const closeTag = new RegExp("</" + open[1] + "[\\s/>]", "i");
      const rest = closeTag.exec(html.slice(i));
      i = rest ? i + rest.index : n;
      textStart = i;
    }
  }
  flush(n);
  return runs;
}

/** The text a piece of HTML source stands for: entities decoded, as the parser would. */
export function decodeHtmlText(raw: string): string {
  const el = document.createElement("textarea");
  el.innerHTML = raw;
  return el.value.replace(/\r\n?/g, "\n");
}

/** Text as HTML source: only what must be escaped, plus the non-breaking space the browser types for a trailing space. */
export function encodeHtmlText(text: string): string {
  return text.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/\u00a0/g, "&nbsp;");
}

/** One piece of editable text: its place in the source, and the preview text node showing it. */
export interface TextMapEntry {
  node: Text;
  start: number;
  end: number;
  /** What the text node held when last in step with the source. */
  text: string;
  /** Source the HTML parser leaves out of the node: the newline right after `<pre>`. Kept when the text is rewritten. */
  sourcePrefix: string;
}

export interface TextMap {
  source: string;
  entries: TextMapEntry[];
  /** Whitespace-only text nodes present when the map was built (layout, not content). */
  layout: ReadonlySet<Node>;
}

function previewTextNodes(host: HTMLElement): Text[] {
  const walker = document.createTreeWalker(host, NodeFilter.SHOW_TEXT);
  const out: Text[] = [];
  for (let node = walker.nextNode(); node; node = walker.nextNode()) {
    let rawText = false;
    for (let el = node.parentElement; el && el !== host; el = el.parentElement) {
      if (RAW_TEXT_ELEMENTS.has(el.tagName.toLowerCase())) rawText = true;
    }
    if (!rawText) out.push(node as Text);
  }
  return out;
}

/**
 * The parser ignores one newline straight after `<pre>` or `<listing>` (written
 * literally or as a character reference); it is in the source but not in the
 * node. Returns that source text so it can be kept when the text is rewritten.
 */
function droppedPreNewline(raw: string, decoded: string, node: Text): string {
  const parent = node.parentElement;
  if (!parent || parent.firstChild !== node || !/^(pre|listing)$/i.test(parent.tagName)) return "";
  if (decoded !== `\n${node.nodeValue ?? ""}`) return "";
  return /^(?:\r\n|\n|\r|&#0*10;|&#x0*a;|&NewLine;)/i.exec(raw)?.[0] ?? "";
}

/**
 * Pair the text in `source` with the text nodes rendered in `host`, or return
 * null when they do not line up. Only text with something visible in it is
 * paired; whitespace between elements is layout and is never edited.
 */
export function buildTextMap(source: string, host: HTMLElement): TextMap | null {
  const runs = findTextRuns(source)
    .map((run) => ({ ...run, decoded: decodeHtmlText(source.slice(run.start, run.end)) }))
    .filter((run) => NON_WHITESPACE.test(run.decoded));
  const allNodes = previewTextNodes(host);
  const nodes = allNodes.filter((node) => NON_WHITESPACE.test(node.nodeValue ?? ""));
  if (runs.length !== nodes.length) return null;

  const entries: TextMapEntry[] = [];
  for (let i = 0; i < runs.length; i++) {
    const text = nodes[i].nodeValue ?? "";
    if (collapseWhitespace(runs[i].decoded) !== collapseWhitespace(text)) return null;
    entries.push({ node: nodes[i], start: runs[i].start, end: runs[i].end, text, sourcePrefix: droppedPreNewline(source.slice(runs[i].start, runs[i].end), runs[i].decoded, nodes[i]) });
  }
  const layout = new Set<Node>(allNodes.filter((node) => !NON_WHITESPACE.test(node.nodeValue ?? "")));
  return { source, entries, layout };
}

/**
 * Browsers sometimes put typed characters in a brand-new text node instead of
 * the one being edited, for example when the caret is at the end of a link.
 * Fold any such node back into `target`, the mapped node the edit was aimed
 * at, so the edit is not lost. Returns where the caret should go, or null if
 * there was nothing to fold.
 */
export function absorbStrayText(host: HTMLElement, map: TextMap, target: Node | null): { node: Text; offset: number } | null {
  if (!target || !map.entries.some((entry) => entry.node === target)) return null;
  const into = target as Text;
  const mapped = new Set<Node>(map.entries.map((entry) => entry.node));
  let caret: { node: Text; offset: number } | null = null;
  for (const node of previewTextNodes(host)) {
    if (node === into || mapped.has(node) || map.layout.has(node)) continue;
    const value = node.nodeValue ?? "";
    if (into.compareDocumentPosition(node) & Node.DOCUMENT_POSITION_PRECEDING) {
      into.insertData(0, value);
      caret = { node: into, offset: value.length };
    } else {
      into.appendData(value);
      caret = { node: into, offset: (into.nodeValue ?? "").length };
    }
    node.remove();
  }
  return caret;
}

/**
 * Write the preview's text edits into the source. A piece whose text node is
 * unchanged keeps its exact original source; a changed one is replaced by its
 * new text, escaped. The map is updated in place so later edits still line up.
 *
 * `emptied` is set when a text node was emptied or removed by the browser,
 * after which the preview should be redrawn from the new source.
 */
export function applyTextEdits(map: TextMap): { source: string; changed: boolean; emptied: boolean } {
  let out = "";
  let cursor = 0;
  let shift = 0;
  let changed = false;
  let emptied = false;

  for (const entry of map.entries) {
    const originalStart = entry.start;
    const originalEnd = entry.end;
    const current = entry.node.isConnected ? (entry.node.nodeValue ?? "") : "";
    out += map.source.slice(cursor, originalStart);

    let piece: string;
    if (current === entry.text && entry.node.isConnected) {
      piece = map.source.slice(originalStart, originalEnd);
    } else {
      piece = entry.sourcePrefix + encodeHtmlText(current);
      changed = true;
      if (current === "") emptied = true;
    }

    entry.start = originalStart + shift;
    shift += piece.length - (originalEnd - originalStart);
    entry.end = entry.start + piece.length;
    entry.text = current;
    out += piece;
    cursor = originalEnd;
  }
  out += map.source.slice(cursor);
  map.source = out;
  return { source: out, changed, emptied };
}

/** Input types that only change characters inside one text node, or replay our own edits. */
const TEXT_ONLY_INPUT_TYPES = new Set([
  "insertText",
  "insertReplacementText",
  "insertCompositionText",
  "deleteContentBackward",
  "deleteContentForward",
  "deleteWordBackward",
  "deleteWordForward",
  "deleteSoftLineBackward",
  "deleteSoftLineForward",
  "deleteByCut",
  "historyUndo",
  "historyRedo",
]);

export interface SelectionPoint {
  anchorNode: Node | null;
  anchorOffset: number;
  focusNode: Node | null;
  focusOffset: number;
}

/**
 * Whether a browser edit is safe to let through: it must change characters
 * inside a single mapped text node, never split, join, add or remove elements.
 * Everything else (Enter, formatting, dropping, pasting rich content) is
 * refused, which is what keeps the map valid.
 */
export function isTextOnlyEdit(mapped: ReadonlySet<Node>, selection: SelectionPoint, inputType: string): boolean {
  if (!TEXT_ONLY_INPUT_TYPES.has(inputType)) return false;
  if (inputType === "historyUndo" || inputType === "historyRedo") return true;

  const { anchorNode, anchorOffset, focusNode, focusOffset } = selection;
  if (!anchorNode || anchorNode !== focusNode || !mapped.has(anchorNode)) return false;

  const length = anchorNode.nodeValue?.length ?? 0;
  const from = Math.min(anchorOffset, focusOffset);
  const to = Math.max(anchorOffset, focusOffset);
  if (from === to) {
    if (/^delete.*Backward$/.test(inputType) && from === 0) return false;
    if (/^delete.*Forward$/.test(inputType) && from === length) return false;
  }
  return true;
}
