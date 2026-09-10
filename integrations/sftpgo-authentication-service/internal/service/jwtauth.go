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

// Package service implements JWTAuthService, which validates the gateway-issued
// x-jwt-assertion presented as the password on SFTPGo's external_auth_hook (the
// web attachment access path).
//
// This mirrors, field-for-field, the validation performed by
// apps/csm-portal/backend/internal/middleware/auth.go: same claim shape, same
// issuer/audience/expiration/leeway checks, and the same x5c-stripping JWKS
// transport workaround for IdPs that publish certificates with a negative
// serial number (rejected by Go's x509 parser since Go 1.23). That file is the
// proven-against-the-real-IdP implementation; nothing here diverges from it on
// purpose. It is a separate copy because this service has its own go.mod,
// independent of the CSM backend's.
package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"

	"github.com/wso2-open-operations/cs-tools/integrations/sftpgo-authentication-service/internal/config"
	"github.com/wso2-open-operations/cs-tools/integrations/sftpgo-authentication-service/internal/httpclient"
	"github.com/wso2-open-operations/cs-tools/integrations/sftpgo-authentication-service/internal/log"
)

// JWTUserInfo holds the authenticated user's identity extracted from the JWT.
type JWTUserInfo struct {
	Email  string
	UserID string
	Groups []string
}

// jwtClaims defines the expected JWT payload fields, matching
// middleware.jwtClaims in the CSM backend.
type jwtClaims struct {
	Email  string   `json:"email"`
	UserID string   `json:"userid"`
	Groups []string `json:"groups"`
	jwt.RegisteredClaims
}

// JWTAuthService validates x-jwt-assertion tokens against a JWKS endpoint.
type JWTAuthService struct {
	cfg     *config.Config
	logger  *log.AppLogger
	keyFunc jwt.Keyfunc
}

// NewJWTAuthService builds a JWTAuthService and fetches the JWKS eagerly, the
// same way middleware.Auth does at startup.
//
// Unlike the CSM backend (where a JWKS load failure is fatal because the whole
// service is authenticated), this service also serves the pre-existing
// pre-login/keyboard-interactive hooks that must keep working regardless of
// this feature's configuration. So a misconfigured or unreachable JWKS
// endpoint here returns an error instead of panicking; the caller decides
// whether to run without the external_auth_hook enabled rather than crashing
// the whole process.
func NewJWTAuthService(cfg *config.Config, logger *log.AppLogger) (*JWTAuthService, error) {
	if cfg.AuthJWKSEndpoint == "" {
		return nil, fmt.Errorf("AUTH_JWKS_ENDPOINT is not configured")
	}
	if cfg.AuthIssuer == "" {
		return nil, fmt.Errorf("AUTH_ISSUER is not configured")
	}
	// Fix #11: an empty AUTH_AUDIENCE would otherwise make ValidateAndExtract
	// skip audience validation entirely (see the `if len(s.cfg.AuthAudiences) >
	// 0` guard below), accepting any valid token from the trusted issuer
	// regardless of its intended audience. Reject that at construction time
	// instead of silently allowing all audiences. This is treated the same as
	// a JWKS-fetch failure: the caller (main.go) disables the external-auth-hook
	// path and keeps the pre-login/keyboard-interactive hooks running, rather
	// than crashing the whole service over a misconfiguration of a path those
	// hooks don't use.
	if len(cfg.AuthAudiences) == 0 {
		return nil, fmt.Errorf("AUTH_AUDIENCE is not configured: at least one accepted audience is required when AUTH_JWKS_ENDPOINT/AUTH_ISSUER are set")
	}

	svc := &JWTAuthService{cfg: cfg, logger: logger}

	if cfg.AuthTokenValidatorEnabled {
		// CheckRedirect refuses a redirect to a non-HTTPS destination (see
		// httpclient.RefuseInsecureRedirect): this client's response is used
		// as-is to establish cryptographic trust for the external-auth-hook
		// path, so a downgraded, MITM-able HTTP fetch of the JWKS document
		// must never be silently followed.
		client := &http.Client{
			Transport:     &x5cStrippingTransport{base: http.DefaultTransport},
			Timeout:       time.Duration(cfg.HTTPTimeout) * time.Second,
			CheckRedirect: httpclient.RefuseInsecureRedirect,
		}
		// Override.HTTPTimeout governs jwkset's synchronous initial JWKS fetch
		// during this call; left unset, jwkset defaults to 60s regardless of
		// this service's own HTTP_TIMEOUT (default 15s), which could block
		// startup for a minute if the endpoint stalls.
		jwks, err := keyfunc.NewDefaultOverrideCtx(context.Background(), []string{cfg.AuthJWKSEndpoint}, keyfunc.Override{
			Client:      client,
			HTTPTimeout: time.Duration(cfg.HTTPTimeout) * time.Second,
		})
		if err != nil {
			return nil, fmt.Errorf("initialise JWKS from %s: %w", cfg.AuthJWKSEndpoint, err)
		}
		svc.keyFunc = jwks.Keyfunc
	}

	return svc, nil
}

// ValidateAndExtract verifies tokenStr (signature, issuer, audience, expiration,
// with clock-skew leeway) and returns the identity claims on success. Mirrors
// middleware.extractUserInfo exactly.
func (s *JWTAuthService) ValidateAndExtract(tokenStr string) (*JWTUserInfo, error) {
	var c jwtClaims

	if !s.cfg.AuthTokenValidatorEnabled {
		// Local mode: decode without signature verification.
		_, _, err := new(jwt.Parser).ParseUnverified(tokenStr, &c)
		if err != nil {
			return nil, fmt.Errorf("decode token: %w", err)
		}
	} else {
		// WithValidMethods pins the accepted signing algorithm to RS256, matching
		// the RSA keys the platform's real JWKS endpoint publishes (same as
		// apps/csm-portal/backend/internal/middleware/auth.go, which sources
		// keys from the same JWKS via the same MicahParks/keyfunc client). This
		// closes algorithm-confusion at the library-contract level instead of
		// relying on golang-jwt/jwt/v5's implicit key-type checking (an RSA
		// public key returned by keyFunc will not satisfy an HMAC Verify call,
		// but that is incidental, not a documented guarantee).
		token, err := jwt.ParseWithClaims(tokenStr, &c, s.keyFunc,
			jwt.WithValidMethods([]string{"RS256"}),
			jwt.WithIssuer(s.cfg.AuthIssuer),
			jwt.WithLeeway(config.ExternalAuthClockSkew),
			jwt.WithExpirationRequired(),
		)
		if err != nil {
			return nil, fmt.Errorf("validate token: %w", err)
		}
		if !token.Valid {
			return nil, fmt.Errorf("invalid token")
		}
		if len(s.cfg.AuthAudiences) > 0 {
			tokenAuds, _ := token.Claims.GetAudience()
			if !hasAnyAudience(tokenAuds, s.cfg.AuthAudiences) {
				return nil, fmt.Errorf("token audience not accepted")
			}
		}
	}

	if c.Email == "" {
		return nil, fmt.Errorf("token missing email claim")
	}
	if c.UserID == "" {
		return nil, fmt.Errorf("token missing userid claim")
	}

	return &JWTUserInfo{
		Email:  c.Email,
		UserID: c.UserID,
		Groups: c.Groups,
	}, nil
}

func hasAnyAudience(tokenAuds jwt.ClaimStrings, expected []string) bool {
	for _, want := range expected {
		for _, got := range tokenAuds {
			if got == want {
				return true
			}
		}
	}
	return false
}

// x5cStrippingTransport removes the "x5c" certificate chain from every key in
// a JWKS response before it reaches the jwkset parser. Verification only
// needs "n"/"e" (or the EC/OKP equivalents); jwkset unconditionally parses
// "x5c" as X.509 certificates, and some IdPs publish certs with a negative
// serial number that Go's x509 parser rejects since Go 1.23, which would
// otherwise make the whole JWK Set fail to load. Copied verbatim from
// apps/csm-portal/backend/internal/middleware/auth.go.
type x5cStrippingTransport struct {
	base http.RoundTripper
}

func (t *x5cStrippingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return resp, err
	}

	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("read JWKS response body: %w", err)
	}

	var jwks struct {
		Keys []map[string]any `json:"keys"`
	}
	if err := json.Unmarshal(body, &jwks); err != nil {
		// Not a JWKS document we can sanitize; hand back the original body untouched.
		resp.Body = io.NopCloser(bytes.NewReader(body))
		return resp, nil
	}

	for _, key := range jwks.Keys {
		delete(key, "x5c")
	}
	sanitized, err := json.Marshal(jwks)
	if err != nil {
		return nil, fmt.Errorf("marshal sanitized JWKS: %w", err)
	}

	resp.Body = io.NopCloser(bytes.NewReader(sanitized))
	resp.ContentLength = int64(len(sanitized))
	resp.Header.Set("Content-Length", fmt.Sprint(len(sanitized)))
	return resp, nil
}
