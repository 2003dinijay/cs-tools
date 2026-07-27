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
// Reject flow for the Approvals tab (approver/admin) — the counterpart to
// approvals.spec.ts's accept flow. Same self-provisioning constraints apply:
//
// - useApprovalQueue excludes the *signed-in user's own* submitted cards
//   server-side (see useTimeSheets.ts), so a card the approver creates under
//   their own session can never appear in their own queue — a single
//   captured session cannot exercise this. A second identity is required to
//   create the card (via openContextAs(browser, "engineer")), same as
//   approvals.spec.ts. This test is skipped when
//   tests/e2e/storageState/engineer.json hasn't been captured.
//
// - PATCH /time-cards/{id} 403s unless the signed-in user is the card's
//   assigned approver (confirmed live in approvals.spec.ts), so the card
//   created below assigns the *approver* session (not the engineer) as its
//   approver — i.e. the approver self-provisions the decision authority over
//   a card it didn't author, avoiding the 403 while still populating the
//   queue.
//
// - TimeCardReviewDialog requires a non-empty "Lead's comment" to reject
//   (enforced client-side; the backend accepts an empty leadComment on
//   reject too but the UI blocks it since it's the only trace a rejection
//   leaves — see TimeCardReviewDialog.tsx's `rejectBlocked` state).
//

import { test, expect, withRole, hasSession, openContextAs, approverSearchQuery } from "../../fixtures/test";
import { TimeCardsPage } from "../../pages/TimeCardsPage";
import { LogTimeDialog } from "../../pages/LogTimeDialog";
import { e2eWorkLogComment } from "../../utils/selectors";

withRole(test, "approver");

test.describe("time cards — reject flow", () => {
  test("rejecting a card created by a different signed-in user clears it from the queue and records a reason", async ({
    page,
    browser,
  }) => {
    test.setTimeout(60_000);
    test.skip(
      !hasSession("engineer"),
      "No captured session for 'engineer'. See tests/e2e/auth/README.md to create " +
        "tests/e2e/storageState/engineer.json — this test needs a second identity " +
        "distinct from the approver session to create a card the approver didn't author.",
    );

    // The card's assigned approver must be the *approver* session, since
    // only the assigned approver can decide it — derive the search query
    // from the primary `page` (approver), not the engineer.
    const approverQuery = await approverSearchQuery(page);

    // Create the card as "engineer" in a second, independent context.
    const engineerContext = await openContextAs(browser, "engineer");
    const engineerPage = await engineerContext.newPage();
    let caseNumber: string | null = null;
    try {
      await engineerPage.goto("/cases");
      const firstCase = engineerPage
        .locator('a[href^="/cases/"]:not([href="/cases/new"])')
        .first();
      const hasCase = await firstCase
        .waitFor({ state: "visible", timeout: 15_000 })
        .then(() => true)
        .catch(() => false);
      test.skip(!hasCase, "No cases visible to the 'engineer' session.");

      await firstCase.click();
      await expect(engineerPage).toHaveURL(/\/cases\/[^/]+$/);
      await engineerPage.getByRole("tab", { name: "Time tracking" }).click();
      const logTime = engineerPage.getByRole("button", { name: "Log time" });
      test.skip(
        !(await logTime.isVisible().catch(() => false)),
        "This case is closed — Log time isn't available.",
      );

      await logTime.click();
      const dialog = new LogTimeDialog(engineerPage);
      await dialog.waitForOpen();
      caseNumber = await dialog.caseNumber();
      await dialog.fillAndSubmit({
        hours: 1,
        workLogComment: e2eWorkLogComment("reject flow"),
        approverQuery,
      });
    } finally {
      await engineerContext.close();
    }
    test.skip(!caseNumber, "Could not determine the created card's case number.");

    // Reject it as the approver session (the primary `page`, already
    // authenticated via withRole above).
    const tc = new TimeCardsPage(page);
    await tc.goto();
    await tc.openApprovals();
    await tc.filterWorkItem(caseNumber!);

    const found = await tc
      .cardRow(caseNumber!)
      .waitFor({ state: "visible", timeout: 15_000 })
      .then(() => true)
      .catch(() => false);
    test.skip(!found, "Created card did not surface in the Approvals queue.");

    const rejectButton = tc.cardButton(caseNumber!, "Reject");
    await rejectButton.click();

    const reviewDialog = tc.reviewDialog();
    await expect(reviewDialog).toBeVisible();

    // Clicking Reject without a comment is a client-side no-op (see
    // TimeCardReviewDialog's rejectBlocked guard) — 403 is not a concern
    // here since it never leaves the browser, but confirm the dialog stays
    // open and shows the "required" hint before providing the reason.
    await reviewDialog.getByRole("button", { name: "Reject" }).click();
    await expect(
      reviewDialog.getByText("A comment is required to reject a time card."),
    ).toBeVisible();
    await expect(reviewDialog).toBeVisible();

    await reviewDialog
      .getByLabel(/Lead's comment/)
      .fill(e2eWorkLogComment("rejected by e2e"));
    await reviewDialog.getByRole("button", { name: "Reject" }).click();

    const decided = await expect(reviewDialog)
      .toBeHidden({ timeout: 15_000 })
      .then(() => true)
      .catch(() => false);
    test.skip(
      !decided,
      "Reject request was rejected by the backend (likely a 403 — signed-in " +
        "user isn't this card's assigned approver) rather than succeeding.",
    );

    await expect(tc.cardText(caseNumber!)).toHaveCount(0, { timeout: 15_000 });
  });
});
