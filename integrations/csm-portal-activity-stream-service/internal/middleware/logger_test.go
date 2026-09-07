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

package middleware_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-portal-activity-stream-service/internal/middleware"
)

// capturingHandler is a minimal slog.Handler that records every attr of the
// first record it receives, so a test can assert on what Logger actually
// logs — not just what the client's own recorder captured, which (via
// httptest.ResponseRecorder's own first-write-wins behavior) can pass even
// when what Logger itself logs is wrong.
type capturingHandler struct {
	attrs map[string]slog.Value
}

func newCapturingHandler() *capturingHandler {
	return &capturingHandler{attrs: map[string]slog.Value{}}
}

func (h *capturingHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *capturingHandler) Handle(_ context.Context, r slog.Record) error {
	r.Attrs(func(a slog.Attr) bool {
		h.attrs[a.Key] = a.Value
		return true
	})
	return nil
}

func (h *capturingHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *capturingHandler) WithGroup(_ string) slog.Handler      { return h }

// captureLog swaps slog's default logger for the duration of fn, restoring
// the previous one afterward, and returns whatever the first log record
// captured. Not safe to run concurrently with another test doing the same —
// callers must not mark themselves t.Parallel().
func captureLog(t *testing.T, fn func()) *capturingHandler {
	t.Helper()
	h := newCapturingHandler()
	prev := slog.Default()
	slog.SetDefault(slog.New(h))
	defer slog.SetDefault(prev)
	fn()
	return h
}

func TestLogger_CallsNextAndPreservesResponse(t *testing.T) {
	t.Parallel()

	var nextCalled bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("hello"))
	})

	r := httptest.NewRequest(http.MethodGet, "/some/path", nil)
	w := httptest.NewRecorder()
	middleware.Logger(next).ServeHTTP(w, r)

	if !nextCalled {
		t.Fatal("Logger did not call the wrapped handler")
	}
	if w.Code != http.StatusTeapot {
		t.Errorf("status = %d, want %d", w.Code, http.StatusTeapot)
	}
	if body := w.Body.String(); body != "hello" {
		t.Errorf("body = %q, want %q", body, "hello")
	}
}

func TestLogger_DefaultsStatusToOKWhenUnset(t *testing.T) {
	t.Parallel()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	middleware.Logger(next).ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestLogger_IgnoresWriteHeaderAfterWrite(t *testing.T) {
	t.Parallel()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("hi"))
		w.WriteHeader(http.StatusCreated)
	})

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	middleware.Logger(next).ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d (superfluous WriteHeader after Write must be ignored)", w.Code, http.StatusOK)
	}
}

func TestLogger_IgnoresRepeatedWriteHeader(t *testing.T) {
	t.Parallel()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.WriteHeader(http.StatusAccepted)
	})

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	middleware.Logger(next).ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d (only the first WriteHeader call should count)", w.Code, http.StatusCreated)
	}
}

// Not t.Parallel(): swaps the global slog default for the duration of the
// test (see captureLog) and must not overlap with another test doing the
// same.
func TestLogger_LogsEffectiveStatusNotASupersededWriteHeader(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("hi")) // implicitly commits 200
		w.WriteHeader(http.StatusCreated)
	})

	var clientStatus int
	captured := captureLog(t, func() {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		middleware.Logger(next).ServeHTTP(w, r)
		clientStatus = w.Code
	})

	if clientStatus != http.StatusOK {
		t.Fatalf("client status = %d, want %d", clientStatus, http.StatusOK)
	}
	if loggedStatus := captured.attrs["status"].Int64(); loggedStatus != int64(clientStatus) {
		t.Errorf("logged status = %d, want it to match what the client actually received (%d)", loggedStatus, clientStatus)
	}
}

// Not t.Parallel(): see TestLogger_LogsEffectiveStatusNotASupersededWriteHeader.
func TestLogger_LogsFirstOfRepeatedWriteHeaderCalls(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.WriteHeader(http.StatusAccepted)
	})

	var clientStatus int
	captured := captureLog(t, func() {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		middleware.Logger(next).ServeHTTP(w, r)
		clientStatus = w.Code
	})

	if clientStatus != http.StatusCreated {
		t.Fatalf("client status = %d, want %d", clientStatus, http.StatusCreated)
	}
	if loggedStatus := captured.attrs["status"].Int64(); loggedStatus != int64(clientStatus) {
		t.Errorf("logged status = %d, want it to match what the client actually received (%d)", loggedStatus, clientStatus)
	}
}

// Not t.Parallel(): see TestLogger_LogsEffectiveStatusNotASupersededWriteHeader.
// Logger wraps the whole mux (see cmd/server/main.go's real middleware
// chain), not individual routes — r.Pattern is only populated once the mux
// itself has matched, so that's the shape this test (and the one below it)
// exercises.
func TestLogger_LogsMatchedRoutePatternNotRawPath(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /cases/{id}/activities/stream", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	captured := captureLog(t, func() {
		r := httptest.NewRequest(http.MethodGet, "/cases/attacker@example.com/activities/stream", nil)
		w := httptest.NewRecorder()
		middleware.Logger(mux).ServeHTTP(w, r)
	})

	if got := captured.attrs["path"].String(); got != "GET /cases/{id}/activities/stream" {
		t.Errorf("logged path = %q, want the matched route pattern, not the literal caller-supplied path", got)
	}
}

// Not t.Parallel(): see TestLogger_LogsEffectiveStatusNotASupersededWriteHeader.
func TestLogger_LogsUnmatchedForNoRouteMatch(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	captured := captureLog(t, func() {
		r := httptest.NewRequest(http.MethodGet, "/does-not-exist/attacker@example.com", nil)
		w := httptest.NewRecorder()
		middleware.Logger(mux).ServeHTTP(w, r)
	})

	if got := captured.attrs["path"].String(); got != "unmatched" {
		t.Errorf(`logged path = %q, want "unmatched" rather than falling back to the literal caller-supplied path`, got)
	}
}
