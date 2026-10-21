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
	// machine from this backend, step by step. It is the default.
	LicenseModeLegacy LicenseMode = "legacy"
	// LicenseModeService makes a single call to the subscription service,
	// which owns provisioning.
	LicenseModeService LicenseMode = "service"
)

// EnvLicenseMode is the environment variable that selects the LicenseMode.
const EnvLicenseMode = "PRODUCT_CONSUMPTION_LICENSE_MODE"

// ParseLicenseMode validates the raw value of EnvLicenseMode. An empty value
// selects LicenseModeLegacy so an unset environment keeps the existing
// behaviour; anything other than "legacy" or "service" is an error.
func ParseLicenseMode(raw string) (LicenseMode, error) {
	switch LicenseMode(raw) {
	case "", LicenseModeLegacy:
		return LicenseModeLegacy, nil
	case LicenseModeService:
		return LicenseModeService, nil
	}
	return "", fmt.Errorf("%s=%q is invalid: must be %q or %q (default %q)",
		EnvLicenseMode, raw, LicenseModeLegacy, LicenseModeService, LicenseModeLegacy)
}
