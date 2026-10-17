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

import { ConversationListRowAction } from "@features/support/types/conversations";

enum ConversationStateCategory {
  OpenLike = "openLike",
  Other = "other",
}

function normalizeConversationStateCategory(
  stateLabel?: string | null,
): ConversationStateCategory {
  const label = stateLabel?.trim().toLowerCase() ?? "";
  // Exact match only: substring matching would mis-classify terminal states
  // like "Reopened" (contains "open") or "Inactive" (contains "active").
  // Abandoned, Resolved and Converted are terminal and stay View-only.
  return label === "open" || label === "active"
    ? ConversationStateCategory.OpenLike
    : ConversationStateCategory.Other;
}

/**
 * Whether a conversation in the given state can be resumed (i.e. the user can
 * send further messages). Terminal states such as Abandoned, Resolved and
 * Converted are not resumable.
 *
 * @param stateLabel - Status label from the API.
 * @returns {boolean} True if the chat can be resumed.
 */
export function isConversationResumable(stateLabel?: string | null): boolean {
  return (
    normalizeConversationStateCategory(stateLabel) ===
    ConversationStateCategory.OpenLike
  );
}

/**
 * Maps conversation state label to list row primary action (view vs resume).
 *
 * @param stateLabel - Status label from the API.
 * @returns {ConversationListRowAction} Row action for the card footer button.
 */
export function resolveConversationListRowAction(
  stateLabel?: string | null,
): ConversationListRowAction {
  switch (normalizeConversationStateCategory(stateLabel)) {
    case ConversationStateCategory.OpenLike:
      return ConversationListRowAction.Resume;
    case ConversationStateCategory.Other:
    default:
      return ConversationListRowAction.View;
  }
}

/**
 * Normalizes a linked case id; blank or whitespace-only ids count as no case.
 *
 * @param caseId - Raw linked case id from the conversation.
 * @returns {string | undefined} Trimmed id, or undefined when there is no case.
 */
export function getLinkedCaseId(caseId?: string | null): string | undefined {
  return caseId?.trim() || undefined;
}

/**
 * Path for View on a conversation. Converted chats carry a linked case;
 * View should open that case, not the chat-history page.
 *
 * @param projectId - Project in the URL.
 * @param conversationId - Conversation id used when there is no case.
 * @param caseId - Linked case id when the chat was converted.
 * @returns {string} In-app path.
 */
export function conversationViewPath(
  projectId: string,
  conversationId: string,
  caseId?: string | null,
): string {
  const linkedCaseId = getLinkedCaseId(caseId);
  if (linkedCaseId) {
    return `/projects/${projectId}/support/cases/${linkedCaseId}`;
  }
  return `/projects/${projectId}/support/conversations/${conversationId}`;
}
