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

export interface Runbook {
  id: string;
  /** Alert title; an incident whose subject contains it gets this runbook. */
  alertName: string;
  /** Null when the alert is known but no runbook has been written yet. */
  runbookUrl: string | null;
  createdOn: string;
  createdBy: string;
  updatedOn: string;
  updatedBy: string;
}

export interface RunbookInput {
  alertName: string;
  runbookUrl: string | null;
}

export type RunbookCoverage = "all" | "linked" | "missing";

export function isValidRunbookUrl(value: string): boolean {
  if (!value.trim()) return true;
  try {
    const url = new URL(value.trim());
    return url.protocol === "https:" || url.protocol === "http:";
  } catch {
    return false;
  }
}

/** The runbook for an incident: the longest alert name its subject contains, case-insensitively. Mirrors entity-service. */
export function findRunbookForIncident(
  runbooks: Pick<Runbook, "alertName" | "runbookUrl">[],
  subject: string,
): Pick<Runbook, "alertName" | "runbookUrl"> | undefined {
  const haystack = subject.toLowerCase();
  let best: Pick<Runbook, "alertName" | "runbookUrl"> | undefined;
  let bestLen = 0;
  for (const rb of runbooks) {
    const needle = rb.alertName.trim().toLowerCase();
    if (needle && needle.length > bestLen && haystack.includes(needle)) {
      best = rb;
      bestLen = needle.length;
    }
  }
  return best;
}

/** The work note a new incident gets for its matched runbook, or the lack of one. */
export function runbookWorkNote(runbook: Pick<Runbook, "runbookUrl"> | undefined): string {
  if (runbook?.runbookUrl) {
    return `Runbook for this alert: ${runbook.runbookUrl}`;
  }
  return "No runbook available for this alert.";
}

/**
 * Reads an import file: a JSON object of alert name to link ("" for none),
 * or an array of {alertName, runbookUrl}. Throws with a readable message otherwise.
 */
export function parseRunbookImport(text: string): RunbookInput[] {
  let data: unknown;
  try {
    data = JSON.parse(text);
  } catch {
    throw new Error("The file is not valid JSON.");
  }
  const toInput = (alertName: unknown, url: unknown): RunbookInput => {
    if (typeof alertName !== "string" || !alertName.trim()) {
      throw new Error("Every entry needs an alert name.");
    }
    if (url !== null && url !== undefined && typeof url !== "string") {
      throw new Error(`The link for "${alertName}" is not text.`);
    }
    const link = typeof url === "string" ? url.trim() : "";
    if (!isValidRunbookUrl(link)) {
      throw new Error(`The link for "${alertName}" is not a full http(s) link.`);
    }
    return { alertName: alertName.trim(), runbookUrl: link || null };
  };
  if (Array.isArray(data)) {
    return data.map((row) => {
      const r = (row ?? {}) as Record<string, unknown>;
      return toInput(r.alertName, r.runbookUrl);
    });
  }
  if (data && typeof data === "object") {
    return Object.entries(data as Record<string, unknown>).map(([name, url]) => toInput(name, url));
  }
  throw new Error("Expected an object of alert name to link, or a list of entries.");
}
