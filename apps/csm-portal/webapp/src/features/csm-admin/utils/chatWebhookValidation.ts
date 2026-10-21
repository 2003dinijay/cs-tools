// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

const CHAT_WEBHOOK_HOST = "chat.googleapis.com";

/**
 * A fast, client-side mirror of entity-service's own `SetChatWebhook`
 * validation (see that repo's own `CLAUDE.md`) — an empty value is valid (it
 * clears the webhook); a non-empty one must be a well-formed `https://`
 * URL whose host is exactly `chat.googleapis.com`. This is a display
 * convenience only: the backend is the real, authoritative check and its own
 * refusal reason is shown verbatim if this ever disagrees with it.
 */
export function chatWebhookUrlError(value: string): string | null {
  const trimmed = value.trim();
  if (trimmed === "") return null;
  let url: URL;
  try {
    url = new URL(trimmed);
  } catch {
    return "Enter a valid https://chat.googleapis.com/... URL.";
  }
  if (
    url.protocol !== "https:" ||
    url.username !== "" ||
    url.password !== "" ||
    url.hostname.toLowerCase() !== CHAT_WEBHOOK_HOST
  ) {
    return "Must be a well-formed https://chat.googleapis.com/... URL.";
  }
  return null;
}
