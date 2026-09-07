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

// Package httpsec is a small, shared set of guards against sending OAuth2
// client credentials or bearer tokens over plaintext HTTP — used by both
// internal/ledger and internal/notify, since both authenticate against a
// remote service with the same shared OAuth2 app credentials. Kept as its
// own package rather than duplicated in each, so the two clients can't
// silently drift apart on what "secure enough" means.
package httpsec

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
)

// errInvalidURL is returned as-is (never wrapped) by RequireSecureURL: unlike
// a wrapped error, this carries no risk of echoing url.Parse's own error
// text — which includes the input it failed on — into a log line.
var errInvalidURL = errors.New("invalid URL")

// RequireSecureURL returns an error unless rawURL uses https, or its host
// is loopback (localhost/127.0.0.1/::1) — plain http is allowed only for
// loopback so local development can still point these clients at a
// non-TLS-terminated server without needing a certificate. Any real
// deployment target (entity-service, the email-sending service, any future
// service client this component grows) must be https.
//
// Deliberately never includes rawURL itself, or url.Parse's own error text,
// in the returned error: this error is logged at startup (see
// cmd/server/main.go), a malformed or insecure URL can carry userinfo or a
// query-string token, and url.Parse's error message echoes back the input
// it failed on — either would otherwise leak into logs.
func RequireSecureURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return errInvalidURL
	}
	if u.Scheme == "https" || isLoopbackHost(u.Hostname()) {
		return nil
	}
	return fmt.Errorf("URL must use https (plain http is only allowed for localhost/127.0.0.1)")
}

func isLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// RejectInsecureRedirects sets client's CheckRedirect to refuse following
// any redirect whose target isn't https. Defense in depth against a
// compromised or misconfigured server redirecting an authenticated request
// — bearer token in the Authorization header, or an OAuth2 token-fetch
// request carrying the client secret — to a plaintext endpoint mid-flight;
// net/http follows a same-body redirect by default without this guard,
// including across schemes.
//
// Unlike RequireSecureURL, loopback is NOT exempted here: RequireSecureURL's
// loopback allowance is for an explicitly configured local endpoint chosen
// by whoever set TokenURL/BaseURL, but a same-host redirect from a
// legitimate https endpoint down to plaintext loopback is exactly the
// downgrade this guard exists to stop — allowing it here would undermine
// the https requirement the initial URL check just enforced. The error
// also never includes the redirect target URL, for the same
// log-leakage reason RequireSecureURL's own doc comment gives.
func RejectInsecureRedirects(client *http.Client) {
	client.CheckRedirect = func(req *http.Request, _ []*http.Request) error {
		if req.URL.Scheme == "https" {
			return nil
		}
		return fmt.Errorf("httpsec: refusing to follow redirect to a non-https URL")
	}
}
