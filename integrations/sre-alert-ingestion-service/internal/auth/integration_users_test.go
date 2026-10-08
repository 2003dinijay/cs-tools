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
	"crypto/pbkdf2"
	"crypto/sha256"
	"encoding/base64"
	"net/http/httptest"
	"testing"
	"time"
)

// hashes produces the stored salt/hash pair for secret, the way alerts-core's
// cmd/user writes it.
func hashes(t *testing.T, secret string, iterations int) (saltB64, hashB64 string) {
	t.Helper()
	salt := []byte("0123456789abcdef")
	key, err := pbkdf2.Key(sha256.New, secret, salt, iterations, keyLen)
	if err != nil {
		t.Fatalf("pbkdf2: %v", err)
	}
	return base64.StdEncoding.EncodeToString(salt), base64.StdEncoding.EncodeToString(key)
}

// A hash written by alerts-core must verify here: both must agree on the algorithm.
func TestVerifySecret(t *testing.T) {
	salt, hash := hashes(t, "correct-secret", 100)

	if !verifySecret("correct-secret", salt, hash, 100) {
		t.Error("the correct secret must verify")
	}
	if verifySecret("wrong-secret", salt, hash, 100) {
		t.Error("a wrong secret must not verify")
	}
	if verifySecret("correct-secret", salt, hash, 101) {
		t.Error("a mismatched iteration count must not verify")
	}
	if verifySecret("correct-secret", "not-base64!!", hash, 100) {
		t.Error("a malformed salt must not verify")
	}
}

func TestParseCredentials(t *testing.T) {
	t.Run("bearer base64 pair", func(t *testing.T) {
		token := base64.StdEncoding.EncodeToString([]byte("alice:s3cr3t"))
		r := httptest.NewRequest("POST", "/", nil)
		r.Header.Set("Authorization", "Bearer "+token)
		u, s, ok := parseCredentials(r)
		if !ok || u != "alice" || s != "s3cr3t" {
			t.Errorf("got (%q, %q, %v)", u, s, ok)
		}
	})

	t.Run("basic auth", func(t *testing.T) {
		r := httptest.NewRequest("POST", "/", nil)
		r.SetBasicAuth("alice", "s3cr3t")
		u, s, ok := parseCredentials(r)
		if !ok || u != "alice" || s != "s3cr3t" {
			t.Errorf("got (%q, %q, %v)", u, s, ok)
		}
	})

	// RFC 7235 makes the scheme case-insensitive, as BasicAuth already is for Basic.
	t.Run("scheme is case-insensitive", func(t *testing.T) {
		token := base64.StdEncoding.EncodeToString([]byte("alice:s3cr3t"))
		for _, scheme := range []string{"Bearer ", "bearer ", "BEARER "} {
			r := httptest.NewRequest("POST", "/", nil)
			r.Header.Set("Authorization", scheme+token)
			if _, _, ok := parseCredentials(r); !ok {
				t.Errorf("%q should parse", scheme)
			}
		}
	})

	t.Run("malformed", func(t *testing.T) {
		for _, h := range []string{
			"", "Bearer not-base64!!",
			"Bearer " + base64.StdEncoding.EncodeToString([]byte("no-colon")),
			"Bearer " + base64.StdEncoding.EncodeToString([]byte(":no-user")),
			"Bogus scheme",
		} {
			r := httptest.NewRequest("POST", "/", nil)
			if h != "" {
				r.Header.Set("Authorization", h)
			}
			if _, _, ok := parseCredentials(r); ok {
				t.Errorf("%q should not parse", h)
			}
		}
	})
}

// The cache is what keeps a Cassandra read and 10,000 PBKDF2 rounds off the hot path.
func TestCache(t *testing.T) {
	a := NewIntegrationUsers(nil, time.Second, time.Minute)

	if a.cachedHit("alice", "s3cr3t") {
		t.Error("nothing cached yet, must miss")
	}
	a.remember("alice", "s3cr3t")
	if !a.cachedHit("alice", "s3cr3t") {
		t.Error("the remembered secret must hit")
	}
	if a.cachedHit("alice", "wrong-secret") {
		t.Error("a wrong secret must miss even for a cached user")
	}
	if a.cachedHit("bob", "s3cr3t") {
		t.Error("another username must miss")
	}
}

func TestCache_Expires(t *testing.T) {
	a := NewIntegrationUsers(nil, time.Second, time.Millisecond)
	a.remember("alice", "s3cr3t")
	time.Sleep(5 * time.Millisecond)
	if a.cachedHit("alice", "s3cr3t") {
		t.Error("an expired entry must miss, so a revoked user stops working")
	}
}

func TestCache_DisabledByZeroTTL(t *testing.T) {
	a := NewIntegrationUsers(nil, time.Second, 0)
	a.remember("alice", "s3cr3t")
	if a.cachedHit("alice", "s3cr3t") {
		t.Error("a zero TTL must disable caching entirely")
	}
}
