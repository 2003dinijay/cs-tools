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

Scaffolding only — **no specs yet**. Runs locally against `pnpm run dev`
(:3000), authenticated by a **captured browser session** so no login page or
2FA is driven. See [`auth/README.md`](./auth/README.md) to capture one.

There is no mock backend here: specs will hit the same backend the dev server
is configured against (`public/config.js`), so anything a spec creates is a
**real record** in that environment. Tag created data so it stays identifiable.

## Run

```bash
pnpm run test:e2e                          # all specs (boots dev server)
node_modules/.bin/playwright test --ui     # author/debug interactively
node_modules/.bin/playwright show-report   # open the last HTML report
```

> Note: `pnpm exec playwright …` fails in this repo ("packages field missing");
> call the binary directly via `node_modules/.bin/playwright …`.

Environment switches (see `playwright.config.ts`):

| Var | Effect |
|---|---|
| `E2E_BASE_URL` | Target a deployed environment instead of `http://localhost:3000` |
| `E2E_NO_WEBSERVER=1` | Don't boot the dev server (use with `E2E_BASE_URL`) |
| `CI` | `forbidOnly`, and never reuse an already-running dev server |

## Layout

| Path | Purpose |
|---|---|
| `auth/README.md` | How to capture a session bundle (localStorage + sessionStorage) |
| `fixtures/test.ts` | `withRole(test, role)` replays a session and skips the file if absent; `openContextAs(browser, role)` opens a second authenticated context |
| `pages/` | Page objects — one per screen, no assertions inside |
| `specs/` | The specs, grouped in subfolders by feature area |
| `utils/` | Shared selectors / data-tagging helpers |
| `storageState/` | Captured session bundles — **git-ignored, real tokens** |

## Roles

`PortalRole` in `fixtures/test.ts` is `"admin" | "lead" | "portal" | "security"`,
matching the portal's user types. Capabilities are split across two sources, so
a test account needs both set correctly:

- **admin** — the ServiceNow user role `sn_customerservice.customer_admin`
  (read from `GET /users/me`), which gates Settings user management and
  registry service tokens.
- **lead / portal / security** — project **membership** flags (`isLead`,
  `isPortalUser`, `isSecurityContact`) on the project under test, read from the
  project contacts list.

Project-level feature visibility (Operations, Security Center, Updates,
Engagements, Usage & Metrics …) is independent of all of these — it comes from
`GET /projects/{id}/features`. Pick the project a spec runs against
accordingly.
