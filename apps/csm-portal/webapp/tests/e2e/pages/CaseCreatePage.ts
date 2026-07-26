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
import { CASES, E2E_PROJECT } from "../utils/selectors";

/**
 * Page object for `/cases/new` (`CsmCaseCreatePage.tsx`). Backend-required
 * fields (see `canSubmit`): Project, Deployment (hidden/auto-derived for
 * Managed Cloud/cloud-support projects — see `isCloudProject`), Deployed
 * product, Severity, Issue type, Subject, and a non-empty Description.
 * There is no delete endpoint for cases, so every case this creates is a
 * permanent record — always tag the subject with `e2eCaseSubject`.
 */
export class CaseCreatePage {
  constructor(private readonly page: Page) {}

  async goto(): Promise<void> {
    await this.page.goto(CASES.create.path);
    await expect(
      this.page.getByRole("heading", { name: CASES.create.heading }),
    ).toBeVisible();
  }

  /** Types into the (async, first-page-preloaded) Project picker and picks
   * the first result. No typing is required for a first pick — opening the
   * field alone loads a page of projects (see `AsyncProjectSelect.tsx`) — but
   * a query narrows a large tenant down for a stable "first option".
   *
   * Options are scoped to `getByRole("listbox")` — the just-opened MUI
   * Autocomplete popper — never queried page-wide. The page also embeds the
   * rich-text description editor's "Font variant" `<select>`, whose native
   * `<option>` elements ("Heading 1"..."Quote") are always present in the DOM
   * (not just while its dropdown is open) and match a page-wide
   * `getByRole("option")`; a native `<select>` has no `role="listbox"`
   * ancestor, so scoping through the open listbox excludes them entirely.
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

  /** Opens a MUI Select by its field label and clicks the named option —
   * Deployment, Deployed product, Severity, Issue type all use this pattern
   * (fixed enum or id-keyed Select, not an async Autocomplete). Scoped to the
   * open listbox for the same reason as `pickProject` — see its doc comment. */
  async selectOption(label: string, optionLabel: string): Promise<void> {
    // Field accessible names carry the required-field marker (e.g. "Deployment *"),
    // so a marker-tolerant role query is used instead of getByLabel(exact).
    const escaped = label.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
    await this.page
      .getByRole("combobox", { name: new RegExp(`^${escaped}\\s*\\*?$`) })
      .click();
    await this.page
      .getByRole("listbox")
      .getByRole("option", { name: optionLabel, exact: true })
      .click();
  }

  subjectField(): Locator {
    return this.page.getByRole("textbox", { name: /^Subject\s*\*?$/ });
  }

  /** The rich-text description editor's content-editable region (this
   * component's fixed `data-testid`, wired in `Editor.tsx` — not an
   * FE-test-only id invented for this suite). */
  descriptionEditor(): Locator {
    return this.page.getByTestId("case-description-editor");
  }

  async fillDescription(text: string): Promise<void> {
    await this.descriptionEditor().click();
    await this.descriptionEditor().fill(text);
  }

  createButton(): Locator {
    return this.page.getByRole("button", { name: "Create case" });
  }

  /**
   * Fills every backend-required field and submits. `projectQuery` narrows
   * the async Project picker to a stable first result (pass "" to accept
   * whatever the tenant's first page returns). Deployment is only filled
   * when the picked project isn't a cloud-support project (the field is
   * hidden entirely for those — see `isCloudProject` in the source page);
   * pass `deployment: undefined` for a cloud-support project.
   */
  async fillRequiredFieldsAndSubmit(opts: {
    projectQuery?: string;
    deployment?: string;
    deployedProduct: string;
    severity: string;
    issueType: string;
    subject: string;
    description: string;
  }): Promise<void> {
    await this.pickProject(opts.projectQuery ?? E2E_PROJECT);
    if (opts.deployment) {
      await this.selectOption("Deployment", opts.deployment);
    }
    await this.selectOption("Deployed product", opts.deployedProduct);
    await this.selectOption("Severity", opts.severity);
    await this.selectOption("Issue type", opts.issueType);
    await this.subjectField().fill(opts.subject);
    await this.fillDescription(opts.description);

    await expect(this.createButton()).toBeEnabled();
    await this.createButton().click();
    await expect(this.page).toHaveURL(/\/cases\/[^/]+$/, { timeout: 15_000 });
  }
}
