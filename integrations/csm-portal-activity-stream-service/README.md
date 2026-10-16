# CSM Portal Activity Stream Service

A small Go service that pushes live "this case changed" notifications to the CSM portal
over Server-Sent Events (SSE). It consumes case events from the event bus and fans a
minimal signal out to every browser currently watching the affected case.

- [Architecture](#architecture)
- [Developer guide](#developer-guide)
- [Configuration](#configuration)
- [Known limitations](#known-limitations)

## Architecture

### Purpose and position

When a case gets a comment, changes status, is assigned, changes severity or is
acknowledged, an open case page should refresh without polling. This service owns that
push channel: one long-lived `GET /cases/{id}/activities/stream` connection per watching
browser tab. For how it fits into the wider platform (portal, backends, entity service,
event backbone), see the
[platform architecture](../../apps/csm-portal/README.md#architecture).

```text
 entity service ──publishes──▶ Event Hub topic "case-events" (Kafka protocol)
                                      │  one consumer group PER REPLICA, starting at the latest offset
                                      ▼
                          caseevents.Handler  ──▶  stream.BroadcastHub (in process, per case id)
                                                         │
 browser (fetch-based EventSource) ◀── SSE ── StreamCaseActivities ◀─┘
          │  x-jwt-assertion (+ x-user-id-token)
          └─ gateway ──▶ this service ──GET /cases/{id}──▶ entity service (read-access check)
```

The service is split from the portal backend for two reasons: the REST listener's write
and idle timeouts would kill long-lived streams, and every replica must see every event
(see [Scaling model](#scaling-model)), so the stream scales independently of the
backend.

### Interactions

| Direction | Peer | Detail |
|---|---|---|
| Inbound | Browser, via the API gateway | `GET /cases/{id}/activities/stream` on the stream port (default 9092). Needs a valid `x-jwt-assertion` header. |
| Inbound | Health probe | `GET /health` on the health port (default 8080), JSON `{"status":"ok"}`, no auth. |
| Inbound | Event Hub | Case events from the `case-events` topic (Kafka protocol over TLS with SASL/PLAIN). |
| Outbound | Entity service | `GET /cases/{id}` with the caller's identity token, once per new stream, to confirm the caller may read the case. OAuth2 client credentials for the service itself. |

The service stores nothing. State is the in-memory subscriber map.

### Behaviour

- **Events broadcast**: `case.comment_added`, `case.status_changed`, `case.assigned`,
  `case.severity_changed`, `case.acknowledged`. Other event types are ignored. A record
  that cannot be decoded is logged and dropped.
- **Wire format**: each broadcast is `event: case_updated` with `data:` a compact JSON
  object `{"caseId","type","timestamp"}` (`type` is the event type; `timestamp` is
  omitted if the event had none). It never contains comment text, field values, names or
  emails. A `: ping` comment is sent every 15 s to keep proxies from closing idle
  connections. Clients are expected to refetch the case on `case_updated`.
- **Authorization**: the caller must hold a valid token and must be able to read the
  specific case. The service calls the entity service's get-case operation (forwarding
  `x-user-id-token`, falling back to the `x-jwt-assertion` value) before registering the
  subscription, so an unauthorized caller learns nothing about the case.
- **Authentication**: `middleware.Auth` validates `x-jwt-assertion` on every request
  except `GET /health`: signature (keys fetched from `AUTH_JWKS_ENDPOINT`), issuer,
  expiry (required), an accepted audience, and non-empty `email` and `userid` claims.
  This is defense in depth behind the gateway. Browsers cannot set custom headers with
  the native `EventSource`, so the web app uses a fetch-based polyfill.
- **Backpressure**: each subscriber has a buffer of 4 messages and publishing never
  blocks, so a slow client cannot stall the consumer (it may miss signals; the next
  refetch catches up).
- **Consumer semantics**: offsets are committed after each record is handled, whatever
  the outcome. There is no retry or dead-letter tier because the handler only logs and
  fans out.

### Scaling model

The broadcast hub only knows about subscribers connected to its own process, so every
replica must independently observe every event. Each replica therefore joins its own
consumer group, named `<EVENT_HUB_CONSUMER_GROUP>-replica-<hostname>` (hostname is the pod
name in container platforms; a random id is used if it is unavailable), and starts at the
latest offset so a new group does not replay retained history as a burst of stale
notifications. Trade-offs: a redeploy (new pod name) creates a new group, and old groups
accumulate on the broker because there is no API to delete them.

### Optional event bus and two listeners

If `EVENT_HUB_BROKER` is unset the service still starts, but serves only the health
listener: the stream listener is not started and no consumer runs. (The stream handler
itself would answer 503 "Live updates are not available right now." if it were reached
with no hub.) With it set, two HTTP servers run:

| Listener | Port variable (default) | Timeouts | Routes |
|---|---|---|---|
| Health | `HEALTH_PORT` (8080) | header 10 s, read/write 30 s, idle 60 s | `GET /health` |
| Stream | `STREAM_PORT` (9092) | header 10 s, read 30 s, write and idle disabled | `GET /cases/{id}/activities/stream` |

Middleware, outermost first: `SecurityHeaders` (nosniff, upgrade-insecure-requests,
HSTS), `CORS` (fail-closed allow-list; sits before `Auth` because a preflight carries no
token), `CorrelationID` (reads or generates `X-CSM-Correlation-ID`, echoes it, forwards it
to the entity service), `Auth`, `Logger`. CORS allows `GET, OPTIONS` and the headers
`Content-Type, Authorization, x-jwt-assertion, x-user-id-token, X-CSM-Correlation-ID`.
A gateway normally supplies CORS itself; the in-service CORS matters for local
development and as defense in depth. Shutdown on `SIGINT`/`SIGTERM` is graceful (15 s).

### Project structure

```text
csm-portal-activity-stream-service/
├── cmd/server/main.go        # Wiring: env, entity client, consumer, hub, both listeners
├── internal/
│   ├── apierror/             # Typed upstream error
│   ├── caseevents/           # Handler: decode event, broadcast the five case event types
│   ├── entity/               # Minimal entity-service client (GetCase only)
│   ├── eventbus/             # kafka-go consumer (manual commit, configurable start offset)
│   ├── events/               # Envelope and event type constants (hand-synced copy)
│   ├── handler/              # StreamCaseActivities (SSE) and response helpers
│   ├── middleware/           # auth, cors, correlation, logger, security_headers
│   └── stream/               # BroadcastHub: per-case in-process pub/sub
├── .choreo/component.yaml    # Two endpoints: health (8080) and stream (9092)
├── openapi.yaml              # Health endpoint contract
├── openapi-stream.yaml       # SSE endpoint contract
└── .env.example
```

## Developer guide

### Prerequisites

- Go `1.26.0` or newer (see `go.mod`) - [install](https://go.dev/doc/install)
- A reachable entity service and OAuth2 client for it (always required at startup)
- An Event Hub namespace or other Kafka endpoint with TLS and SASL/PLAIN for the stream
  itself. The consumer always dials with TLS 1.2+ and SASL/PLAIN, so a plain local broker
  does not work.
- A JWKS endpoint, unless you disable signature validation for local development.

### Build and run

```bash
# from integrations/csm-portal-activity-stream-service
go build -o server ./cmd/server
cp .env.example .env     # fill in the values; never commit .env
go run ./cmd/server
```

`.env` is loaded from the working directory if present; variables already set in the
process environment win. Missing required variables cause an immediate exit.

For local development without an identity provider, set `AUTH_TOKEN_VALIDATOR_ENABLED=false`.
The token is then only decoded, not verified, but the three `AUTH_*` variables must still
be set (any placeholder), and the token must carry `email` and `userid` claims. Never use
this mode outside local development. Allow your dev web app's origin with
`STREAM_CORS_ALLOWED_ORIGINS`.

```bash
curl http://localhost:8080/health
# Replace <jwt> with a token carrying "email" and "userid" claims, and <case-uuid> with a case id
curl -N -H "x-jwt-assertion: <jwt>" \
  http://localhost:9092/cases/<case-uuid>/activities/stream
```

You should see `: ping` every 15 s, and `event: case_updated` when a matching event is
published for that case. A non-UUID id returns 400; a case the caller cannot read returns
the mapped upstream error.

### Test and lint

```bash
go vet ./...
go test -race ./...
```

Optional security scan: `go install github.com/securego/gosec/v2/cmd/gosec@latest` then
`gosec -fmt=text ./...`. There is no Makefile and no Git hook for this module.

### Extending

To stream another event type:

1. Add the type constant to `internal/events/events.go`. The `Envelope` is a hand-synced
   copy shared in shape with the notification service and the entity service; keep them
   in step.
2. Add the type to the `switch` in `internal/caseevents/handler.go`.
3. Update `openapi-stream.yaml` if the payload shape changes.
4. Check that the web clients refetch on any `case_updated` event (they currently do).

Keep payloads free of comment text, field values and personal data, and keep logs to ids
and error summaries.

## Configuration

Environment variables only, optionally loaded from `.env` (process environment wins).
Placeholders below are examples, not real values.

| Variable | Required | Default | Description | Example |
|---|---|---|---|---|
| `OAUTH2_TOKEN_URL` | Yes | none | OAuth2 token endpoint for entity-service calls. | `https://auth.example.com/oauth2/token` |
| `OAUTH2_CLIENT_ID` | Yes | none | OAuth2 client id (client-credentials grant). | `<client-id>` |
| `OAUTH2_CLIENT_SECRET` | Yes | none | OAuth2 client secret. Secret. | `<client-secret>` |
| `CUSTOMER_ENTITY_BASE_URL` | Yes | none | Base URL of the entity service. | `https://entity.example.com` |
| `CUSTOMER_ENTITY_SCOPES` | No | none | Comma-separated OAuth2 scopes for the entity service. | `scope-a,scope-b` |
| `AUTH_JWKS_ENDPOINT` | Yes | none | JWKS URL used to verify incoming tokens. Fetched at startup (startup panics if unreachable while validation is enabled). | `https://idp.example.com/oauth2/jwks` |
| `AUTH_ISSUER` | Yes | none | Expected `iss` claim. | `https://idp.example.com/oauth2/token` |
| `AUTH_AUDIENCE` | Yes | none | Comma-separated accepted `aud` values; the token needs at least one. | `<audience-a>,<audience-b>` |
| `AUTH_TOKEN_VALIDATOR_ENABLED` | No | enabled | Only the exact string `false` disables signature, issuer, expiry and audience checks (decode only). Local development only. | `false` |
| `EVENT_HUB_BROKER` | No (feature gate) | unset | Kafka-compatible bootstrap address. Unset means no stream listener and no consumer. | `<namespace>.servicebus.windows.net:9093` |
| `EVENT_HUB_CONNECTION_STRING` | Yes, once `EVENT_HUB_BROKER` is set | none | Namespace-level shared-access connection string, used as the SASL/PLAIN password. Secret. | `<connection-string>` |
| `EVENT_HUB_TOPIC` | Yes, once `EVENT_HUB_BROKER` is set | none | Topic carrying case events. | `case-events` |
| `EVENT_HUB_CONSUMER_GROUP` | No | `csm-portal-activity-stream-service` | Base consumer group name; each replica appends `-replica-<hostname>`. | `csm-portal-activity-stream-service` |
| `STREAM_PORT` | No | `9092` | SSE listener port (bare number 1 to 65535). | `9092` |
| `HEALTH_PORT` | No | `8080` | Health listener port (bare number 1 to 65535). | `8080` |
| `STREAM_CORS_ALLOWED_ORIGINS` | No | none (deny all cross-origin) | Comma-separated browser origins allowed on the stream listener. | `http://localhost:3001` |
| `CORS_ALLOWED_ORIGINS` | No | none (deny all cross-origin) | Same, for the health listener. | `http://localhost:3001` |

Notes:

- A non-numeric or out-of-range port exits at startup.
- Clock skew for token expiry is fixed at 5 s; heartbeat (15 s), subscriber buffer (4) and
  shutdown timeout (15 s) are constants in code.
- The deployment descriptor `.choreo/component.yaml` declares both endpoints with public
  network visibility, so access control relies on the gateway and the token checks above.

## Known limitations

- No per-user or per-replica cap on concurrent SSE connections; a hostile client could
  exhaust goroutines or file descriptors. Add admission control or confirm a platform-level
  limit before exposing this to untrusted clients.
- The event `Envelope` is hand-synced across Go modules; changes are propagated manually.
- A redeploy (new pod name) starts a fresh group at the latest offset, so events published
  while the old pod was gone are missed and the old group is left behind on the broker.
  Clients that reconnect should refetch the case. A restart of the same pod resumes from
  its committed offset.
