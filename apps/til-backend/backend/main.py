"""FastAPI backend for "Today I Learned" -- a company-wide feed of learnings
from customers, partners, and internal sources.

Two entry points call this one service: One WSO2's /me/til page, and
(separately, once registered) the Google Chat App's "+" Dialog. Neither
holds any logic of its own beyond collecting the three fields -- this
service is the single place a submission is validated, stored, and
notified, which is what guarantees the two entry points can never disagree
about what a valid entry looks like.
"""
from __future__ import annotations

import os

from dotenv import load_dotenv

load_dotenv()

from typing import Optional

from fastapi import Depends, FastAPI, Request, Response
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import JSONResponse

import db
import entity_client
from auth import require_auth
from chat_notify import notify_new_submission
from sanitize import sanitize_what_html
from validation import TIL_WHERE_OPTIONS, WHAT_MAX_LENGTH, validate_submission_payload

CORS_ALLOWED_ORIGIN = os.environ.get("CORS_ALLOWED_ORIGIN", "http://localhost:3000")
# Base URL of the One WSO2 webapp itself (NOT this backend) -- used only to
# build each entry's shareable link (.../knowledge-base/{id}) for the Chat
# notification. Optional: absent just means that link is omitted, same
# "unset key = quietly off" posture as TIL_CHAT_WEBHOOK_URL.
ONE_WSO2_BASE_URL = os.environ.get("ONE_WSO2_BASE_URL", "").rstrip("/")


async def lifespan(app: FastAPI):
    db.init_db()
    yield


app = FastAPI(lifespan=lifespan, title="Today I Learned Backend API", version="0.1.0")

app.add_middleware(
    CORSMiddleware,
    allow_origins=[CORS_ALLOWED_ORIGIN],
    allow_methods=["*"],
    allow_headers=["*"],
)


@app.get("/user-info")
async def user_info(user: dict = Depends(require_auth)):
    return {"email": user["email"], "displayName": user["name"], "canModerate": user["is_moderator"]}


@app.get("/customers/search")
async def search_customers(
    q: str = "",
    user: dict = Depends(require_auth),  # noqa: ARG001 -- every signed-in employee may search; nothing here is sensitive beyond what the form already shows
):
    """Backs the "Customer name" autocomplete when where == "Customer" --
    proxies entity_client.search_customers so the real customer list is
    typed correctly rather than free-text-guessed. Returns [] (not an
    error) when ENTITY_SERVICE_* isn't configured at all, so the frontend
    can fall back to plain free-text entry exactly as it did before this
    feature existed."""
    return await entity_client.search_customers(q.strip())


@app.get("/submissions")
async def list_submissions(
    limit: int = 100,
    cursor: Optional[str] = None,
    user: dict = Depends(require_auth),  # noqa: ARG001 -- every entry is readable by every signed-in employee
):
    capped_limit = min(max(limit, 1), 200)
    return db.list_submissions(limit=capped_limit, cursor=cursor)


@app.post("/submissions")
async def create_submission(request: Request, user: dict = Depends(require_auth)):
    try:
        body = await request.json()
    except Exception:
        return JSONResponse(status_code=400, content={"error": "Request body must be valid JSON."})
    error = validate_submission_payload(body)
    if error:
        return JSONResponse(status_code=400, content={"error": error})

    who = body["who"].strip()
    where = body["where"]
    where_detail = body.get("whereDetail")
    where_detail = where_detail.strip() if isinstance(where_detail, str) else None
    # Re-sanitized here even though the frontend editor already does --
    # never trust that a direct API call went through it. See sanitize.py.
    what = sanitize_what_html(body["what"])

    # submittedByEmail comes from the verified token by default -- this is
    # the one guarantee that makes "no anonymous entries" actually true
    # regardless of what the free-text "who" field says. The ONE exception:
    # the TIL Chat App authenticates as its own service account (never the
    # human who typed into the Dialog), so it separately asserts who the
    # real submitter was via `onBehalfOfEmail`. That field is honored ONLY
    # when the caller's own verified identity IS the configured Chat
    # service account -- for every other caller it's rejected outright
    # (never silently ignored, which could mask a misconfiguration letting
    # someone impersonate another employee).
    on_behalf_of = body.get("onBehalfOfEmail")
    if on_behalf_of is not None:
        if not user["is_chat_service_account"]:
            return JSONResponse(
                status_code=403,
                content={"error": "onBehalfOfEmail is only accepted from the configured Chat service account."},
            )
        if not isinstance(on_behalf_of, str) or "@" not in on_behalf_of:
            return JSONResponse(status_code=400, content={"error": "onBehalfOfEmail must be a valid email."})
        submitted_by_email = on_behalf_of
    else:
        submitted_by_email = user["email"]

    submission = db.create_submission(
        who=who, where=where, what=what, submitted_by_email=submitted_by_email, where_detail=where_detail
    )

    entry_url = f"{ONE_WSO2_BASE_URL}/knowledge-base/{submission['id']}" if ONE_WSO2_BASE_URL else None
    await notify_new_submission(who=who, where=where, what=what, where_detail=where_detail, entry_url=entry_url)

    return submission


@app.get("/submissions/{submission_id}")
async def get_submission(
    submission_id: str,
    user: dict = Depends(require_auth),  # noqa: ARG001 -- same "every entry, every employee" rule as the list
):
    submission = db.get_submission(submission_id)
    if submission is None:
        return JSONResponse(status_code=404, content={"error": "Entry not found."})
    return submission


@app.delete("/submissions/{submission_id}")
async def delete_submission(submission_id: str, user: dict = Depends(require_auth)):
    existing = db.get_submission(submission_id)
    if existing is None:
        return JSONResponse(status_code=404, content={"error": "Entry not found."})

    # Delete is allowed for a moderator (TIL_MODERATOR_GROUP) OR the entry's
    # own submitter -- checked against the verified submittedByEmail, never
    # the free-text `who` field, for the same reason that field was never
    # trusted for identity in the first place.
    is_owner = user["email"] == existing["submittedByEmail"]
    if not (user["is_moderator"] or is_owner):
        return JSONResponse(status_code=403, content={"error": "Not authorized to delete entries."})

    db.delete_submission(submission_id)
    return Response(status_code=204)


@app.get("/health")
async def health():
    return {"status": "ok"}
