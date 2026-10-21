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

import "fmt"

// LicenseMode selects how ProcessLicenseDownload provisions a deployment license.
type LicenseMode string

const (
	// LicenseModeLegacy drives the subscription service's provisioning state
	// machine from this backend, step by step. It must be set explicitly.
	LicenseModeLegacy LicenseMode = "legacy"
	// LicenseModeService makes a single call to the subscription service,
	// which owns provisioning. It is the default.
	LicenseModeService LicenseMode = "service"
)

// EnvLicenseMode is the environment variable that selects the LicenseMode.
const EnvLicenseMode = "PRODUCT_CONSUMPTION_LICENSE_MODE"

// UsageImportMode selects how POST /deployment-usages treats an uploaded
// usage report. It is independent of LicenseMode.
type UsageImportMode string

const (
	// UsageImportModeLegacy forwards the upload without inspecting the report
	// or checking that the caller can see its deployment. It must be set
	// explicitly.
	UsageImportModeLegacy UsageImportMode = "legacy"
	// UsageImportModeService reads the report's deployment number, requires
	// the caller to be able to see that deployment, then forwards. It is the
	// default.
	UsageImportModeService UsageImportMode = "service"
)

// EnvUsageImportMode is the environment variable that selects the UsageImportMode.
const EnvUsageImportMode = "PRODUCT_CONSUMPTION_USAGE_IMPORT_MODE"

// ParseLicenseMode validates the raw value of EnvLicenseMode. An empty value
// selects LicenseModeService, so an environment that must keep the old flow
// has to set "legacy" explicitly; anything other than "legacy" or "service" is an error.
func ParseLicenseMode(raw string) (LicenseMode, error) {
	m, err := parseMode(EnvLicenseMode, raw)
	return LicenseMode(m), err
}

// ParseUsageImportMode validates the raw value of EnvUsageImportMode with the
// same rules as ParseLicenseMode: empty selects UsageImportModeService.
func ParseUsageImportMode(raw string) (UsageImportMode, error) {
	m, err := parseMode(EnvUsageImportMode, raw)
	return UsageImportMode(m), err
}

// parseMode is the shared "legacy" | "service" parser; empty means "service".
func parseMode(env, raw string) (string, error) {
	switch raw {
	case "", "service":
		return "service", nil
	case "legacy":
		return "legacy", nil
	}
	return "", fmt.Errorf("%s=%q is invalid: must be %q or %q (default %q)", env, raw, "legacy", "service", "service")
}
