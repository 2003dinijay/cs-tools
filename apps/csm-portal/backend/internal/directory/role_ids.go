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
	"encoding/json"
	"fmt"
)

// ParseRoleIDs parses a real-role-name -> identity-provider role ID mapping
// from its configuration form: a JSON object string, e.g.
//
//	{"example-timecard-approver-role": "11111111-1111-1111-1111-111111111111"}
//
// The key is the role name exactly as it appears on the token's "roles"
// claim -- the same strings AUTH_<ROLE>_ROLES already lists -- not an
// invented portal-role key. Keying by the real name (rather than, say,
// "timecard_approver") is deliberate: AUTH_<ROLE>_ROLES can list more than
// one real role name granting the same portal permission, and only a real
// name identifies a specific role an ID can actually belong to.
// handler.ResolveGrantableRoles is what cross-references this map against
// AccessConfig to answer "which portal role does this real name grant," for
// a caller (e.g. GET /roles/grantable) that needs the portal-role vocabulary
// instead.
//
// A JSON object was chosen over the flat "key|id,key|id" form used
// elsewhere in this package (ParseTeamRegistry et al.) specifically because
// the key here is already an opaque, deployment-specific role name that may
// itself contain punctuation (e.g. "sn_customerservice.x") -- delimiter-splitting
// that reliably would need its own escaping rules, which plain JSON already
// provides for free.
//
// There is no default and no requirement that every AUTH_<ROLE>_ROLES name
// be present: which real roles have a known ID at all is deployment- and
// feature-specific, so an unconfigured name simply has no entry (callers
// treat that as "this role has no SCIM lookup configured").
func ParseRoleIDs(raw string) (map[string]string, error) {
	if raw == "" {
		return map[string]string{}, nil
	}

	var ids map[string]string
	if err := json.Unmarshal([]byte(raw), &ids); err != nil {
		return nil, fmt.Errorf("role id mapping: invalid JSON: %w", err)
	}
	// json.Unmarshal of the literal "null" succeeds with ids == nil and no
	// error -- indistinguishable from "{}" to the zero-value check below, so
	// it must be rejected explicitly. Left unchecked, ASGARDEO_ROLE_IDS=null
	// would silently behave as "no roles configured" instead of failing
	// startup the way any other malformed value does.
	if ids == nil {
		return nil, fmt.Errorf("role id mapping: configuration must not be JSON null")
	}

	for name, id := range ids {
		if name == "" {
			return nil, fmt.Errorf("role id mapping: a role name key is empty")
		}
		if id == "" {
			return nil, fmt.Errorf("role id mapping: role %q has an empty id", name)
		}
	}
	return ids, nil
}

// ParseRoleGroupNames parses a portal-role-key -> Asgardeo-group-display-
// name(s) mapping from its configuration form: a JSON object string, e.g.
//
//	{"cs_engineer": ["CSM Engineers"]}
//
// Unlike ParseRoleIDs, the key here is the portal role's own stable key --
// the same strings handler.NewAccessGuard assigns ("viewer", "cs_engineer",
// "admin", ...) -- not a real Asgardeo role name: a group has no equivalent
// of a role's display name to cross-reference against AUTH_<ROLE>_ROLES, so
// the deployment names the portal role being granted directly. The value is
// the group's own display name (the one visible in the Asgardeo console),
// not its opaque id -- matched case-sensitively against SCIM's "display"
// field for each group a user belongs to, the same exact-match convention
// AUTH_<ROLE>_ROLES itself already uses for role names. It is an array, not
// a single string, because more than one group can grant the same portal
// role.
//
// There is no default and no requirement that every portal role be present:
// a portal role with no entry here simply has no group that grants it --
// the same "no entry means not wired up here" posture ParseRoleIDs
// documents for a real role name.
func ParseRoleGroupNames(raw string) (map[string][]string, error) {
	if raw == "" {
		return map[string][]string{}, nil
	}

	var names map[string][]string
	if err := json.Unmarshal([]byte(raw), &names); err != nil {
		return nil, fmt.Errorf("role group name mapping: invalid JSON: %w", err)
	}
	// Same "null" trap ParseRoleIDs guards against -- json.Unmarshal of the
	// literal "null" succeeds with names == nil and no error, indistinguishable
	// from "{}" to the zero-value check below.
	if names == nil {
		return nil, fmt.Errorf("role group name mapping: configuration must not be JSON null")
	}

	for key, groupNames := range names {
		if key == "" {
			return nil, fmt.Errorf("role group name mapping: a portal role key is empty")
		}
		if len(groupNames) == 0 {
			return nil, fmt.Errorf("role group name mapping: role %q has no group names", key)
		}
		for _, name := range groupNames {
			if name == "" {
				return nil, fmt.Errorf("role group name mapping: role %q has an empty group name", key)
			}
		}
	}
	return names, nil
}
