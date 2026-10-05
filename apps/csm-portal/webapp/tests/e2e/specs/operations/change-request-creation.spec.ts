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
// Change request creation (POST /change-requests). Unlike case creation,
// there's no safe low-severity option to file under — every real submit
// here creates a permanent ServiceNow change request with no delete
// endpoint, so the happy-path test is deliberately the only one that
// actually submits, and its subject is always E2E-tagged (see
// e2eChangeRequestSubject) so it's identifiable in staging afterward.
//

import { test, expect, withRole } from "../../fixtures/test";
import { ChangeRequestCreatePage } from "../../pages/ChangeRequestCreatePage";
import { ChangeRequestDetailPage } from "../../pages/ChangeRequestDetailPage";
import {
  FAKE_CR_ID,
  FAKE_CREATOR,
  FAKE_DEPLOYMENTS,
  FAKE_DEPLOYMENT_PRODUCTS,
  FAKE_ENVIRONMENTS,
  FAKE_GROUPS,
  FAKE_PROJECTS,
  installFakeChangeRequestApi,
} from "../../utils/fakeChangeRequestApi";
import { e2eChangeRequestSubject } from "../../utils/selectors";

withRole(test, "approver");

test.describe("change request creation — page structure", () => {
  test("offers exactly Normal, Standard and Emergency, with none pre-selected", async ({ page }) => {
    const cr = new ChangeRequestCreatePage(page);
    await cr.goto();

    const radios = cr.typeGroup().getByRole("radio");
    await expect(radios).toHaveCount(3);
    await expect(cr.typeRadio("Normal")).not.toBeChecked();
    await expect(cr.typeRadio("Standard")).not.toBeChecked();
    await expect(cr.typeRadio("Emergency")).not.toBeChecked();
    // Order mirrors ServiceNow's "What type of change is required?" screen.
    await expect(radios.nth(0)).toHaveAttribute("value", "normal");
    await expect(radios.nth(1)).toHaveAttribute("value", "standard");
    await expect(radios.nth(2)).toHaveAttribute("value", "emergency");
  });

  test("requires a change type and a subject before Create change request is enabled", async ({ page }) => {
    const cr = new ChangeRequestCreatePage(page);
    await cr.goto();

    await expect(cr.createButton()).toBeDisabled();
    // A subject alone is not enough -- the type is required.
    await cr.subjectField().fill(e2eChangeRequestSubject("validation check"));
    await expect(cr.createButton()).toBeDisabled();
    await cr.selectType("Emergency");
    await expect(cr.createButton()).toBeEnabled();
  });

  test("sends the chosen type in the POST /change-requests payload", async ({ page }) => {
    const cr = new ChangeRequestCreatePage(page);
    await cr.goto();
    await cr.selectType("Standard");
    await cr.subjectField().fill(e2eChangeRequestSubject("payload check"));

    // Intercept (and abort) the create so this check never leaves a
    // permanent staging record behind -- it only inspects the request body.
    let sentType: unknown;
    await page.route(
      (url) => url.pathname.endsWith("/change-requests"),
      async (route) => {
        if (route.request().method() !== "POST") return route.fallback();
        sentType = (route.request().postDataJSON() as { type?: string }).type;
        await route.abort();
      },
    );
    await cr.createButton().click();
    await expect.poll(() => sentType).toBe("standard");
  });
});

test.describe("change request creation — Customer Approval / Customer Review checkboxes", () => {
  test("offers both as real checkboxes, unchecked by default, with their helper lines", async ({ page }) => {
    const cr = new ChangeRequestCreatePage(page);
    await cr.goto();

    await expect(cr.customerApprovalCheckbox()).not.toBeChecked();
    await expect(cr.customerReviewCheckbox()).not.toBeChecked();
    await expect(
      page.getByText("Adds a customer approval step after internal approval, before scheduling."),
    ).toBeVisible();
    await expect(page.getByText("Adds a customer review step after Review, before closing.")).toBeVisible();
    await expect(page.getByRole("switch", { name: /customer (approval|review)/i })).toHaveCount(0);
  });

  for (const [approval, review] of [
    [false, false],
    [true, false],
    [false, true],
    [true, true],
  ] as const) {
    test(`always sends both flags in the POST payload (approval ${approval}, review ${review})`, async ({ page }) => {
      const cr = new ChangeRequestCreatePage(page);
      await cr.goto();
      await cr.selectType("Normal");
      await cr.subjectField().fill(e2eChangeRequestSubject("customer flags payload check"));
      if (approval) await cr.customerApprovalCheckbox().check();
      if (review) await cr.customerReviewCheckbox().check();

      // Intercept (and abort) the create so this check never leaves a
      // permanent staging record behind -- it only inspects the request body.
      let sent: Record<string, unknown> | undefined;
      await page.route(
        (url) => url.pathname.endsWith("/change-requests"),
        async (route) => {
          if (route.request().method() !== "POST") return route.fallback();
          sent = route.request().postDataJSON() as Record<string, unknown>;
          await route.abort();
        },
      );
      await cr.createButton().click();
      await expect.poll(() => sent).toBeDefined();
      expect(sent).toMatchObject({ customerApprovalRequired: approval, customerReviewRequired: review });
    });
  }
});

test.describe("change request creation — happy path", () => {
  test("creates a real change request and lands on its detail page", async ({ page }) => {
    // Real network round trip to create, then a navigation and a second
    // fetch to load the detail page — comfortably exceeds the 30s default.
    test.setTimeout(60_000);

    const cr = new ChangeRequestCreatePage(page);
    await cr.goto();

    const subject = e2eChangeRequestSubject("e2e change request creation");
    await cr.fillSubjectAndSubmit(subject);

    // CsmChangeRequestDetailPage titles itself with the CR's own subject
    // once loaded, which is the strongest available confirmation that the
    // record we just created (not some other one) is what's showing.
    await expect(
      page.getByRole("heading", { level: 5, name: subject }),
    ).toBeVisible({ timeout: 15_000 });
  });
});

// ---------------------------------------------------------------------------
// Customer Project / Deployments / Environments / Deployment products,
// Customer Group, Category and the Communication area.
//
// Run against the in-browser fake of the change-request slice of the backend
// (see utils/fakeChangeRequestApi.ts), which serves the project picker, the
// POST /change-requests/link-options cascade lookup and a create / PATCH that
// validate exactly like the backend does -- so the cascade, the wire payload
// and the 400 path are exercised deterministically and no permanent record is
// created anywhere.
// ---------------------------------------------------------------------------

const ACME = FAKE_PROJECTS[0]!;
const BETA = FAKE_PROJECTS[1]!;
const ACME_PROD = FAKE_DEPLOYMENTS[0]!;
const ACME_STG = FAKE_DEPLOYMENTS[1]!;
const [PROD_ENV, STG_ENV] = FAKE_ENVIRONMENTS;
const ACME_PRODUCTS = FAKE_DEPLOYMENT_PRODUCTS.filter((p) => p.deploymentId === ACME_PROD.id);
const STG_PRODUCTS = FAKE_DEPLOYMENT_PRODUCTS.filter((p) => p.deploymentId === ACME_STG.id);

test.describe("change request creation — customer project cascade (mocked backend)", () => {
  test("lays out Customer Project, Deployments, Environments, Deployment products, Customer Group, Category and Communication", async ({
    page,
  }) => {
    await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR);
    const cr = new ChangeRequestCreatePage(page);
    await cr.goto();

    for (const locator of [
      cr.projectField(),
      cr.deploymentsField(),
      cr.environmentsField(),
      cr.deploymentProductsField(),
      cr.customerGroupField(),
      cr.categoryField(),
      cr.commentField(),
      cr.workNotesField(),
    ]) {
      await expect(locator).toBeVisible();
    }
    // Category pre-selects "Other", like the ServiceNow form.
    await expect(cr.categoryField()).toHaveText("Other");
    // None of the new fields is required: type + subject still gate Create.
    await cr.selectType("Normal");
    await cr.subjectField().fill(e2eChangeRequestSubject("layout"));
    await expect(cr.createButton()).toBeEnabled();
  });

  test("keeps Deployments and Environments disabled until a project and deployments are chosen; Deployment products is read-only", async ({
    page,
  }) => {
    await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR);
    const cr = new ChangeRequestCreatePage(page);
    await cr.goto();

    await expect(cr.deploymentsField()).toBeDisabled();
    await expect(cr.environmentsField()).toBeDisabled();
    await expect(cr.deploymentProductsField()).toHaveAttribute("readonly", "");
    await expect(page.getByText("Derived from the selected deployments")).toBeVisible();

    await cr.selectProject(ACME.name);
    await expect(cr.deploymentsField()).toBeEnabled();
    await expect(cr.environmentsField()).toBeDisabled();

    await cr.selectDeployments([ACME_PROD.name]);
    await expect(cr.environmentsField()).toBeEnabled();
  });

  test("choosing a project offers exactly that project's deployments", async ({ page }) => {
    await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR);
    const cr = new ChangeRequestCreatePage(page);
    await cr.goto();

    await cr.selectProject(ACME.name);
    expect(await cr.optionsOf(cr.deploymentsField())).toEqual([ACME_PROD.name, ACME_STG.name]);
  });

  test("choosing deployments derives the environments (preselected) and the deployment products", async ({ page }) => {
    await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR);
    const cr = new ChangeRequestCreatePage(page);
    await cr.goto();
    await cr.selectProject(ACME.name);

    await cr.selectDeployments([ACME_PROD.name]);
    await expect(cr.chipsOf(cr.deploymentsField())).toHaveText([ACME_PROD.name]);
    await expect(cr.chipsOf(cr.environmentsField())).toHaveText([PROD_ENV!.name]);
    await expect(cr.chipsOf(cr.deploymentProductsField())).toHaveText(ACME_PRODUCTS.map((p) => p.name));

    await cr.selectDeployments([ACME_STG.name]);
    await expect(cr.chipsOf(cr.environmentsField())).toHaveText([PROD_ENV!.name, STG_ENV!.name]);
    await expect(cr.chipsOf(cr.deploymentProductsField())).toHaveText(
      [...ACME_PRODUCTS, ...STG_PRODUCTS].map((p) => p.name),
    );
  });

  test("Environments only ever offers what the chosen deployments provide, and stays individually deselectable", async ({
    page,
  }) => {
    await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR);
    const cr = new ChangeRequestCreatePage(page);
    await cr.goto();
    await cr.selectProject(ACME.name);
    await cr.selectDeployments([ACME_PROD.name]);

    expect(await cr.optionsOf(cr.environmentsField())).toEqual([PROD_ENV!.name]);

    await cr.selectDeployments([ACME_STG.name]);
    expect(await cr.optionsOf(cr.environmentsField())).toEqual([PROD_ENV!.name, STG_ENV!.name]);
    await cr.toggleOptions(cr.environmentsField(), [STG_ENV!.name]);
    await expect(cr.chipsOf(cr.environmentsField())).toHaveText([PROD_ENV!.name]);
  });

  test("removing a deployment drops the environment and products only it provided", async ({ page }) => {
    await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR);
    const cr = new ChangeRequestCreatePage(page);
    await cr.goto();
    await cr.selectProject(ACME.name);
    await cr.selectDeployments([ACME_PROD.name, ACME_STG.name]);

    await cr.selectDeployments([ACME_PROD.name]); // toggles Production off
    await expect(cr.chipsOf(cr.deploymentsField())).toHaveText([ACME_STG.name]);
    await expect(cr.chipsOf(cr.environmentsField())).toHaveText([STG_ENV!.name]);
    await expect(cr.chipsOf(cr.deploymentProductsField())).toHaveText(STG_PRODUCTS.map((p) => p.name));
  });

  test("changing the project clears the dependents and offers the new project's deployments", async ({ page }) => {
    await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR);
    const cr = new ChangeRequestCreatePage(page);
    await cr.goto();
    await cr.selectProject(ACME.name);
    await cr.selectDeployments([ACME_PROD.name]);
    await expect(cr.chipsOf(cr.deploymentProductsField())).not.toHaveCount(0);

    await cr.selectProject(BETA.name);
    await expect(cr.chipsOf(cr.deploymentsField())).toHaveCount(0);
    await expect(cr.chipsOf(cr.environmentsField())).toHaveCount(0);
    await expect(cr.chipsOf(cr.deploymentProductsField())).toHaveCount(0);
    await expect(cr.environmentsField()).toBeDisabled();
    expect(await cr.optionsOf(cr.deploymentsField())).toEqual(["Beta Development"]);
  });

  test("clearing the project clears the dependents and disables Deployments again", async ({ page }) => {
    await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR);
    const cr = new ChangeRequestCreatePage(page);
    await cr.goto();
    await cr.selectProject(ACME.name);
    await cr.selectDeployments([ACME_PROD.name]);

    await cr.clearProject();
    await expect(cr.projectField()).toHaveValue("");
    await expect(cr.deploymentsField()).toBeDisabled();
    await expect(cr.chipsOf(cr.deploymentsField())).toHaveCount(0);
    await expect(cr.chipsOf(cr.deploymentProductsField())).toHaveCount(0);
  });
});

test.describe("change request creation — the customer scope on the wire (mocked backend)", () => {
  test("POST /change-requests carries projectId, deploymentIds, environmentIds, deploymentProductIds, customerGroupId, category, comment and workNote with the exact names", async ({
    page,
  }) => {
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR);
    const cr = new ChangeRequestCreatePage(page);
    await cr.goto();
    await cr.selectType("Normal");
    await cr.subjectField().fill(e2eChangeRequestSubject("scope payload"));
    await cr.selectProject(ACME.name);
    await cr.selectDeployments([ACME_PROD.name, ACME_STG.name]);
    await cr.selectCustomerGroup(FAKE_GROUPS[0]!.name);
    await cr.selectCategory("DevOps");
    await cr.commentField().fill("  Window is 02:00-04:00 UTC.  ");
    await cr.workNotesField().fill("Pre-checks done.");

    await cr.createButton().click();
    await expect(page).toHaveURL(new RegExp(`/operations/change-requests/${FAKE_CR_ID}$`));

    const create = api.requestBodies().find((r) => r.request === "POST /change-requests");
    expect(create?.body).toMatchObject({
      type: "normal",
      projectId: ACME.id,
      deploymentIds: [ACME_PROD.id, ACME_STG.id],
      environmentIds: [PROD_ENV!.id, STG_ENV!.id],
      deploymentProductIds: [...ACME_PRODUCTS, ...STG_PRODUCTS].map((p) => p.id),
      customerGroupId: FAKE_GROUPS[0]!.id,
      category: "devops",
      comment: "Window is 02:00-04:00 UTC.",
      workNote: "Pre-checks done.",
    });
    expect(api.journal()).toEqual([
      { kind: "comment", text: "Window is 02:00-04:00 UTC." },
      { kind: "workNote", text: "Pre-checks done." },
    ]);
  });

  test("sends only the environments left selected", async ({ page }) => {
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR);
    const cr = new ChangeRequestCreatePage(page);
    await cr.goto();
    await cr.selectType("Normal");
    await cr.subjectField().fill(e2eChangeRequestSubject("env subset"));
    await cr.selectProject(ACME.name);
    await cr.selectDeployments([ACME_PROD.name, ACME_STG.name]);
    await cr.toggleOptions(cr.environmentsField(), [STG_ENV!.name]);

    await cr.createButton().click();
    await expect(page).toHaveURL(new RegExp(`/operations/change-requests/${FAKE_CR_ID}$`));
    const create = api.requestBodies().find((r) => r.request === "POST /change-requests");
    expect(create?.body).toMatchObject({ deploymentIds: [ACME_PROD.id, ACME_STG.id], environmentIds: [PROD_ENV!.id] });
  });

  test("omits every new field that was left empty (arrays only when non-empty); Category still defaults to other", async ({
    page,
  }) => {
    const api = await installFakeChangeRequestApi(page, "standard", FAKE_CREATOR);
    const cr = new ChangeRequestCreatePage(page);
    await cr.goto();
    await cr.fillSubjectAndSubmit(e2eChangeRequestSubject("empty scope"), "Standard");

    const body = api.requestBodies().find((r) => r.request === "POST /change-requests")?.body ?? {};
    for (const key of ["projectId", "deploymentIds", "environmentIds", "deploymentProductIds", "customerGroupId", "comment", "workNote"]) {
      expect(body, key).not.toHaveProperty(key);
    }
    expect(body).toMatchObject({ type: "standard", category: "other" });
  });

  test("a project alone is sent without empty deployment / environment / product arrays", async ({ page }) => {
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR);
    const cr = new ChangeRequestCreatePage(page);
    await cr.goto();
    await cr.selectType("Normal");
    await cr.subjectField().fill(e2eChangeRequestSubject("project only"));
    await cr.selectProject(BETA.name);
    await cr.createButton().click();
    await expect(page).toHaveURL(new RegExp(`/operations/change-requests/${FAKE_CR_ID}$`));

    const body = api.requestBodies().find((r) => r.request === "POST /change-requests")?.body ?? {};
    expect(body).toHaveProperty("projectId", BETA.id);
    for (const key of ["deploymentIds", "environmentIds", "deploymentProductIds"]) {
      expect(body, key).not.toHaveProperty(key);
    }
  });

  test("shows the backend's 400 about an inconsistent combination verbatim and stays on the form", async ({ page }) => {
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR);
    const cr = new ChangeRequestCreatePage(page);
    await cr.goto();
    await cr.selectType("Normal");
    await cr.subjectField().fill(e2eChangeRequestSubject("stale deployment"));
    await cr.selectProject(ACME.name);
    await cr.selectDeployments([ACME_PROD.name]);

    // The deployment is deactivated server-side after the form loaded its options.
    api.retireDeployment(ACME_PROD.id);
    await cr.createButton().click();

    await expect(page.getByText(`deploymentIds: deployment ${ACME_PROD.name} is not an active deployment of the selected project`)).toBeVisible();
    await expect(page).toHaveURL(/\/operations\/change-requests\/new$/);
    await expect(cr.createButton()).toBeEnabled();
  });
});

test.describe("change request creation — draft and clone (mocked backend)", () => {
  test("the project, deployments, environments, customer group, category and notes survive leaving the form and coming back", async ({
    page,
  }) => {
    await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR);
    const cr = new ChangeRequestCreatePage(page);
    await cr.goto();
    await cr.selectProject(ACME.name);
    await cr.selectDeployments([ACME_PROD.name, ACME_STG.name]);
    await cr.toggleOptions(cr.environmentsField(), [STG_ENV!.name]);
    await cr.selectCustomerGroup(FAKE_GROUPS[1]!.name);
    await cr.selectCategory("Network");
    await cr.commentField().fill("draft comment");
    await cr.workNotesField().fill("draft note");

    // Leave for another page of the portal and come back (same tab, so the sessionStorage draft is still there).
    await page.goto("/operations?tab=change_requests");
    await expect(page).toHaveURL(/\/operations\?tab=change_requests/);
    await expect(page.getByRole("button", { name: "New change request" }).or(page.getByText("Change requests").first())).toBeVisible();
    await cr.goto();

    await expect(cr.projectField()).toHaveValue(ACME.name);
    await expect(cr.chipsOf(cr.deploymentsField())).toHaveText([ACME_PROD.name, ACME_STG.name]);
    await expect(cr.chipsOf(cr.environmentsField())).toHaveText([PROD_ENV!.name]);
    await expect(cr.chipsOf(cr.deploymentProductsField())).toHaveText(
      [...ACME_PRODUCTS, ...STG_PRODUCTS].map((p) => p.name),
    );
    await expect(cr.customerGroupField()).toHaveValue(FAKE_GROUPS[1]!.name);
    await expect(cr.categoryField()).toHaveText("Network");
    await expect(cr.commentField()).toHaveValue("draft comment");
    await expect(cr.workNotesField()).toHaveValue("draft note");
  });

  test("Clone carries the project, customer group and category, and leaves the deployments for the new target to be chosen", async ({
    page,
  }) => {
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR);
    const cr = new ChangeRequestCreatePage(page);
    const detail = new ChangeRequestDetailPage(page);

    // Create a source change request with the full scope ...
    await cr.goto();
    await cr.selectType("Normal");
    await cr.subjectField().fill(e2eChangeRequestSubject("clone source"));
    await cr.selectProject(ACME.name);
    await cr.selectDeployments([ACME_PROD.name]);
    await cr.selectCustomerGroup(FAKE_GROUPS[0]!.name);
    await cr.selectCategory("DevOps");
    await cr.createButton().click();
    await expect(detail.lifecycleStepper()).toBeVisible();
    expect(api.scope().deploymentIds).toEqual([ACME_PROD.id]);

    // ... and clone it.
    await detail.cloneButton().click();
    await expect(page.getByRole("heading", { name: "New change request" })).toBeVisible();
    await expect(cr.projectField()).toHaveValue(ACME.name);
    await expect(cr.customerGroupField()).toHaveValue(FAKE_GROUPS[0]!.name);
    await expect(cr.categoryField()).toHaveText("DevOps");
    await expect(cr.deploymentsField()).toBeEnabled();
    await expect(cr.chipsOf(cr.deploymentsField())).toHaveCount(0);
    await expect(cr.chipsOf(cr.environmentsField())).toHaveCount(0);
    await expect(cr.chipsOf(cr.deploymentProductsField())).toHaveCount(0);
    await expect(page.getByText(/deployments, environments, deployment products, schedule/i)).toBeVisible();
  });
});
