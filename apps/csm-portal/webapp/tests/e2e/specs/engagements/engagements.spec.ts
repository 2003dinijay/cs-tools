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
// Engagements (`/engagements`, `CsmEngagementsPage.tsx`) — a thin
// `CsmIssuesView` wrapper locked to `caseTypes: ["engagement"]`, with the
// case-type filter hidden and the engagement-type filter shown instead.
// Detail opens the same `CsmCaseDetailPage` used by `/cases/:id` (no
// tab-set reduction for engagements — only announcements trim tabs, see
// `CsmCaseDetailPage.tsx`'s `isAnnouncement` filter), just under
// `/engagements/:caseId`.
//
// READ-ONLY: there is no engagement-create flow, and per the write-safety
// rule this spec must never mutate a pre-existing engagement case. Nothing
// here submits a form, clicks a lifecycle/action button, or posts a
// comment — only navigation, search, and filter selection.
//

import { test, expect, withRole } from "../../fixtures/test";
import { CasesListPage } from "../../pages/CasesListPage";
import { CaseDetailPage } from "../../pages/CaseDetailPage";
import { ENGAGEMENTS } from "../../utils/selectors";

withRole(test, "approver");

function engagementsListPage(page: import("@playwright/test").Page): CasesListPage {
  return new CasesListPage(page, ENGAGEMENTS.path, ENGAGEMENTS.path);
}

test.describe("engagements — list", () => {
  test("list renders with the engagement-type filter, and the type filter hidden", async ({
    page,
  }) => {
    const engagements = engagementsListPage(page);
    await engagements.goto();

    await expect(page.getByRole("heading", { name: ENGAGEMENTS.heading })).toBeVisible();
    await expect(engagements.searchBox()).toBeVisible();

    await engagements.openFilters();
    await expect(page.locator("#cases-filter-engagement-type")).toBeVisible();
    // `hideTypeFilter` on `CsmEngagementsPage` — the generic "Case type"
    // multi-select must not render at all on this locked-to-engagement view.
    await expect(page.locator("#cases-filter-type")).toHaveCount(0);
  });
});

test.describe("engagements — filters", () => {
  test("selecting engagement type and state filters keeps the list in a coherent state", async ({
    page,
  }) => {
    const engagements = engagementsListPage(page);
    await engagements.goto();

    const initialCount = await engagements.rowCount();
    test.skip(initialCount === 0, "No engagements on staging to filter over.");

    await engagements.selectFilterOption("cases-filter-engagement-type", "Migration");
    await engagements.selectFilterOption("cases-filter-state", "Open");

    // Same signal as the all-cases list spec: the toggle's accessible name
    // flips to "Clear filters (N)" once a selection is active.
    await expect(engagements.filtersToggleButton()).toHaveText(/^Clear filters/);

    const filteredCount = await engagements.rowCount();
    expect(filteredCount).toBeLessThanOrEqual(initialCount);

    await engagements.filtersToggleButton().click();
    await expect(engagements.filtersToggleButton()).toHaveText(/^Filters$/);
  });
});

test.describe("engagements — detail (read-only)", () => {
  test("opening the first engagement renders its detail page", async ({ page }) => {
    const engagements = engagementsListPage(page);
    await engagements.goto();

    const rowCount = await engagements.rowCount();
    test.skip(rowCount === 0, "No engagements on staging to open.");

    const href = await engagements.firstRow().getAttribute("href");
    const caseId = href?.split("/").pop();
    test.skip(!caseId, "Could not resolve the first engagement's case id from its row link.");

    const detail = new CaseDetailPage(page, ENGAGEMENTS.path);
    await detail.goto(caseId!);

    // Engagements render the same full tab set as an ordinary case detail
    // page (only announcements trim it down) — assert the tablist and a
    // couple of representative tabs are present, nothing more.
    await expect(page.getByRole("tablist")).toBeVisible();
    await expect(detail.tab("activities")).toBeVisible();
    await expect(detail.tab("details")).toBeVisible();

    // Read-only: this test never clicks a lifecycle button, "More" action,
    // or comment submit — only navigation and tab assertions above.
  });
});
