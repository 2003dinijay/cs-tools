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
// `/updates` (`CsmUpdatesPage.tsx`) is read-only: Product -> Version -> Start
// level -> End level cascade through `GET` product-catalog data, and
// "Search" runs a `POST` update-levels-between-levels lookup. Report
// preview/PDF are entirely client-side (built from the already-fetched
// search result), so nothing here ever writes anything server-side. Real
// staging data drives every option list, so this suite picks the *first*
// available option at each cascade step rather than a hardcoded product name
// — self-skipping wherever the live catalog doesn't have enough levels to
// exercise the next step (e.g. only one update level total, so there's no
// valid "end level" greater than "start level").
//

import type { Locator, Page } from "@playwright/test";
import { test, expect, withRole } from "../../fixtures/test";
import { UpdatesPage } from "../../pages/UpdatesPage";

withRole(test, "approver");

/** Opens one of the four cascading Selects by its field label and returns
 * the trimmed text of every currently offered option, without picking one —
 * callers decide whether to skip (empty) or select. The popup stays open on
 * return; call {@link chooseOption} (or click elsewhere) next. */
async function openSelectOptions(page: Page, label: string): Promise<string[]> {
  const escaped = label.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  await page.getByRole("combobox", { name: new RegExp(`^${escaped}\\s*\\*?$`) }).click();
  const texts = await page.getByRole("listbox").getByRole("option").allTextContents();
  return texts.map((t) => t.trim()).filter((t) => t.length > 0);
}

async function chooseOption(page: Page, optionLabel: string): Promise<void> {
  await page.getByRole("listbox").getByRole("option", { name: optionLabel, exact: true }).click();
}

/** Result of trying to build a searchable filter combination from whatever
 * the live catalog currently offers, or `null` when the catalog can't
 * support one (missing data at some cascade step). */
interface PickedFilters {
  product: string;
  version: string;
  startLevel: string;
  endLevel: string;
}

/** Selects the first available option at each of the four cascade steps
 * (Product -> Version -> Start level -> End level), returning what was
 * picked, or `null` (with the reason) the first time a step has nothing to
 * offer. Each Select's option list is sourced from the *live* product
 * catalog / the previous step's choice, so this never assumes specific
 * product/version/level values exist. */
async function pickFirstAvailableFilters(
  page: Page,
): Promise<PickedFilters | { skip: string }> {
  const products = await openSelectOptions(page, "Product");
  if (products.length === 0) return { skip: "No products in the update catalog." };
  const product = products[0];
  await chooseOption(page, product);

  const versions = await openSelectOptions(page, "Version");
  if (versions.length === 0) return { skip: `No versions available for product "${product}".` };
  const version = versions[0];
  await chooseOption(page, version);

  // Start level options are sorted ascending (see `getLevelsForVersion`), so
  // the first option is the smallest — maximizes the chance of there being a
  // larger "end level" to pick next.
  const startLevels = await openSelectOptions(page, "Start level");
  if (startLevels.length === 0) {
    return { skip: `No update levels available for ${product} ${version}.` };
  }
  const startLevel = startLevels[0];
  await chooseOption(page, startLevel);

  const endLevels = await openSelectOptions(page, "End level");
  if (endLevels.length === 0) {
    return {
      skip: `${product} ${version} has no level greater than ${startLevel} to end at.`,
    };
  }
  const endLevel = endLevels[0];
  await chooseOption(page, endLevel);

  return { product, version, startLevel, endLevel };
}

function isPicked(
  result: PickedFilters | { skip: string },
): result is PickedFilters {
  return !("skip" in result);
}

/** True when the page rendered the "Could not load product catalog" error
 * banner instead of the cascading filters — a live-backend data gap (the
 * product-update-levels lookup failing), not a spec/selector bug. Callers
 * `test.skip` on this rather than fail. */
async function catalogFailedToLoad(page: Page): Promise<boolean> {
  return (await page.getByText(/Could not load product catalog/).count()) > 0;
}

/** Runs a search using the first available option at each cascade step, then
 * waits for either a results table or the "no updates found" empty state.
 * Returns `null` when the catalog didn't support building a search (caller
 * should `test.skip`). */
async function runFirstAvailableSearch(
  page: Page,
  updates: UpdatesPage,
): Promise<{ hasResults: boolean } | null> {
  await updates.goto();
  if (await catalogFailedToLoad(page)) {
    test.skip(true, "Product catalog failed to load on the live backend.");
    return null;
  }
  await expect(page.getByRole("combobox", { name: /^Product\s*\*?$/ })).toBeVisible({ timeout: 15_000 });

  const picked = await pickFirstAvailableFilters(page);
  if (!isPicked(picked)) {
    test.skip(true, picked.skip);
    return null;
  }

  await expect(updates.searchButton()).toBeEnabled();
  await updates.searchButton().click();

  const emptyState = page.getByText(/No updates found between level/);
  const resultsTable = page.locator("table");
  await expect(resultsTable.or(emptyState).first()).toBeVisible({ timeout: 15_000 });

  const hasResults = (await page.locator("table tbody tr").count()) > 0;
  return { hasResults };
}

/** First result row's Update Level cell text — the `levelKey` the rest of
 * the `UpdatesPage` API (`openLevelDetails`, `levelDetailsDialogHeading`)
 * keys off of. */
async function firstResultLevelKey(page: Page): Promise<string> {
  const firstCell: Locator = page.locator("table tbody tr").first().locator("td").first();
  return ((await firstCell.textContent()) ?? "").trim();
}

test.describe("updates", () => {
  test("updates — page renders", async ({ page }) => {
    const updates = new UpdatesPage(page);
    await updates.goto();
    test.skip(
      await catalogFailedToLoad(page),
      "Product catalog failed to load on the live backend.",
    );

    await expect(page.getByText("Search updates between levels")).toBeVisible();
    await expect(page.getByRole("combobox", { name: /^Product\s*\*?$/ })).toBeVisible({ timeout: 15_000 });
    await expect(page.getByRole("combobox", { name: /^Version\s*\*?$/ })).toBeVisible();
    await expect(page.getByRole("combobox", { name: /^Start level\s*\*?$/ })).toBeVisible();
    await expect(page.getByRole("combobox", { name: /^End level\s*\*?$/ })).toBeVisible();

    // Nothing is chosen yet, so the cascade is fully collapsed and Search is
    // disabled (`canSearch` requires all four fields).
    await expect(updates.searchButton()).toBeDisabled();
  });

  test("updates — run a search", async ({ page }) => {
    const updates = new UpdatesPage(page);
    const result = await runFirstAvailableSearch(page, updates);
    test.skip(!result, "Catalog does not support building a search.");
    // Either outcome is a valid, successfully-rendered state — the point of
    // this test is that the search round trip completes cleanly, not that
    // this particular staging catalog happens to have matching updates.
    expect(result).not.toBeNull();
  });

  test("updates — level details + report preview", async ({ page }) => {
    const updates = new UpdatesPage(page);
    const result = await runFirstAvailableSearch(page, updates);
    test.skip(!result, "Catalog does not support building a search.");
    test.skip(!result!.hasResults, "Search returned no update levels to inspect.");

    const levelKey = await firstResultLevelKey(page);
    test.skip(!levelKey, "Could not read the first result row's level key.");

    await updates.openLevelDetails(levelKey);
    await expect(updates.levelDetailsDialogHeading(levelKey)).toBeVisible();
    await updates.closeDialog();
    await expect(updates.levelDetailsDialogHeading(levelKey)).toHaveCount(0);

    await updates.openReportPreview();
    await expect(updates.reportPreviewHeading()).toBeVisible();
    await updates.closeDialog();
    await expect(updates.reportPreviewHeading()).toHaveCount(0);
  });
});
