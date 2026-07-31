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

import { act, renderHook, waitFor } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import { downloadCsv } from "@utils/csvExport";
import { useFilteredCsvExport } from "@hooks/useFilteredCsvExport";

// Mock only `downloadCsv` (the browser-facing side effect) so the row-building
// / paging logic in `useFilteredCsvExport` still runs for real — this avoids
// needing to reconstruct CSV text out of a jsdom `Blob` (which lacks `.text()`
// and doesn't interop with the platform `Blob`/`Response` used elsewhere).
vi.mock("@utils/csvExport", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@utils/csvExport")>();
  return { ...actual, downloadCsv: vi.fn() };
});

interface Row {
  id: number;
  name: string;
}

function makeFetchPage(rows: Row[], failAtOffset?: number) {
  return vi.fn(async (offset: number, limit: number) => {
    if (failAtOffset !== undefined && offset === failAtOffset) {
      throw new Error("boom");
    }
    return { items: rows.slice(offset, offset + limit), total: rows.length };
  });
}

describe("useFilteredCsvExport", () => {
  beforeEach(() => {
    vi.mocked(downloadCsv).mockClear();
  });

  it("pages the full result set and downloads one CSV containing every row", async () => {
    const rows = Array.from({ length: 120 }, (_, i) => ({ id: i, name: `Row ${i}` }));
    const fetchPage = makeFetchPage(rows);
    const { result } = renderHook(() =>
      useFilteredCsvExport<Row>({
        header: ["ID", "Name"],
        toRow: (r) => [String(r.id), r.name],
        entityName: "widgets",
        fetchPage,
        pageSize: 50,
      }),
    );

    let outcome: Awaited<ReturnType<typeof result.current.runExport>> | undefined;
    await act(async () => {
      outcome = await result.current.runExport();
    });

    expect(outcome).toEqual({ truncated: false, rowCount: 120, total: 120 });
    expect(fetchPage).toHaveBeenCalledTimes(3); // 50 + 50 + 20
    expect(downloadCsv).toHaveBeenCalledTimes(1);

    const [header, csvRows, filename] = vi.mocked(downloadCsv).mock.calls[0];
    expect(filename).toMatch(/^widgets-export-.*\.csv$/);
    expect(filename).not.toContain("partial");
    expect(header).toEqual(["ID", "Name"]);
    expect(csvRows).toHaveLength(120);
    expect(csvRows[0]).toEqual(["0", "Row 0"]);
    expect(csvRows[119]).toEqual(["119", "Row 119"]);
  });

  it("tags the filename and reports truncation when the row cap is hit", async () => {
    const rows = Array.from({ length: 500 }, (_, i) => ({ id: i, name: `Row ${i}` }));
    const fetchPage = makeFetchPage(rows);
    const { result } = renderHook(() =>
      useFilteredCsvExport<Row>({
        header: ["ID", "Name"],
        toRow: (r) => [String(r.id), r.name],
        entityName: "widgets",
        fetchPage,
        pageSize: 50,
        maxRows: 100,
      }),
    );

    let outcome: Awaited<ReturnType<typeof result.current.runExport>> | undefined;
    await act(async () => {
      outcome = await result.current.runExport();
    });

    expect(outcome?.truncated).toBe(true);
    expect(outcome?.total).toBe(500);
    expect(outcome?.rowCount).toBeLessThan(500);
    const [, , filename] = vi.mocked(downloadCsv).mock.calls[0];
    expect(filename).toContain("-partial.csv");
  });

  it("reports live progress while paging, and clears it when done", async () => {
    const rows = Array.from({ length: 30 }, (_, i) => ({ id: i, name: `Row ${i}` }));
    const fetchPage = makeFetchPage(rows);
    const { result } = renderHook(() =>
      useFilteredCsvExport<Row>({
        header: ["ID", "Name"],
        toRow: (r) => [String(r.id), r.name],
        entityName: "widgets",
        fetchPage,
        pageSize: 10,
      }),
    );

    expect(result.current.progress).toBeNull();
    let runPromise!: Promise<unknown>;
    act(() => {
      runPromise = result.current.runExport();
    });
    await waitFor(() => expect(result.current.isExporting).toBe(true));
    await act(async () => {
      await runPromise;
    });

    expect(result.current.isExporting).toBe(false);
    expect(result.current.progress).toBeNull();
  });

  it("never downloads a file when a page fetch fails partway through", async () => {
    const rows = Array.from({ length: 200 }, (_, i) => ({ id: i, name: `Row ${i}` }));
    const fetchPage = makeFetchPage(rows, 50); // fails on the 2nd page
    const { result } = renderHook(() =>
      useFilteredCsvExport<Row>({
        header: ["ID", "Name"],
        toRow: (r) => [String(r.id), r.name],
        entityName: "widgets",
        fetchPage,
        pageSize: 50,
      }),
    );

    await expect(
      act(async () => {
        await result.current.runExport();
      }),
    ).rejects.toThrow("boom");

    expect(downloadCsv).not.toHaveBeenCalled();
    expect(result.current.isExporting).toBe(false);
  });

  it("always fetches against the latest config, even if it changed since mount", async () => {
    const initialFetch = makeFetchPage([{ id: 1, name: "stale" }]);
    const latestFetch = makeFetchPage([{ id: 2, name: "fresh" }]);

    const { result, rerender } = renderHook(
      (fetchPage: typeof initialFetch) =>
        useFilteredCsvExport<Row>({
          header: ["ID", "Name"],
          toRow: (r) => [String(r.id), r.name],
          entityName: "widgets",
          fetchPage,
        }),
      { initialProps: initialFetch },
    );

    rerender(latestFetch);

    await act(async () => {
      await result.current.runExport();
    });

    expect(initialFetch).not.toHaveBeenCalled();
    expect(latestFetch).toHaveBeenCalledTimes(1);
  });
});
