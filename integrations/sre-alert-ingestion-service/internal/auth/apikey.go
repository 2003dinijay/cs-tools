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

package auth

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"sort"
	"strings"
)

// HeaderName carries the key for the seven vendors that can set a custom header.
const HeaderName = "X-API-Key"

// QueryParam carries the key for AWS SNS and GCP, which cannot set a header. The
// value reaches gateway access logs, so rotate these keys more often.
const QueryParam = "key"

// APIKey checks a per-vendor secret sent as the HeaderName header, a bearer token,
// the QueryParam query parameter or a Basic auth password; accepting all four is
// what lets all ten vendors authenticate. Keys are SHA-256 digests compared in
// constant time, held in memory: no database read on the webhook hot path.
type APIKey struct {
	digests map[string][32]byte
}

// NewAPIKey maps vendor name to secret. A key naming an unknown vendor always
// fails at startup; with requireEveryVendor, so does a vendor with no key, so
// enforcement cannot leave a route open. Audit mode clears it for partial rollout.
func NewAPIKey(keys map[string]string, vendors []string, requireEveryVendor bool) (*APIKey, error) {
	digests := make(map[string][32]byte, len(keys))
	var unknown []string
	for vendor, key := range keys {
		if key == "" {
			continue
		}
		if !slices.Contains(vendors, vendor) {
			unknown = append(unknown, vendor)
			continue
		}
		digests[vendor] = sha256.Sum256([]byte(key))
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return nil, fmt.Errorf("webhook key given for unknown vendor(s): %s", strings.Join(unknown, ", "))
	}
	if requireEveryVendor {
		var missing []string
		for _, vendor := range vendors {
			if _, ok := digests[vendor]; !ok {
				missing = append(missing, vendor)
			}
		}
		if len(missing) > 0 {
			sort.Strings(missing)
			return nil, fmt.Errorf("no webhook key for vendor(s): %s", strings.Join(missing, ", "))
		}
	}
	return &APIKey{digests: digests}, nil
}

// Authenticate reports whether the request carries vendor's key. A vendor with
// no key configured is rejected, so a missing key fails closed.
func (a *APIKey) Authenticate(r *http.Request, vendor string) error {
	want, ok := a.digests[vendor]
	if !ok {
		return fmt.Errorf("%w: no key configured for vendor %q", ErrUnauthorized, vendor)
	}
	presented := presentedKeys(r)
	if len(presented) == 0 {
		return fmt.Errorf("%w: no credential presented", ErrUnauthorized)
	}
	for _, c := range presented {
		got := sha256.Sum256([]byte(c.key))
		if subtle.ConstantTimeCompare(got[:], want[:]) == 1 {
			return nil
		}
	}
	return fmt.Errorf("%w: key from %s does not match", ErrUnauthorized, presented[0].where)
}

// credential is one reading of what a request presented; a bearer token yields
// two, the raw key and the base64 pair.
type credential struct {
	key string
	// where names the position for the log line, never the secret itself.
	where string
}

// presentedKeys returns every credential the request could carry, so a stale
// copy in one position does not mask a valid key in another.
func presentedKeys(r *http.Request) []credential {
	var out []credential
	if v := r.Header.Get(HeaderName); v != "" {
		out = append(out, credential{v, "header"})
	}
	if token, ok := bearerToken(r); ok {
		out = append(out, credential{token, "bearer token"})
		// The shape alerts-core's RequireAuth accepts, so one header suits both.
		if _, key, ok := decodeColonPair(token); ok {
			out = append(out, credential{key, "bearer token"})
		}
	}
	if v := r.URL.Query().Get(QueryParam); v != "" {
		out = append(out, credential{v, "query parameter"})
	}
	// Only reached when Authorization is Basic; a Bearer header fails BasicAuth.
	if _, password, hasBasic := r.BasicAuth(); hasBasic && password != "" {
		out = append(out, credential{password, "basic auth"})
	}
	return out
}

// bearerToken returns the Authorization: Bearer token; the scheme is matched
// case-insensitively, per RFC 7235.
func bearerToken(r *http.Request) (string, bool) {
	const prefix = "bearer "
	header := r.Header.Get("Authorization")
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return "", false
	}
	token := strings.TrimSpace(header[len(prefix):])
	return token, token != ""
}

// decodeColonPair decodes base64("<name>:<value>"), reporting false for a token
// that is not base64 or has no colon — the case for a bare key.
func decodeColonPair(token string) (name, value string, ok bool) {
	raw, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return "", "", false
	}
	name, value, found := strings.Cut(string(raw), ":")
	if !found || value == "" {
		return "", "", false
	}
	return name, value, true
}

// Audit wraps an Authenticator and never rejects, only logs. It is the rollout
// step between "none" and enforcement: without it, turning auth on would drop
// live alerts from every vendor not yet reconfigured.
type Audit struct {
	inner  Authenticator
	logger *slog.Logger
}

// NewAudit wraps inner so it logs what it would reject without rejecting anything.
func NewAudit(inner Authenticator, logger *slog.Logger) Authenticator {
	return Audit{inner: inner, logger: logger}
}

// Authenticate always returns nil, logging what the wrapped Authenticator would
// have rejected.
func (a Audit) Authenticate(r *http.Request, vendor string) error {
	if err := a.inner.Authenticate(r, vendor); err != nil {
		a.logger.Warn("auth would reject request",
			"vendor", vendor,
			"path", r.URL.Path,
			"reason", err.Error())
	}
	return nil
}
