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
import { ANNOUNCEMENTS } from "../utils/selectors";

/**
 * Page object for `/announcements` (`CsmAnnouncementsPage.tsx`) — a
 * read-only list (announcements are cases of `type: "announcement"`
 * surfaced via `POST /cases/search`; there is no create/edit flow yet).
 * Like `CasesList`, each row is a bare `<a>` (`RouterLink`) with no
 * accessible name of its own — rows are matched by `href`, not by role/name.
 */
export class AnnouncementsPage {
  constructor(private readonly page: Page) {}

  async goto(): Promise<void> {
    await this.page.goto(ANNOUNCEMENTS.path);
    await expect(
      this.page.getByRole("heading", { name: ANNOUNCEMENTS.heading }),
    ).toBeVisible();
  }

  /** `aria-label="Search announcements"` — wired via `slotProps.htmlInput`
   * (`CsmAnnouncementsPage.tsx`), unlike the cases/problems search boxes
   * which have no aria-label and must be matched by placeholder instead. */
  searchBox(): Locator {
    return this.page.getByLabel("Search announcements");
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

  /** State filter (`#announcements-filter-state`) — a fixed-enum
   * MultiSelect, same pattern as the cases list. Options are scoped to the
   * just-opened MUI listbox (`getByRole("listbox")`), never queried
   * page-wide — see `CaseCreatePage.selectOption` for why an unscoped
   * `getByRole("option")` is unsafe on any page that also embeds the
   * rich-text description editor. */
  async selectStateFilter(optionLabel: string): Promise<void> {
    await this.page.locator("#announcements-filter-state").click();
    await this.page
      .getByRole("listbox")
      .getByRole("option", { name: optionLabel, exact: true })
      .click();
    await this.page.keyboard.press("Escape");
  }

  /** Project filter (`AsyncProjectMultiSelect`, id
   * `announcements-filter-project`) — types `query` and picks the matching
   * option, scoped to the open listbox — see `selectStateFilter` above. */
  async selectProjectFilter(query: string, optionLabel: string): Promise<void> {
    const input = this.page.locator("#announcements-filter-project");
    await input.click();
    await input.fill(query);
    await this.page
      .getByRole("listbox")
      .getByRole("option", { name: optionLabel, exact: true })
      .click();
  }

  /** Only rendered once a filter is active (see `activeFilterCount` in the
   * source — there is no persistent "Clear filters" button otherwise). */
  clearFiltersButton(): Locator {
    return this.page.getByRole("button", { name: "Clear filters" });
  }

  async clearFilters(): Promise<void> {
    await this.clearFiltersButton().click();
  }

  /** A row by the announcement's `id` (matches its detail link's `href`
   * exactly — rows carry no accessible name of their own). */
  row(id: string): Locator {
    return this.page.locator(`a[href="/announcements/${id}"]`);
  }

  async openAnnouncement(id: string): Promise<void> {
    await this.row(id).click();
  }

  rows(): Locator {
    return this.page.locator('a[href^="/announcements/"]');
  }

  async rowCount(): Promise<number> {
    return this.rows().count();
  }
}
