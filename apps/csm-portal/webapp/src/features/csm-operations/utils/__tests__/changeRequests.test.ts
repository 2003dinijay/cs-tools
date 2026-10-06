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

import { describe, expect, it } from "vitest";
import {
  approvalStageLabel,
  buildChangeRequestSearchFilters,
  buildCloneChangeRequestNavState,
  CHANGE_REQUEST_CREATE_TYPE_OPTIONS,
  changeRequestBlockingReason,
  CHANGE_REQUEST_CATEGORY_OPTIONS,
  changeRequestCategoryLabel,
  changeRequestCategoryValue,
  changeRequestScopeLockedReason,
  changeRequestTransitionLabel,
  changeRequestTransitionRequiresReason,
  countActiveCRFilters,
  customerBypassTarget,
  customerGateWithheldTargets,
  customerRequestPendingReason,
  isCustomerBypassTransition,
  isDestructiveChangeRequestTransition,
  pendingCustomerRequest,
  customerApprovalLockedReason,
  customerReviewLockedReason,
  DEFAULT_CHANGE_REQUEST_CATEGORY,
  DEFAULT_CR_FILTERS,
  isChangeRequestCategory,
  isChangeRequestCreator,
  isCreatableChangeRequestType,
  NO_CUSTOMER_CONTACTS_HELPER,
  noCustomerContactsHelper,
} from "@features/csm-operations/utils/changeRequests";
import type { BeChangeRequestApproval, BeChangeRequestDetail } from "@api/backend/types";

const FULL_CR: BeChangeRequestDetail = {
  id: "chg-1",
  number: "CHG0009988",
  subject: "Upgrade the gateway cluster",
  description: "<p>Upgrade to the latest patch level.</p>",
  project: { id: "proj-1", name: "Project A" },
  case: { id: "case-1", name: "CASE0001234" },
  deployment: { id: "dep-1", name: "prod" },
  deployedProduct: { id: "dp-1", name: "API Manager" },
  product: { id: "product-1", name: "API Manager" },
  assignedEngineer: { id: "user-1", name: "Jane Doe" },
  assignedTeam: { id: "team-1", name: "Platform" },
  plannedStartOn: "2026-01-01T00:00:00Z",
  plannedEndOn: "2026-01-02T00:00:00Z",
  duration: "1 day",
  impact: "medium",
  state: "closed",
  type: "normal",
  createdOn: "2025-12-01T00:00:00Z",
  updatedOn: "2025-12-02T00:00:00Z",
  createdBy: "someone@example.com",
  justification: "<p>Needed for the security patch.</p>",
  impactDescription: "<p>Brief outage expected.</p>",
  serviceOutage: "<p>5 minutes.</p>",
  communicationPlan: "<p>Notify via status page.</p>",
  rollbackPlan: "<p>Revert to the previous image.</p>",
  testPlan: "<p>Run the smoke suite.</p>",
  hasCustomerApproved: true,
  hasCustomerReviewed: true,
  approvedBy: { id: "approver-1", name: "Approver Name" },
  approvedOn: "2025-12-05T00:00:00Z",
  legalNextStates: [],
};

describe("buildCloneChangeRequestNavState", () => {
  it("carries over the fields that are genuinely the same on read and create", () => {
    const state = buildCloneChangeRequestNavState(FULL_CR);
    expect(state.subject).toBe("Upgrade the gateway cluster");
    expect(state.description).toContain("Upgrade to the latest patch level.");
    expect(state.justification).toContain("Needed for the security patch.");
    expect(state.testPlan).toContain("Run the smoke suite.");
    expect(state.type).toBe("normal");
    expect(state.impact).toBe("medium");
    expect(state.assignedEngineerId).toBe("user-1");
    expect(state.assignedEngineerLabel).toBe("Jane Doe");
    expect(state.sourceNumber).toBe("CHG0009988");
  });

  it("never surfaces a field that create-time payload has no slot for", () => {
    const state = buildCloneChangeRequestNavState(FULL_CR);
    const keys = Object.keys(state);
    // impactDescription/serviceOutage/communicationPlan/rollbackPlan are
    // read-only on the backend today — BeCreateChangeRequestPayload has no
    // field for any of them, so they must never appear in the clone state.
    expect(keys).not.toContain("impactDescription");
    expect(keys).not.toContain("serviceOutage");
    expect(keys).not.toContain("communicationPlan");
    expect(keys).not.toContain("rollbackPlan");
    // priority/risk/riskImpactAnalysis have no clone source. `implementationPlan`
    // is readable now too, but isn't wired into clone yet (separate feature
    // decision), so it also must not appear here.
    expect(keys).not.toContain("priority");
    expect(keys).not.toContain("risk");
    expect(keys).not.toContain("implementationPlan");
    expect(keys).not.toContain("riskImpactAnalysis");
  });

  it("carries the customer project and category, but never the deployments / deployment products / customer group", () => {
    const state = buildCloneChangeRequestNavState({
      ...FULL_CR,
      customerContacts: [{ id: "c-1", name: "Alice Aaron" }],
      category: { id: "devops", name: "DevOps" },
      deployments: [{ id: "dep-1", name: "prod" }],
      deploymentProducts: [{ id: "dp-1", name: "API Manager 4.3.0" }],
    });
    expect(state.projectId).toBe("proj-1");
    expect(state.projectLabel).toBe("Project A");
    expect(state.category).toBe("devops");
    // A clone exists to promote the change to a different deployment, so what
    // names the *target* is left for the user to choose; the Customer Group is
    // derived from the project and read-only, so it is never carried either.
    const keys = Object.keys(state);
    expect(keys).not.toContain("deployments");
    expect(keys).not.toContain("deploymentIds");
    expect(keys).not.toContain("customerContacts");
    expect(keys).not.toContain("customerGroupId");
    expect(keys).not.toContain("customerGroupLabel");
    expect(keys).not.toContain("environments");
    expect(keys).not.toContain("environmentIds");
    expect(keys).not.toContain("deploymentProducts");
    expect(keys).not.toContain("deploymentProductIds");
  });

  it("leaves project / category out when the source has none (or an unknown category)", () => {
    const state = buildCloneChangeRequestNavState({
      ...FULL_CR,
      project: undefined,
      customerContacts: [],
      category: { id: "something_new", label: "Something new" },
    });
    expect(state.projectId).toBeUndefined();
    expect(state.category).toBeUndefined();
  });

  it("never carries the single-deployment, linked-case or team references", () => {
    const state = buildCloneChangeRequestNavState(FULL_CR);
    const keys = Object.keys(state);
    expect(keys).not.toContain("deployment");
    expect(keys).not.toContain("deployedProduct");
    expect(keys).not.toContain("project");
    expect(keys).not.toContain("case");
    expect(keys).not.toContain("product");
    expect(keys).not.toContain("assignedTeam");
  });

  it("carries the customer approval / review checkbox settings, but not the customer's confirmation", () => {
    const state = buildCloneChangeRequestNavState({
      ...FULL_CR,
      customerApprovalRequired: true,
      customerReviewRequired: false,
    });
    expect(state.customerApprovalRequired).toBe(true);
    expect(state.customerReviewRequired).toBe(false);
    expect(Object.keys(state)).not.toContain("hasCustomerApproved");
    expect(Object.keys(state)).not.toContain("hasCustomerReviewed");
  });

  it("never carries state, schedule, or approval fields", () => {
    const state = buildCloneChangeRequestNavState(FULL_CR);
    const keys = Object.keys(state);
    expect(keys).not.toContain("state");
    expect(keys).not.toContain("plannedStartOn");
    expect(keys).not.toContain("plannedEndOn");
    expect(keys).not.toContain("hasCustomerApproved");
    expect(keys).not.toContain("hasCustomerReviewed");
    expect(keys).not.toContain("approvedBy");
    expect(keys).not.toContain("approvedOn");
  });

  it("never carries auto-numbered, created-by, or timestamp fields", () => {
    const state = buildCloneChangeRequestNavState(FULL_CR);
    const keys = Object.keys(state);
    expect(keys).not.toContain("id");
    expect(keys).not.toContain("createdOn");
    expect(keys).not.toContain("updatedOn");
    expect(keys).not.toContain("createdBy");
    expect(keys).not.toContain("duration");
    expect(keys).not.toContain("legalNextStates");
  });

  it("omits a blank rich-text field instead of copying an empty-looking paragraph", () => {
    const state = buildCloneChangeRequestNavState({
      ...FULL_CR,
      description: "<p><br></p>",
      justification: null,
      testPlan: undefined,
    });
    expect(state.description).toBeUndefined();
    expect(state.justification).toBeUndefined();
    expect(state.testPlan).toBeUndefined();
  });

  it("omits the assigned engineer entirely when the source record has none", () => {
    const state = buildCloneChangeRequestNavState({ ...FULL_CR, assignedEngineer: null });
    expect(state.assignedEngineerId).toBeUndefined();
    expect(state.assignedEngineerLabel).toBeUndefined();
  });

  it("sanitizes rich-text content before it reaches the clone form's editor", () => {
    const state = buildCloneChangeRequestNavState({
      ...FULL_CR,
      description: '<p>Safe</p><script>alert("xss")</script>',
    });
    expect(state.description).not.toContain("<script>");
    expect(state.description).toContain("Safe");
  });
});

describe("changeRequestBlockingReason", () => {
  function approval(overrides: Partial<BeChangeRequestApproval>): BeChangeRequestApproval {
    return {
      stage: "Assess",
      approverType: "STATIC_GROUP",
      approverName: null,
      status: "APPROVED",
      approvers: [],
      ...overrides,
    };
  }

  it("returns null when there are no approval stages yet", () => {
    expect(changeRequestBlockingReason(undefined)).toBeNull();
    expect(changeRequestBlockingReason([])).toBeNull();
  });

  it("returns null when every stage is settled (approved/rejected/not required)", () => {
    expect(
      changeRequestBlockingReason([
        approval({ status: "APPROVED" }),
        approval({ stage: "Authorize", status: "NOT_REQUIRED" }),
      ]),
    ).toBeNull();
  });

  // After a Re-schedule the superseded Customer Approval stage keeps its place in
  // the list, all its approvers cancelled, and the backend reports it PENDING
  // (nothing was approved or rejected on it). It is not what the change waits on.
  it("skips a superseded stage whose approvers were all cancelled, naming the stage that is really waiting", () => {
    const approver = (status: string) => ({ id: `u-${status}`, name: "Someone", status });
    expect(
      changeRequestBlockingReason(
        [
          approval({ stage: "Peer Approval", status: "APPROVED", approvers: [approver("APPROVED")] }),
          approval({ stage: "CAB Approval", status: "APPROVED", approvers: [approver("APPROVED")] }),
          approval({ stage: "Customer Approval", status: "PENDING", approvers: [approver("CANCELLED"), approver("CANCELED")] }),
          approval({ stage: "CAB Approval", status: "PENDING", approvers: [approver("REQUESTED"), approver("CANCELLED")] }),
        ],
        "authorize",
      ),
    ).toBe("Awaiting CAB Approval");
  });

  it("returns null when the only PENDING stage has nobody left to ask", () => {
    expect(
      changeRequestBlockingReason(
        [approval({ stage: "CAB Approval", status: "PENDING", approvers: [{ id: "u", name: "A", status: "NOT_REQUIRED" }] })],
        "authorize",
      ),
    ).toBeNull();
  });

  it("labels a legacy Authorize stage as CAB Approval", () => {
    expect(
      changeRequestBlockingReason([approval({ stage: "Authorize", status: "REQUESTED" })]),
    ).toBe("Awaiting CAB Approval");
  });

  it("labels a legacy Assess stage as Peer Approval, and treats PENDING the same as REQUESTED", () => {
    expect(
      changeRequestBlockingReason([approval({ stage: "Assess", status: "PENDING" })]),
    ).toBe("Awaiting Peer Approval");
  });

  it.each([
    ["Peer Approval", "Awaiting Peer Approval"],
    ["CAB Approval", "Awaiting CAB Approval"],
    ["CAB", "Awaiting CAB Approval"],
    ["ECAB Approval", "Awaiting ECAB Approval"],
    ["ECAB", "Awaiting ECAB Approval"],
    ["Emergency CAB", "Awaiting ECAB Approval"],
    ["emergency_cab_approval", "Awaiting ECAB Approval"],
  ])("names stage %s as '%s' with no doubled 'approval'", (stage, expected) => {
    const reason = changeRequestBlockingReason([approval({ stage, status: "REQUESTED" })]);
    expect(reason).toBe(expected);
    expect(reason?.match(/approval/gi)).toHaveLength(1);
  });

  it("names the post-implementation Review stage 'Awaiting Review'", () => {
    expect(
      changeRequestBlockingReason([approval({ stage: "Review", status: "REQUESTED" })]),
    ).toBe("Awaiting Review");
  });

  it("prefers the stage label over the approver group name for a recognised stage", () => {
    expect(
      changeRequestBlockingReason([
        approval({ stage: "Authorize", status: "REQUESTED", approverName: "Devops Approval" }),
      ]),
    ).toBe("Awaiting CAB Approval");
  });

  it("names the approver group for a stage it has no label for", () => {
    expect(
      changeRequestBlockingReason([
        approval({ stage: "Vendor Sign-off", status: "REQUESTED", approverName: "Acme Contact" }),
      ]),
    ).toBe("Awaiting Acme Contact approval");
  });

  it("does not double the word 'approval' when the approver name already carries it", () => {
    const reason = changeRequestBlockingReason([
      approval({ stage: "Vendor Sign-off", status: "REQUESTED", approverName: "Security Approval Board" }),
    ]);
    expect(reason).toBe("Awaiting Security Approval Board");
    expect(reason?.match(/approval/gi)).toHaveLength(1);
  });

  it("returns the first waiting stage, in stage order, when several are unsettled", () => {
    expect(
      changeRequestBlockingReason([
        approval({ stage: "Assess", status: "APPROVED" }),
        approval({ stage: "Authorize", status: "REQUESTED" }),
        approval({ stage: "Customer Approval", status: "PENDING" }),
      ]),
    ).toBe("Awaiting CAB Approval");
  });

  it("is case-insensitive on the status value", () => {
    expect(
      changeRequestBlockingReason([approval({ stage: "Assess", status: "requested" })]),
    ).toBe("Awaiting Peer Approval");
  });
});

describe("approvalStageLabel", () => {
  it.each([
    ["Assess", "Peer Approval"],
    ["Peer Approval", "Peer Approval"],
    ["Authorize", "CAB Approval"],
    ["CAB Approval", "CAB Approval"],
    ["Emergency CAB", "ECAB Approval"],
    ["ECAB Approval", "ECAB Approval"],
    ["Review", "Review"],
    ["Customer Approval", "Customer Approval"],
    ["customer_approval", "Customer Approval"],
    ["CUSTOMER-APPROVAL", "Customer Approval"],
    ["Customer Review", "Customer Review"],
    ["customer review", "Customer Review"],
    ["Something New", "Something New"],
  ])("maps %s to %s", (stage, expected) => {
    expect(approvalStageLabel(stage)).toBe(expected);
  });

  it("falls back to a generic label for a blank stage", () => {
    expect(approvalStageLabel("")).toBe("Approval");
    expect(approvalStageLabel(undefined)).toBe("Approval");
  });
});

describe("changeRequestTransitionLabel", () => {
  it("labels the New -> Assess transition 'Request Approval', never 'Move to Assess'", () => {
    expect(changeRequestTransitionLabel("assess")).toBe("Request Approval");
    expect(changeRequestTransitionLabel("assess")).not.toMatch(/move to assess/i);
  });

  it("has no curated 'Schedule' action label for the scheduled state", () => {
    expect(changeRequestTransitionLabel("scheduled")).not.toMatch(/^schedule$/i);
    expect(changeRequestTransitionLabel("scheduled", "authorize")).not.toMatch(/^schedule$/i);
  });

  it("labels scheduled 'Bypass customer approval' only when leaving customer_approval", () => {
    expect(changeRequestTransitionLabel("scheduled", "customer_approval")).toBe(
      "Bypass customer approval",
    );
    expect(changeRequestTransitionLabel("scheduled")).not.toBe("Bypass customer approval");
    expect(changeRequestTransitionLabel("scheduled", "authorize")).not.toBe("Bypass customer approval");
  });

  it("labels closed 'Bypass customer review' only when leaving customer_review; out of Review it is plain Close", () => {
    expect(changeRequestTransitionLabel("closed", "customer_review")).toBe("Bypass customer review");
    expect(changeRequestTransitionLabel("closed", "review")).toBe("Close");
    expect(changeRequestTransitionLabel("closed")).toBe("Close");
  });

  it("never uses the retired 'Record customer approval' wording", () => {
    for (const from of [undefined, "new", "customer_approval", "customer_review", "review"]) {
      for (const target of ["scheduled", "closed", "authorize", "customer_review"]) {
        expect(changeRequestTransitionLabel(target, from)).not.toMatch(/record customer/i);
      }
    }
  });

  it("labels authorize 'Re-schedule' only when leaving customer_approval", () => {
    expect(changeRequestTransitionLabel("authorize", "customer_approval")).toBe("Re-schedule");
    expect(changeRequestTransitionLabel("authorize", "assess")).not.toBe("Re-schedule");
    expect(changeRequestTransitionLabel("authorize")).not.toBe("Re-schedule");
    expect(isDestructiveChangeRequestTransition("authorize")).toBe(false);
  });

  it("labels the customer review and close transitions", () => {
    expect(changeRequestTransitionLabel("customer_review", "review")).toBe("Send for customer review");
    expect(changeRequestTransitionLabel("closed", "review")).toBe("Close");
  });

  it("labels the failed-review off-ramp 'Roll back': destructive and needing a reason, like Cancel change", () => {
    for (const from of ["review", "customer_review"]) {
      expect(changeRequestTransitionLabel("rollback", from)).toBe("Roll back");
    }
    expect(isDestructiveChangeRequestTransition("rollback")).toBe(true);
    expect(changeRequestTransitionRequiresReason("rollback")).toBe(true);
    expect(changeRequestTransitionRequiresReason("canceled")).toBe(true);
    expect(changeRequestTransitionRequiresReason("closed")).toBe(false);
  });
});

describe("customer bypass transitions", () => {
  // Every (target, fromState) pair that is, or must stay, an ordinary move: only the two
  // customer bypasses are bypasses.
  const STATES = [
    "new", "assess", "authorize", "customer_approval", "scheduled", "implement", "review",
    "customer_review", "rollback", "closed", "canceled",
  ];
  const TARGETS = [
    "assess", "authorize", "customer_approval", "scheduled", "implement", "review",
    "customer_review", "rollback", "closed", "canceled", "awaiting_vendor",
  ];

  it("is true for exactly scheduled out of customer_approval and closed out of customer_review", () => {
    const bypasses = TARGETS.flatMap((target) =>
      STATES.filter((from) => isCustomerBypassTransition(target, from)).map((from) => `${from} -> ${target}`),
    );
    expect(bypasses.sort()).toEqual(["customer_approval -> scheduled", "customer_review -> closed"]);
  });

  it("is false without a known fromState", () => {
    expect(isCustomerBypassTransition("scheduled")).toBe(false);
    expect(isCustomerBypassTransition("closed", null)).toBe(false);
    expect(isCustomerBypassTransition("closed", undefined)).toBe(false);
  });

  it("names the bypass target of each customer gate, and none elsewhere", () => {
    expect(customerBypassTarget("customer_approval")).toBe("scheduled");
    expect(customerBypassTarget("customer_review")).toBe("closed");
    for (const from of STATES.filter((s) => !s.startsWith("customer_"))) {
      expect(customerBypassTarget(from), from).toBeNull();
    }
    expect(customerBypassTarget(undefined)).toBeNull();
  });

  it("agrees with customerBypassTarget for every gate", () => {
    for (const from of STATES) {
      const target = customerBypassTarget(from);
      if (target) expect(isCustomerBypassTransition(target, from), from).toBe(true);
    }
  });

  it("needs a stated reason, like Roll back and Cancel change, but only as a bypass", () => {
    expect(changeRequestTransitionRequiresReason("scheduled", "customer_approval")).toBe(true);
    expect(changeRequestTransitionRequiresReason("closed", "customer_review")).toBe(true);
    // The same targets reached any other way are ordinary moves.
    expect(changeRequestTransitionRequiresReason("closed", "review")).toBe(false);
    expect(changeRequestTransitionRequiresReason("closed")).toBe(false);
    expect(changeRequestTransitionRequiresReason("scheduled")).toBe(false);
    expect(changeRequestTransitionRequiresReason("scheduled", "authorize")).toBe(false);
    // Unchanged: the destructive off-ramps need a reason from any state.
    for (const from of [undefined, "review", "customer_review", "implement"]) {
      expect(changeRequestTransitionRequiresReason("rollback", from)).toBe(true);
      expect(changeRequestTransitionRequiresReason("canceled", from)).toBe(true);
    }
  });

  it("is not destructive (warning, never the error colour); only rollback and canceled are", () => {
    expect(isDestructiveChangeRequestTransition("scheduled")).toBe(false);
    expect(isDestructiveChangeRequestTransition("closed")).toBe(false);
    expect(isDestructiveChangeRequestTransition("rollback")).toBe(true);
    expect(isDestructiveChangeRequestTransition("canceled")).toBe(true);
  });
});

describe("pendingCustomerRequest", () => {
  const stage = (
    name: string,
    approvers: Array<[string, string]>,
    status = "PENDING",
  ): BeChangeRequestApproval => ({
    stage: name,
    approverType: "STATIC_GROUP",
    approverName: "Customer Group",
    status,
    approvers: approvers.map(([n, st], i) => ({ id: `u-${i}`, name: n, status: st })),
  });

  it("names the contacts still being asked at Customer Approval", () => {
    expect(
      pendingCustomerRequest(
        [stage("Customer Approval", [["Mira Santos", "REQUESTED"], ["Noel Prasad", "REQUESTED"]])],
        "customer_approval",
      ),
    ).toEqual({ kind: "approval", contactNames: ["Mira Santos", "Noel Prasad"], askedCount: 2 });
  });

  it("names the contacts still being asked at Customer Review", () => {
    expect(
      pendingCustomerRequest([stage("Customer Review", [["Mira Santos", "REQUESTED"]])], "customer_review"),
    ).toEqual({ kind: "review", contactNames: ["Mira Santos"], askedCount: 1 });
  });

  it("lists only the approvers still REQUESTED, case-insensitively and without duplicates", () => {
    expect(
      pendingCustomerRequest(
        [
          stage("Customer Approval", [
            ["Mira Santos", "requested"],
            ["Noel Prasad", "CANCELLED"],
            ["Mira Santos", "REQUESTED"],
          ]),
        ],
        "customer_approval",
      ),
    ).toEqual({ kind: "approval", contactNames: ["Mira Santos"], askedCount: 2 });
  });

  it("is pending even when no approver has a name", () => {
    expect(
      pendingCustomerRequest(
        [{ ...stage("Customer Approval", []), approvers: [{ id: "u-1", status: "REQUESTED" }] }],
        "customer_approval",
      ),
    ).toEqual({ kind: "approval", contactNames: [], askedCount: 1 });
  });

  it("counts everyone still asked, the nameless and the ones sharing a name included", () => {
    const asked = pendingCustomerRequest(
      [
        {
          ...stage("Customer Approval", [["Dana Lee", "REQUESTED"]]),
          approvers: [
            { id: "u-1", name: "Dana Lee", status: "REQUESTED" },
            { id: "u-2", name: "", status: "REQUESTED" },
            { id: "u-3", status: "REQUESTED" },
            { id: "u-4", name: "Dana Lee", status: "REQUESTED" },
            { id: "u-5", name: "Sam Roe", status: "CANCELLED" },
          ],
        },
      ],
      "customer_approval",
    );
    expect(asked).toEqual({ kind: "approval", contactNames: ["Dana Lee"], askedCount: 4 });
  });

  it("is null once nobody is being asked, whatever the stage's own status says", () => {
    // A superseded request after a Re-schedule: the backend still reports the stage PENDING.
    expect(
      pendingCustomerRequest(
        [stage("Customer Approval", [["Mira Santos", "CANCELLED"]], "PENDING")],
        "customer_approval",
      ),
    ).toBeNull();
    expect(
      pendingCustomerRequest(
        [stage("Customer Approval", [["Mira Santos", "APPROVED"], ["Noel Prasad", "CANCELLED"]], "APPROVED")],
        "customer_approval",
      ),
    ).toBeNull();
    expect(pendingCustomerRequest([stage("Customer Approval", [])], "customer_approval")).toBeNull();
  });

  it("only looks at the stage of the change's current gate", () => {
    const approvals = [
      stage("Customer Approval", [["Mira Santos", "REQUESTED"]]),
      stage("Customer Review", [["Noel Prasad", "REQUESTED"]]),
    ];
    expect(pendingCustomerRequest(approvals, "customer_approval")?.contactNames).toEqual(["Mira Santos"]);
    expect(pendingCustomerRequest(approvals, "customer_review")?.contactNames).toEqual(["Noel Prasad"]);
  });

  it("ignores internal stages with REQUESTED approvers", () => {
    expect(
      pendingCustomerRequest([stage("CAB Approval", [["Cam Cab", "REQUESTED"]])], "customer_approval"),
    ).toBeNull();
  });

  it("is null outside a customer gate, and while the approvals have not loaded", () => {
    const approvals = [stage("Customer Approval", [["Mira Santos", "REQUESTED"]])];
    for (const state of ["new", "assess", "authorize", "scheduled", "implement", "review", "closed", undefined, null]) {
      expect(pendingCustomerRequest(approvals, state), String(state)).toBeNull();
    }
    expect(pendingCustomerRequest(undefined, "customer_approval")).toBeNull();
    expect(pendingCustomerRequest(null, "customer_approval")).toBeNull();
  });
});

describe("customerRequestPendingReason", () => {
  it("says who the request is waiting on and that they answer in the Customer Portal", () => {
    expect(customerRequestPendingReason({ kind: "approval", contactNames: ["Mira Santos", "Noel Prasad"] })).toBe(
      "Customer approval is pending from Mira Santos, Noel Prasad. They answer in the Customer Portal, so it can't be bypassed from here.",
    );
    expect(customerRequestPendingReason({ kind: "review", contactNames: ["Mira Santos"] })).toBe(
      "Customer review is pending from Mira Santos. They answer in the Customer Portal, so it can't be bypassed from here.",
    );
  });

  it("summarises a long contact list", () => {
    expect(
      customerRequestPendingReason({ kind: "approval", contactNames: ["A", "B", "C", "D", "E"] }),
    ).toBe(
      "Customer approval is pending from A, B, C and 2 more. They answer in the Customer Portal, so it can't be bypassed from here.",
    );
  });

  it("counts the people asked who have no name (or the same one) in the 'and N more'", () => {
    // Dana Lee plus two approvers the backend sends no name for: three are asked, one is named.
    expect(
      customerRequestPendingReason({ kind: "approval", contactNames: ["Dana Lee"], askedCount: 3 }),
    ).toBe(
      "Customer approval is pending from Dana Lee and 2 more. They answer in the Customer Portal, so it can't be bypassed from here.",
    );
    // Three contacts that share a name read as that name and two more, not as one person.
    expect(
      customerRequestPendingReason({ kind: "review", contactNames: ["Sam Lee"], askedCount: 3 }),
    ).toBe(
      "Customer review is pending from Sam Lee and 2 more. They answer in the Customer Portal, so it can't be bypassed from here.",
    );
    // With five distinct names it still says "A, B, C and 2 more".
    expect(
      customerRequestPendingReason({ kind: "approval", contactNames: ["A", "B", "C", "D", "E"], askedCount: 5 }),
    ).toContain("A, B, C and 2 more.");
    // An absent askedCount reads as the number of names.
    expect(customerRequestPendingReason({ kind: "approval", contactNames: ["A", "B"] })).toContain("from A, B.");
  });

  it("words a rolled-back review as the customer's to give, not as a bypass", () => {
    expect(
      customerRequestPendingReason({ kind: "review", contactNames: ["Mira Santos", "Noel Prasad"], askedCount: 2 }, "rollback"),
    ).toBe(
      "Customer review is pending from Mira Santos, Noel Prasad. A failed review is theirs to give in the Customer Portal, so the change can't be rolled back from here.",
    );
    expect(customerRequestPendingReason({ kind: "review", contactNames: [] }, "rollback")).toBe(
      "Customer review is pending. A failed review is the customer's to give in the Customer Portal, so the change can't be rolled back from here.",
    );
    expect(customerRequestPendingReason(null, "rollback")).toBeNull();
  });

  it("falls back to a generic sentence without contact names", () => {
    expect(customerRequestPendingReason({ kind: "approval", contactNames: [] })).toBe(
      "Customer approval is pending. The customer answers in the Customer Portal, so it can't be bypassed from here.",
    );
    expect(customerRequestPendingReason({ kind: "review", contactNames: [] })).toMatch(/^Customer review is pending\./);
  });

  it("is null when nothing is pending", () => {
    expect(customerRequestPendingReason(null)).toBeNull();
    expect(customerRequestPendingReason(undefined)).toBeNull();
  });
});

describe("customerGateWithheldTargets", () => {
  it("lists what the backend withholds while the customer's request is live", () => {
    expect(customerGateWithheldTargets("customer_approval")).toEqual(["scheduled"]);
    expect(customerGateWithheldTargets("customer_review")).toEqual(["closed", "rollback"]);
  });

  it("is empty outside a customer gate", () => {
    for (const state of ["new", "assess", "authorize", "scheduled", "implement", "review", "closed", "rollback", "canceled", null, undefined]) {
      expect(customerGateWithheldTargets(state), String(state)).toEqual([]);
    }
  });
});

describe("changeRequestBlockingReason — customer states", () => {
  it("names the customer approval gate from the state, with or without approvals data", () => {
    expect(changeRequestBlockingReason(undefined, "customer_approval")).toBe(
      "Awaiting Customer Approval",
    );
    expect(
      changeRequestBlockingReason(
        [{ stage: "Authorize", approverType: "STATIC_GROUP", approverName: null, status: "APPROVED", approvers: [] }],
        "customer_approval",
      ),
    ).toBe("Awaiting Customer Approval");
  });

  it("names the customer review gate from the state", () => {
    expect(changeRequestBlockingReason(undefined, "customer_review")).toBe(
      "Awaiting Customer Review",
    );
  });

  it("still derives the reason from approval stages for any other state", () => {
    expect(
      changeRequestBlockingReason(
        [{ stage: "Authorize", approverType: "STATIC_GROUP", approverName: null, status: "REQUESTED", approvers: [] }],
        "authorize",
      ),
    ).toBe("Awaiting CAB Approval");
  });
});

describe("customer approval / review edit locks", () => {
  it("locks Customer Approval from customer_approval onwards (incl. off-ramps), not before", () => {
    for (const s of ["new", "assess", "authorize"]) {
      expect(customerApprovalLockedReason(s)).toBeNull();
    }
    for (const s of ["customer_approval", "scheduled", "implement", "review", "customer_review", "closed", "rollback", "canceled"]) {
      expect(customerApprovalLockedReason(s)).toMatch(/locked/i);
    }
  });

  it("locks Customer Review from customer_review onwards, but not at review or earlier", () => {
    for (const s of ["new", "assess", "authorize", "customer_approval", "scheduled", "implement", "review"]) {
      expect(customerReviewLockedReason(s)).toBeNull();
    }
    for (const s of ["customer_review", "closed", "rollback", "canceled"]) {
      expect(customerReviewLockedReason(s)).toMatch(/locked/i);
    }
  });
});

describe("CHANGE_REQUEST_CREATE_TYPE_OPTIONS", () => {
  it("offers exactly Normal, Standard, Emergency, in that order, with the backend enum values", () => {
    expect(CHANGE_REQUEST_CREATE_TYPE_OPTIONS.map((o) => [o.value, o.label])).toEqual([
      ["normal", "Normal"],
      ["standard", "Standard"],
      ["emergency", "Emergency"],
    ]);
    CHANGE_REQUEST_CREATE_TYPE_OPTIONS.forEach((o) => expect(o.description.length).toBeGreaterThan(10));
  });

  it("only treats those three values as creatable", () => {
    expect(isCreatableChangeRequestType("normal")).toBe(true);
    expect(isCreatableChangeRequestType("standard")).toBe(true);
    expect(isCreatableChangeRequestType("emergency")).toBe(true);
    expect(isCreatableChangeRequestType("azure")).toBe(false);
    expect(isCreatableChangeRequestType("model")).toBe(false);
    expect(isCreatableChangeRequestType("")).toBe(false);
    expect(isCreatableChangeRequestType(undefined)).toBe(false);
  });
});

describe("isChangeRequestCreator", () => {
  it("matches the requester id against the user id", () => {
    expect(isChangeRequestCreator({ requestedBy: { id: "u-1" } }, { id: "u-1" })).toBe(true);
  });

  it("matches createdBy against the user id or email, case-insensitively", () => {
    expect(isChangeRequestCreator({ createdBy: "U-1" }, { id: "u-1" })).toBe(true);
    expect(isChangeRequestCreator({ createdBy: "Jane@Example.com" }, { email: "jane@example.com" })).toBe(true);
  });

  it("is false for someone else, an unloaded user, or a CR with no creator data", () => {
    expect(isChangeRequestCreator({ requestedBy: { id: "u-1" }, createdBy: "x" }, { id: "u-2", email: "b@x.com" })).toBe(false);
    expect(isChangeRequestCreator({ requestedBy: { id: "u-1" } }, undefined)).toBe(false);
    expect(isChangeRequestCreator({}, { id: "u-1" })).toBe(false);
    expect(isChangeRequestCreator({ requestedBy: null, createdBy: "" }, { id: "" })).toBe(false);
  });
});

describe("countActiveCRFilters", () => {
  it("is 0 for the default filters", () => {
    expect(countActiveCRFilters(DEFAULT_CR_FILTERS)).toBe(0);
  });

  it("is 1 when an SRE team filter is set", () => {
    expect(
      countActiveCRFilters({ ...DEFAULT_CR_FILTERS, sreTeamIds: ["team-apollo"] }),
    ).toBe(1);
  });

  it("is 1 when a project filter is set", () => {
    expect(
      countActiveCRFilters({ ...DEFAULT_CR_FILTERS, projectIds: ["proj-1"] }),
    ).toBe(1);
  });
});

describe("buildChangeRequestSearchFilters", () => {
  it("returns an empty object for the defaults with no search text", () => {
    expect(buildChangeRequestSearchFilters(DEFAULT_CR_FILTERS, "")).toEqual({});
  });

  it("includes states/impacts/closed-date bounds when set", () => {
    expect(
      buildChangeRequestSearchFilters(
        {
          ...DEFAULT_CR_FILTERS,
          states: ["implement"],
          impacts: ["high"],
          closedStartDate: "2026-01-01",
          closedEndDate: "2026-01-31",
        },
        "rollback",
      ),
    ).toEqual({
      searchQuery: "rollback",
      states: ["implement"],
      impacts: ["high"],
      closedStartDate: "2026-01-01T00:00:00Z",
      closedEndDate: "2026-01-31T23:59:59Z",
    });
  });

  it("sends selected SRE teams as an assignmentGroupId/in generic filter entry", () => {
    expect(
      buildChangeRequestSearchFilters(
        { ...DEFAULT_CR_FILTERS, sreTeamIds: ["team-apollo", "team-atlas"] },
        "",
      ),
    ).toEqual({
      filters: [{ field: "assignmentGroupId", op: "in", values: ["team-apollo", "team-atlas"] }],
    });
  });

  it("omits the generic filters array entirely when no SRE team is selected", () => {
    expect(buildChangeRequestSearchFilters(DEFAULT_CR_FILTERS, "")).not.toHaveProperty("filters");
  });

  it("sends selected projects as the named projectIds field", () => {
    expect(
      buildChangeRequestSearchFilters(
        { ...DEFAULT_CR_FILTERS, projectIds: ["proj-1", "proj-2"] },
        "",
      ),
    ).toEqual({ projectIds: ["proj-1", "proj-2"] });
  });

  it("omits projectIds entirely when no project is selected", () => {
    expect(buildChangeRequestSearchFilters(DEFAULT_CR_FILTERS, "")).not.toHaveProperty(
      "projectIds",
    );
  });
});

describe("change request category helpers", () => {
  it("offers the 13 ServiceNow categories, with 'other' as the default", () => {
    expect(CHANGE_REQUEST_CATEGORY_OPTIONS).toHaveLength(13);
    expect(DEFAULT_CHANGE_REQUEST_CATEGORY).toBe("other");
    expect(CHANGE_REQUEST_CATEGORY_OPTIONS.find((o) => o.value === "other")?.label).toBe("Other");
  });

  it("recognises only backend enum values", () => {
    expect(isChangeRequestCategory("devops")).toBe(true);
    expect(isChangeRequestCategory("Other")).toBe(false);
    expect(isChangeRequestCategory("")).toBe(false);
    expect(isChangeRequestCategory(undefined)).toBe(false);
  });

  it("reads the enum value off a detail category, whether it carries a name or a label", () => {
    expect(changeRequestCategoryValue({ id: "network", name: "Network" })).toBe("network");
    expect(changeRequestCategoryValue({ id: "network", label: "Network" })).toBe("network");
    expect(changeRequestCategoryValue("devops")).toBe("devops");
    expect(changeRequestCategoryValue({ id: "unknown" })).toBe("");
    expect(changeRequestCategoryValue("unknown")).toBe("");
    expect(changeRequestCategoryValue(null)).toBe("");
  });

  it("labels a category from the known list first, then from the backend, with a dash when absent", () => {
    expect(changeRequestCategoryLabel({ id: "regular_release_cloud", name: "x" })).toBe("Regular Release - Cloud");
    expect(changeRequestCategoryLabel({ id: "mystery", name: "Mystery" })).toBe("Mystery");
    expect(changeRequestCategoryLabel({ id: "mystery", label: "Old label" })).toBe("Old label");
    expect(changeRequestCategoryLabel("hotfix_release_cloud")).toBe("Hotfix Release - Cloud");
    expect(changeRequestCategoryLabel("Free text")).toBe("Free text");
    expect(changeRequestCategoryLabel(undefined)).toBe("—");
  });
});

describe("changeRequestScopeLockedReason", () => {
  it("is editable before implementation and locked from implement onwards", () => {
    for (const state of ["new", "assess", "authorize", "customer_approval", "scheduled"]) {
      expect(changeRequestScopeLockedReason(state)).toBeNull();
    }
    for (const state of ["implement", "review", "customer_review", "closed", "rollback", "canceled"]) {
      expect(changeRequestScopeLockedReason(state)).toMatch(/can't be changed/);
    }
    expect(changeRequestScopeLockedReason(undefined)).toBeNull();
  });
});

describe("changeRequestBlockingReason — customer group stages", () => {
  const customerStage = (stage: string, status = "REQUESTED"): BeChangeRequestApproval => ({
    stage,
    approverType: "STATIC_GROUP",
    approverName: "Acme Reviewers",
    status,
    approvers: [],
  });

  it.each([
    ["Customer Approval", "Awaiting Customer Approval"],
    ["Customer Review", "Awaiting Customer Review"],
    ["customer_review", "Awaiting Customer Review"],
  ])("names a waiting %s stage '%s', never 'approval approval'", (stage, expected) => {
    const reason = changeRequestBlockingReason([customerStage(stage)], "implement");
    expect(reason).toBe(expected);
    expect(reason).not.toMatch(/approval approval/i);
  });

  it("uses the same wording from the state whether or not a customer stage exists", () => {
    expect(changeRequestBlockingReason([customerStage("Customer Approval")], "customer_approval")).toBe(
      "Awaiting Customer Approval",
    );
    expect(changeRequestBlockingReason([], "customer_approval")).toBe("Awaiting Customer Approval");
    expect(changeRequestBlockingReason([customerStage("Customer Review")], "customer_review")).toBe(
      "Awaiting Customer Review",
    );
    expect(changeRequestBlockingReason([], "customer_review")).toBe("Awaiting Customer Review");
  });
});

describe("noCustomerContactsHelper", () => {
  it.each(["customer_approval", "customer_review"])(
    "returns the helper at %s when the project has no registered contacts",
    (state) => {
      expect(noCustomerContactsHelper(state, [])).toBe(NO_CUSTOMER_CONTACTS_HELPER);
      expect(noCustomerContactsHelper(state, null)).toBe(NO_CUSTOMER_CONTACTS_HELPER);
    },
  );

  it("is silent when the project has registered contacts", () => {
    expect(noCustomerContactsHelper("customer_approval", [{ id: "c1", name: "Alice" }])).toBeNull();
  });

  it("is silent when the payload carries no customerContacts field at all (unknown)", () => {
    expect(noCustomerContactsHelper("customer_approval", undefined)).toBeNull();
  });

  it("is silent outside the customer gates", () => {
    for (const state of ["new", "assess", "authorize", "scheduled", "implement", "review", "closed", "canceled", undefined]) {
      expect(noCustomerContactsHelper(state, [])).toBeNull();
    }
  });

  it("says what is going on", () => {
    expect(NO_CUSTOMER_CONTACTS_HELPER).toMatch(
      /^No registered customer contacts are assigned to this change request's project, so no customer approvers were assigned\./,
    );
  });
});
