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

package dto

import "regexp"

// dateOnlyRe mirrors the upstream DateString constraint verbatim:
//
//	@constraint:String {
//	    pattern: re `^\d{4}-(0[1-9]|1[0-2])-(0[1-9]|[12]\d|3[01])$`
//	}
//
// Copied rather than loosened on purpose — anything the upstream accepts must be
// accepted here, and nothing more. Note it requires zero-padded month and day,
// so "2026-6-16" is rejected while "2026-06-16" passes.
var dateOnlyRe = regexp.MustCompile(`^\d{4}-(0[1-9]|1[0-2])-(0[1-9]|[12]\d|3[01])$`)

// IsValidDateOnly reports whether s is a plain YYYY-MM-DD date the upstream will
// accept.
//
// Validating at the portal boundary matters because a malformed date otherwise
// travels all the way to the Choreo integration service, which rejects it with
// its own schema language:
//
//	Failed to bind request payload to the expected schema: payload validation
//	failed: Validation failed for '$.filters.startDate:pattern' constraint(s).
//
// That reaches the customer verbatim and names neither the offending value nor
// the expected format. Rejecting it here produces an actionable message and
// saves a pointless upstream round trip.
func IsValidDateOnly(s string) bool {
	return dateOnlyRe.MatchString(s)
}
