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

package site24x7

import "testing"

// With no SITE24X7_ALERT_CONFIG on the deployment, the built-in config applies.
func TestLoadConfig_BuiltInWhenEnvUnset(t *testing.T) {
	t.Setenv("SITE24X7_ALERT_CONFIG", "")
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	for field, want := range map[string]string{"Service": "Service", "Environment": "Environment", "Category": "Category"} {
		if cfg.TagList[field] != want {
			t.Errorf("TagList[%s] = %q, want %q", field, cfg.TagList[field], want)
		}
	}
	for field, want := range map[string]string{"Service": " ", "Category": "Service Interruption",
		"Environment": "Production", "Severity": "Critical", "Metric_Name": "Not Available"} {
		if cfg.Defaults[field] != want {
			t.Errorf("Defaults[%s] = %q, want %q", field, cfg.Defaults[field], want)
		}
	}
}

// A set env var still replaces the built-in config, so a deployment can override it.
func TestLoadConfig_EnvOverrides(t *testing.T) {
	t.Setenv("SITE24X7_ALERT_CONFIG", `{"TagList":{"Service":"svc"},"Defaults":{"Category":"cat"}}`)
	cfg, err := LoadConfig()
	if err != nil || cfg.TagList["Service"] != "svc" || cfg.Defaults["Category"] != "cat" || cfg.Defaults["Environment"] != "" {
		t.Errorf("cfg = %+v, err = %v; want the env config only", cfg, err)
	}
}

// LoadConfig hands out copies, so one caller can't change the defaults for the next.
func TestLoadConfig_ReturnsCopy(t *testing.T) {
	t.Setenv("SITE24X7_ALERT_CONFIG", "")
	a, _ := LoadConfig()
	a.Defaults["Environment"] = "changed"
	if b, _ := LoadConfig(); b.Defaults["Environment"] != "Production" {
		t.Error("mutating one LoadConfig result changed the built-in defaults")
	}
}

func TestTransform_WithBuiltInConfig(t *testing.T) {
	t.Setenv("SITE24X7_ALERT_CONFIG", "")
	cfg, _ := LoadConfig()

	t.Run("tags present", func(t *testing.T) {
		a, err := Transform([]byte(`{"STATUS":"DOWN","MONITORNAME":"api-check","MONITOR_ID":"42",
			"TAGS":["Service:checkout","Category:Security","Environment:Staging"]}`), cfg)
		if err != nil {
			t.Fatal(err)
		}
		if a.Service != "checkout" || a.Category != "Security" || a.Environment != "Staging" || a.Severity != "Critical" {
			t.Errorf("got %+v", a)
		}
	})

	t.Run("no tags falls back to defaults", func(t *testing.T) {
		a, err := Transform([]byte(`{"STATUS":"TROUBLE","MONITORNAME":"api-check","MONITOR_ID":"42"}`), cfg)
		if err != nil {
			t.Fatal(err)
		}
		if a.Service != " " || a.Category != "Service Interruption" || a.Environment != "Production" || a.Severity != "Minor" {
			t.Errorf("got %+v", a)
		}
	})

	t.Run("MONITORNAME present wins over the default", func(t *testing.T) {
		a, err := Transform([]byte(`{"STATUS":"DOWN","MONITORNAME":"api-check","MONITOR_ID":"42"}`), cfg)
		if err != nil || a.MetricName != "api-check" {
			t.Errorf("metric = %q, err = %v; want api-check", a.MetricName, err)
		}
	})

	t.Run("no MONITORNAME uses default metric name", func(t *testing.T) {
		a, err := Transform([]byte(`{"STATUS":"DOWN","MONITOR_ID":"42"}`), cfg)
		if err != nil || a.MetricName != "Not Available" {
			t.Errorf("metric = %q, err = %v; want Not Available", a.MetricName, err)
		}
	})

	t.Run("no STATUS uses default severity", func(t *testing.T) {
		a, err := Transform([]byte(`{"MONITORNAME":"api-check","MONITOR_ID":"42"}`), cfg)
		if err != nil || a.Severity != "Critical" {
			t.Errorf("severity = %q, err = %v; want Critical", a.Severity, err)
		}
	})
}
