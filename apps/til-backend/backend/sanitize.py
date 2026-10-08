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

"""Server-side sanitizing for the "what" rich-text field.

The frontend (TilRichTextField + tilRichText.ts) already sanitizes with
DOMPurify on every edit and again on every read, but this backend never
trusts that a direct API call (bypassing the editor entirely) did the same
-- same posture as auth.py independently re-verifying a token the gateway
already checked. Same allowlist as the frontend's SANITIZE_CONFIG: p/br/
strong/em/u/ol/ul/li/a/img, with href/target on <a>, src/alt on <img>,
http(s)/mailto/tel only -- an <img src="..."> only ever points at this
service's own POST /uploads result (see uploads.py), never arbitrary user
HTML, so there's no new injection surface from allowing the tag itself.
"""
from __future__ import annotations

import re
from urllib.parse import urlparse

import bleach

ALLOWED_TAGS = ["p", "br", "strong", "em", "u", "ol", "ul", "li", "a", "img"]
# "width" backs the webapp editor's resize overlay -- a plain HTML
# dimension attribute (e.g. "50%"), never a CSS "style" string, so
# there's no style-based injection surface from allowing it. Mirrors
# tilRichText.ts's own SANITIZE_CONFIG addition on the frontend -- see
# its comment for the full reasoning.
ALLOWED_ATTRIBUTES = {"a": ["href", "target"], "img": ["src", "alt", "width"]}
ALLOWED_PROTOCOLS = ["http", "https", "mailto", "tel"]

# Matches exactly the path shape uploads.py's save_upload() produces
# (f"{uuid.uuid4().hex}.{ext}" under /uploads/), checked against only the
# PATH portion of an <img src> -- the host varies per environment (local
# dev / Staging / prod all serve from a different origin via
# request.base_url), so it can't be pinned to one exact origin here. A
# direct API call (bypassing the editor, which never offers any other
# image source) could otherwise embed an arbitrary external image -- e.g.
# a tracking pixel that fires whenever any OTHER employee opens the entry,
# since nothing before this validated the src was actually one of this
# service's own uploads, just that the <img> tag and a URL-shaped src were
# present.
_UPLOAD_IMG_PATH_RE = re.compile(r"/uploads/[0-9a-f]{32}\.(?:png|jpe?g|gif|webp)$", re.IGNORECASE)
_IMG_SRC_ATTR_RE = re.compile(r'<img\b[^>]*\bsrc="([^"]*)"[^>]*>', re.IGNORECASE)

_BLOCK_END_RE = re.compile(r"</(p|li|br)>", re.IGNORECASE)
_TAG_RE = re.compile(r"<[^>]*>")
# bleach's own strip=True removes a disallowed TAG but keeps its inner text
# (the right default for e.g. a stray <div> or <span>) -- script/style are
# the one case that needs their CONTENT gone too, not just the wrapper,
# matching DOMPurify's behavior on the frontend. Not a safety gap either
# way (surviving script text is inert, never executed), just avoids inert
# JS/CSS source showing up as visible junk text in an entry.
_SCRIPT_OR_STYLE_RE = re.compile(r"<(script|style)\b[^>]*>.*?</\1>", re.IGNORECASE | re.DOTALL)


def _strip_non_upload_images(html: str) -> str:
    """Removes an <img> tag outright (not just its src) if the path portion
    of its src doesn't match an actual upload's shape. bleach's own
    tag/attribute/protocol allowlist only confirms a src is *some*
    http(s) URL -- it says nothing about WHICH host or path, so this runs
    as a second pass afterward."""

    def replace(match: re.Match[str]) -> str:
        src = match.group(1)
        path = urlparse(src).path
        return match.group(0) if _UPLOAD_IMG_PATH_RE.search(path) else ""

    return _IMG_SRC_ATTR_RE.sub(replace, html)


def sanitize_what_html(html: str) -> str:
    without_scripts = _SCRIPT_OR_STYLE_RE.sub("", html)
    cleaned = bleach.clean(
        without_scripts,
        tags=ALLOWED_TAGS,
        attributes=ALLOWED_ATTRIBUTES,
        protocols=ALLOWED_PROTOCOLS,
        strip=True,
    )
    return _strip_non_upload_images(cleaned)


def what_plain_text(html: str) -> str:
    """Mirrors tilRichText.ts's toPlainText -- block tags become a space
    first so adjacent paragraphs don't read as one glued-together word."""
    with_breaks = _BLOCK_END_RE.sub(" ", html)
    return _TAG_RE.sub("", with_breaks).replace("&nbsp;", " ").strip()


_LIST_ITEM_RE = re.compile(r"<li>(.*?)</li>", re.IGNORECASE | re.DOTALL)
_PARAGRAPH_RE = re.compile(r"<p>(.*?)</p>", re.IGNORECASE | re.DOTALL)
_REMAINING_BLOCK_RE = re.compile(r"</?(ol|ul)>", re.IGNORECASE)
_IMG_RE = re.compile(r"<img\b[^>]*>", re.IGNORECASE)


def what_for_chat(html: str) -> str:
    """Google Chat's textParagraph widget understands a small HTML subset --
    <b>/<i>/<u>/<a href> and <br>, but NOT <p>/<ul>/<li>/<strong>/<em> -- so
    those need converting rather than passed through as-is (which would
    show literal tags in the card). Already-sanitized input (sanitize_what_
    html's allowlist), so no new injection surface here, just a format
    translation for Chat's narrower one.

    Images are dropped entirely, not converted -- by design, an entry's
    image is only ever meant to be seen on the entry's own page, never in
    the Chat Space post or the Novera DM broadcast, and textParagraph has
    no image support to translate to regardless."""
    text = _IMG_RE.sub("", html)
    text = text.replace("<strong>", "<b>").replace("</strong>", "</b>")
    text = text.replace("<em>", "<i>").replace("</em>", "</i>")
    # <u> and <a href="..."> pass through unchanged -- both already in
    # Chat's supported subset.
    text = _LIST_ITEM_RE.sub(r"• \1<br>", text)
    text = _PARAGRAPH_RE.sub(r"\1<br>", text)
    text = _REMAINING_BLOCK_RE.sub("", text)
    return text.strip()
