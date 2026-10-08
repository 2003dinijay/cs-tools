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
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"
)

// discard keeps Audit's log lines out of the test output.
func discard() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestNew_None(t *testing.T) {
	a, err := New(ModeNone, nil, nil, nil, discard())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := a.Authenticate(httptest.NewRequest("POST", "/", nil), "aws"); err != nil {
		t.Errorf("none should accept every request, got %v", err)
	}
}

func TestNew_UnknownModeFails(t *testing.T) {
	if _, err := New("basic", nil, nil, nil, discard()); err == nil {
		t.Error("unknown mode should fail at startup")
	}
}

func TestNew_APIKeyRequiresEveryVendor(t *testing.T) {
	_, err := New(ModeAPIKey, map[string]string{"aws": "k"}, []string{"aws", "datadog"}, nil, discard())
	if err == nil {
		t.Fatal("apikey mode should refuse to start with datadog unprotected")
	}
}

func TestNew_AuditAllowsPartialConfig(t *testing.T) {
	a, err := New(ModeAudit, map[string]string{"aws": "k"}, []string{"aws", "datadog"}, nil, discard())
	if err != nil {
		t.Fatalf("audit mode should start with a partial config: %v", err)
	}
	// The whole point of audit: the unconfigured vendor still gets through.
	if err := a.Authenticate(httptest.NewRequest("POST", "/", nil), "datadog"); err != nil {
		t.Errorf("audit must not reject, got %v", err)
	}
}

func TestNew_RejectsKeyForUnknownVendor(t *testing.T) {
	for _, mode := range []string{ModeAudit, ModeAPIKey} {
		if _, err := New(mode, map[string]string{"awz": "k"}, []string{"aws"}, nil, discard()); err == nil {
			t.Errorf("%s: a typo'd vendor name should fail at startup", mode)
		}
	}
}
