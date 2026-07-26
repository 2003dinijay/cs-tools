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
import { UPDATES } from "../utils/selectors";

/**
 * Page object for `/updates` (`CsmUpdatesPage.tsx`). The four filter Selects
 * (Product → Version → Start level → End level) cascade: each is disabled
 * until its parent is chosen, and choosing a parent clears everything
 * downstream — fill them in this order.
 */
export class UpdatesPage {
  constructor(private readonly page: Page) {}

  async goto(): Promise<void> {
    await this.page.goto(UPDATES.path);
    // `name` matching is substring-by-default, and "Updates" is also a
    // substring of the "Search updates between levels" subheading below it —
    // match the page title exactly to avoid a strict-mode ambiguity.
    await expect(
      this.page.getByRole("heading", { name: UPDATES.heading, exact: true }),
    ).toBeVisible();
  }

  /** Opens one of the four cascading Selects (Product, Version, Start level,
   * End level) by its field label and picks the named option. Scoped to the
   * just-opened MUI listbox (`getByRole("listbox")`), never queried
   * page-wide — see `CaseCreatePage.selectOption` for why an unscoped
   * `getByRole("option")` is unsafe on any page that also embeds the
   * rich-text description editor. */
  async selectOption(label: string, optionLabel: string): Promise<void> {
    const escaped = label.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
    await this.page.getByRole("combobox", { name: new RegExp(`^${escaped}\\s*\\*?$`) }).click();
    await this.page
      .getByRole("listbox")
      .getByRole("option", { name: optionLabel, exact: true })
      .click();
  }

  searchButton(): Locator {
    return this.page.getByRole("button", { name: "Search", exact: true });
  }

  clearButton(): Locator {
    return this.page.getByRole("button", { name: "Clear", exact: true });
  }

  previewReportButton(): Locator {
    return this.page.getByRole("button", { name: "Preview report" });
  }

  downloadPdfButton(): Locator {
    return this.page.getByRole("button", { name: "Download PDF" });
  }

  /** Fills the four cascading filters and runs the search. */
  async search(opts: {
    product: string;
    version: string;
    startLevel: string;
    endLevel: string;
  }): Promise<void> {
    await this.selectOption("Product", opts.product);
    await this.selectOption("Version", opts.version);
    await this.selectOption("Start level", opts.startLevel);
    await this.selectOption("End level", opts.endLevel);
    await this.searchButton().click();
  }

  /** A result row's "View" button, by its update-level key (opens
   * `UpdateDetailsDialog`, titled "Update Level {levelKey}"). */
  viewLevelButton(levelKey: string): Locator {
    return this.page
      .getByRole("row", { name: new RegExp(`^${levelKey}\\b`) })
      .getByRole("button", { name: "View" });
  }

  async openLevelDetails(levelKey: string): Promise<void> {
    await this.viewLevelButton(levelKey).click();
  }

  levelDetailsDialogHeading(levelKey: string): Locator {
    return this.page.getByRole("heading", { name: `Update Level ${levelKey}` });
  }

  /** Closes whichever dialog is currently open (level-details or report
   * preview — both use the same `aria-label="Close"` icon button). */
  async closeDialog(): Promise<void> {
    await this.page.getByRole("button", { name: "Close" }).click();
  }

  async openReportPreview(): Promise<void> {
    await this.previewReportButton().click();
  }

  reportPreviewHeading(): Locator {
    return this.page.getByRole("heading", { name: "Update Levels Report" });
  }

  async downloadPdf(): Promise<void> {
    await this.downloadPdfButton().click();
  }
}
