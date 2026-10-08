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
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func testDirectory() *Directory {
	return NewDirectory(nil, slog.New(slog.NewTextHandler(io.Discard, nil)), DirectoryConfig{
		QueryTimeout: time.Second, RefreshInterval: 30 * time.Second, MaxStale: 15 * time.Minute,
	})
}

func testUser(t *testing.T, username, secret string) User {
	t.Helper()
	salt := []byte("0123456789abcdef")
	hash, err := HashSecret(secret, salt, Iterations)
	if err != nil {
		t.Fatal(err)
	}
	return User{Username: username, SecretHash: base64.StdEncoding.EncodeToString(hash),
		Salt: base64.StdEncoding.EncodeToString(salt), Iterations: Iterations, Enabled: true}
}

// TestDirectory_Lookup: found, not found, not loaded yet, and past MaxStale.
func TestDirectory_Lookup(t *testing.T) {
	d := testDirectory()
	if _, err := d.Lookup("wake"); !errors.Is(err, ErrDirectoryUnavailable) {
		t.Fatalf("before the first load = %v, want ErrDirectoryUnavailable", err)
	}
	d.store(map[string]User{"wake": {Username: "wake"}}, time.Now().Add(-14*time.Minute))
	if u, err := d.Lookup("wake"); err != nil || u.Username != "wake" {
		t.Fatalf("Lookup(wake) = %+v, %v", u, err)
	}
	if _, err := d.Lookup("nobody"); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("Lookup(nobody) = %v, want ErrUserNotFound", err)
	}
	d.store(map[string]User{"wake": {Username: "wake"}}, time.Now().Add(-16*time.Minute))
	if _, err := d.Lookup("wake"); !errors.Is(err, ErrDirectoryUnavailable) {
		t.Fatalf("past MaxStale = %v, want ErrDirectoryUnavailable", err)
	}
}

// TestRequireAuth_FromDirectory: every decision comes from the in-memory copy, a stale copy is 503, and a rotation invalidates the verified cache.
func TestRequireAuth_FromDirectory(t *testing.T) {
	d := testDirectory()
	h := RequireAuth(d, slog.New(slog.NewTextHandler(io.Discard, nil)))(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	call := func(user, secret string) int {
		r := httptest.NewRequest(http.MethodPost, "/alertz", nil)
		r.SetBasicAuth(user, secret)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		return rec.Code
	}

	if got := call("wake", "s3cr3t"); got != http.StatusServiceUnavailable {
		t.Fatalf("before the first load = %d, want 503", got)
	}

	wake := testUser(t, "wake", "s3cr3t")
	off := testUser(t, "off", "s3cr3t")
	off.Enabled = false
	d.store(map[string]User{"wake": wake, "off": off}, time.Now())
	cases := []struct {
		name, user, secret string
		want               int
	}{
		{"valid", "wake", "s3cr3t", http.StatusAccepted},
		{"valid again from cache", "wake", "s3cr3t", http.StatusAccepted},
		{"wrong secret", "wake", "nope", http.StatusUnauthorized},
		{"unknown user", "nobody", "s3cr3t", http.StatusUnauthorized},
		{"disabled user", "off", "s3cr3t", http.StatusUnauthorized},
	}
	for _, tc := range cases {
		if got := call(tc.user, tc.secret); got != tc.want {
			t.Errorf("%s: status = %d, want %d", tc.name, got, tc.want)
		}
	}

	rotated := wake
	newHash, err := HashSecret("new-secret", []byte("fedcba9876543210"), Iterations)
	if err != nil {
		t.Fatal(err)
	}
	rotated.SecretHash = base64.StdEncoding.EncodeToString(newHash)
	rotated.Salt = base64.StdEncoding.EncodeToString([]byte("fedcba9876543210"))
	d.store(map[string]User{"wake": rotated}, time.Now())
	if got := call("wake", "s3cr3t"); got != http.StatusUnauthorized {
		t.Errorf("old secret after rotation = %d, want 401 despite the cached verification", got)
	}
	if got := call("wake", "new-secret"); got != http.StatusAccepted {
		t.Errorf("new secret after rotation = %d, want 202", got)
	}

	d.store(map[string]User{"wake": rotated}, time.Now().Add(-time.Hour))
	if got := call("wake", "new-secret"); got != http.StatusServiceUnavailable {
		t.Errorf("stale copy = %d, want 503", got)
	}
}
