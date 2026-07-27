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
// "My time sheets" tab, backed by the real POST /time-cards/search
// (client-grouped into weeks). The backend requires `filters.projectIds` to
// be non-empty to return anything (confirmed live) — useMyTimeSheets now
// defaults to every project the signed-in user can see when no explicit
// project filter is picked, so a just-created card is expected to appear
// here without the user touching the project filter.
//
// Unlike the Approvals/Reject tabs' card component, the "My time sheets"
// card (`TimeSheetCard`) doesn't render the case number as row text at all
// (confirmed live, 2026-07-26 — a row only shows the project, a relative
// timestamp, duration, billable state, and a status chip) — it only shows
// up once you open that row's own "View details" dialog. So
// `TimeCardsPage.cardRow()`'s hasText match (which works fine on the other
// tabs) can never find a row here by case number; `expectNewestRowForCase`
// below verifies identity through that dialog instead. Cards render
// newest-first within this tab (see `cardRow`'s doc comment on ordering),
// so the newest is always the very first row on the page — including after
// narrowing by the Work item filter, since that only ever removes rows.
//

import { test, expect, withRole, approverSearchQuery } from "../../fixtures/test";
import { TimeCardsPage } from "../../pages/TimeCardsPage";
import { LogTimeDialog } from "../../pages/LogTimeDialog";
import { e2eWorkLogComment } from "../../utils/selectors";

/** Opens the first (newest) row's "View details" dialog and asserts it's
 * for `caseNumber`, then closes it. See the file-level note above for why
 * this — not `TimeCardsPage.cardRow()` — is how identity is checked here. */
async function expectNewestRowForCase(
  page: import("@playwright/test").Page,
  tc: TimeCardsPage,
  caseNumber: string,
): Promise<void> {
  // The list renders loading-skeleton rows immediately, then swaps in real
  // rows once `POST /time-cards/search` resolves — comfortably under 3s in
  // practice, but a freshly reloaded page (auth restore + the fetch itself)
  // can take noticeably longer than that, so this needs a generous timeout
  // rather than the page's own load event being enough of a signal.
  const firstRow = page.locator('[data-testid^="timecard-row-"]').first();
  await expect(firstRow).toBeVisible({ timeout: 10_000 });
  await firstRow.getByRole("button", { name: "View details" }).click();
  await expect(tc.reviewDialog().getByText(caseNumber, { exact: false }).first()).toBeVisible({ timeout: 5_000 });
  await tc.reviewDialog().getByRole("button", { name: "Close" }).click();
}

withRole(test, "approver");

/** Logs a real, uniquely-labelled time card from whichever case is first in
 * the list and returns its case number, or null if there's nothing to open. */
async function logTimeOnFirstCase(
  page: import("@playwright/test").Page,
  label: string,
): Promise<string | null> {
  const approverQuery = await approverSearchQuery(page);
  await page.goto("/cases");
  const firstCase = page
    .locator('a[href^="/cases/"]:not([href="/cases/new"])')
    .first();
  const hasCase = await firstCase
    .waitFor({ state: "visible", timeout: 15_000 })
    .then(() => true)
    .catch(() => false);
  if (!hasCase) return null;
  await firstCase.click();
  await expect(page).toHaveURL(/\/cases\/[^/]+$/);

  await page.getByRole("tab", { name: "Time tracking" }).click();
  const logTime = page.getByRole("button", { name: "Log time" });
  if (!(await logTime.isVisible().catch(() => false))) return null;

  await logTime.click();
  const dialog = new LogTimeDialog(page);
  await dialog.waitForOpen();
  const caseNumber = await dialog.caseNumber();
  await dialog.fillAndSubmit({
    hours: 1,
    workLogComment: e2eWorkLogComment(label),
    approverQuery,
  });
  return caseNumber;
}

test.describe("time cards — my time sheets", () => {
  test("page loads: tab, filters, and an empty/loaded state render", async ({ page }) => {
    const tc = new TimeCardsPage(page);
    await tc.goto();
    await expect(tc.myTab()).toBeVisible();
    await tc.openFilters();
    await expect(page.getByRole("combobox", { name: /^Project\s*\*?$/ })).toBeVisible();
    await expect(page.getByRole("combobox", { name: /^Work item\s*\*?$/ })).toBeVisible();
    await expect(page.getByRole("combobox", { name: /^State\s*\*?$/ })).toBeVisible();
    await expect(page.getByText("Could not load your time sheets.")).toHaveCount(0);
  });

  test("a newly logged card appears grouped in My time sheets", async ({ page }) => {
    test.setTimeout(90_000);
    const caseNumber = await logTimeOnFirstCase(page, "my-sheets display");
    test.skip(!caseNumber, "No open case available to log time against.");

    const tc = new TimeCardsPage(page);
    // A just-submitted card isn't always retrievable from
    // `POST /time-cards/search` the instant we land back on the list — the
    // real backend can lag between the create write and the record showing
    // up in a search/list read. Retry the list load (full reload) until the
    // just-logged card's row actually renders (identity checked via its
    // "View details" dialog — see the file-level note above).
    await expect(async () => {
      await tc.goto();
      await expectNewestRowForCase(page, tc, caseNumber!);
    }).toPass({ timeout: 60_000, intervals: [2_000, 3_000, 5_000, 8_000] });
  });

  test("state and work-item filters narrow to the matching card", async ({ page }) => {
    // Creates a real card (network round trip against live staging) *and*
    // drives three sequential filter interactions afterward — comfortably
    // exceeds the config default of 30s.
    test.setTimeout(90_000);
    const caseNumber = await logTimeOnFirstCase(page, "my-sheets filters");
    test.skip(!caseNumber, "No open case available to log time against.");

    const tc = new TimeCardsPage(page);
    // See the eventual-consistency note above — the card may not be
    // retrievable the instant we land back on the list.
    await expect(async () => {
      await tc.goto();
      await expectNewestRowForCase(page, tc, caseNumber!);
    }).toPass({ timeout: 60_000, intervals: [2_000, 3_000, 5_000, 8_000] });

    // Work item filter narrows to the matching card. It's a multi-select
    // sourced from case numbers on the current page, not free text, so
    // there's no equivalent "type something that can't match" case anymore
    // (nothing to select if it doesn't exist) — clearing instead confirms
    // the filter was actually applied and removable.
    await tc.filterWorkItem(caseNumber!);
    await expectNewestRowForCase(page, tc, caseNumber!);
    await tc.clearFilters();
    // "Clear filters" only shows up while a filter is active, so it going
    // away means the work item filter actually got removed.
    await expect(page.getByRole("button", { name: "Clear filters" })).toHaveCount(0);

    // State filter: the card was just created, so it's "submitted".
    await tc.filterState("Submitted");
    await expectNewestRowForCase(page, tc, caseNumber!);
  });
});
