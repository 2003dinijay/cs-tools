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

import { useCallback, useRef, useState } from "react";
import { downloadCsv, fetchAllPages, type CsvExportPageFetcher } from "@utils/csvExport";

export interface UseFilteredCsvExportConfig<T> {
  /** Column headers, in display order — must match the listing's own column
   * order/labels so the export is recognizably "the same table as a file". */
  header: string[];
  /** Maps one fetched item to a CSV row, in the same order as `header`. */
  toRow: (item: T) => string[];
  /** Noun used to build the downloaded filename, e.g. "incidents",
   * "change-requests". Lower-case, hyphenated; no spaces. */
  entityName: string;
  /**
   * Fetches one page, `[offset, offset+limit)`, of the *current* filtered
   * search — bind whatever filters/sort are currently applied into this
   * closure. Re-supplied on every render (see the ref pattern below) so
   * `runExport` always fetches against whatever is selected at the moment the
   * user clicks Export, not whatever was selected when the hook first
   * mounted.
   */
  fetchPage: CsvExportPageFetcher<T>;
  /** Page size per request. Defaults to 50 (see `fetchAllPages`). */
  pageSize?: number;
  /** Row cap override. Defaults to `CSV_EXPORT_ROW_CAP`. */
  maxRows?: number;
}

export interface CsvExportProgress {
  loaded: number;
  total: number;
}

export interface UseFilteredCsvExportResult {
  isExporting: boolean;
  /** `null` when not currently exporting; otherwise rows fetched so far vs.
   * the server-reported total for the current filter. */
  progress: CsvExportProgress | null;
  /**
   * Kicks off the export: pages the current filtered search to exhaustion (or
   * the safety cap), then downloads one CSV file. Resolves with whether the
   * export was truncated by the row cap, or throws if a page fetch failed
   * partway through (in which case nothing is downloaded — no partial file is
   * ever handed to the user as if it were complete).
   */
  runExport: () => Promise<{ truncated: boolean; rowCount: number; total: number }>;
}

/**
 * Backs every "Export CSV" action on the CSM portal's filtered listing pages
 * (cases, service requests, incidents, change requests, and any other view
 * built on the shared cases search).
 *
 * There is no server-side export endpoint — a project decision, not an
 * oversight: exporting is client-side, paging the same list-search endpoint
 * the listing itself already uses, filters and sort included, until the full
 * filtered result set has been fetched (see `fetchAllPages`). This is
 * necessarily chattier and slower than a dedicated backend export for a large
 * result set (a month of incidents across a busy account can be hundreds of
 * rows, i.e. several roundtrips at the backend's 50-row page cap) — that
 * tradeoff is the accepted cost of not adding a backend endpoint, not a bug
 * in this hook.
 */
export function useFilteredCsvExport<T>(
  config: UseFilteredCsvExportConfig<T>,
): UseFilteredCsvExportResult {
  const [isExporting, setIsExporting] = useState(false);
  const [progress, setProgress] = useState<CsvExportProgress | null>(null);

  // Always read the latest config at call time (current filters/sort), not
  // whatever closure captured `runExport` on first render — `runExport`
  // itself is stable (empty deps) so callers don't need to worry about it
  // changing identity every keystroke of a filter field.
  const configRef = useRef(config);
  configRef.current = config;

  const runExport = useCallback(async () => {
    const { header, toRow, entityName, fetchPage, pageSize, maxRows } = configRef.current;
    setIsExporting(true);
    setProgress({ loaded: 0, total: 0 });
    try {
      const { items, truncated, total } = await fetchAllPages(fetchPage, {
        pageSize,
        maxRows,
        onProgress: (loaded, pageTotal) => setProgress({ loaded, total: pageTotal }),
      });

      const rows = items.map(toRow);
      const timestamp = new Date().toISOString().replace(/[:.]/g, "-");
      // A truncated export is tagged in the filename itself, not just an
      // on-screen notice — the notice can be missed or the file can be
      // shared/opened later, outside the app, with no way to tell it isn't
      // the complete set otherwise.
      const filename = `${entityName}-export-${timestamp}${truncated ? "-partial" : ""}.csv`;
      downloadCsv(header, rows, filename);

      return { truncated, rowCount: rows.length, total };
    } finally {
      setIsExporting(false);
      setProgress(null);
    }
  }, []);

  return { isExporting, progress, runExport };
}
