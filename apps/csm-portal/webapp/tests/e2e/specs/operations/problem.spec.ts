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
// Problem management (list + create + detail). `POST /problems` is wired to
// the real csm-portal-backend with no delete endpoint, so — same rule as
// change requests/incidents — the happy-path test is deliberately the only
// one that actually submits, tagged via e2eProblemSubject. Subject is the
// only required field on create (see ProblemCreatePage), which makes this
// the simplest of the create flows to cover.
//

import { test, expect, withRole } from "../../fixtures/test";
import { ProblemsListPage } from "../../pages/ProblemsListPage";
import { ProblemCreatePage } from "../../pages/ProblemCreatePage";
import { ProblemDetailPage } from "../../pages/ProblemDetailPage";
import { e2eProblemSubject, PROBLEM_CREATE } from "../../utils/selectors";

withRole(test, "approver");

test.describe("problems tab", () => {
  test("lists and links to create", async ({ page }) => {
    const problems = new ProblemsListPage(page);
    await problems.goto();

    await expect(
      page.getByRole("tab", { name: "Problem management", selected: true }),
    ).toBeVisible();
    await expect(problems.createProblemButton()).toBeVisible();

    await problems.createProblemButton().click();
    await expect(page).toHaveURL(new RegExp(PROBLEM_CREATE.path.replace("/", "\\/")));
  });
});

test.describe("problems list", () => {
  test("search + state filter re-query the list", async ({ page }) => {
    const problems = new ProblemsListPage(page);
    await problems.goto();

    // A deliberately narrow, near-certainly-empty query — this suite makes
    // no assumption about what problems already exist in staging, so it only
    // asserts the search interaction round-trips (re-queries) rather than
    // asserting on a specific result set.
    await problems.search("zzz-no-such-problem-e2e-query-zzz");
    await expect(problems.searchBox()).toHaveValue("zzz-no-such-problem-e2e-query-zzz");
    await expect(problems.rows()).toHaveCount(0);

    await problems.clearSearch();
    await expect(problems.searchBox()).toHaveValue("");

    // State filter: exercised as an interaction only (options are a fixed
    // enum, so this doesn't depend on data either) — no assertion on row
    // count since staging data is unknown and may legitimately be empty.
    await problems.selectStateFilter("New");
    await problems.clearFilters();
  });
});

test.describe("problem creation — page structure", () => {
  test("Create problem is disabled until Subject is filled", async ({ page }) => {
    const create = new ProblemCreatePage(page);
    await create.goto();

    await expect(create.createButton()).toBeDisabled();

    await create.subjectField().fill(e2eProblemSubject("validation check"));
    await expect(create.createButton()).toBeEnabled();
  });
});

test.describe("problem creation — happy path", () => {
  test("creates a real problem and lands on its detail page", async ({ page }) => {
    // Real network round trips (create, then a navigation and fetch to load
    // the detail page) — comfortably exceeds the 30s default.
    test.setTimeout(60_000);

    const create = new ProblemCreatePage(page);
    await create.goto();

    const subject = e2eProblemSubject("e2e problem creation");
    await create.fillSubjectAndSubmit(subject);

    // ProblemDetailPage titles itself with the problem's own subject once
    // loaded, which is the strongest available confirmation that the record
    // we just created (not some other one) is what's showing.
    await expect(
      page.getByRole("heading", { level: 5, name: subject }),
    ).toBeVisible({ timeout: 15_000 });
  });
});

test.describe("problem detail", () => {
  test("read-only view renders with a back button and no mutation controls", async ({ page }) => {
    test.setTimeout(60_000);

    // Self-provisioned: create a problem here rather than depending on the
    // happy-path test's ordering or on any pre-existing staging data.
    const create = new ProblemCreatePage(page);
    await create.goto();
    const subject = e2eProblemSubject("e2e problem detail read-only");
    await create.fillSubjectAndSubmit(subject);

    const match = page.url().match(/\/operations\/problems\/([^/?#]+)$/);
    expect(match).not.toBeNull();
    const id = match?.[1] as string;

    const detail = new ProblemDetailPage(page);
    await detail.goto(id);

    await expect(page.getByRole("heading", { level: 5, name: subject })).toBeVisible();
    await expect(detail.backButton()).toBeVisible();

    // Problems are read-only — there is no Edit dialog (no mutation endpoint
    // yet), unlike change requests/incidents.
    await expect(page.getByRole("button", { name: /^Edit/ })).toHaveCount(0);
    await expect(page.getByRole("button", { name: /^Delete/ })).toHaveCount(0);
    await expect(page.getByRole("button", { name: /^Save/ })).toHaveCount(0);

    await detail.goBack();
    await expect(
      page.getByRole("tab", { name: "Problem management", selected: true }),
    ).toBeVisible();
  });
});
