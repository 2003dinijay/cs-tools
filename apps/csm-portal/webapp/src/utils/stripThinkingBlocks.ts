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

// A complete block, an opening tag that was never closed (the answer was cut
// off), and a half-written opening tag at the very end.
const THINKING_BLOCK_RE = /<thinking>[\s\S]*?<\/thinking>/gi;
const THINKING_OPEN_RE = /<thinking>[\s\S]*$/i;
const THINKING_PARTIAL_OPEN_RE = /<t(?:h(?:i(?:n(?:k(?:i(?:n(?:g)?)?)?)?)?)?)?$/i;

/**
 * Remove the model's `<thinking>…</thinking>` reasoning from a Novera answer.
 *
 * The agent can emit its reasoning inside the answer itself, and the customer
 * portal's backend persists the answer as-is, so a stored chat transcript can
 * contain it. Markdown rendering escapes raw HTML rather than hiding it, which
 * is why it shows up as literal text. Apply this to assistant text only — never
 * strip what a person typed.
 *
 * Mirrors `stripThinkingBlocks` in the customer portal webapp
 * (`features/support/utils/chat.ts`); keep the two in step.
 */
export function stripThinkingBlocks(text: string): string {
  if (!/<t/i.test(text)) return text;
  const stripped = text
    .replace(THINKING_BLOCK_RE, "")
    .replace(THINKING_OPEN_RE, "")
    .replace(THINKING_PARTIAL_OPEN_RE, "");
  return stripped === text ? text : stripped.trimStart();
}
