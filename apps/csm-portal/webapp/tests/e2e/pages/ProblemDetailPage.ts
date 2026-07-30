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

/**
 * Page object for `/operations/problems/:id` (`ProblemDetailPage.tsx`) —
 * read-only: there is no Edit dialog for problems (no mutation endpoint
 * yet), only linked-record chips and resolution text.
 */
export class ProblemDetailPage {
  constructor(private readonly page: Page) {}

  /**
   * A freshly-created problem isn't always retrievable the instant we
   * navigate to it — the real DEV-SN backend can lag between the create
   * write and the record becoming readable. Retry the navigation (full
   * reload) until the stable "Back to problems" button actually renders,
   * rather than failing on the first attempt. The heading is the problem's
   * own subject/number (dynamic), so we wait on that stable button instead.
   */
  async goto(id: string): Promise<void> {
    await expect(async () => {
      await this.page.goto(`/operations/problems/${id}`);
      await expect(this.backButton()).toBeVisible({ timeout: 3_000 });
    }).toPass({ timeout: 45_000, intervals: [1_000, 2_000, 3_000, 5_000] });
  }

  backButton(): Locator {
    return this.page.getByRole("button", { name: "Back to problems" });
  }

  async goBack(): Promise<void> {
    await this.backButton().click();
  }

  /** The linked-record chip for a field (Origin record / Primary incident /
   * Change request / an individual linked incident) — matched by its visible
   * label (the target's number or id, see `ProblemRefItem` in the source). */
  linkedRecordChip(label: string): Locator {
    return this.page.getByRole("button", { name: label, exact: true });
  }

  async openLinkedRecord(label: string): Promise<void> {
    await this.linkedRecordChip(label).click();
  }
}
