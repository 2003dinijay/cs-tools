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

package entity

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-portal-activity-stream-service/internal/apierror"
)

func newCustomerTokenServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "test-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	}))
}

func TestGetCase(t *testing.T) {
	tests := []struct {
		name           string
		responseBody   []byte
		responseStatus int
		wantErr        bool
	}{
		{
			name:           "success",
			responseBody:   []byte(`{"id":"CASE-1","state":"open"}`),
			responseStatus: http.StatusOK,
			wantErr:        false,
		},
		{
			name:           "not found",
			responseBody:   []byte(`{"message":"Case not found"}`),
			responseStatus: http.StatusNotFound,
			wantErr:        true,
		},
		{
			name:           "upstream error",
			responseBody:   []byte(`{"message":"Internal server error"}`),
			responseStatus: http.StatusInternalServerError,
			wantErr:        true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create a test server that responds with the test case's response
			apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.responseStatus)
				_, _ = w.Write(tc.responseBody)
			}))
			defer apiSrv.Close()

			tokenSrv := newCustomerTokenServer(t)
			defer tokenSrv.Close()

			tokenFetchTimeout = 5 * time.Second
			t.Cleanup(func() { tokenFetchTimeout = 10 * time.Second })

			cfg := CustomerEntityConfig{
				BaseURL:      apiSrv.URL,
				TokenURL:     tokenSrv.URL,
				ClientID:     "test-client",
				ClientSecret: "test-secret",
				Scopes:       []string{"test"},
			}
			client := NewCustomerEntityClient(cfg)

			ctx := context.Background()
			body, err := client.GetCase(ctx, "CASE-1")

			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				var apiErr *apierror.Error
				if !errors.As(err, &apiErr) {
					t.Fatalf("expected *apierror.Error, got %T", err)
				}
				if apiErr.StatusCode != tc.responseStatus {
					t.Errorf("error status = %d, want %d", apiErr.StatusCode, tc.responseStatus)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				var got map[string]any
				if err := json.Unmarshal(body, &got); err != nil {
					t.Fatalf("unmarshal response: %v", err)
				}
				if got["id"] != "CASE-1" {
					t.Errorf("case ID = %v, want CASE-1", got["id"])
				}
			}
		})
	}
}

func TestCustomerEntityClient_ContextValuesForwarded(t *testing.T) {
	// Sent through a buffered channel rather than a shared variable: the
	// server handler runs on its own goroutine, and reading a plain
	// variable it wrote from the test goroutine with no synchronization is
	// a data race (go test -race flags it) even though GetCase returning
	// happens-after the handler running in practice.
	capturedHeaders := make(chan http.Header, 1)
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders <- r.Header.Clone()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"CASE-1"}`))
	}))
	defer apiSrv.Close()

	tokenSrv := newCustomerTokenServer(t)
	defer tokenSrv.Close()

	cfg := CustomerEntityConfig{
		BaseURL:      apiSrv.URL,
		TokenURL:     tokenSrv.URL,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Scopes:       []string{"test"},
	}
	client := NewCustomerEntityClient(cfg)

	ctx := context.Background()
	ctx = WithUserIDToken(ctx, "test-id-token")
	ctx = WithCorrelationID(ctx, "test-correlation-id")

	_, err := client.GetCase(ctx, "CASE-1")
	if err != nil {
		t.Fatalf("GetCase failed: %v", err)
	}

	headers := <-capturedHeaders
	if got := headers.Get("x-user-id-token"); got != "test-id-token" {
		t.Errorf("x-user-id-token = %q, want %q", got, "test-id-token")
	}
	if got := headers.Get("X-CSM-Correlation-ID"); got != "test-correlation-id" {
		t.Errorf("X-CSM-Correlation-ID = %q, want %q", got, "test-correlation-id")
	}
}
