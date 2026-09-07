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

import "testing"

// TestIsValidDateOnly_MatchesTheUpstreamConstraint pins this check against the
// upstream DateString pattern it copies. Accepting anything the upstream rejects
// would leak a Choreo schema error to the customer; rejecting anything it accepts
// would break a working filter, so the two must agree exactly.
func TestIsValidDateOnly_MatchesTheUpstreamConstraint(t *testing.T) {
	valid := []string{
		"2026-06-16",
		"2026-01-01",
		"2026-12-31",
		"1999-02-28",
		"2026-02-30", // pattern-valid; the upstream owns real calendar validation
	}
	for _, s := range valid {
		if !IsValidDateOnly(s) {
			t.Errorf("IsValidDateOnly(%q) = false, want true — the upstream accepts this", s)
		}
	}

	invalid := map[string]string{
		"unpadded month":    "2026-6-16",
		"unpadded day":      "2026-06-6",
		"month 00":          "2026-00-16",
		"month 13":          "2026-13-16",
		"day 00":            "2026-06-00",
		"day 32":            "2026-06-32",
		"RFC3339":           "2026-06-16T00:00:00Z",
		"with time":         "2026-06-16 00:00:00",
		"slashes":           "2026/06/16",
		"year and month":    "2026-06",
		"trailing space":    "2026-06-16 ",
		"leading space":     " 2026-06-16",
		"two-digit year":    "26-06-16",
		"empty":             "",
		"not a date at all": "yyyy-mm-dd",
	}
	for name, s := range invalid {
		if IsValidDateOnly(s) {
			t.Errorf("%s: IsValidDateOnly(%q) = true, want false — the upstream rejects this", name, s)
		}
	}
}
