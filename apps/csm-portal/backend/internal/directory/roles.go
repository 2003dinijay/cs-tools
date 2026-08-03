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

package directory

import (
	"fmt"
	"strings"
)

// DefaultRoles is the assignable-role allow-list used when none is configured.
//
// Unlike the team registry, roles DO get a committed default: these names are
// generic platform vocabulary (agent, admin, customer, ...) that is already
// public in this repo, not organisation-specific vocabulary, so committing them
// leaks nothing and a zero-config deployment behaves exactly as it did when the
// list was hardcoded. Adding a role is a configuration change, not a code
// change -- deliberately not a closed compile-time set.
var DefaultRoles = []string{
	"agent",
	"admin",
	"commenter",
	"customer",
	"customer_admin",
	"partner",
	"partner_admin",
	"internal",
	"external",
	"timecard_approver",
}

// ParseRoles parses the assignable-role allow-list from its configuration form:
// a comma-separated list of role names, whitespace around each trimmed. An
// empty string yields DefaultRoles.
//
// A duplicate is an error rather than a silent de-duplication: it would double
// an entry in the role catalogue, and it is always a typo. Errors name the
// offending value so a bad deploy fails at startup, not at the first request.
func ParseRoles(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return append([]string(nil), DefaultRoles...), nil
	}

	parts := strings.Split(raw, ",")
	roles := make([]string, 0, len(parts))
	seen := make(map[string]bool, len(parts))

	for _, part := range parts {
		name := strings.TrimSpace(part)
		if name == "" {
			// Tolerate a trailing or doubled comma.
			continue
		}
		if seen[name] {
			return nil, fmt.Errorf("user role list: %q appears more than once", name)
		}
		seen[name] = true
		roles = append(roles, name)
	}

	if len(roles) == 0 {
		return nil, fmt.Errorf("user role list: no role names found in %q", raw)
	}
	return roles, nil
}

// roleDisplayName turns a role key into something readable, so callers do not
// have to hand-maintain their own label map.
func roleDisplayName(key string) string {
	words := strings.Split(key, "_")
	for i, w := range words {
		if w == "" {
			continue
		}
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}
