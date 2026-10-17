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

import { describe, expect, it, vi } from "vitest";
import { csvField, fetchAllPages, rowsToCsvText } from "@utils/csvExport";

describe("csvField", () => {
  it("quotes a plain value", () => {
    expect(csvField("hello")).toBe('"hello"');
  });

  it("quotes an empty value", () => {
    expect(csvField("")).toBe('""');
  });

  it("keeps a value containing a comma in one field", () => {
    expect(csvField("a, b")).toBe('"a, b"');
  });

  it("doubles internal quotes", () => {
    expect(csvField('say "hi"')).toBe('"say ""hi"""');
  });

  it("keeps embedded newlines and carriage returns inside the quotes", () => {
    expect(csvField("line1\nline2")).toBe('"line1\nline2"');
    expect(csvField("a\rb")).toBe('"a\rb"');
    expect(csvField("a\r\nb")).toBe('"a\r\nb"');
  });

  it.each([
    ["=", "=SUM(A1:A9)"],
    ["+", "+1+1"],
    ["-", "-2+3"],
    ["@", "@SUM(A1)"],
    ["tab", "\t=1+1"],
    ["carriage return", "\r=1+1"],
  ])("neutralizes a value starting with %s by prefixing an apostrophe", (_, value) => {
    expect(csvField(value)).toBe(`"'${value}"`);
  });

  it("escapes quotes in a neutralized value too", () => {
    expect(csvField('=HYPERLINK("http://example.com")')).toBe(
      `"'=HYPERLINK(""http://example.com"")"`,
    );
  });

  it("only checks the first character", () => {
    expect(csvField("a=b")).toBe('"a=b"');
    expect(csvField("Jane-Doe @ example.com")).toBe('"Jane-Doe @ example.com"');
  });
});

describe("rowsToCsvText", () => {
  it("joins header and rows with CRLF, quoting every field", () => {
    const text = rowsToCsvText(["A", "B"], [["1", "2"], ["3", "4"]]);
    expect(text).toBe('"A","B"\r\n"1","2"\r\n"3","4"');
  });

  it("neutralizes formula-looking cells in data rows", () => {
    expect(rowsToCsvText(["Subject"], [["=cmd|' /C calc'!A0"]])).toBe(
      `"Subject"\r\n"'=cmd|' /C calc'!A0"`,
    );
  });
});

describe("fetchAllPages", () => {
  function makeFetcher(items: number[], pageErrorAt?: number) {
    return vi.fn(async (offset: number, limit: number) => {
      if (pageErrorAt !== undefined && offset === pageErrorAt) {
        throw new Error("network blip");
      }
      return {
        items: items.slice(offset, offset + limit),
        total: items.length,
      };
    });
  }

  it("pages until the result set is exhausted", async () => {
    const items = Array.from({ length: 125 }, (_, i) => i);
    const fetchPage = makeFetcher(items);
    const result = await fetchAllPages(fetchPage, { pageSize: 50 });

    expect(result.items).toEqual(items);
    expect(result.truncated).toBe(false);
    expect(result.total).toBe(125);
    // 125 rows at 50/page -> 3 requests (50 + 50 + 25).
    expect(fetchPage).toHaveBeenCalledTimes(3);
  });

  it("stops after a single page when total fits in it", async () => {
    const items = [1, 2, 3];
    const fetchPage = makeFetcher(items);
    const result = await fetchAllPages(fetchPage, { pageSize: 50 });
    expect(result.items).toEqual(items);
    expect(fetchPage).toHaveBeenCalledTimes(1);
  });

  it("returns an empty result without looping when there's nothing to fetch", async () => {
    const fetchPage = makeFetcher([]);
    const result = await fetchAllPages(fetchPage, { pageSize: 50 });
    expect(result.items).toEqual([]);
    expect(result.total).toBe(0);
    expect(result.truncated).toBe(false);
    expect(fetchPage).toHaveBeenCalledTimes(1);
  });

  it("truncates at the row cap instead of looping forever, and says so", async () => {
    const items = Array.from({ length: 500 }, (_, i) => i);
    const fetchPage = makeFetcher(items);
    const result = await fetchAllPages(fetchPage, { pageSize: 50, maxRows: 120 });

    // Cap of 120 rows at 50/page -> stops after the 3rd page (150 fetched,
    // still >= cap), never fetching all 500.
    expect(result.truncated).toBe(true);
    expect(result.items.length).toBeLessThan(500);
    expect(result.items.length).toBeGreaterThanOrEqual(120);
    expect(result.total).toBe(500);
  });

  it("reports progress after every page via onProgress", async () => {
    const items = Array.from({ length: 30 }, (_, i) => i);
    const fetchPage = makeFetcher(items);
    const onProgress = vi.fn();
    await fetchAllPages(fetchPage, { pageSize: 10, onProgress });

    expect(onProgress).toHaveBeenNthCalledWith(1, 10, 30);
    expect(onProgress).toHaveBeenNthCalledWith(2, 20, 30);
    expect(onProgress).toHaveBeenNthCalledWith(3, 30, 30);
  });

  it("propagates a mid-export page failure instead of returning a partial result", async () => {
    const items = Array.from({ length: 100 }, (_, i) => i);
    const fetchPage = makeFetcher(items, 50);
    await expect(fetchAllPages(fetchPage, { pageSize: 50 })).rejects.toThrow(
      "network blip",
    );
  });

  it("never loops forever if a page comes back empty despite a nonzero total", async () => {
    const fetchPage = vi.fn(async () => ({ items: [] as number[], total: 999 }));
    const result = await fetchAllPages(fetchPage, { pageSize: 50 });
    expect(result.items).toEqual([]);
    expect(fetchPage).toHaveBeenCalledTimes(1);
  });
});
