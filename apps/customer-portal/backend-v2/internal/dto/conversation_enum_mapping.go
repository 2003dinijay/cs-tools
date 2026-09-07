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

package dto

import "strconv"

// conversationStateIDs mirrors entity-service's private
// snConversationStateKeyMap (internal/service/sn_conversation_service.go) —
// ServiceNow's own numeric choice-list key for each conversation state.
// entity-service's search response already normalizes State to these exact
// domain enum strings (see snConversationStateLabelMap), so no
// label-word-parsing is needed, just a direct enum lookup both directions.
var conversationStateIDs = map[string]string{
	"ACTIVE":    "2",
	"RESOLVED":  "3",
	"CONVERTED": "4",
	"ABANDONED": "5",
	"CLOSED":    "6",
}

var conversationStateIDToEnum = reverseStringMap(conversationStateIDs)

// conversationStateLabels supplies portal-facing display text for these enum
// values — entity-service's response carries the domain enum string only,
// not a ServiceNow display label, so this is this backend's own
// presentation text.
var conversationStateLabels = map[string]string{
	"ACTIVE":    "Active",
	"RESOLVED":  "Resolved",
	"CONVERTED": "Converted",
	"ABANDONED": "Abandoned",
	"CLOSED":    "Closed",
}

// conversationStateRef builds the {id, label} the frontend's
// Conversation.state expects from entity-service's already-normalized State
// enum string.
func conversationStateRef(state *string) *IDLabelRef {
	if state == nil || *state == "" {
		return nil
	}
	label := conversationStateLabels[*state]
	if label == "" {
		label = *state
	}
	return &IDLabelRef{ID: conversationStateIDs[*state], Label: label}
}

// conversationIDsToEnums converts the frontend's numeric stateKeys filter to
// entity-service's own enum vocabulary, silently skipping any id with no
// known mapping.
func conversationIDsToEnums(ids []int) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if enum, ok := conversationStateIDToEnum[strconv.Itoa(id)]; ok {
			out = append(out, enum)
		}
	}
	return out
}
