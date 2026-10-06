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
// A customer PROPOSES A NEW IMPLEMENTATION TIME in the customer portal, on the real
// local stack, and the change goes round the loop and comes back.
//
// The process (entity-service/CLAUDE.md, "Propose new implementation time = Re-schedule"):
// a contact who is asked at Customer Approval proposes a whole window (start AND end).
// For a Normal change that is a Re-schedule: the customer's pending request is
// cancelled, the change goes back to Authorize for a fresh CAB approval, and only when
// WSO2's CAB has approved the new time is the customer asked again, with a fresh request.
//
// What the tests prove, with who acts as who:
//   1. The dialog collects a start AND an end, and an empty, a past or an inverted window
//      shows inline errors and sends NOTHING (no PATCH leaves the browser).
//   2. THE LOOP, for CHG-FIXED-007 (a Normal change):
//        dave proposes a window -> the success copy promises a second approval -> the
//        change leaves Customer Approval for Authorize and the buttons go, for dave and
//        for erin -> the database holds the window in UTC and the customers' rows
//        cancelled, a CAB stage requested -> alice (CAB) approves through the CSM
//        portal's backend -> the change is back in Customer Approval, dave AND erin are
//        asked again with fresh rows, and their pages offer Approve / Reject / Propose
//        again -> the dialog now shows the window just proposed (the local wall time
//        round-trips through UTC) -> erin approves it and the change is Scheduled.
//
// ⚠️ STATE-CHANGING (re-seeds the stack's Postgres first, so it needs
// E2E_POSTGRES_CONTAINER, and for the loop E2E_CSM_BFF_URL; it SKIPS without them).
//

import { test, expect } from "../../fixtures/test";
import { LOCAL_PERSONAS, openLocalContext, withLocalSession } from "../../auth/localSessions";
import { ChangeRequestDetailsPage } from "../../pages/ChangeRequestDetailsPage";
import {
  FIXTURES,
  STAFF_APPROVERS,
  approverRows,
  changeRequestRow,
  customerApi,
  decideAsStaff,
  futureWindow,
  patchAsStaff,
  psql,
  resetFixtures,
  withFixtureStack,
} from "../../utils/localStack";
import { CHANGE_REQUEST_DETAILS as UI } from "../../utils/selectors";

withLocalSession(test, "dave");
withFixtureStack(test, true);

const { approval, standardNew, projectId } = FIXTURES;

/**
 * The browser's time zone, pinned to one that is neither UTC nor the machine's, so
 * the conversion of a proposed wall time to the UTC the backend stores is really
 * exercised. (The dialog reads the signed-in user's zone when the profile has one and
 * the browser's otherwise: the tests read the zone the dialog says and compute from it.)
 */
const BROWSER_ZONE = "America/New_York";
test.use({ timezoneId: BROWSER_ZONE });

test.describe("Local stack — a customer proposes a new implementation time", () => {
  test.describe.configure({ timeout: 240_000 });

  test.beforeEach(async () => {
    await resetFixtures();
  });

  test(`Propose New Time collects a start and an end; an empty, a past and an inverted window show inline errors and send nothing`, async ({
    page,
  }) => {
    const patches: string[] = [];
    page.on("request", (request) => {
      if (request.method() === "PATCH") patches.push(request.url());
    });

    const dave = new ChangeRequestDetailsPage(page);
    await dave.open(projectId, approval.id, approval.number);
    await dave.button(UI.buttons.proposeNewTime).click();

    const dialog = dave.proposeDialog();
    await expect(dialog).toBeVisible();
    // Both ends are asked for, and the Normal change says what happens to the proposal.
    await expect(dave.proposedStart()).toBeVisible();
    await expect(dave.proposedEnd()).toBeVisible();
    await expect(dialog).toContainText(UI.propose.noticeNormal);
    // The seeded fixture has no planned window, so there is nothing to prefill.
    await expect(dave.proposedStart()).toHaveValue("");
    await expect(dave.proposedEnd()).toHaveValue("");

    const zone = await dave.proposeTimeZone();
    const past = futureWindow(zone, { daysAhead: -2, startHour: 10, hours: 2 });
    const later = futureWindow(zone, { daysAhead: 4, startHour: 16, hours: 4 });

    // 1. Nothing typed: both fields are required.
    await dave.submitProposalButton().click();
    await expect(dialog.getByText(UI.propose.errors.startRequired)).toBeVisible();
    await expect(dialog.getByText(UI.propose.errors.endRequired)).toBeVisible();

    // 2. A window in the past: the start is refused (the end, after the start, is fine).
    await dave.fillProposedWindow(past.start, past.end);
    await expect(dialog.getByText(UI.propose.errors.startPast)).toBeVisible();
    await expect(dialog.getByText(UI.propose.errors.startRequired)).toBeHidden();
    await dave.submitProposalButton().click();
    await expect(dialog.getByText(UI.propose.errors.startPast)).toBeVisible();

    // 3. An inverted window: a good start, an end before it.
    await dave.fillProposedWindow(later.start, `${later.start.slice(0, 10)}T10:00`);
    await expect(dialog.getByText(UI.propose.errors.endNotAfterStart)).toBeVisible();
    await expect(dialog.getByText(UI.propose.errors.startPast)).toBeHidden();
    await dave.submitProposalButton().click();
    await expect(dialog.getByText(UI.propose.errors.endNotAfterStart)).toBeVisible();

    // The dialog stayed open and nothing was sent: the change still waits, unchanged.
    await expect(dialog).toBeVisible();
    expect(patches, "an invalid window must not leave the browser").toEqual([]);
    const api = await customerApi("dave").get(approval.id);
    expect(api.body.state?.label).toBe("Customer Approval");
    expect(api.body.customerCanAnswer).toBe(true);
    const row = await changeRequestRow(approval.id);
    expect([row.state, row.startUtc, row.endUtc]).toEqual(["CUSTOMER_APPROVAL", "", ""]);

    // Cancel closes it, still without a request.
    await dialog.getByRole("button", { name: UI.propose.cancel, exact: true }).click();
    await expect(dialog).toBeHidden();
    expect(patches).toEqual([]);
  });

  test(`${LOCAL_PERSONAS.dave.email} proposes a time for ${approval.number}: Authorize, buttons gone for dave and erin; after CAB approves, both are asked again and erin approves the new time`, async ({
    page,
    browser,
    baseURL,
  }) => {
    const erinContext = await openLocalContext(test, browser, "erin", {
      baseURL,
      timezoneId: BROWSER_ZONE,
    });
    try {
      const dave = new ChangeRequestDetailsPage(page);
      const erinPage = await erinContext.newPage();
      const erin = new ChangeRequestDetailsPage(erinPage);
      await dave.open(projectId, approval.id, approval.number);
      await erin.open(projectId, approval.id, approval.number);
      await expect(erin.button(UI.buttons.proposeNewTime)).toBeVisible();

      // dave proposes the whole window, in the zone the dialog says it reads.
      await dave.button(UI.buttons.proposeNewTime).click();
      await expect(dave.proposeDialog()).toBeVisible();
      const zone = await dave.proposeTimeZone();
      test.info().annotations.push({ type: "time zone read by the dialog", description: zone });
      const window = futureWindow(zone, { daysAhead: 3, startHour: 16, hours: 4 });
      await dave.fillProposedWindow(window.start, window.end);
      await dave.submitProposalButton().click();

      // The copy tells him this is not an approval: WSO2 reviews first.
      await expect(dave.banner(UI.banners.proposedNormal)).toBeVisible();
      await expect(dave.proposeDialog()).toBeHidden();

      // The change left Customer Approval: Authorize, and nothing to answer.
      await expect(dave.currentStage()).toHaveText(UI.stages.authorize);
      await expect(dave.answerButtons()).toHaveCount(0);

      // What the stack holds: the window as UTC, the customers' request cancelled, CAB asked.
      const after = await changeRequestRow(approval.id);
      expect(after.state).toBe("AUTHORIZE");
      expect(after.startUtc, "proposed start, as UTC").toBe(window.startUtc);
      expect(after.endUtc, "proposed end, as UTC").toBe(window.endUtc);
      const daveApi = await customerApi("dave").get(approval.id);
      expect(daveApi.body.state?.label?.toLowerCase()).toBe("authorize");
      expect(daveApi.body.customerCanAnswer).toBe(false);
      const loop1 = await approverRows(approval.id);
      expect(
        loop1.filter((r) => r.stage === "Customer Approval").map((r) => `${r.email}|${r.status}`),
        "the customers' pending request is cancelled, not answered",
      ).toEqual(["dave.mendis@example.com|cancelled", "erin.jayawardena@example.com|cancelled"]);
      const cab = loop1.filter((r) => r.stage === "CAB Approval");
      expect(cab.map((r) => r.status), "CAB asked").toEqual(["requested", "requested", "requested"]);
      expect(cab.map((r) => r.email)).toContain(STAFF_APPROVERS.alice);

      // erin, who did not propose, is not offered anything either.
      await erin.open(projectId, approval.id, approval.number);
      await expect(erin.currentStage()).toHaveText(UI.stages.authorize);
      await expect(erin.answerButtons()).toHaveCount(0);

      // WSO2's CAB approves the new time (alice, through the CSM portal's backend).
      const decided = await decideAsStaff(STAFF_APPROVERS.alice, approval.id, "approved");
      expect(decided.status, JSON.stringify(decided.body)).toBe(200);

      // Back in Customer Approval: dave and erin are asked again, with FRESH requests.
      await dave.open(projectId, approval.id, approval.number);
      await expect(dave.currentStage()).toHaveText(UI.stages.customerApproval);
      for (const name of [UI.buttons.proposeNewTime, UI.buttons.approve, UI.buttons.reject]) {
        await expect(dave.button(name), `${name} is back for dave`).toBeVisible();
      }
      await erin.open(projectId, approval.id, approval.number);
      await expect(erin.currentStage()).toHaveText(UI.stages.customerApproval);
      for (const name of [UI.buttons.proposeNewTime, UI.buttons.approve, UI.buttons.reject]) {
        await expect(erin.button(name), `${name} is back for erin`).toBeVisible();
      }
      const loop2 = await approverRows(approval.id);
      const customerStages = loop2.filter((r) => r.stage === "Customer Approval");
      expect(
        customerStages.map((r) => `${r.email}|${r.status}`),
        "the old request stays as a cancelled record; a new one is requested",
      ).toEqual([
        "dave.mendis@example.com|cancelled",
        "erin.jayawardena@example.com|cancelled",
        "dave.mendis@example.com|requested",
        "erin.jayawardena@example.com|requested",
      ]);
      expect((await changeRequestRow(approval.id)).state).toBe("CUSTOMER_APPROVAL");
      expect((await customerApi("erin").get(approval.id)).body.customerCanAnswer).toBe(true);

      // The dialog now shows the window just proposed: the wall time round-trips through UTC.
      await erin.button(UI.buttons.proposeNewTime).click();
      await expect(erin.proposedStart()).toHaveValue(window.start);
      await expect(erin.proposedEnd()).toHaveValue(window.end);

      // Proposing the very window that is already there is refused in the dialog, before any request.
      const patches: string[] = [];
      erinPage.on("request", (request) => {
        if (request.method() === "PATCH") patches.push(request.url());
      });
      await erin.submitProposalButton().click();
      await expect(erin.proposeDialog().getByText(UI.propose.errors.unchanged)).toBeVisible();
      expect(patches, "an unchanged window must not leave the browser").toEqual([]);
      await erin.proposeDialog().getByRole("button", { name: UI.propose.cancel, exact: true }).click();
      await expect(erin.proposeDialog()).toBeHidden();

      // erin approves the new time: Scheduled.
      await erin.button(UI.buttons.approve).click();
      await expect(erin.banner(UI.banners.approved)).toBeVisible();
      await expect(erin.currentStage()).toHaveText(UI.stages.scheduled);
      const done = await changeRequestRow(approval.id);
      expect([done.state, done.startUtc, done.endUtc]).toEqual([
        "SCHEDULED",
        window.startUtc,
        window.endUtc,
      ]);
    } finally {
      await erinContext.close();
    }
  });

  test(`${LOCAL_PERSONAS.dave.email} proposes a time for a STANDARD change (${standardNew.number}): it stays in Customer Approval and both contacts are asked again at once`, async ({
    page,
  }) => {
    // Request Approval on a Standard change with Customer Approval ticked lands it
    // straight in Customer Approval (no internal approval to go through).
    const requested = await patchAsStaff(STAFF_APPROVERS.alice, standardNew.id, { state: "assess" });
    expect(requested.status, JSON.stringify(requested.body)).toBe(200);

    const dave = new ChangeRequestDetailsPage(page);
    await dave.open(projectId, standardNew.id, standardNew.number);
    await expect(dave.currentStage()).toHaveText(UI.stages.customerApproval);
    await dave.button(UI.buttons.proposeNewTime).click();

    // A Standard change says something different: there is no internal review to wait for.
    await expect(dave.proposeDialog()).toContainText(UI.propose.noticeStandard);
    const zone = await dave.proposeTimeZone();
    const window = futureWindow(zone, { daysAhead: 5, startHour: 9, hours: 3 });
    await dave.fillProposedWindow(window.start, window.end);
    await dave.submitProposalButton().click();

    await expect(dave.banner(UI.banners.proposedStandard)).toBeVisible();
    await expect(dave.proposeDialog()).toBeHidden();

    // Still Customer Approval, and dave is asked again: the buttons did not go.
    await expect(dave.currentStage()).toHaveText(UI.stages.customerApproval);
    for (const name of [UI.buttons.proposeNewTime, UI.buttons.approve, UI.buttons.reject]) {
      await expect(dave.button(name), `${name} stays for dave`).toBeVisible();
    }
    const row = await changeRequestRow(standardNew.id);
    expect([row.state, row.startUtc, row.endUtc]).toEqual([
      "CUSTOMER_APPROVAL",
      window.startUtc,
      window.endUtc,
    ]);
    expect(
      (await approverRows(standardNew.id)).map((r) => `${r.stage}|${r.email}|${r.status}`),
      "the first request is cancelled, a fresh one asked of both",
    ).toEqual([
      "Customer Approval|dave.mendis@example.com|cancelled",
      "Customer Approval|erin.jayawardena@example.com|cancelled",
      "Customer Approval|dave.mendis@example.com|requested",
      "Customer Approval|erin.jayawardena@example.com|requested",
    ]);
    expect((await customerApi("dave").get(standardNew.id)).body.customerCanAnswer).toBe(true);

    // Approving the new time schedules it.
    await dave.button(UI.buttons.approve).click();
    await expect(dave.banner(UI.banners.approved)).toBeVisible();
    await expect(dave.currentStage()).toHaveText(UI.stages.scheduled);
  });

  test(`${approval.number} on hold: a proposal is refused in the dialog with the reason, and Approve is still taken`, async ({
    page,
  }) => {
    // WSO2 put the change on hold. A held change refuses a proposed time (409) but not an answer.
    await psql(
      `update change_request set is_on_hold = true, on_hold_reason = 'E2E hold' where id = '${approval.id}'`,
    );

    const dave = new ChangeRequestDetailsPage(page);
    await dave.open(projectId, approval.id, approval.number);
    await dave.button(UI.buttons.proposeNewTime).click();
    const zone = await dave.proposeTimeZone();
    const window = futureWindow(zone, { daysAhead: 6, startHour: 11, hours: 2 });
    await dave.fillProposedWindow(window.start, window.end);
    await dave.submitProposalButton().click();

    // The dialog stays open, so the customer can read why; nothing changed.
    await expect(dave.proposeDialog().getByRole("alert").filter({ hasText: UI.propose.errors.onHold })).toBeVisible();
    await expect(dave.proposeDialog()).toBeVisible();
    const row = await changeRequestRow(approval.id);
    expect([row.state, row.startUtc, row.endUtc]).toEqual(["CUSTOMER_APPROVAL", "", ""]);
    expect((await customerApi("dave").get(approval.id)).body.customerCanAnswer).toBe(true);

    await dave.proposeDialog().getByRole("button", { name: UI.propose.cancel, exact: true }).click();
    await expect(dave.proposeDialog()).toBeHidden();

    // Approve is still taken while the change is on hold.
    await dave.button(UI.buttons.approve).click();
    await expect(dave.banner(UI.banners.approved)).toBeVisible();
    await expect(dave.currentStage()).toHaveText(UI.stages.scheduled);
  });

  test(`the loop can be repeated: dave moves the window again after CAB approved the first, the start drags the end along, and the history keeps every round`, async ({
    page,
    browser,
    baseURL,
  }) => {
    const erinContext = await openLocalContext(test, browser, "erin", {
      baseURL,
      timezoneId: BROWSER_ZONE,
    });
    try {
      const dave = new ChangeRequestDetailsPage(page);
      const erin = new ChangeRequestDetailsPage(await erinContext.newPage());
      await dave.open(projectId, approval.id, approval.number);

      // Round one: dave proposes, alice (CAB) approves.
      await dave.button(UI.buttons.proposeNewTime).click();
      const zone = await dave.proposeTimeZone();
      const first = futureWindow(zone, { daysAhead: 3, startHour: 16, hours: 4 });
      await dave.fillProposedWindow(first.start, first.end);
      await dave.submitProposalButton().click();
      await expect(dave.banner(UI.banners.proposedNormal)).toBeVisible();
      await expect(dave.currentStage()).toHaveText(UI.stages.authorize);
      expect((await decideAsStaff(STAFF_APPROVERS.alice, approval.id, "approved")).status).toBe(200);

      // Round two: back in Customer Approval, dave changes his mind. The dialog starts from the
      // window on the change; moving the START moves the END with it, keeping the four hours.
      await dave.open(projectId, approval.id, approval.number);
      await expect(dave.currentStage()).toHaveText(UI.stages.customerApproval);
      await dave.button(UI.buttons.proposeNewTime).click();
      await expect(dave.proposedStart()).toHaveValue(first.start);
      await expect(dave.proposedEnd()).toHaveValue(first.end);
      const second = futureWindow(zone, { daysAhead: 5, startHour: 9, hours: 4 });
      await dave.proposedStart().fill(second.start);
      await expect(dave.proposedEnd(), "the end follows the start, same duration").toHaveValue(second.end);
      await dave.submitProposalButton().click();
      await expect(dave.banner(UI.banners.proposedNormal)).toBeVisible();
      await expect(dave.currentStage()).toHaveText(UI.stages.authorize);
      await expect(dave.answerButtons()).toHaveCount(0);
      const mid = await changeRequestRow(approval.id);
      expect([mid.state, mid.startUtc, mid.endUtc]).toEqual(["AUTHORIZE", second.startUtc, second.endUtc]);

      // A second CAB round, decided by bob this time: back to Customer Approval, erin approves.
      expect((await decideAsStaff(STAFF_APPROVERS.bob, approval.id, "approved")).status).toBe(200);
      await erin.open(projectId, approval.id, approval.number);
      await expect(erin.currentStage()).toHaveText(UI.stages.customerApproval);
      await erin.button(UI.buttons.approve).click();
      await expect(erin.banner(UI.banners.approved)).toBeVisible();
      await expect(erin.currentStage()).toHaveText(UI.stages.scheduled);

      const done = await changeRequestRow(approval.id);
      expect([done.state, done.startUtc, done.endUtc]).toEqual(["SCHEDULED", second.startUtc, second.endUtc]);
      expect(
        (await approverRows(approval.id)).map((r) => `${r.stage}|${r.email.split("@")[0]}|${r.status}`),
        "every round stays on record: the customers' cancelled requests, each CAB round, the final answer",
      ).toEqual([
        "Customer Approval|dave.mendis|cancelled",
        "Customer Approval|erin.jayawardena|cancelled",
        "CAB Approval|alice.perera|approved",
        "CAB Approval|bob.fernando|cancelled",
        "CAB Approval|carol.silva|cancelled",
        "Customer Approval|dave.mendis|cancelled",
        "Customer Approval|erin.jayawardena|cancelled",
        "CAB Approval|alice.perera|cancelled",
        "CAB Approval|bob.fernando|approved",
        "CAB Approval|carol.silva|cancelled",
        "Customer Approval|dave.mendis|cancelled",
        "Customer Approval|erin.jayawardena|approved",
      ]);
    } finally {
      await erinContext.close();
    }
  });
});
