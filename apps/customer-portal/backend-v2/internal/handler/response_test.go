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

package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/apierror"
)

func TestMapUpstreamError_BadRequestPassesThroughUpstreamMessage(t *testing.T) {
	err := &apierror.Error{StatusCode: http.StatusBadRequest, Body: "caseTypes must be valid UUIDs"}
	rec := httptest.NewRecorder()

	mapUpstreamError(rec, err, "Failed to search cases.")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	var body errorBody
	if decodeErr := json.NewDecoder(rec.Body).Decode(&body); decodeErr != nil {
		t.Fatalf("decode response: %v", decodeErr)
	}
	if body.Message != "caseTypes must be valid UUIDs" {
		t.Fatalf("expected upstream message passed through, got %q", body.Message)
	}
}

func TestMapUpstreamError_BadRequestFallsBackWhenBodyEmpty(t *testing.T) {
	err := &apierror.Error{StatusCode: http.StatusBadRequest, Body: ""}
	rec := httptest.NewRecorder()

	mapUpstreamError(rec, err, "Failed to search cases.")

	var body errorBody
	if decodeErr := json.NewDecoder(rec.Body).Decode(&body); decodeErr != nil {
		t.Fatalf("decode response: %v", decodeErr)
	}
	if body.Message != ErrMsgBadRequest {
		t.Fatalf("expected generic fallback, got %q", body.Message)
	}
}

func TestMapUpstreamError_UnauthorizedUsesFixedMessageNotUpstreamBody(t *testing.T) {
	err := &apierror.Error{StatusCode: http.StatusUnauthorized, Body: "some internal upstream detail"}
	rec := httptest.NewRecorder()

	mapUpstreamError(rec, err, "fallback")

	var body errorBody
	if decodeErr := json.NewDecoder(rec.Body).Decode(&body); decodeErr != nil {
		t.Fatalf("decode response: %v", decodeErr)
	}
	if body.Message != ErrMsgUnauthorized {
		t.Fatalf("expected fixed unauthorized message, got %q", body.Message)
	}
}

func TestSummarizeErr_IncludesUpstreamBody(t *testing.T) {
	err := &apierror.Error{StatusCode: http.StatusBadRequest, Body: "caseTypes must be valid UUIDs"}

	got := summarizeErr(err)

	want := "upstream status 400: caseTypes must be valid UUIDs"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

// TestSummarizeErr_OmitsEmptyBody guards against ever logging a raw,
// unbounded upstream response body: entity.newUpstreamError leaves Body
// empty when the upstream response isn't the documented {"message":...}
// shape, and summarizeErr must not turn that into a misleading
// "status N: " with a dangling empty message.
func TestSummarizeErr_OmitsEmptyBody(t *testing.T) {
	err := &apierror.Error{StatusCode: http.StatusBadGateway, Body: ""}

	got := summarizeErr(err)

	want := "upstream status 502"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}
