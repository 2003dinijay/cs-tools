# til-backend

Backend for "Today I Learned" — a company-wide feed of learnings from
customers, partners, and internal sources. Two entry points call this one
service: the One WSO2 `/me/til` page, and (once set up — see
`chat_app/README.md`) the Google Chat App's `+` Dialog.

## Run locally

```bash
python3 -m venv venv
source venv/bin/activate
pip install -r requirements.txt
cp .env.example .env   # fill in DB_PASSWORD, ASGARDEO_JWKS_URL / ASGARDEO_ISSUER / TIL_MODERATOR_GROUP

# Prerequisite, once per database -- db.py only ever verifies this table
# exists, it never creates it:
mysql -h localhost -u root -p -e "CREATE DATABASE IF NOT EXISTS til"
mysql -h localhost -u root -p til < sql/create_til_tables.sql

uvicorn main:app --reload --port 8077
```

Without a real `.env`, every endpoint fails closed with 503 ("Authentication
is not configured") rather than silently allowing requests through — verified
live, not assumed. Starting the app against a database missing
`til_submissions` fails loudly too (a clear `RuntimeError` naming the exact
SQL file to run), rather than silently creating the table itself.

## Test

```bash
python3 -m pytest -q
```

Validation rules (`test_validation.py`), HTML sanitizing (`test_sanitize.py`),
the MySQL storage layer (`test_db.py`), the request-handling rules —
including the `onBehalfOfEmail` trust boundary — in `test_main.py`, and the
Chat notification card builder (`test_chat_notify.py`). The DB-touching
suites run against a real local `til_test` database (`conftest.py` creates it
and its schema automatically, and truncates between tests) — the same MySQL
server as the real `til` database, never that database itself.

## API

| Method | Path | Notes |
|---|---|---|
| GET | `/user-info` | `{ email, displayName, canModerate }` |
| GET | `/customers/search?q=` | Customer-name suggestions for the submission form's "Where: Customer" field, proxied from [entity-service](../../../entity-service)'s `POST /accounts/search`. `[]` (not an error) when `ENTITY_SERVICE_*` isn't configured — the frontend falls back to free-text entry. |
| GET | `/submissions?limit=&cursor=` | Newest first; `nextCursor` is `null` on the last page |
| POST | `/submissions` | `{ who, where, what }`; `submittedByEmail` always comes from the verified token (see `onBehalfOfEmail` exception below) |
| GET | `/submissions/{id}` | One entry by id |
| DELETE | `/submissions/{id}` | 403 unless the caller is in `TIL_MODERATOR_GROUP` |
| GET | `/health` | For Choreo/liveness checks |

## Design notes

- **Storage**: MySQL (`db.py`, via PyMySQL). The app never creates
  `til_submissions` itself; `sql/create_til_tables.sql` is a prerequisite an
  operator runs once per database, and `init_db()` just verifies it exists,
  failing loudly (not auto-creating) if it doesn't.
- **Auth**: independently verifies the caller's JWT signature against
  Asgardeo's own JWKS, never trusts a forwarded header blindly. Fails closed
  (503) if `ASGARDEO_JWKS_URL`/`ASGARDEO_ISSUER`/`TIL_MODERATOR_GROUP` are
  unset, rather than defaulting open.
- **No anonymous entries**: `submittedByEmail` is derived from the verified
  token, never the request body — with one deliberate, narrow exception for
  the Chat App's service account (`onBehalfOfEmail`), covered in
  `chat_app/README.md` and tested in `test_main.py`.
- **Customer search** (`entity_client.py`): a thin proxy to entity-service's
  `POST /accounts/search`, used by the "Where: Customer" autocomplete on the
  submission form. Supports both a real `client_credentials` grant
  (`ENTITY_SERVICE_TOKEN_URL`/`_CLIENT_ID`/`_CLIENT_SECRET`, requires the
  client to be on entity-service's own `AUTH_INTERNAL_CLIENT_IDS` allowlist)
  and an `ENTITY_SERVICE_DEV_STATIC_TOKEN` shortcut for local development
  only — see the file's own doc comment for why that shortcut is safe
  *only* locally. Unset entirely = the endpoint returns `[]`, not an error.
- **Chat notification**: `chat_notify.py` posts a card into the TIL Space via
  a simple Incoming Webhook (not a full Chat App) — created in the Space's own
  settings in under a minute, no Google Cloud project needed. Absent
  `TIL_CHAT_WEBHOOK_URL` is a silent no-op, not an error, matching One WSO2's
  own "unset key = quietly off" convention.
- **The `+` button Dialog itself**: needs a real Google Chat App, which needs
  Google Cloud + Workspace admin console access. `chat_app/` has real,
  ready-to-paste code (`Code.gs`, `appsscript.json`) and exact deploy steps —
  this is the one piece someone with that access needs to actually register.

## Still needed before a real deployment

1. A real Choreo (or equivalent) deployment, with `ASGARDEO_JWKS_URL`,
   `ASGARDEO_ISSUER`, and `TIL_MODERATOR_GROUP` set to the real values —
   the moderator group especially needs an actual decision on who moderates.
2. The Chat App itself registered (`chat_app/README.md`).
3. `getServiceAccountToken()` in `chat_app/Code.gs` filled in against
   whatever this org's real service-account auth flow turns out to be, and
   `TIL_CHAT_SERVICE_ACCOUNT_EMAIL` set on the backend to match.
4. A real MySQL database provisioned per environment (staging/prod), with
   `sql/create_til_tables.sql` run against it once and `DB_HOST`/`DB_PORT`/
   `DB_USER`/`DB_PASSWORD`/`DB_NAME` set to match.
5. An entity-service `client_id`/`client_secret` pair allowlisted in
   `AUTH_INTERNAL_CLIENT_IDS` for whichever environment `ENTITY_SERVICE_BASE_URL`
   points at, if the customer-search feature is wanted in that environment.
