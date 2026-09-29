# Internal API users

`internal/auth` provides PBKDF2-hashed (10000 iterations, random salt) service-account credentials backed by the `alertintegration.integration_users` Cassandra table, plus an `auth.RequireAuth` middleware. It is not currently wired into any route: `/alertz` is reachable via a project-level exposure (gateway/network scoping) rather than a per-caller secret, so no route in this service enforces it today. Use `auth.RequireAuth` if a future endpoint needs per-caller authentication.

Secrets are never stored in plaintext; only the PBKDF2 hash and salt live in Cassandra. Rows have no expiry and there's no admin API or startup seeding, so accounts are managed one at a time with `cmd/user`, run against the same Cassandra instance and `CASSANDRA_*` env vars the server itself uses.

## cmd/user

```bash
go run ./cmd/user create -username <name> [-secret <value>]   # create or rotate; omit -secret to auto-generate one
go run ./cmd/user list                                        # show all internal users
go run ./cmd/user enable -username <name>                     # re-enable a disabled user
go run ./cmd/user disable -username <name>                    # disable a user
```

Load `CASSANDRA_*` from `.env` first (it's gitignored):

```bash
set -a && source .env && set +a
```

## Creating a user

```bash
go run ./cmd/user create -username webhook-integration-user
```

With no `-secret`, one is generated and printed once:

```
created internal user "webhook-integration-user"
secret (shown once, store securely): <generated-secret>
```

Copy it into your secrets manager immediately; it is not recoverable afterwards, only the hash is stored.

## Setting a specific secret

```bash
go run ./cmd/user create -username webhook-integration-user -secret 'my-chosen-secret'
```

Prefer reading the value from a file or env var (e.g. `-secret "$(cat secret.txt)"`) over typing it inline, since inline arguments land in your shell history.

## Rotating a secret

Re-running `create` for the same `-username` overwrites that row (upsert). To rotate: generate or pick a new secret, re-run the command, then update the caller's stored credential to match. The old secret stops working the moment the row is overwritten, so update the caller first if a brief outage during rotation isn't acceptable.

## Listing users

```bash
go run ./cmd/user list
```

```
USERNAME                       ENABLED  ITERATIONS  CREATED_AT
webhook-integration-user       true     10000       2026-09-29T10:00:00Z
```

Only metadata is shown; `secret_hash`/`salt` are never printed.

## Enabling / disabling a user

```bash
go run ./cmd/user disable -username webhook-integration-user
go run ./cmd/user enable -username webhook-integration-user
```

`auth.RequireAuth` rejects any request for a disabled user with a generic 401. Disabling keeps the row (and its hash) intact, so re-enabling doesn't require issuing a new secret.

## Authenticating

Once a route is wrapped with `auth.RequireAuth`, callers can authenticate with either header form:

```bash
# Bearer, base64("username:secret")
TOKEN=$(printf '%s:%s' webhook-integration-user '<secret>' | base64)
curl -X POST https://<host>/<protected-route> -H "Authorization: Bearer $TOKEN"

# Basic, via curl's -u
curl -X POST https://<host>/<protected-route> -u webhook-integration-user:<secret>
```

## Schema

```sql
CREATE TABLE IF NOT EXISTS alertintegration.integration_users (
  username     text PRIMARY KEY,
  secret_hash  text,      -- base64 PBKDF2-SHA256 derived key
  salt         text,      -- base64 random salt, unique per user
  iterations   int,       -- PBKDF2 iteration count used for this row
  enabled      boolean,
  created_at   timestamp
);
```
