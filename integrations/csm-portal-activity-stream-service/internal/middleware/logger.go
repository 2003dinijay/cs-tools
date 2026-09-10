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

package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

// responseWriter wraps http.ResponseWriter to capture the *effective* status
// code written by the downstream handler, so the access log reflects what
// the client actually received. Real ResponseWriter semantics commit the
// status at the first WriteHeader or the first Write, whichever comes
// first — every call after that is a no-op on the wire even though the
// handler is still free to call WriteHeader again. Without tracking that
// commit point here too, a handler that (legitimately or not) calls
// WriteHeader more than once, or writes a body without ever calling
// WriteHeader at all, would log a status the client never saw.
type responseWriter struct {
	http.ResponseWriter
	status    int
	committed bool
}

func (rw *responseWriter) WriteHeader(code int) {
	if rw.committed {
		return
	}
	rw.committed = true
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

// Write implicitly commits a 200 (net/http's own behavior) if the handler
// never called WriteHeader explicitly before its first Write.
func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.committed {
		rw.committed = true
		rw.status = http.StatusOK
	}
	return rw.ResponseWriter.Write(b)
}

// Flush implements http.Flusher by delegating to the underlying
// ResponseWriter if it supports it. Embedding http.ResponseWriter as an
// interface field only promotes that interface's own methods, not the
// concrete writer's full method set, so without this a handler behind
// Logger that type-asserts w.(http.Flusher) — e.g. a long-lived SSE
// connection, see handler.StreamCaseActivities — would fail the assertion
// and never be able to flush partial writes to the client. A flush commits
// whatever status is currently set, same as a real ResponseWriter.
func (rw *responseWriter) Flush() {
	rw.committed = true
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Logger is an HTTP middleware that logs each completed request via slog. The
// correlation ID is included automatically in every record when ConfigureLogger
// has been called (it is attached by the ctxHandler from the context).
//
// Logs r.Pattern (the matched route template, e.g. "GET /cases/{id}/activities/stream"
// — set by http.ServeMux once it dispatches, so it's already populated by the
// time next.ServeHTTP returns here) instead of r.URL.Path: the literal path can
// carry caller-controlled values (a path parameter, or anything at all on a
// route that matched nothing), and this service's own logging policy is IDs
// and error summaries only, never arbitrary request content. Logs the fixed
// string "unmatched" when nothing matched, rather than falling back to the
// literal path.
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		pattern := r.Pattern
		if pattern == "" {
			pattern = "unmatched"
		}
		slog.InfoContext(r.Context(), "request completed",
			"method", r.Method,
			"path", pattern,
			"status", rw.status,
			"elapsed", time.Since(start).String(),
		)
	})
}
