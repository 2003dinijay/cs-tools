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

package productconsumption

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestParseLicenseMode(t *testing.T) {
	tests := []struct {
		in      string
		want    LicenseMode
		wantErr bool
	}{
		{"", LicenseModeService, false},
		{"legacy", LicenseModeLegacy, false},
		{"service", LicenseModeService, false},
		{"Service", "", true},
		{"single", "", true},
		{" legacy", "", true},
	}
	for _, tt := range tests {
		got, err := ParseLicenseMode(tt.in)
		if (err != nil) != tt.wantErr || got != tt.want {
			t.Errorf("ParseLicenseMode(%q) = %q, %v; want %q, err=%v", tt.in, got, err, tt.want, tt.wantErr)
		}
	}
}

// dispatchPaths runs ProcessLicenseDownload against a server that records
// request paths. The server answers status 5 so the legacy flow ends at the
// license call.
func dispatchPaths(t *testing.T, mode LicenseMode) []string {
	t.Helper()
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"t","token_type":"Bearer","expires_in":3600}`))
	}))
	t.Cleanup(tokenSrv.Close)

	var mu sync.Mutex
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.URL.Path)
		mu.Unlock()
		if strings.HasSuffix(r.URL.Path, "/consumption/status") {
			_, _ = w.Write([]byte(`{"result":{"status":5.0,"applicationId":"app-1"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"result":{"success":true,"license":{"subscriptionData":{},"signature":"sig"}}}`))
	}))
	t.Cleanup(srv.Close)

	c := NewClient(Config{SubscriptionBaseURL: srv.URL, TokenURL: tokenSrv.URL, ClientID: "id", ClientSecret: "s", LicenseMode: mode})
	if _, err := c.ProcessLicenseDownload(context.Background(), LicenseDownloadRequest{
		Email: "someone@example.com", ProjectID: "6fa0b42d-1bfa-a694-a002-c9d3604bcb77",
		DeploymentID: "937bd77b-1ba0-8750-a002-c9d3604bcbbc",
	}); err != nil {
		t.Fatalf("ProcessLicenseDownload(%q): %v", mode, err)
	}
	return paths
}

func hasStatusCall(paths []string) bool {
	for _, p := range paths {
		if strings.HasSuffix(p, "/consumption/status") {
			return true
		}
	}
	return false
}

// TestProcessLicenseDownload_Dispatch pins the mode selection: an unset mode
// selects service (a single license call); legacy must be set explicitly and
// starts with the consumption-status call.
func TestProcessLicenseDownload_Dispatch(t *testing.T) {
	unset, err := ParseLicenseMode("")
	if err != nil {
		t.Fatal(err)
	}
	if paths := dispatchPaths(t, unset); hasStatusCall(paths) || len(paths) != 1 {
		t.Errorf("unset mode: want exactly one license call, got %v", paths)
	}
	if paths := dispatchPaths(t, ""); hasStatusCall(paths) || len(paths) != 1 {
		t.Errorf("zero-value mode must behave as service, got %v", paths)
	}
	if paths := dispatchPaths(t, LicenseModeService); hasStatusCall(paths) || len(paths) != 1 {
		t.Errorf("service mode: want exactly one license call, got %v", paths)
	}
	if paths := dispatchPaths(t, LicenseModeLegacy); !hasStatusCall(paths) || len(paths) != 2 {
		t.Errorf("explicit legacy mode: want status call then license call, got %v", paths)
	}
}

func TestParseUsageImportMode(t *testing.T) {
	tests := []struct {
		in      string
		want    UsageImportMode
		wantErr bool
	}{
		{"", UsageImportModeService, false},
		{"service", UsageImportModeService, false},
		{"legacy", UsageImportModeLegacy, false},
		{"Legacy", "", true},
		{"both", "", true},
	}
	for _, tt := range tests {
		got, err := ParseUsageImportMode(tt.in)
		if (err != nil) != tt.wantErr || got != tt.want {
			t.Errorf("ParseUsageImportMode(%q) = %q, %v; want %q, err=%v", tt.in, got, err, tt.want, tt.wantErr)
		}
	}
	if _, err := ParseUsageImportMode("bad"); err == nil || !strings.Contains(err.Error(), EnvUsageImportMode) {
		t.Errorf("error should name %s, got %v", EnvUsageImportMode, err)
	}
}
