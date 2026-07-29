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

// resetAbtRegistry clears package-level ABT registry state between tests so
// each test starts from a clean, unloaded cache.
func resetAbtRegistry(t *testing.T) {
	t.Helper()
	abtMu.Lock()
	abtFetcher = nil
	abtLoaded = false
	abtTeams = nil
	abtMu.Unlock()
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
