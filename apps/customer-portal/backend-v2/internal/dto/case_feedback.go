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

// CaseFeedbackEmoji is the trimmed emoji reference on GET /cases/{id}/feedback
// — id/name/selectedImage only, dropping unselectedImage/value/chips.
type CaseFeedbackEmoji struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	SelectedImage string `json:"selectedImage"`
}

// CaseFeedback is the portal's response for GET /cases/{id}/feedback.
type CaseFeedback struct {
	ID                string            `json:"id"`
	Emoji             CaseFeedbackEmoji `json:"emoji"`
	ChipIDs           []string          `json:"chips"`
	AssessmentID      string            `json:"assessmentId"`
	CreatedBy         string            `json:"createdBy"`
	CreatedOn         string            `json:"createdOn"`
	AdditionalComment *string           `json:"additionalComment"`
}

// MapCaseFeedback builds the portal response from entity-service's CaseFeedback.
func MapCaseFeedback(r entity.CaseFeedback) CaseFeedback {
	return CaseFeedback{
		ID: r.ID,
		Emoji: CaseFeedbackEmoji{
			ID:            r.Emoji.ID,
			Name:          r.Emoji.Name,
			SelectedImage: r.Emoji.SelectedImage,
		},
		ChipIDs:           r.ChipIDs,
		AssessmentID:      r.AssessmentID,
		CreatedBy:         r.CreatedBy,
		CreatedOn:         r.CreatedOn,
		AdditionalComment: r.AdditionalComment,
	}
}

// SubmitCaseFeedbackRequest is the portal's request body for
// POST /cases/{id}/feedback — identical shape to entity-service's own payload.
type SubmitCaseFeedbackRequest struct {
	EmojiID           string   `json:"emojiId"`
	ChipIDs           []string `json:"chipIds,omitempty"`
	AdditionalComment *string  `json:"additionalComment,omitempty"`
}

// BuildEntitySubmitCaseFeedbackRequest translates the portal request into
// entity-service's shape.
func BuildEntitySubmitCaseFeedbackRequest(req SubmitCaseFeedbackRequest) entity.SubmitCaseFeedbackRequest {
	return entity.SubmitCaseFeedbackRequest{
		EmojiID:           req.EmojiID,
		ChipIDs:           req.ChipIDs,
		AdditionalComment: req.AdditionalComment,
	}
}

// SubmittedCaseFeedback holds the feedback record created by a submission.
type SubmittedCaseFeedback struct {
	ID           string `json:"id"`
	AssessmentID string `json:"assessmentId"`
	CaseID       string `json:"caseId"`
	CreatedBy    string `json:"createdBy"`
	CreatedOn    string `json:"createdOn"`
}

// SubmitCaseFeedbackResponse is the portal's response for POST /cases/{id}/feedback.
type SubmitCaseFeedbackResponse struct {
	Message  string                `json:"message"`
	Feedback SubmittedCaseFeedback `json:"feedback"`
}

// MapSubmitCaseFeedbackResponse builds the portal response from entity-service's
// SubmitCaseFeedbackResponse.
func MapSubmitCaseFeedbackResponse(r entity.SubmitCaseFeedbackResponse) SubmitCaseFeedbackResponse {
	return SubmitCaseFeedbackResponse{
		Message: r.Message,
		Feedback: SubmittedCaseFeedback{
			ID:           r.Feedback.ID,
			AssessmentID: r.Feedback.AssessmentID,
			CaseID:       r.Feedback.CaseID,
			CreatedBy:    r.Feedback.CreatedBy,
			CreatedOn:    r.Feedback.CreatedOn,
		},
	}
}
