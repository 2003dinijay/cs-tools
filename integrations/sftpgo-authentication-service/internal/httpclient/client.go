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

package httpclient

import (
	"fmt"
	"net/http"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/sftpgo-authentication-service/internal/constants"
	"github.com/wso2-open-operations/cs-tools/integrations/sftpgo-authentication-service/internal/log"
)

// LoggingTransport implements http.RoundTripper and logs requests and responses.
type LoggingTransport struct {
	// Transport is the underlying RoundTripper used to execute the request.
	Transport http.RoundTripper
	// Logger is the application-wide logger.
	Logger *log.AppLogger
}

// RoundTrip executes a single HTTP transaction and logs the details.
func (t *LoggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()

	// Log Request
	t.logRequest(req)

	resp, err := t.Transport.RoundTrip(req)
	duration := time.Since(start)

	if err != nil {
		t.Logger.Error("OUTGOING REQUEST FAILED: duration=%v error=%v", duration, err)
		return nil, err
	}

	// Log Response
	t.logResponse(resp, duration)

	return resp, nil
}

// logRequest logs the standard details of an outgoing HTTP request.
//
// Request/response bodies are deliberately never logged, even at TRACE: this
// client also carries IdPService.PostToAuthnEndpoint, whose request bodies
// include keyboard-interactive answers -- which can be the user's password.
// There is no generic, safe way to redact "the sensitive field" from an
// arbitrary JSON payload here, so the body is not logged at all rather than
// risk leaking credentials into logs. Headers are still logged at TRACE
// (with Authorization redacted by sanitizeHeaders).
func (t *LoggingTransport) logRequest(req *http.Request) {
	t.Logger.Debug("OUTGOING REQUEST: method=%s url=%s", req.Method, req.URL.String())

	if t.Logger.IsTraceEnabled() {
		t.Logger.Trace("OUTGOING REQUEST HEADERS: %v", sanitizeHeaders(req.Header))
	}
}

// logResponse logs the details of an incoming HTTP response, including duration.
//
// As with logRequest, the response body is never logged -- see logRequest's
// comment for why.
func (t *LoggingTransport) logResponse(resp *http.Response, duration time.Duration) {
	t.Logger.Debug("INCOMING RESPONSE: status=%s duration=%v", resp.Status, duration)

	if t.Logger.IsTraceEnabled() {
		t.Logger.Trace("INCOMING RESPONSE HEADERS: %v", sanitizeHeaders(resp.Header))
	}
}

// sanitizeHeaders returns a copy of the headers with the Authorization value redacted.
func sanitizeHeaders(h http.Header) http.Header {
	sanitized := h.Clone()
	if sanitized.Get(constants.HeaderAuthorization) != "" {
		sanitized.Set(constants.HeaderAuthorization, "[REDACTED]")
	}
	return sanitized
}

// NewLoggingClient returns an http.Client configured with the LoggingTransport
// and RefuseInsecureRedirect, so no outgoing call this service makes can be
// redirected to a non-HTTPS destination.
func NewLoggingClient(timeout time.Duration, logger *log.AppLogger) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &LoggingTransport{
			Transport: http.DefaultTransport,
			Logger:    logger,
		},
		CheckRedirect: RefuseInsecureRedirect,
	}
}

// RefuseInsecureRedirect is an http.Client CheckRedirect function that aborts
// any redirect whose destination is not HTTPS.
//
// Go's default redirect handling only strips sensitive headers (Authorization,
// Cookie, etc.) when a redirect crosses to a different host; a same-host
// HTTPS -> HTTP downgrade keeps those headers intact. Every client this
// service builds carries a bearer token or admin credential in its
// Authorization header (the SFTPGo admin API client, the JWKS-fetching
// client, the IdP client), so an attacker-controlled or misconfigured
// same-host redirect to plain HTTP would otherwise leak that credential in
// cleartext. Use this on every http.Client this service constructs.
func RefuseInsecureRedirect(req *http.Request, via []*http.Request) error {
	if req.URL.Scheme != "https" {
		return fmt.Errorf("refusing redirect to non-HTTPS URL: %s", req.URL)
	}
	return nil
}
