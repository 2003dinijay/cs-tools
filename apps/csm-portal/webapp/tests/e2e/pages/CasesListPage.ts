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
import { CASES } from "../utils/selectors";

/**
 * Page object for the shared "issues list" (`CsmIssuesView.tsx`), which backs
 * three different routes with the same markup:
 *  - `/cases` — all support cases (`detailBasePath` defaults to `/cases`)
 *  - `/engagements` — engagements (`detailBasePath` = `/engagements`)
 *  - `/operations` (Service requests tab) — `detailBasePath` =
 *    `/operations/service-requests`, but the *route itself* is `/operations`
 *    with `?tab=service_requests`; pass `basePath: "/operations?tab=service_requests"`
 *    for `goto()` and `detailBasePath: "/operations/service-requests"`
 *    separately for row links, since they differ here (see constructor).
 *
 * Row links carry NO accessible name/aria-label in this component (unlike
 * `ChildCasesWidget`'s related-cases rows or `ProblemsTab`'s "View problem …"
 * rows) — each row is a bare `<a>` (`RouterLink`) whose only identifying
 * attribute is its `href`. `row()`/`openCase()` below match on that `href`
 * rather than an accessible name.
 */
export class CasesListPage {
  private readonly detailBasePath: string;

  constructor(
    private readonly page: Page,
    private readonly basePath: string = CASES.path,
    detailBasePath?: string,
  ) {
    this.detailBasePath = detailBasePath ?? this.basePath;
  }

  /** Navigates to the list and waits for the search box to render — a stable
   * signal across all three routes this POM covers (unlike the page heading,
   * which is absent for the Operations-tab Service Requests list). */
  async goto(): Promise<void> {
    await this.page.goto(this.basePath);
    await expect(this.searchBox()).toBeVisible();
  }

  /** All three routes this POM covers share `CasesFilterBar.tsx`'s single
   * search placeholder ("Search by case #, subject, customer, project,
   * assignee…") — unlike the separate Problems/Announcements list pages,
   * which have their own, different placeholders. */
  searchBox(): Locator {
    return this.page.getByPlaceholder(/Search by case #/);
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

  /** The combined "Filters"/"Clear filters (N)" toggle button — its accessible
   * name changes once a filter is active (see `CasesFilterBar.tsx`), so this
   * always targets it by role alone, not by name. */
  filtersToggleButton(): Locator {
    return this.page.getByRole("button", { name: /^(Filters|Clear filters)/ });
  }

  async openFilters(): Promise<void> {
    // Only the plain "Filters" button opens the panel — once any filter is
    // active the same-looking button becomes "Clear filters" and resets
    // instead, so guard on the filter grid already being visible.
    const alreadyOpen = await this.page
      .locator("#cases-filter-state")
      .isVisible()
      .catch(() => false);
    if (!alreadyOpen) await this.filtersToggleButton().click();
  }

  /**
   * Selects an option in one of the fixed-enum MultiSelect filters
   * (`cases-filter-severity`, `-state`, `-work-state`, `-engagement-type`,
   * `-type`) by its DOM `id` (see `MultiSelectField.tsx` — MUI's non-native
   * Select renders the `id` prop directly on the `[role=combobox]` div, so
   * `#id` addresses it precisely regardless of its current label text).
   * Leaves the dropdown open (MUI `multiple` Selects don't auto-close on
   * pick) — press Escape afterwards if a subsequent action needs it closed.
   * Options are scoped to the just-opened MUI listbox (`getByRole("listbox")`),
   * never queried page-wide, so a stray `role=option` elsewhere on the page
   * can never be picked instead — see `CaseCreatePage.selectOption` for the
   * concrete case (rich-text editor "Font variant" `<select>`) this guards
   * against.
   */
  async selectFilterOption(filterId: string, optionName: string): Promise<void> {
    await this.openFilters();
    await this.page.locator(`#${filterId}`).click();
    await this.page
      .getByRole("listbox")
      .getByRole("option", { name: optionName, exact: true })
      .click();
    await this.page.keyboard.press("Escape");
  }

  /** Clicks the "Updated" column header to flip sort direction
   * (`TableSortLabel` in `CasesList.tsx` — always server-sorted by
   * `updatedOn`, this only toggles asc/desc). */
  async toggleSort(): Promise<void> {
    await this.page.getByText("Updated", { exact: true }).click();
  }

  /** Rows-per-page control. `optionLabel` must match one of "10", "20", or
   * the backend max page size (see `BE_MAX_PAGE_LIMIT`/`ROWS_PER_PAGE_OPTIONS`
   * in `CsmIssuesView.tsx`) — the label text itself is whatever
   * `labelRowsPerPage` renders (e.g. "Cases per page"), so this targets the
   * rows-per-page `combobox` by role, not by its dynamic label. */
  rowsPerPageSelect(): Locator {
    return this.page.locator(".MuiTablePagination-select");
  }

  async setRowsPerPage(optionLabel: string): Promise<void> {
    await this.rowsPerPageSelect().click();
    await this.page
      .getByRole("listbox")
      .getByRole("option", { name: optionLabel, exact: true })
      .click();
  }

  /** MUI's built-in pagination-actions labels; matched loosely (case aside)
   * since the exact wording ("Go to next page" vs "Next page") can vary by
   * MUI/oxygen-ui version and isn't pinned down in this codebase. */
  nextPageButton(): Locator {
    return this.page.getByRole("button", { name: /next page/i });
  }

  previousPageButton(): Locator {
    return this.page.getByRole("button", { name: /previous page/i });
  }

  async goToNextPage(): Promise<void> {
    await this.nextPageButton().click();
  }

  async goToPreviousPage(): Promise<void> {
    await this.previousPageButton().click();
  }

  // ── Saved views ──────────────────────────────────────────────────────────

  savedViewsButton(): Locator {
    return this.page.getByRole("button", { name: "Saved views" });
  }

  async openSavedViewsMenu(): Promise<void> {
    await this.savedViewsButton().click();
  }

  /** Opens the "Save current view…" dialog from the Saved views menu. */
  async openSaveViewDialog(): Promise<void> {
    await this.openSavedViewsMenu();
    await this.page.getByRole("menuitem", { name: "Save current view…" }).click();
  }

  saveViewNameField(): Locator {
    return this.page.getByRole("textbox", { name: /^View name\s*\*?$/ });
  }

  /** Fills the save-view dialog's name field and confirms via its "Save"
   * button (the dialog's own DialogActions button, scoped so it isn't
   * confused with any other "Save" button on the page). */
  async saveCurrentView(name: string): Promise<void> {
    await this.openSaveViewDialog();
    await this.saveViewNameField().fill(name);
    await this.page.getByRole("dialog").getByRole("button", { name: "Save" }).click();
  }

  /** Applies a saved (or suggested) view by its exact name from the Saved
   * views menu. */
  async applySavedView(name: string): Promise<void> {
    await this.openSavedViewsMenu();
    await this.page.getByRole("menuitem", { name, exact: false }).click();
  }

  /** Deletes a saved view via its row's trash icon
   * (`aria-label="Delete saved view {name}"`, `CasesFilterBar.tsx`). Opens
   * the Saved views menu first if it isn't already open. */
  async deleteSavedView(name: string): Promise<void> {
    const button = this.page.getByRole("button", { name: `Delete saved view ${name}` });
    if (!(await button.isVisible().catch(() => false))) {
      await this.openSavedViewsMenu();
    }
    await button.click();
  }

  // ── Rows ─────────────────────────────────────────────────────────────────

  /** All data rows (each row is an `<a>` whose `href` starts with this list's
   * `detailBasePath` — see the class doc for why there's no accessible-name
   * based selector here). */
  rows(): Locator {
    return this.page.locator(`a[href^="${this.detailBasePath}/"]`);
  }

  async rowCount(): Promise<number> {
    return this.rows().count();
  }

  firstRow(): Locator {
    return this.rows().first();
  }

  /** The row for a specific case, matched by the exact id/number segment used
   * in its detail link (`${detailBasePath}/${caseId}` — see `CasesList.tsx`'s
   * `to={...}`). */
  row(caseId: string): Locator {
    return this.page.locator(`a[href="${this.detailBasePath}/${caseId}"]`);
  }

  async openCase(caseId: string): Promise<void> {
    await this.row(caseId).click();
  }
}
