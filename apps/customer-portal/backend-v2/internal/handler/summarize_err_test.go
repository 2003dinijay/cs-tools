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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/apierror"
)

// TestSummarizeErr_DistinguishesNonStatusFailures is the regression guard for an
// observability defect: every failure without an upstream HTTP status used to
// summarize as the single string "upstream request failed", so a 500 caused by a
// malformed base URL was indistinguishable in the logs from one caused by bad
// credentials or an undecodable response body. Diagnosing one meant guessing.
func TestSummarizeErr_DistinguishesNonStatusFailures(t *testing.T) {
	// A base URL with no scheme — fails in the transport, before any network I/O.
	noScheme := &url.Error{Op: "Post", URL: "apis-stg.example.invalid/v1.0/recommendations",
		Err: errors.New(`unsupported protocol scheme ""`)}
	// A base URL with a stray newline — fails at url.Parse.
	badChar := &url.Error{Op: "parse", URL: "https://example.invalid/v1.0\n",
		Err: errors.New("net/http: invalid control character in URL")}

	var typeErr error
	if err := json.Unmarshal([]byte(`{"createdBy":{"name":"x"}}`), &struct {
		CreatedBy string `json:"createdBy"`
	}{}); err != nil {
		typeErr = err
	}
	if typeErr == nil {
		t.Fatal("expected an UnmarshalTypeError from the fixture")
	}

	for name, tc := range map[string]struct {
		err  error
		want string
	}{
		"missing scheme":   {noScheme, "upstream Post failed: unsupported protocol scheme"},
		"control char":     {badChar, "upstream parse failed: net/http: invalid control character"},
		"timeout":          {fmt.Errorf("wrapped: %w", context.DeadlineExceeded), "upstream request timed out"},
		"canceled":         {fmt.Errorf("wrapped: %w", context.Canceled), "upstream request canceled"},
		"type mismatch":    {fmt.Errorf("decode: %w", typeErr), `field "createdBy" expects string, got object`},
		"malformed json":   {fmt.Errorf("decode: %w", &json.SyntaxError{Offset: 17}), "malformed JSON at byte 17"},
		"unknown fallback": {errors.New("something else entirely"), "upstream request failed"},
	} {
		got := summarizeErr(tc.err)
		if !strings.Contains(got, tc.want) {
			t.Errorf("%s: summarizeErr = %q, want it to contain %q", name, got, tc.want)
		}
	}
}

// TestSummarizeErr_NeverLogsTheRequestURL keeps the reason this was previously
// collapsed: url.Error.Error() appends the full request URL, which for other
// clients can carry filter query params. Only url.Error.Err may be logged.
func TestSummarizeErr_NeverLogsTheRequestURL(t *testing.T) {
	secretish := "https://internal.example.invalid/v1/cases?email=someone%40wso2.com&token=abc123"
	err := &url.Error{Op: "Get", URL: secretish, Err: errors.New("dial tcp: lookup failed")}

	got := summarizeErr(err)
	for _, leak := range []string{"someone", "token=abc123", secretish, "?email="} {
		if strings.Contains(got, leak) {
			t.Errorf("summarizeErr leaked %q in %q", leak, got)
		}
	}
	if !strings.Contains(got, "dial tcp") {
		t.Errorf("summarizeErr = %q, want the URL-free reason retained", got)
	}
}

// TestSummarizeErr_UpstreamStatusStillWins checks the pre-existing behaviour is
// untouched: a typed upstream error keeps reporting its status and message,
// which is already caller-safe validation text.
func TestSummarizeErr_UpstreamStatusStillWins(t *testing.T) {
	err := apierror.NewUpstreamError(http.StatusBadRequest, []byte(`{"message":"caseTypes must be valid UUIDs"}`))
	got := summarizeErr(err)
	if !strings.Contains(got, "upstream status 400") || !strings.Contains(got, "caseTypes must be valid UUIDs") {
		t.Errorf("summarizeErr = %q, want the status and upstream message preserved", got)
	}
}
