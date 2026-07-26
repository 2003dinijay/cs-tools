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
import { E2E_PROJECT, SECURITY_REPORT_CREATE } from "../utils/selectors";

/**
 * Page object for `/security-center/reports/new`
 * (`CreateSecurityReportPage.tsx`). Required: Project, Deployment, Deployed
 * product, Subject (auto-generated once Deployed product is picked, but
 * still editable/overridable — always overwrite it with
 * `e2eSecurityReportSubject` so the created record is taggable), a non-empty
 * Description, and AT LEAST ONE attachment (`canSubmit` requires
 * `attachments.length > 0` — unlike the case/SR create forms, where
 * attachments are optional). Submitting creates a case
 * (`type: "security_report_analysis"`), so it navigates to `/cases/:id`, not
 * a `/security-center/...` path.
 */
export class CreateSecurityReportPage {
  constructor(private readonly page: Page) {}

  async goto(): Promise<void> {
    await this.page.goto(SECURITY_REPORT_CREATE.path);
    await expect(
      this.page.getByRole("heading", { name: SECURITY_REPORT_CREATE.heading }),
    ).toBeVisible();
  }

  /** Options are scoped to `getByRole("listbox")` — the just-opened MUI
   * Autocomplete popper — never queried page-wide. This page also embeds the
   * rich-text description editor's "Font variant" `<select>`, whose native
   * `<option>` elements are always present in the DOM (not just while its
   * dropdown is open) and would match a page-wide `getByRole("option")`; a
   * native `<select>` has no `role="listbox"` ancestor, so scoping through
   * the open listbox excludes them entirely.
   *
   * Waits a bounded 8s for the first option to appear, then throws a typed
   * `Error("PROJECT_PICKER_EMPTY")` rather than letting a generic Playwright
   * timeout bubble up — `POST /projects/search` is intermittently 503 in
   * DEV/staging, and 8s is enough to distinguish "backend didn't answer" from
   * "answered with zero projects" without needlessly stretching every
   * failing test; callers should catch this and self-skip. */
  async pickProject(query = E2E_PROJECT): Promise<void> {
    const input = this.page.getByRole("combobox", { name: /^Project\s*\*?$/ });
    await input.click();
    if (query) await input.fill(query);
    const option = this.page.getByRole("listbox").getByRole("option").first();
    const appeared = await option
      .waitFor({ state: "visible", timeout: 8_000 })
      .then(() => true)
      .catch(() => false);
    if (!appeared) {
      throw new Error("PROJECT_PICKER_EMPTY");
    }
    await option.click();
  }

  /** Opens a MUI Select and clicks the named option, scoped to the open
   * listbox for the same reason as `pickProject` — see its doc comment. */
  async selectOption(label: string, optionLabel: string): Promise<void> {
    const escaped = label.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
    await this.page.getByRole("combobox", { name: new RegExp(`^${escaped}\\s*\\*?$`) }).click();
    await this.page
      .getByRole("listbox")
      .getByRole("option", { name: optionLabel, exact: true })
      .click();
  }

  subjectField(): Locator {
    return this.page.getByRole("textbox", { name: /^Subject\s*\*?$/ });
  }

  descriptionEditor(): Locator {
    return this.page.getByTestId("case-description-editor");
  }

  async fillDescription(text: string): Promise<void> {
    await this.descriptionEditor().click();
    await this.descriptionEditor().fill(text);
  }

  /** "Add attachments" button's hidden `<input type="file" multiple>`
   * (`AttachmentsField.tsx`) — at least one file is required to submit.
   *
   * This page also embeds the rich-text description editor's own hidden
   * `<input type="file" accept="image/*">` (single-file, for inline image
   * embeds) earlier in the DOM — a page-wide `input[type="file"]` query's
   * `.first()` resolves to THAT input, not the attachments field, silently
   * uploading nothing to `AttachmentsField` and leaving `canSubmit` false
   * forever. `[multiple]` is the stable discriminator (source-backed in
   * `AttachmentsField.tsx`; the editor's image input never sets it). */
  async addAttachments(filePaths: string | string[]): Promise<void> {
    await this.page.locator('input[type="file"][multiple]').first().setInputFiles(filePaths);
  }

  createButton(): Locator {
    return this.page.getByRole("button", { name: "Create security report" });
  }

  cancelButton(): Locator {
    return this.page.getByRole("button", { name: "Cancel" });
  }

  async cancel(): Promise<void> {
    await this.cancelButton().click();
  }

  /** Fills every required field (including the mandatory attachment) and
   * submits. Returns once the app has navigated to the new case's detail
   * page (`/cases/:id`). */
  async fillRequiredFieldsAndSubmit(opts: {
    projectQuery?: string;
    deployment: string;
    deployedProduct: string;
    subject: string;
    description: string;
    attachmentPath: string;
  }): Promise<void> {
    await this.pickProject(opts.projectQuery ?? E2E_PROJECT);
    await this.selectOption("Deployment", opts.deployment);
    await this.selectOption("Deployed product", opts.deployedProduct);
    await this.subjectField().fill(opts.subject);
    await this.fillDescription(opts.description);
    await this.addAttachments(opts.attachmentPath);

    await expect(this.createButton()).toBeEnabled();
    await this.createButton().click();
    await expect(this.page).toHaveURL(/\/cases\/[^/]+$/, { timeout: 15_000 });
  }
}
