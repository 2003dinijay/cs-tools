# TIL Chat App — the "+" button Dialog

This is the one piece I genuinely cannot build or test from here: registering
a Google Chat App requires a Google Cloud project and Google Workspace admin
console access, neither of which I have. What's here is real, ready-to-deploy
code — someone with that access needs to paste it in and register it, which
should take minutes once you have access, not days.

## What this is

A single Google Apps Script project (`Code.gs` in this folder) that:

1. Registers a slash command / `+` menu item, **"Share a learning"**.
2. Opens a Dialog (card form) with the same three fields as the One WSO2
   page: Who, Where (dropdown), What (5000-char limit, enforced both in the
   form's `maxLength` hint and again server-side — the server-side check in
   `til-backend/validation.py` is the one that actually matters).
3. On submit, POSTs straight to `til-backend`'s `POST /submissions` — the
   exact same endpoint the One WSO2 page calls. There is exactly one write
   path into the system regardless of which entry point was used.
4. The caller's identity comes from `event.user.email`, which Google Chat
   provides on every Dialog submission automatically — this is what makes
   "no anonymous entries" true for this entry point too, with nothing extra
   to build.

## Deploy steps (needs Google Workspace admin access)

1. Create the "Today I Learned" Chat Space, configured so only the bot can
   post (Space settings → restrict who can post to "Managers only", with
   the bot as the sole manager) — this is what satisfies "disable replies."
2. In [Google Apps Script](https://script.google.com), create a new
   **standalone** project, paste in `Code.gs`, and set the script property
   `TIL_BACKEND_URL` to the deployed til-backend's base URL.
3. In the project's `appsscript.json` (Project Settings → Show
   "appsscript.json"), merge in the contents of `appsscript.json` from this
   folder. **This only marks the project as a Chat app** (`addOns.chat` is
   deliberately an empty object — Chat apps don't take name/avatar/slash-
   command config through the manifest at all, unlike other Workspace
   add-ons; see [Configure a Google Chat app](https://developers.google.com/workspace/add-ons/chat/configure)).
4. Deploy → New deployment → **Add-on** → **Chat app**, which opens the
   Google Cloud Console's Chat API configuration page. Enter the actual
   app details there -- the manifest doesn't carry them:
   - **App name**: Today I Learned
   - **Avatar URL**: any square icon (not read from the manifest)
   - **Description**: Share and browse learnings from customers, partners, and internal sources.
   - **Interactive features** → **Slash commands** → add one:
     name `/learn`, command ID `1`, description "Share something you
     learned", trigger type **Opens a dialog**.
   - **Visibility**: internal-only, scoped to the WSO2 domain -- this is
     the step that specifically needs Workspace admin console access.
   - **Connection settings** → **Triggers**: point the message/added-to-
     space callbacks at this script's `onMessage`/`onAddedToSpace`
     functions (`Code.gs` already defines `onMessage` and `onAppCommand`
     to match Apps Script's expected callback names).
5. Add the deployed Chat App to the "Today I Learned" Space.

## Why a service-account-style call, not the human's own token

The Apps Script runs as whoever deployed it (or a dedicated service
account, if one is set up) — it does **not** have the submitting user's
Asgardeo token. So the call to `til-backend` here authenticates with a
**separate service credential**, not a user token, the same architectural
split the PAR Legacy Migration Tool already uses in production (a human's
own session vs. a dedicated service account hitting a different group).

This is implemented and tested, not just flagged — see `test_main.py`:

- The service account's own token identifies it as `til-chat-sa@...` (or
  whatever `TIL_CHAT_SERVICE_ACCOUNT_EMAIL` is set to).
- It separately asserts `onBehalfOfEmail` (`event.user.email`, which Google
  Chat provides on every Dialog event) in the request body.
- `til-backend` honors `onBehalfOfEmail` **only** when the authenticated
  caller's own identity exactly matches `TIL_CHAT_SERVICE_ACCOUNT_EMAIL` —
  every other caller gets a 403, not silent ignoring, if they ever send
  that field. This is what prevents any other caller from impersonating
  a different employee's submission.
- `TIL_CHAT_SERVICE_ACCOUNT_EMAIL` is optional and absent by default — until
  it's set to the real deployed service account's email, `onBehalfOfEmail`
  is rejected from everyone, which is the safe starting state.

Provisioning the actual service-account credential (an Asgardeo
application, or whatever auth Apps Script ends up using to call
`til-backend`) is still a real step for whoever sets this up — but the
backend-side trust boundary around it is already built and covered by
tests.
