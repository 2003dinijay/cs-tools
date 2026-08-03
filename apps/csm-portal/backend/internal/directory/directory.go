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
	"sort"
)

// Directory is the resolved, immutable reference catalogue a running process
// serves: the team registry and the assignable-role allow-list.
//
// Everything derivable from the configured rows is derived exactly once, in
// New, at startup:
//
//   - teams sorted for stable paging, with each team's backing group id already
//     converted to this platform's UUID form (Team.GroupID -> Result.GroupID),
//   - key -> team and group-name -> team lookups as maps,
//   - the group-name list a membership query needs,
//   - the role catalogue with its display names, and the role membership set.
//
// After construction a Directory is read-only, so it needs no locking and no
// upstream call: every request that only needs the mapping (POST /teams/search,
// POST /roles/search, turning a team key into a group name, turning a group name
// back into a team) is answered from these fields. Configuration changes take a
// restart, which is what deployment configuration means everywhere else here.
type Directory struct {
	teams       []Team
	teamResults []TeamResult
	byKey       map[string]Team
	byGroupName map[string]Team
	groupNames  []string

	roleResults []RoleResult
	roleSet     map[string]bool
}

// New resolves a parsed registry and role list into the in-memory catalogue.
//
// A duplicate team key or duplicate display name is an error: both would be
// silently swallowed by the lookup maps, and a shadowed row is always a typo.
// Callers are expected to treat any error here as fatal at startup -- a
// half-resolved directory would mis-route dashboards rather than fail visibly.
func New(teams []Team, roles []string) (*Directory, error) {
	d := &Directory{
		teams:       append([]Team(nil), teams...),
		teamResults: make([]TeamResult, 0, len(teams)),
		byKey:       make(map[string]Team, len(teams)),
		byGroupName: make(map[string]Team, len(teams)),
		groupNames:  make([]string, 0, len(teams)),
		roleResults: make([]RoleResult, 0, len(roles)),
		roleSet:     make(map[string]bool, len(roles)),
	}

	for _, t := range teams {
		if _, dup := d.byKey[t.Key]; dup {
			return nil, fmt.Errorf("team registry: teamKey %q is configured more than once", t.Key)
		}
		if _, dup := d.byGroupName[t.Name]; dup {
			return nil, fmt.Errorf("team registry: displayName %q is configured more than once", t.Name)
		}
		d.byKey[t.Key] = t
		d.byGroupName[t.Name] = t
		d.groupNames = append(d.groupNames, t.Name)

		result := TeamResult{ID: t.Key, Name: t.Name, Family: string(t.Family)}
		if t.GroupID != "" {
			result.GroupID = sourceIDToUUID(t.GroupID)
		}
		d.teamResults = append(d.teamResults, result)
	}
	// Configuration order is whatever the deployer typed; sort so paging is
	// stable and independent of it.
	sort.Slice(d.teamResults, func(i, j int) bool { return d.teamResults[i].Name < d.teamResults[j].Name })

	for _, r := range roles {
		if d.roleSet[r] {
			return nil, fmt.Errorf("user role list: %q is configured more than once", r)
		}
		d.roleSet[r] = true
		d.roleResults = append(d.roleResults, RoleResult{ID: r, Name: roleDisplayName(r)})
	}
	sort.Slice(d.roleResults, func(i, j int) bool { return d.roleResults[i].ID < d.roleResults[j].ID })

	return d, nil
}

// TeamCount is how many teams were resolved at startup. Logged at startup so
// "the registry loaded" is an observable fact rather than an assumption.
func (d *Directory) TeamCount() int { return len(d.teams) }

// RoleCount is how many assignable roles were resolved at startup.
func (d *Directory) RoleCount() int { return len(d.roleResults) }

// TeamByKey looks a team up by its registry key. ok is false if no configured
// team matches.
func (d *Directory) TeamByKey(key string) (Team, bool) {
	t, ok := d.byKey[key]
	return t, ok
}

// TeamByGroupName looks up the team whose group name in the backing data source
// exactly matches name. ok is false if no configured team matches.
func (d *Directory) TeamByGroupName(name string) (Team, bool) {
	t, ok := d.byGroupName[name]
	return t, ok
}

// GroupNames returns the backing data source's group display name for every
// configured team, suitable for a single group-names-IN membership query.
// Empty if no teams are configured.
func (d *Directory) GroupNames() []string {
	return append([]string(nil), d.groupNames...)
}

// IsValidRole reports whether role is in the configured allow-list. It backs
// both the role catalogue and the user-search roleIds filter, so the dropdown
// and the filter can never disagree.
func (d *Directory) IsValidRole(role string) bool { return d.roleSet[role] }

// sourceIDToUUID converts a backing data source's compact 32-character
// hexadecimal record id into this platform's canonical UUID form by inserting
// hyphens at the 8-4-4-4-12 positions. Returns the input unchanged if it is not
// exactly 32 hex characters, so a value that is already a UUID (or is simply
// malformed) passes through rather than being mangled.
func sourceIDToUUID(id string) string {
	if len(id) != 32 || !isHex(id) {
		return id
	}
	return id[0:8] + "-" + id[8:12] + "-" + id[12:16] + "-" + id[16:20] + "-" + id[20:32]
}

// isHex reports whether every byte in s is a valid hexadecimal digit.
func isHex(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}
