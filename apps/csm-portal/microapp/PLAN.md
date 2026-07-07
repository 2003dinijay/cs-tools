# CSM Portal Microapp — Implementation Plan

Build `apps/csm-portal/microapp` the same way `apps/customer-portal/microapp` was built from its webapp: a **separate, mobile-first React SPA** that runs inside the WSO2 super app (React Native WebView), reuses the existing `csm-portal/backend` API contracts, and borrows domain logic/types from `csm-portal/webapp` — but does **not** share code or auth with it.

---

## 1. Reference architecture (how customer-portal/microapp was made)

Studying `customer-portal/microapp` against `customer-portal/webapp` shows the pattern:

| Concern | Webapp | Microapp |
|---|---|---|
| Auth | `@asgardeo/react` SDK, browser OAuth flow | **No Asgardeo SDK.** Tokens come from the super app via a `window.nativebridge` + `ReactNativeWebView.postMessage` bridge (`requestToken` / `requestIdToken`), cached in `localStorage`, refreshed per request |
| Routing | `react-router` `BrowserRouter`, lazy-loaded chunks | `react-router-dom` **`HashRouter`** (required for file-served WebView), eager imports |
| HTTP | `fetch` wrapper hook (`useAuthApiClient`) with correlation IDs | **axios** instance (`services/apiClient.ts`) with request interceptor that awaits a singleton `refreshToken()` promise, sets `Authorization` + `x-user-id-token` |
| State | React Query + contexts | React Query + contexts + **zustand** (`store/user.ts`) |
| Layout | Desktop `AppLayout` with side nav | Mobile `MainLayout` = `AppBar` (top) + `TabBar` (bottom) + FAB; safe-area insets requested from the device via the bridge |
| Structure | `features/<feature>/{api,components,pages,types,utils}` | Flat `pages/` + `components/{core,layout,features,shared,ui}` + `services/` (one file per domain) + `types/` (`*.dto.ts` + `*.model.ts` pairs) |
| Config | `public/config.js` → `window.config` | Same pattern, plus `IS_MICROAPP: true`; backend URL via `VITE_BACKEND_URL` in `.env` |
| UI kit | `@wso2/oxygen-ui` 0.6 + MUI | `@wso2/oxygen-ui` 0.2 + MUI 7 + `motion` for transitions |
| Scoping | — | `SelectProjectPage` + `RequireProject` route guard + `ProjectProvider` context persisting selection |

Key files to use as templates:

- `customer-portal/microapp/src/components/microapp-bridge/` — the native bridge (token, alerts, local data, safe-area, open URL, app version). **Copy as-is**; it is portal-agnostic.
- `customer-portal/microapp/src/services/apiClient.ts` + `services/auth.ts` — axios interceptors, singleton token refresh, JWT decode → zustand user store, group checks.
- `customer-portal/microapp/src/context/AppProvider.tsx` — provider stack: `ColorModeProvider → LayoutProvider → SnackbarProvider → ProjectProvider → MeProvider`.
- `customer-portal/microapp/src/App.tsx` — HashRouter, safe-area bootstrap, route tree with guard → `MainLayout` → pages.
- `vite.config.ts`, `tsconfig*.json`, `eslint.config.js`, `.prettierrc`, `public/config.js`, `.env.example` — tooling scaffold.

---

## 2. What's different for the CSM portal

1. **Audience & scoping.** Customer portal scopes everything to a selected *project* (`RequireProject`). CSMs are internal users working *across* accounts/projects. Drop the select-project gate; instead scope by "me" (`/users/me`) and let account/project be a filter, not a gate. If a scope picker is still wanted, reuse the `SelectProjectPage` pattern as an optional account switcher.
2. **Feature set.** Mirror the webapp's nav (`config/csmNavItems.ts`): Dashboard, Support (Cases), Operations, Engagements, Security Center, Updates, Time cards, Customers (Accounts + Projects), Admin (Users). Since the webapp gates WIP sections (`CSM_PORTAL_DISABLE_WIP_FEATURES`, `WipRouteGuard`, `CsmComingSoonPage`), replicate the same flag + coming-soon guard in the microapp so all sections exist but ship gated identically.
3. **Backend.** Point at `csm-portal/backend` (Go, `openapi.yaml`). Endpoints already exist for cases, comments, attachments, accounts, projects, deployments, products, change-requests, time-cards, vulnerabilities, updates, users. Reuse the webapp's header conventions: `Authorization: Bearer`, `x-user-id-token`, and `X-CSM-Correlation-ID` (the webapp backend echoes it; carry it into the axios client for parity with `BackendApiError.correlationId`).
4. **Types.** Derive `*.dto.ts` from `backend/openapi.yaml` / `webapp/src/api/backend/types.ts` and feature `types/` folders; write `*.model.ts` view-models + mappers per the microapp convention (webapp's `api/backend/mappers.ts` is the starting point).
5. **Nav.** Bottom `TabBar` fits ~5 items. Proposal: **Home, Cases, Time cards, Updates, More** — "More" opens the remaining sections (Operations, Engagements, Security Center, Customers, Admin), mirroring how customer-portal handles overflow via its All-Items/menu pages.

---

## 3. Target structure

```
apps/csm-portal/microapp/
├── public/config.js            # window.config (gitignored real values; template in README)
├── .env.example                # VITE_BACKEND_URL
├── vite.config.ts              # base "./", port 3000, aliases (copy from customer-portal)
├── tsconfig.json / .app / .node / .paths
├── eslint.config.js, .prettierrc
├── index.html
└── src/
    ├── main.tsx                # QueryClient (no-retry on 4xx) + OxygenUIThemeProvider
    ├── App.tsx                 # HashRouter + safe-area insets + routes
    ├── theme/                  # copy customer-portal theme, apply CSM branding
    ├── config/
    │   ├── config.ts           # window.config reader (IS_MICROAPP etc.)
    │   ├── endpoints.ts        # from backend openapi.yaml
    │   ├── constants.tsx
    │   └── featureFlags.ts     # CSM_PORTAL_DISABLE_WIP_FEATURES equivalent
    ├── components/
    │   ├── microapp-bridge/    # COPY VERBATIM from customer-portal/microapp
    │   ├── core/               # AppBar, TabBar, Fab, Snackbar, ErrorBoundary
    │   ├── layout/             # MainLayout, RequireAuth (replaces RequireProject)
    │   ├── shared/             # EmptyState, ErrorState, InfiniteScroll, SectionCard, ...
    │   ├── ui/                 # LoadingFallback, AuthorizationFallback, WidgetBox, ...
    │   └── features/           # per-feature widgets (dashboard, cases, timecards, ...)
    ├── pages/                  # HomePage, CasesPage, CaseDetailPage, CreateCasePage,
    │                           # TimeCardsPage, UpdatesPage, OperationsPage,
    │                           # ChangeRequestDetailPage, EngagementsPage,
    │                           # SecurityCenterPage, VulnerabilityDetailPage,
    │                           # CustomersPage, AccountDetailPage, ProjectDetailPage,
    │                           # AdminUsersPage, ProfilePage, MorePage, ComingSoonPage
    ├── services/               # apiClient.ts, auth.ts, cases.ts, timecards.ts,
    │                           # updates.ts, accounts.ts, projects.ts, changes.ts,
    │                           # vulnerabilities.ts, users.ts, metadata.ts
    ├── store/user.ts           # zustand user store
    ├── context/                # AppProvider + layout/snackbar/theme/me (+ scope if needed)
    ├── types/                  # *.dto.ts + *.model.ts per domain
    └── utils/                  # logger, ApiError, constants, filters, date hooks
```

Route tree (HashRouter):

```
/                     HomePage (dashboard widgets)
/cases                CasesPage            /cases/:id   CaseDetailPage
/create               CreateCasePage
/time-cards           TimeCardsPage
/updates              UpdatesPage
/operations           OperationsPage       /changes/:id ChangeRequestDetailPage   [WIP-gated]
/engagements          EngagementsPage                                             [WIP-gated]
/security-center      SecurityCenterPage   /vulnerabilities/:id                    [WIP-gated]
/customers            CustomersPage (Accounts|Projects tabs)                       [WIP-gated]
/customers/accounts/:id   AccountDetailPage
/customers/projects/:id   ProjectDetailPage
/admin/users          AdminUsersPage                                              [WIP-gated]
/profile              ProfilePage
/more                 MorePage (overflow nav)
```

---

## 4. Phased implementation

### Phase 0 — Scaffold (½–1 day)
Copy customer-portal/microapp tooling: `package.json` (rename, same dep set), Vite/TS/ESLint/Prettier configs, `index.html`, `public/config.js` template, README. `npm run dev` renders a blank themed page.

### Phase 1 — Platform core (1–2 days)
Copy `microapp-bridge/` verbatim; copy `services/apiClient.ts`, `services/auth.ts`, `store/user.ts`, `utils/{logger,ApiError,constants}.ts`; adapt group names in `checkUserGroups` to CSM roles; add `X-CSM-Correlation-ID` to the request interceptor. Copy provider stack (`context/`), theme, `main.tsx`. Verify token flow inside the super app dev shell (and a `.env` local fallback for browser dev).

### Phase 2 — Shell & navigation (1–2 days)
`App.tsx` route tree above; `MainLayout` with `AppBar` + `TabBar` (5 tabs incl. More); `MorePage`; `ComingSoonPage` + WIP flag guard replicating the webapp's `WipRouteGuard`; safe-area wiring; ErrorBoundary; Snackbar.

### Phase 3 — Types & services (2–3 days)
`config/endpoints.ts` from `backend/openapi.yaml`; DTOs from webapp `api/backend/types.ts` + feature types; models + mappers (port `api/backend/mappers.ts` logic); one service file per domain; React Query hooks colocated with feature components. Port webapp mapper tests where they exist (`mappers.test.ts`).

### Phase 4 — Core features (1–1.5 weeks)
Priority order (matches webapp maturity):
1. **Home/Dashboard** — my assigned open cases, case counts, quick links (`useGetCsmDashboard`, `useGetMyAssignedOpenCases` equivalents).
2. **Cases** — infinite-scroll list with filters, detail (comments, attachments, call requests), create/patch case. This is the biggest feature; the customer-portal microapp's cases pages are the closest template.
3. **Time cards** — list/search + create (`/time-cards`, `/time-cards/search`).
4. **Updates** — product update levels (`/updates/...`).
5. **Profile** — `/users/me`, sign-out via bridge.

### Phase 5 — WIP-gated features (1–1.5 weeks, can trail v1)
Operations (change requests), Engagements, Security Center (vulnerabilities), Customers (accounts/projects search + detail), Admin Users. Build behind the same WIP flag the webapp uses so they light up together.

### Phase 6 — Polish & verification (2–4 days)
Mobile ergonomics (pull-to-refresh, `InfiniteScroll`, skeletons, empty/error states with correlation "Reference ID"), dark mode, `tsc -b && vite build` clean, lint/prettier, on-device test in the super app (safe areas, token refresh, deep links via hash routes), README config docs.

---

## 5. Decisions & risks

- **Copy, don't share.** Follow the existing repo convention: microapp is a sibling app with duplicated primitives, not a shared package. A future `packages/` extraction (bridge, types) is possible but out of scope.
- **Oxygen UI version skew.** customer-portal/microapp pins `@wso2/oxygen-ui` 0.2.x while the webapps use 0.6.x. Prefer starting the CSM microapp on 0.6.x for parity with the CSM webapp theme — verify the microapp theme files against 0.6 APIs early (Phase 1), fall back to 0.2.x only if blocked.
- **Auth claims.** Confirm the super app's Asgardeo app registration issues the CSM roles/groups needed by `checkUserGroups` and the backend's `x-user-id-token` validation; this is the most likely integration blocker — test in Phase 1, not Phase 4.
- **Backend CORS/gateway.** Ensure the CSM backend gateway accepts the microapp origin and exposes `X-CSM-Correlation-ID` via `Access-Control-Expose-Headers` (the webapp code notes this gap).
- **Websockets/chat.** Customer portal has Novera chat over WS; CSM webapp has conversation messages only via REST — no WS work needed in v1.

**Estimated total: ~4–5 weeks** for one developer, all features; ~2.5 weeks to a v1 with Phases 0–4 only.
