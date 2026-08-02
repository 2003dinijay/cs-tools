# CSM Portal Webapp

React + TypeScript + Vite frontend for the CSM portal, using the `@wso2/oxygen-ui` (MUI-based) design system, TanStack React Query for data fetching, and `react-router` v7 (the unified `react-router` package — **not** `react-router-dom`). See `README.md` for setup, `window.config` keys, and the `dev:local` local-gateway mode; this file covers code-organization and workflow conventions instead.

## Path aliases

Defined once in `vite.config.ts`, mirrored in `tsconfig.app.json`. Use them instead of relative imports beyond one level.

| Alias | Resolves to |
|---|---|
| `@` | `src` |
| `@api` | `src/api` |
| `@assets` | `src/assets` |
| `@components` | `src/components` |
| `@config` | `src/config` |
| `@constants` | `src/constants` |
| `@context` | `src/context` |
| `@features` | `src/features` |
| `@hooks` | `src/hooks` |
| `@layouts` | `src/layouts` |
| `@providers` | `src/providers` |
| `@utils` | `src/utils` |

A handful of narrower aliases (`@case-details*`, `@time-tracking`, `@deployments`, `@update-cards`) point at specific component subfolders inside individual features — a one-off pattern from a couple of features, not something to replicate for new ones.

## Code organization

Feature-based: each `src/features/<name>/` folder owns its own `api/` (React Query hooks), `components/`, `pages/`, `types/`, `utils/`. Tests are colocated as `<File>.test.tsx` next to the file under test, not in a separate `__tests__` tree.

## Routing (`src/App.tsx`)

- Every route page component is `React.lazy`-loaded individually (one `lazy(() => import(...))` call per page); only error pages and the coming-soon placeholder are eager.
- Declared with `react-router` v7's JSX `<Routes>`/`<Route>` API, not a data router (`createBrowserRouter`).
- Everything except `/401`/`/403`/`/404` sits under `<Route element={<AuthGuard />}>` → `<Route element={<FeatureRouteGuard />}>` (a runtime feature-flag/WIP gate keyed by `CSM_PORTAL_FEATURE_OVERRIDES`).
- Legacy paths (e.g. `/accounts`, `/projects`) are kept as `<Navigate>` redirects for backward compatibility rather than removed outright.

## Data fetching

TanStack React Query is the only data-fetching layer — no other state-management library. Custom hook naming is verb-prefixed and consistent:

- `useGetX` — read a single resource (`useGetCsmCaseDetail`, `useGetAccount`, `useGetUsersMe`)
- `usePostX` / `usePatchX` — mutations (`usePostCsmCase`, `usePatchUsersMe`)
- `useSearchX` / `useXOptions` — typeahead/autocomplete queries
- Plain domain-named hooks for composed/derived state (`useCaseComposition`, `useIsTeamLead`)

## Testing

- Runner: Vitest (`jsdom` environment), configured inline in `vite.config.ts` — no separate `vitest.config.ts`.
- **`@api/backend/client` and `@config/apiConfig` read `window.config` at module load time**, which doesn't exist under Vitest. Any test that transitively imports the real client/config (even through an unrelated import chain — e.g. a dashboard list-widget renderer pulling in a mapper that imports `@config/apiConfig`) must mock it first:
  ```ts
  vi.mock("@api/backend/client", () => ({ useBackendApi: () => ({ post: postMock }) }));
  vi.mock("@config/apiConfig", () => ({ apiConfig: { backendUrl: "https://example.test" } }));
  ```
- Mock the low-level client when testing a component wired directly to a data hook; mock the hook module itself when testing a component that just *consumes* an already-built hook.
- React Query tests wrap `render`/`renderHook` in `QueryClientProvider` with a fresh `new QueryClient({ defaultOptions: { queries: { retry: false } } })` per test.
- Components needing router context render inside `<MemoryRouter>` (from `react-router`). To assert on where a click actually navigated (not just that a handler fired), wrap in `<Routes>` with a destination route rendering a probe component that reads `useLocation()`, rather than mocking `useNavigate` — see `DashboardWidgetPreviewPage.test.tsx` / `DashboardWidgetTile.test.tsx` for the pattern.
- Charting components (`@wso2/oxygen-ui-charts-react`, a `recharts` wrapper) don't render meaningfully under `jsdom` (`ResponsiveContainer` measures a real layout size, always `0` in `jsdom`) — mock the package to a plain list of clickable stand-ins exposing the data/handlers under test, not the real chart. See `DashboardWidgetTile.test.tsx`'s mock of `Pie`/`Bar`/`PieChart`/`BarChart`.

## Commands

```bash
pnpm run dev       # Vite dev server
pnpm run build     # tsc -b && vite build
pnpm run test      # vitest
pnpm run test:e2e  # playwright test
pnpm run lint      # eslint .
```

## Security

- **Never commit secrets or runtime config** — `public/config.js` (the real, per-deployment file copied from `public/config.js.example`) is git-ignored on purpose. It only ever holds *public* runtime values (IdP base URL, OAuth **client id** — not a secret, this is a PKCE/SPA flow with no client secret — backend base URL, theme, log level, feature flags); if a value you're adding isn't safe to ship to a browser, it doesn't belong in `window.config` at all.
- **Auth is Asgardeo's job, not ours** — `AsgardeoProvider` (`AppWithConfig.tsx`) and `useAsgardeo()`/`AuthGuard.tsx` own the whole auth lifecycle; the app **never reads or writes auth tokens itself** (no token in `localStorage`/`sessionStorage` anywhere in this codebase — the SDK manages that internally). Don't add code that pulls a token out of storage or a hook return value to attach it manually; use the existing API client wiring instead. The only `sessionStorage` key outside the SDK is `POST_LOGIN_REDIRECT_KEY`, and it stores a post-login redirect **path**, never a token.
- **Sanitize before `dangerouslySetInnerHTML`, always** — every current use (case-comment rendering, rich-text description/plan fields, product-update content, the admin-configured top banner/announcement HTML) goes through `DOMPurify.sanitize` first (`src/utils/sanitizeHtml.ts` for the shared cases). `sanitizeHtml.ts` also force-hardens every sanitized `target="_blank"` link with `rel="noopener noreferrer"` against reverse tabnabbing — match that when adding any other `target="_blank"` link by hand. Never add a new `dangerouslySetInnerHTML` call that skips sanitization, even for content that looks like it's "our own" (e.g. admin-authored) — an admin-configured field is still attacker-controlled if that admin account is ever compromised.
- **`useLogger()`/`Logger` (`src/hooks/useLogger.ts`, `src/hooks/logger.ts`) is a plain console wrapper with no redaction** — it does not scrub or block sensitive fields before printing. The discipline has to live at the call site: log `error.message`/a short summary, not a whole caught `Error` object, request/response body, or user/profile object, since any of those can carry tokens, PII, or internal ids straight into the browser console (and potentially a session-replay/log-shipping tool downstream).
- **Don't let the client-facing UI surface backend internals** — render whatever (already-generic) message the backend sends; don't append stack traces, raw error objects, or your own extra technical detail on top of it. See the dashboard widget-preview page's URL design below for the same principle applied to URLs, not just error text: prefer a `@me`-style sentinel over embedding a raw internal id in anything bookmarkable/shareable/loggable.
- **No `eval`, `new Function`, or dynamic `<script>` injection** anywhere in `src/` — keep it that way; there's no legitimate use case for it in this app.

---

## The config-driven dashboard widget system

`AgentsLandingPagePilot` (rendered from `CsmDashboardPage.tsx`, alongside the older `AbtDashboardHeader` dashboard/team switcher) is a **fully backend-config-driven** widget grid — there is no frontend-side widget registry. The backend's `DASHBOARDS_CONFIG` env var (a JSON array, parsed into Go's `dashboard.Dashboard`/`dashboard.WidgetTemplate` — see `apps/csm-portal/backend/internal/dashboard/widgets.go`) is the single source of truth for which dashboards exist, which widgets each one has, and how each widget is filtered. The frontend never hardcodes a widget list; it only knows how to *render* whatever shape/resourceType combination the backend sends.

Other components in this same folder (`CaseCompositionCharts`, `CompositionDonut`, `CaseCountsMatrix`, `CustomerSummarySection`, `RecentActivitySection`, `MyAssignedCases`) are a separate, older composition-chart system that isn't currently wired into `CsmDashboardPage.tsx` — don't confuse them with the widget system below.

### The wire shape (`src/api/backend/types.ts`)

`GET /dashboards` returns `BeDashboardListItem[]` (id/displayName/isDefault/isTeamBased — enough for the dashboard switcher). `GET /dashboards/{id}` returns the full `BeDashboard`, whose `widgets: BeDashboardWidget[]` is the actual per-widget config:

```ts
interface BeDashboardWidget {
  widgetId: string;
  displayName: string;
  description?: string;        // subtitle shown under displayName; not shape-specific by design
  resourceType: BeWidgetResourceType;  // "case" | "incident" | "change_request" | "account" | "project"
                                        // | "user" | "time_card" | "problem" | "product_vulnerability"
  shape: BeWidgetShape;         // "count" | "list" | "pie" | "bar"
  gridWidth: number;            // 1-12, CSS grid columns this widget occupies
  filters: Record<string, unknown>;  // opaque; passed verbatim as the filters of that
                                      // resourceType's own POST /{resourceType}s/search
  groupBy?: string;             // present on the wire; unused today — `slices` (below) is what
                                 // actually drives pie/bar grouping
  listLimit?: number;           // shape "list" only: how many rows to show
  slices?: BeDashboardPieSlice[]; // shapes "pie"/"bar" only: see below
  section?: string;            // groups widgets under a titled sub-heading — see below
}

interface BeDashboardPieSlice {
  label: string;
  color?: BeWidgetPaletteColor;  // "primary" | "secondary" | "success" | "error" | "info" | "warning"
  filters: Record<string, unknown>;  // this slice's own criteria only — merge under the widget's
                                      // own base `filters` (slice keys win) to resolve its total
}
```

Every field beyond `widgetId`/`displayName`/`resourceType`/`shape`/`gridWidth`/`filters` is optional and `omitempty` on the wire — a widget/dashboard that doesn't set them behaves exactly as if the field didn't exist.

### How each `shape` resolves its data

All four shapes ultimately do the same thing — issue that `resourceType`'s own `POST /{resourceType}s/search` and read the response — they just differ in *how many* searches and what they do with the result(s):

- **`count`** (`useWidgetData`, shape `"count"`): one search, `pagination.limit: 1`, reads `total`. Renders a big number, the whole tile is a link to that resource's own tab (`WIDGET_RESOURCE_CONFIG[resourceType].buildHref(filters)`).
- **`list`** (`useWidgetData`, shape `"list"`): one search, `pagination.limit: listLimit ?? 4`, reads the item array. Renders through `WIDGET_LIST_RENDERERS[resourceType]` (`widgetListConfig.tsx`) — reuses each resource's **own real tab component** where one exists (e.g. `case` renders through the identical `CasesList` the Cases tab uses, via the same `mapCaseSearchViewToRow` mapper), falling back to the generic `DashboardMiniTable` otherwise. A "View more" link (shown only once `total > (listLimit ?? 4)`) goes to a dedicated, real, bookmarkable preview route (`DashboardWidgetPreviewPage`, `/dashboard/:previewSlug`) with pagination and search — not directly to the resource's own tab.
- **`pie`** / **`bar`** (`useWidgetPieData`): one search **per `slices` entry**, each with `pagination.limit: 1`, merging that slice's own `filters` under the widget's base `filters` (slice keys win). `useWidgetPieData` is shape-agnostic — it just resolves N labeled totals; `DashboardPieChart`/`DashboardBarChart` (both under `components/`) render the same resolved data as wedges or bars respectively. Each wedge/bar/legend row navigates to that slice's own filtered destination the same way a `count` tile does. A total of `0` across every slice renders an empty state (inbox icon + "Nothing to show here right now"), not an all-grey chart.

`DashboardWidgetTile.tsx` is the per-widget dispatcher: it reads `shape` and picks which of the above to render, plus the shared per-widget loading/error states.

### `section` grouping

`AgentsLandingPagePilot.tsx`'s `groupWidgetsBySection` groups a dashboard's widgets by their `section` value, preserving the order each distinct value (including the untitled default, for widgets with no `section`) first appears among the widgets array. Widgets sharing a `section` render together under one bold heading, separate from the rest — e.g. a handful of SLA-violation `count` widgets can all set `"section": "SLA Violation"` to appear grouped under that heading without needing a separate dashboard. No `section` set anywhere (the common case) renders exactly as before this field existed: one untitled group, with `pie`/`bar` widgets still visually separated from `count`/`list` ones within that group.

### Per-resourceType config (`config/widgetResourceConfig.ts`, `config/widgetListConfig.tsx`)

`WIDGET_RESOURCE_CONFIG` is the one place that knows, per `resourceType`: its search endpoint, response items key, list-row label extractors, tile icon/color, `buildHref` (translates a widget's opaque filters into that resource's **own** tab's URL filter scheme — e.g. the dashboard's lowercase `catastrophic`/`critical`/... severity values get translated to the Cases tab's own `S0`/`S1`/... codes), and `previewSlug` (the URL segment for that resourceType's `/dashboard/:previewSlug` preview page). Adding a new `resourceType` the backend can send means adding an entry here and to `WIDGET_LIST_RENDERERS` — everything else (tile rendering, preview page, pagination) is generic over any entry in these two config objects.

### Current-user placeholder

The backend resolves a `"__current_user__"` filter placeholder into the caller's real platform user id **before** it reaches the frontend (see the backend's `ResolveFilters`/`ResolveSliceFilters`) — the frontend never sees or handles that placeholder for a widget's own data-fetching. Separately, the "View more" preview page's own **URL** masks the current user's id to a `@me` sentinel (`widgetPreviewUrl.ts`'s `buildWidgetPreviewHref`/`resolveCurrentUserSentinels`) purely so a bookmarked/shared link doesn't carry a bare internal user id — that's a frontend-only, URL-display concern, unrelated to the backend's own placeholder resolution.
