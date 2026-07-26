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
import { SECURITY_CENTER } from "../utils/selectors";

/**
 * Page object for `/security-center` (`CsmSecurityCenterPage.tsx`) — tabbed
 * landing for Security reports / Vulnerabilities.
 *
 * The Security reports tab is a `CsmIssuesView` with its DEFAULT
 * `detailBasePath` ("/cases", not "/security-center/...") — a report row
 * therefore opens `/cases/:id`, the ordinary case-detail page, not a
 * dedicated security-report view. Use `CasesListPage`/`CaseDetailPage`
 * against that tab if a spec needs full list/detail interaction; this POM
 * only covers the tab-level navigation and the Vulnerabilities tab.
 */
export class SecurityCenterPage {
  constructor(private readonly page: Page) {}

  async goto(): Promise<void> {
    await this.page.goto(SECURITY_CENTER.path);
    await expect(
      this.page.getByRole("heading", { name: SECURITY_CENTER.heading }),
    ).toBeVisible();
  }

  reportsTab(): Locator {
    return this.page.getByRole("tab", { name: SECURITY_CENTER.tabs.reports });
  }

  vulnerabilitiesTab(): Locator {
    return this.page.getByRole("tab", { name: SECURITY_CENTER.tabs.vulnerabilities });
  }

  async openReportsTab(): Promise<void> {
    await this.reportsTab().click();
  }

  async openVulnerabilitiesTab(): Promise<void> {
    await this.vulnerabilitiesTab().click();
  }

  newSecurityReportButton(): Locator {
    return this.page.getByRole("button", { name: "New security report" });
  }

  async openCreateReport(): Promise<void> {
    await this.newSecurityReportButton().click();
  }

  /** Vulnerabilities tab's search box (`aria-label="Search vulnerabilities"`,
   * `ProductVulnerabilitiesTab.tsx`). */
  vulnerabilitiesSearchBox(): Locator {
    return this.page.getByLabel("Search vulnerabilities");
  }

  /** Priority filter select (`aria-label="Filter by priority"`). */
  priorityFilter(): Locator {
    return this.page.getByLabel("Filter by priority");
  }

  /** Options are scoped to the just-opened MUI listbox
   * (`getByRole("listbox")`), never queried page-wide — see
   * `CaseCreatePage.selectOption` for why an unscoped `getByRole("option")`
   * is unsafe on any page that also embeds the rich-text description editor. */
  async selectPriorityFilter(optionLabel: string): Promise<void> {
    await this.priorityFilter().click();
    await this.page
      .getByRole("listbox")
      .getByRole("option", { name: optionLabel, exact: true })
      .click();
  }

  /** A vulnerability row, by its CVE/vulnerability id
   * (`role="button"`, `aria-label="View vulnerability {cveId}"` —
   * `ProductVulnerabilitiesTab.tsx`). */
  vulnerabilityRow(cveOrId: string): Locator {
    return this.page.getByRole("button", { name: `View vulnerability ${cveOrId}` });
  }

  async openVulnerability(cveOrId: string): Promise<void> {
    await this.vulnerabilityRow(cveOrId).click();
  }
}
