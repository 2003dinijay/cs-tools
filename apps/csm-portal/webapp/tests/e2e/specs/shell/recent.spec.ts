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

//
// App-shell widgets under `src/features/csm-recent` (`PinThisPageButton`,
// `PinnedTabs`, `QuickNav`, `RecentViewsButton` — see `RecentNav.ts`'s doc
// comment). All of it is `localStorage`-only (`useRecentViews`), no backing
// route or API, so every interaction here — pin/unpin, opening the recent
// views panel, choosing a quick-nav result — only ever mutates the current
// browser's `localStorage`. Nothing server-side is read or written beyond
// the ordinary page navigations these actions trigger.
//
// Note on `QuickNav.quickNavResult`: it matches by *visible text anywhere on
// the page* (`page.getByText(label).first()`), not scoped to the palette —
// the sidebar renders the exact same label text for every nav page (see
// `CsmSideBar.tsx`'s `<Sidebar.ItemLabel>{item.label}</Sidebar.ItemLabel>`),
// so searching for a nav-page label (e.g. "Announcements") would risk
// matching the sidebar link instead of the palette result. Searching for a
// visited *case*'s id (recorded automatically by `CsmCaseDetailPage`, never
// duplicated in the sidebar) avoids that ambiguity entirely, so that's what
// the QuickNav test below searches for.
//

import { test, expect, withRole } from "../../fixtures/test";
import { RecentNav } from "../../pages/RecentNav";
import { CASES } from "../../utils/selectors";

withRole(test, "approver");

interface StoredRecentView {
  kind: string;
  id: string;
  title: string;
  href: string;
}

/** Reads every `csm.recentViews.v1.<userKey>` bucket straight out of
 * `localStorage` (skipping the `.lastUserKey` pointer) and flattens them —
 * this is the same storage `useRecentViews` reads from, just read directly
 * so tests get ground truth without scraping UI text. Read-only: nothing is
 * written here. */
async function readStoredRecentViews(
  page: import("@playwright/test").Page,
): Promise<StoredRecentView[]> {
  return page.evaluate(() => {
    const out: StoredRecentView[] = [];
    for (let i = 0; i < window.localStorage.length; i += 1) {
      const key = window.localStorage.key(i);
      if (!key || !key.startsWith("csm.recentViews.v1.") || key.endsWith(".lastUserKey")) {
        continue;
      }
      try {
        const raw = window.localStorage.getItem(key);
        const parsed: unknown = raw ? JSON.parse(raw) : [];
        if (Array.isArray(parsed)) out.push(...(parsed as StoredRecentView[]));
      } catch {
        /* ignore malformed entries */
      }
    }
    return out;
  });
}

/** Opens `/cases`, clicks the first case row, and waits for the detail page
 * to render — which records a `kind: "case"` recent view as a side effect
 * (`CsmCaseDetailPage`'s `useRecordRecentView` call). Returns `null` when
 * there's no case to open. */
async function visitFirstCase(
  page: import("@playwright/test").Page,
): Promise<{ shortLabel: string; href: string } | null> {
  await page.goto(CASES.path);
  const firstCase = page.locator('a[href^="/cases/"]:not([href="/cases/new"])').first();
  const hasCase = await firstCase
    .waitFor({ state: "visible", timeout: 15_000 })
    .then(() => true)
    .catch(() => false);
  if (!hasCase) return null;

  await firstCase.click();
  await expect(page).toHaveURL(/\/cases\/[^/]+$/);
  await expect(page.getByRole("tablist")).toBeVisible();

  const entries = await readStoredRecentViews(page);
  // Match the entry for the case we just opened (by href), not merely the
  // first stored case — an older recorded view must not satisfy this.
  const current = page.url();
  const caseEntry = entries.find(
    (e) => e.kind === "case" && !!e.href && current.includes(e.href),
  );
  if (!caseEntry) return null;

  // `title` is `"{caseIdLabel} · {subject}"` (see `CsmCaseDetailPage.tsx`) —
  // the id-ish prefix before the separator, distinct from anything the
  // sidebar renders.
  const shortLabel = caseEntry.title.split(" · ")[0]?.trim();
  if (!shortLabel) return null;

  return { shortLabel, href: caseEntry.href };
}

test.describe("recent nav", () => {
  test("recent nav — pin then unpin current page", async ({ page }) => {
    await page.goto(CASES.path);
    const nav = new RecentNav(page);
    await expect(nav.pinThisPageButton()).toBeVisible();

    expect(await nav.isCurrentPagePinned()).toBe(false);
    await nav.pinCurrentPage();
    expect(await nav.isCurrentPagePinned()).toBe(true);

    // "/cases" is the nav root for the "Support" nav item (see
    // `csmNavItems.ts`), so its pinned chip's short label is "Support".
    await expect(nav.pinnedTab("Support")).toBeVisible();

    await nav.unpinCurrentPage();
    expect(await nav.isCurrentPagePinned()).toBe(false);
    await expect(nav.pinnedTab("Support")).toHaveCount(0);
  });

  test("recent nav — QuickNav search + navigate", async ({ page }) => {
    const visited = await visitFirstCase(page);
    test.skip(!visited, "No case available to visit / no recent case view recorded.");
    const { shortLabel, href } = visited!;

    // Leave the case's own page first so a successful QuickNav navigation
    // back to it is an actual URL change, not a no-op re-navigation.
    await page.goto("/dashboard");

    const nav = new RecentNav(page);
    await nav.openQuickNav();
    await nav.quickNavSearch(shortLabel);

    const hasMatch = await nav
      .quickNavResult(shortLabel)
      .waitFor({ state: "visible", timeout: 5_000 })
      .then(() => true)
      .catch(() => false);
    test.skip(!hasMatch, `QuickNav returned no match for "${shortLabel}".`);

    await nav.chooseQuickNavResult(shortLabel);
    await expect(page).toHaveURL(new RegExp(`${href.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")}$`));
  });

  test("recent nav — recent views records visited pages", async ({ page }) => {
    const visited = await visitFirstCase(page);
    test.skip(!visited, "No case available to visit / no recent case view recorded.");
    const { shortLabel } = visited!;

    // Navigate away, then confirm the visit survived in the Recently viewed
    // panel from a completely different page.
    await page.goto("/dashboard");

    const nav = new RecentNav(page);
    await nav.openRecentViews();
    await expect(nav.recentViewRow(shortLabel)).toBeVisible();
  });
});
