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
// `/announcements` (`CsmAnnouncementsPage.tsx`) is a read-only list of cases
// of `type: "announcement"`, backed by `POST /cases/search`. There is no
// create/edit flow yet, so every test here is a pure read against real
// staging data — no case is ever created or mutated. Rows carry no
// accessible name of their own (bare `<a>` `RouterLink`s), so they are
// matched by `href`, same as the cases list.
//

import { test, expect, withRole } from "../../fixtures/test";
import { AnnouncementsPage } from "../../pages/AnnouncementsPage";

withRole(test, "approver");

/** Waits for the announcements list's own search request to resolve, then
 * returns however many rows landed on the current page. Set up the waiter
 * *before* triggering the navigation/interaction that fires it, so it can't
 * miss a request that resolves faster than the `await` after it. */
function waitForAnnouncementsSearch(
  page: import("@playwright/test").Page,
): Promise<import("@playwright/test").Response> {
  return page.waitForResponse(
    (r) => r.url().includes("/cases/search") && r.request().method() === "POST",
  );
}

test.describe("announcements", () => {
  test("announcements — list renders", async ({ page }) => {
    const ann = new AnnouncementsPage(page);
    const initialSearch = waitForAnnouncementsSearch(page);
    await ann.goto();
    await initialSearch;

    await expect(page.getByRole("heading", { name: "Announcements" })).toBeVisible();
    await expect(ann.searchBox()).toBeVisible();
    // State filter (`#announcements-filter-state`) and project filter
    // (`#announcements-filter-project`) are always rendered, regardless of
    // whether there's any data — assert their presence directly rather than
    // through a POM method (there's no "select" to make yet).
    await expect(page.locator("#announcements-filter-state")).toBeVisible();
    await expect(page.locator("#announcements-filter-project")).toBeVisible();
  });

  test("announcements — search + filters", async ({ page }) => {
    const ann = new AnnouncementsPage(page);
    const initialSearch = waitForAnnouncementsSearch(page);
    await ann.goto();
    await initialSearch;

    const initialCount = await ann.rowCount();
    test.skip(initialCount === 0, "No announcements available to search/filter.");

    // Search — a broad single-letter query is virtually guaranteed to match
    // something in a subject/number if there's any data at all; the point is
    // to exercise the debounced search round trip without asserting a
    // specific narrowed count (that depends on live data this suite doesn't
    // control).
    const searchResp = waitForAnnouncementsSearch(page);
    await ann.search("e");
    await searchResp;
    await expect(page.getByRole("heading", { name: "Announcements" })).toBeVisible();
    await expect(ann.clearSearchButton()).toBeVisible();

    // Clearing the search returns the query key to exactly what `goto()`
    // already fetched (empty `search`, same state/project filters), and
    // that entry is still within its 30s `staleTime` (see
    // `useSearchAnnouncements`) — React Query serves it straight from cache
    // instead of firing a new `/cases/search`, so don't wait on the network
    // here; assert the UI settled back to the unfiltered state instead.
    await ann.clearSearch();
    await expect(ann.searchBox()).toHaveValue("");
    await expect(ann.clearSearchButton()).toHaveCount(0);
    await expect(ann.rows()).toHaveCount(initialCount);

    // State filter — a fixed enum, "Open" is always a valid option regardless
    // of data. `clearFilters` only renders once a filter is active.
    await ann.selectStateFilter("Open");
    await expect(ann.clearFiltersButton()).toBeVisible();
    await ann.clearFilters();
    await expect(ann.clearFiltersButton()).toHaveCount(0);
  });

  test("announcements — open detail", async ({ page }) => {
    const ann = new AnnouncementsPage(page);
    const initialSearch = waitForAnnouncementsSearch(page);
    await ann.goto();
    await initialSearch;

    const count = await ann.rowCount();
    test.skip(count === 0, "No announcements available to open.");

    const href = await ann.rows().first().getAttribute("href");
    test.skip(!href, "First announcement row has no href.");
    const id = href!.replace("/announcements/", "");

    await ann.openAnnouncement(id);

    // `/announcements/:caseId` renders the same `CsmCaseDetailPage` as
    // `/cases/:caseId` (see `CaseDetailPage.ts`'s doc comment) — assert the
    // case-detail shell rendered without depending on this specific
    // announcement's (dynamic) subject as a heading.
    await expect(page).toHaveURL(new RegExp(`/announcements/${id}$`));
    await expect(page.getByRole("tablist")).toBeVisible();
  });
});
