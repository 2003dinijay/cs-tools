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
    NOVERA_NOTIFY_SECRET  Shared secret sent as the x-til-notification-secret
                          header. Novera's own route fails closed (503) if
                          it hasn't been configured with the same value, so
                          an empty/missing secret here just means Novera
                          rejects the call -- logged, never raised.
"""
from __future__ import annotations

import html as html_module
import os

import httpx

from sanitize import what_for_chat

NOVERA_NOTIFY_URL = os.environ.get("NOVERA_NOTIFY_URL", "")
NOVERA_NOTIFY_SECRET = os.environ.get("NOVERA_NOTIFY_SECRET", "")


async def notify_novera(
    who: str, where: str, what: str, where_detail: str | None = None, entry_url: str | None = None
) -> None:
    if not NOVERA_NOTIFY_URL:
        return
    payload = {
        # Escaped for the same reason chat_notify.py escapes this exact
        # field for the Space card: `who` is free text this service only
        # .strip()s, never HTML-escapes, and Novera's own card embeds it
        # directly in a textParagraph. Defense in depth -- Novera's own
        # broadcast code escapes it too, but this shouldn't rely on that
        # alone any more than chat_notify.py relies on the frontend editor
        # alone.
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
    try:
        async with httpx.AsyncClient() as client:
            response = await client.post(
                NOVERA_NOTIFY_URL,
                json=payload,
                headers={"x-til-notification-secret": NOVERA_NOTIFY_SECRET},
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
    except httpx.RequestError as exc:
        print(f"novera_notify: failed to reach Novera: {exc}", flush=True)
