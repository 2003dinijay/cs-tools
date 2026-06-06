# CSM Portal Backend

Go HTTP server (`net/http`, Go 1.22+) that acts as a backend-for-frontend (BFF) for the CSM portal. It authenticates callers, forwards requests to upstream services, and shapes responses for the frontend.

## Upstream service modules

Each upstream service has its own client package under `internal/`:

| Package | Upstream | Notes |
|---------|----------|-------|
| `entity` | Entity service | Most case/account/project endpoints; raw `[]byte` passthrough |
| `scim` | SCIM service | User/group lookups |
| `updates` | Updates service | Product update levels; returns typed structs (not raw passthrough) |

New upstream services get their own package under `internal/` following the same `Client` + `do()` pattern.

## Commands

```
make setup   # wire up git hooks (once after clone)
make test    # vet + race-detector tests
make build   # runs tests then compiles ./cmd/server
```

Tests run automatically on `git push` via the pre-push hook.

## Adding a new endpoint

Follow these steps in order:

1. **Upstream client** (`internal/<module>/`) — add a method on `Client` that calls `c.do()`; use `url.PathEscape()` for every path parameter
2. **Handler interface** — extend the local interface in the relevant handler file (e.g. `entityCaseClient` in `cases.go`); keep it minimal — only methods that handler actually calls
3. **Handler func** — auth check → path/body guards → call client → `mapUpstreamError` on failure → write response
4. **Route** (`cmd/server/main.go`) — register using Go 1.22 method-prefixed patterns: `"POST /cases/{id}/comments"`
5. **OpenAPI spec** (`openapi.yaml`) — add the path with 200/400/401/403/404/500 responses; `403` is always required because `mapUpstreamError` can return it
6. **Tests** — add handler tests; update the mock in `helpers_test.go` to satisfy the extended interface

## Handler conventions

- **Auth**: always check `middleware.UserInfoFromContext(r.Context()) == nil` first → 401
- **Body size**: cap with `http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)` (1 MiB) before reading
- **Path params**: guard against empty string after `r.PathValue("id")`
- **Upstream errors**: always use `mapUpstreamError(w, err, "<fallback message>")` — never write custom status mappings inline
- **Response**: return raw `[]byte` with `writeJSON` for simple passthroughs; unmarshal into typed structs only when the response shape needs to change

## OpenAPI spec

- Error responses use `$ref: '#/components/schemas/ErrorPayload'`
- Every endpoint must declare a `403` response
- The `Case` schema includes a computed `next_states` read-only field populated server-side from `state`

## Response shape

- All JSON response fields must use **camelCase** (e.g. `createdAt`, `projectId`, `issueType`)
- When transforming an upstream response into a typed struct, use `json:"fieldName"` struct tags to enforce this — never return snake_case or PascalCase field names to the frontend
- The `next_states` field is an exception — it uses snake_case to match the upstream entity service contract

## Security

- **Never commit secrets** — API keys, tokens, passwords, and service URLs with credentials must not appear in source code or config files; use environment variables
- **No sensitive data in logs** — do not log request bodies, JWT payloads, or user PII; log only IDs and error summaries
- **JWT is the only auth mechanism** — all endpoints must validate the caller via `middleware.UserInfoFromContext`; there are no public endpoints
- **Input validation** — validate and reject unexpected input at the boundary (path params, body size, JSON structure) before forwarding to upstream services
- **Error messages** — never leak upstream error details or stack traces to the caller; use the fixed `ErrMsg*` constants or a short fallback message

## Testing

- Mocks live in `internal/handler/helpers_test.go` — when you extend a handler interface, add the new field and method to the mock there
- `upstreamErrors(fallback)` returns the standard upstream error table used across all handler tests
- `withUser()` injects a test user into the request context
- `decodeJSON[T]()` decodes response bodies in assertions
