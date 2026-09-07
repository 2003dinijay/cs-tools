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

package main

import (
	"net/http"
	"strings"
	"testing"
)

const goodBase = "https://apis-stg.example.invalid/zjma/chat/default-endpoint/v1.0"

// TestSanitizeURLValue_RecoversTheStagingMisconfiguration is the regression
// guard for the recommendations 500: AI_CHAT_AGENT_BASE_URL was stored with a
// character before "https", so url.Parse never recognised the scheme and every
// call failed with "first path segment in URL cannot contain colon" — before any
// network I/O, which is why it returned in 4ms and looked like an upstream fault.
func TestSanitizeURLValue_RecoversTheStagingMisconfiguration(t *testing.T) {
	for name, in := range map[string]string{
		"double quoted":       `"` + goodBase + `"`,
		"single quoted":       `'` + goodBase + `'`,
		"leading space":       " " + goodBase,
		"trailing space":      goodBase + " ",
		"surrounding spaces":  "  " + goodBase + "  ",
		"trailing newline":    goodBase + "\n",
		"quoted with padding": `  "` + goodBase + `"  `,
		"quoted with inner":   `" ` + goodBase + ` "`,
		"already clean":       goodBase,
	} {
		if got := sanitizeURLValue(in); got != goodBase {
			t.Errorf("%s: sanitizeURLValue(%q) = %q, want %q", name, in, got, goodBase)
		}
	}
}

// TestSanitizeURLValue_LeavesLegitimateValuesAlone checks the stripping is
// conservative — an unmatched quote is not silently removed, since that would
// mask a genuinely corrupt value rather than fix a formatting slip.
func TestSanitizeURLValue_LeavesLegitimateValuesAlone(t *testing.T) {
	for _, in := range []string{
		`"` + goodBase, // opening quote only
		goodBase + `"`, // closing quote only
		"",
	} {
		got := sanitizeURLValue(in)
		if want := strings.TrimSpace(in); got != want {
			t.Errorf("sanitizeURLValue(%q) = %q, want it left as %q", in, got, want)
		}
	}
}

// TestCheckURLValue_RejectsUnusableURLs covers the other shapes that fail inside
// the HTTP client rather than at parse time, so they are caught at startup
// instead of becoming a per-request 500 on one endpoint.
func TestCheckURLValue_RejectsUnusableURLs(t *testing.T) {
	for name, tc := range map[string]struct {
		in      string
		schemes []string
		wantBad bool
	}{
		"valid https":       {goodBase, []string{"http", "https"}, false},
		"valid ws":          {"ws://localhost:8080", []string{"ws", "wss"}, false},
		"no scheme":         {"apis-stg.example.invalid/v1.0", []string{"http", "https"}, true},
		"single slash":      {"https:/apis-stg.example.invalid/v1.0", []string{"http", "https"}, true},
		"wrong scheme":      {"ws://apis-stg.example.invalid", []string{"http", "https"}, true},
		"ws where https":    {goodBase, []string{"ws", "wss"}, true},
		"host only, scheme": {"https://", []string{"http", "https"}, true},
	} {
		problem := checkURLValue(tc.in, tc.schemes...)
		if tc.wantBad && problem == urlOK {
			t.Errorf("%s: checkURLValue(%q) accepted it, want a rejection", name, tc.in)
		}
		if !tc.wantBad && problem != urlOK {
			t.Errorf("%s: checkURLValue(%q) rejected it: %s", name, tc.in, problem)
		}
	}
}

// TestSanitizedValueIsActuallyRequestable is the end-to-end check that matters:
// the sanitized value must survive the same construction the upstream clients
// perform (baseURL + path), which is where the original failure occurred.
func TestSanitizedValueIsActuallyRequestable(t *testing.T) {
	raw := `"` + goodBase + `"` // the shape that broke staging
	clean := sanitizeURLValue(raw)

	if _, err := http.NewRequest(http.MethodPost, raw+"/recommendations", nil); err == nil {
		t.Fatal("expected the unsanitized value to fail request construction; the test fixture no longer reproduces the bug")
	}
	req, err := http.NewRequest(http.MethodPost, clean+"/recommendations", nil)
	if err != nil {
		t.Fatalf("sanitized value still fails request construction: %v", err)
	}
	if req.URL.Host == "" || req.URL.Scheme != "https" {
		t.Errorf("sanitized request URL = %q, want an absolute https URL", req.URL.String())
	}
}

// TestURLProblem_MessagesAreConstant asserts every classification maps to a
// fixed, non-empty message. The reason is logged, so it must never be built from
// the configured value — a newline in that value could otherwise forge a log
// entry (gosec G706).
func TestURLProblem_MessagesAreConstant(t *testing.T) {
	for _, p := range []urlProblem{urlUnparseable, urlNoScheme, urlBadScheme, urlNoHost} {
		msg := p.String()
		if msg == "" || msg == "ok" {
			t.Errorf("urlProblem(%d).String() = %q, want a descriptive constant", p, msg)
		}
		if strings.ContainsAny(msg, "\n\r") {
			t.Errorf("urlProblem(%d) message contains a newline: %q", p, msg)
		}
	}
	if urlOK.String() != "ok" {
		t.Errorf("urlOK.String() = %q, want \"ok\"", urlOK.String())
	}
}
