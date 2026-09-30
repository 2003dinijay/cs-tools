// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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
package snsconfirm

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type sentMail struct{ to, subject, body string }

type fakeMailer struct {
	mu   sync.Mutex
	sent []sentMail
}

func (m *fakeMailer) Send(_ context.Context, to []string, subject, html string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, sentMail{strings.Join(to, ","), subject, html})
	return nil
}

// newTest returns a Handler whose SNS endpoint is a local server answering status.
func newTest(t *testing.T, status int, teams map[string]string, mailer Mailer) (*Handler, string, *atomic.Int32, *bytes.Buffer) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)
	var logs bytes.Buffer
	h := New(slog.New(slog.NewTextHandler(&logs, nil)), Config{Teams: teams}, mailer, time.Second)
	h.allowURL = func(u *url.URL) bool { return "http://"+u.Host == srv.URL }
	return h, srv.URL + "/?Action=ConfirmSubscription&Token=abc", &hits, &logs
}

func confirmation(subscribeURL string) []byte {
	return []byte(`{"Type":"SubscriptionConfirmation","TopicArn":"arn:aws:sns:us-east-1:000000000000:example",` +
		`"Message":"You have chosen to subscribe to the topic","SubscribeURL":"` + subscribeURL + `"}`)
}

func waitMail(t *testing.T, h *Handler) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	h.Wait(ctx)
}

func TestNotConfirmation_LeftForTransform(t *testing.T) {
	h, _, hits, _ := newTest(t, 200, nil, nil)
	for _, raw := range []string{`{"Type":"Notification","Message":"{}"}`, `not json`, `{}`} {
		if h.HandleIfConfirmation([]byte(raw), "") {
			t.Errorf("%s: handled as a confirmation", raw)
		}
	}
	if hits.Load() != 0 {
		t.Error("nothing should be fetched")
	}
}

func TestConfirmed_EmailsTheTeam(t *testing.T) {
	mailer := &fakeMailer{}
	teams := map[string]string{"ManagedCloud": "managed-cloud@example.com", "Default": "sre@example.com"}
	h, subURL, hits, _ := newTest(t, 200, teams, mailer)

	if !h.HandleIfConfirmation(confirmation(subURL), "ManagedCloud") {
		t.Fatal("confirmation not handled")
	}
	waitMail(t, h)
	if hits.Load() != 1 {
		t.Errorf("SubscribeURL fetched %d times, want 1", hits.Load())
	}
	if len(mailer.sent) != 1 || mailer.sent[0].to != "managed-cloud@example.com" ||
		mailer.sent[0].subject != "AWS SNS Subscription [Auto-Confirmed]" || !strings.Contains(mailer.sent[0].body, "Successfully Confirmed") {
		t.Errorf("sent = %+v", mailer.sent)
	}
}

func TestTeamFallsBackToDefault(t *testing.T) {
	for _, team := range []string{"", "Unknown"} {
		mailer := &fakeMailer{}
		h, subURL, _, _ := newTest(t, 200, map[string]string{"Default": "sre@example.com"}, mailer)
		h.HandleIfConfirmation(confirmation(subURL), team)
		waitMail(t, h)
		if len(mailer.sent) != 1 || mailer.sent[0].to != "sre@example.com" {
			t.Errorf("team %q: sent = %+v, want the Default address", team, mailer.sent)
		}
	}
}

func TestConfirmFails_ActionRequiredEmail(t *testing.T) {
	mailer := &fakeMailer{}
	h, subURL, _, _ := newTest(t, 500, map[string]string{"Default": "sre@example.com"}, mailer)
	h.HandleIfConfirmation(confirmation(subURL), "")
	waitMail(t, h)
	if len(mailer.sent) != 1 || mailer.sent[0].subject != "AWS SNS Subscription [ACTION REQUIRED]" ||
		!strings.Contains(mailer.sent[0].body, "ConfirmSubscription") {
		t.Errorf("sent = %+v, want the manual-action email with the link", mailer.sent)
	}
}

func TestNonSNSURL_NotFetched(t *testing.T) {
	mailer := &fakeMailer{}
	var hits atomic.Int32
	evil := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits.Add(1) }))
	defer evil.Close()
	h := New(slog.New(slog.NewTextHandler(io.Discard, nil)), Config{Teams: map[string]string{"Default": "sre@example.com"}}, mailer, time.Second)

	if !h.HandleIfConfirmation(confirmation(evil.URL+"/steal"), "") {
		t.Fatal("confirmation not handled")
	}
	waitMail(t, h)
	if hits.Load() != 0 {
		t.Error("a non-SNS SubscribeURL must not be fetched")
	}
	if len(mailer.sent) != 1 || !strings.Contains(mailer.sent[0].subject, "ACTION REQUIRED") {
		t.Errorf("sent = %+v", mailer.sent)
	}
}

func TestNoEmailAndConfirmFailed_LogsCritical(t *testing.T) {
	h, subURL, _, logs := newTest(t, 500, nil, nil)
	h.HandleIfConfirmation(confirmation(subURL), "Choreo")
	if !strings.Contains(logs.String(), "CRITICAL") {
		t.Errorf("want a CRITICAL log, got:\n%s", logs.String())
	}
}

func TestMissingSubscribeURL_HandledWithoutFetch(t *testing.T) {
	h, _, hits, _ := newTest(t, 200, nil, nil)
	if !h.HandleIfConfirmation([]byte(`{"Type":"SubscriptionConfirmation"}`), "") || hits.Load() != 0 {
		t.Error("want handled (no alert) and nothing fetched")
	}
}

func TestIsSNSURL(t *testing.T) {
	cases := map[string]bool{
		"https://sns.us-east-1.amazonaws.com/?Action=ConfirmSubscription":     true,
		"https://sns.cn-north-1.amazonaws.com.cn/?Action=ConfirmSubscription": true,
		"http://sns.us-east-1.amazonaws.com/":                                 false,
		"https://sns.us-east-1.amazonaws.com.evil.com/":                       false,
		"https://evil.com/sns.us-east-1.amazonaws.com":                        false,
		"https://169.254.169.254/latest/meta-data":                            false,
	}
	for raw, want := range cases {
		u, _ := url.Parse(raw)
		if got := isSNSURL(u); got != want {
			t.Errorf("isSNSURL(%s) = %v, want %v", raw, got, want)
		}
	}
}
