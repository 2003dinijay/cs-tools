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

package notify_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/notify"
)

// captured is one request the fake email service received.
type captured struct {
	To       []string `json:"to"`
	CC       []string `json:"cc"`
	Subject  string   `json:"subject"`
	Template []byte   `json:"template"`
}

// newCapturingClient stands up a fake email service and a client
// pointed at it, returning the requests it receives.
func newCapturingClient(t *testing.T, debug []string) (*notify.Client, *[]captured) {
	t.Helper()
	var got []captured
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "token") {
			// oauth2 parses a token response as a form unless it is
			// told otherwise.
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"t","token_type":"Bearer","expires_in":3600}`))
			return
		}
		var c captured
		_ = json.NewDecoder(r.Body).Decode(&c)
		got = append(got, c)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	client, err := notify.NewClient(notify.Config{
		BaseURL:         srv.URL,
		TokenURL:        srv.URL + "/token",
		ClientID:        "id",
		ClientSecret:    "secret",
		FromAddress:     "noreply@example.com",
		DebugRecipients: debug,
	})
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	return client, &got
}

// *** THE PROPERTY THAT MATTERS: with debug recipients set, the real
// audience receives nothing. *** Enforced in SendEmail rather than in
// each task, so no caller can bypass it and no future task has to
// remember it exists.
func TestDebugMailModeDivertsEverything(t *testing.T) {
	client, got := newCapturingClient(t, []string{"tester@example.com"})

	err := client.SendEmail(context.Background(),
		[]string{"real-one@example.com", "real-two@example.com"},
		[]string{"real-list@example.com"},
		"Weekly Query Support Consumption Report", "<p>body</p>")
	if err != nil {
		t.Fatalf("SendEmail: %v", err)
	}
	if len(*got) != 1 {
		t.Fatalf("requests = %d, want 1", len(*got))
	}
	sent := (*got)[0]

	if len(sent.To) != 1 || sent.To[0] != "tester@example.com" {
		t.Errorf("to = %v, want only the debug recipient", sent.To)
	}
	if len(sent.CC) != 0 {
		t.Errorf("cc = %v, want empty -- a debug send must not copy the real list", sent.CC)
	}
	for _, addr := range append(sent.To, sent.CC...) {
		if strings.HasPrefix(addr, "real-") {
			t.Errorf("%s is a real recipient and must receive nothing in debug mode", addr)
		}
	}
	if !strings.HasPrefix(sent.Subject, "[DEBUG] ") {
		t.Errorf("subject = %q, want a [DEBUG] prefix", sent.Subject)
	}

	// The banner has to name the audience that WOULD have received it.
	// Without that a tester sees a plausible report and cannot tell
	// whether the routing was right, which is most of what they are
	// checking.
	body := string(sent.Template)
	for _, want := range []string{"real-one@example.com", "real-two@example.com", "real-list@example.com"} {
		if !strings.Contains(body, want) {
			t.Errorf("banner does not name the intended recipient %s", want)
		}
	}
	if !strings.Contains(body, "DEBUG MAIL MODE") {
		t.Error("body carries no debug banner")
	}
	if !strings.Contains(body, "<p>body</p>") {
		t.Error("the real report body was lost")
	}
}

// With no debug recipients configured, nothing changes at all.
func TestWithoutDebugModeNothingIsDiverted(t *testing.T) {
	client, got := newCapturingClient(t, nil)

	if err := client.SendEmail(context.Background(),
		[]string{"real@example.com"}, []string{"list@example.com"},
		"Subject", "<p>body</p>"); err != nil {
		t.Fatalf("SendEmail: %v", err)
	}
	sent := (*got)[0]
	if len(sent.To) != 1 || sent.To[0] != "real@example.com" {
		t.Errorf("to = %v, want the real recipient untouched", sent.To)
	}
	if len(sent.CC) != 1 || sent.CC[0] != "list@example.com" {
		t.Errorf("cc = %v, want the real list untouched", sent.CC)
	}
	if sent.Subject != "Subject" {
		t.Errorf("subject = %q, want it unprefixed", sent.Subject)
	}
	if strings.Contains(string(sent.Template), "DEBUG MAIL MODE") {
		t.Error("a normal send must carry no debug banner")
	}
}

// A blank entry is not a debug recipient. Otherwise
// DEBUG_MAIL_RECIPIENTS="" set to an empty string in a deployment would
// divert every mail to nobody and silently stop all reporting.
func TestBlankDebugRecipientsAreIgnored(t *testing.T) {
	client, got := newCapturingClient(t, []string{"", "   "})

	if err := client.SendEmail(context.Background(),
		[]string{"real@example.com"}, nil, "Subject", "<p>body</p>"); err != nil {
		t.Fatalf("SendEmail: %v", err)
	}
	if sent := (*got)[0]; len(sent.To) != 1 || sent.To[0] != "real@example.com" {
		t.Errorf("to = %v, want the real recipient -- a blank list is not debug mode", sent.To)
	}
}
