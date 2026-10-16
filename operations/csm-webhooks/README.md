# CSM Webhooks

Public endpoints that receive inbound webhooks from external systems and forward them to the
platform's internal entity-service. **One folder per source.** Each folder is its own deployable
component with its own `.choreo/component.yaml`, Go module, public endpoint and secret.

| Folder | Source | Public route |
|---|---|---|
| [`github/`](github/) | GitHub issue and issue-comment webhooks | `POST /webhooks/github` |

## Contents

- [Architecture](#architecture)
- [Developer guide](#developer-guide)
- [Configuration](#configuration)

## Architecture

### Purpose and place in the platform

entity-service is only visible inside the organization's network boundary and must stay that way:
exposing one of its routes to the internet would publish every other route with it. External
systems, however, can only call a public URL. These components close that gap. Each is a thin,
public front door that proves a delivery really came from its source, then hands it to
entity-service over the internal network as an authenticated internal client. Everything that reads
or writes platform data stays behind that boundary.

For how this fits into the wider CSM platform, see the
[platform architecture](../../apps/csm-portal/README.md#architecture).

### Rules every webhook component follows

- **Verify and forward, nothing else.** Check the source's signature over the raw body, then
  forward. No database credentials, no decisions about what a payload means.
- **Forward the body byte for byte.** The signature is computed over the raw bytes, so re-encoding
  the JSON (even reordering keys) would make it unverifiable downstream. The payload is carried as
  raw JSON, not a parsed struct.
- **One identical rejection for every signature failure.** The caller must not learn which check
  failed and is never shown the expected signature.
- **Public on purpose.** The component's endpoint is declared with public network visibility.
- **One directory under `cmd/`.** A second `package main` anywhere in a module breaks the Go
  buildpack used for deployment (`no Go files in /workspace`).
- **Registered as an internal client.** The OAuth2 client id a component forwards as must be listed
  in entity-service's `M2M_CLIENT_IDS`, or every forward is refused with 401.

### The GitHub component (`github/`)

Triggers: inbound HTTPS `POST /webhooks/github` deliveries from a GitHub repository webhook (events
**Issues** and **Issue comments**), plus an unauthenticated `GET /health` liveness probe.
Language/runtime: Go 1.26.6 (see `github/go.mod`), a long-running HTTP server.

```text
GitHub --POST /webhooks/github--> csm-github-webhook (public)
                                    | verify X-Hub-Signature-256 (HMAC-SHA256 over raw body)
                                    v
                             API gateway (adds the caller-identity header)
                                    |
                                    v
            entity-service  POST /github/deliveries  (internal)
                                    |
                                    v
                        issue sync  -->  PostgreSQL
```

Request handling, in order:

1. Read the body, capped at 2 MiB (`413` if exceeded).
2. Verify `X-Hub-Signature-256` with constant-time comparison. Only the SHA-256 header is
   accepted; the legacy SHA-1 header is ignored. A missing, malformed or wrong signature, or an
   empty secret, yields `401`.
3. Require `X-GitHub-Delivery` (delivery id, reused by GitHub across redeliveries) and
   `X-GitHub-Event` (event type), else `400`.
4. Forward `{id, event, payload}` to entity-service `POST /github/deliveries`.

| Status | When |
|---|---|
| 200 | Applied, deliberately skipped, or a duplicate delivery. All three return 200 so GitHub stops retrying (entity-service answers 409 for an already-applied delivery; this component maps it to 200). |
| 400 | Unreadable body, or a missing delivery or event header. |
| 401 | The signature did not verify. |
| 413 | Body larger than 2 MiB. |
| 500 | Forwarding failed. GitHub retries, which is the only recovery: nothing else re-reads its event stream. |

The full contract is in [`github/openapi.yaml`](github/openapi.yaml).

Deployment detail: entity-service reads the caller identity from a header that the API gateway adds
after validating the bearer token. The component only sends `Authorization`; the OAuth2 transport
does this itself. If the connection is wired so it bypasses the gateway, entity-service sees no
identity and answers 401 on every forward.

Server timeouts are set explicitly (header 10s, read 30s, write 45s, idle 120s), with the write
timeout deliberately longer than the 30s forward timeout so a failing forward returns a retryable
500 instead of a cut connection. The OAuth2 token fetch has its own 15s timeout.

Code structure:

```text
csm-webhooks/
├── README.md
└── github/
    ├── .choreo/component.yaml     # Public REST endpoint, port 8080, schema = openapi.yaml
    ├── openapi.yaml               # Published API contract
    ├── .env.example               # Configuration template
    ├── cmd/server/main.go         # Only package main: config, routes, request handler
    └── internal/
        ├── webhook/verify.go      # HMAC-SHA256 signature verification, header names
        └── entity/client.go       # OAuth2 client-credentials client; forwards to entity-service
```

## Developer guide

### Prerequisites

- Go 1.26.6 or newer.
- For an end-to-end run: a reachable entity-service and an OAuth2 client-credentials client
  registered as an internal client there (`M2M_CLIENT_IDS`). For signature and request-handling
  work alone, the unit tests need neither.

### Build, run, test

```bash
cd operations/csm-webhooks/github
cp .env.example .env     # fill in the values; note: the process reads the real environment only
```

The component does **not** load a `.env` file itself; export the variables in your shell (or use
your tooling's env-file support) before starting it:

```bash
set -a; source .env; set +a
go run ./cmd/server           # listens on :$PORT (default 8080)

go build -o server ./cmd/server
go vet ./...
go test -race ./...
```

Locally there is no API gateway, so entity-service will not see a caller identity. The repository's
`scripts/csm-compose/gateway-shim` stands in for it: point `ENTITY_BASE_URL` at the shim, and the
shim at entity-service.

Send a signed test delivery with a placeholder secret:

```bash
BODY='{"action":"opened","issue":{}}'
SIG="sha256=$(printf '%s' "$BODY" | openssl dgst -sha256 -hmac "$GITHUB_WEBHOOK_SECRET" | awk '{print $NF}')"
curl -i -X POST http://localhost:8080/webhooks/github \
  -H "Content-Type: application/json" \
  -H "X-GitHub-Event: issues" \
  -H "X-GitHub-Delivery: 00000000-0000-0000-0000-000000000000" \
  -H "X-Hub-Signature-256: $SIG" \
  --data "$BODY"
curl -i http://localhost:8080/health
```

### Setting up a source repository

1. The repository must be mapped to an account in entity-service's data (an owner/repository to
   account mapping). **An unmapped repository is ignored silently**, which is not an error.
2. The repository's issue templates must apply the labels the sync classifies on; a template that
   references a missing label yields issues that can never be classified.
3. Create the webhook with payload URL = this component's public route, content type
   `application/json`, the same secret as `GITHUB_WEBHOOK_SECRET`, and events **Issues** and
   **Issue comments**.
4. The integration identity entity-service filters out as its own writes (its
   `GITHUB_INTEGRATION_LOGIN` setting) should be a dedicated bot account, not a person, or that
   person's activity becomes invisible to the sync.

### Adding a new webhook source

Create a sibling folder `operations/csm-webhooks/<source>/` on the same terms as `github/`:

1. New Go module and a single `cmd/server/main.go` (the only `package main`).
2. `internal/webhook`: verify the source's signature scheme over the raw body in constant time;
   return one error for every failure; refuse (never skip) verification when the secret is empty.
3. `internal/entity`: forward the verified delivery to a new internal entity-service endpoint using
   the OAuth2 client-credentials pattern from `github/internal/entity/client.go`, with bounded
   timeouts on both the token fetch and the forward. Map "already applied" to a 200 for the source.
4. Cap the request body size, set all HTTP server timeouts, and expose an unauthenticated
   `GET /health`.
5. Add `.choreo/component.yaml` (public visibility), `openapi.yaml`, and `.env.example`.
6. Add the new component's OAuth2 client id to entity-service's `M2M_CLIENT_IDS`, and implement the
   receiving endpoint and payload handling in entity-service (not here).
7. Add tests for signature verification, including that a re-encoded equivalent of the same JSON
   fails to verify.

## Configuration

### GitHub component (`github/`)

All variables are read from the process environment. There is no `.env` auto-loading. Every
variable marked required causes the process to exit at startup if empty. All example values are
placeholders.

| Variable | Required | Default | Description | Example |
|---|---|---|---|---|
| `GITHUB_WEBHOOK_SECRET` | Yes | none | HMAC key; must equal the secret configured on the repository webhook. This is the only authentication on the route, since GitHub cannot present a bearer token. Keep it out of source control. | `<webhook-secret>` |
| `ENTITY_BASE_URL` | Yes | none | Base URL of entity-service (trailing slash is trimmed). In deployment, route it through the API gateway. | `https://entity.example.com` |
| `ENTITY_TOKEN_URL` | Yes | none | OAuth2 token endpoint for the client-credentials grant. | `https://auth.example.com/oauth2/token` |
| `ENTITY_CLIENT_ID` | Yes | none | OAuth2 client id the component forwards as. Must be listed in entity-service's `M2M_CLIENT_IDS`. | `<client-id>` |
| `ENTITY_CLIENT_SECRET` | Yes | none | OAuth2 client secret. | `<client-secret>` |
| `ENTITY_SCOPES` | No | empty | Comma-separated OAuth2 scopes to request. | `scope-a,scope-b` |
| `PORT` | No | `8080` | HTTP listen port. | `8080` |

Fixed in code, not configurable: request body cap 2 MiB; forward timeout 30s; token-fetch timeout
15s; server timeouts as listed under Architecture.
