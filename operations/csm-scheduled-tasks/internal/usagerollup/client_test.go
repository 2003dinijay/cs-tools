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
package usagerollup

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/apierror"
)

// newTestClient starts a loopback token endpoint and a loopback entity-service
// stand-in that records what it receives.
func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"test-token","token_type":"Bearer","expires_in":3600}`))
	}))
	t.Cleanup(tokenSrv.Close)
	apiSrv := httptest.NewServer(handler)
	t.Cleanup(apiSrv.Close)

	c, err := NewClient(Config{BaseURL: apiSrv.URL, TokenURL: tokenSrv.URL, ClientID: "id", ClientSecret: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestClient_RollupDailyPostsTheDay(t *testing.T) {
	var path, auth string
	var body map[string]any
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		path, auth = r.URL.Path, r.Header.Get("Authorization")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		_, _ = w.Write([]byte(`{"rowsWritten":5,"status":"SUCCESS"}`))
	})
	written, err := c.RollupDaily(context.Background(), "2026-10-21")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if written != 5 || path != "/instances/usage-rollups/daily" || auth != "Bearer test-token" || body["date"] != "2026-10-21" {
		t.Errorf("written=%d path=%q auth=%q body=%v", written, path, auth, body)
	}
}

func TestClient_RollupMonthlyPostsYearAndMonth(t *testing.T) {
	var path string
	var body map[string]any
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		_, _ = w.Write([]byte(`{"rowsWritten":2,"status":"SUCCESS"}`))
	})
	written, err := c.RollupMonthly(context.Background(), 2026, 9)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if written != 2 || path != "/instances/usage-rollups/monthly" || body["year"] != float64(2026) || body["month"] != float64(9) {
		t.Errorf("written=%d path=%q body=%v", written, path, body)
	}
}

func TestClient_NonSuccessIsAnAPIError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"date must not be in the future"}`, http.StatusBadRequest)
	})
	_, err := c.RollupDaily(context.Background(), "2099-01-01")
	var apiErr *apierror.Error
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("error = %v, want an *apierror.Error with status 400", err)
	}
}

func TestNewClient_RefusesInsecureURLs(t *testing.T) {
	if _, err := NewClient(Config{BaseURL: "http://entity.example.com", TokenURL: "https://sts.example.com/token"}); err == nil {
		t.Error("a plain-http base URL was accepted")
	}
	if _, err := NewClient(Config{BaseURL: "https://entity.example.com", TokenURL: "http://sts.example.com/token"}); err == nil {
		t.Error("a plain-http token URL was accepted")
	}
}
