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
	"bufio"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"
)

// responseWriter wraps http.ResponseWriter to capture the status code written
// by the downstream handler so it can be included in the access log.
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

// Hijack forwards to the underlying ResponseWriter's http.Hijacker
// implementation. Wrapping http.ResponseWriter in responseWriter otherwise
// hides Hijack from callers that type-assert for it — including
// gorilla/websocket's Upgrade, which GET /ws (internal/handler/websocket.go)
// relies on to switch the connection to WebSocket.
func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := rw.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("underlying ResponseWriter does not support hijacking")
	}
	return hijacker.Hijack()
}

// deadlineSetter matches the unexported interface net/http's response type
// implements, which http.ResponseController relies on via a type assertion.
type deadlineSetter interface {
	SetWriteDeadline(time.Time) error
	SetReadDeadline(time.Time) error
}

// SetWriteDeadline and SetReadDeadline forward to the underlying
// ResponseWriter, for the same reason Hijack does above: they let
// http.NewResponseController(w).SetWriteDeadline/SetReadDeadline work through
// this wrapper — used by handlers whose upstream call chain can legitimately
// exceed the server's global WriteTimeout (see
// internal/handler/product_consumption.go's GetDeploymentLicense).
func (rw *responseWriter) SetWriteDeadline(deadline time.Time) error {
	ds, ok := rw.ResponseWriter.(deadlineSetter)
	if !ok {
		return fmt.Errorf("underlying ResponseWriter does not support setting a write deadline")
	}
	return ds.SetWriteDeadline(deadline)
}

func (rw *responseWriter) SetReadDeadline(deadline time.Time) error {
	ds, ok := rw.ResponseWriter.(deadlineSetter)
	if !ok {
		return fmt.Errorf("underlying ResponseWriter does not support setting a read deadline")
	}
	return ds.SetReadDeadline(deadline)
}

// Logger is an HTTP middleware that logs each completed request via slog. The
// correlation ID is included automatically in every record when ConfigureLogger
// has been called (it is attached by the ctxHandler from the context).
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		slog.InfoContext(r.Context(), "request completed",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.status,
			"elapsed", time.Since(start).String(),
		)
	})
}
