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

package entity

import (
	"context"
	"fmt"
	"net/url"
)

// SearchCases calls POST /cases/search.
func (c *Client) SearchCases(ctx context.Context, req SearchCasesRequest) (SearchCasesResponse, error) {
	var out SearchCasesResponse
	err := c.postJSON(ctx, "/cases/search", req, &out)
	return out, err
}

// GetCase calls GET /cases/{id}.
func (c *Client) GetCase(ctx context.Context, id string) (CaseView, error) {
	var out CaseView
	err := c.getJSON(ctx, fmt.Sprintf("/cases/%s", url.PathEscape(id)), &out)
	return out, err
}

// CreateCase calls POST /cases.
func (c *Client) CreateCase(ctx context.Context, req CreateCaseRequest) (CreateCaseResponse, error) {
	var out CreateCaseResponse
	err := c.postJSON(ctx, "/cases", req, &out)
	return out, err
}

// UpdateCase calls PATCH /cases/{id}.
func (c *Client) UpdateCase(ctx context.Context, id string, req UpdateCaseRequest) (UpdateCaseResponse, error) {
	var out UpdateCaseResponse
	err := c.patchJSON(ctx, fmt.Sprintf("/cases/%s", url.PathEscape(id)), req, &out)
	return out, err
}

// CreateCaseComment calls POST /cases/{id}/comments.
func (c *Client) CreateCaseComment(ctx context.Context, caseID string, req CreateCaseCommentRequest) (CreateCaseCommentResponse, error) {
	var out CreateCaseCommentResponse
	err := c.postJSON(ctx, fmt.Sprintf("/cases/%s/comments", url.PathEscape(caseID)), req, &out)
	return out, err
}

// SearchCaseActivities calls POST /cases/{id}/activities/search.
func (c *Client) SearchCaseActivities(ctx context.Context, caseID string, req SearchCaseActivitiesRequest) (SearchCaseActivitiesResponse, error) {
	req.CaseID = caseID // never serialized (json:"-"); set for consistency with the struct's doc comment
	var out SearchCaseActivitiesResponse
	err := c.postJSON(ctx, fmt.Sprintf("/cases/%s/activities/search", url.PathEscape(caseID)), req, &out)
	return out, err
}

// GetCaseFeedback calls GET /cases/{id}/feedback.
func (c *Client) GetCaseFeedback(ctx context.Context, caseID string) (CaseFeedback, error) {
	var out CaseFeedback
	err := c.getJSON(ctx, fmt.Sprintf("/cases/%s/feedback", url.PathEscape(caseID)), &out)
	return out, err
}

// SubmitCaseFeedback calls POST /cases/{id}/feedback.
func (c *Client) SubmitCaseFeedback(ctx context.Context, caseID string, req SubmitCaseFeedbackRequest) (SubmitCaseFeedbackResponse, error) {
	var out SubmitCaseFeedbackResponse
	err := c.postJSON(ctx, fmt.Sprintf("/cases/%s/feedback", url.PathEscape(caseID)), req, &out)
	return out, err
}
