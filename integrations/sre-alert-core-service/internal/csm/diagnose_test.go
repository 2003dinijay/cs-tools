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

package csm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sony/gobreaker/v2"
)

// newLoggedTestClient serves the token, then answers every API call with status; it returns the client and its log lines.
func newLoggedTestClient(t *testing.T, tokenStatus, status int) (*Client, *bytes.Buffer) {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(tokenStatus)
			_, _ = io.WriteString(w, `{"access_token":"t","token_type":"bearer","expires_in":3600}`)
			return
		}
		w.WriteHeader(status)
		_, _ = io.WriteString(w, `{"services":[],"total":0}`)
	}))
	saved := http.DefaultTransport
	http.DefaultTransport = srv.Client().Transport
	t.Cleanup(func() {
		srv.Client().CloseIdleConnections()
		http.DefaultTransport = saved
		srv.Close()
	})
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	return NewClient(Config{BaseURL: srv.URL, TokenURL: srv.URL + "/token", ClientID: "c", ClientSecret: "s", Logger: logger}), &logs
}

func lastLogLine(t *testing.T, logs *bytes.Buffer) map[string]any {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(logs.String()), "\n")
	var line map[string]any
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &line); err != nil {
		t.Fatalf("decode log line %q: %v", lines[len(lines)-1], err)
	}
	return line
}

func TestRequestLog_SaysWhetherTheCallReachedCSM(t *testing.T) {
	cases := map[string]struct {
		tokenStatus, status int
		wantMsg, wantKind   string
		wantReached         bool
		wantStatus          float64
	}{
		"success":        {http.StatusOK, http.StatusOK, "csm request succeeded", "", false, http.StatusOK},
		"status error":   {http.StatusOK, http.StatusServiceUnavailable, "csm request failed", KindHTTPStatus, true, http.StatusServiceUnavailable},
		"token rejected": {http.StatusUnauthorized, http.StatusOK, "csm request failed", KindTokenFailed, false, 0},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			client, logs := newLoggedTestClient(t, c.tokenStatus, c.status)
			_, _, err := client.SearchService(context.Background(), "svc")
			if (err != nil) != (c.wantKind != "") {
				t.Fatalf("err = %v", err)
			}
			line := lastLogLine(t, logs)
			if line["msg"] != c.wantMsg || line["path"] != "/services/search" {
				t.Errorf("log = %v", line)
			}
			if c.wantKind != "" && (line["error_kind"] != c.wantKind || line["reached_csm"] != c.wantReached) {
				t.Errorf("error_kind/reached_csm = %v/%v, want %s/%v", line["error_kind"], line["reached_csm"], c.wantKind, c.wantReached)
			}
			if c.wantStatus != 0 && line["status_code"] != c.wantStatus {
				t.Errorf("status_code = %v, want %v", line["status_code"], c.wantStatus)
			}
		})
	}
}

func TestErrorKind_NamesWhereTheCallStopped(t *testing.T) {
	refused := NewClient(Config{BaseURL: "https://127.0.0.1:1", TokenURL: "https://127.0.0.1:1/token", ClientID: "c", ClientSecret: "s", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	_, err := refused.do(context.Background(), http.MethodGet, "/x", nil)
	if k := ErrorKind(err); k != KindTokenFailed && k != KindConnection {
		t.Errorf("unreachable host: kind = %q (%v)", k, err)
	}
	if k := ErrorKind(gobreaker.ErrOpenState); k != KindBreakerOpen {
		t.Errorf("open breaker: kind = %q", k)
	}
	if k := ErrorKind(&Error{StatusCode: 302, Location: "https://login.example/"}); k != KindHTTPStatus {
		t.Errorf("redirect: kind = %q", k)
	}
	if ErrorKind(nil) != "" || ErrorKind(errors.New("x")) != KindOther {
		t.Errorf("nil/other kinds wrong")
	}
	attrs := ErrorAttrs(&Error{StatusCode: 302, Location: "https://login.example/"})
	if !strings.Contains(strings.Join(func() []string {
		var out []string
		for _, a := range attrs {
			if s, ok := a.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}(), ","), "redirect_location,https://login.example/") {
		t.Errorf("attrs = %v, want the redirect location", attrs)
	}
}
