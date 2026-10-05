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
// In-browser fake of the change-request slice of the backend contract, for
// specs that must walk an approval flow deterministically without creating
// permanent ServiceNow records. Installed with `page.route`, so only the
// change-request endpoints (and the user id returned by `/users/me`) are
// faked -- everything else still reaches the real backend the rest of the
// suite runs against, and the browser session is still the captured one.
//
// The contract encoded here is the one the UI is built against:
//   - `legalNextStates` offers no manual way to `scheduled` -- except from
//     `customer_approval`, where `scheduled` means "record the customer's
//     approval" -- and `authorize` (listed from Assess) is the approval path,
//     never a button;
//   - Request Approval (`PATCH {state:"assess"}`) on a Normal CR enters Assess
//     with a "Peer Approval" stage; on an Emergency CR it enters Authorize with
//     an "ECAB Approval" stage only; on a Standard CR it goes straight to
//     the post-approval state with no approvals;
//   - approving Peer Approval adds a "CAB Approval" stage and moves to
//     Authorize; approving CAB/ECAB moves the CR on by itself;
//   - "the post-approval state" is `customer_approval` when the CR has
//     `customerApprovalRequired`, else `scheduled`;
//   - from `customer_approval` legalNextStates = [scheduled, canceled];
//   - Review offers [customer_review, canceled] when `customerReviewRequired`,
//     else [closed, canceled]; `customer_review` -> [closed, canceled];
//   - `customerApprovalRequired` / `customerReviewRequired` are on the detail
//     response and editable via PATCH until their gate passes; a late edit is
//     refused with a 400 and a readable message;
//   - the CR's creator can never approve;
//   - the customer scope (see FAKE_PROJECTS & co. below): `POST /projects/search`
//     lists the fake projects, `POST /change-requests/link-options` answers the
//     Customer Project -> Deployments -> Environments / Deployment products
//     cascade, and `POST /change-requests` / `PATCH /change-requests/{id}` store
//     `projectId` / `deploymentIds` / `environmentIds` / `deploymentProductIds` /
//     `customerGroupId` / `category` / `comment` / `workNote` after the backend's
//     own validation -- deployments must belong to the project, environments must
//     be provided by a chosen deployment, deployment products must be exactly the
//     derived set (all else a 400 with a readable message), and the project /
//     deployments / environments are locked from `implement` onwards. The detail
//     response returns `project`, `deployments`, `environments`,
//     `deploymentProducts`, `customerGroup`, `category` (EntityRef / EntityRef[] /
//     the category enum value).
//


import type { Page, Route } from "@playwright/test";

export type FakeCrType = "normal" | "standard" | "emergency";

export interface FakeUser {
  id: string;
  name: string;
  email: string;
}

export const FAKE_CREATOR: FakeUser = { id: "00000000-0000-0000-0000-00000000e001", name: "Casey Creator", email: "casey.creator@example.com" };
export const FAKE_PEER: FakeUser = { id: "00000000-0000-0000-0000-00000000e002", name: "Pat Peer", email: "pat.peer@example.com" };
export const FAKE_CAB: FakeUser = { id: "00000000-0000-0000-0000-00000000e003", name: "Cam Cab", email: "cam.cab@example.com" };
export const FAKE_ECAB: FakeUser = { id: "00000000-0000-0000-0000-00000000e004", name: "Eli Ecab", email: "eli.ecab@example.com" };

export const FAKE_CR_ID = "00000000-0000-0000-0000-00000000c001";

// ---------------------------------------------------------------------------
// Customer scope fixtures: two projects, each with deployments that are
// instances of an environment, each deployment carrying deployed products.
// ---------------------------------------------------------------------------

export interface FakeRef {
  id: string;
  name: string;
}
export interface FakeDeployment extends FakeRef {
  projectId: string;
  type: string;
  environmentId: string;
}
export interface FakeDeploymentProduct extends FakeRef {
  deploymentId: string;
}

export const FAKE_PROJECTS: FakeRef[] = [
  { id: "00000000-0000-0000-0000-00000000f001", name: "Acme Project" },
  { id: "00000000-0000-0000-0000-00000000f002", name: "Beta Project" },
];
export const FAKE_ENVIRONMENTS: FakeRef[] = [
  { id: "00000000-0000-0000-0000-00000000e101", name: "Primary Production" },
  { id: "00000000-0000-0000-0000-00000000e102", name: "Staging" },
  { id: "00000000-0000-0000-0000-00000000e103", name: "Development" },
];
export const FAKE_DEPLOYMENTS: FakeDeployment[] = [
  { id: "00000000-0000-0000-0000-00000000d001", name: "Acme Production", projectId: FAKE_PROJECTS[0]!.id, type: "primary_production", environmentId: FAKE_ENVIRONMENTS[0]!.id },
  { id: "00000000-0000-0000-0000-00000000d002", name: "Acme Staging", projectId: FAKE_PROJECTS[0]!.id, type: "staging", environmentId: FAKE_ENVIRONMENTS[1]!.id },
  { id: "00000000-0000-0000-0000-00000000d003", name: "Beta Development", projectId: FAKE_PROJECTS[1]!.id, type: "development", environmentId: FAKE_ENVIRONMENTS[2]!.id },
];
export const FAKE_DEPLOYMENT_PRODUCTS: FakeDeploymentProduct[] = [
  { id: "00000000-0000-0000-0000-00000000b001", name: "API Manager 4.3.0", deploymentId: FAKE_DEPLOYMENTS[0]!.id },
  { id: "00000000-0000-0000-0000-00000000b002", name: "Identity Server 7.0.0", deploymentId: FAKE_DEPLOYMENTS[0]!.id },
  { id: "00000000-0000-0000-0000-00000000b003", name: "API Manager 4.2.0", deploymentId: FAKE_DEPLOYMENTS[1]!.id },
  { id: "00000000-0000-0000-0000-00000000b004", name: "Choreo 1.0.0", deploymentId: FAKE_DEPLOYMENTS[2]!.id },
];
export const FAKE_GROUPS: FakeRef[] = [
  { id: "00000000-0000-0000-0000-00000000a101", name: "Acme Customers" },
  { id: "00000000-0000-0000-0000-00000000a102", name: "Beta Customers" },
];

/** States from which project / deployments / environments can no longer change. */
const SCOPE_LOCKED = ["implement", "review", "customer_review", "closed", "rollback", "canceled"];

const CATEGORIES = [
  "hardware", "software", "service", "system_software", "applications_software", "network",
  "telecom", "documentation", "other", "regular_release_cloud", "hotfix_release_cloud", "devops", "cloud_computing",
];

/** The customer scope the fake CR currently holds. */
export interface FakeScope {
  projectId: string | null;
  deploymentIds: string[];
  environmentIds: string[];
  deploymentProductIds: string[];
  customerGroupId: string | null;
  category: string | null;
}

/** A request the fake served, with the JSON body it carried (if any). */
export interface FakeRequestBody {
  request: string;
  body: Record<string, unknown> | undefined;
}

const sameSet = (a: string[], b: string[]): boolean => a.length === b.length && a.every((x) => b.includes(x));
const derivedProductIds = (deploymentIds: string[]): string[] =>
  FAKE_DEPLOYMENT_PRODUCTS.filter((p) => deploymentIds.includes(p.deploymentId)).map((p) => p.id);
const environmentIdsOf = (deploymentIds: string[]): string[] => [
  ...new Set(FAKE_DEPLOYMENTS.filter((d) => deploymentIds.includes(d.id)).map((d) => d.environmentId)),
];

interface Approver {
  id: string;
  name: string;
  status: string;
}
interface Stage {
  stage: string;
  approverType: "STATIC_GROUP";
  approverName: string;
  status: string;
  approvers: Approver[];
}

/** The two ServiceNow-style creation checkboxes the CR carries. */
export interface FakeCustomerFlags {
  customerApprovalRequired: boolean;
  customerReviewRequired: boolean;
}

/** States from which each checkbox can no longer be changed (backend refuses with 400). */
const APPROVAL_FLAG_LOCKED = ["customer_approval", "scheduled", "implement", "review", "customer_review", "closed", "rollback", "canceled"];
const REVIEW_FLAG_LOCKED = ["customer_review", "closed", "rollback", "canceled"];

export interface FakeChangeRequestApi {
  /** Who the app believes is signed in (applied on the next page load). */
  setViewer(user: FakeUser): void;
  /** The CR's current lifecycle state, as the fake backend holds it. */
  state(): string;
  /** The CR's current checkbox settings, as the fake backend holds them. */
  flags(): FakeCustomerFlags;
  /** Moves the fake CR to `next` out-of-band (e.g. while an edit dialog is
   * still open on a stale copy), without touching its approval stages. */
  setState(next: string): void;
  /** Every request the fake served, as "METHOD /path". */
  requests(): string[];
  /** Every request the fake served that carried a JSON body, in order. */
  requestBodies(): FakeRequestBody[];
  /** The customer scope the fake CR currently holds (as stored, ids only). */
  scope(): FakeScope;
  /** The `comment` / `workNote` journal entries the fake received, in order. */
  journal(): Array<{ kind: "comment" | "workNote"; text: string }>;
  /**
   * Deactivates a deployment server-side, behind the form's back: it drops out
   * of `link-options` and any create / PATCH that still names it is refused
   * with a 400 -- the "stale options" scenario.
   */
  retireDeployment(deploymentId: string): void;
}

function legalNextStates(state: string, flags: FakeCustomerFlags): string[] {
  switch (state) {
    case "new":
      return ["assess", "canceled"];
    case "assess":
      return ["authorize", "canceled"]; // authorize = the approval path, never a button
    case "authorize":
      return ["canceled"];
    case "customer_approval":
      return ["scheduled", "canceled"]; // scheduled = "Record customer approval"
    case "scheduled":
      return ["implement", "canceled"];
    case "implement":
      return ["review", "canceled"];
    case "review":
      return flags.customerReviewRequired ? ["customer_review", "canceled"] : ["closed", "canceled"];
    case "customer_review":
      return ["closed", "canceled"];
    default:
      return [];
  }
}

const nextStage = (name: string, group: string, who: FakeUser): Stage => ({
  stage: name,
  approverType: "STATIC_GROUP",
  approverName: group,
  status: "REQUESTED",
  approvers: [{ id: who.id, name: who.name, status: "REQUESTED" }],
});

export async function installFakeChangeRequestApi(
  page: Page,
  initialType: FakeCrType,
  viewer: FakeUser = FAKE_CREATOR,
  initialFlags: Partial<FakeCustomerFlags> = {},
): Promise<FakeChangeRequestApi> {
  let type = initialType;
  let currentViewer = viewer;
  let state = "new";
  let subject = "[E2E] approval flow (mocked)";
  const scope: FakeScope = {
    projectId: null,
    deploymentIds: [],
    environmentIds: [],
    deploymentProductIds: [],
    customerGroupId: null,
    category: null,
  };
  const journal: Array<{ kind: "comment" | "workNote"; text: string }> = [];
  const retired = new Set<string>();
  const bodies: FakeRequestBody[] = [];
  const flags: FakeCustomerFlags = {
    customerApprovalRequired: initialFlags.customerApprovalRequired ?? false,
    customerReviewRequired: initialFlags.customerReviewRequired ?? false,
  };
  /** Where a CR lands once its internal approval is granted. */
  const afterInternalApproval = (): string => (flags.customerApprovalRequired ? "customer_approval" : "scheduled");
  let stages: Stage[] = [];
  const log: string[] = [];

  const detail = (): Record<string, unknown> => ({
    id: FAKE_CR_ID,
    number: "CHG0099001",
    subject,
    createdOn: "2026-01-01T00:00:00Z",
    createdBy: FAKE_CREATOR.email,
    state,
    type,
    assignedTeam: { id: "00000000-0000-0000-0000-00000000a001", name: "Platform" },
    requestedBy: { id: FAKE_CREATOR.id, name: FAKE_CREATOR.name },
    customerApprovalRequired: flags.customerApprovalRequired,
    customerReviewRequired: flags.customerReviewRequired,
    legalNextStates: legalNextStates(state, flags),
    project: FAKE_PROJECTS.find((p) => p.id === scope.projectId),
    deployments: FAKE_DEPLOYMENTS.filter((d) => scope.deploymentIds.includes(d.id)).map(({ id, name }) => ({ id, name })),
    environments: FAKE_ENVIRONMENTS.filter((e) => scope.environmentIds.includes(e.id)),
    deploymentProducts: FAKE_DEPLOYMENT_PRODUCTS.filter((p) => scope.deploymentProductIds.includes(p.id)).map(({ id, name }) => ({ id, name })),
    customerGroup: FAKE_GROUPS.find((g) => g.id === scope.customerGroupId) ?? null,
    category: scope.category,
  });

  /**
   * The backend's validation of a create / PATCH body's customer scope, applied to
   * `next` (the scope as it would be after the write). Returns the 400 message,
   * or null when the combination is consistent.
   */
  const validateScope = (
    body: Record<string, unknown>,
    next: FakeScope,
    isPatch: boolean,
  ): string | null => {
    if (typeof body.category === "string" && !CATEGORIES.includes(body.category)) {
      return `category must be one of ${CATEGORIES.join(", ")}`;
    }
    if (isPatch) {
      const touchesScope =
        (body.projectId !== undefined && body.projectId !== scope.projectId) ||
        (body.deploymentIds !== undefined && !sameSet(body.deploymentIds as string[], scope.deploymentIds)) ||
        (body.environmentIds !== undefined && !sameSet(body.environmentIds as string[], scope.environmentIds));
      if (touchesScope && SCOPE_LOCKED.includes(state)) {
        return `projectId, deploymentIds and environmentIds can no longer be changed once the change request is ${state}`;
      }
      if (body.projectId !== undefined && body.projectId !== scope.projectId && scope.deploymentIds.length > 0 && body.deploymentIds === undefined) {
        return "changing projectId while deployments are stored requires deploymentIds in the same request";
      }
    }
    if (next.deploymentIds.length > 0 && !next.projectId) {
      return "deploymentIds requires projectId";
    }
    if (next.projectId && !FAKE_PROJECTS.some((p) => p.id === next.projectId)) {
      return `projectId: project ${next.projectId} not found`;
    }
    for (const id of next.deploymentIds) {
      const d = FAKE_DEPLOYMENTS.find((x) => x.id === id);
      if (!d) return `deploymentIds: deployment ${id} not found`;
      if (d.projectId !== next.projectId || retired.has(d.id)) {
        return `deploymentIds: deployment ${d.name} is not an active deployment of the selected project`;
      }
    }
    const provided = environmentIdsOf(next.deploymentIds);
    for (const id of next.environmentIds) {
      if (!provided.includes(id)) {
        const name = FAKE_ENVIRONMENTS.find((e) => e.id === id)?.name ?? id;
        return `environmentIds: environment ${name} is not provided by any of the selected deployments`;
      }
    }
    if (body.deploymentProductIds !== undefined && !sameSet(body.deploymentProductIds as string[], derivedProductIds(next.deploymentIds))) {
      return "deploymentProductIds: deployment products are derived from the selected deployments and must be exactly that set";
    }
    return null;
  };

  /** The scope after applying the scope fields of `body` on top of the stored one. */
  const applyScope = (body: Record<string, unknown>, base: FakeScope, isPatch: boolean): FakeScope => {
    const next: FakeScope = { ...base, deploymentIds: [...base.deploymentIds], environmentIds: [...base.environmentIds] };
    if (body.projectId !== undefined) next.projectId = body.projectId as string;
    if (body.deploymentIds !== undefined) next.deploymentIds = body.deploymentIds as string[];
    if (body.environmentIds !== undefined) {
      next.environmentIds = body.environmentIds as string[];
    } else if (!isPatch || body.deploymentIds !== undefined) {
      // Omitted environments default to those of the chosen deployments.
      next.environmentIds = environmentIdsOf(next.deploymentIds);
    }
    next.deploymentProductIds = derivedProductIds(next.deploymentIds);
    if (body.customerGroupId !== undefined) next.customerGroupId = body.customerGroupId as string | null;
    if (body.category !== undefined) next.category = body.category as string | null;
    return next;
  };

  const cors = (route: Route): Record<string, string> => ({
    "access-control-allow-origin": route.request().headers()["origin"] ?? "*",
    "access-control-allow-headers": "*",
    "access-control-allow-methods": "GET,POST,PATCH,PUT,DELETE,OPTIONS",
    "access-control-allow-credentials": "true",
  });
  const json = (route: Route, body: unknown, status = 200): Promise<void> =>
    route.fulfill({
      status,
      contentType: "application/json",
      headers: cors(route),
      body: JSON.stringify(body),
    });

  // Same identity swap for /users/me: keep the real profile (roles, time
  // zone, ...) but make the signed-in user one of the fake people above.
  await page.route(
    (url) => url.pathname.endsWith("/users/me"),
    async (route) => {
      const type = route.request().resourceType();
      if (route.request().method() !== "GET" || (type !== "fetch" && type !== "xhr")) return route.fallback();
      const real = await route.fetch();
      const profile = (await real.json()) as Record<string, unknown>;
      const [firstName, ...rest] = currentViewer.name.split(" ");
      await route.fulfill({
        response: real,
        json: { ...profile, id: currentViewer.id, email: currentViewer.email, firstName, lastName: rest.join(" ") },
      });
    },
  );

  /** Common prologue for the endpoints below: only XHR/fetch, answers CORS preflights. */
  const isApiCall = async (route: Route): Promise<boolean> => {
    const req = route.request();
    const rtype = req.resourceType();
    if (rtype !== "fetch" && rtype !== "xhr") {
      await route.fallback();
      return false;
    }
    if (req.method() === "OPTIONS") {
      await route.fulfill({ status: 204, headers: cors(route) });
      return false;
    }
    return true;
  };
  const bodyOf = (route: Route): Record<string, unknown> | undefined => {
    try {
      return (route.request().postDataJSON() as Record<string, unknown> | null) ?? undefined;
    } catch {
      return undefined;
    }
  };
  const record = (route: Route, label: string): Record<string, unknown> | undefined => {
    log.push(label);
    const body = bodyOf(route);
    bodies.push({ request: label, body });
    return body;
  };

  // Customer Project picker.
  await page.route(
    (url) => url.pathname.endsWith("/projects/search"),
    async (route) => {
      if (route.request().method() !== "POST") return route.fallback();
      if (!(await isApiCall(route))) return;
      const body = record(route, "POST /projects/search");
      const q = String((body?.searchQuery as string | undefined) ?? "").toLowerCase();
      const projects = FAKE_PROJECTS.filter((p) => p.name.toLowerCase().includes(q));
      return json(route, { projects, hasMore: false, totalRecords: projects.length });
    },
  );

  // Customer Group picker.
  await page.route(
    (url) => url.pathname.endsWith("/groups/search"),
    async (route) => {
      if (route.request().method() !== "POST") return route.fallback();
      if (!(await isApiCall(route))) return;
      const body = record(route, "POST /groups/search");
      const q = String(((body?.filters as { searchQuery?: string } | undefined)?.searchQuery) ?? "").toLowerCase();
      const groups = FAKE_GROUPS.filter((g) => g.name.toLowerCase().includes(q)).map((g) => ({ ...g, active: true }));
      return json(route, { groups, total: groups.length, limit: 20, offset: 0 });
    },
  );

  // The Customer Project -> Deployments -> Environments / Deployment products cascade.
  await page.route(
    (url) => url.pathname.endsWith("/change-requests/link-options"),
    async (route) => {
      if (route.request().method() !== "POST") return route.fallback();
      if (!(await isApiCall(route))) return;
      const body = record(route, "POST /change-requests/link-options") ?? {};
      const projectId = body.projectId as string | undefined;
      if (!projectId) return json(route, { message: "projectId is required" }, 400);
      const chosen = (body.deploymentIds as string[] | undefined) ?? [];
      const projectDeployments = FAKE_DEPLOYMENTS.filter((d) => d.projectId === projectId && !retired.has(d.id));
      const stray = chosen.find((id) => !projectDeployments.some((d) => d.id === id));
      if (stray) return json(route, { message: `deployment ${stray} does not belong to the selected project` }, 400);
      return json(route, {
        deployments: projectDeployments.map((d) => ({
          id: d.id,
          name: d.name,
          type: d.type,
          environment: FAKE_ENVIRONMENTS.find((e) => e.id === d.environmentId) ?? null,
        })),
        environments: FAKE_ENVIRONMENTS.filter((e) => environmentIdsOf(chosen).includes(e.id)),
        deploymentProducts: FAKE_DEPLOYMENT_PRODUCTS.filter((p) => chosen.includes(p.deploymentId)).map((p) => ({
          id: p.id,
          name: p.name,
          deployment: FAKE_DEPLOYMENTS.filter((d) => d.id === p.deploymentId).map(({ id, name }) => ({ id, name }))[0],
        })),
      });
    },
  );

  // Create. The fake holds exactly one CR (FAKE_CR_ID); creating "creates" it.
  await page.route(
    (url) => url.pathname.endsWith("/change-requests"),
    async (route) => {
      if (route.request().method() !== "POST") return route.fallback();
      if (!(await isApiCall(route))) return;
      const body = record(route, "POST /change-requests") ?? {};
      if (!["normal", "standard", "emergency"].includes(body.type as string)) {
        return json(route, { message: "type is required: a change request must be one of standard, normal or emergency" }, 400);
      }
      const next = applyScope(body, scope, false);
      const problem = validateScope(body, next, false);
      if (problem) return json(route, { message: problem }, 400);
      Object.assign(scope, next);
      type = body.type as FakeCrType;
      state = "new";
      subject = String(body.subject ?? subject);
      if (body.customerApprovalRequired !== undefined) flags.customerApprovalRequired = body.customerApprovalRequired as boolean;
      if (body.customerReviewRequired !== undefined) flags.customerReviewRequired = body.customerReviewRequired as boolean;
      for (const kind of ["comment", "workNote"] as const) {
        const text = body[kind];
        if (typeof text === "string" && text.trim()) journal.push({ kind, text });
      }
      return json(
        route,
        { message: "Change request created.", changeRequest: { id: FAKE_CR_ID, number: "CHG0099001", createdOn: "2026-01-01T00:00:00Z", createdBy: currentViewer.email } },
        201,
      );
    },
  );

  await page.route(
    (url) => new RegExp(`/change-requests/${FAKE_CR_ID}(/.*)?$`).test(url.pathname),
    async (route) => {
      const req = route.request();
      const rtype = req.resourceType();
      if (rtype !== "fetch" && rtype !== "xhr") return route.fallback();
      if (req.method() === "OPTIONS") return route.fulfill({ status: 204, headers: cors(route) });

      const path = new URL(req.url()).pathname.replace(/^.*\/change-requests\//, "/change-requests/");
      record(route, `${req.method()} ${path}`);

      if (path.endsWith("/approvals/decision") && req.method() === "POST") {
        const { decision } = req.postDataJSON() as { decision: "approved" | "rejected" };
        const current = stages.find((s) => s.status === "REQUESTED");
        const row = current?.approvers.find((a) => a.id === currentViewer.id && a.status === "REQUESTED");
        if (!current || !row || currentViewer.id === FAKE_CREATOR.id) {
          return json(route, { message: "Access to the requested resource is forbidden!" }, 403);
        }
        row.status = decision === "approved" ? "APPROVED" : "REJECTED";
        current.status = row.status;
        if (decision === "approved") {
          if (current.stage === "Peer Approval") {
            state = "authorize";
            stages = [...stages, nextStage("CAB Approval", "CAB", FAKE_CAB)];
          } else {
            state = afterInternalApproval(); // CAB / ECAB approval moves the CR on itself
          }
        }
        return json(route, { id: FAKE_CR_ID, state });
      }
      if (path.endsWith("/approvals") && req.method() === "GET") {
        // Like the Postgres-backed API: `canDecide` is true only on the
        // caller's own REQUESTED row, and never for the CR's creator.
        return json(route, {
          approvals: stages.map((st) => ({
            ...st,
            approvers: st.approvers.map((a) => ({
              ...a,
              canDecide: a.id === currentViewer.id && a.status === "REQUESTED" && currentViewer.id !== FAKE_CREATOR.id,
            })),
          })),
        });
      }
      if (path.endsWith("/comments/search")) {
        return json(route, { comments: [], hasMore: false, totalRecords: 0 });
      }
      if (req.method() === "PATCH") {
        const body = req.postDataJSON() as {
          state?: string;
          customerApprovalRequired?: boolean;
          customerReviewRequired?: boolean;
        } & Record<string, unknown>;
        // Customer scope / category / customer group, validated like the backend.
        const touchesScopeFields = ["projectId", "deploymentIds", "environmentIds", "deploymentProductIds", "customerGroupId", "category"].some(
          (k) => body[k] !== undefined,
        );
        if (touchesScopeFields) {
          const next = applyScope(body, scope, true);
          const problem = validateScope(body, next, true);
          if (problem) return json(route, { message: problem }, 400);
          Object.assign(scope, next);
        }
        for (const kind of ["comment", "workNote"] as const) {
          const text = body[kind];
          if (typeof text === "string" && text.trim()) journal.push({ kind, text });
        }
        // Checkbox edits: refused once the gate they control has passed.
        if (body.customerApprovalRequired !== undefined) {
          if (APPROVAL_FLAG_LOCKED.includes(state)) {
            return json(route, { message: `customerApprovalRequired cannot be changed once the change request is ${state}` }, 400);
          }
          flags.customerApprovalRequired = body.customerApprovalRequired;
        }
        if (body.customerReviewRequired !== undefined) {
          if (REVIEW_FLAG_LOCKED.includes(state)) {
            return json(route, { message: `customerReviewRequired cannot be changed once the change request is ${state}` }, 400);
          }
          flags.customerReviewRequired = body.customerReviewRequired;
        }
        const target = body.state;
        if (target === undefined) {
          return json(route, { id: FAKE_CR_ID, state, message: "Change request updated.", changeRequest: detail() });
        }
        if (target === "assess") {
          if (type === "standard") state = afterInternalApproval();
          else if (type === "emergency") {
            state = "authorize";
            stages = [nextStage("ECAB Approval", "ECAB", FAKE_ECAB)];
          } else {
            state = "assess";
            stages = [nextStage("Peer Approval", "Peers", FAKE_PEER)];
          }
        } else if (target === "scheduled" && state === "customer_approval") {
          state = "scheduled"; // the customer's approval was recorded
        } else if (target !== "scheduled" && target !== "authorize" && target !== "customer_approval") {
          if (!legalNextStates(state, flags).includes(target)) {
            return json(route, { message: `Illegal transition from ${state} to ${target}.` }, 400);
          }
          state = target;
        } else {
          return json(route, { message: `Illegal transition to ${String(target)}.` }, 400);
        }
        return json(route, { id: FAKE_CR_ID, state });
      }
      if (req.method() === "GET" && path === `/change-requests/${FAKE_CR_ID}`) {
        return json(route, detail());
      }
      return route.fallback();
    },
  );

  return {
    setViewer: (user) => {
      currentViewer = user;
    },
    state: () => state,
    setState: (next) => {
      state = next;
    },
    flags: () => ({ ...flags }),
    requests: () => [...log],
    requestBodies: () => [...bodies],
    scope: () => ({ ...scope, deploymentIds: [...scope.deploymentIds], environmentIds: [...scope.environmentIds], deploymentProductIds: [...scope.deploymentProductIds] }),
    journal: () => [...journal],
    retireDeployment: (deploymentId) => {
      retired.add(deploymentId);
    },
  };
}
