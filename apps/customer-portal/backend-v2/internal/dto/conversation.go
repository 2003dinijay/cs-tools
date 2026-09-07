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

import "github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"

// ConversationDetails is the portal's response for GET /conversations/{id}.
type ConversationDetails struct {
	ID             string         `json:"id"`
	Number         *string        `json:"number"`
	InitialMessage *string        `json:"initialMessage"`
	MessageCount   int            `json:"messageCount"`
	Project        *ReferenceItem `json:"project"`
	Case           *ReferenceItem `json:"case"`
	State          *ReferenceItem `json:"state"`
	CreatedOn      string         `json:"createdOn"`
	CreatedBy      string         `json:"createdBy"`
	UpdatedOn      string         `json:"updatedOn"`
	UpdatedBy      string         `json:"updatedBy"`
}

// MapConversationDetails builds the portal response from entity-service's
// ConversationDetails.
func MapConversationDetails(r entity.ConversationDetails) ConversationDetails {
	out := ConversationDetails{
		ID:             r.ID,
		Number:         r.Number,
		InitialMessage: r.InitialMessage,
		MessageCount:   r.MessageCount,
		CreatedOn:      r.CreatedOn,
		CreatedBy:      r.CreatedBy,
		UpdatedOn:      r.UpdatedOn,
		UpdatedBy:      r.UpdatedBy,
	}
	if r.Project != nil {
		out.Project = &ReferenceItem{ID: r.Project.ID, Label: r.Project.Name}
	}
	if r.Case != nil {
		out.Case = &ReferenceItem{ID: r.Case.ID, Label: r.Case.Name}
	}
	if r.State != nil {
		out.State = &ReferenceItem{ID: *r.State, Label: *r.State}
	}
	return out
}

// Conversation status values accepted by PATCH /conversations/{id}. "closed"
// is a user-initiated close from the chat list; "abandoned" is set
// automatically when a case is created from a chat before the assistant has
// responded; "converted" marks a chat that turned into a case.
const (
	ConversationStatusClosed    = "closed"
	ConversationStatusAbandoned = "abandoned"
	ConversationStatusConverted = "converted"
)

// conversationStatusToState maps the portal's PATCH /conversations/{id}
// status values to entity-service's ConversationState enum string values —
// only the three states this endpoint can transition to are represented
// here.
var conversationStatusToState = map[string]string{
	ConversationStatusClosed:    "CLOSED",
	ConversationStatusAbandoned: "ABANDONED",
	ConversationStatusConverted: "CONVERTED",
}

// ConversationStatusUpdate is the portal's request body for
// PATCH /conversations/{id}.
type ConversationStatusUpdate struct {
	Status string `json:"status"`
}

// BuildEntityConversationStateUpdate translates the portal's status value
// into entity-service's UpdateConversationRequest. ok is false when Status
// isn't one of "closed", "abandoned", or "converted".
func BuildEntityConversationStateUpdate(req ConversationStatusUpdate) (entity.UpdateConversationRequest, bool) {
	state, ok := conversationStatusToState[req.Status]
	if !ok {
		return entity.UpdateConversationRequest{}, false
	}
	return entity.UpdateConversationRequest{State: state}, true
}
