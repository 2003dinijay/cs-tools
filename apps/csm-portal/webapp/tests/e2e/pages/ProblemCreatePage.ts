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
import { PROBLEM_CREATE } from "../utils/selectors";

/**
 * Page object for `/operations/problems/new` (`CreateProblemPage.tsx`).
 * Subject is the only required field — Category/Subcategory/Origin
 * case/Primary incident are all optional "advanced linking" fields, and
 * there is no Priority field on create (SN computes it server-side). No
 * delete endpoint, so every problem created here is permanent — tag the
 * subject with `e2eProblemSubject`.
 */
export class ProblemCreatePage {
  constructor(private readonly page: Page) {}

  async goto(): Promise<void> {
    await this.page.goto(PROBLEM_CREATE.path);
    await expect(
      this.page.getByRole("heading", { name: PROBLEM_CREATE.heading }),
    ).toBeVisible();
  }

  subjectField(): Locator {
    return this.page.getByRole("textbox", { name: /^Subject\s*\*?$/ });
  }

  /** Category/Subcategory are optional fixed-enum Selects (Subcategory's
   * options depend on the chosen Category — see
   * `PROBLEM_SUBCATEGORY_OPTIONS_BY_CATEGORY`). Scoped to the just-opened MUI
   * listbox (`getByRole("listbox")`), never queried page-wide — see
   * `CaseCreatePage.selectOption` for why an unscoped `getByRole("option")`
   * is unsafe on any page that also embeds the rich-text description editor. */
  async selectOption(label: string, optionLabel: string): Promise<void> {
    const escaped = label.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
    await this.page.getByRole("combobox", { name: new RegExp(`^${escaped}\\s*\\*?$`) }).click();
    await this.page
      .getByRole("listbox")
      .getByRole("option", { name: optionLabel, exact: true })
      .click();
  }

  /** Types into the "Origin case" async search-and-pick field and picks the
   * first result, scoped to the open listbox — see `selectOption` above. */
  async pickOriginCase(query: string): Promise<void> {
    const input = this.page.getByRole("combobox", { name: /^Origin case\s*\*?$/ });
    await input.click();
    await input.fill(query);
    const option = this.page.getByRole("listbox").getByRole("option").first();
    await option.waitFor({ state: "visible", timeout: 10_000 });
    await option.click();
  }

  /** Types into the "Primary incident" async search-and-pick field and picks
   * the first result, scoped to the open listbox — see `selectOption` above. */
  async pickPrimaryIncident(query: string): Promise<void> {
    const input = this.page.getByRole("combobox", { name: /^Primary incident\s*\*?$/ });
    await input.click();
    await input.fill(query);
    const option = this.page.getByRole("listbox").getByRole("option").first();
    await option.waitFor({ state: "visible", timeout: 10_000 });
    await option.click();
  }

  createButton(): Locator {
    return this.page.getByRole("button", { name: "Create problem" });
  }

  /** Fills the only required field (Subject) and submits. Returns once the
   * app has navigated to the new problem's detail page
   * (`/operations/problems/:id`). The id segment must not match the literal
   * "new" of this very create route — a bare `[^/]+$` is satisfied by
   * `/operations/problems/new` itself, which would let this assertion pass
   * instantly on a still-pending (or failed) submit, before the app ever
   * navigates to the created record's real id. */
  async fillSubjectAndSubmit(subject: string): Promise<void> {
    await this.subjectField().fill(subject);
    await expect(this.createButton()).toBeEnabled();
    await this.createButton().click();
    await expect(this.page).toHaveURL(/\/operations\/problems\/(?!new(?:[/?#]|$))[^/]+$/, {
      timeout: 15_000,
    });
  }
}
