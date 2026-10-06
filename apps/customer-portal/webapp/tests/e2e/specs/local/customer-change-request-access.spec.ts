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
// WHO MAY NOT: the negative side of a customer answering a change request, on the
// real local stack.
//
//   1. A customer of ANOTHER project (mira.santos, "Lumen Works Platform") is never
//      offered Approve / Reject / Propose on Example Corp's CHG-FIXED-007, cannot
//      list it, and cannot answer it or propose a time for it (403), whatever
//      address she types.
//   2. A viewer with NOTHING PENDING sees no buttons: a contact who was never asked
//      (the change is not in a customer state at all: CHG-FIXED-005 in New,
//      CHG-FIXED-006 in Review) and a contact whose own request was cancelled while
//      the change still waits on her colleague.
//   3. A direct PATCH from a customer's token that carries anything but an answer, or
//      a proposed time, is refused (403), and the two mixed forms are refused (400);
//      nothing about the change moves.
//
// "Cannot SEE it" needs a word of care. entity-service hides another project's rows
// with Postgres row-level security, and the local compose stack connects as the
// `postgres` role, a SUPERUSER, which bypasses it (entity-service/CLAUDE.md; "Local
// seed personas"): there, mira's by-id read of CHG-FIXED-007 answers 200, with
// `customerCanAnswer: false`, so the assertion that holds on every stack is "never
// offered, never answerable". Run against entity-service as a non-superuser role and
// say so with E2E_ENTITY_RLS=1, and the same read is asserted to be a 404, and the
// page to be the error page.
//
// ⚠️ STATE-CHANGING in the small (it re-seeds first, and one test cancels erin's request
// in the database), so it needs E2E_POSTGRES_CONTAINER and SKIPS without it.
//

import { test, expect } from "../../fixtures/test";
import { LOCAL_PERSONAS, openLocalContext, withLocalSession } from "../../auth/localSessions";
import { ChangeRequestDetailsPage } from "../../pages/ChangeRequestDetailsPage";
import { ChangeRequestsPage } from "../../pages/ChangeRequestsPage";
import {
  FIXTURES,
  approverRows,
  changeRequestRow,
  customerApi,
  psql,
  resetFixtures,
  withFixtureStack,
} from "../../utils/localStack";
import { CHANGE_REQUEST_DETAILS as UI } from "../../utils/selectors";

withLocalSession(test, "dave");
withFixtureStack(test);

const { approval, inReview, standardNew, projectId } = FIXTURES;

/** True when entity-service under test runs as a role that row-level security applies to. */
const ENTITY_ENFORCES_RLS = process.env.E2E_ENTITY_RLS === "1";

/** What the Lumen Works Platform project is called (its id is random per database). */
const LUMEN_PROJECT = LOCAL_PERSONAS.mira.project;

test.describe("Local stack — who may not answer a change request", () => {
  test.describe.configure({ timeout: 180_000 });

  test.beforeEach(async () => {
    await resetFixtures();
  });

  test(`${LOCAL_PERSONAS.mira.email}, a customer of another project, is never offered the answer, cannot list ${approval.number} and is refused when she tries to answer or propose`, async ({
    browser,
    baseURL,
  }) => {
    const mira = customerApi("mira");
    const lumenId = await mira.projectIdByName(LUMEN_PROJECT);
    expect(lumenId, `${LUMEN_PROJECT} is not one of mira's projects: is the stack seeded?`).toBeTruthy();

    // 1. The API. A read by id is a 404 where row-level security applies, and where it is
    // bypassed (a superuser role) it is at least never an offer to answer.
    const read = await mira.get(approval.id);
    if (ENTITY_ENFORCES_RLS) {
      expect(read.status, "another project's change request must not exist for her").toBe(404);
    } else {
      expect([200, 404], `unexpected status ${read.status}`).toContain(read.status);
      if (read.status === 200) expect(read.body.customerCanAnswer, "offered to a stranger").not.toBe(true);
    }

    // 2. She cannot list it: neither under her own project nor under Example Corp's.
    for (const id of [lumenId!, projectId]) {
      const numbers = await mira.listedNumbers(id);
      expect(
        numbers.filter((n) => n.startsWith("CHG-FIXED")),
        `Example Corp's change requests are listed for mira under project ${id}`,
      ).toEqual([]);
    }

    // 3. She cannot answer it, and cannot propose a time for it.
    const attempts: [string, unknown][] = [
      ["approve", { isCustomerApproved: true }],
      ["reject", { isCustomerApproved: false }],
      ["propose", { plannedStartOn: "2027-03-01 10:00:00", plannedEndOn: "2027-03-01 12:00:00" }],
    ];
    for (const [what, body] of attempts) {
      const result = await mira.patch(approval.id, body);
      expect([403, 404], `mira's "${what}" answered ${result.status}: ${JSON.stringify(result.body)}`).toContain(
        result.status,
      );
    }
    const untouched = await changeRequestRow(approval.id);
    expect([untouched.state, untouched.startUtc, untouched.endUtc]).toEqual(["CUSTOMER_APPROVAL", "", ""]);
    expect(
      (await approverRows(approval.id)).map((r) => `${r.email}|${r.status}`),
      "the two contacts' requests are untouched",
    ).toEqual(["dave.mendis@example.com|requested", "erin.jayawardena@example.com|requested"]);

    // 4. The browser, at the address she would type: under her own project and under Example Corp's.
    const context = await openLocalContext(test, browser, "mira", { baseURL });
    try {
      const page = await context.newPage();
      const details = new ChangeRequestDetailsPage(page);
      const own = await details.openAndSettle(lumenId!, approval.id, approval.number);
      await expect(details.answerButtons(), "buttons offered to a customer of another project").toHaveCount(0);
      if (ENTITY_ENFORCES_RLS) expect(own.loaded, "another project's change request rendered").toBe(false);

      // Example Corp's address: the project itself is refused her (404), so no page is built.
      const [projectRefusal] = await Promise.all([
        page.waitForResponse(
          (r) => new URL(r.url()).pathname.endsWith(`/projects/${projectId}`) && r.request().method() === "GET",
          { timeout: 60_000 },
        ),
        page.goto(`/projects/${projectId}/operations/change-requests/${approval.id}`),
      ]);
      expect(projectRefusal.status(), "Example Corp's project must be refused to a customer who is not its contact").toBe(404);
      await expect(details.answerButtons(), "buttons offered under Example Corp's address").toHaveCount(0);

      // Her own list shows none of Example Corp's change requests (when her project has the page at all).
      const list = new ChangeRequestsPage(page);
      const listed = await list.open(lumenId!).then(() => true, () => false);
      if (listed) {
        await list.waitForList();
        await expect(page.getByText(/CHG-FIXED-\d+/)).toHaveCount(0);
      } else {
        test.info().annotations.push({
          type: "note",
          description: `${LUMEN_PROJECT} has no change requests page; the API listing above is the assertion`,
        });
      }
    } finally {
      await context.close();
    }
  });

  test(`a viewer with nothing pending sees no buttons: ${standardNew.number} in New, ${inReview.number} in Review, and a contact whose own request was cancelled`, async ({
    page,
    browser,
    baseURL,
  }) => {
    const dave = new ChangeRequestDetailsPage(page);

    // Not in a customer state: nobody is asked, so there is nothing to answer.
    for (const [change, stage] of [
      [standardNew, UI.stages.new],
      [inReview, UI.stages.review],
    ] as const) {
      await dave.open(projectId, change.id, change.number);
      await expect(dave.currentStage(), `${change.number} is not in ${stage}`).toHaveText(stage);
      await expect(dave.answerButtons(), `${change.number} offers an answer in ${stage}`).toHaveCount(0);
      expect((await customerApi("dave").get(change.id)).body.customerCanAnswer).toBe(false);
    }

    // In Customer Approval, but erin's own request was cancelled while dave's stands
    // (what a colleague's answer or a re-schedule leaves behind): the offer is per viewer.
    await psql(
      `update approval_stage_approver a set status = 'cancelled' from "user" u ` +
        `where a.approver_user_id = u.id and a.work_item_id = '${approval.id}' ` +
        `and u.email = '${LOCAL_PERSONAS.erin.email}'`,
    );
    const erinContext = await openLocalContext(test, browser, "erin", { baseURL });
    try {
      const erin = new ChangeRequestDetailsPage(await erinContext.newPage());
      await erin.open(projectId, approval.id, approval.number);
      await expect(erin.currentStage()).toHaveText(UI.stages.customerApproval);
      await expect(erin.answerButtons(), "erin has no pending request but was offered an answer").toHaveCount(0);
      expect((await customerApi("erin").get(approval.id)).body.customerCanAnswer).toBe(false);

      await dave.open(projectId, approval.id, approval.number);
      for (const name of [UI.buttons.proposeNewTime, UI.buttons.approve, UI.buttons.reject]) {
        await expect(dave.button(name), `${name} is missing for the contact who is still asked`).toBeVisible();
      }
      expect((await customerApi("dave").get(approval.id)).body.customerCanAnswer).toBe(true);

      // And the backend agrees with the page, not just the page with itself: erin's answer is refused.
      const refused = await customerApi("erin").patch(approval.id, { isCustomerApproved: true });
      expect([403, 409], `erin's answer without a pending request: ${JSON.stringify(refused.body)}`).toContain(
        refused.status,
      );
      expect((await changeRequestRow(approval.id)).state).toBe("CUSTOMER_APPROVAL");
    } finally {
      await erinContext.close();
    }
  });

  test(`a direct PATCH from dave's token with anything but an answer or a proposed time is refused, and nothing about ${approval.number} moves`, async () => {
    const dave = customerApi("dave");
    const window = { plannedStartOn: "2027-03-01 10:00:00", plannedEndOn: "2027-03-01 12:00:00" };

    const forbidden: [string, unknown][] = [
      ["an answer with a title change", { isCustomerApproved: true, title: "hijacked" }],
      ["a title change alone", { title: "hijacked" }],
      ["a state change", { state: "scheduled" }],
      ["a state change to rollback", { state: "rollback" }],
      ["a proposed window with a title change", { ...window, title: "hijacked" }],
      ["a proposed window with a state", { ...window, state: "authorize" }],
      ["a description change", { description: "hijacked" }],
    ];
    for (const [what, body] of forbidden) {
      const result = await dave.patch(approval.id, body);
      expect(result.status, `${what}: ${JSON.stringify(result.body)}`).toBe(403);
    }

    // The two mixed forms are told to be sent as separate requests / one answer at a time.
    const mixedProposal = await dave.patch(approval.id, { isCustomerApproved: true, ...window });
    expect(mixedProposal.status, JSON.stringify(mixedProposal.body)).toBe(400);
    const bothAnswers = await dave.patch(approval.id, { isCustomerApproved: true, isCustomerReviewed: true });
    expect(bothAnswers.status, JSON.stringify(bothAnswers.body)).toBe(400);

    // An answer for the wrong stage is a conflict, not an approval of this one.
    const wrongStage = await dave.patch(approval.id, { isCustomerReviewed: true });
    expect(wrongStage.status, JSON.stringify(wrongStage.body)).toBe(409);

    // Nothing moved.
    const row = await changeRequestRow(approval.id);
    expect([row.state, row.startUtc, row.endUtc, row.title]).toEqual([
      "CUSTOMER_APPROVAL",
      "",
      "",
      "E2E fixture: change in Customer Approval with a pending customer group approval",
    ]);
    expect(
      (await approverRows(approval.id)).map((r) => `${r.email}|${r.status}`),
      "no request was answered or cancelled",
    ).toEqual(["dave.mendis@example.com|requested", "erin.jayawardena@example.com|requested"]);
    const after = await dave.get(approval.id);
    expect(after.body.state?.label).toBe("Customer Approval");
    expect(after.body.customerCanAnswer, "dave still has his answer to give").toBe(true);
  });
});
