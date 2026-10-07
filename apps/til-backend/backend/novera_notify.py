# Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
#
# WSO2 LLC. licenses this file to you under the Apache License,
# Version 2.0 (the "License"); you may not use this file except
# in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied.  See the License for the
# specific language governing permissions and limitations
# under the License.

"""Broadcasts a new submission into every connected user's own 1:1 DM with
Novera (WSO2's internal Google Chat AI agent) -- a SEPARATE channel from
chat_notify.py's own Chat Space webhook post, which stays untouched. Novera
owns the actual broadcast (it knows every connected user's own DM space);
this module's only job is the one outbound call telling it a new entry
exists, same "single write path notifies everyone" posture as
chat_notify.py already has for the Space.

Same "absent key = quietly off" posture as the rest of this service:
unset NOVERA_NOTIFY_URL means this is a no-op, not an error -- share/read
behavior is completely unaffected either way.

Config (env):
    NOVERA_NOTIFY_URL     Novera's POST /internal/notifications/til-entry
                          endpoint. Absent = this module does nothing.

    CHOREO_TOKEN_URL, NOVERA_NOTIFY_CLIENT_ID, NOVERA_NOTIFY_CLIENT_SECRET
                          Choreo API Gateway sits in front of NOVERA_NOTIFY_URL
                          and is the ONLY auth layer now (confirmed live: with
                          gateway subscription auth actually enforced, Novera's
                          own route no longer needs an application-level
                          secret on top of it). These three fetch a
                          client_credentials access token from Choreo's token
                          endpoint for an application subscribed to Novera's
                          API, sent as a standard `Authorization: Bearer`
                          header -- same client_credentials shape as
                          entity_client.py's ENTITY_SERVICE_* vars, just a
                          different downstream and header name (Choreo
                          Connect's own 401 response names "Bearer" as the
                          expected scheme). Absent = the call is sent without
                          an Authorization header, same "unset key = quietly
                          off" posture as the rest of this module -- Choreo
                          will then reject it at the gateway, logged below
                          same as any other rejection.
"""
from __future__ import annotations

import html as html_module
import os
import time

import httpx

from sanitize import what_for_chat

NOVERA_NOTIFY_URL = os.environ.get("NOVERA_NOTIFY_URL", "")
CHOREO_TOKEN_URL = os.environ.get("CHOREO_TOKEN_URL", "")
NOVERA_NOTIFY_CLIENT_ID = os.environ.get("NOVERA_NOTIFY_CLIENT_ID", "")
NOVERA_NOTIFY_CLIENT_SECRET = os.environ.get("NOVERA_NOTIFY_CLIENT_SECRET", "")

# (token, expires_at_monotonic) -- module-level, process-wide. Same shape as
# entity_client.py's own _token_cache, re-fetched a minute before real expiry
# rather than on every call.
_token_cache: dict[str, float | str] = {}


async def _get_gateway_token(client: httpx.AsyncClient) -> str | None:
    """None when the three CHOREO/NOVERA_NOTIFY_* gateway-auth vars aren't
    configured -- caller sends the request without an Authorization header
    in that case, same "unset key = quietly off" posture as the rest of
    this module."""
    if not (CHOREO_TOKEN_URL and NOVERA_NOTIFY_CLIENT_ID and NOVERA_NOTIFY_CLIENT_SECRET):
        return None

    cached_token = _token_cache.get("token")
    cached_expiry = _token_cache.get("expires_at")
    if isinstance(cached_token, str) and isinstance(cached_expiry, float) and time.monotonic() < cached_expiry:
        return cached_token

    resp = await client.post(
        CHOREO_TOKEN_URL,
        data={"grant_type": "client_credentials"},
        auth=(NOVERA_NOTIFY_CLIENT_ID, NOVERA_NOTIFY_CLIENT_SECRET),
        timeout=10.0,
    )
    resp.raise_for_status()
    payload = resp.json()
    token = payload["access_token"]
    expires_in = int(payload.get("expires_in", 3600))
    _token_cache["token"] = token
    _token_cache["expires_at"] = time.monotonic() + max(expires_in - 60, 30)
    return token


async def notify_novera(
    title: str, who: str, where: str, what: str, where_detail: str | None = None, entry_url: str | None = None
) -> None:
    if not NOVERA_NOTIFY_URL:
        return
    # Payload build moved INSIDE the try -- what_for_chat (or anything else
    # here) raising would otherwise propagate straight out of this function
    # uncaught, turning an already-saved submission into a 500 for the
    # caller despite db.create_submission and the Space webhook post both
    # having already succeeded. Same reasoning extends the except clause to
    # Exception broadly, not just httpx.RequestError -- this call is
    # best-effort by design (see module docstring), so nothing in it should
    # ever be allowed to fail the request it's attached to.
    try:
        payload = {
            # Escaped for the same reason chat_notify.py escapes this exact
            # field for the Space card: `who` is free text this service only
            # .strip()s, never HTML-escapes, and Novera's own card embeds it
            # directly in a textParagraph. Defense in depth -- Novera's own
            # broadcast code escapes it too, but this shouldn't rely on that
            # alone any more than chat_notify.py relies on the frontend editor
            # alone.
            # Same reasoning as "who" above -- free text til-backend only
            # .strip()s, never HTML-escapes.
            "title": html_module.escape(title),
            "who": html_module.escape(who),
            "where": where,
            "whereDetail": where_detail,
            # Same Chat-markup subset conversion as the Space card (Novera's own
            # broadcast posts through the same Chat API cardsV2 primitive) --
            # reusing it here keeps the two notification channels looking like
            # the same product instead of reimplementing the format translation.
            "what": what_for_chat(what),
            "entryUrl": entry_url,
        }
        async with httpx.AsyncClient() as client:
            headers = {}
            gateway_token = await _get_gateway_token(client)
            if gateway_token:
                headers["Authorization"] = f"Bearer {gateway_token}"
            response = await client.post(
                NOVERA_NOTIFY_URL,
                json=payload,
                headers=headers,
                timeout=10,
            )
        # Not raised -- best-effort, same as chat_notify.py's own webhook
        # posts. A rejected response (e.g. Novera's secret doesn't match)
        # must still be visible to an operator, not just silently eaten.
        if response.status_code >= 300:
            print(
                f"novera_notify: Novera rejected the broadcast (status {response.status_code}): "
                f"{response.text[:200]}",
                flush=True,
            )
    except Exception as exc:
        print(f"novera_notify: failed to reach Novera: {exc}", flush=True)
