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

// Package escalation is a minimal client for the existing
// ai-escalate-comment-detector service (a separate deployable, not part of
// this repo) -- specifically its single-comment POST /escalations endpoint,
// which sends one customer comment to an LLM and returns a frustration
// score. dispatch.handleCommentAdded calls it for a comment it has already
// determined is from an external (customer) author, to decide whether to
// send a frustration-detection Chat alert. No OAuth2 is involved -- that
// service has no inbound auth of its own, same as this one (see this repo's
// own "Why no Auth middleware" doc comment).
package escalation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/apierror"
)

// Config holds the escalation detector's own configuration.
type Config struct {
	BaseURL string
}

// Client is an HTTP client for ai-escalate-comment-detector's single-comment
// escalation endpoint.
//
// New never fails, so it is safe to construct with a zero-value Config (this
// channel not yet configured for a given deployment) -- a missing BaseURL
// only surfaces as an error the first time DetectEscalation is called.
type Client struct {
	http    *http.Client
	baseURL string
}

// New constructs a Client.
func New(cfg Config) *Client {
	return &Client{
		http:    &http.Client{Timeout: 20 * time.Second},
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
	}
}

// escalationRequest mirrors ai-escalate-comment-detector's own
// EscalationRequest record. CaseNumber/ProductName/CommentPostedTimestampInSN/
// SnCustomerServiceCaseSysID are required fields on that service's own type
// but are not used for anything beyond echoing back in its response and its
// own debug logging -- DetectEscalation's caller supplies what it actually
// has (CaseID/CaseNumber/Product/Comment) and this client fills the rest
// with best-effort values (the current time, an empty sys id) rather than
// needing a wider payload just to satisfy that service's own required-field
// shape.
type escalationRequest struct {
	CaseID                     string `json:"caseId"`
	CaseNumber                 string `json:"caseNumber"`
	Comment                    string `json:"comment"`
	CommentPostedTimestampInSN string `json:"commentPostedTimestampInSN"`
	ProductName                string `json:"productName"`
	SnCustomerServiceCaseSysID string `json:"snCustomerServiceCaseSysId"`
}

// escalationResponse mirrors ai-escalate-comment-detector's own
// EscalationResponse record -- only the fields DetectEscalation's caller
// needs are decoded.
type escalationResponse struct {
	IsFrustrated    bool    `json:"isFrustrated"`
	FrustratedLevel float64 `json:"frustratedLevel"`
	Reason          string  `json:"reason"`
	IsEmailTrigger  bool    `json:"isEmailTrigger"`
}

// Result is DetectEscalation's return value: the detector's own frustration
// analysis of one comment.
type Result struct {
	// IsFrustrated/FrustratedLevel/Reason are the raw OpenAI-derived
	// analysis.
	IsFrustrated    bool
	FrustratedLevel float64
	Reason          string
	// ShouldAlert is the detector's own isEmailTrigger -- whether
	// FrustratedLevel crossed that service's own configured threshold.
	// Callers should gate an actual Chat alert on this, not IsFrustrated
	// alone: IsFrustrated is the model's own judgment call, ShouldAlert is
	// that judgment measured against a configured bar.
	ShouldAlert bool
}

// DetectEscalation sends one customer comment to ai-escalate-comment-detector
// and returns its frustration analysis.
func (c *Client) DetectEscalation(ctx context.Context, caseID, caseNumber, product, comment string) (Result, error) {
	if c.baseURL == "" {
		return Result{}, fmt.Errorf("escalation: no detector base URL configured")
	}
	if comment == "" {
		return Result{}, fmt.Errorf("escalation: comment is required")
	}

	reqBody, err := json.Marshal(escalationRequest{
		CaseID:                     caseID,
		CaseNumber:                 caseNumber,
		Comment:                    comment,
		CommentPostedTimestampInSN: time.Now().UTC().Format(time.RFC3339),
		ProductName:                product,
		SnCustomerServiceCaseSysID: "",
	})
	if err != nil {
		return Result{}, fmt.Errorf("escalation: encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/escalations", bytes.NewReader(reqBody))
	if err != nil {
		return Result{}, fmt.Errorf("escalation: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("escalation: post request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return Result{}, fmt.Errorf("escalation: read response: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		const maxErrBody = 256
		excerpt := respBody
		if len(excerpt) > maxErrBody {
			excerpt = excerpt[:maxErrBody]
		}
		return Result{}, &apierror.Error{StatusCode: resp.StatusCode, Body: string(excerpt)}
	}

	var decoded escalationResponse
	if err := json.Unmarshal(respBody, &decoded); err != nil {
		return Result{}, fmt.Errorf("escalation: decode response: %w", err)
	}

	return Result{
		IsFrustrated:    decoded.IsFrustrated,
		FrustratedLevel: decoded.FrustratedLevel,
		Reason:          decoded.Reason,
		ShouldAlert:     decoded.IsEmailTrigger,
	}, nil
}
