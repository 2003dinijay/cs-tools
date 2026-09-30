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
	"encoding/base64"
	"errors"
	"net/http/httptest"
	"testing"
)

const testKey = "s3cret-key-value"

func testAPIKey(t *testing.T) *APIKey {
	t.Helper()
	a, err := NewAPIKey(map[string]string{"aws": testKey}, []string{"aws"}, true)
	if err != nil {
		t.Fatalf("NewAPIKey: %v", err)
	}
	return a
}

// Each vendor can only send the key in certain positions, so all three must work.
func TestAPIKey_AcceptsEveryPlacement(t *testing.T) {
	a := testAPIKey(t)

	t.Run("header", func(t *testing.T) {
		r := httptest.NewRequest("POST", "/aws", nil)
		r.Header.Set(HeaderName, testKey)
		if err := a.Authenticate(r, "aws"); err != nil {
			t.Errorf("header key rejected: %v", err)
		}
	})

	t.Run("query parameter", func(t *testing.T) {
		r := httptest.NewRequest("POST", "/aws?"+QueryParam+"="+testKey, nil)
		if err := a.Authenticate(r, "aws"); err != nil {
			t.Errorf("query key rejected: %v", err)
		}
	})

	t.Run("basic auth password", func(t *testing.T) {
		r := httptest.NewRequest("POST", "/aws", nil)
		r.SetBasicAuth("ignored-username", testKey)
		if err := a.Authenticate(r, "aws"); err != nil {
			t.Errorf("basic auth key rejected: %v", err)
		}
	})

	t.Run("bearer raw key", func(t *testing.T) {
		r := httptest.NewRequest("POST", "/aws", nil)
		r.Header.Set("Authorization", "Bearer "+testKey)
		if err := a.Authenticate(r, "aws"); err != nil {
			t.Errorf("bearer raw key rejected: %v", err)
		}
	})

	// The shape alerts-core's RequireKey accepts, so one header suits both.
	t.Run("bearer base64 name:key", func(t *testing.T) {
		r := httptest.NewRequest("POST", "/aws", nil)
		token := base64.StdEncoding.EncodeToString([]byte("alert-ingestion:" + testKey))
		r.Header.Set("Authorization", "Bearer "+token)
		if err := a.Authenticate(r, "aws"); err != nil {
			t.Errorf("bearer base64 pair rejected: %v", err)
		}
	})

	// RFC 7235 makes the scheme case-insensitive.
	t.Run("lowercase bearer scheme", func(t *testing.T) {
		r := httptest.NewRequest("POST", "/aws", nil)
		r.Header.Set("Authorization", "bearer "+testKey)
		if err := a.Authenticate(r, "aws"); err != nil {
			t.Errorf("lowercase scheme rejected: %v", err)
		}
	})
}

// A stale copy in one position must not mask a valid key in another.
func TestAPIKey_ChecksEveryCandidate(t *testing.T) {
	a := testAPIKey(t)
	r := httptest.NewRequest("POST", "/aws?"+QueryParam+"=stale-key", nil)
	r.Header.Set(HeaderName, "also-stale")
	r.Header.Set("Authorization", "Bearer "+testKey)
	if err := a.Authenticate(r, "aws"); err != nil {
		t.Errorf("a valid bearer token should pass despite stale copies: %v", err)
	}
}

func TestAPIKey_Rejects(t *testing.T) {
	a := testAPIKey(t)

	t.Run("no credential", func(t *testing.T) {
		r := httptest.NewRequest("POST", "/aws", nil)
		if err := a.Authenticate(r, "aws"); !errors.Is(err, ErrUnauthorized) {
			t.Errorf("want ErrUnauthorized, got %v", err)
		}
	})

	t.Run("wrong key", func(t *testing.T) {
		r := httptest.NewRequest("POST", "/aws", nil)
		r.Header.Set(HeaderName, "not-the-key")
		if err := a.Authenticate(r, "aws"); !errors.Is(err, ErrUnauthorized) {
			t.Errorf("want ErrUnauthorized, got %v", err)
		}
	})

	// A key that is a prefix of the real one must not pass; the digest compare
	// makes length irrelevant.
	t.Run("prefix of the key", func(t *testing.T) {
		r := httptest.NewRequest("POST", "/aws", nil)
		r.Header.Set(HeaderName, testKey[:4])
		if err := a.Authenticate(r, "aws"); !errors.Is(err, ErrUnauthorized) {
			t.Errorf("want ErrUnauthorized, got %v", err)
		}
	})

	// Fail closed: a vendor with no key must be refused, not waved through.
	t.Run("vendor with no key configured", func(t *testing.T) {
		lenient, err := NewAPIKey(map[string]string{"aws": testKey}, []string{"aws", "datadog"}, false)
		if err != nil {
			t.Fatalf("NewAPIKey: %v", err)
		}
		r := httptest.NewRequest("POST", "/datadog", nil)
		r.Header.Set(HeaderName, testKey)
		if err := lenient.Authenticate(r, "datadog"); !errors.Is(err, ErrUnauthorized) {
			t.Errorf("an unconfigured vendor must fail closed, got %v", err)
		}
	})
}

// One vendor's key must not open another vendor's route.
func TestAPIKey_KeysAreNotInterchangeable(t *testing.T) {
	a, err := NewAPIKey(map[string]string{"aws": "aws-key", "datadog": "datadog-key"},
		[]string{"aws", "datadog"}, true)
	if err != nil {
		t.Fatalf("NewAPIKey: %v", err)
	}
	r := httptest.NewRequest("POST", "/datadog", nil)
	r.Header.Set(HeaderName, "aws-key")
	if err := a.Authenticate(r, "datadog"); !errors.Is(err, ErrUnauthorized) {
		t.Errorf("aws key should not authenticate datadog, got %v", err)
	}
}

// The header is checked first, ahead of a stale key left in the URL.
func TestAPIKey_HeaderTakesPrecedence(t *testing.T) {
	a := testAPIKey(t)
	r := httptest.NewRequest("POST", "/aws?"+QueryParam+"=stale-key", nil)
	r.Header.Set(HeaderName, testKey)
	if err := a.Authenticate(r, "aws"); err != nil {
		t.Errorf("header should win over query parameter: %v", err)
	}
}

func TestPresentedKeys_ReportsPositionNotSecret(t *testing.T) {
	r := httptest.NewRequest("POST", "/aws", nil)
	r.Header.Set(HeaderName, testKey)
	got := presentedKeys(r)
	if len(got) != 1 || got[0].key != testKey {
		t.Fatalf("presentedKeys = %+v", got)
	}
	if got[0].where == testKey {
		t.Error("position must not be the secret itself")
	}
}

func TestBearerToken(t *testing.T) {
	for name, header := range map[string]string{
		"no header":      "",
		"basic scheme":   "Basic abc",
		"bearer only":    "Bearer",
		"empty token":    "Bearer   ",
		"missing scheme": "abc123",
	} {
		t.Run(name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/aws", nil)
			if header != "" {
				r.Header.Set("Authorization", header)
			}
			if _, ok := bearerToken(r); ok {
				t.Errorf("%q should not yield a token", header)
			}
		})
	}
}

func TestDecodeColonPair(t *testing.T) {
	t.Run("valid pair", func(t *testing.T) {
		token := base64.StdEncoding.EncodeToString([]byte("user:secret"))
		name, value, ok := decodeColonPair(token)
		if !ok || name != "user" || value != "secret" {
			t.Errorf("got %q, %q, %v", name, value, ok)
		}
	})
	for name, token := range map[string]string{
		"not base64":  "!!!not-base64!!!",
		"no colon":    base64.StdEncoding.EncodeToString([]byte("nocolon")),
		"empty value": base64.StdEncoding.EncodeToString([]byte("user:")),
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, ok := decodeColonPair(token); ok {
				t.Errorf("%q should not decode to a pair", token)
			}
		})
	}
}
