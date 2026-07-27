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

// Service-catalog variable classification + value encoding for the service request form. Ports
// the webapp's features/csm-operations/utils/catalogVariables.ts. ServiceNow returns the full raw
// variable set for a catalog item — including auto-populated "context" fields
// (project/deployment/product) and hidden system fields (case type, priority, ...) — so these
// must be classified client-side, exactly as the webapp (and, upstream of it, the customer portal)
// does. One deviation from the webapp: the Description field is a plain multiline TextField here
// rather than a rich-text editor (mirrors NewCasePage.tsx's own case-description field, and this
// app has no rich-text editor component), so its value is plain text, not HTML.

import type { CatalogItemVariableDto } from "@src/types";

// Variable `type` strings ServiceNow emits (matched case-insensitively).
export const VARIABLE_TYPE_SINGLE_LINE = "Single Line Text";
export const VARIABLE_TYPE_MULTI_LINE = "Multi Line Text";
export const VARIABLE_TYPE_SELECT = "Select Box";
export const VARIABLE_TYPE_CHECKBOX = "Checkbox";
export const VARIABLE_TYPE_RADIO = "Radio Buttons";

/** Boolean-ish field that renders a Yes/No dropdown (no choice list in the API). */
export function isChoiceField(variable: CatalogItemVariableDto): boolean {
  const t = (variable.type ?? "").trim().toLowerCase();
  return (
    t === VARIABLE_TYPE_SELECT.toLowerCase() ||
    t === VARIABLE_TYPE_RADIO.toLowerCase() ||
    t === VARIABLE_TYPE_CHECKBOX.toLowerCase()
  );
}

/** Multi-line free text (but not the Description field, which gets its own larger text area). */
export function isMultiLineField(variable: CatalogItemVariableDto): boolean {
  const t = (variable.type ?? "").trim().toLowerCase();
  return (
    (t === VARIABLE_TYPE_MULTI_LINE.toLowerCase() || t.includes("multi")) &&
    !isDescriptionField(variable.questionText ?? "")
  );
}

/** The Description field — rendered with a larger multiline text area. */
export function isDescriptionField(questionText: string): boolean {
  const normalized = (questionText ?? "")
    .replace(/^\s*\*?\s*/, "")
    .trim()
    .toLowerCase();
  return normalized === "description";
}

/** File Copy Path field — a plain (optional) text input, not an upload. */
export function isFileCopyPathField(variable: CatalogItemVariableDto): boolean {
  const q = (variable.questionText ?? "").trim();
  const t = (variable.type ?? "").trim();
  return /file\s*copy\s*path/i.test(q) || /file\s*copy\s*path/i.test(t);
}

function isAttachmentType(type: string): boolean {
  const t = (type ?? "").trim().toLowerCase();
  return (
    t === "attachment" ||
    t === "file" ||
    t === "file upload" ||
    t.includes("attachment") ||
    t.includes("file upload") ||
    (t.includes("file") && !t.includes("configuration")) ||
    t.includes("attach") ||
    t.includes("document") ||
    t.includes("upload")
  );
}

function isAttachmentFieldByQuestionText(questionText: string): boolean {
  const q = (questionText ?? "").trim().toLowerCase();
  if (!q) return false;
  const patterns = [
    /attachment/i,
    /attach\s/i,
    /attach$/i,
    /file\s*upload/i,
    /upload\s*file/i,
    /vulnerability\s*scan\s*report/i,
    /scan\s*report/i,
    /upload\s*document/i,
    /document\s*upload/i,
    /attach\s*report/i,
    /attach\s*document/i,
  ];
  return patterns.some((p) => p.test(q));
}

/** Attachment/file-upload field (by type or questionText). Optional + collected in the shared
 *  attachments section, so these are not rendered as text inputs. */
export function isAttachmentField(variable: CatalogItemVariableDto): boolean {
  // A File Copy Path field is a plain (optional) text input, not an upload — exclude it explicitly
  // since its type/label can contain "file" and would otherwise be swallowed by isAttachmentType.
  if (isFileCopyPathField(variable)) return false;
  return isAttachmentType(variable.type ?? "") || isAttachmentFieldByQuestionText(variable.questionText ?? "");
}

const CONTEXT_FIELD_PATTERNS = [/^project$/i, /^deployments?$/i, /^product$/i, /^wso2\s*product$/i, /^environment$/i];

const HIDDEN_FIELD_PATTERNS = [
  /^case\s*type$/i,
  /^service\s*request\s*category$/i,
  /^classification$/i,
  /^class\s*fication$/i,
  /^srns$/i,
  /^state$/i,
  /^assignment\s*group$/i,
  /^assigned\s*to$/i,
  /^priority$/i,
  /^impact$/i,
];

/** Auto-populated from the project/deployment/product cascade — not shown. */
export function isContextField(questionText: string): boolean {
  const normalized = questionText?.trim().toLowerCase() ?? "";
  return CONTEXT_FIELD_PATTERNS.some((p) => p.test(normalized));
}

/** System field ServiceNow defaults — sent by the backend, not shown. */
export function isHiddenField(questionText: string): boolean {
  const normalized = questionText?.trim().toLowerCase() ?? "";
  return HIDDEN_FIELD_PATTERNS.some((p) => p.test(normalized));
}

const DATE_TIME_FIELD_PATTERNS: RegExp[] = [
  /start\s*(date|time)/i,
  /end\s*(date|time)/i,
  /scheduled\s*(date|time|start|end)/i,
  /implementation\s*(date|time|start|end)/i,
  /planned\s*(start|end|date|time)/i,
  /actual\s*(start|end|date|time)/i,
  /date\s*(and\s*)?\/?\s*time/i,
  /time\s*(and\s*)?\/?\s*date/i,
];

/** Renders a datetime picker (by type or questionText). */
export function isDateTimeField(variable: CatalogItemVariableDto): boolean {
  const t = (variable.type ?? "").trim();
  if (/date.*time|datetime/i.test(t) || /^date$/i.test(t)) return true;
  const q = (variable.questionText ?? "").trim();
  return DATE_TIME_FIELD_PATTERNS.some((p) => p.test(q));
}

/** Strip the leading `*`/whitespace ServiceNow prefixes onto required labels. */
export function variableLabel(variable: CatalogItemVariableDto): string {
  return (variable.questionText ?? "").replace(/^\s*\*?\s*/, "").trim() || variable.id;
}

/** User-editable variables — excludes context and hidden fields, sorted by the backend's display
 *  order. This is the set the form renders and validates. */
export function getUserEditableVariables(variables: CatalogItemVariableDto[]): CatalogItemVariableDto[] {
  return variables
    .filter((v) => !isContextField(v.questionText ?? "") && !isHiddenField(v.questionText ?? ""))
    .sort((a, b) => (a.order ?? Number.MAX_SAFE_INTEGER) - (b.order ?? Number.MAX_SAFE_INTEGER));
}

/** First empty required field label, or null if all are filled. Hot fix (mirrors the webapp and,
 *  upstream, the customer portal): every typable field is mandatory; attachment and File Copy Path
 *  fields are optional and skipped. */
export function getFirstEmptyRequiredField(
  variables: CatalogItemVariableDto[],
  values: Record<string, string>,
): string | null {
  for (const v of getUserEditableVariables(variables)) {
    if (isAttachmentField(v) || isFileCopyPathField(v)) continue;
    if (!(values[v.id] ?? "").trim()) return variableLabel(v);
  }
  return null;
}

/** Encode a variable's raw input into the value the payload carries: datetime becomes an
 *  ISO-UTC string, everything else is sent as trimmed text. */
export function encodeVariableValue(variable: CatalogItemVariableDto, raw: string): string {
  const trimmed = (raw ?? "").trim();
  if (!trimmed) return "";
  if (isDateTimeField(variable)) {
    const ms = Date.parse(trimmed);
    return Number.isNaN(ms) ? trimmed : new Date(ms).toISOString();
  }
  return trimmed;
}

/** "YYYY-MM-DDTHH:MM" (the local-datetime wire format these fields store their state as) to a
 *  local Date, for handing the value to the DateTimePicker. Same browser-local semantics a native
 *  `datetime-local` input has. */
export function parseDateTimeLocal(value: string): Date | null {
  const match = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})$/.exec(value);
  if (!match) return null;
  const date = new Date(Number(match[1]), Number(match[2]) - 1, Number(match[3]), Number(match[4]), Number(match[5]));
  return Number.isNaN(date.getTime()) ? null : date;
}

/** Local Date back to "YYYY-MM-DDTHH:MM", the inverse of {@link parseDateTimeLocal}. */
export function formatDateTimeLocal(date: Date): string {
  const y = date.getFullYear();
  const mo = String(date.getMonth() + 1).padStart(2, "0");
  const d = String(date.getDate()).padStart(2, "0");
  const h = String(date.getHours()).padStart(2, "0");
  const mi = String(date.getMinutes()).padStart(2, "0");
  return `${y}-${mo}-${d}T${h}:${mi}`;
}
