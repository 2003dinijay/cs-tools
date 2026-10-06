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

"""Posts a card into the TIL Google Chat Space(s) on every new submission.

Uses each Chat Space's own Incoming Webhook (Space settings -> Apps &
integrations -> Webhooks -> Add webhook), NOT a full Chat App registration.
This is deliberately the simple path: a webhook URL is created in the Chat
UI in under a minute by anyone who owns the Space, no Google Cloud project,
no OAuth, no admin console access needed -- it unblocks "post a notification
into the Space" immediately, independent of whether the "+" button Dialog
(which DOES need a real Chat App -- see README.md's "The `+` button form"
section) gets built in this same window.

Two independent Spaces, both optional:
  TIL_CHAT_WEBHOOK_URL               The Collaboration Space -- members can
                                      reply/react to a posted entry.
  TIL_CHAT_ANNOUNCEMENT_WEBHOOK_URL  The Announcement Space -- broadcast
                                      only, no replies.

Same "absent key = quietly off" posture as the rest of this service:
either, both, or neither can be set, and a missing one is never an error --
just that Space not getting the notification.
"""
from __future__ import annotations

import os

import html as html_module

import httpx

from sanitize import what_for_chat

TIL_CHAT_WEBHOOK_URL = os.environ.get("TIL_CHAT_WEBHOOK_URL", "")
TIL_CHAT_ANNOUNCEMENT_WEBHOOK_URL = os.environ.get("TIL_CHAT_ANNOUNCEMENT_WEBHOOK_URL", "")


def _build_card(who: str, where: str, what: str, where_detail: str | None, entry_url: str | None) -> dict:
    subtitle = f"{where} — {where_detail}" if where_detail else where
    # `what` is rich-text HTML -- Chat's textParagraph widget only understands
    # a subset of that (no <p>/<ul>/<li>/<strong>/<em>), so translate rather
    # than pass the raw markup through. See sanitize.py's what_for_chat.
    what_card_text = what_for_chat(what)
    widgets = [
        {"textParagraph": {"text": f"<b>{html_module.escape(who)}</b>"}},
        {"textParagraph": {"text": what_card_text}},
    ]
    # Absent (no ONE_WSO2_BASE_URL set -- see main.py) just means no button,
    # not an error: the card is still useful without a deep link.
    if entry_url:
        widgets.append(
            {
                "buttonList": {
                    "buttons": [
                        {
                            "text": "View entry",
                            "onClick": {"openLink": {"url": entry_url}},
                        }
                    ]
                }
            }
        )
    return {
        "cardsV2": [
            {
                "cardId": "til-entry",
                "card": {
                    "header": {"title": "Today I Learned", "subtitle": subtitle},
                    "sections": [{"widgets": widgets}],
                },
            }
        ]
    }


async def _post_to_webhook(webhook_url: str, payload: dict) -> None:
    # Best-effort: a notification failure must never fail the submission
    # itself (the record is already durably saved by the time this runs) --
    # and one Space's dead webhook must not stop the other Space's post.
    try:
        async with httpx.AsyncClient() as client:
            response = await client.post(webhook_url, json=payload, timeout=10)
        # Not raised (see "best-effort" above) -- but a rejected response
        # must still be visible to an operator, same as a network failure.
        if response.status_code >= 300:
            print(
                f"chat_notify: webhook rejected the post (status {response.status_code}): "
                f"{response.text[:200]}",
                flush=True,
            )
    except httpx.RequestError as exc:
        # Logged, not raised -- an operator should be able to notice a dead
        # webhook URL without that failure ever touching the submitter.
        print(f"chat_notify: failed to post to a TIL webhook: {exc}", flush=True)


async def notify_new_submission(
    who: str, where: str, what: str, where_detail: str | None = None, entry_url: str | None = None
) -> None:
    if not TIL_CHAT_WEBHOOK_URL and not TIL_CHAT_ANNOUNCEMENT_WEBHOOK_URL:
        return
    payload = _build_card(who, where, what, where_detail, entry_url)
    if TIL_CHAT_WEBHOOK_URL:
        await _post_to_webhook(TIL_CHAT_WEBHOOK_URL, payload)
    if TIL_CHAT_ANNOUNCEMENT_WEBHOOK_URL:
        await _post_to_webhook(TIL_CHAT_ANNOUNCEMENT_WEBHOOK_URL, payload)
