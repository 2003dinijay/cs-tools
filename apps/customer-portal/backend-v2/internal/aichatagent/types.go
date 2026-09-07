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

package aichatagent

// These types mirror the upstream Python AI chat agent's wire format 1:1
// so json.Unmarshal can decode its responses directly.

// CaseClassificationPayload is the input for POST /case_classification.
type CaseClassificationPayload struct {
	ChatHistory   string              `json:"chatHistory"`
	EnvProducts   map[string][]string `json:"envProducts"`
	Region        string              `json:"region"`
	Tier          string              `json:"tier"`
	ProjectTypeID string              `json:"projectTypeId"`
}

// ChatCaseInfo carries the case details inferred by case classification.
type ChatCaseInfo struct {
	Description      string `json:"description"`
	ShortDescription string `json:"shortDescription"`
	ProductName      string `json:"productName"`
	ProductVersion   string `json:"productVersion"`
	Environment      string `json:"environment"`
	Tier             string `json:"tier"`
	Region           string `json:"region"`
}

// CaseClassificationResponse is the response for POST /case_classification.
type CaseClassificationResponse struct {
	IssueType     string       `json:"issueType"`
	SeverityLevel string       `json:"severityLevel"`
	CaseInfo      ChatCaseInfo `json:"caseInfo"`
}

// ChatPayload is the input for POST /chat.
type ChatPayload struct {
	Message        string              `json:"message"`
	AccountID      string              `json:"accountId"`
	ConversationID string              `json:"conversationId"`
	EnvProducts    map[string][]string `json:"envProducts,omitempty"`
}

// DetectedIntent is the intent classification result for UI rendering.
type DetectedIntent struct {
	IntentID    string  `json:"intentId"`
	IntentLabel string  `json:"intentLabel"`
	Confidence  float64 `json:"confidence"`
	Severity    string  `json:"severity"`
	CaseType    string  `json:"caseType"`
}

// SlotOption is one selectable value for a slot, rendered as a dropdown/selector by the UI.
type SlotOption struct {
	Slot    string   `json:"slot"`
	Label   string   `json:"label"`
	Options []string `json:"options"`
	Type    string   `json:"type"`
}

// SlotState is the current slot-filling progress for UI rendering.
type SlotState struct {
	IntentID     string            `json:"intentId"`
	FilledSlots  map[string]string `json:"filledSlots"`
	MissingSlots []string          `json:"missingSlots"`
	IsComplete   bool              `json:"isComplete"`
	SlotOptions  []SlotOption      `json:"slotOptions"`
}

// Action is a UI action button rendered by the frontend.
type Action struct {
	Type    string         `json:"type"`
	Label   string         `json:"label"`
	Style   string         `json:"style"`
	Payload map[string]any `json:"payload"`
}

// RecommendationItem is a single matching KB article.
type RecommendationItem struct {
	Title     string  `json:"title"`
	ArticleID string  `json:"articleId"`
	Score     float64 `json:"score"`
}

// RecommendationResponse is the response for POST /recommendations.
type RecommendationResponse struct {
	Query           string               `json:"query"`
	Recommendations []RecommendationItem `json:"recommendations"`
}

// KbReference is a single KB article referenced in a chat response.
type KbReference struct {
	SysKbID string `json:"sysKbId"`
	Title   string `json:"title"`
}

// ChatResponse is the response for POST /chat.
type ChatResponse struct {
	Message         string                  `json:"message"`
	SessionID       string                  `json:"sessionId"`
	ConversationID  string                  `json:"conversationId"`
	Intent          *DetectedIntent         `json:"intent,omitempty"`
	SlotState       *SlotState              `json:"slotState,omitempty"`
	Actions         []Action                `json:"actions,omitempty"`
	Recommendations *RecommendationResponse `json:"recommendations,omitempty"`
	Resolved        *bool                   `json:"resolved,omitempty"`
	KbReferences    []KbReference           `json:"kbReferences,omitempty"`
}

// ChatMessage is a single chat message for UI rendering (role is "user" or "assistant").
type ChatMessage struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp"`
}

// ConversationData carries chat-history context for a recommendation request.
type ConversationData struct {
	ChatHistory string              `json:"chatHistory"`
	EnvProducts map[string][]string `json:"envProducts"`
	Region      string              `json:"region"`
	Tier        string              `json:"tier"`
}

// RecommendationRequest is the input for POST /recommendations.
type RecommendationRequest struct {
	ChatHistory      []ChatMessage    `json:"chatHistory"`
	ConversationData ConversationData `json:"conversationData"`
}

// ConversationSummaryResponse is the response for GET /chat/summary/{projectId}/{conversationId}.
type ConversationSummaryResponse struct {
	AccountID               string `json:"accountId"`
	ConversationID          string `json:"conversationId"`
	MessagesExchanged       int    `json:"messagesExchanged"`
	TroubleshootingAttempts int    `json:"troubleshootingAttempts"`
	KbArticlesReviewed      int    `json:"kbArticlesReviewed"`
}
