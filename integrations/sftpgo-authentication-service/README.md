# SFTPGo Authentication Service

This project provides a secure, individual-account-based authentication and provisioning solution for [SFTPGo](https://github.com/drakkan/sftpgo), integrated with [Asgardeo](https://asgardeo.io) (or [WSO2 IS](https://github.com/wso2/wso2is)). 

**Key Goals & Capabilities:**
- **Centralized Identity**: Authenticates users against corporate credentials in Asgardeo, enforcing Multi-Factor Authentication (Password + OTP/TOTP).
- **Dynamic User Provisioning**: Uses SFTPGo's `Pre-login Hook` to check permissions and map Virtual Folders in real-time based on subscriptions or roles.
- **Granular Scope**: Supports distinct access patterns for internal staff (scoped to specific directories) vs. customers (access to subscription-based virtual folders).
- **Automated Management**: Automatically provisions missing physical directories via SFTPGo Admin APIs.


## Features

- **Dual Organization Support**: Separate Asgardeo organizations for internal and external users
- **Pre-Login User Provisioning**: Automatic user and folder configuration based on IdP roles
- **Keyboard-Interactive Authentication**: Multi-step authentication with MFA support (TOTP, OTP)
- **Dynamic Folder Management**: Automatic folder provisioning via SFTPGo Admin API
- **Session-Based Auth Flow**: Database-backed session management for authentication steps
- **Security Hardening**: SCIM injection protection, path traversal prevention, input validation

## Architecture

```text
┌─────────────┐         ┌──────────────────┐         ┌─────────────────┐
│   SFTPGo    │────────▶│  Auth Service    │────────▶│   Asgardeo      │
│   Server    │◀────────│  (This Service)  │◀────────│   (Internal)    │
└─────────────┘         └──────────────────┘         └─────────────────┘
                               │      ▲
                               │      │               ┌─────────────────┐
                               │      └──────────────▶│   Asgardeo      │
                               │                      │   (External)    │
                               ▼                      └─────────────────┘
                        ┌──────────────┐
                        │ PostgreSQL DB│
                        │  (Sessions)  │
                        └──────────────┘
```

## Project Structure

```text
.
├── cmd/server/main.go              # Application entry point
├── internal/
│   ├── config/                     # Configuration management
│   │   ├── config.go              # Environment variable loading
│   │   └── config_test.go         # Configuration tests
│   ├── handler/                    # HTTP request handlers
│   │   ├── handler.go             # Pre-login and auth handlers
│   │   └── utils.go               # Handler utilities
│   ├── service/                    # Business logic layer
│   │   ├── database.go            # Session management
│   │   ├── idp.go                 # Asgardeo integration
│   │   ├── sftpgo.go              # SFTPGo Admin API client
│   │   └── subscription.go        # External folder API
│   ├── models/                     # Data structures
│   ├── log/                        # Custom logger
│   └── util/                       # Shared utilities
├── db/migrations/                  # Database schema
├── openapi.yaml                    # API specification
└── Dockerfile                      # Container image
```

## Setup

### Prerequisites

- Go 1.24+
- PostgreSQL database
- Two Asgardeo organizations (for internal and external users)
- SFTPGo server with admin API access

> **Note**: This project requires Asgardeo or WSO2 Identity Server (WSO2 IS) as the Identity Provider (IdP). It utilizes proprietary app-native authentication APIs specific to these platforms/products.


### Enabled flows

The service has two independent use cases ("flows"). `ENABLED_FLOWS` selects which
ones a deployment runs, so it only needs the configuration of the flows it uses.

| Flow | Routes | Purpose |
|---|---|---|
| `web-attachment` | `POST /external-auth-hook` | Verifies a gateway JWT against a JWKS and returns an SFTPGo user (web attachment access path). Does not use the database, the identity provider, the SFTPGo admin API or the subscription/project APIs. |
| `sftp-channel` | `POST /prelogin-hook`, `POST /auth-hook` | SFTP login through the identity provider, with folder provisioning. Uses the database, the identity provider, the SFTPGo admin API and the subscription/project APIs. |

`ENABLED_FLOWS` is a comma-separated, trimmed, case-sensitive list of `web-attachment`
and `sftp-channel`. Unset or empty enables both flows. A value with an unknown name,
or with no name at all (for example `,`), stops the service at startup with an error
naming the bad value and the allowed ones. Duplicates are harmless.

Only the routes of an enabled flow are registered (a disabled flow's paths answer
`404`), and only an enabled flow's dependencies are built: with `web-attachment` alone
the service starts without a reachable database.

| Variable | Required when |
|---|---|
| `HOOK_API_KEY`, `DIR_PATH` | Always |
| `AUTH_JWKS_ENDPOINT` (https), `AUTH_ISSUER`, `ATTACHMENTS_DIR_PATH` | `web-attachment` (see the compatibility rule below) |
| `BASIC_AUTHENTICATOR_ID`, `INTERNAL_CLIENT_ID`, `INTERNAL_CLIENT_SECRET`, `OAUTH_CALLBACK_URL`, `INTERNAL_IDP_BASE_PATH`, `SCIM_SCOPE`, `CHECK_ROLE`, `SFTPGO_API_BASE` (https), `ADMIN_USER`, `ADMIN_KEY`, `FOLDER_PATH`, `SUBSCRIPTION_API`, `PROJECT_API`, `DB_CONN_STRING` | `sftp-channel` |

`AUTH_AUDIENCE` (comma-separated, OR-matched) is required when `ENABLED_FLOWS` is set and
includes `web-attachment` (the JWT validator refuses to start without an accepted
audience), and optional when `ENABLED_FLOWS` is unset. `AUTH_TOKEN_VALIDATOR_ENABLED`, the `EXTERNAL_*`
variables and the tuning variables with defaults (`PORT`, `LOG_LEVEL`, timeouts,
`EMAIL_REGEX_PATTERN`, `INTERNAL_USER_SUFFIX`) stay optional. A missing required
variable stops the service at startup with `missing critical environment variables:`
followed by the names, each flow-specific name annotated with the flow that needs it.

**`HOOK_API_KEY` is required in every configuration.** The service refuses to start
without it, and every hook route rejects a request whose `API-Key` header is missing
or wrong.

**Backward-compatibility rule.** When `ENABLED_FLOWS` is not set, behavior is the same
as before the variable existed, except that `HOOK_API_KEY` is now required: the
`sftp-channel` variables stay required, and the `web-attachment` variables
(`AUTH_JWKS_ENDPOINT`, `AUTH_ISSUER`, `ATTACHMENTS_DIR_PATH`) stay optional. If the JWKS
is not configured, the service starts with a warning and `/external-auth-hook` answers
`503`. The strict per-flow check for the `web-attachment` variables (including `AUTH_AUDIENCE`)
applies only when `ENABLED_FLOWS` is explicitly set and includes `web-attachment`. In
that strict mode the service also exits at startup if the JWT validator cannot be
built, instead of starting with `/external-auth-hook` answering `503`. An unreachable
JWKS host is not caught at startup (the key set is fetched in the background and the
failure is only logged); it surfaces as failed validations at request time.

Example, `web-attachment` only (no database, identity provider or SFTPGo admin API):
```bash
ENABLED_FLOWS="web-attachment"
HOOK_API_KEY="your-key"
DIR_PATH="/path/on/sftpgo/server"
ATTACHMENTS_DIR_PATH="/path/on/sftpgo/server/attachments"
AUTH_JWKS_ENDPOINT="https://your-idp/oauth2/jwks"
AUTH_ISSUER="https://your-idp/oauth2/token"
AUTH_AUDIENCE="aud-1"   # required in this configuration
```

Example, both flows (`ENABLED_FLOWS` may also be left unset): every variable in
`.env.example`, with `ENABLED_FLOWS="web-attachment,sftp-channel"`.

### Environment Variables

Copy `.env.example` to `.env` and configure the variables of the flows you enable:

#### Internal Organization
```bash
INTERNAL_CLIENT_ID="your_internal_client_id"
INTERNAL_CLIENT_SECRET="your_internal_client_secret"
INTERNAL_IDP_BASE_PATH="https://api.asgardeo.io/t/internal-org"
CHECK_ROLE="internal"          # Role display name for internal users
```

#### External Organization
```bash
EXTERNAL_CLIENT_ID="your_external_client_id"
EXTERNAL_CLIENT_SECRET="your_external_client_secret"
EXTERNAL_IDP_BASE_PATH="https://api.asgardeo.io/t/external-org"
```

#### Common IdP Configuration
```bash
OAUTH_CALLBACK_URL="https://your-app/callback"
SCIM_SCOPE="internal_user_mgt_view"                     # Scope required to fetch user details from the IdP
BASIC_AUTHENTICATOR_ID="..."                            # Authenticator ID for BasicAuthenticator in the IdP flow
```

#### Service Configuration
```bash
PORT="9090"                    # Server port
LOG_LEVEL="INFO"              # DEBUG, INFO, WARN, ERROR (default: INFO)
HTTP_TIMEOUT="15"             # HTTP client timeout (seconds)
HOOK_API_KEY="your-key"       # Required: API key every hook request must present in the API-Key header
# ENABLED_FLOWS="web-attachment,sftp-channel"   # Optional: see "Enabled flows" below (default: both)
# EMAIL_REGEX_PATTERN="..."   # Optional: custom email validation regex (uses built-in default if not set)
```

#### SFTPGo Configuration
```bash
SFTPGO_API_BASE="http://localhost:8080/api/v2"
ADMIN_USER="admin"
ADMIN_KEY="your-sftpgo-admin-api-key"
FOLDER_PATH="/path/on/sftpgo/server"
DIR_PATH="/path/on/sftpgo/server"
```

#### Database Configuration
```bash
DB_CONN_STRING="postgres://user:password@127.0.0.1:5432/sftpgo_sessions?sslmode=disable"
DB_MAX_OPEN_CONNS="25"         # Optional, default: 25
DB_MAX_IDLE_CONNS="25"         # Optional, default: 25
DB_CONN_MAX_LIFETIME="5m"      # Optional, default: 5m
```

#### Subscription APIs
```bash
SUBSCRIPTION_API="https://api.example.com/subscriptions?customerEmail=%s"
PROJECT_API="https://api.example.com/projects?projectKey=%s"
```

#### External-Auth Hook Configuration (web attachment access path)
Required by the `web-attachment` flow when `ENABLED_FLOWS` is set (see "Enabled flows").
Configures JWT validation for `/external-auth-hook`, used by a web app's
backend to obtain SFTPGo REST API tokens on behalf of its own already-authenticated
callers, by forwarding its own gateway-issued bearer token as the HTTP Basic-auth
password on SFTPGo's `GET /api/v2/user/token`. With `ENABLED_FLOWS` unset and these
variables missing, the service still starts and the pre-login/keyboard-interactive
hooks keep working unchanged; only `/external-auth-hook` is disabled (fails closed with
`503`) until configured.
```bash
AUTH_JWKS_ENDPOINT="https://your-idp/oauth2/jwks"
AUTH_ISSUER="https://your-idp/oauth2/token"
AUTH_AUDIENCE="aud-1,aud-2"           # Comma-separated, OR-matched; required when ENABLED_FLOWS includes web-attachment
AUTH_TOKEN_VALIDATOR_ENABLED="true"   # Optional, default true; "false" skips signature verification (local dev only)
```

### Database Setup

Apply the migrations to create the `sftpgo_auth_sessions` table and the session
cleanup procedure:

```bash
psql "postgres://youruser:yourpassword@127.0.0.1:5432/yourdatabase" -f db/migrations/001_create_sftpgo_auth_sessions_table.up.sql
psql "postgres://youruser:yourpassword@127.0.0.1:5432/yourdatabase" -f db/migrations/002_add_session_cleanup_procedure.up.sql
```

### Session Cleanup

Rows in `sftpgo_auth_sessions` are only ever removed lazily: `GetSession`
opportunistically deletes a session if it happens to read back that exact,
already-expired `request_id`. An abandoned keyboard-interactive login (or any
session no caller ever reads again) otherwise stays in the table forever.

Migration `002_add_session_cleanup_procedure.up.sql` adds a stored procedure,
`sftpgo_auth_cleanup_expired_sessions(batch_size INT DEFAULT 1000)`, that
deletes expired rows (`expires_at < now()`) in small batches, committing after
each batch so cleanup never holds one long-running lock over the whole table.

This procedure is not run from the Go service (no ticker/background
goroutine) -- it is scheduled to run **inside the database**, which keeps
working across service restarts and stays correct with multiple service
instances running at once:

- **If the `pg_cron` extension is already installed and enabled** on the
  target Postgres instance, the migration self-schedules the cleanup to run
  once daily via `cron.schedule(...)` (08:00 IST / 02:30 UTC, expressed as UTC
  since pg_cron runs on the server's configured timezone). No extra step is
  needed.
- **If `pg_cron` is not installed**, the migration does *not* attempt to
  install it (`CREATE EXTENSION pg_cron` requires superuser and isn't
  available on every managed Postgres instance, so trying it in a migration
  would break deployments that lack that access). In that case the
  scheduling step is silently skipped, and it becomes an **operational
  requirement** for whoever runs this service to schedule the cleanup
  externally -- e.g. a cron job on the host running:
  ```bash
  psql "$DB_CONN_STRING" -c "CALL sftpgo_auth_cleanup_expired_sessions();"
  ```
  or an equivalent job on a managed scheduler. Without either the pg_cron
  schedule or an external one, expired sessions accumulate indefinitely.

### Install Dependencies

```bash
go mod tidy
```

## Running the Service

### Development
```bash
go run ./cmd/server/main.go
```

### Production Build
```bash
go build -o sftpgo-authn-service ./cmd/server/main.go
./sftpgo-authn-service
```

### Docker
```bash
docker build -t sftpgo-authn-service .
docker run -p 9090:9090 --env-file .env sftpgo-authn-service
```

## API Endpoints

### POST /prelogin-hook
Pre-login user provisioning hook called by SFTPGo.
Requires the `API-Key` header (`HOOK_API_KEY`). Only served when the `sftp-channel` flow is enabled.

**Request:**
```json
{
  "id": 0,
  "username": "user@example.com"
}
```

**Response (200):**
```json
{
  "username": "user@example.com",
  "home_dir": "/data/user_example_com",
  "permissions": {
    "/": ["list"],
    "/project1": ["upload", "list", "download", "create_dirs", "delete"]
  },
  "status": 1,
  "virtual_folders": [
    {"name": "project1", "virtual_path": "/project1"}
  ]
}
```

### POST /auth-hook
Keyboard-interactive authentication hook.
Requires the `API-Key` header (`HOOK_API_KEY`). Only served when the `sftp-channel` flow is enabled.

**Request:**
```json
{
  "request_id": "req123",
  "step": 1,
  "username": "user@example.com",
  "answers": []
}
```

**Response:**
```json
{
  "instruction": "Enter your password:",
  "questions": ["Password:"],
  "echos": [false],
  "auth_result": 0
}
```

### POST /external-auth-hook
SFTPGo `external_auth_hook` for the web attachment access path (distinct from the
`/prelogin-hook` + `/auth-hook` SFTP-channel flow above). SFTPGo calls this on
every `GET /api/v2/user/token` attempt; the `password` field carries the caller's
JWT, which is independently verified against `AUTH_JWKS_ENDPOINT`/`AUTH_ISSUER`
(same validation rules as the CSM backend's own gateway-token check — issuer,
audience, expiration, clock-skew leeway). Requires the `API-Key` header
(`HOOK_API_KEY`). Only served when the `web-attachment` flow is enabled.

**Request** (sent by SFTPGo, not by the caller):
```json
{
  "username": "user@example.com",
  "password": "<jwt>",
  "protocol": "HTTP",
  "ip": "",
  "public_key": "",
  "keyboard_interactive": "",
  "tls_cert": ""
}
```

**Response (200) on success** — same shape as `/prelogin-hook`, username set to
the JWT's `email` claim:
```json
{
  "username": "user@example.com",
  "home_dir": "/data/user_example_com",
  "permissions": {"/": ["list"]},
  "status": 1
}
```

**Response (200) on failure** — an empty-username body, per SFTPGo's own
`external_auth_hook` contract (a non-200 status is treated as a *hook execution
error*, not a clean denial; see `denyExternalAuth` in
`internal/handler/external_auth.go` for the exact reasoning):
```json
{"username": "", "home_dir": "", "permissions": null, "status": 0}
```

## How It Works

### User Type Detection
The service automatically detects user type based on email domain:
- **@wso2.com** → Internal organization
- **Others** → External organization

### User Provisioning
1. SFTPGo calls `/prelogin-hook` before login
2. Service fetches user from appropriate Asgardeo org
3. Determines permissions based on role
4. Provisions required folders via SFTPGo Admin API
5. Returns user configuration to SFTPGo

### Authentication Flow
1. SFTPGo calls `/auth-hook` with step 1
2. Service initiates flow with appropriate Asgardeo org
3. User provides credentials through keyboard-interactive prompts
4. Service manages session state in database
5. MFA steps handled if required
6. Final auth result returned to SFTPGo

## Security Considerations

- **API Key Auth**: `API-Key` header validation for every hook; `HOOK_API_KEY` is required at startup
- **Input Validation**: Usernames validated for length and invalid characters
- **SCIM Injection Protection**: Quotes escaped in SCIM filter queries
- **Path Traversal Prevention**: Folder names validated for illegal characters
- **Prepared Statements**: All database queries use prepared statements
- **Secrets Management**: Sensitive config loaded from environment variables
- **Non-Root Container**: Docker image runs as unprivileged user (UID 10014)
- **HTTPS Support**: CA certificates included in container

## Testing

Run all tests:
```bash
go test ./...
```

Run with coverage:
```bash
go test -cover ./...
```

Static analysis:
```bash
go vet ./...
```

## Troubleshooting

### Common Issues

**Database connection fails:**
- Verify `DB_CONN_STRING` format: `postgres://user:password@host:port/dbname?sslmode=disable`
- Check PostgreSQL is running and accessible
- Ensure `sftpgo_auth_sessions` table exists

**Authentication fails:**
- Check appropriate org credentials (INTERNAL vs EXTERNAL)
- Verify IdP base paths are correct
- Review logs for specific error messages

**User not provisioned:**
- Ensure user exists in appropriate Asgardeo org
- For external users, ensure the user is having a valid subscription
- Check SCIM scope permissions of appropriate client credentials
- Verify SFTPGo Admin API credentials

## Development

### Adding New Features
1. Update models in `internal/models/`
2. Implement business logic in `internal/service/`
3. Add handlers in `internal/handler/`
4. Update `openapi.yaml`
5. Add tests

### Code Style
- Follow Go conventions
- Use meaningful variable names
- Add comments for exported functions
- Keep functions focused and small
