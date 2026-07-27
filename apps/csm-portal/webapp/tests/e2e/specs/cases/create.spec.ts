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
// Case creation (POST /cases). Every real submit here creates a permanent
// record with no delete endpoint, so the happy-path test is deliberately the
// only one that actually submits, tagged via e2eCaseSubject.
//
// Deployment and Deployed product are dynamic, async, tenant-dependent
// fields (CsmCaseCreatePage.tsx: Deployment is hidden entirely for
// cloud-support projects, and Deployed product options depend on whichever
// deployment ends up selected) — unlike Severity/Issue type, which are fixed
// enums. Both tests below discover the first available option for each by
// opening the Select and reading its first `option` rather than hard-coding
// a label, and self-skip when staging has no seeded deployment/product for
// the first project the async picker returns.
//

import { test, expect, withRole } from "../../fixtures/test";
import { CaseCreatePage } from "../../pages/CaseCreatePage";
import { e2eCaseSubject } from "../../utils/selectors";

withRole(test, "approver");

test.describe("case creation — page structure", () => {
  test("requires every backend-mandated field before Create case is enabled", async ({ page }) => {
    test.setTimeout(60_000);

    const create = new CaseCreatePage(page);
    await create.goto();

    await expect(create.createButton()).toBeDisabled();

    try {
      await create.pickProject();
    } catch {
      test.skip(true, "No projects available in staging to exercise the cascade.");
      return;
    }
    await expect(create.createButton()).toBeDisabled();

    // Deployment only renders for non-cloud-support projects (isCloudProject
    // in the source page) — skip straight to Deployed product when it's
    // absent, same as fillRequiredFieldsAndSubmit's own `deployment?` param.
    const deploymentField = page.getByRole("combobox", { name: /^Deployment\s*\*?$/ });
    if (await deploymentField.isVisible().catch(() => false)) {
      await deploymentField.click();
      const deploymentOption = page.getByRole("listbox").getByRole("option").first();
      const hasDeployment = await deploymentOption
        .waitFor({ state: "visible", timeout: 10_000 })
        .then(() => true)
        .catch(() => false);
      test.skip(!hasDeployment, "No deployments available for this project in staging.");
      await deploymentOption.click();
      await expect(create.createButton()).toBeDisabled();
    }

    const productField = page.getByRole("combobox", { name: /^Deployed product\s*\*?$/ });
    await expect(productField).toBeEnabled({ timeout: 10_000 });
    await productField.click();
    const productOption = page.getByRole("listbox").getByRole("option").first();
    const hasProduct = await productOption
      .waitFor({ state: "visible", timeout: 10_000 })
      .then(() => true)
      .catch(() => false);
    test.skip(!hasProduct, "No deployed products available for this project/deployment in staging.");
    await productOption.click();
    await expect(create.createButton()).toBeDisabled();

    await create.selectOption("Severity", "S3 · Medium");
    await expect(create.createButton()).toBeDisabled();

    await create.selectOption("Issue type", "Error");
    await expect(create.createButton()).toBeDisabled();

    await create.subjectField().fill(e2eCaseSubject("page structure check"));
    await expect(create.createButton()).toBeDisabled();

    await create.fillDescription("[E2E] page-structure required-fields check.");
    await expect(create.createButton()).toBeEnabled();
  });
});

test.describe("case creation — happy path", () => {
  test("creates a real case and lands on its detail page", async ({ page }) => {
    // Real network round trips (project/deployment/product lookups, then
    // create), plus a navigation and a second fetch to load the detail
    // page — comfortably exceeds the 30s default.
    test.setTimeout(60_000);

    const create = new CaseCreatePage(page);
    await create.goto();
    try {
      await create.pickProject();
    } catch {
      test.skip(true, "No projects available in staging to create a case against.");
      return;
    }

    const deploymentField = page.getByRole("combobox", { name: /^Deployment\s*\*?$/ });
    if (await deploymentField.isVisible().catch(() => false)) {
      await deploymentField.click();
      const deploymentOption = page.getByRole("listbox").getByRole("option").first();
      const hasDeployment = await deploymentOption
        .waitFor({ state: "visible", timeout: 10_000 })
        .then(() => true)
        .catch(() => false);
      test.skip(!hasDeployment, "No deployments available for this project in staging.");
      await deploymentOption.click();
    }

    const productField = page.getByRole("combobox", { name: /^Deployed product\s*\*?$/ });
    await expect(productField).toBeEnabled({ timeout: 10_000 });
    await productField.click();
    const productOption = page.getByRole("listbox").getByRole("option").first();
    const hasProduct = await productOption
      .waitFor({ state: "visible", timeout: 10_000 })
      .then(() => true)
      .catch(() => false);
    test.skip(!hasProduct, "No deployed products available for this project/deployment in staging.");
    await productOption.click();

    const subject = e2eCaseSubject("e2e case creation");
    await create.selectOption("Severity", "S3 · Medium");
    await create.selectOption("Issue type", "Error");
    await create.subjectField().fill(subject);
    await create.fillDescription("[E2E] happy-path case created by create.spec.ts.");

    await expect(create.createButton()).toBeEnabled();
    await create.createButton().click();
    await expect(page).toHaveURL(/\/cases\/[^/]+$/, { timeout: 15_000 });

    // CsmCaseDetailPage titles itself with the case's own subject once
    // loaded, which is the strongest available confirmation that the record
    // we just created (not some other one) is what's showing.
    await expect(
      page.getByRole("heading", { level: 5, name: subject }),
    ).toBeVisible({ timeout: 15_000 });
  });
});
