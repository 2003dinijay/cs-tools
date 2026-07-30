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

// Command acp-closure runs one full ACP evaluation pass and exits — the
// Choreo Scheduled/Manual Task's cron owns the schedule, not this process.
package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/acp-closure-service/internal/entity"
	"github.com/wso2-open-operations/cs-tools/acp-closure-service/internal/notify"
	"github.com/wso2-open-operations/cs-tools/acp-closure-service/internal/sweep"
)

// projectUpdater is declared locally so main can hold either the real
// *entity.Client or *sweep.DryRunProjectUpdater in one variable — Go
// interface satisfaction is structural, so neither concrete type needs to
// reference this declaration.
type projectUpdater interface {
	UpdateProject(ctx context.Context, id string, body []byte) ([]byte, error)
}

func main() {
	loadDotEnv(".env")

	dryRun := envBool("DRY_RUN", true)
	testProjectID := os.Getenv("TEST_PROJECT_ID")
	runID := newRunID()

	slog.Info("acp-closure-service starting", "runID", runID, "dryRun", dryRun, "testProjectID", testProjectID)

	entityClient := entity.NewClient(entity.Config{
		BaseURL:      mustEnv("CSM_INTEGRATION_BASE_URL"),
		TokenURL:     mustEnv("CSM_INTEGRATION_TOKEN_URL"),
		ClientID:     mustEnv("CSM_INTEGRATION_CLIENT_ID"),
		ClientSecret: mustEnv("CSM_INTEGRATION_CLIENT_SECRET"),
		Scopes:       entity.RequiredScopes,
	})

	var updater projectUpdater = entityClient
	if dryRun {
		updater = &sweep.DryRunProjectUpdater{Logger: slog.Default()}
	}

	notifier := &notify.LoggingNotifier{Logger: slog.Default()}

	ctx := entity.WithCorrelationID(context.Background(), runID)

	result, err := sweep.Run(ctx, entityClient, updater, notifier, time.Now(), testProjectID)
	if err != nil {
		slog.Error("acp-closure-service sweep failed", "runID", runID, "err", err)
		os.Exit(1)
	}

	slog.Info("acp-closure-service finished",
		"runID", runID,
		"dryRun", dryRun,
		"projectsEvaluated", result.ProjectsEvaluated,
		"failureCount", len(result.Failures),
	)
	for _, f := range result.Failures {
		slog.Error("project failed", "runID", runID, "projectID", f.ProjectID, "err", f.Err)
	}
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		slog.Error("required environment variable is not set", "key", key)
		os.Exit(1)
	}
	return v
}

// envBool parses key as a bool, defaulting to def on anything except a
// successfully-parsed value — unset, empty, or malformed all fall back to
// def rather than erroring. Used for DRY_RUN specifically so any
// misconfiguration fails toward dry-run, not toward real writes.
func envBool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	parsed, err := strconv.ParseBool(v)
	if err != nil {
		slog.Warn("invalid boolean environment variable, using default", "key", key, "value", v, "default", def)
		return def
	}
	return parsed
}

// loadDotEnv reads a .env file and sets any unset environment variables from
// it. Silently ignored if the file does not exist; logs a warning for any
// other error.
func loadDotEnv(path string) {
	f, err := os.Open(path) // #nosec G304 -- path is always the hardcoded literal ".env" at the only call site
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			slog.Warn("loadDotEnv: failed to open .env file", "err", err)
		}
		return
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if len(v) >= 2 && ((v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'')) {
			v = v[1 : len(v)-1]
		}
		if os.Getenv(k) == "" {
			_ = os.Setenv(k, v)
		}
	}
	if err := scanner.Err(); err != nil {
		slog.Warn("loadDotEnv: error reading .env file", "err", err)
	}
}

// newRunID generates a UUID v4 identifying this invocation, forwarded as the
// correlation ID on every outbound call so a full run's log lines and
// upstream requests can be tied together.
func newRunID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("main: failed to read random bytes: " + err.Error())
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant bits
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
