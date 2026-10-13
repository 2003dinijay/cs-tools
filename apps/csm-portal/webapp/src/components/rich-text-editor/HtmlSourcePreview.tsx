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

import { Box, Button, Typography } from "@wso2/oxygen-ui";
import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState, type JSX } from "react";
import { absorbStrayText, applyTextEdits, buildTextMap, isTextOnlyEdit, type TextMap } from "@components/rich-text-editor/htmlTextEditing";
import { sanitizeRichTextHtml } from "@utils/sanitizeHtml";

/**
 * Shows how HTML typed in the source box will look, and optionally lets the
 * wording and spacing be fixed right on it.
 *
 * The rich editor cannot be used for this when the markup has styling it cannot
 * represent (a bordered box, coloured links): loading it there rewrites it. So
 * the source view gets its own rendering instead, prepared the way the portals
 * prepare backend HTML for display: DOMPurify's default policy, the same as the
 * announcement views, with the styling left as written (including in dark mode,
 * so the preview matches what the announcement will look like). Scripts, event
 * handlers and `javascript:` links are removed, exactly as they are for customers.
 *
 * With `onChange`, "Edit wording in preview" makes the rendering editable for
 * TEXT ONLY: typing, deleting and fixing spaces inside the text that is already
 * there. Anything that would add, remove, split or join elements (Enter,
 * formatting, dropping, pasting rich content, deleting across elements) is
 * refused. Each edit is written into the SOURCE string as that edit alone (see
 * htmlTextEditing), never by serialising the preview, so every character that
 * was not edited stays exactly as it was, including what the sanitiser removes
 * from the preview.
 */
export default function HtmlSourcePreview({
  html,
  onChange,
}: {
  html: string;
  /** Receives the new source after each text edit. Omit it for a read-only preview. */
  onChange?: (html: string) => void;
}): JSX.Element | null {
  const safeHtml = useMemo(() => sanitizeRichTextHtml(html), [html]);
  const hasContent = html.trim() !== "";
  const canEdit = onChange !== undefined;
  // Whether the source and the preview line up well enough to write edits back.
  // Worked out on a scratch element, since the real one is only drawn in an effect.
  const editable = useMemo(() => {
    if (!canEdit || !hasContent) return false;
    const scratch = document.createElement("div");
    scratch.innerHTML = safeHtml;
    return buildTextMap(html, scratch) !== null;
  }, [canEdit, hasContent, html, safeHtml]);

  const hostRef = useRef<HTMLDivElement>(null);
  const mapRef = useRef<TextMap | null>(null);
  // The source we last handed to onChange. When it comes straight back as `html`
  // the preview already shows it, so redrawing would only move the caret.
  const lastEmittedRef = useRef<string | null>(null);
  const redrawRef = useRef(false);
  const [editing, setEditing] = useState(false);
  const [redrawTick, setRedrawTick] = useState(0);

  // The preview is drawn here, not through React, so that an edit made in it is
  // not wiped by a re-render. The HTML is sanitised above.
  useLayoutEffect(() => {
    const host = hostRef.current;
    if (!host) return;
    if (!redrawRef.current && lastEmittedRef.current === html) return;
    redrawRef.current = false;
    lastEmittedRef.current = null;
    host.innerHTML = safeHtml;
    mapRef.current = canEdit ? buildTextMap(html, host) : null;
  }, [html, safeHtml, canEdit, hasContent, redrawTick]);

  // The text node the last native edit was aimed at, so text the browser puts
  // in a new node next to it can be folded back in.
  const targetRef = useRef<Node | null>(null);

  const syncEdits = useCallback((composing = false) => {
    const map = mapRef.current;
    const host = hostRef.current;
    if (!map || !onChange) return;
    if (!composing && host) {
      const caret = absorbStrayText(host, map, targetRef.current);
      if (caret) window.getSelection()?.collapse(caret.node, caret.offset);
    }
    const { source, changed, emptied } = applyTextEdits(map);
    if (!changed) return;
    lastEmittedRef.current = source;
    if (emptied) {
      // A text node was emptied or removed, so the map no longer matches the
      // page: redraw from the new source once the parent has handed it back.
      redrawRef.current = true;
      setRedrawTick((t) => t + 1);
    }
    onChange(source);
  }, [onChange]);

  useEffect(() => {
    const host = hostRef.current;
    if (!host || !editing || !editable) return;
    host.focus();

    const onBeforeInput = (event: Event): void => {
      const e = event as InputEvent;
      const map = mapRef.current;
      if (!map) {
        e.preventDefault();
        return;
      }
      if (e.inputType === "insertFromPaste") {
        // Pasted rich content would bring elements in. Take the plain text only.
        e.preventDefault();
        const pasted = (e.dataTransfer?.getData("text/plain") ?? "").replace(/\s*\r?\n\s*/g, " ");
        if (pasted && typeof document.execCommand === "function") document.execCommand("insertText", false, pasted);
        return;
      }
      const mapped = new Set<Node>(map.entries.map((entry) => entry.node));
      const selection = window.getSelection();
      targetRef.current = selection?.anchorNode ?? null;
      let allowed = isTextOnlyEdit(
        mapped,
        {
          anchorNode: selection?.anchorNode ?? null,
          anchorOffset: selection?.anchorOffset ?? 0,
          focusNode: selection?.focusNode ?? null,
          focusOffset: selection?.focusOffset ?? 0,
        },
        e.inputType,
      );
      // Where the browser says exactly what the edit touches, hold it to the same rule.
      if (allowed && !e.inputType.startsWith("history")) {
        for (const range of e.getTargetRanges?.() ?? []) {
          if (range.startContainer !== range.endContainer || !mapped.has(range.startContainer)) allowed = false;
        }
      }
      if (!allowed) e.preventDefault();
    };
    const refuse = (event: Event): void => event.preventDefault();

    host.addEventListener("beforeinput", onBeforeInput);
    host.addEventListener("drop", refuse);
    host.addEventListener("dragstart", refuse);
    return () => {
      host.removeEventListener("beforeinput", onBeforeInput);
      host.removeEventListener("drop", refuse);
      host.removeEventListener("dragstart", refuse);
    };
  }, [editing, editable]);

  if (!hasContent) return null;

  const isEditing = editing && editable;

  return (
    <Box sx={{ mt: 1.5 }}>
      <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 1, mb: 0.5 }}>
        <Typography variant="caption" color="text.secondary">
          Preview: how this will look when shown to customers
        </Typography>
        {canEdit && editable && (
          <Button size="small" variant="text" aria-pressed={isEditing} onClick={() => setEditing((v) => !v)}>
            {isEditing ? "Done editing" : "Edit wording in preview"}
          </Button>
        )}
      </Box>
      <Box
        ref={hostRef}
        data-testid="html-source-preview"
        contentEditable={isEditing}
        suppressContentEditableWarning
        role={isEditing ? "textbox" : undefined}
        aria-multiline={isEditing ? true : undefined}
        aria-label={isEditing ? "Announcement text, editable" : undefined}
        spellCheck={isEditing}
        onInput={isEditing ? (event) => syncEdits((event.nativeEvent as InputEvent).isComposing) : undefined}
        onCompositionEnd={isEditing ? () => syncEdits() : undefined}
        sx={{
          border: 1,
          borderColor: isEditing ? "primary.main" : "divider",
          borderRadius: 1,
          p: 1.5,
          maxHeight: 420,
          overflow: "auto",
          overflowWrap: "anywhere",
          fontSize: "0.875rem",
          outline: isEditing ? "2px solid" : "none",
          outlineColor: "primary.main",
          cursor: isEditing ? "text" : "default",
        }}
      />
      {isEditing && (
        <Typography variant="caption" color="text.secondary" sx={{ display: "block", mt: 0.5 }}>
          Click in the text to fix wording or spacing. Each change is written to the HTML source above; styling, links
          and structure are left as they are. Use the source box for anything else.
        </Typography>
      )}
      {canEdit && !editable && (
        <Typography variant="caption" color="text.secondary" sx={{ display: "block", mt: 0.5 }}>
          Text can't be edited in the preview for this markup. Use the HTML source box.
        </Typography>
      )}
    </Box>
  );
}
