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

// Package opensearch transforms an inbound OpenSearch alerting monitor
// payload into the canonical alert JSON that the core component accepts. It
// is a Go port of the ServiceNow OpenSearchIncidentUtils Script Include.
package opensearch

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

// source identifies alerts produced by this adapter.
const source = "OpenSearch"

// Alert-state values OpenSearch sends in the "state" field.
const (
	stateCompleted = "COMPLETED"
	stateError     = "ERROR"
)

// Tier-3 hardcoded defaults, used when a field is absent from both the
// payload and the operator-supplied config. Keyed by canonical field name.
var defaults = map[string]string{
	"METRIC_NAME": "OpenSearchAlert",
	"SERVICE":     "managed_services",
	"CATEGORY":    "service_interruption",
	"ENVIRONMENT": "production",
	"SEVERITY":    "1",
}

// numericSeverityMap maps OpenSearch's native numeric severity to the
// canonical severity labels the core component expects.
var numericSeverityMap = map[string]string{
	"1": "Critical",
	"2": "Major",
	"3": "Minor",
	"4": "Warning",
	"5": "OK",
}

// ErrInvalidStructure is returned when the payload is not a recognisable
// OpenSearch alert (missing both monitor_id and monitor_name).
var ErrInvalidStructure = errors.New("INVALID OPENSEARCH ALERT PAYLOAD STRUCTURE")

// Alert is the canonical alert model handed to the core component.
type Alert struct {
	Service          string `json:"service"`
	MetricName       string `json:"metric_name"`
	Severity         string `json:"severity"`
	Category         string `json:"category"`
	Environment      string `json:"environment"`
	Source           string `json:"source"`
	UniqueIdentifier string `json:"unique_identifier"`
	Description      string `json:"description"`
}

// Config holds Tier-2 operator overrides (the analog of the ServiceNow
// "edge.api.opensearch.alert.config" system property). Any field left empty
// falls back to the Tier-3 default.
type Config map[string]string

// LoadConfig reads Tier-2 overrides from the OPENSEARCH_ALERT_CONFIG env var
// (a JSON object).
func LoadConfig() (Config, error) {
	raw := os.Getenv("OPENSEARCH_ALERT_CONFIG")
	if strings.TrimSpace(raw) == "" {
		return Config{}, nil
	}
	var cfg Config
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return nil, fmt.Errorf("invalid OPENSEARCH_ALERT_CONFIG: %w", err)
	}
	return cfg, nil
}

// Transform parses a raw inbound OpenSearch alert and produces the
// canonical Alert. cfg supplies Tier-2 overrides.
func Transform(raw []byte, cfg Config) (Alert, error) {
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return Alert{}, fmt.Errorf("%w: %v", ErrInvalidStructure, err)
	}

	// Validation: a genuine OpenSearch alert carries a monitor identity.
	if str(payload, "monitor_id") == "" && str(payload, "monitor_name") == "" {
		return Alert{}, ErrInvalidStructure
	}

	monitorName := str(payload, "monitor_name")
	triggerName := str(payload, "trigger_name")
	state := str(payload, "state")

	alert := Alert{
		Service:          configValue(cfg, "SERVICE", str(payload, "service")),
		MetricName:       configValue(cfg, "METRIC_NAME", buildMetricName(monitorName, triggerName)),
		Severity:         resolveSeverity(state, str(payload, "severity"), cfg),
		Category:         configValue(cfg, "CATEGORY", str(payload, "category")),
		Environment:      configValue(cfg, "ENVIRONMENT", ""), // never taken from payload
		Source:           source,
		UniqueIdentifier: str(payload, "alert_id"),
		Description:      compactJSON(raw),
	}
	return alert, nil
}

// resolveSeverity maps the alert state and raw severity to a canonical
// label. A COMPLETED run resolves to OK and an ERROR run to Critical
// regardless of the reported severity; otherwise the numeric severity is
// mapped.
func resolveSeverity(state, rawSeverity string, cfg Config) string {
	switch state {
	case stateCompleted:
		return "OK"
	case stateError:
		return "Critical"
	}
	if rawSeverity == "" {
		rawSeverity = firstNonEmpty(cfg["SEVERITY"], defaults["SEVERITY"])
	}
	if mapped, ok := numericSeverityMap[rawSeverity]; ok {
		return mapped
	}
	return rawSeverity
}

// buildMetricName joins the monitor and trigger names into a
// human-readable metric name, or returns "" when neither is present
// (letting config/defaults win).
func buildMetricName(monitorName, triggerName string) string {
	parts := make([]string, 0, 2)
	for _, p := range []string{monitorName, triggerName} {
		if p != "" {
			parts = append(parts, p)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return "OpenSearch Alert: " + strings.Join(parts, " / ")
}

// configValue applies the 3-tier resolution: payload value, then operator
// config, then the hardcoded default for the field.
func configValue(cfg Config, field, payloadValue string) string {
	return firstNonEmpty(payloadValue, cfg[field], defaults[field])
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func str(payload map[string]any, key string) string {
	v, ok := payload[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case json.Number:
		return t.String()
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", t))
	}
}

func compactJSON(raw []byte) string {
	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err != nil {
		return string(raw)
	}
	return buf.String()
}
