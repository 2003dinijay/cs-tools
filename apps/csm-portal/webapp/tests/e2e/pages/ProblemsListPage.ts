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

import { type Locator, type Page, expect } from "@playwright/test";

const PROBLEMS_TAB_PATH = "/operations?tab=problems";

/**
 * Page object for the Operations → "Problem management" tab
 * (`ProblemsTab.tsx`). Unlike the shared cases/issues list, problem rows DO
 * carry an accessible name (`aria-label="View problem {number}"`), so rows
 * here are addressed by role/name, not by `href`.
 */
export class ProblemsListPage {
  constructor(private readonly page: Page) {}

  async goto(): Promise<void> {
    await this.page.goto(PROBLEMS_TAB_PATH);
    await expect(
      this.page.getByRole("tab", { name: "Problem management", selected: true }),
    ).toBeVisible();
  }

  searchBox(): Locator {
    return this.page.getByPlaceholder("Search by number or subject…");
  }

  clearSearchButton(): Locator {
    return this.page.getByRole("button", { name: "Clear search" });
  }

  async search(query: string): Promise<void> {
    await this.searchBox().fill(query);
  }

  async clearSearch(): Promise<void> {
    await this.clearSearchButton().click();
  }

  filtersToggleButton(): Locator {
    return this.page.getByRole("button", { name: /^Filters/ });
  }

  async openFilters(): Promise<void> {
    const isOpen = await this.page
      .locator("#problem-filter-state")
      .isVisible()
      .catch(() => false);
    if (!isOpen) await this.filtersToggleButton().click();
  }

  /** Selects a State filter option (`#problem-filter-state`, a fixed-enum
   * MultiSelect — see `ProblemsFilterBar.tsx`). Options are scoped to the
   * just-opened MUI listbox (`getByRole("listbox")`), never queried
   * page-wide — see `CaseCreatePage.selectOption` for why an unscoped
   * `getByRole("option")` is unsafe on any page that also embeds the
   * rich-text description editor. */
  async selectStateFilter(optionLabel: string): Promise<void> {
    await this.openFilters();
    await this.page.locator("#problem-filter-state").click();
    await this.page
      .getByRole("listbox")
      .getByRole("option", { name: optionLabel, exact: true })
      .click();
    await this.page.keyboard.press("Escape");
  }

  async clearFilters(): Promise<void> {
    await this.page.getByRole("button", { name: "Clear filters" }).click();
  }

  createProblemButton(): Locator {
    return this.page.getByRole("button", { name: "Create problem" });
  }

  /** A problem row by its number (`aria-label="View problem {number}"`,
   * `ProblemsTab.tsx`). */
  row(problemNumber: string): Locator {
    return this.page.getByRole("row", { name: `View problem ${problemNumber}` });
  }

  async openProblem(problemNumber: string): Promise<void> {
    await this.row(problemNumber).click();
  }

  rows(): Locator {
    return this.page.getByRole("row", { name: /^View problem / });
  }

  async rowCount(): Promise<number> {
    return this.rows().count();
  }
}
