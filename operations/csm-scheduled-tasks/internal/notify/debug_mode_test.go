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

package notify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// debugModeServer stands in for the token endpoint and the email service,
// capturing the one send-email request the test makes.
func debugModeServer(t *testing.T) (*httptest.Server, *sendEmailRequest) {
	t.Helper()
	captured := &sendEmailRequest{}
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"t","token_type":"bearer","expires_in":3600}`))
	})
	mux.HandleFunc("/send-email", func(w http.ResponseWriter, r *http.Request) {
		var raw struct {
			To       []string `json:"to"`
			CC       []string `json:"cc"`
			Subject  string   `json:"subject"`
			Template []byte   `json:"template"`
		}
		if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
			t.Errorf("decode send-email request: %v", err)
		}
		captured.To, captured.CC, captured.Subject, captured.Template = raw.To, raw.CC, raw.Subject, raw.Template
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, captured
}

func newDebugModeClient(t *testing.T, srv *httptest.Server, debug []string) *Client {
	t.Helper()
	c, err := NewClient(Config{
		BaseURL:         srv.URL,
		TokenURL:        srv.URL + "/token",
		ClientID:        "id",
		ClientSecret:    "secret",
		FromAddress:     "sender@example.com",
		DebugRecipients: debug,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c
}

func TestDebugMailModeRedirectsEveryEmail(t *testing.T) {
	srv, got := debugModeServer(t)
	c := newDebugModeClient(t, srv, []string{" debug-one@example.com ", "", "debug-two@example.com"})
	if !c.DebugMode() {
		t.Fatal("DebugMode() = false with recipients configured")
	}

	body := "<!DOCTYPE html><html><head><title>x</title></head><body class=\"b\"><p>report</p></body></html>"
	err := c.SendEmail(context.Background(), []string{"lead@example.com", "team@example.com"}, []string{"mgmt@example.com"}, "[Report] Weekly", body)
	if err != nil {
		t.Fatalf("SendEmail: %v", err)
	}
	if strings.Join(got.To, ",") != "debug-one@example.com,debug-two@example.com" {
		t.Errorf("to = %v, want the trimmed debug recipients only", got.To)
	}
	if len(got.CC) != 0 {
		t.Errorf("cc = %v, want none: the real audience is never addressed", got.CC)
	}
	if got.Subject != "[Test copy] [Report] Weekly" {
		t.Errorf("subject = %q", got.Subject)
	}
	sent := string(got.Template)
	notice := "This email would have been sent to To: lead@example.com, team@example.com; Cc: mgmt@example.com."
	if !strings.Contains(sent, notice) {
		t.Errorf("body lacks the notice %q:\n%s", notice, sent)
	}
	if head, tail := strings.Index(sent, "<body class=\"b\">"), strings.Index(sent, "[Test copy]"); head < 0 || tail < head {
		t.Errorf("the notice must sit just inside <body>, after the head:\n%s", sent)
	}
	if !strings.Contains(sent, "<p>report</p>") {
		t.Errorf("the report itself is missing from the body:\n%s", sent)
	}
}

func TestDebugMailModeOffSendsAsAddressed(t *testing.T) {
	srv, got := debugModeServer(t)
	c := newDebugModeClient(t, srv, []string{"  "})
	if c.DebugMode() {
		t.Fatal("DebugMode() = true with only a blank recipient")
	}
	if err := c.SendEmail(context.Background(), []string{"lead@example.com"}, []string{"mgmt@example.com"}, "[Report] Weekly", "<p>report</p>"); err != nil {
		t.Fatalf("SendEmail: %v", err)
	}
	if strings.Join(got.To, ",") != "lead@example.com" || strings.Join(got.CC, ",") != "mgmt@example.com" || got.Subject != "[Report] Weekly" {
		t.Errorf("sent as to=%v cc=%v subject=%q; want unchanged", got.To, got.CC, got.Subject)
	}
	if strings.Contains(string(got.Template), "Test copy") {
		t.Errorf("body carries a test-copy notice with debug mode off")
	}
}

func TestTestCopyNoticeWithoutBodyTagGoesFirst(t *testing.T) {
	out := testCopyNotice("<p>plain</p>", []string{"a@example.com"}, nil)
	if !strings.HasPrefix(out, "<p style=") || !strings.HasSuffix(out, "<p>plain</p>") {
		t.Errorf("notice must be prepended to a bodiless fragment:\n%s", out)
	}
	if strings.Contains(out, "Cc:") {
		t.Errorf("no Cc given, none should be named:\n%s", out)
	}
}
