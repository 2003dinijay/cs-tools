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

package appconfig

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateAcceptsDefault(t *testing.T) {
	cfg := Default()
	if err := Validate(&cfg); err != nil {
		t.Errorf("expected Default() to be valid, got: %v", err)
	}
}

func TestValidateAggregatesMultipleIssues(t *testing.T) {
	cfg := Default()
	cfg.Server.Port = 0
	cfg.Jobs.RecomputePageSize = 0
	cfg.API.IssuesDefaultLimit = 1000 // exceeds IssuesMaxLimit

	err := Validate(&cfg)
	if err == nil {
		t.Fatal("expected an error")
	}
	var verr *ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected *ValidationError, got %T", err)
	}
	if len(verr.Issues) < 3 {
		t.Errorf("expected at least 3 aggregated issues, got %d: %v", len(verr.Issues), verr.Issues)
	}
}

func TestValidateDatabasePointersOnlyCheckedWhenSet(t *testing.T) {
	cfg := Default()
	if err := Validate(&cfg); err != nil {
		t.Errorf("expected nil database pointers to be valid, got: %v", err)
	}

	zero := int32(0)
	cfg.Database.MaxConns = &zero
	if err := Validate(&cfg); err == nil {
		t.Error("expected maxConns=0 to be rejected")
	} else if !strings.Contains(err.Error(), "database.maxConns") {
		t.Errorf("expected error naming database.maxConns, got: %v", err)
	}
}

func TestValidateMinConnsExceedingMaxConnsRejected(t *testing.T) {
	cfg := Default()
	minConns, maxConns := int32(5), int32(2)
	cfg.Database.MinConns = &minConns
	cfg.Database.MaxConns = &maxConns

	err := Validate(&cfg)
	if err == nil || !strings.Contains(err.Error(), "database.minConns") {
		t.Errorf("expected an error naming database.minConns, got: %v", err)
	}
}

func TestValidateGithubMaxRetriesZeroIsLegal(t *testing.T) {
	cfg := Default()
	cfg.GitHub.MaxRetries = 0
	if err := Validate(&cfg); err != nil {
		t.Errorf("expected github.maxRetries=0 to be valid, got: %v", err)
	}
	cfg.GitHub.MaxRetries = -1
	if err := Validate(&cfg); err == nil {
		t.Error("expected github.maxRetries=-1 to be rejected")
	}
}
