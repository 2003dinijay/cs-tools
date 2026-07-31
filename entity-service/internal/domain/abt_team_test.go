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
	"errors"
	"testing"
)

// abtTeamsFixtureJSON is a representative (not real) flat ABT registry:
// several CRE teams, an SRE team, and an unclassified team with no "family"
// field at all (mirrors the 6 unclassified teams in the real 15-team list).
const abtTeamsFixtureJSON = `{
	"teams": [
		{"teamKey": "castor", "displayName": "Castor", "family": "CRE"},
		{"teamKey": "sirius", "displayName": "Sirius", "family": "CRE"},
		{"teamKey": "vega", "displayName": "Vega", "family": "CRE"},
		{"teamKey": "apollo", "displayName": "Apollo SRE Group", "family": "SRE"},
		{"teamKey": "iam_us", "displayName": "IAM-US", "family": "CRE"},
		{"teamKey": "customer_onboarding", "displayName": "Customer Onboarding"}
	]
}`

// resetAbtRegistry clears package-level ABT registry state so the calling test
// starts from a clean, unloaded cache, and registers a cleanup that clears it
// again on exit. Without the cleanup the last test to run leaves its fetcher and
// cached teams in place, which any later test in this package would silently
// inherit -- making results order-dependent.
func resetAbtRegistry(t *testing.T) {
	t.Helper()
	clearAbtRegistry()
	t.Cleanup(clearAbtRegistry)
}

func clearAbtRegistry() {
	abtMu.Lock()
	defer abtMu.Unlock()
	abtFetcher = nil
	abtLoaded = false
	abtTeams = nil
}

// TestAbtRegistry_StartsClean deliberately does NOT call resetAbtRegistry: it asserts that
// whatever ran before it left the package-level registry empty. It is the test that makes
// resetAbtRegistry's cleanup load-bearing -- drop the cleanup and this fails under
// -shuffle=on whenever it is not scheduled first.
func TestAbtRegistry_StartsClean(t *testing.T) {
	abtMu.Lock()
	fetcher, loaded, teams := abtFetcher, abtLoaded, abtTeams
	abtMu.Unlock()

	if fetcher != nil || loaded || teams != nil {
		t.Fatalf("registry not clean on entry: fetcher set=%t loaded=%t teams=%d; an earlier test leaked state",
			fetcher != nil, loaded, len(teams))
	}
}

func TestAbtRegistry_FetchAndCache_PopulatesCorrectly(t *testing.T) {
	resetAbtRegistry(t)
	SetAbtTeamsFetcher(func(ctx context.Context) (json.RawMessage, error) {
		return json.RawMessage(abtTeamsFixtureJSON), nil
	})

	names := AbtGroupNames()
	if len(names) != 6 {
		t.Fatalf("AbtGroupNames() returned %d names, want 6: %v", len(names), names)
	}

	// Sirius: CRE.
	sirius, ok := FindAbtTeamByGroupName("Sirius")
	if !ok {
		t.Fatalf("expected to find Sirius")
	}
	if sirius.TeamKey != "sirius" || sirius.Family != AbtFamilyCRE {
		t.Fatalf("Sirius = %+v, want teamKey=sirius family=cre", sirius)
	}

	// IAM-US: flat team (not a sub-team), CRE.
	iamUS, ok := FindAbtTeamByGroupName("IAM-US")
	if !ok {
		t.Fatalf("expected to find IAM-US")
	}
	if iamUS.TeamKey != "iam_us" || iamUS.Family != AbtFamilyCRE {
		t.Fatalf("IAM-US = %+v, want teamKey=iam_us family=cre", iamUS)
	}

	// Apollo SRE Group: SRE.
	apollo, ok := FindAbtTeamByGroupName("Apollo SRE Group")
	if !ok {
		t.Fatalf("expected to find Apollo SRE Group")
	}
	if apollo.TeamKey != "apollo" || apollo.Family != AbtFamilySRE {
		t.Fatalf("Apollo = %+v, want teamKey=apollo family=sre", apollo)
	}

	// Customer Onboarding: no family field at all -- must still parse and
	// match, with an empty Family rather than a parse error.
	onboarding, ok := FindAbtTeamByGroupName("Customer Onboarding")
	if !ok {
		t.Fatalf("expected to find Customer Onboarding")
	}
	if onboarding.TeamKey != "customer_onboarding" || onboarding.Family != "" {
		t.Fatalf("Customer Onboarding = %+v, want teamKey=customer_onboarding family=\"\"", onboarding)
	}

	// Unknown name -> no match.
	if _, ok := FindAbtTeamByGroupName("Does Not Exist"); ok {
		t.Fatalf("expected no match for unknown group name")
	}
}

func TestAbtRegistry_FetchFailure_DegradesToEmptyRegistry(t *testing.T) {
	resetAbtRegistry(t)
	SetAbtTeamsFetcher(func(ctx context.Context) (json.RawMessage, error) {
		return nil, errors.New("upstream unavailable")
	})

	names := AbtGroupNames()
	if names == nil {
		t.Fatalf("AbtGroupNames() returned nil, want empty non-nil slice")
	}
	if len(names) != 0 {
		t.Fatalf("AbtGroupNames() = %v, want empty on fetch failure", names)
	}

	if _, ok := FindAbtTeamByGroupName("Castor"); ok {
		t.Fatalf("expected no match when registry failed to load")
	}
}

func TestAbtRegistry_NoFetcherRegistered_DegradesToEmptyRegistry(t *testing.T) {
	resetAbtRegistry(t)

	names := AbtGroupNames()
	if len(names) != 0 {
		t.Fatalf("AbtGroupNames() = %v, want empty when no fetcher registered", names)
	}
}

// TestAbtRegistry_FetchFailure_IsNotCached guards the availability property that
// matters most here: one transient upstream failure must not leave the registry
// permanently empty. A failed load is not committed, so the very next lookup
// retries and picks up the recovered upstream.
func TestAbtRegistry_FetchFailure_IsNotCached(t *testing.T) {
	resetAbtRegistry(t)

	calls := 0
	failing := true
	SetAbtTeamsFetcher(func(ctx context.Context) (json.RawMessage, error) {
		calls++
		if failing {
			return nil, errors.New("upstream unavailable")
		}
		return json.RawMessage(abtTeamsFixtureJSON), nil
	})

	if names := AbtGroupNames(); len(names) != 0 {
		t.Fatalf("AbtGroupNames() = %v, want empty while upstream is failing", names)
	}

	// Upstream recovers. No restart, no SetAbtTeamsFetcher call.
	failing = false

	names := AbtGroupNames()
	if len(names) != 6 {
		t.Fatalf("after recovery AbtGroupNames() returned %d names, want 6: %v", len(names), names)
	}
	if _, ok := FindAbtTeamByGroupName("Castor"); !ok {
		t.Fatalf("expected Castor to resolve after the registry recovered")
	}
	if calls != 2 {
		t.Fatalf("fetcher called %d times, want 2 (one failed attempt, then one retry that is cached)", calls)
	}
}

// TestAbtRegistry_NoFetcher_IsNotCached checks the same for the missing-fetcher
// path: a lookup made before service wiring registers the fetcher must not
// permanently mark the registry loaded.
func TestAbtRegistry_NoFetcher_IsNotCached(t *testing.T) {
	resetAbtRegistry(t)

	if names := AbtGroupNames(); len(names) != 0 {
		t.Fatalf("AbtGroupNames() = %v, want empty with no fetcher registered", names)
	}

	abtMu.Lock()
	loaded := abtLoaded
	abtMu.Unlock()
	if loaded {
		t.Fatalf("abtLoaded = true after a lookup with no fetcher; the empty registry is now permanent")
	}
}

func TestAbtRegistry_LoadsOnlyOnce(t *testing.T) {
	resetAbtRegistry(t)
	calls := 0
	SetAbtTeamsFetcher(func(ctx context.Context) (json.RawMessage, error) {
		calls++
		return json.RawMessage(abtTeamsFixtureJSON), nil
	})

	_ = AbtGroupNames()
	_ = AbtGroupNames()
	_, _ = FindAbtTeamByGroupName("Castor")

	if calls != 1 {
		t.Fatalf("fetcher called %d times, want exactly 1 (cached after first load)", calls)
	}
}
