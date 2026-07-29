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

package domain

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
)

// AbtFamily classifies an ABT (Account-Based Team) as either a customer
// renewal/expansion team (CRE) or a site reliability team (SRE). Not every
// team has a family assigned -- it is empty for the unclassified teams.
type AbtFamily string

const (
	// AbtFamilyCRE identifies a Customer Renewal & Expansion team.
	AbtFamilyCRE AbtFamily = "cre"
	// AbtFamilySRE identifies a Site Reliability Engineering team.
	AbtFamilySRE AbtFamily = "sre"
)

// AbtTeam is one of WSO2's Account-Based Teams. The registry of teams is
// owned by a private-repo Ballerina endpoint and fetched live at runtime —
// team names are never hardcoded in this public repo. The registry is a flat
// list; there is no sub-team nesting.
type AbtTeam struct {
	TeamKey     string
	DisplayName string // the exact ServiceNow group name; matched against group-members/search's groupName
	Family      AbtFamily // may be empty -- not every team has a family assigned
}

// AbtTeamsFetcher fetches the raw ABT team registry JSON body from the
// upstream Ballerina cs-entity-service (GET abt-teams). It is registered via
// SetAbtTeamsFetcher during service wiring, before the first call to
// AbtGroupNames or FindAbtTeamByGroupName.
type AbtTeamsFetcher func(ctx context.Context) (json.RawMessage, error)

var (
	abtMu      sync.Mutex
	abtFetcher AbtTeamsFetcher
	abtLoaded  bool
	abtTeams   []AbtTeam
)

// SetAbtTeamsFetcher registers the function used to populate the in-memory
// ABT team registry on first use. Calling this again (e.g. in tests, or if
// service wiring re-runs) invalidates any cached registry so the next lookup
// re-fetches via the new fetcher.
func SetAbtTeamsFetcher(fetch AbtTeamsFetcher) {
	abtMu.Lock()
	defer abtMu.Unlock()
	abtFetcher = fetch
	abtLoaded = false
	abtTeams = nil
}

// AbtGroupNames returns the ServiceNow group display name for every cached
// ABT team, suitable for a single groupNames-IN membership query. Returns an
// empty slice if the registry has never been populated or failed to load.
func AbtGroupNames() []string {
	ensureAbtRegistryLoaded()

	abtMu.Lock()
	defer abtMu.Unlock()

	names := make([]string, 0, len(abtTeams))
	for _, team := range abtTeams {
		if team.DisplayName != "" {
			names = append(names, team.DisplayName)
		}
	}
	return names
}

// FindAbtTeamByGroupName looks up the ABT team whose ServiceNow group name
// exactly matches groupName. ok is false if no team in the registry matches.
func FindAbtTeamByGroupName(groupName string) (team AbtTeam, ok bool) {
	ensureAbtRegistryLoaded()

	abtMu.Lock()
	defer abtMu.Unlock()

	for _, t := range abtTeams {
		if t.DisplayName == groupName {
			return t, true
		}
	}
	return AbtTeam{}, false
}

// ensureAbtRegistryLoaded populates abtTeams on first use. Any failure (no
// fetcher registered, fetch error, or parse error) is logged and degrades to
// an empty registry rather than retrying on every subsequent call.
func ensureAbtRegistryLoaded() {
	abtMu.Lock()
	fetch := abtFetcher
	alreadyLoaded := abtLoaded
	abtMu.Unlock()

	if alreadyLoaded {
		return
	}

	var teams []AbtTeam
	if fetch == nil {
		log.Printf("abtteam: no fetcher registered; ABT team registry will be empty")
	} else {
		raw, err := fetch(context.Background())
		if err != nil {
			log.Printf("abtteam: fetch abt-teams registry failed: %v", err)
		} else {
			teams, err = parseAbtTeamsResponse(raw)
			if err != nil {
				log.Printf("abtteam: parse abt-teams registry failed: %v", err)
				teams = nil
			}
		}
	}

	abtMu.Lock()
	defer abtMu.Unlock()
	// Another goroutine may have loaded (or reset, via SetAbtTeamsFetcher)
	// concurrently; only commit if still unloaded for this fetcher generation.
	if !abtLoaded {
		abtTeams = teams
		abtLoaded = true
	}
}

// wireAbtTeamsResponse mirrors the Ballerina GET abt-teams response.
type wireAbtTeamsResponse struct {
	Teams []wireAbtTeam `json:"teams"`
}

type wireAbtTeam struct {
	TeamKey     string `json:"teamKey"`
	DisplayName string `json:"displayName"`
	Family      string `json:"family"` // "CRE" | "SRE" | absent
}

// parseAbtTeamsResponse parses the wire response and normalizes the
// uppercase wire "family" ("CRE" | "SRE") into this package's lowercase
// AbtFamily constants. Teams with no family (absent/unrecognized wire value)
// get an empty AbtFamily rather than a parse error -- not every team is
// classified.
func parseAbtTeamsResponse(raw json.RawMessage) ([]AbtTeam, error) {
	var wire wireAbtTeamsResponse
	if err := json.Unmarshal(raw, &wire); err != nil {
		return nil, fmt.Errorf("abtteam: parse abt-teams response: %w", err)
	}

	teams := make([]AbtTeam, 0, len(wire.Teams))
	for _, wt := range wire.Teams {
		teams = append(teams, AbtTeam{
			TeamKey:     wt.TeamKey,
			DisplayName: wt.DisplayName,
			Family:      normalizeAbtFamily(wt.Family),
		})
	}
	return teams, nil
}

// normalizeAbtFamily normalizes the wire's uppercase "CRE"/"SRE" into this
// package's lowercase AbtFamily constants. Any other value (including the
// absent/empty case) is passed through lowercased rather than rejected, so an
// unclassified team or a future family value never breaks registry parsing.
func normalizeAbtFamily(wireFamily string) AbtFamily {
	switch wireFamily {
	case "CRE":
		return AbtFamilyCRE
	case "SRE":
		return AbtFamilySRE
	default:
		return AbtFamily(strings.ToLower(wireFamily))
	}
}
