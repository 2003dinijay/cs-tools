<!--
Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).

WSO2 LLC. licenses this file to you under the Apache License,
Version 2.0 (the "License"); you may not use this file except
in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing,
software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
KIND, either express or implied.  See the License for the
specific language governing permissions and limitations
under the License.
-->

# Customer Portal E2E (Playwright, local)

Runs locally against `pnpm run dev` (:3000) or a deployed environment, authenticated by a
**captured browser session** so no login page or 2FA is driven. See
[`auth/README.md`](./auth/README.md) to capture one. To run against the **local compose
stack** instead, see "Local stack" below.

There is no mock backend here: specs hit the same backend the dev server is configured
against (`public/config.js`), so anything a spec creates is a **real record** in that
environment. Tag created data so it stays identifiable.

## Run

Configuration lives in **`.env.e2e`** (at the webapp root, committed), so no env
vars need to be typed on the command line:

```bash
pnpm run test:e2e                          # all specs
node_modules/.bin/playwright test --ui     # author/debug interactively
node_modules/.bin/playwright show-report   # open the last HTML report
```

> Note: `pnpm exec playwright …` fails in this repo ("packages field missing");
> call the binary directly via `node_modules/.bin/playwright …`.

### Configuration

`playwright.config.ts` loads the env files itself (via node's built-in
`process.loadEnvFile`, no dotenv dependency). Precedence, highest first:

1. **real environment / CLI** — `E2E_BASE_URL=… pnpm run test:e2e`
2. **`.env.e2e.local`** — your personal overrides, git-ignored (`.env.*.local`)
3. **`.env.e2e`** — committed team defaults

| Var | Effect |
|---|---|
| `E2E_BASE_URL` | Environment under test. Default in `.env.e2e` is staging; falls back to `http://localhost:3000` if unset everywhere |
| `E2E_NO_WEBSERVER=1` | Don't boot the local dev server — required when `E2E_BASE_URL` points at a running deployment |
| `CI` | `forbidOnly`, and never reuse an already-running dev server |

No secrets belong in these files. Login is by replaying
`storageState/session.json` (git-ignored), not by credentials in env vars.

### Base URL must match the captured session

**The captured bundle decides which environment you can run against** — it only
restores into the origin it was captured from (see `fixtures/test.ts`).
`.env.e2e` ships pointing at staging because that is where the current
`session.json` was captured.

To run against the local dev server instead: recapture `session.json` while
signed in at `http://localhost:3000`, then create `.env.e2e.local` with

```bash
E2E_BASE_URL=http://localhost:3000
# Must be set empty, not omitted: keys absent from .env.e2e.local still come
# from .env.e2e, and an empty value reads as falsy so Playwright boots
# `pnpm run dev` itself.
E2E_NO_WEBSERVER=
```

`withSession()` skips (rather than fails) any test whose session bundle is
missing or captured against a different origin than the run targets, and the
skip message names the mismatch.

## Local stack (docker-compose + the mock identity provider)

Everything above targets a deployed environment and signs in as a staging account. To run
against the **local compose stack** instead (`docker-compose up -d`; customer webapp
`http://localhost:3000`, customer backend `:8090`, mock OIDC provider `http://localhost:9100`),
sign in as one of the customers the local seed registers on its projects
(`scripts/csm-compose/seed-entity-service.sql`; "Local seed personas" in
`entity-service/CLAUDE.md`):

| Persona (`E2E_LOCAL_PERSONA`) | Email | Registered contact of |
|---|---|---|
| `dave` | `dave.mendis@example.com` | "Example Corp Production" (project `00000000-0000-0000-0000-000000000401`) |
| `erin` | `erin.jayawardena@example.com` | same project |
| `mira` | `mira.santos@lumenworks.example` | "Lumen Works Platform" (a generated project: found by name, its id is random per database) |
| `noel` | `noel.prasad@lumenworks.example` | same project |

The mock provider signs in **any** email with no credential check, so nothing here needs a
password or TOTP seed. A session is minted by driving the app's own sign-in once and is then
replayed like any hand-captured bundle:

```bash
# from apps/customer-portal/webapp, with the stack up and seeded
for p in dave erin mira noel; do
  E2E_LOCAL_PERSONA=$p pnpm run test:e2e:local-auth      # writes tests/e2e/storageState/local-$p.json
done
node_modules/.bin/playwright test tests/e2e/specs/local --project=chromium
```

* **Groups stay empty.** The mock provider's sign-in form pre-fills its Groups box with
  `cs_engineer`, a CSM *staff* group, and copies it into the token verbatim. The mint step clears
  it and asserts the signed-in user holds only the `customer` role. What a customer may see comes
  from their user record (role `customer`, a registered portal contact of a project), not from
  the token.
* **Tokens last one hour** and the provider issues no refresh token. Re-run the mint line to
  refresh a bundle (it overwrites). A spec whose bundle is missing, expired, minted for another
  origin than `E2E_BASE_URL`, or whose stack does not answer, is **skipped** with the reason, never
  failed.
* **Other ports.** The webapp origin is part of a bundle; mint and run against the same one:
  `E2E_BASE_URL=http://localhost:13000 E2E_LOCAL_PERSONA=dave pnpm run test:e2e:local-auth`, then
  `E2E_BASE_URL=http://localhost:13000 node_modules/.bin/playwright test tests/e2e/specs/local --project=chromium`.
  (`.env.e2e` already sets `E2E_BASE_URL=http://localhost:3000` and `E2E_NO_WEBSERVER=1`.)
* **The auth setup project** of the main config (`auth/auth.setup.ts`, the staging sign-in) skips
  itself when no staging credentials are set, so these specs run without any.
* **Fixtures move.** The seeded `CHG-FIXED-*` change requests are driven forward by whoever
  approves or rejects them; `docker-compose up -d migrate` re-runs the (self-healing) seed and puts
  them back. The read-only smoke spec here changes nothing.
* **Operations needs the seed.** The menu appears only when the project's type grants change request
  / service request read access; the seed sets that on the local "Subscription" type. A database
  seeded before that existed shows no Operations menu until `migrate` is re-run.

| File | Purpose |
|---|---|
| `auth/local-session.setup.ts` | Mints `storageState/local-<persona>.json` (run through `playwright.local-auth.config.ts`, a separate config so a regression run never mints as a side effect) |
| `auth/localSessions.ts` | `LOCAL_PERSONAS`, `withLocalSession(test, "dave")` (replays the bundle; skips on missing / expired / wrong-origin / unreachable), `sessionMinutesLeft` |
| `specs/local/customer-change-requests.spec.ts` | Smoke: dave lists `CHG-FIXED-007` (Customer Approval) under Operations > Change requests, and no other customer's change request |

## Layout

| Path | Purpose |
|---|---|
| `auth/README.md` | How to capture a session bundle (localStorage + sessionStorage) |
| `fixtures/test.ts` | `withSession(test)` replays `storageState/session.json`, skipping each test that uses it when the bundle is absent or captured against a different origin (it skips from `beforeEach`, so tests are reported individually as skipped rather than the file being skipped as a unit); `openContextAs(browser, name)` opens a second authenticated context |
| `pages/` | Page objects — one per screen, no assertions inside |
| `specs/` | The specs, grouped in subfolders by feature area |
| `utils/` | Shared selectors / data-tagging helpers |
| `storageState/` | Captured session bundles — **git-ignored, real tokens** |

## Roles

One session (`session.json`) is captured today, so specs run as whatever that
account is. For role-gated coverage, capture additional bundles as
`storageState/<name>.json` and pass the name to `withSession(test, name)` or
`openContextAs(browser, name)`.

Each bundle should be captured from an account holding one role on the project
under test, so a spec can assert what that role can and cannot do:

- **admin** — manages users and registry service tokens in Settings.
- **lead** — a portal user who can also escalate a case past EL3.
- **portal** — the baseline: signs in, creates and manages cases.
- **security** — receives security advisories and raises security reports.

Project-level feature visibility (Operations, Security Center, Updates,
Engagements, Usage & Metrics …) is independent of all of these — it comes from
`GET /projects/{id}/features`. Pick the project a spec runs against
accordingly.
