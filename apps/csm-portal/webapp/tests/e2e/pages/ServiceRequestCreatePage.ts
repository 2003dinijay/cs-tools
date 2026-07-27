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
import { E2E_PROJECT, SERVICE_REQUEST_CREATE } from "../utils/selectors";

/**
 * Page object for `/operations/service-requests/new`
 * (`CreateServiceRequestPage.tsx`). Backend-required, in cascade order:
 * Project → Deployment → Deployed product → Catalog → Catalog item, plus any
 * required dynamic catalog variables (`getFirstEmptyRequiredField`) — each
 * step's options only load once its parent is picked, so fields must be
 * filled in this order. There is no delete endpoint, so every SR created
 * here is permanent — tag its description/comments with
 * `e2eServiceRequestSubject`.
 */
export class ServiceRequestCreatePage {
  constructor(private readonly page: Page) {}

  async goto(): Promise<void> {
    await this.page.goto(SERVICE_REQUEST_CREATE.path);
    await expect(
      this.page.getByRole("heading", { name: SERVICE_REQUEST_CREATE.heading }),
    ).toBeVisible();
  }

  /** Types into the (async, first-page-preloaded) Project picker and picks
   * the first result — mirrors `IncidentCreatePage.pickService`.
   *
   * Options are scoped to `getByRole("listbox")` — the just-opened MUI
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

  /** Opens a MUI Select by its field label and clicks the named option —
   * Deployment, Deployed product, Catalog, Catalog item all use this. Scoped
   * to the open listbox for the same reason as `pickProject` — see its doc
   * comment. */
  async selectOption(label: string, optionLabel: string): Promise<void> {
    const escaped = label.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
    await this.page.getByRole("combobox", { name: new RegExp(`^${escaped}\\s*\\*?$`) }).click();
    await this.page
      .getByRole("listbox")
      .getByRole("option", { name: optionLabel, exact: true })
      .click();
  }

  /** A dynamic catalog-variable field, by the label ServiceNow returned for
   * it (`CatalogVariableFields.tsx` renders one input per variable, each
   * labelled with its own `question`/name text — there's no fixed set of
   * labels, so callers must know their catalog item's actual variable
   * labels). Control type varies per variable (choice → Select, multi-line
   * → plain TextField, date/time → picker), so this stays on `getByLabel`
   * rather than pinning a single ARIA role — but still marker-tolerant
   * (required variables render the same thin-space `*` suffix as any other
   * required MUI field). */
  variableField(label: string): Locator {
    const escaped = label.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
    return this.page.getByLabel(new RegExp(`^${escaped}\\s*\\*?$`));
  }

  createButton(): Locator {
    return this.page.getByRole("button", { name: "Create service request" });
  }

  /**
   * Fills the cascade (Project → Deployment → Deployed product → Catalog →
   * Catalog item) and submits. `variables` fills any required dynamic
   * catalog-variable fields by their label (pass `{}` for a catalog item with
   * no required variables). Returns once the app has navigated to the new
   * SR's detail page (it's a case under the hood, so the URL is
   * `/cases/:id`, not `/operations/service-requests/:id` — see
   * `CreateServiceRequestPage.tsx`'s `navigate` call).
   */
  async fillRequiredFieldsAndSubmit(opts: {
    projectQuery?: string;
    deployment: string;
    deployedProduct: string;
    catalog: string;
    catalogItem: string;
    variables?: Record<string, string>;
  }): Promise<void> {
    await this.pickProject(opts.projectQuery ?? E2E_PROJECT);
    await this.selectOption("Deployment", opts.deployment);
    await this.selectOption("Deployed product", opts.deployedProduct);
    await this.selectOption("Catalog", opts.catalog);
    await this.selectOption("Catalog item", opts.catalogItem);
    for (const [label, value] of Object.entries(opts.variables ?? {})) {
      await this.variableField(label).fill(value);
    }

    await expect(this.createButton()).toBeEnabled();
    await this.createButton().click();
    await expect(this.page).toHaveURL(/\/cases\/[^/]+$/, { timeout: 15_000 });
  }
}
