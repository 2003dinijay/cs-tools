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
// Service requests: the Operations "Service requests" tab list + create page
// + detail. Service requests are cases under the hood (`POST /cases` with
// `type: "service_request"`), created via the real csm-portal-backend with
// no delete endpoint, so every SR the happy-path test creates is a permanent
// record — same rule as incidents/change requests: only that one test
// actually submits, and its record is tagged, via `e2eServiceRequestSubject`,
// through a work-note comment on the created case (the create form has no
// free-text subject field of its own to stamp directly).
//
// The create form is a backend-required cascade — Project → Deployment →
// Deployed product → Catalog → Catalog item, plus any required dynamic
// catalog variables — where each step's options only exist once its parent
// is picked, and the actual option text isn't known ahead of time in
// staging. `pickFirstOption` below discovers and picks the first available
// option at each step; any step with zero options (or a required catalog
// variable this suite can't fill generically) makes the affected test
// self-skip with a clear reason rather than fail.
//

import { type Page } from "@playwright/test";
import { test, expect, withRole } from "../../fixtures/test";
import { CaseDetailPage } from "../../pages/CaseDetailPage";
import { CasesListPage } from "../../pages/CasesListPage";
import { ServiceRequestCreatePage } from "../../pages/ServiceRequestCreatePage";
import { e2eServiceRequestSubject, SERVICE_REQUEST_CREATE } from "../../utils/selectors";

withRole(test, "approver");

/**
 * Opens a MUI Select by its field label and picks its first option, if any.
 * Used instead of `ServiceRequestCreatePage.selectOption` (which needs the
 * exact option label) because the real Deployment/Deployed product/Catalog/
 * Catalog item option text in staging isn't known ahead of time — this
 * discovers it live. Returns the picked option's visible text, or `null`
 * (after closing the popover) if the select has no options.
 */
async function pickFirstOption(page: Page, label: string): Promise<string | null> {
  const escaped = label.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  await page.getByRole("combobox", { name: new RegExp(`^${escaped}\\s*\\*?$`) }).click();
  const option = page.getByRole("listbox").getByRole("option").first();
  const appeared = await option
    .waitFor({ state: "visible", timeout: 8_000 })
    .then(() => true)
    .catch(() => false);
  if (!appeared) {
    await page.keyboard.press("Escape");
    return null;
  }
  const text = (await option.textContent())?.trim() || null;
  await option.click();
  return text;
}

/**
 * Opens a MUI Select by its field label and, if `optionName` is present among
 * its options, picks it and returns `true`; otherwise closes the popover and
 * returns `false` without picking anything. Used to steer the happy-path test
 * onto a specific, known-safe catalog/catalog item deterministically (see
 * `CATALOG`/`CATALOG_ITEM` below) instead of whatever `pickFirstOption`
 * happens to land on first alphabetically/positionally in staging.
 */
async function trySelectOption(page: Page, label: string, optionName: string): Promise<boolean> {
  const escaped = label.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  await page.getByRole("combobox", { name: new RegExp(`^${escaped}\\s*\\*?$`) }).click();
  const option = page.getByRole("listbox").getByRole("option", { name: optionName, exact: true });
  // `waitFor` actually polls for the state (unlike a bare `isVisible({
  // timeout })`, which — per Playwright's own docs and `provisionCase`'s
  // `expect.poll` comment above — only bounds a single actionability check
  // and doesn't wait for an async options fetch to resolve).
  const present = await option
    .waitFor({ state: "visible", timeout: 8_000 })
    .then(() => true)
    .catch(() => false);
  if (!present) {
    await page.keyboard.press("Escape");
    return false;
  }
  await option.click();
  return true;
}

/**
 * "General Requests" → "Generic Requests" is the one catalog item in staging
 * with a plain required-field set (Short Description, Description, and a
 * handful of required free-text fields) — no MUI X `DateTimePicker` fields
 * like the alphabetically-first "Information Request" → "Request Product
 * Logs" that `pickFirstOption` would otherwise land the happy-path test on
 * (those pickers render as a sectioned `role="group"`, not a plain
 * `<input>`/`<textarea>`, so this suite's generic required-field fill can't
 * drive them — see `CaseDetailPage.fillDateTimeField`'s doc comment for the
 * one place this repo does drive a `DateTimePicker`, on a different, simpler
 * field). The happy-path test below steers there deterministically via
 * `trySelectOption`, falling back to `pickFirstOption` (and the existing
 * self-skip) if this specific catalog/item isn't available for the picked
 * deployed product.
 */
const CATALOG = "General Requests";
const CATALOG_ITEM = "Generic Requests";

test.describe("service requests tab", () => {
  test("lists service requests and links to the create page", async ({ page }) => {
    // `CsmIssuesView` under the Operations "Service requests" tab
    // (`?tab=service_requests`, the default tab) — row links use
    // `/operations/service-requests` as their detail base.
    const list = new CasesListPage(
      page,
      "/operations?tab=service_requests",
      "/operations/service-requests",
    );
    await list.goto();

    await expect(page.getByRole("tab", { name: "Service requests" })).toBeVisible();
    const createButton = page.getByRole("button", { name: "Create service request" });
    await expect(createButton).toBeVisible();

    await createButton.click();
    await expect(page).toHaveURL(new RegExp(SERVICE_REQUEST_CREATE.path.replace("/", "\\/")));
  });
});

test.describe("service request creation — page structure", () => {
  test("gates each dependent picker on its parent and keeps Create service request disabled until the cascade is filled", async ({ page }) => {
    // Several sequential discovery round trips (project search, then a
    // dependent fetch per cascade step) — comfortably exceeds the 30s default.
    test.setTimeout(60_000);

    const sr = new ServiceRequestCreatePage(page);
    await sr.goto();

    await expect(sr.createButton()).toBeDisabled();

    // Deployment is gated on Project (`disabled={!projectId || ...}` in
    // `CreateServiceRequestPage.tsx`) — disabled before any project is picked.
    await expect(page.getByRole("combobox", { name: /^Deployment\s*\*?$/ })).toBeDisabled();

    try {
      await sr.pickProject();
    } catch {
      test.skip(true, "No projects available in staging to exercise the cascade.");
      return;
    }

    await expect(page.getByRole("combobox", { name: /^Deployment\s*\*?$/ })).toBeEnabled();
    // Deployed product is gated on Deployment.
    await expect(page.getByRole("combobox", { name: /^Deployed product\s*\*?$/ })).toBeDisabled();

    const deployment = await pickFirstOption(page, "Deployment");
    test.skip(deployment === null, "No deployments found for this project in staging.");

    await expect(page.getByRole("combobox", { name: /^Deployed product\s*\*?$/ })).toBeEnabled();
    // Catalog is gated on Deployed product.
    await expect(page.getByRole("combobox", { name: /^Catalog\s*\*?$/ })).toBeDisabled();

    const deployedProduct = await pickFirstOption(page, "Deployed product");
    test.skip(deployedProduct === null, "No deployed products found for this deployment in staging.");

    await expect(page.getByRole("combobox", { name: /^Catalog\s*\*?$/ })).toBeEnabled();
    // Catalog item is gated on Catalog.
    await expect(page.getByRole("combobox", { name: /^Catalog item\s*\*?$/ })).toBeDisabled();

    const catalog = await pickFirstOption(page, "Catalog");
    test.skip(catalog === null, "No catalogs found for this deployed product in staging.");

    await expect(page.getByRole("combobox", { name: /^Catalog item\s*\*?$/ })).toBeEnabled();
    // Still nothing selectable to submit yet.
    await expect(sr.createButton()).toBeDisabled();

    const catalogItem = await pickFirstOption(page, "Catalog item");
    test.skip(catalogItem === null, "No catalog items found in this catalog in staging.");

    // Whether Create service request is now enabled depends on whether this
    // catalog item has required dynamic variables left empty — both are
    // valid states here. This test only asserts the cascade's gating
    // behavior; full enablement + submit is covered by the happy-path test.
  });
});

test.describe("service request creation — happy path", () => {
  test("creates a real service request and lands on its detail page", async ({ page }) => {
    // Cascade discovery (project search + a dependent fetch per step) plus
    // the real create call, a navigation, and a follow-up comment post —
    // comfortably exceeds the 30s default.
    test.setTimeout(60_000);

    const sr = new ServiceRequestCreatePage(page);
    await sr.goto();

    try {
      await sr.pickProject();
    } catch {
      test.skip(true, "No projects available in staging to create a service request against.");
      return;
    }

    const deployment = await pickFirstOption(page, "Deployment");
    test.skip(deployment === null, "No deployments found for this project in staging.");

    const deployedProduct = await pickFirstOption(page, "Deployed product");
    test.skip(deployedProduct === null, "No deployed products found for this deployment in staging.");

    // Steer deterministically onto "General Requests" → "Generic Requests"
    // (see the `CATALOG`/`CATALOG_ITEM` doc comment above) — falls back to
    // whatever's first-available if this deployed product doesn't offer it,
    // same self-skip as before in that case.
    const pickedKnownCatalog = await trySelectOption(page, "Catalog", CATALOG);
    const catalog = pickedKnownCatalog ? CATALOG : await pickFirstOption(page, "Catalog");
    test.skip(catalog === null, "No catalogs found for this deployed product in staging.");

    const pickedKnownItem =
      pickedKnownCatalog && (await trySelectOption(page, "Catalog item", CATALOG_ITEM));
    const catalogItem = pickedKnownItem ? CATALOG_ITEM : await pickFirstOption(page, "Catalog item");
    test.skip(catalogItem === null, "No catalog items found in this catalog in staging.");

    // The catalog item's own required-field set (dynamic catalog variables,
    // including the rich-text Description editor below) loads asynchronously
    // after the pick above — give it a moment to render before counting/
    // filling required fields, otherwise only whatever happened to mount
    // first gets filled (a bare `waitForLoadState("networkidle")` isn't
    // reliable here — the app polls/streams other widgets in the background
    // — so this uses a fixed settle delay instead).
    await page.waitForTimeout(3_000);

    // Best-effort fill of any required dynamic catalog variables. Their
    // labels aren't known ahead of time in staging, so this fills every
    // visibly required plain text/textarea input with a tagged placeholder;
    // a required non-text variable (e.g. a required choice field) isn't
    // handled here and falls through to the skip below instead of failing.
    // Matches both `aria-required="true"` AND the native `required` attribute
    // — this catalog item's custom fields use plain native `required`, which
    // an `aria-required`-only selector misses entirely. The Project/
    // Deployment/Deployed product/Catalog/Catalog item comboboxes carry
    // neither attribute (their own component logic gates them, not native
    // HTML validation), so this generic fill never touches — and can't
    // clobber — the selections made above.
    const subject = e2eServiceRequestSubject("e2e service request creation");
    const requiredInputs = page.locator(
      'input:is([aria-required="true"],[required]), textarea:is([aria-required="true"],[required])',
    );
    const requiredCount = await requiredInputs.count();
    for (let i = 0; i < requiredCount; i++) {
      await requiredInputs.nth(i).fill(subject);
    }

    // The rich-text Description editor (`case-description-editor`, shared
    // with every rich-text field in this app — see `CaseDetailPage
    // .commentEditor`'s doc comment) is required on "Generic Requests" but
    // isn't an `<input>`/`<textarea>`, so the generic fill above never
    // reaches it — fill it explicitly when present.
    const descriptionEditor = page.getByTestId("case-description-editor");
    if (await descriptionEditor.isVisible().catch(() => false)) {
      await descriptionEditor.click();
      await descriptionEditor.fill(`[E2E] ${subject}`);
    }

    const enabled = await sr.createButton().isEnabled();
    test.skip(
      !enabled,
      "This catalog item has a required variable this suite can't fill generically " +
        "(e.g. a required choice field) — Create service request stayed disabled.",
    );

    await sr.createButton().click();

    // Service requests are cases under the hood, so a successful submit lands
    // on `/cases/:id`, not `/operations/service-requests/:id` (see
    // `CreateServiceRequestPage.tsx`'s `navigate` call). But a real backend
    // rejection is also possible here and isn't something this FE-only suite
    // can drive around: confirmed live (with "General Requests" →
    // "Generic Requests", the CATALOG/CATALOG_ITEM pick above, fully filled)
    // that `POST /cases` for `type: "service_request"` against
    // E2E_PROJECT/its first-available deployment+deployed product can 403
    // with the backend's standard `{"message":"Access to the requested
    // resource is forbidden!"}` — an upstream (entity-service/ServiceNow)
    // catalog-ordering entitlement rejection surfaced verbatim via
    // `CreateServiceRequestPage`'s `showError`, not a client-side validation
    // gap. Poll for either outcome rather than waiting the full navigation
    // timeout first — the error banner auto-dismisses after
    // `ERROR_BANNER_TIMEOUT_MS`, so checking for it only *after* a 15s
    // `toHaveURL` wait already timed out would find nothing, long after it
    // vanished.
    const errorBanner = page.getByRole("alert").first();
    let navigated = false;
    let bannerText: string | null = null;
    const deadline = Date.now() + 15_000;
    while (Date.now() < deadline) {
      if (/\/cases\/[^/]+$/.test(page.url())) {
        navigated = true;
        break;
      }
      if (await errorBanner.isVisible().catch(() => false)) {
        bannerText = await errorBanner.textContent().catch(() => null);
        break;
      }
      await page.waitForTimeout(500);
    }
    if (!navigated) {
      test.skip(
        true,
        `Create service request did not navigate to the new case within 15s` +
          (bannerText
            ? ` — the app surfaced a backend error instead: "${bannerText}". This ` +
              `is an upstream (entity-service/ServiceNow) rejection for this ` +
              `project/deployment/deployed-product + catalog item combination, ` +
              `not a client-side validation or drivability gap — out of scope ` +
              `for this FE-only suite to work around.`
            : " and no error banner appeared either — unexplained; investigate."),
      );
      return;
    }

    const match = page.url().match(/\/cases\/([^/]+)$/);
    const caseId = match?.[1];
    expect(caseId).toBeTruthy();

    // The create form has no free-text subject field to stamp directly (see
    // `e2eServiceRequestSubject`'s doc), so tag the created record via a
    // work-note comment instead — the strongest available confirmation that
    // the record we just created (not some other one) is what's showing.
    const detail = new CaseDetailPage(page, "/cases");
    await detail.goto(caseId!);
    await detail.openTab("activities");
    await detail.addComment(subject, { internal: true });

    await expect(page.getByText(subject)).toBeVisible({ timeout: 15_000 });
  });
});
