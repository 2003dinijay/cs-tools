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
	"errors"
	"log/slog"
	"net/http"
	"strings"
)

const bearerPrefix = "Bearer "

// RequireAuth wraps next so a request must carry a valid Authorization header naming an enabled
// integration_users row before next is invoked. Both `-H "Authorization: Bearer <username>.<secret>"`
// and `-u <username>:<secret>` (sent as `Authorization: Basic base64(username:secret)`) are accepted.
// Every failure returns a generic 401; only the username (never the secret) is logged.
func RequireAuth(repo *UserRepo, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			username, secret, ok := parseCredentials(r)
			if !ok {
				logger.Warn("auth: missing or malformed Authorization header", "path", r.URL.Path)
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			u, err := repo.Get(r.Context(), username)
			if err != nil {
				if !errors.Is(err, ErrUserNotFound) {
					logger.Error("auth: lookup failed", "username", username, "error", err)
				} else {
					logger.Warn("auth: unknown user", "username", username)
				}
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			if !u.Enabled {
				logger.Warn("auth: disabled user", "username", username)
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			if !VerifySecret(secret, u.Salt, u.SecretHash, u.Iterations) {
				logger.Warn("auth: secret mismatch", "username", username)
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// parseCredentials extracts username/secret from either scheme: Bearer "<username>.<secret>",
// or Basic "<username>:<secret>" (what -u sends via net/http's BasicAuth decoding).
func parseCredentials(r *http.Request) (username, secret string, ok bool) {
	if header := r.Header.Get("Authorization"); strings.HasPrefix(header, bearerPrefix) {
		token := strings.TrimPrefix(header, bearerPrefix)
		username, secret, found := strings.Cut(token, ".")
		if !found || username == "" || secret == "" {
			return "", "", false
		}
		return username, secret, true
	}

	username, secret, ok = r.BasicAuth()
	if !ok || username == "" || secret == "" {
		return "", "", false
	}
	return username, secret, true
}
