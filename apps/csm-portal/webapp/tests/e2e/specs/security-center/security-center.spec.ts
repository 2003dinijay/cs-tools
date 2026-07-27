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

//
// Security Center (`/security-center`) — tabbed landing for Security reports
// (an ordinary `CsmIssuesView` that opens `/cases/:id`, not a dedicated
// security-report detail view) and Vulnerabilities (read-only). Security
// report creation (`/security-center/reports/new`) is wired to the real
// csm-portal-backend — `POST /cases` (`type: "security_report_analysis"`)
// has no delete endpoint, so anything a spec creates becomes a permanent
// case. Same rule as incidents/change requests/problems: the happy-path
// test is deliberately the only one that actually submits, tagged via
// e2eSecurityReportSubject, and uses the lowest-impact free-text values.
//

import fs from "node:fs";
import os from "node:os";
import path from "node:path";

import { test, expect, withRole } from "../../fixtures/test";
import { SecurityCenterPage } from "../../pages/SecurityCenterPage";
import { CreateSecurityReportPage } from "../../pages/CreateSecurityReportPage";
import { e2eSecurityReportSubject, SECURITY_CENTER, SECURITY_REPORT_CREATE } from "../../utils/selectors";

withRole(test, "approver");

/** A tiny throwaway text file for the mandatory attachment field — created
 * fresh per test run under the OS temp dir, not checked into the repo. */
function makeAttachmentFile(): string {
  const file = path.join(os.tmpdir(), `e2e-security-report-${Date.now()}.txt`);
  fs.writeFileSync(file, "e2e security report attachment — safe to delete.\n");
  return file;
}

test.describe("security center — page + tabs render", () => {
  test("shows the Vulnerabilities and Reports tabs and the create button", async ({ page }) => {
    const securityCenter = new SecurityCenterPage(page);
    await securityCenter.goto();

    await expect(page.getByRole("tab", { name: SECURITY_CENTER.tabs.reports })).toBeVisible();
    await expect(page.getByRole("tab", { name: SECURITY_CENTER.tabs.vulnerabilities })).toBeVisible();
    await expect(
      page.getByRole("button", { name: "New security report", exact: true }),
    ).toBeVisible();
  });
});

test.describe("security center — vulnerabilities list + detail (read-only)", () => {
  test("opens the first vulnerability's read-only detail from the list", async ({ page }) => {
    const securityCenter = new SecurityCenterPage(page);
    await securityCenter.goto();
    await securityCenter.openVulnerabilitiesTab();

    const firstRow = page.getByRole("button", { name: /^View vulnerability / }).first();
    const rowCount = await page.getByRole("button", { name: /^View vulnerability / }).count();
    test.skip(rowCount === 0, "No vulnerabilities on staging to open.");

    await firstRow.click();

    // Detail is read-only: a back button plus no editable form controls.
    await expect(
      page.getByRole("button", { name: /back/i }).first(),
    ).toBeVisible({ timeout: 10_000 });
    await expect(page.locator('input:not([readonly]):not([type="search"])')).toHaveCount(0);
  });
});

test.describe("security report creation — page structure", () => {
  test("Create security report stays disabled until required fields are filled", async ({ page }) => {
    const createReport = new CreateSecurityReportPage(page);
    await createReport.goto();
    await expect(page).toHaveURL(new RegExp(SECURITY_REPORT_CREATE.path.replace("/", "\\/")));

    await expect(createReport.createButton()).toBeDisabled();

    await createReport.subjectField().fill(e2eSecurityReportSubject("validation check"));
    await expect(createReport.createButton()).toBeDisabled();

    await createReport.fillDescription("Validation-only description, not submitted.");
    await expect(createReport.createButton()).toBeDisabled();
  });
});

test.describe("security report creation — happy path", () => {
  test("creates a real case and lands on its detail page", async ({ page }) => {
    // Real network round trips (project/deployment/product search, then
    // create), plus a navigation and a second fetch to load the detail
    // page — comfortably exceeds the 30s default.
    test.setTimeout(60_000);

    const createReport = new CreateSecurityReportPage(page);
    await createReport.goto();

    // Deployment/Deployed product are cascading async pickers with no
    // fixed, known-in-advance staging data — `fillRequiredFieldsAndSubmit`
    // needs their exact option labels, so discover the first available one
    // in each before calling it. Project must be picked first: the
    // Deployment select stays disabled until a project is chosen.
    try {
      await createReport.pickProject();
    } catch {
      test.skip(true, "No projects available in staging to create a security report against.");
      return;
    }

    await page.getByRole("combobox", { name: /^Deployment\s*\*?$/ }).click();
    const deploymentOption = page.getByRole("listbox").getByRole("option").first();
    const hasDeployment = await deploymentOption
      .waitFor({ state: "visible", timeout: 10_000 })
      .then(() => true)
      .catch(() => false);
    test.skip(!hasDeployment, "No deployments on staging for this project.");
    const deploymentLabel = (await deploymentOption.textContent())?.trim();
    // Close the option list without selecting — `selectOption` below
    // reopens it and selects by the discovered label; Deployed product
    // needs Deployment actually committed first (it's disabled until then).
    await page.keyboard.press("Escape");
    test.skip(!deploymentLabel, "Could not read the deployment option's label.");
    await createReport.selectOption("Deployment", deploymentLabel!);

    await page.getByRole("combobox", { name: /^Deployed product\s*\*?$/ }).click();
    const productOption = page.getByRole("listbox").getByRole("option").first();
    const hasProduct = await productOption
      .waitFor({ state: "visible", timeout: 10_000 })
      .then(() => true)
      .catch(() => false);
    test.skip(!hasProduct, "No deployed products on staging for this deployment.");
    const productLabel = (await productOption.textContent())?.trim();
    await page.keyboard.press("Escape");
    test.skip(!productLabel, "Could not read the deployed product option's label.");

    const subject = e2eSecurityReportSubject("e2e security report creation");
    await createReport.selectOption("Deployed product", productLabel!);
    await createReport.subjectField().fill(subject);
    await createReport.fillDescription("Created by an automated e2e test — safe to ignore.");
    await createReport.addAttachments(makeAttachmentFile());

    await expect(createReport.createButton()).toBeEnabled();
    await createReport.createButton().click();
    await expect(page).toHaveURL(/\/cases\/[^/]+$/, { timeout: 15_000 });

    // CsmCaseDetailPage titles itself with the case's own subject once
    // loaded — the strongest available confirmation that the record we
    // just created (not some other one) is what's showing.
    await expect(
      page.getByRole("heading", { level: 5, name: subject }),
    ).toBeVisible({ timeout: 15_000 });
  });
});
