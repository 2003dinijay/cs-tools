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

package dbfallback

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"sre-alert-ingestion-service/internal/model"
)

func discard() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

type chatServer struct {
	srv    *httptest.Server
	calls  atomic.Int64
	mu     sync.Mutex
	bodies []string
}

// newChatServer answers each post with status(call number), starting at 1.
func newChatServer(t *testing.T, status func(int64) int) *chatServer {
	t.Helper()
	cs := &chatServer{}
	cs.srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := cs.calls.Add(1)
		b, _ := io.ReadAll(r.Body)
		cs.mu.Lock()
		cs.bodies = append(cs.bodies, string(b))
		cs.mu.Unlock()
		w.WriteHeader(status(n))
	}))
	t.Cleanup(cs.srv.Close)
	return cs
}

func (cs *chatServer) body(i int) string {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return cs.bodies[i]
}

func newTestClient(t *testing.T, cs *chatServer, gap time.Duration) *Client {
	t.Helper()
	c, err := New(discard(), cs.srv.URL+"/v1/spaces/SPACE1/messages?key=k&token=t", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	c.http = cs.srv.Client()
	c.gap = gap
	return c
}

func ok(int64) int { return http.StatusOK }

func TestNew_RejectsNonHTTPSWithoutEchoingURL(t *testing.T) {
	for _, u := range []string{"http://chat.googleapis.com/v1/spaces/X/messages?key=secret", "not a url", "https://"} {
		_, err := New(discard(), u, time.Second)
		if err == nil {
			t.Errorf("New(%q) succeeded, want error", u)
			continue
		}
		if strings.Contains(err.Error(), "secret") {
			t.Errorf("error leaks the URL: %v", err)
		}
	}
}

func TestNotify_PostsEscapedCard(t *testing.T) {
	cs := newChatServer(t, ok)
	c := newTestClient(t, cs, time.Hour)
	a := model.Alert{Service: "api", Severity: "Critical", MetricName: "cpu", Description: "<users/all> & <b>x</b>"}
	c.Notify("aws", "req-1", []model.Alert{a})
	c.Close(context.Background())

	if cs.calls.Load() != 1 {
		t.Fatalf("calls = %d, want 1", cs.calls.Load())
	}
	body := cs.body(0)
	if strings.Contains(body, "<users/all>") {
		t.Errorf("unescaped sender text reached Chat: %s", body)
	}
	for _, want := range []string{"DB FALLBACK", "Critical | api", "cpu", "req-1", "\\u0026lt;users/all\\u0026gt;"} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q: %s", want, body)
		}
	}
}

func TestNotify_CoalescesDuringGapAndCloseFlushes(t *testing.T) {
	cs := newChatServer(t, ok)
	c := newTestClient(t, cs, time.Hour)
	c.Notify("aws", "r1", []model.Alert{{Service: "one"}})
	deadline := time.Now().Add(5 * time.Second)
	for cs.calls.Load() < 1 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	c.Notify("aws", "r2", []model.Alert{{Service: "two"}})
	c.Notify("gcp", "r3", []model.Alert{{Service: "three"}})
	if cs.calls.Load() != 1 {
		t.Fatalf("calls = %d during the gap, want 1", cs.calls.Load())
	}
	c.Close(context.Background())
	if cs.calls.Load() != 2 {
		t.Fatalf("calls = %d after close, want 2", cs.calls.Load())
	}
	if b := cs.body(1); !strings.Contains(b, "two") || !strings.Contains(b, "three") {
		t.Errorf("second card should hold both coalesced alerts: %s", b)
	}
}

func TestNotify_CapsListedAndPending(t *testing.T) {
	cs := newChatServer(t, ok)
	c := newTestClient(t, cs, time.Hour)
	alerts := make([]model.Alert, maxPending+50)
	for i := range alerts {
		alerts[i] = model.Alert{Service: "svc"}
	}
	c.mu.Lock()
	c.running = true // hold the loop so every alert lands in one batch
	c.mu.Unlock()
	c.Notify("aws", "r", alerts)
	c.mu.Lock()
	pending, dropped := len(c.pending), c.dropped
	c.running = false
	c.mu.Unlock()
	if pending != maxPending || dropped != 50 {
		t.Fatalf("pending = %d, dropped = %d; want %d, 50", pending, dropped, maxPending)
	}

	c.Notify("aws", "r", nil)
	c.Close(context.Background())
	body := cs.body(0)
	if got := strings.Count(body, "svc"); got != maxListed {
		t.Errorf("listed %d alerts, want %d", got, maxListed)
	}
	if want := "530 more not shown"; !strings.Contains(body, want) {
		t.Errorf("body missing %q", want)
	}
}

func TestSend_RetriesServerErrorsButNotClientErrors(t *testing.T) {
	cases := map[string]struct {
		status func(int64) int
		want   int64
	}{
		"5xx then ok": {func(n int64) int {
			if n == 1 {
				return http.StatusServiceUnavailable
			}
			return http.StatusOK
		}, 2},
		"429 retried":     {func(int64) int { return http.StatusTooManyRequests }, sendAttempts},
		"400 not retried": {func(int64) int { return http.StatusBadRequest }, 1},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			cs := newChatServer(t, tc.status)
			c := newTestClient(t, cs, time.Hour)
			c.Notify("aws", "r", []model.Alert{{Service: "svc"}})
			c.Close(context.Background())
			if got := cs.calls.Load(); got != tc.want {
				t.Errorf("calls = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestSpaceID(t *testing.T) {
	if got := spaceID("https://chat.googleapis.com/v1/spaces/AAA/messages?key=k"); got != "AAA" {
		t.Errorf("spaceID = %q, want AAA", got)
	}
	if got := spaceID("https://example.com/hook"); got != "unknown" {
		t.Errorf("spaceID = %q, want unknown", got)
	}
}
