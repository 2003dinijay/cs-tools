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

package config

import (
	"slices"
	"strings"
	"testing"
	"time"
)

func env(values map[string]string) func(string) string {
	return func(k string) string { return values[k] }
}

func minimal() map[string]string {
	return map[string]string{
		"DB_HOST":            "db.internal",
		"DB_USER":            "til",
		"DB_PASSWORD":        "secret",
		"DB_NAME":            "til",
		"AUTH_JWKS_ENDPOINT": "https://idp.example.com/oauth2/jwks",
		"AUTH_ISSUER":        "https://idp.example.com/oauth2/token",
	}
}

func TestLoadDefaults(t *testing.T) {
	cfg, warnings, err := Load(env(minimal()))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != "8077" || cfg.DB.Port != "3306" || cfg.ReadTimeout != 15*time.Second || cfg.WriteTimeout != 45*time.Second {
		t.Fatalf("defaults = %+v", cfg)
	}
	if cfg.Entity != nil || cfg.Novera != nil || len(cfg.CORSOrigins) != 0 {
		t.Fatalf("optional features should be off: %+v", cfg)
	}
	if len(warnings) != 4 {
		t.Fatalf("warnings = %q", warnings)
	}
}

func TestLoadReadsEverything(t *testing.T) {
	v := minimal()
	for k, val := range map[string]string{
		"PORT":                         "9000",
		"REST_READ_TIMEOUT":            "5s",
		"REST_WRITE_TIMEOUT":           "1m",
		"CORS_ALLOWED_ORIGINS":         "https://one.example, https://two.example ,",
		"AUTH_AUDIENCE":                "webapp",
		"AUTH_MODERATOR_GROUPS":        "til-moderators,admins",
		"ONE_WSO2_WEBAPP_CLIENT_ID":    "webapp",
		"ONE_WSO2_BASE_URL":            "https://one.example/",
		"ENTITY_SERVICE_BASE_URL":      "https://entity.example",
		"ENTITY_SERVICE_TOKEN_URL":     "https://idp.example.com/oauth2/token",
		"ENTITY_SERVICE_CLIENT_ID":     "til",
		"ENTITY_SERVICE_CLIENT_SECRET": "s",
		"ENTITY_SERVICE_SCOPES":        "accounts:read",
		"NOVERA_NOTIFY_URL":            "https://gateway.example/novera/notify",
		"NOVERA_TOKEN_URL":             "https://gateway.example/token",
		"NOVERA_CLIENT_ID":             "til",
		"NOVERA_CLIENT_SECRET":         "s",
	} {
		v[k] = val
	}
	cfg, warnings, err := Load(env(v))
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %q", warnings)
	}
	if cfg.Port != "9000" || cfg.ReadTimeout != 5*time.Second || cfg.WriteTimeout != time.Minute {
		t.Fatalf("server = %+v", cfg)
	}
	if !slices.Equal(cfg.CORSOrigins, []string{"https://one.example", "https://two.example"}) ||
		!slices.Equal(cfg.Auth.ModeratorGroups, []string{"til-moderators", "admins"}) ||
		cfg.WebappBaseURL != "https://one.example" {
		t.Fatalf("lists = %+v", cfg)
	}
	if cfg.Entity == nil || cfg.Entity.Scopes[0] != "accounts:read" || cfg.Novera == nil || cfg.Novera.NotifyURL != v["NOVERA_NOTIFY_URL"] {
		t.Fatalf("clients = %+v %+v", cfg.Entity, cfg.Novera)
	}
}

func TestLoadReportsEveryProblem(t *testing.T) {
	_, _, err := Load(env(map[string]string{
		"PORT":                    "http",
		"REST_READ_TIMEOUT":       "soon",
		"AUTH_JWKS_ENDPOINT":      "idp.example.com/jwks",
		"ONE_WSO2_BASE_URL":       "one.example",
		"ENTITY_SERVICE_BASE_URL": "https://entity.example", // the rest of its settings missing
		"NOVERA_NOTIFY_URL":       "https://gateway.example/notify",
	}))
	if err == nil {
		t.Fatal("want an error")
	}
	for _, want := range []string{
		"PORT must be a port number",
		"REST_READ_TIMEOUT must be a positive duration",
		"DB_HOST is required", "DB_USER is required", "DB_PASSWORD is required", "DB_NAME is required",
		"AUTH_JWKS_ENDPOINT must be an absolute http(s) URL",
		"AUTH_ISSUER is required",
		"ONE_WSO2_BASE_URL must be an absolute http(s) URL",
		"ENTITY_SERVICE_TOKEN_URL is required", "ENTITY_SERVICE_CLIENT_SECRET is required",
		"NOVERA_TOKEN_URL is required", "NOVERA_CLIENT_ID is required",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error lacks %q:\n%v", want, err)
		}
	}
}
