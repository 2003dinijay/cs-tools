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

// Package icinga transforms an inbound Icinga host/service notification
// payload into the canonical alert JSON that the core component accepts.
// It is a Go port of the ServiceNow IcingaAlertProcessor Script Include.
//
// Icinga sends one notification per webhook call. The reference script's
// response envelope wraps its single result in a "processedPayloads"
// array for shape-consistency with the batch-style vendors, but
// process() itself validates and handles exactly one notification with no
// per-item error isolation -- a storage failure propagates uncaught, the
// same as every other single-alert vendor -- so Transform returns a
// single Alert, not a slice.
package icinga

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"sre-alert-edge-service/internal/vendors/jsonnum"
)

// source identifies alerts produced by this adapter.
const source = "Icinga"

// Tier-3 hardcoded defaults, used when a field is absent from both the
// payload and the operator-supplied config. Keyed by canonical field
// name. SEVERITY is deliberately absent: the reference script defines
// DEFAULTS.SEVERITY but never actually applies it anywhere -- severity
// comes solely from the state maps below, or the raw state value itself.
var defaults = map[string]string{
	"METRIC_NAME": "IcingaAlert",
	"SERVICE":     "managed_services",
	"CATEGORY":    "service_interruption",
	"ENVIRONMENT": "development",
}

// serviceStateMap maps an Icinga service check's state to the canonical
// severity labels the core component expects.
var serviceStateMap = map[string]string{
	"critical": "Critical",
	"warning":  "Warning",
	"unknown":  "Major",
	"ok":       "OK",
}

// hostStateMap maps an Icinga host check's state to the canonical
// severity labels the core component expects.
var hostStateMap = map[string]string{
	"down":        "Critical",
	"unreachable": "Major",
	"up":          "OK",
}

// ErrInvalidStructure is returned when the payload is missing either
// notification_type or host_name.
var ErrInvalidStructure = errors.New("INVALID ICINGA ALERT PAYLOAD STRUCTURE")

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
// "edge.api.icinga.alert.config" system property). Any field left empty
// falls back to the Tier-3 default.
type Config map[string]string

// LoadConfig reads Tier-2 overrides from the ICINGA_ALERT_CONFIG env var
// (a JSON object).
func LoadConfig() (Config, error) {
	raw := os.Getenv("ICINGA_ALERT_CONFIG")
	if strings.TrimSpace(raw) == "" {
		return Config{}, nil
	}
	var cfg Config
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return nil, fmt.Errorf("invalid ICINGA_ALERT_CONFIG: %w", err)
	}
	return cfg, nil
}

// Transform parses a raw inbound Icinga notification and produces the
// canonical Alert. cfg supplies Tier-2 overrides.
func Transform(raw []byte, cfg Config) (Alert, error) {
	var payload map[string]any
	if err := jsonnum.Unmarshal(raw, &payload); err != nil {
		return Alert{}, fmt.Errorf("%w: %v", ErrInvalidStructure, err)
	}

	// Validation: a genuine Icinga notification carries its type and the
	// host it's about.
	notificationType := str(payload, "notification_type")
	hostName := str(payload, "host_name")
	if notificationType == "" || hostName == "" {
		return Alert{}, ErrInvalidStructure
	}
	notificationType = strings.ToUpper(notificationType)

	// Icinga sends either a host check (no service_name -- the host
	// itself is the check target) or a service check (service_name
	// present).
	hostDisplayName := firstNonEmpty(str(payload, "host_display_name"), hostName)
	hostState := str(payload, "host_state")
	serviceName := str(payload, "service_name")
	serviceState := str(payload, "service_state")

	// Custom vars, passed from Icinga host/service vars.
	vars, _ := payload["vars"].(map[string]any)

	var severity string
	if notificationType == "RECOVERY" {
		severity = "OK"
	} else {
		severity = mapSeverity(serviceState, hostState, serviceName)
	}

	// Icinga uses the host!service pattern as the natural unique key. For
	// host checks (no service), unique_identifier is just the host name.
	uniqueIdentifier := hostName
	if serviceName != "" {
		uniqueIdentifier = hostName + "!" + serviceName
	}

	alert := Alert{
		Service:          configValue(cfg, "SERVICE", firstNonEmpty(str(vars, "service"), str(payload, "service"))),
		MetricName:       configValue(cfg, "METRIC_NAME", buildMetricName(hostDisplayName, serviceName)),
		Severity:         severity,
		Category:         configValue(cfg, "CATEGORY", firstNonEmpty(str(vars, "category"), str(payload, "category"))),
		Environment:      configValue(cfg, "ENVIRONMENT", firstNonEmpty(str(vars, "environment"), str(payload, "environment"))),
		Source:           source,
		UniqueIdentifier: uniqueIdentifier,
		Description:      compactJSON(raw),
	}
	return alert, nil
}

// mapSeverity routes to serviceStateMap or hostStateMap depending on
// whether this is a service check or a host check, falling back to the
// raw (unmapped) state value rather than an empty string when the state
// isn't recognized.
func mapSeverity(serviceState, hostState, serviceName string) string {
	if serviceName != "" {
		return firstNonEmpty(serviceStateMap[strings.ToLower(serviceState)], serviceState)
	}
	return firstNonEmpty(hostStateMap[strings.ToLower(hostState)], hostState)
}

// buildMetricName builds "Icinga Alert: <host> / <service>" for a service
// check, or "Icinga Alert: <host>" for a host check, or "" when both are
// absent (letting config/defaults win).
func buildMetricName(hostDisplayName, serviceName string) string {
	parts := make([]string, 0, 2)
	for _, p := range []string{hostDisplayName, serviceName} {
		if p != "" {
			parts = append(parts, p)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return "Icinga Alert: " + strings.Join(parts, " / ")
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
	if payload == nil {
		return ""
	}
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
