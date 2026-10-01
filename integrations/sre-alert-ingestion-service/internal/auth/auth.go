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

// Package auth is the vendor-route auth hook, selected by auth.mode: "none" accepts
// everything, "audit" logs mismatches without rejecting, "apikey" enforces (see
// APIKey). Another scheme is a new Authenticator and mode, not a router change.
package auth

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
)

// The supported auth.mode values.
const (
	ModeNone   = "none"
	ModeAudit  = "audit"
	ModeAPIKey = "apikey"
	// ModeIntegrationUsers verifies against alerts-core's integration_users table.
	ModeIntegrationUsers = "integration_users"
)

// ErrUnauthorized is returned by an Authenticator that rejects a request; the router answers 401.
var ErrUnauthorized = errors.New("unauthorized")

// Authenticator decides whether a vendor webhook request may proceed. It runs after the
// router has matched the vendor and before the body is transformed.
type Authenticator interface {
	Authenticate(r *http.Request, vendor string) error
}

// None accepts every request.
type None struct{}

// Authenticate always succeeds.
func (None) Authenticate(*http.Request, string) error { return nil }

// New returns the Authenticator for mode, erroring on an unknown one so a typo in
// config.toml fails at startup rather than leaving the routes open. ModeNone ignores
// every argument; ModeIntegrationUsers needs users.
func New(mode string, keys map[string]string, vendors []string, users *IntegrationUsers, logger *slog.Logger) (Authenticator, error) {
	switch mode {
	case ModeNone:
		return None{}, nil
	case ModeAudit:
		// Not requireEveryVendor: audit is for the window where some have no key.
		inner, err := NewAPIKey(keys, vendors, false)
		if err != nil {
			return nil, err
		}
		return Audit{inner: inner, logger: logger}, nil
	case ModeAPIKey:
		return NewAPIKey(keys, vendors, true)
	case ModeIntegrationUsers:
		if users == nil {
			return nil, fmt.Errorf("auth.mode %q needs a Cassandra session", mode)
		}
		return users, nil
	default:
		return nil, fmt.Errorf("unknown auth.mode %q (want %q, %q, %q or %q)", mode, ModeNone, ModeAudit, ModeAPIKey, ModeIntegrationUsers)
	}
}
