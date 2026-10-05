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
// Deterministic Change Request lifecycle coverage — the compulsory-
// assigned-team gate, Assess-entry approver auto-provisioning, the
// approve/cancel-sibling cascade from Assess to Authorize, and a terminal
// approval display — run against the four fixed-UUID fixtures in
// scripts/csm-compose/seed-entity-service.sql (CR-FIXED-001..004), not a
// freshly self-provisioned CR the way change-request-detail.spec.ts works.
//
// That's a deliberate, necessary difference, not a style choice:
// CreateChangeRequest always 503s against this stack's own Postgres data
// source (work_item.number has no DB sequence — see entity-service's own
// CLAUDE.md, "CreateCase and case numbers"/"Change requests"), so there is
// no "create one, then drive it" path available locally at all. The fixed
// fixtures exist specifically to give this spec something to navigate
// straight to by id.
//
// The flip side of a fixed fixture: CR-FIXED-002/003 each get moved forward
// by exactly the transition this spec exercises (New -> Assess, and an
// approval decision), and that move is NOT reset by re-running the seed
// file — `ON CONFLICT (id) DO NOTHING` only ever applies on first insert, so
// a mutated row stays mutated. Unlike staging (where change-request-detail's
// self-provisioned CRs are simply abandoned, never reused), this spec is
// meant to run before every local push, so its own fixtures have to come
// back to their starting state every time. resetFixtures() below does that
// with plain, idempotent UPDATEs against the already-running local
// docker-compose Postgres (`csm-platform-postgres-1`) before anything else
// runs — CR-FIXED-001/004 are read-only fixtures (nothing here ever mutates
// them) and need no reset.
//
// Runs only against the local stack (E2E_NO_WEBSERVER=1, see
// package.json's "test:e2e:cr-lifecycle") signed in as jane.doe@example.com
// — the approver CR-FIXED-003/004 are seeded with (see
// tests/e2e/auth/generate-session.spec.ts for how that session is minted
// with no human step, as the "crApprover" role).
//

import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { test, expect, withRole } from "../../fixtures/test";
import { ChangeRequestCreatePage } from "../../pages/ChangeRequestCreatePage";
import { ChangeRequestDetailPage } from "../../pages/ChangeRequestDetailPage";
import {
  FAKE_CAB,
  FAKE_CR_ID,
  FAKE_CREATOR,
  FAKE_DEPLOYMENTS,
  FAKE_DEPLOYMENT_PRODUCTS,
  FAKE_ECAB,
  FAKE_BETA_CONTACT,
  FAKE_CUST_ONE,
  FAKE_CUST_TWO,
  FAKE_OUTSIDER,
  FAKE_PEER,
  FAKE_PROJECT_CONTACTS,
  FAKE_PROJECTS,
  installFakeChangeRequestApi,
  type FakeChangeRequestApi,
  type FakeUser,
} from "../../utils/fakeChangeRequestApi";

const execFileAsync = promisify(execFile);

const CR_NO_TEAM = "00000000-0000-0000-0000-000000001001";
const CR_WITH_TEAM = "00000000-0000-0000-0000-000000001002";
const CR_PENDING_APPROVAL = "00000000-0000-0000-0000-000000001003";
const CR_RESOLVED = "00000000-0000-0000-0000-000000001004";

const APOLLO_GROUP_ID = "00000000-0000-0000-0000-000000000901"; // "Example Corp ABT" — see seed-entity-service.sql
const STAGE_ID = "00000000-0000-0000-0000-000000001005";
const JANE_APPROVER_ROW = "00000000-0000-0000-0000-000000001006";
const JOHN_APPROVER_ROW = "00000000-0000-0000-0000-000000001007";

/** Restores CR-FIXED-002/003 to the exact starting state documented in
 * seed-entity-service.sql, unconditionally, via the already-running local
 * docker-compose Postgres container — see this file's own top comment for
 * why a fixed fixture needs this instead of the idempotent seed file's own
 * `ON CONFLICT DO NOTHING` inserts. Plain UPDATEs, not re-running the seed
 * file, since the rows already exist after the first ever seed. */
async function resetFixtures(): Promise<void> {
  const sql = `
    UPDATE change_request SET state = 'NEW'::change_request_state_enum, requested_by_user_id = NULL WHERE id = '${CR_WITH_TEAM}';
    UPDATE work_item SET assignment_group_id = '${APOLLO_GROUP_ID}' WHERE id = '${CR_WITH_TEAM}';
    -- Also clears the CAB/ECAB Approval stages the approval flow adds.
    DELETE FROM approval_stage_approver WHERE work_item_id = '${CR_WITH_TEAM}';
    DELETE FROM approval_stage WHERE work_item_id = '${CR_WITH_TEAM}';

    -- requested_by_user_id stays NULL here (not jane.doe/john.smith, both
    -- team members): PatchChangeRequest now auto-cancels the change's own
    -- requester instead of leaving them Requested (mirrors real ServiceNow,
    -- confirmed live against CHG0039122 — see entity-service's own CLAUDE.md).
    -- Either seeded user as requester would turn this fixture's "both
    -- members become pending approvers" demonstration into a demonstration
    -- of that unrelated exclusion instead.
    UPDATE change_request SET state = 'ASSESS'::change_request_state_enum, requested_by_user_id = NULL WHERE id = '${CR_PENDING_APPROVAL}';
    -- The "approve cascades to Authorize" test below approves Jane's row,
    -- which (since entity-service also auto-provisions the CAB Approval
    -- stage now, not just Peer Approval) creates a SECOND approval_stage + a fresh pair
    -- of approver rows for this same work item, under new gen_random_uuid()
    -- ids neither upsert below ever matches. Left alone, those accumulate
    -- across runs -- the Approvals table ends up with two rows per approver,
    -- and Playwright's strict-mode approverRow("Jane Doe") then matches more
    -- than one and fails. Delete anything that isn't this fixture's own
    -- known seeded stage/approvers before re-seeding them.
    DELETE FROM approval_stage_approver WHERE work_item_id = '${CR_PENDING_APPROVAL}'
      AND id NOT IN ('${JANE_APPROVER_ROW}', '${JOHN_APPROVER_ROW}');
    DELETE FROM approval_stage WHERE work_item_id = '${CR_PENDING_APPROVAL}' AND id <> '${STAGE_ID}';
    INSERT INTO approval_stage (id, created_on, updated_on, created_by, updated_by, work_item_id, assignment_group_id, raw_status)
      VALUES ('${STAGE_ID}', now(), now(), 'seed', 'seed', '${CR_PENDING_APPROVAL}', '${APOLLO_GROUP_ID}', 'requested')
      ON CONFLICT (id) DO UPDATE SET raw_status = 'requested';
    INSERT INTO approval_stage_approver (id, created_on, updated_on, created_by, updated_by, stage_id, work_item_id, approver_user_id, status)
      VALUES
        ('${JANE_APPROVER_ROW}', now(), now(), 'seed', 'seed', '${STAGE_ID}', '${CR_PENDING_APPROVAL}', '00000000-0000-0000-0000-000000000001', 'requested'),
        ('${JOHN_APPROVER_ROW}', now(), now(), 'seed', 'seed', '${STAGE_ID}', '${CR_PENDING_APPROVAL}', '00000000-0000-0000-0000-000000000002', 'requested')
      ON CONFLICT (id) DO UPDATE SET status = 'requested';
  `;
  await execFileAsync("docker", [
    "exec",
    "-i",
    process.env.E2E_POSTGRES_CONTAINER ?? "csm-platform-postgres-1",
    "psql",
    "-U",
    "postgres",
    "-d",
    "csm_platform",
    "-v",
    "ON_ERROR_STOP=1",
    "-c",
    sql,
  ]);
}

withRole(test, "crApprover");

// The seeded-fixture describes below need the local docker-compose stack and
// reset its Postgres rows first; scoped to this wrapper so the approval-flow
// describes at the bottom of the file (which run against an in-browser fake of
// the change-request API, see utils/fakeChangeRequestApi.ts) don't need docker.
test.describe("seeded fixtures (local stack)", () => {
test.beforeAll(async () => {
  await resetFixtures();
});

test.describe("change request lifecycle — compulsory team gate", () => {
  test("Request Approval is disabled with no assigned team, and states why", async ({ page }) => {
    const detail = new ChangeRequestDetailPage(page);
    await detail.goto(CR_NO_TEAM);

    const blocked = page.getByLabel(/Request Approval: .*assigned team/i);
    await expect(blocked).toBeVisible();
    await expect(blocked.getByRole("button", { name: "Request Approval" })).toBeDisabled();
    await expect(detail.scheduleButton()).toHaveCount(0);
  });
});

test.describe("change request lifecycle — Assess-entry auto-provisioning", () => {
  test("Request Approval succeeds once a team is assigned, and provisions that team's members as approvers", async ({
    page,
  }) => {
    test.setTimeout(60_000);

    const detail = new ChangeRequestDetailPage(page);
    await detail.goto(CR_WITH_TEAM);

    const requestApprovalButton = detail.requestApprovalButton();
    await expect(requestApprovalButton).toBeEnabled();

    const [response] = await Promise.all([
      page.waitForResponse(
        (r) => new RegExp(`/change-requests/${CR_WITH_TEAM}$`).test(r.url()) && r.request().method() === "PATCH",
        { timeout: 15_000 },
      ),
      detail.requestApproval(),
    ]);
    expect(response.ok(), `Request Approval PATCH failed (${response.status()})`).toBeTruthy();

    await expect(requestApprovalButton).toBeHidden({ timeout: 15_000 });

    // Both of the assigned team's seeded members (Jane Doe, John Smith —
    // see scripts/csm-compose/seed-entity-service.sql) should now appear as
    // Requested approvers, with no manual provisioning step.
    await expect(detail.approverStatus("Jane Doe", "Peer Approval")).toHaveText("Requested");
    await expect(detail.approverStatus("John Smith", "Peer Approval")).toHaveText("Requested");
  });
});

test.describe("change request lifecycle — approve cascades to Authorize", () => {
  test("approving one of two pending approvers cascades the state to Authorize and cancels the other", async ({
    page,
  }) => {
    test.setTimeout(60_000);

    const detail = new ChangeRequestDetailPage(page);
    await detail.goto(CR_PENDING_APPROVAL);

    // Only the signed-in user's (Jane Doe's) own pending row renders an
    // Approve button — John Smith's sibling row has none. Rows are scoped to
    // the "Peer Approval" stage because, once Jane approves, the backend
    // adds a CAB Approval stage listing the same two seeded users.
    const PEER = "Peer Approval";
    await expect(detail.approverStatus("Jane Doe", PEER)).toHaveText("Requested");
    await expect(detail.approverStatus("John Smith", PEER)).toHaveText("Requested");
    await expect(detail.approveButton("John Smith", PEER)).toHaveCount(0);

    const approveButton = detail.approveButton("Jane Doe", PEER);
    await expect(approveButton).toBeVisible();

    const [response] = await Promise.all([
      page.waitForResponse((r) => /\/change-requests\/[^/]+\/approvals?/.test(r.url()), { timeout: 15_000 }),
      approveButton.click(),
    ]);
    expect(response.ok(), `Approve decision failed (${response.status()})`).toBeTruthy();

    await expect(detail.approverStatus("Jane Doe", PEER)).toHaveText("Approved");
    await expect(detail.approverStatus("John Smith", PEER)).toHaveText("Cancelled");

    // Peer approval cascades to the CAB Approval stage (its own group, the
    // next stage), the CR is in Authorize, and there is no Schedule button.
    await expect(detail.currentStep()).toContainText("Authorize");
    await expect(detail.blockingReason()).toHaveText("Awaiting CAB Approval");
    await expect(detail.approverStatus("Jane Doe", "CAB Approval")).toHaveText("Requested");
    await expect(detail.scheduleButton()).toHaveCount(0);
  });
});

test.describe("change request lifecycle — terminal approval display", () => {
  test("an already-decided change request shows its resolved approval state with no pending actions", async ({
    page,
  }) => {
    const detail = new ChangeRequestDetailPage(page);
    await detail.goto(CR_RESOLVED);

    await expect(detail.approverStatus("Jane Doe", "Peer Approval")).toHaveText("Approved");
    await expect(detail.approverStatus("John Smith", "Peer Approval")).toHaveText("Cancelled");
    await expect(detail.approveButton()).toHaveCount(0);
    await expect(detail.rejectButton()).toHaveCount(0);
  });
});
});

//
// Approval-flow lifecycle per change type, against the in-browser fake in
// utils/fakeChangeRequestApi.ts (no records created, no seeded fixtures, no
// docker). Visible state is asserted after every step:
//
//   Normal    New -> Request Approval -> Assess [Peer Approval]
//                 -> Authorize [CAB Approval] -> (auto) Scheduled
//                 -> Implement -> Review -> Closed
//   Emergency New -> Request Approval -> Authorize [ECAB Approval only]
//                 -> (auto) Scheduled
//   Standard  New -> Request Approval -> (auto) Scheduled, no approvals
//
// With "Customer Approval" ticked, every route above stops at Customer
// Approval before Scheduled until "Record customer approval" is clicked; with
// "Customer Review" ticked, Review offers "Send for customer review" instead
// of "Close", then Customer Review offers Close.
//
// Also asserts at every step that there is no "Schedule" button and no
// "Move to Assess" label, and that the CR's creator can Cancel but never
// Approve/Reject. Each "switch user" is a page reload with the faked
// `/users/me` identity changed. The browser is still signed in with the
// captured session so the portal boots normally.
//

async function openDetail(detail: ChangeRequestDetailPage): Promise<void> {
  await detail.goto(FAKE_CR_ID);
}

async function expectNoManualSchedule(detail: ChangeRequestDetailPage): Promise<void> {
  await expect(detail.scheduleButton()).toHaveCount(0);
  await expect(detail.page.getByText(/move to assess/i)).toHaveCount(0);
}

/** Customer Approval / Customer Review appear on the stepper only when ticked. */
async function expectCustomerStepsOnLine(
  detail: ChangeRequestDetailPage,
  flags: { approval: boolean; review: boolean },
): Promise<void> {
  const expected = ["New", "Assess", "Authorize"];
  if (flags.approval) expected.push("Customer Approval");
  expected.push("Scheduled", "Implement", "Review");
  if (flags.review) expected.push("Customer Review");
  expected.push("Closed");
  await expect(detail.stepLabels()).toHaveText(expected);
}

test.describe("change request approval flow — Normal", () => {
  for (const approval of [false, true]) {
    for (const review of [false, true]) {
      test(`Normal, customer approval ${approval ? "on" : "off"}, customer review ${review ? "on" : "off"}: every step shows the right state and actions`, async ({
        page,
      }) => {
        test.setTimeout(120_000);
        const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, {
          customerApprovalRequired: approval,
          customerReviewRequired: review,
        });
        const detail = new ChangeRequestDetailPage(page);

        // New: the creator requests approval. The flags are shown read-only.
        await openDetail(detail);
        await expect(detail.currentStep()).toContainText("New");
        await expect(detail.flagValue("Customer approval required")).toHaveText(approval ? "Yes" : "No");
        await expect(detail.flagValue("Customer review required")).toHaveText(review ? "Yes" : "No");
        await expectCustomerStepsOnLine(detail, { approval, review });
        await expectNoManualSchedule(detail);
        await detail.requestApproval();

        // Peer Approval is pending; the creator can't decide but can still cancel.
        await expect(detail.currentStep()).toContainText("Assess");
        await expect(detail.blockingReason()).toHaveText("Awaiting Peer Approval");
        await expect(detail.approverStage("Pat Peer")).toHaveText("Peer Approval");
        await expect(detail.approveButton()).toHaveCount(0);
        await expect(detail.rejectButton()).toHaveCount(0);
        await expect(detail.creatorApprovalNotice()).toBeVisible();
        await detail.changeStateButton().click();
        await expect(detail.cancelChangeMenuItem()).toBeEnabled();
        await page.keyboard.press("Escape");
        await expectNoManualSchedule(detail);

        // A peer approves; CAB Approval is the next, separate stage.
        api.setViewer(FAKE_PEER);
        await page.reload();
        await expect(detail.approveButton("Pat Peer")).toBeVisible();
        await detail.approve("Pat Peer");
        await expect(detail.currentStep()).toContainText("Authorize");
        await expect(detail.blockingReason()).toHaveText("Awaiting CAB Approval");
        await expect(detail.approverStage("Cam Cab")).toHaveText("CAB Approval");
        await expect(detail.approverStage("Pat Peer")).toHaveText("Peer Approval");
        await expect(detail.approverStatus("Pat Peer")).toHaveText("Approved");
        await expectNoManualSchedule(detail);

        // A CAB member approves; the page refreshes itself to Customer
        // Approval when that box is ticked, else straight to Scheduled.
        api.setViewer(FAKE_CAB);
        await page.reload();
        await detail.approve("Cam Cab");
        expect(api.requests().some((r) => r === `POST /change-requests/${FAKE_CR_ID}/approvals/decision`)).toBe(true);

        api.setViewer(FAKE_CREATOR);
        await page.reload();
        if (approval) {
          await expect(detail.currentStep()).toContainText("Customer Approval");
          await expect(detail.blockingReason()).toHaveText("Awaiting Customer Approval");
          await expect(page.getByRole("button", { name: "Start implementation" })).toHaveCount(0);
          await expect(detail.recordCustomerApprovalButton()).toBeVisible();
          await detail.changeStateButton().click();
          await expect(detail.cancelChangeMenuItem()).toBeEnabled();
          await page.keyboard.press("Escape");
          await expectNoManualSchedule(detail);
          await detail.recordCustomerApproval();
        } else {
          await expect(detail.recordCustomerApprovalButton()).toHaveCount(0);
        }

        // Scheduled: nothing awaited, no Schedule button.
        await expect(detail.currentStep()).toContainText("Scheduled");
        await expect(detail.blockingReason()).toHaveCount(0);
        await expect(detail.recordCustomerApprovalButton()).toHaveCount(0);
        await expectNoManualSchedule(detail);

        // The engineer-driven tail.
        await page.getByRole("button", { name: "Start implementation" }).click();
        await expect(detail.currentStep()).toContainText("Implement");
        await page.getByRole("button", { name: "Mark implemented" }).click();
        await expect(detail.currentStep()).toContainText("Review");
        if (review) {
          // Review offers only "Send for customer review" -- no Close.
          await expect(detail.sendForCustomerReviewButton()).toBeVisible();
          await expect(detail.closeButton()).toHaveCount(0);
          await detail.changeStateButton().click();
          await expect(page.getByRole("menuitem", { name: "Close", exact: true })).toHaveCount(0);
          await page.keyboard.press("Escape");
          await detail.sendForCustomerReviewButton().click();
          await expect(detail.currentStep()).toContainText("Customer Review");
          await expect(detail.blockingReason()).toHaveText("Awaiting Customer Review");
          await expect(detail.sendForCustomerReviewButton()).toHaveCount(0);
        } else {
          // Review offers Close and no customer review.
          await expect(detail.closeButton()).toBeVisible();
          await expect(detail.sendForCustomerReviewButton()).toHaveCount(0);
        }
        await detail.closeButton().click();
        await expect(detail.currentStep()).toContainText("Closed");
        await expect(detail.blockingReason()).toHaveCount(0);
        await expectNoManualSchedule(detail);
        expect(api.state()).toBe("closed");
      });
    }
  }

  test("a non-creator approver sees Approve and Reject, with no creator notice", async ({ page }) => {
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR);
    const detail = new ChangeRequestDetailPage(page);
    await openDetail(detail);
    await detail.requestApproval();
    await expect(detail.currentStep()).toContainText("Assess");

    api.setViewer(FAKE_PEER);
    await page.reload();
    await expect(detail.approveButton("Pat Peer")).toBeEnabled();
    await expect(detail.rejectButton("Pat Peer")).toBeEnabled();
    await expect(detail.creatorApprovalNotice()).toHaveCount(0);
  });
});

test.describe("change request approval flow — Emergency", () => {
  test("Request Approval -> ECAB Approval only (no Peer or CAB) -> auto Scheduled", async ({ page }) => {
    test.setTimeout(60_000);
    const api = await installFakeChangeRequestApi(page, "emergency", FAKE_CREATOR);
    const detail = new ChangeRequestDetailPage(page);

    await openDetail(detail);
    await detail.requestApproval();

    await expect(detail.currentStep()).toContainText("Authorize");
    await expect(detail.blockingReason()).toHaveText("Awaiting ECAB Approval");
    await expect(detail.approverStage("Eli Ecab")).toHaveText("ECAB Approval");
    await expect(page.getByRole("cell", { name: "Peer Approval", exact: true })).toHaveCount(0);
    await expect(page.getByRole("cell", { name: "CAB Approval", exact: true })).toHaveCount(0);
    await expect(detail.approveButton()).toHaveCount(0); // creator
    await expectNoManualSchedule(detail);

    api.setViewer(FAKE_ECAB);
    await page.reload();
    await detail.approve("Eli Ecab");
    await expect(detail.currentStep()).toContainText("Scheduled");
    await expectNoManualSchedule(detail);
  });
});

test.describe("change request approval flow — Emergency with Customer Approval", () => {
  test("ECAB approval stops at Customer Approval; only 'Record customer approval' reaches Scheduled", async ({ page }) => {
    test.setTimeout(60_000);
    const api = await installFakeChangeRequestApi(page, "emergency", FAKE_CREATOR, { customerApprovalRequired: true });
    const detail = new ChangeRequestDetailPage(page);

    await openDetail(detail);
    await expect(detail.flagValue("Customer approval required")).toHaveText("Yes");
    await detail.requestApproval();
    await expect(detail.currentStep()).toContainText("Authorize");
    await expect(detail.blockingReason()).toHaveText("Awaiting ECAB Approval");
    await expect(page.getByRole("cell", { name: "Peer Approval", exact: true })).toHaveCount(0);
    await expectNoManualSchedule(detail);

    api.setViewer(FAKE_ECAB);
    await page.reload();
    await detail.approve("Eli Ecab");

    api.setViewer(FAKE_CREATOR);
    await page.reload();
    await expect(detail.currentStep()).toContainText("Customer Approval");
    await expect(detail.blockingReason()).toHaveText("Awaiting Customer Approval");
    await expect(page.getByRole("button", { name: "Start implementation" })).toHaveCount(0);
    await expectNoManualSchedule(detail);
    await detail.recordCustomerApproval();

    await expect(detail.currentStep()).toContainText("Scheduled");
    await expect(detail.blockingReason()).toHaveCount(0);
    await expect(page.getByRole("button", { name: "Start implementation" })).toBeVisible();
    await expectNoManualSchedule(detail);
  });
});

test.describe("change request approval flow — Standard with Customer Approval", () => {
  test("Request Approval goes to Customer Approval (not Scheduled), then Record customer approval schedules it", async ({
    page,
  }) => {
    await installFakeChangeRequestApi(page, "standard", FAKE_CREATOR, { customerApprovalRequired: true });
    const detail = new ChangeRequestDetailPage(page);

    await openDetail(detail);
    await detail.requestApproval();

    await expect(detail.currentStep()).toContainText("Customer Approval");
    await expect(detail.blockingReason()).toHaveText("Awaiting Customer Approval");
    await expect(page.getByText(/no approval stages recorded/i)).toBeVisible();
    await expect(page.getByRole("button", { name: "Start implementation" })).toHaveCount(0);
    await expectNoManualSchedule(detail);

    await detail.recordCustomerApproval();
    await expect(detail.currentStep()).toContainText("Scheduled");
    await expect(detail.blockingReason()).toHaveCount(0);
    await expect(page.getByRole("button", { name: "Start implementation" })).toBeVisible();
  });
});

test.describe("change request approval flow — editing the customer checkboxes", () => {
  test("both are editable before their gate, are sent via PATCH, and show on the Approval tab afterwards", async ({
    page,
  }) => {
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR);
    const detail = new ChangeRequestDetailPage(page);
    await openDetail(detail);
    await expect(detail.flagValue("Customer approval required")).toHaveText("No");

    await detail.openEditDialog();
    await expect(detail.editCustomerApprovalCheckbox()).not.toBeChecked();
    await expect(detail.editCustomerApprovalCheckbox()).toBeEnabled();
    await expect(detail.editCustomerReviewCheckbox()).toBeEnabled();
    await detail.editCustomerApprovalCheckbox().check();
    await detail.editCustomerReviewCheckbox().check();
    const [request] = await Promise.all([
      page.waitForRequest((r) => r.method() === "PATCH" && r.url().endsWith(`/change-requests/${FAKE_CR_ID}`)),
      detail.saveEdit(),
    ]);
    expect(request.postDataJSON()).toEqual({ customerApprovalRequired: true, customerReviewRequired: true });
    await expect(detail.editDialog()).toHaveCount(0);

    expect(api.flags()).toEqual({ customerApprovalRequired: true, customerReviewRequired: true });
    await expect(detail.flagValue("Customer approval required")).toHaveText("Yes");
    await expect(detail.flagValue("Customer review required")).toHaveText("Yes");
    await expectCustomerStepsOnLine(detail, { approval: true, review: true });
  });

  test("Customer Approval is disabled with an explanation once the CR is scheduled; Customer Review stays editable", async ({
    page,
  }) => {
    await installFakeChangeRequestApi(page, "standard", FAKE_CREATOR);
    const detail = new ChangeRequestDetailPage(page);
    await openDetail(detail);
    await detail.requestApproval();
    await expect(detail.currentStep()).toContainText("Scheduled");

    await detail.openEditDialog();
    await expect(detail.editCustomerApprovalCheckbox()).toBeDisabled();
    await expect(detail.editDialog().getByText(/locked/i).first()).toBeVisible();
    await expect(detail.editCustomerReviewCheckbox()).toBeEnabled();
  });

  test("Customer Review is disabled once the CR has reached customer review", async ({ page }) => {
    const api = await installFakeChangeRequestApi(page, "standard", FAKE_CREATOR, { customerReviewRequired: true });
    const detail = new ChangeRequestDetailPage(page);
    await openDetail(detail);
    api.setState("customer_review");
    await page.reload();
    await expect(detail.currentStep()).toContainText("Customer Review");

    await detail.openEditDialog();
    await expect(detail.editCustomerReviewCheckbox()).toBeDisabled();
    await expect(detail.editCustomerApprovalCheckbox()).toBeDisabled();
  });

  test("shows the backend's refusal when the gate passed while the dialog was open (400)", async ({ page }) => {
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR);
    const detail = new ChangeRequestDetailPage(page);
    await openDetail(detail);
    await detail.openEditDialog();
    await detail.editCustomerApprovalCheckbox().check();

    // The CR moves on behind the open dialog's back; the backend refuses.
    api.setState("scheduled");
    await detail.saveEdit();
    await expect(detail.editDialog().getByRole("alert")).toContainText(
      "customerApprovalRequired cannot be changed once the change request is scheduled",
    );
    await expect(detail.editDialog()).toBeVisible();
    expect(api.flags().customerApprovalRequired).toBe(false);
  });
});

test.describe("change request approval flow — Standard", () => {
  test("Request Approval goes straight to Scheduled, with no approval stages", async ({ page }) => {
    await installFakeChangeRequestApi(page, "standard", FAKE_CREATOR);
    const detail = new ChangeRequestDetailPage(page);

    await openDetail(detail);
    await detail.requestApproval();

    await expect(detail.currentStep()).toContainText("Scheduled");
    await expect(page.getByText(/no approval stages recorded/i)).toBeVisible();
    await expect(detail.blockingReason()).toHaveCount(0);
    await expectNoManualSchedule(detail);
    await expect(page.getByRole("button", { name: "Start implementation" })).toBeVisible();
  });
});

// ---------------------------------------------------------------------------
// Customer Project / Deployments / Deployment products (and the read-only Customer Group) on a
// change request, end to end against the in-browser fake backend: create ->
// detail Overview -> edit (cascade, whole-scope PATCH, backend refusal) ->
// approvals -> locked once implementation starts.
// ---------------------------------------------------------------------------

const ACME = FAKE_PROJECTS[0]!;
const BETA = FAKE_PROJECTS[1]!;
const [ACME_PROD, ACME_STG, BETA_DEV] = FAKE_DEPLOYMENTS;
const GAMMA = FAKE_PROJECTS[2]!;
const ACME_CONTACTS = FAKE_PROJECT_CONTACTS[ACME.id]!.map((u) => u.name);
const BETA_CONTACTS = FAKE_PROJECT_CONTACTS[BETA.id]!.map((u) => u.name);
const productsOf = (deploymentId: string): string[] =>
  FAKE_DEPLOYMENT_PRODUCTS.filter((p) => p.deploymentId === deploymentId).map((p) => p.name);

test.describe("change request lifecycle — project and deployments (mocked backend)", () => {
  test("Normal change: create with project + deployments, see them on the detail page, edit the scope, approve, and have it locked once implementing", async ({
    page,
  }) => {
    test.setTimeout(150_000);
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR);
    const create = new ChangeRequestCreatePage(page);
    const detail = new ChangeRequestDetailPage(page);

    // 1. Create a Normal change with a project, two deployments and a category.
    await create.goto();
    await create.selectType("Normal");
    await create.subjectField().fill("[E2E] project + deployments lifecycle (mocked)");
    await create.selectProject(ACME.name);
    await create.selectDeployments([ACME_PROD!.name, ACME_STG!.name]);
    await create.selectCategory("DevOps");
    await create.createButton().click();
    await expect(page).toHaveURL(new RegExp(`/operations/change-requests/${FAKE_CR_ID}$`));
    await expect(detail.lifecycleStepper()).toBeVisible();

    // 2. The detail Overview shows every one of them.
    await expect(detail.overviewCell("Customer Project")).toContainText(ACME.name);
    await expect(detail.overviewChips("Deployments")).toHaveText([ACME_PROD!.name, ACME_STG!.name]);
    await expect(detail.page.getByText("Environments", { exact: true })).toHaveCount(0);
    await expect(detail.overviewChips("Deployment products")).toHaveText([
      ...productsOf(ACME_PROD!.id),
      ...productsOf(ACME_STG!.id),
    ]);
    // The Customer Group is the project's registered contacts, derived and read-only.
    await expect(detail.overviewChips("Customer group")).toHaveText(ACME_CONTACTS);
    await expect(detail.overviewCell("Category")).toContainText("DevOps");
    await expect(detail.currentStep()).toContainText("New");

    // 3. Request Approval, then edit the scope while it is still editable (Assess).
    await detail.requestApproval();
    await expect(detail.currentStep()).toContainText("Assess");
    await detail.openEditDialog();
    await expect(detail.editProjectField()).toHaveValue(ACME.name);
    await expect(detail.editChipsOf(detail.editDeploymentsField())).toHaveText([ACME_PROD!.name, ACME_STG!.name]);
    await detail.editToggleOptions(detail.editDeploymentsField(), [ACME_STG!.name]); // drop Staging
    await expect(detail.editChipsOf(detail.editDeploymentProductsField())).toHaveText(productsOf(ACME_PROD!.id));
    await expect(detail.editChipsOf(detail.editCustomerGroupField())).toHaveText(ACME_CONTACTS);
    const [patch] = await Promise.all([
      page.waitForRequest((r) => r.method() === "PATCH" && r.url().endsWith(`/change-requests/${FAKE_CR_ID}`)),
      detail.saveEdit(),
    ]);
    // The whole scope goes out together, with the exact wire names.
    expect(patch.postDataJSON()).toEqual({
      projectId: ACME.id,
      deploymentIds: [ACME_PROD!.id],
      deploymentProductIds: FAKE_DEPLOYMENT_PRODUCTS.filter((p) => p.deploymentId === ACME_PROD!.id).map((p) => p.id),
    });
    await expect(detail.editDialog()).toHaveCount(0);
    await expect(detail.overviewChips("Deployments")).toHaveText([ACME_PROD!.name]);
    await expect(detail.overviewChips("Deployment products")).toHaveText(productsOf(ACME_PROD!.id));
    expect(api.scope().deploymentIds).toEqual([ACME_PROD!.id]);

    // 4. Peer and CAB approve; the creator starts implementation.
    api.setViewer(FAKE_PEER);
    await page.reload();
    await detail.approve("Pat Peer");
    await expect(detail.currentStep()).toContainText("Authorize");
    api.setViewer(FAKE_CAB);
    await page.reload();
    await detail.approve("Cam Cab");
    api.setViewer(FAKE_CREATOR);
    await page.reload();
    await expect(detail.currentStep()).toContainText("Scheduled");

    // Still editable while Scheduled ...
    await detail.openEditDialog();
    await expect(detail.editDeploymentsField()).toBeEnabled();
    await detail.editDialog().getByRole("button", { name: "Cancel" }).click();

    await page.getByRole("button", { name: "Start implementation" }).click();
    await expect(detail.currentStep()).toContainText("Implement");

    // 5. ... and locked, with the reason, from Implement on. The detail still shows it.
    await detail.openEditDialog();
    await expect(detail.editDialog().getByText(/can't be changed once implementation has started/i)).toBeVisible();
    await expect(detail.editProjectField()).toBeDisabled();
    await expect(detail.editDeploymentsField()).toBeDisabled();
    await expect(detail.saveButton()).toBeDisabled();
    await detail.editDialog().getByRole("button", { name: "Cancel" }).click();
    await expect(detail.overviewChips("Deployments")).toHaveText([ACME_PROD!.name]);
  });

  test("editing: changing the project clears the dependents and sends the new project with empty lists", async ({ page }) => {
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, {}, {
      projectId: ACME.id,
      deploymentIds: [ACME_PROD!.id],
    });
    const detail = new ChangeRequestDetailPage(page);
    await detail.goto(FAKE_CR_ID);
    await expect(detail.overviewChips("Deployments")).toHaveText([ACME_PROD!.name]);

    await detail.openEditDialog();
    // A saved project can be swapped but not cleared.
    await detail.editProjectField().click();
    await page.getByRole("option", { name: BETA.name }).click();
    await expect(detail.editChipsOf(detail.editDeploymentsField())).toHaveCount(0);
    await expect(detail.editChipsOf(detail.editDeploymentProductsField())).toHaveCount(0);
    // The read-only Customer Group follows the project: customer B's contacts replace customer A's.
    await expect(detail.editChipsOf(detail.editCustomerGroupField())).toHaveText(BETA_CONTACTS);
    const [patch] = await Promise.all([
      page.waitForRequest((r) => r.method() === "PATCH" && r.url().endsWith(`/change-requests/${FAKE_CR_ID}`)),
      detail.saveEdit(),
    ]);
    expect(patch.postDataJSON()).toEqual({
      projectId: BETA.id,
      deploymentIds: [],
      deploymentProductIds: [],
    });
    await expect(detail.editDialog()).toHaveCount(0);
    await expect(detail.overviewCell("Customer Project")).toContainText(BETA.name);
    await expect(detail.overviewCell("Deployments")).toContainText("—");
    await expect(detail.overviewChips("Customer group")).toHaveText(BETA_CONTACTS);
    expect(api.scope()).toMatchObject({ projectId: BETA.id, deploymentIds: [] });
  });

  test("editing: picking deployments of a new project derives the products", async ({ page }) => {
    await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR);
    const detail = new ChangeRequestDetailPage(page);
    await detail.goto(FAKE_CR_ID);
    await expect(detail.overviewCell("Customer Project")).toContainText("—");

    await detail.openEditDialog();
    await expect(detail.editDeploymentsField()).toBeDisabled();
    await detail.editProjectField().click();
    await page.getByRole("option", { name: BETA.name }).click();
    await detail.editToggleOptions(detail.editDeploymentsField(), [BETA_DEV!.name]);
    await expect(detail.editChipsOf(detail.editDeploymentProductsField())).toHaveText(productsOf(BETA_DEV!.id));
    await detail.saveEdit();
    await expect(detail.editDialog()).toHaveCount(0);
    await expect(detail.overviewCell("Customer Project")).toContainText(BETA.name);
    await expect(detail.overviewChips("Deployments")).toHaveText([BETA_DEV!.name]);
  });

  test("editing: the category is sent on its own when only it changed", async ({ page }) => {
    await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, {}, { category: "other" });
    const detail = new ChangeRequestDetailPage(page);
    await detail.goto(FAKE_CR_ID);
    await expect(detail.overviewCell("Category")).toContainText("Other");

    await detail.openEditDialog();
    await expect(detail.editCategoryField()).toHaveText("Other");
    await detail.editCategoryField().click();
    await page.getByRole("option", { name: "Hotfix Release - Cloud", exact: true }).click();
    const [patch] = await Promise.all([
      page.waitForRequest((r) => r.method() === "PATCH" && r.url().endsWith(`/change-requests/${FAKE_CR_ID}`)),
      detail.saveEdit(),
    ]);
    expect(patch.postDataJSON()).toEqual({ category: "hotfix_release_cloud" });
    await expect(detail.editDialog()).toHaveCount(0);
    await expect(detail.overviewCell("Category")).toContainText("Hotfix Release - Cloud");
  });

  test("editing: shows the backend's refusal verbatim when a chosen deployment was deactivated behind the dialog", async ({ page }) => {
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, {}, { projectId: ACME.id, deploymentIds: [ACME_PROD!.id] });
    const detail = new ChangeRequestDetailPage(page);
    await detail.goto(FAKE_CR_ID);
    await detail.openEditDialog();
    await detail.editToggleOptions(detail.editDeploymentsField(), [ACME_STG!.name]);

    api.retireDeployment(ACME_STG!.id);
    await detail.saveEdit();
    await expect(detail.editDialog().getByRole("alert")).toContainText(
      `deploymentIds: deployment ${ACME_STG!.name} is not an active deployment of the selected project`,
    );
    await expect(detail.editDialog()).toBeVisible();
    expect(api.scope().deploymentIds).toEqual([ACME_PROD!.id]);
  });

  test("the detail Overview shows a dash for each field a change request has none of", async ({ page }) => {
    await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR);
    const detail = new ChangeRequestDetailPage(page);
    await detail.goto(FAKE_CR_ID);
    for (const label of ["Customer Project", "Deployments", "Deployment products", "Customer group", "Category"]) {
      await expect(detail.overviewCell(label), label).toContainText("—");
    }
  });
});

//
// Customer group: the people a customer-gated change request is directed to --
// the registered contacts of its Customer Project, derived and read-only.
// Against the same fake (see its header for the exact contract): with a project
// whose contacts include someone eligible, entering Customer Approval / Customer
// Review provisions a stage for them; while it is live only Cancel is offered;
// their decision moves the CR (approve -> Scheduled / Closed, reject ->
// Canceled). With no project, a project without registered contacts, or none of
// them eligible, no stage exists and the manual "Record customer approval" /
// Close stay.
//

const NO_CUSTOMER_GROUP_TEXT =
  /^No registered customer contacts are assigned to this change request's project, so no customer approvers were assigned\./;

/** A change request on the Acme project: its customer group is Mia and Max. */
const ON_ACME = { projectId: ACME.id };

/** Cancel is the only action offered: no primary button, one menu entry. */
async function expectOnlyCancelOffered(detail: ChangeRequestDetailPage): Promise<void> {
  await expect(detail.recordCustomerApprovalButton()).toHaveCount(0);
  await expect(detail.closeButton()).toHaveCount(0);
  await expect(detail.page.getByRole("button", { name: "Start implementation" })).toHaveCount(0);
  await detail.changeStateButton().click();
  await expect(detail.page.getByRole("menuitem")).toHaveCount(1);
  await expect(detail.cancelChangeMenuItem()).toBeEnabled();
  await detail.page.keyboard.press("Escape");
}

async function switchTo(page: import("@playwright/test").Page, api: FakeChangeRequestApi, user: FakeUser): Promise<void> {
  api.setViewer(user);
  await page.reload();
}

/** Drives a fresh Normal CR through Peer and CAB approval (the creator
 * requests, Pat Peer and Cam Cab approve), leaving the viewer as Cam Cab. */
async function approveInternally(page: import("@playwright/test").Page, api: FakeChangeRequestApi, detail: ChangeRequestDetailPage): Promise<void> {
  await openDetail(detail);
  await detail.requestApproval();
  await expect(detail.currentStep()).toContainText("Assess");
  await switchTo(page, api, FAKE_PEER);
  await detail.approve("Pat Peer");
  await expect(detail.currentStep()).toContainText("Authorize");
  await switchTo(page, api, FAKE_CAB);
  await detail.approve("Cam Cab");
}

test.describe("change request approval flow — customer group (the project's registered contacts)", () => {
  test("Normal with Customer Approval and Customer Review on a project with registered contacts: every step shows the right state, stage rows and buttons for the creator, a contact and a non-contact", async ({
    page,
  }) => {
    test.setTimeout(180_000);
    const api = await installFakeChangeRequestApi(
      page,
      "normal",
      FAKE_CREATOR,
      { customerApprovalRequired: true, customerReviewRequired: true },
      ON_ACME,
    );
    const detail = new ChangeRequestDetailPage(page);
    await approveInternally(page, api, detail);

    // Customer Approval, creator: the group's stage is provisioned; Cancel only.
    await switchTo(page, api, FAKE_CREATOR);
    await expect(detail.currentStep()).toContainText("Customer Approval");
    await expect(detail.blockingReason()).toHaveText("Awaiting Customer Approval");
    await expect(page.getByText(NO_CUSTOMER_GROUP_TEXT)).toHaveCount(0);
    // The Overview lists the same people as the read-only Customer Group.
    await expect(detail.overviewChips("Customer group")).toHaveText(ACME_CONTACTS);
    for (const member of [FAKE_CUST_ONE, FAKE_CUST_TWO]) {
      await expect(detail.approverRow(member.name, "Customer Approval")).toBeVisible();
      await expect(detail.approverStatus(member.name, "Customer Approval")).toHaveText("Requested");
      await expect(detail.approverRow(member.name, "Customer Approval")).toContainText("Customer Group");
    }
    await expect(detail.approverStatus("Pat Peer", "Peer Approval")).toHaveText("Approved");
    await expect(detail.approverStatus("Cam Cab", "CAB Approval")).toHaveText("Approved");
    await expect(detail.approveButton()).toHaveCount(0);
    await expect(detail.rejectButton()).toHaveCount(0);
    await expectOnlyCancelOffered(detail);
    await expectNoManualSchedule(detail);

    // Customer Approval, non-member: sees the rows, no Approve/Reject, no manual path.
    await switchTo(page, api, FAKE_OUTSIDER);
    await expect(detail.currentStep()).toContainText("Customer Approval");
    await expect(detail.approverRow(FAKE_CUST_ONE.name, "Customer Approval")).toBeVisible();
    await expect(detail.approveButton()).toHaveCount(0);
    await expect(detail.rejectButton()).toHaveCount(0);
    await expect(detail.recordCustomerApprovalButton()).toHaveCount(0);

    // Customer Approval, group member: Approve/Reject on their own row only.
    await switchTo(page, api, FAKE_CUST_ONE);
    await expect(detail.approveButton(FAKE_CUST_ONE.name, "Customer Approval")).toBeEnabled();
    await expect(detail.rejectButton(FAKE_CUST_ONE.name, "Customer Approval")).toBeEnabled();
    await expect(detail.approveButton()).toHaveCount(1);
    await expect(detail.approveButton(FAKE_CUST_TWO.name, "Customer Approval")).toHaveCount(0);
    await expect(detail.recordCustomerApprovalButton()).toHaveCount(0);

    // The member approves; the page refreshes itself to Scheduled (no reload).
    await detail.approve(FAKE_CUST_ONE.name, "Customer Approval");
    await expect(detail.currentStep()).toContainText("Scheduled");
    expect(api.state()).toBe("scheduled");
    await expect(detail.blockingReason()).toHaveCount(0);
    await expect(detail.approverStatus(FAKE_CUST_ONE.name, "Customer Approval")).toHaveText("Approved");
    await expect(detail.approveButton()).toHaveCount(0);
    await expectNoManualSchedule(detail);

    // The engineer-driven tail up to Review.
    await switchTo(page, api, FAKE_CREATOR);
    await page.getByRole("button", { name: "Start implementation" }).click();
    await expect(detail.currentStep()).toContainText("Implement");
    await page.getByRole("button", { name: "Mark implemented" }).click();
    await expect(detail.currentStep()).toContainText("Review");
    await expect(detail.sendForCustomerReviewButton()).toBeVisible();
    await expect(detail.closeButton()).toHaveCount(0);

    // Customer Review: a stage for the same group; Cancel only.
    await detail.sendForCustomerReviewButton().click();
    await expect(detail.currentStep()).toContainText("Customer Review");
    await expect(detail.blockingReason()).toHaveText("Awaiting Customer Review");
    await expect(detail.approverStatus(FAKE_CUST_TWO.name, "Customer Review")).toHaveText("Requested");
    await expect(detail.approverRow(FAKE_CUST_TWO.name, "Customer Review")).toContainText("Customer Group");
    await expect(detail.approveButton()).toHaveCount(0); // creator
    await expectOnlyCancelOffered(detail);

    // Customer Review, non-member: nothing to decide.
    await switchTo(page, api, FAKE_OUTSIDER);
    await expect(detail.currentStep()).toContainText("Customer Review");
    await expect(detail.approveButton()).toHaveCount(0);
    await expect(detail.rejectButton()).toHaveCount(0);

    // Customer Review, the other member approves -> Closed (no reload).
    await switchTo(page, api, FAKE_CUST_TWO);
    await expect(detail.approveButton()).toHaveCount(1);
    await detail.approve(FAKE_CUST_TWO.name, "Customer Review");
    await expect(detail.currentStep()).toContainText("Closed");
    expect(api.state()).toBe("closed");
    await expect(detail.blockingReason()).toHaveCount(0);
    await expect(detail.approveButton()).toHaveCount(0);
    await expect(detail.changeStateButton()).toHaveCount(0);
    await expectNoManualSchedule(detail);
  });

  test("a contact rejecting the Customer Approval cancels the change request", async ({ page }) => {
    test.setTimeout(120_000);
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, { customerApprovalRequired: true }, ON_ACME);
    const detail = new ChangeRequestDetailPage(page);
    await approveInternally(page, api, detail);

    await switchTo(page, api, FAKE_CUST_TWO);
    await expect(detail.currentStep()).toContainText("Customer Approval");
    await detail.reject(FAKE_CUST_TWO.name);
    await expect(detail.approverStatus(FAKE_CUST_TWO.name, "Customer Approval")).toHaveText("Rejected");
    expect(api.state()).toBe("canceled");
    await expect(detail.blockingReason()).toHaveCount(0);
    await expect(detail.approveButton()).toHaveCount(0);
    await expect(detail.changeStateButton()).toHaveCount(0);
  });

  test("a contact rejecting the Customer Review moves the change request to Rollback (terminal, no actions left)", async ({ page }) => {
    test.setTimeout(120_000);
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, { customerReviewRequired: true }, ON_ACME);
    const detail = new ChangeRequestDetailPage(page);
    await approveInternally(page, api, detail);

    await switchTo(page, api, FAKE_CREATOR);
    await page.getByRole("button", { name: "Start implementation" }).click();
    await page.getByRole("button", { name: "Mark implemented" }).click();
    await detail.sendForCustomerReviewButton().click();
    await expect(detail.currentStep()).toContainText("Customer Review");

    await switchTo(page, api, FAKE_CUST_ONE);
    await detail.reject(FAKE_CUST_ONE.name);
    await expect(detail.approverStatus(FAKE_CUST_ONE.name, "Customer Review")).toHaveText("Rejected");
    expect(api.state()).toBe("rollback");
    await expect(page.locator(".MuiChip-label", { hasText: /^Rollback$/ }).first()).toBeVisible();
    await expect(detail.blockingReason()).toHaveCount(0);
    await expect(detail.approveButton()).toHaveCount(0);
    await expect(detail.changeStateButton()).toHaveCount(0);
  });

  test("no project: no customer stage, the Approval tab explains why, and Record customer approval still schedules it", async ({
    page,
  }) => {
    test.setTimeout(120_000);
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, { customerApprovalRequired: true });
    const detail = new ChangeRequestDetailPage(page);
    await approveInternally(page, api, detail);

    await switchTo(page, api, FAKE_CREATOR);
    await expect(detail.currentStep()).toContainText("Customer Approval");
    await expect(detail.blockingReason()).toHaveText("Awaiting Customer Approval");
    await expect(page.getByText(NO_CUSTOMER_GROUP_TEXT)).toBeVisible();
    await expect(page.getByRole("cell", { name: "Customer Approval", exact: true })).toHaveCount(0);
    await expect(detail.recordCustomerApprovalButton()).toBeVisible();

    await detail.recordCustomerApproval();
    await expect(detail.currentStep()).toContainText("Scheduled");
    await expect(page.getByText(NO_CUSTOMER_GROUP_TEXT)).toHaveCount(0);
    expect(api.state()).toBe("scheduled");
  });

  test("no project: Customer Review shows the helper and manual Close stays available", async ({ page }) => {
    test.setTimeout(120_000);
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, { customerReviewRequired: true });
    const detail = new ChangeRequestDetailPage(page);
    await approveInternally(page, api, detail);

    await switchTo(page, api, FAKE_CREATOR);
    await page.getByRole("button", { name: "Start implementation" }).click();
    await page.getByRole("button", { name: "Mark implemented" }).click();
    await detail.sendForCustomerReviewButton().click();
    await expect(detail.currentStep()).toContainText("Customer Review");
    await expect(detail.blockingReason()).toHaveText("Awaiting Customer Review");
    await expect(page.getByText(NO_CUSTOMER_GROUP_TEXT)).toBeVisible();
    await expect(page.getByRole("cell", { name: "Customer Review", exact: true })).toHaveCount(0);

    await detail.closeButton().click();
    await expect(detail.currentStep()).toContainText("Closed");
    expect(api.state()).toBe("closed");
  });

  test("a project without registered contacts gets no stage and the helper; picking a project that has contacts provisions the stage and the manual path goes away", async ({
    page,
  }) => {
    test.setTimeout(150_000);
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, { customerApprovalRequired: true }, { projectId: GAMMA.id });
    const detail = new ChangeRequestDetailPage(page);
    await approveInternally(page, api, detail);

    await switchTo(page, api, FAKE_CREATOR);
    await expect(detail.currentStep()).toContainText("Customer Approval");
    await expect(page.getByText(NO_CUSTOMER_GROUP_TEXT)).toBeVisible();
    await expect(detail.overviewChips("Customer group")).toHaveCount(0);
    await expect(detail.recordCustomerApprovalButton()).toBeVisible();

    // Edit the project to Acme: the group is re-derived and the stage provisioned.
    await detail.openEditDialog();
    await detail.editProjectField().click();
    await page.getByRole("option", { name: ACME.name }).click();
    await expect(detail.editChipsOf(detail.editCustomerGroupField())).toHaveText(ACME_CONTACTS);
    await detail.saveEdit();
    await expect(detail.editDialog()).toHaveCount(0);
    await expect(page.getByText(NO_CUSTOMER_GROUP_TEXT)).toHaveCount(0);
    await expect(detail.overviewChips("Customer group")).toHaveText(ACME_CONTACTS);
    await expect(detail.recordCustomerApprovalButton()).toHaveCount(0);

    await switchTo(page, api, FAKE_CUST_ONE);
    await expect(detail.approverStatus(FAKE_CUST_ONE.name, "Customer Approval")).toHaveText("Requested");
    await detail.approve(FAKE_CUST_ONE.name, "Customer Approval");
    await expect(detail.currentStep()).toContainText("Scheduled");
  });

  test("changing the project while the customer stage is live replaces it: the new project's contacts are asked, the old project's can no longer decide", async ({
    page,
  }) => {
    test.setTimeout(150_000);
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, { customerApprovalRequired: true }, ON_ACME);
    const detail = new ChangeRequestDetailPage(page);
    await approveInternally(page, api, detail);

    await switchTo(page, api, FAKE_CREATOR);
    await expect(detail.approverStatus(FAKE_CUST_ONE.name, "Customer Approval")).toHaveText("Requested");
    await detail.openEditDialog();
    await detail.editProjectField().click();
    await page.getByRole("option", { name: BETA.name }).click();
    await expect(detail.editChipsOf(detail.editCustomerGroupField())).toHaveText(BETA_CONTACTS);
    await detail.saveEdit();
    await expect(detail.editDialog()).toHaveCount(0);

    await expect(detail.overviewChips("Customer group")).toHaveText(BETA_CONTACTS);
    expect(api.stages().filter((st) => st.stage === "Customer Approval").map((st) => st.status)).toEqual(["CANCELLED", "REQUESTED"]);
    await expect(detail.approverRow(FAKE_BETA_CONTACT.name, "Customer Approval")).toBeVisible();

    // Customer A's contact is no longer asked and cannot decide ...
    await switchTo(page, api, FAKE_CUST_ONE);
    await expect(detail.approveButton()).toHaveCount(0);
    await expect(detail.rejectButton()).toHaveCount(0);
    // ... customer B's contact is.
    await switchTo(page, api, FAKE_BETA_CONTACT);
    await expect(detail.approveButton(FAKE_BETA_CONTACT.name, "Customer Approval")).toBeEnabled();
    await detail.approve(FAKE_BETA_CONTACT.name, "Customer Approval");
    await expect(detail.currentStep()).toContainText("Scheduled");
  });

  test("isolation: a change request of customer A is never put to customer B's contact, who sees no Approve / Reject", async ({ page }) => {
    test.setTimeout(150_000);
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, { customerApprovalRequired: true }, ON_ACME);
    const detail = new ChangeRequestDetailPage(page);
    await approveInternally(page, api, detail);

    await switchTo(page, api, FAKE_BETA_CONTACT);
    await expect(detail.currentStep()).toContainText("Customer Approval");
    await expect(detail.overviewChips("Customer group")).toHaveText(ACME_CONTACTS);
    await expect(page.getByText(FAKE_BETA_CONTACT.name, { exact: true })).toHaveCount(0);
    await expect(detail.approveButton()).toHaveCount(0);
    await expect(detail.rejectButton()).toHaveCount(0);
    expect(api.stages().find((st) => st.stage === "Customer Approval")?.approvers.map((a) => a.name)).toEqual(ACME_CONTACTS);
  });

  test("a project whose only contact is the creator provisions no stage: manual path stays, no helper", async ({ page }) => {
    test.setTimeout(120_000);
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, { customerApprovalRequired: true }, { projectId: GAMMA.id });
    api.setProjectContacts(GAMMA.id, [FAKE_CREATOR]);
    const detail = new ChangeRequestDetailPage(page);
    await approveInternally(page, api, detail);

    await switchTo(page, api, FAKE_CREATOR);
    await expect(detail.currentStep()).toContainText("Customer Approval");
    await expect(page.getByRole("cell", { name: "Customer Approval", exact: true })).toHaveCount(0);
    await expect(page.getByText(NO_CUSTOMER_GROUP_TEXT)).toHaveCount(0);
    await expect(detail.recordCustomerApprovalButton()).toBeVisible();
  });
});

// ---------------------------------------------------------------------------
// Roll back -- the failed-review off-ramp. Offered, next to the forward move and
// Cancel change, from Review and Customer Review only; destructive (menu-only),
// and it needs a stated reason, which is posted as a comment before the PATCH.
// Rollback is final: the stepper shows the Rollback off-ramp and no actions are
// left. Runs against the in-browser fake of the backend contract.
// ---------------------------------------------------------------------------

/** Roll back is not offered: either no overflow menu at all, or none of its entries is "Roll back". */
async function expectNoRollbackOffered(detail: ChangeRequestDetailPage): Promise<void> {
  await expect(detail.page.getByRole("button", { name: "Roll back", exact: true })).toHaveCount(0);
  if ((await detail.changeStateButton().count()) === 0) return;
  await detail.changeStateButton().click();
  await expect(detail.page.getByRole("menuitem").first()).toBeVisible();
  await expect(detail.rollbackMenuItem()).toHaveCount(0);
  await detail.page.keyboard.press("Escape");
}

/** The Rollback off-ramp: no step on the line is current, the note names the state, nothing is awaited. */
async function expectRolledBack(page: import("@playwright/test").Page, detail: ChangeRequestDetailPage, api: FakeChangeRequestApi): Promise<void> {
  await expect(detail.reasonDialog()).toHaveCount(0);
  await expect(page.getByText(/diverted from the standard path/i)).toBeVisible();
  await expect(page.locator(".MuiChip-label", { hasText: /^Rollback$/ }).first()).toBeVisible();
  await expect(detail.currentStep()).toHaveCount(0);
  await expect(detail.blockingReason()).toHaveCount(0);
  await expect(detail.changeStateButton()).toHaveCount(0);
  await expect(detail.approveButton()).toHaveCount(0);
  expect(api.state()).toBe("rollback");
  // Nobody is left pending on a rolled-back change.
  for (const st of api.stages()) {
    for (const a of st.approvers) expect(a.status, `${st.stage}/${a.name}`).not.toBe("REQUESTED");
  }
}

/** Opens Roll back from the overflow menu, shows reason is required, then confirms with `reason`. */
async function rollBackWithReason(page: import("@playwright/test").Page, detail: ChangeRequestDetailPage, reason: string): Promise<void> {
  await detail.changeStateButton().click();
  await detail.rollbackMenuItem().click();
  const dialog = detail.reasonDialog();
  await expect(dialog.getByRole("heading", { name: "Roll back this change?" })).toBeVisible();
  // A reason is required: the confirm action stays disabled until one is typed.
  await expect(dialog.getByRole("button", { name: "Roll back", exact: true })).toBeDisabled();
  await dialog.getByLabel("Reason").fill(reason);
  await expect(dialog.getByRole("button", { name: "Roll back", exact: true })).toBeEnabled();
  await dialog.getByRole("button", { name: "Roll back", exact: true }).click();
  await expect(page.getByText(/diverted from the standard path/i)).toBeVisible();
}

test.describe("change request approval flow — Roll back", () => {
  for (const review of [true, false]) {
    test(`Normal, customer review ${review ? "on" : "off"}: New -> ... -> Review -> Roll back (reason required), state shown after every step`, async ({
      page,
    }) => {
      test.setTimeout(180_000);
      const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, { customerReviewRequired: review });
      const detail = new ChangeRequestDetailPage(page);

      // Roll back is never on offer on the way to Review.
      await openDetail(detail);
      await expect(detail.currentStep()).toContainText("New");
      await expectNoRollbackOffered(detail);
      await detail.requestApproval();
      await expect(detail.currentStep()).toContainText("Assess");
      await expectNoRollbackOffered(detail);
      await switchTo(page, api, FAKE_PEER);
      await detail.approve("Pat Peer");
      await expect(detail.currentStep()).toContainText("Authorize");
      await expectNoRollbackOffered(detail);
      await switchTo(page, api, FAKE_CAB);
      await detail.approve("Cam Cab");
      await switchTo(page, api, FAKE_CREATOR);
      await expect(detail.currentStep()).toContainText("Scheduled");
      await expectNoRollbackOffered(detail);
      await page.getByRole("button", { name: "Start implementation" }).click();
      await expect(detail.currentStep()).toContainText("Implement");
      await expectNoRollbackOffered(detail);
      await page.getByRole("button", { name: "Mark implemented" }).click();

      // Review: the forward move is the primary button, Roll back sits in the menu with Cancel change.
      await expect(detail.currentStep()).toContainText("Review");
      if (review) {
        await expect(detail.sendForCustomerReviewButton()).toBeVisible();
        await expect(detail.closeButton()).toHaveCount(0);
      } else {
        await expect(detail.closeButton()).toBeVisible();
        await expect(detail.sendForCustomerReviewButton()).toHaveCount(0);
      }
      await detail.changeStateButton().click();
      await expect(detail.page.getByRole("menuitem")).toHaveText(["Roll back", "Cancel change"]);
      await detail.page.keyboard.press("Escape");

      // Backing out of the dialog leaves the change untouched.
      await detail.changeStateButton().click();
      await detail.rollbackMenuItem().click();
      await detail.reasonDialog().getByRole("button", { name: "Close", exact: true }).click();
      await expect(detail.reasonDialog()).toHaveCount(0);
      await expect(detail.currentStep()).toContainText("Review");
      expect(api.state()).toBe("review");

      await rollBackWithReason(page, detail, "Post-deployment smoke test failed.");
      await expectRolledBack(page, detail, api);
      // The reason was recorded as a comment before the state moved.
      expect(api.journal()).toContainEqual({ kind: "comment", text: "Post-deployment smoke test failed." });
      const calls = api.requests();
      expect(calls.indexOf(`POST /change-requests/${FAKE_CR_ID}/comments`)).toBeGreaterThan(-1);
      expect(calls.indexOf(`POST /change-requests/${FAKE_CR_ID}/comments`)).toBeLessThan(
        calls.lastIndexOf(`PATCH /change-requests/${FAKE_CR_ID}`),
      );
      const patchBodies = api.requestBodies().filter((b) => b.request.startsWith("PATCH"));
      expect(patchBodies[patchBodies.length - 1]?.body).toEqual({ state: "rollback" });

      // Still rolled back after a reload: a terminal state with no way out.
      await page.reload();
      await expectRolledBack(page, detail, api);
    });
  }

  test("Normal with customer review on: Review -> Customer Review -> Roll back (manual fallback, no customer group)", async ({ page }) => {
    test.setTimeout(180_000);
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, { customerReviewRequired: true });
    const detail = new ChangeRequestDetailPage(page);
    await approveInternally(page, api, detail);

    await switchTo(page, api, FAKE_CREATOR);
    await page.getByRole("button", { name: "Start implementation" }).click();
    await page.getByRole("button", { name: "Mark implemented" }).click();
    await expect(detail.currentStep()).toContainText("Review");
    await detail.sendForCustomerReviewButton().click();

    // Customer Review without a group: Close is the primary move, Roll back is in the menu.
    await expect(detail.currentStep()).toContainText("Customer Review");
    await expect(detail.blockingReason()).toHaveText("Awaiting Customer Review");
    await expect(detail.closeButton()).toBeVisible();
    await detail.changeStateButton().click();
    await expect(detail.page.getByRole("menuitem")).toHaveText(["Roll back", "Cancel change"]);
    await detail.page.keyboard.press("Escape");

    await rollBackWithReason(page, detail, "The customer rejected the result.");
    await expectRolledBack(page, detail, api);
    expect(api.journal()).toContainEqual({ kind: "comment", text: "The customer rejected the result." });
  });

  test("Customer Review with a customer group: Roll back is not offered while the group's review is pending", async ({ page }) => {
    test.setTimeout(180_000);
    const api = await installFakeChangeRequestApi(
      page,
      "normal",
      FAKE_CREATOR,
      { customerReviewRequired: true },
      ON_ACME,
    );
    const detail = new ChangeRequestDetailPage(page);
    await approveInternally(page, api, detail);

    await switchTo(page, api, FAKE_CREATOR);
    await page.getByRole("button", { name: "Start implementation" }).click();
    await page.getByRole("button", { name: "Mark implemented" }).click();
    // Review still offers Roll back (the internal review can fail).
    await detail.changeStateButton().click();
    await expect(detail.rollbackMenuItem()).toBeVisible();
    await detail.page.keyboard.press("Escape");
    await detail.sendForCustomerReviewButton().click();
    await expect(detail.currentStep()).toContainText("Customer Review");
    await expectNoRollbackOffered(detail);
    await expectOnlyCancelOffered(detail);
  });

  test("Standard: Roll back is offered from Review too, and nowhere before it", async ({ page }) => {
    test.setTimeout(180_000);
    const api = await installFakeChangeRequestApi(page, "standard", FAKE_CREATOR);
    const detail = new ChangeRequestDetailPage(page);
    await openDetail(detail);
    await expectNoRollbackOffered(detail);
    await detail.requestApproval();
    await expect(detail.currentStep()).toContainText("Scheduled");
    await expectNoRollbackOffered(detail);
    await page.getByRole("button", { name: "Start implementation" }).click();
    await expect(detail.currentStep()).toContainText("Implement");
    await expectNoRollbackOffered(detail);
    await page.getByRole("button", { name: "Mark implemented" }).click();
    await expect(detail.currentStep()).toContainText("Review");
    await rollBackWithReason(page, detail, "Backout after failed verification.");
    await expectRolledBack(page, detail, api);
  });
});

test.describe("seeded fixtures (local stack) — create with an assignment group", () => {
  test("a team picked from the Assignment group picker is saved on create (no FK 400)", async ({ page }) => {
    test.setTimeout(60_000);

    const cr = new ChangeRequestCreatePage(page);
    await cr.goto();
    await cr.selectType("Normal");
    await cr.subjectField().fill(`[E2E] local create with assignment group ${new Date().toISOString()}`);

    const group = page.getByRole("combobox", { name: /^Assignment group/ });
    await group.fill("Apollo");
    await page.getByRole("option", { name: /Apollo/ }).first().click();

    const [response] = await Promise.all([
      page.waitForResponse((r) => r.request().method() === "POST" && /\/change-requests$/.test(r.url())),
      cr.createButton().click(),
    ]);
    expect(response.status(), await response.text()).toBe(201);
    await expect(page).toHaveURL(/\/operations\/change-requests\/(?!new(?:[/?#]|$))[^/]+$/, { timeout: 15_000 });
  });
});
