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

import DOMPurify from "dompurify";
import { describe, expect, it } from "vitest";
import { normalizeAnnouncementDescriptionHtml } from "@features/announcements/utils/announcements";
import { INLINE_COMMENT_HTML_PURIFY } from "@features/support/utils/support";

/**
 * The announcement description the CSM portal stores for a bulletin: a styled
 * box (padding, rounded border, gradient) holding one row per advisory, with
 * inline-coloured links. This is the layout ServiceNow-created announcements
 * carry, and the one the CSM portal must now be able to store unchanged.
 */
const BULLETIN_HTML = `<p>Kindly refer to the <strong>Security Advisories</strong> section below.</p>
<p>Please note that only versions affected according to our<a style="color: #ff7300 !important; text-decoration: none;" href="https://example.test/policy" target="_blank" rel="noopener noreferrer"> back porting policy</a> are included.</p>
<p><strong>Security Advisories</strong></p>
<div style="width: 100%;box-sizing: border-box;padding: 15px;border-radius: 12px;border: 1px solid rgba(128,128,128,0.25);background: linear-gradient(160deg, rgba(255,255,255,0.05) 0%, rgba(255,255,255,0) 60%), rgba(128,128,128,0.06);box-shadow: 0 2px 8px rgba(0,0,0,0.12);">
  <div style="margin: 0;">
    <div><span><strong>WSO2-2026-0001: Security Advisory for WSO2 Identity Server</strong></span></div>
    <div>Severity: Medium</div>
    <div><a style="color: #1a8038 !important; text-decoration: underline;" href="https://example.test/a.pdf">Download here</a></div>
  </div>
</div>`;

/** What AnnouncementDetailsPanel does to the description before putting it on the page. */
function renderedDescription(html: string): string {
  return DOMPurify.sanitize(normalizeAnnouncementDescriptionHtml(html), INLINE_COMMENT_HTML_PURIFY);
}

describe("announcement description rendering keeps a styled bulletin intact", () => {
  const out = renderedDescription(BULLETIN_HTML);

  it("keeps the bordered, padded, gradient box and the rows inside it", () => {
    const doc = new DOMParser().parseFromString(out, "text/html");
    const box = [...doc.querySelectorAll("div")].find((d) => /border-radius/.test(d.getAttribute("style") ?? ""));
    expect(box).toBeDefined();
    expect(box?.getAttribute("style")).toContain("padding: 15px");
    expect(box?.getAttribute("style")).toContain("linear-gradient");
    expect(box?.textContent).toContain("WSO2-2026-0001");
    expect(box?.textContent).toContain("Severity: Medium");
  });

  it("keeps every inline-coloured link", () => {
    const doc = new DOMParser().parseFromString(out, "text/html");
    const links = [...doc.querySelectorAll("a")];
    expect(links).toHaveLength(2);
    expect(links.map((a) => a.getAttribute("style"))).toEqual([
      expect.stringContaining("#ff7300"),
      expect.stringContaining("#1a8038"),
    ]);
  });

  it("does not lose any element or inline style", () => {
    const count = (html: string, re: RegExp): number => (html.match(re) ?? []).length;
    expect(count(out, /<div/g)).toBe(count(BULLETIN_HTML, /<div/g));
    expect(count(out, /<span/g)).toBe(count(BULLETIN_HTML, /<span/g));
    expect(count(out, /style="/g)).toBe(count(BULLETIN_HTML, /style="/g));
  });

  it("still removes script and event handlers from announcement content", () => {
    const hostile = renderedDescription('<div style="padding:1px" onclick="alert(1)">x<script>alert(2)</script><img src=x onerror="alert(3)"></div>');
    expect(hostile).not.toMatch(/<script/i);
    expect(hostile).not.toMatch(/onclick|onerror/i);
    expect(hostile).toContain("padding");
  });
});
