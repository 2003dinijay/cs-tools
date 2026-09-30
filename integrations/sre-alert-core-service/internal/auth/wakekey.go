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
	"log/slog"
	"net/http"
	"strings"
)

// WakeHeaderName is the alternative to an Authorization header on the wake call.
const WakeHeaderName = "X-API-Key"

// RequireKey guards the wake endpoint with WAKE_API_KEY, sent as a bearer token
// (the key alone, or base64("<name>:<key>")) or the WakeHeaderName header. Failures
// are a bare 401, reason logged not returned. An empty key disables the check.
func RequireKey(key string, logger *slog.Logger, next http.Handler) http.Handler {
	if key == "" {
		return next
	}
	want := sha256.Sum256([]byte(key))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		presented := wakeKeysPresented(r)
		if len(presented) == 0 {
			logger.Warn("wake auth: no credential presented", "path", r.URL.Path)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		for _, c := range presented {
			got := sha256.Sum256([]byte(c))
			if subtle.ConstantTimeCompare(got[:], want[:]) == 1 {
				next.ServeHTTP(w, r)
				return
			}
		}
		logger.Warn("wake auth: key does not match", "path", r.URL.Path)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	})
}

// presentedKeys returns every key the request could be carrying: a bearer token
// counts twice, as the raw value and as the value half of a base64 pair.
func wakeKeysPresented(r *http.Request) []string {
	var out []string
	if token, ok := wakeBearerToken(r); ok {
		out = append(out, token)
		if _, key, ok := wakeDecodeColonPair(token); ok {
			out = append(out, key)
		}
	}
	if v := r.Header.Get(WakeHeaderName); v != "" {
		out = append(out, v)
	}
	return out
}

// bearerToken returns the Authorization: Bearer token; the scheme is matched
// case-insensitively, per RFC 7235.
func wakeBearerToken(r *http.Request) (string, bool) {
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
func wakeDecodeColonPair(token string) (name, value string, ok bool) {
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
