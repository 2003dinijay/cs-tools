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
// What the STATE-CHANGING local specs need beyond a customer's browser session:
// a way to put the seeded change requests back, a way to read what the stack
// really recorded, and the two other parties of a change request's life — the
// customer's own API calls and WSO2 staff deciding in the CSM portal.
//
// LOCAL STACK ONLY (docker-compose + the mock identity provider). Nothing here
// talks to a deployed environment.
//
// Where the pieces come from
//   - The customer backend and the identity provider are READ FROM THE APP UNDER
//     TEST: the webapp serves them in /config.js, so a spec can never aim its API
//     calls at another stack than the browser it drives.
//   - The database and the CSM BFF cannot be derived, so they are named in the
//     environment, and a spec that needs one SKIPS (never fails, never guesses a
//     default) when it is unset. That is deliberate: re-seeding rewrites rows, so
//     a default container name could silently reset somebody else's stack.
//       E2E_POSTGRES_CONTAINER   Docker container of the stack's Postgres
//                                (isolated stack: csmenv-postgres-1)
//       E2E_CSM_BFF_URL          the CSM portal backend as the browser reaches it
//                                (isolated stack: http://localhost:18082)
//       E2E_POSTGRES_USER / E2E_POSTGRES_DB   default postgres / csm_platform
//   - After every reset the helpers read the fixtures back THROUGH THE APP's
//     backend and fail loudly if they are not in their starting state, which is
//     what catches an E2E_POSTGRES_CONTAINER that belongs to a different stack.
//

import { spawn } from "node:child_process";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import type { test } from "@playwright/test";
import { EXAMPLE_CORP_PROJECT_ID, LOCAL_PERSONAS, type LocalPersona } from "../auth/localSessions";

/** The seeded change requests the specs drive (scripts/csm-compose/seed-entity-service.sql). */
export const FIXTURES = {
  projectId: EXAMPLE_CORP_PROJECT_ID,
  /** Normal change in Customer Approval; dave and erin are asked. */
  approval: { id: "00000000-0000-0000-0000-000000001303", number: "CHG-FIXED-007" },
  /** Change in Customer Review; dave and erin are asked. */
  review: { id: "00000000-0000-0000-0000-000000001304", number: "CHG-FIXED-008" },
  /** Normal change in Review with Customer Review ticked (not asked of anyone yet). */
  inReview: { id: "00000000-0000-0000-0000-000000001202", number: "CHG-FIXED-006" },
  /** Standard change in New with Customer Approval ticked. */
  standardNew: { id: "00000000-0000-0000-0000-000000001201", number: "CHG-FIXED-005" },
} as const;

export type FixtureChange = { id: string; number: string };

/** The staff persona that decides internal stages (a CAB / ECAB / peer approver). */
export const STAFF_APPROVERS = {
  alice: "alice.perera@example.com",
  bob: "bob.fernando@example.com",
  carol: "carol.silva@example.com",
} as const;

const SEED_FILE = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "../../../../../../scripts/csm-compose/seed-entity-service.sql",
);

/** The origin the run targets (same default as playwright.config.ts). */
function appOrigin(): string {
  return new URL(process.env.E2E_BASE_URL ?? "http://localhost:3000").origin;
}

// --- The app under test: where its backend and identity provider are -------------

export type StackEndpoints = {
  /** Customer backend (the BFF the webapp calls). */
  customerApi: string;
  /** The mock OIDC provider. */
  oidc: string;
  /** The webapp's own client id. */
  customerClientId: string;
};

let endpoints: Promise<StackEndpoints> | undefined;

/**
 * Reads the backend and identity-provider origins from the webapp's own
 * `/config.js`, so the API calls of a spec go to the very stack its browser does.
 */
export function stackEndpoints(): Promise<StackEndpoints> {
  endpoints ??= (async () => {
    const response = await fetch(`${appOrigin()}/config.js`, {
      signal: AbortSignal.timeout(10_000),
    });
    const text = await response.text();
    const read = (key: string): string => {
      const match = new RegExp(`${key}\\s*:\\s*"([^"]+)"`).exec(text);
      if (!match) throw new Error(`${appOrigin()}/config.js has no ${key}`);
      return match[1].replace(/\/+$/, "");
    };
    return {
      customerApi: read("CUSTOMER_PORTAL_BACKEND_BASE_URL"),
      oidc: read("CUSTOMER_PORTAL_AUTH_BASE_URL"),
      customerClientId: read("CUSTOMER_PORTAL_AUTH_CLIENT_ID"),
    };
  })();
  return endpoints;
}

// --- Tokens (the mock identity provider signs in any email, no credential) ---------

const FORM = { "content-type": "application/x-www-form-urlencoded" };
const tokenCache = new Map<string, { token: string; expiresAt: number }>();

/**
 * An access token from the mock provider for `email` as `clientId`, by the same
 * authorization-code exchange the apps perform (no PKCE).
 */
async function mintAccessToken(
  email: string,
  clientId: string,
  groups: string,
): Promise<string> {
  const key = `${clientId}|${email}|${groups}`;
  const cached = tokenCache.get(key);
  if (cached && cached.expiresAt > Date.now()) return cached.token;

  const { oidc } = await stackEndpoints();
  const redirectUri = appOrigin();
  const authorize = await fetch(`${oidc}/oauth2/authorize`, {
    method: "POST",
    redirect: "manual",
    headers: FORM,
    body: new URLSearchParams({
      client_id: clientId,
      redirect_uri: redirectUri,
      state: "e2e",
      scope: "openid",
      email,
      groups,
    }),
  });
  const location = authorize.headers.get("location");
  const code = location ? new URL(location).searchParams.get("code") : null;
  if (!code) {
    throw new Error(`the identity provider at ${oidc} gave no code for ${email} (HTTP ${authorize.status})`);
  }
  const token = await fetch(`${oidc}/oauth2/token`, {
    method: "POST",
    headers: FORM,
    body: new URLSearchParams({
      grant_type: "authorization_code",
      code,
      client_id: clientId,
      redirect_uri: redirectUri,
    }),
  });
  const body = (await token.json()) as { access_token?: string };
  if (!token.ok || !body.access_token) {
    throw new Error(`the identity provider at ${oidc} gave no token for ${email} (HTTP ${token.status})`);
  }
  // Tokens live an hour; reuse one for at most ten minutes.
  tokenCache.set(key, { token: body.access_token, expiresAt: Date.now() + 10 * 60_000 });
  return body.access_token;
}

export type ApiResult<T = unknown> = { status: number; body: T };

async function call<T>(
  method: string,
  url: string,
  token: string,
  body?: unknown,
): Promise<ApiResult<T>> {
  const response = await fetch(url, {
    method,
    headers: {
      authorization: `Bearer ${token}`,
      ...(body === undefined ? {} : { "content-type": "application/json" }),
    },
    body: body === undefined ? undefined : JSON.stringify(body),
    signal: AbortSignal.timeout(30_000),
  });
  const text = await response.text();
  let parsed: unknown = text;
  try {
    parsed = text ? JSON.parse(text) : null;
  } catch {
    /* not JSON: keep the text */
  }
  return { status: response.status, body: parsed as T };
}

// --- A customer's own API calls ----------------------------------------------------

/** What the customer API says about a change request (the part the specs read). */
export type CustomerChangeRequest = {
  id: string;
  number: string;
  state?: { id?: string; label?: string } | null;
  customerCanAnswer?: boolean;
  /** Whether WSO2 holds the change (the reason is never sent to a customer). */
  isOnHold?: boolean;
  hasCustomerApproved?: boolean;
  startDate?: string | null;
  endDate?: string | null;
  title?: string;
};

/**
 * Calls the customer backend as a seeded customer, exactly as the webapp does
 * (their bearer token; no groups, so a customer and nothing else).
 */
export function customerApi(persona: LocalPersona) {
  const token = async () => {
    const { customerClientId } = await stackEndpoints();
    return mintAccessToken(LOCAL_PERSONAS[persona].email, customerClientId, "");
  };
  return {
    /** `GET /change-requests/{id}`. */
    async get(changeRequestId: string): Promise<ApiResult<CustomerChangeRequest>> {
      const { customerApi: base } = await stackEndpoints();
      return call("GET", `${base}/change-requests/${changeRequestId}`, await token());
    },
    /** `PATCH /change-requests/{id}`: whatever body the caller wants to try. */
    async patch(changeRequestId: string, body: unknown): Promise<ApiResult> {
      const { customerApi: base } = await stackEndpoints();
      return call("PATCH", `${base}/change-requests/${changeRequestId}`, await token(), body);
    },
    /** `GET /change-requests/{id}/approvals`. */
    async approvals(changeRequestId: string): Promise<ApiResult<{ approvals?: { stage?: string }[] }>> {
      const { customerApi: base } = await stackEndpoints();
      return call("GET", `${base}/change-requests/${changeRequestId}/approvals`, await token());
    },
    /** `POST /change-requests/{id}/approvals/decision`: whatever body the caller wants to try. */
    async decision(changeRequestId: string, body: unknown): Promise<ApiResult> {
      const { customerApi: base } = await stackEndpoints();
      return call("POST", `${base}/change-requests/${changeRequestId}/approvals/decision`, await token(), body);
    },
    /** `POST /projects/{id}/change-requests/search`: the numbers the project lists. */
    async listedNumbers(projectId: string, filters: Record<string, unknown> = {}): Promise<string[]> {
      const { customerApi: base } = await stackEndpoints();
      const result = await call<{ changeRequests?: { number?: string }[] }>(
        "POST",
        `${base}/projects/${projectId}/change-requests/search`,
        await token(),
        // 50 is the most the API takes per page (a larger limit answers 400, "limit cannot exceed 50").
        { filters, pagination: { offset: 0, limit: 50 } },
      );
      // A project the caller may not read answers 403/404 and lists nothing; any other refusal (a 400 for the
      // page size, say) must not read as "an empty list", or the assertions on what is NOT listed pass vacuously.
      if (result.status === 403 || result.status === 404) return [];
      if (result.status !== 200) {
        throw new Error(`listing ${projectId}'s change requests answered ${result.status}: ${JSON.stringify(result.body)}`);
      }
      return (result.body.changeRequests ?? []).flatMap((c) => (c.number ? [c.number] : []));
    },
    /** The id of the project this customer is a contact of, found by name (generated projects have random ids). */
    async projectIdByName(name: string): Promise<string | undefined> {
      const { customerApi: base } = await stackEndpoints();
      const result = await call<{ projects?: { id: string; name: string }[] }>(
        "POST",
        `${base}/projects/search`,
        await token(),
        {},
      );
      return result.body.projects?.find((p) => p.name === name)?.id;
    },
  };
}

// --- WSO2 staff, deciding internal approvals through the CSM portal's backend -------

/** The CSM portal backend, as named by E2E_CSM_BFF_URL; undefined when unset. */
export function csmBffUrl(): string | undefined {
  return process.env.E2E_CSM_BFF_URL?.trim().replace(/\/+$/, "") || undefined;
}

/**
 * A staff member decides their pending approval on a change request — the same
 * call the CSM portal's Approvals tab makes (`POST /change-requests/{id}/approvals/decision`).
 *
 * @param email - A seeded approver (see {@link STAFF_APPROVERS}).
 * @param changeRequestId - The change request.
 * @param decision - approved / rejected.
 */
export async function decideAsStaff(
  email: string,
  changeRequestId: string,
  decision: "approved" | "rejected",
): Promise<ApiResult> {
  const bff = csmBffUrl();
  if (!bff) throw new Error("E2E_CSM_BFF_URL is not set");
  // The seeded engineers' group: the CSM portal's own sign-in pre-fills it.
  const token = await mintAccessToken(email, "csm-portal-webapp", "cs_engineer");
  return call("POST", `${bff}/change-requests/${changeRequestId}/approvals/decision`, token, { decision });
}

/**
 * A staff member changes a change request through the CSM portal's backend
 * (`PATCH /change-requests/{id}`), e.g. `{ state: "assess" }`, the one human action
 * out of New that the CSM portal calls "Request Approval".
 *
 * @param email - A seeded staff persona (see {@link STAFF_APPROVERS}).
 * @param changeRequestId - The change request.
 * @param body - The PATCH body.
 */
export async function patchAsStaff(
  email: string,
  changeRequestId: string,
  body: unknown,
): Promise<ApiResult> {
  const bff = csmBffUrl();
  if (!bff) throw new Error("E2E_CSM_BFF_URL is not set");
  const token = await mintAccessToken(email, "csm-portal-webapp", "cs_engineer");
  return call("PATCH", `${bff}/change-requests/${changeRequestId}`, token, body);
}

// --- The stack's Postgres (the isolated stack's, named by the environment) ----------

/** The Postgres container named by E2E_POSTGRES_CONTAINER; undefined when unset. */
export function postgresContainer(): string | undefined {
  return process.env.E2E_POSTGRES_CONTAINER?.trim() || undefined;
}

/**
 * Runs SQL in the stack's Postgres (psql inside its container) and resolves with
 * what it printed (unaligned, tuples only). The SQL goes in on stdin so the whole
 * seed file fits.
 */
export async function psql(sql: string): Promise<string> {
  const container = postgresContainer();
  if (!container) throw new Error("E2E_POSTGRES_CONTAINER is not set");
  const user = process.env.E2E_POSTGRES_USER ?? "postgres";
  const db = process.env.E2E_POSTGRES_DB ?? "csm_platform";
  return await new Promise<string>((resolve, reject) => {
    const child = spawn("docker", [
      "exec", "-i", container, "psql", "-U", user, "-d", db,
      "-v", "ON_ERROR_STOP=1", "-q", "-t", "-A", "-f", "-",
    ]);
    let stdout = "";
    let stderr = "";
    child.stdout.on("data", (d: Buffer) => (stdout += d.toString()));
    child.stderr.on("data", (d: Buffer) => (stderr += d.toString()));
    child.on("error", reject);
    child.on("close", (code) =>
      code === 0 ? resolve(stdout.trim()) : reject(new Error(`psql exited ${code}: ${stderr}`)),
    );
    child.stdin.end(sql);
  });
}

/** Whether the named Postgres container is running (false when docker is absent). */
async function containerRunning(): Promise<boolean> {
  const container = postgresContainer();
  if (!container) return false;
  return await new Promise<boolean>((resolve) => {
    const child = spawn("docker", ["inspect", "-f", "{{.State.Running}}", container]);
    let out = "";
    child.stdout.on("data", (d: Buffer) => (out += d.toString()));
    child.on("error", () => resolve(false));
    child.on("close", (code) => resolve(code === 0 && out.trim() === "true"));
  });
}

/**
 * Makes a spec SKIP, with the reason, unless it can reset the seeded fixtures:
 * E2E_POSTGRES_CONTAINER names a running container. Call it at the top of every
 * spec file that moves a fixture on.
 *
 * @param t - The `test` object of the spec file.
 * @param alsoNeedsCsmBff - True for specs in which WSO2 staff decide something.
 */
export function withFixtureStack(t: typeof test, alsoNeedsCsmBff = false): void {
  t.beforeEach(async () => {
    t.skip(
      !postgresContainer(),
      "This spec moves seeded change requests on and puts them back by re-running the seed, so it " +
        "needs the Postgres of the stack under test named explicitly: E2E_POSTGRES_CONTAINER=<container> " +
        "(the isolated stack's is csmenv-postgres-1). It never guesses one.",
    );
    t.skip(
      !(await containerRunning()),
      `E2E_POSTGRES_CONTAINER=${postgresContainer()} is not a running container (is docker up?).`,
    );
    if (alsoNeedsCsmBff) {
      t.skip(
        !csmBffUrl(),
        "This spec has WSO2 staff decide the internal approval through the CSM portal's backend: " +
          "set E2E_CSM_BFF_URL (the isolated stack's is http://localhost:18082).",
      );
    }
  });

  // Leave the stack as it was found: the fixtures back in their starting state, so
  // the read-only smoke spec (and the next person) finds CHG-FIXED-007 waiting.
  // Guarded the same way, since afterAll also runs when every test skipped.
  t.afterAll(async () => {
    if (!postgresContainer() || !(await containerRunning())) return;
    await psql(fs.readFileSync(SEED_FILE, "utf8"));
  });
}

/**
 * Puts the CHG-FIXED-* fixtures back to their starting state by re-running the
 * (self-healing) seed in the stack's Postgres, then proves the stack under test
 * sees it: through the CUSTOMER backend, dave must be asked in 007 and 008. A
 * container that belongs to another stack fails here, with that said, rather
 * than as a puzzling assertion later.
 */
export async function resetFixtures(): Promise<void> {
  await psql(fs.readFileSync(SEED_FILE, "utf8"));
  const dave = customerApi("dave");
  const deadline = Date.now() + 15_000;
  let last = "";
  for (;;) {
    const [approval, review] = await Promise.all([
      dave.get(FIXTURES.approval.id),
      dave.get(FIXTURES.review.id),
    ]);
    last = `${FIXTURES.approval.number}: HTTP ${approval.status} ${approval.body?.state?.label} answer=${approval.body?.customerCanAnswer}; ` +
      `${FIXTURES.review.number}: HTTP ${review.status} ${review.body?.state?.label} answer=${review.body?.customerCanAnswer}`;
    if (
      approval.status === 200 && approval.body.state?.label === "Customer Approval" && approval.body.customerCanAnswer === true &&
      review.status === 200 && review.body.state?.label === "Customer Review" && review.body.customerCanAnswer === true
    ) {
      return;
    }
    if (Date.now() > deadline) {
      throw new Error(
        `After re-seeding ${postgresContainer()}, dave does not see the fixtures waiting for him through ` +
          `${(await stackEndpoints()).customerApi} (${last}). Either E2E_POSTGRES_CONTAINER is not the database ` +
          "of the stack at E2E_BASE_URL, or that stack is older than the customerCanAnswer field.",
      );
    }
    await new Promise((r) => setTimeout(r, 500));
  }
}

/** What the database holds about a change request's state and window. */
export type ChangeRequestRow = {
  state: string;
  /** `YYYY-MM-DDTHH:MM:SSZ` or "" when unset. */
  startUtc: string;
  endUtc: string;
  title: string;
};

/** Reads a change request's raw row (state enum, planned window in UTC, subject). */
export async function changeRequestRow(id: string): Promise<ChangeRequestRow> {
  const fmt = (col: string) => `coalesce(to_char(cr.${col} at time zone 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'),'')`;
  const out = await psql(
    `select cr.state, ${fmt("start_on")}, ${fmt("end_on")}, wi.subject ` +
      `from change_request cr join work_item wi on wi.id = cr.id where cr.id = '${id}'`,
  );
  const [state, startUtc, endUtc, ...title] = out.split("|");
  return { state, startUtc, endUtc, title: title.join("|") };
}

/** One approver row: the stage it belongs to, who, and its approval_stage_approver.state (UPPER_SNAKE_CASE: REQUESTED, APPROVED, REJECTED, CANCELLED, ...). */
export type ApproverRow = { stage: string; email: string; state: string };

/** Every approver row of a change request, oldest stage first (the whole history). */
export async function approverRows(id: string): Promise<ApproverRow[]> {
  const out = await psql(
    "select s.checkpoint_label, u.email, a.state from approval_stage s " +
      "join approval_stage_approver a on a.stage_id = s.id " +
      'join "user" u on u.id = a.approver_user_id ' +
      `where s.work_item_id = '${id}' order by s.created_on, s.id, u.email`,
  );
  return out
    ? out.split("\n").map((line) => {
        const [stage, email, state] = line.split("|");
        return { stage, email, state };
      })
    : [];
}

// --- Wall-clock helpers for the Propose New Time dialog (independent of the app's) ---

/** `YYYY-MM-DDTHH:mm` on `zone`'s wall clock at the instant `at`. */
export function wallTime(at: Date, zone: string): string {
  const parts = new Intl.DateTimeFormat("en-CA", {
    timeZone: zone,
    year: "numeric", month: "2-digit", day: "2-digit",
    hour: "2-digit", minute: "2-digit", hourCycle: "h23",
  }).formatToParts(at);
  const p = (type: string) => parts.find((x) => x.type === type)?.value ?? "00";
  return `${p("year")}-${p("month")}-${p("day")}T${p("hour")}:${p("minute")}`;
}

/** The UTC instant at which `zone`'s wall clock reads `local` (`YYYY-MM-DDTHH:mm`). */
export function wallTimeToUtc(local: string, zone: string): Date {
  const [datePart, timePart] = local.split("T");
  const [y, mo, d] = datePart.split("-").map(Number);
  const [h, mi] = timePart.split(":").map(Number);
  const asUtc = Date.UTC(y, mo - 1, d, h, mi);
  let instant = asUtc;
  // Two passes settle the zone's offset (it can differ around the guess).
  for (let i = 0; i < 2; i++) {
    const shown = wallTime(new Date(instant), zone);
    const [sd, st] = shown.split("T");
    const [sy, smo, sdd] = sd.split("-").map(Number);
    const [sh, smi] = st.split(":").map(Number);
    instant -= Date.UTC(sy, smo - 1, sdd, sh, smi) - asUtc;
  }
  return new Date(instant);
}

/**
 * A proposed window `daysAhead` days from today on `zone`'s calendar, starting at
 * `startHour`:00 and lasting `hours`, as the two `datetime-local` strings the
 * dialog takes and the UTC instants the backend must end up holding.
 */
export function futureWindow(
  zone: string,
  options: { daysAhead: number; startHour?: number; hours?: number },
): { start: string; end: string; startUtc: string; endUtc: string } {
  const { daysAhead, startHour = 16, hours = 4 } = options;
  const today = wallTime(new Date(), zone).split("T")[0];
  const [y, mo, d] = today.split("-").map(Number);
  const day = new Date(Date.UTC(y, mo - 1, d + daysAhead));
  const date = day.toISOString().slice(0, 10);
  const pad = (n: number) => String(n).padStart(2, "0");
  const start = `${date}T${pad(startHour)}:00`;
  const end = `${date}T${pad(startHour + hours)}:00`;
  const iso = (local: string) => wallTimeToUtc(local, zone).toISOString().replace(/\.\d{3}Z$/, "Z");
  return { start, end, startUtc: iso(start), endUtc: iso(end) };
}
