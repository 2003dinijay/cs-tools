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

import { describe, expect, it } from "vitest";
import { hasWso2StaffMarker, toPdfSafeText } from "@utils/pdfText";

describe("toPdfSafeText", () => {
  it("removes the WSO2 staff marker and the space before it (it is drawn separately)", () => {
    expect(toPdfSafeText("Danidu Wijendra ⓦ")).toBe("Danidu Wijendra");
    expect(toPdfSafeText("Ada Lovelace\u00A0ⓦ")).toBe("Ada Lovelace");
  });

  it("leaves Latin-1 text and the cp1252 punctuation jsPDF can draw untouched", () => {
    const text = "José Müller – “quoted” … 5 € • Ørsted ß";
    expect(toPdfSafeText(text)).toBe(text);
  });

  it("returns an empty string for null, undefined and empty input", () => {
    expect(toPdfSafeText(null)).toBe("");
    expect(toPdfSafeText(undefined)).toBe("");
    expect(toPdfSafeText("")).toBe("");
  });

  it("removes zero-width and directional characters", () => {
    expect(toPdfSafeText("Jo\u200Bhn\u200E Doe\uFEFF")).toBe("John Doe");
  });

  it("draws other Unicode spaces as a plain space", () => {
    expect(toPdfSafeText("A\u2009B\u3000C")).toBe("A B C");
  });

  it("folds characters to a plain letter where one exists", () => {
    expect(toPdfSafeText("ń ź ś č")).toBe("n z s c");
    expect(toPdfSafeText("① Ａ")).toBe("1 A");
  });

  it("replaces characters with no encodable form by a single '?'", () => {
    expect(toPdfSafeText("李雷")).toBe("??");
    expect(toPdfSafeText("done 🎉")).toBe("done ?");
    expect(toPdfSafeText("Łukasz")).toBe("?ukasz");
  });

  it("only ever returns characters jsPDF's built-in fonts can encode", () => {
    const out = toPdfSafeText("Danidu ⓦ 李 🎉 Łukasz ✓ “x” – ok");
    const encodable = (char: string) =>
      (char.codePointAt(0) ?? 0) <= 0xff || "“”–".includes(char);
    expect([...out].every(encodable)).toBe(true);
  });
});

describe("hasWso2StaffMarker", () => {
  it("is true only for text that carries the WSO2 staff marker", () => {
    expect(hasWso2StaffMarker("Danidu Wijendra ⓦ")).toBe(true);
    expect(hasWso2StaffMarker("Danidu Wijendra")).toBe(false);
    expect(hasWso2StaffMarker("")).toBe(false);
    expect(hasWso2StaffMarker(null)).toBe(false);
    expect(hasWso2StaffMarker(undefined)).toBe(false);
  });
});
