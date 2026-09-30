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
	"net/http"
	"net/http/httptest"
	"testing"
)

const wakeTestKey = "wake-secret-value"

// discard is defined in auth_test.go.

// reached records whether the wrapped handler ran.
func wakeGuarded(key string) (http.Handler, *bool) {
	ran := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		ran = true
		w.WriteHeader(http.StatusAccepted)
	})
	return RequireKey(key, discard(), next), &ran
}

func TestRequireKey_Accepts(t *testing.T) {
	cases := map[string]func(*http.Request){
		"bearer raw key": func(r *http.Request) {
			r.Header.Set("Authorization", "Bearer "+wakeTestKey)
		},
		"bearer base64 pair": func(r *http.Request) {
			r.Header.Set("Authorization", "Bearer "+base64.StdEncoding.EncodeToString([]byte("ingestion:"+wakeTestKey)))
		},
		"lowercase scheme": func(r *http.Request) {
			r.Header.Set("Authorization", "bearer "+wakeTestKey)
		},
		"api key header": func(r *http.Request) {
			r.Header.Set(WakeHeaderName, wakeTestKey)
		},
	}
	for name, set := range cases {
		t.Run(name, func(t *testing.T) {
			h, ran := wakeGuarded(wakeTestKey)
			r := httptest.NewRequest("POST", "/alertz", nil)
			set(r)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, r)
			if rec.Code != http.StatusAccepted || !*ran {
				t.Errorf("code = %d, handler ran = %v", rec.Code, *ran)
			}
		})
	}
}

func TestRequireKey_Rejects(t *testing.T) {
	cases := map[string]func(*http.Request){
		"no credential": func(*http.Request) {},
		"wrong key": func(r *http.Request) {
			r.Header.Set("Authorization", "Bearer nope")
		},
		"prefix of the key": func(r *http.Request) {
			r.Header.Set("Authorization", "Bearer "+wakeTestKey[:5])
		},
		"basic scheme is not accepted": func(r *http.Request) {
			r.SetBasicAuth("user", wakeTestKey)
		},
	}
	for name, set := range cases {
		t.Run(name, func(t *testing.T) {
			h, ran := wakeGuarded(wakeTestKey)
			r := httptest.NewRequest("POST", "/alertz", nil)
			set(r)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, r)
			if rec.Code != http.StatusUnauthorized || *ran {
				t.Errorf("code = %d, handler ran = %v", rec.Code, *ran)
			}
		})
	}
}

// An empty key is the local-development escape hatch; main.go logs a warning.
func TestRequireKey_EmptyKeyDisablesCheck(t *testing.T) {
	h, ran := wakeGuarded("")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("POST", "/alertz", nil))
	if rec.Code != http.StatusAccepted || !*ran {
		t.Errorf("code = %d, handler ran = %v", rec.Code, *ran)
	}
}

// The body is never read, so a rejected wake costs nothing beyond the headers.
func TestRequireKey_LeaksNoReason(t *testing.T) {
	h, _ := wakeGuarded(wakeTestKey)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("POST", "/alertz", nil))
	if body := rec.Body.String(); body != "unauthorized\n" {
		t.Errorf("body = %q, want a bare \"unauthorized\"", body)
	}
}
