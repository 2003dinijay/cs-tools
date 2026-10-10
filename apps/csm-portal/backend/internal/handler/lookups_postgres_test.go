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

package handler

import (
	"context"
	"encoding/json"
	"testing"
)

// fakeEntityProductsClient is a minimal entityProductsClient for testing
// postgresLookupsClient without a real entity-service.
type fakeEntityProductsClient struct {
	teamsPages [][]entityTeam
	teamsTotal int
	teamsCalls int
	teamsErr   error
}

func (f *fakeEntityProductsClient) SearchProducts(ctx context.Context, body []byte) ([]byte, error) {
	return nil, nil
}

func (f *fakeEntityProductsClient) SearchTeams(ctx context.Context, body []byte) ([]byte, error) {
	if f.teamsErr != nil {
		return nil, f.teamsErr
	}
	page := f.teamsCalls
	f.teamsCalls++
	var teams []entityTeam
	if page < len(f.teamsPages) {
		teams = f.teamsPages[page]
	}
	return json.Marshal(entitySearchTeamsResponse{Teams: teams, Total: f.teamsTotal})
}

func TestPostgresLookupsClient_GetABTTeamList_FiltersByTypeSuffix(t *testing.T) {
	fake := &fakeEntityProductsClient{
		teamsPages: [][]entityTeam{{
			{Name: "Apollo", Type: "sre-abt"},
			{Name: "Americas", Type: "cre"},
			{Name: "Atlas", Type: "cre-abt"},
			{Name: "CRE Leadership", Type: "cre-leadership"},
			{Name: "Near Miss", Type: "abt-support"},
		}},
		teamsTotal: 5,
	}
	c := NewPostgresLookupsClient(fake)

	got, err := c.GetABTTeamList(context.Background())
	if err != nil {
		t.Fatalf("GetABTTeamList: %v", err)
	}
	want := []string{"Apollo", "Atlas"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i, name := range want {
		if got[i] != name {
			t.Errorf("got[%d] = %q, want %q", i, got[i], name)
		}
	}
}

func TestPostgresLookupsClient_GetABTTeamList_DedupesAndSorts(t *testing.T) {
	fake := &fakeEntityProductsClient{
		teamsPages: [][]entityTeam{{
			{Name: "Vega", Type: "cre-abt"},
			{Name: "Vega", Type: "cre-abt"},
			{Name: "Apollo", Type: "sre-abt"},
			{Name: "  ", Type: "cre-abt"},
		}},
		teamsTotal: 4,
	}
	c := NewPostgresLookupsClient(fake)

	got, err := c.GetABTTeamList(context.Background())
	if err != nil {
		t.Fatalf("GetABTTeamList: %v", err)
	}
	want := []string{"Apollo", "Vega"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i, name := range want {
		if got[i] != name {
			t.Errorf("got[%d] = %q, want %q", i, got[i], name)
		}
	}
}

func TestPostgresLookupsClient_GetABTTeamList_PagesThrough(t *testing.T) {
	fake := &fakeEntityProductsClient{
		teamsPages: [][]entityTeam{
			make([]entityTeam, entityTeamsPageLimit), // full first page -> keep paging
			{{Name: "Draco", Type: "cre-abt"}},
		},
		teamsTotal: entityTeamsPageLimit + 1,
	}
	for i := range fake.teamsPages[0] {
		fake.teamsPages[0][i] = entityTeam{Name: "Filler", Type: "cre"}
	}
	c := NewPostgresLookupsClient(fake)

	got, err := c.GetABTTeamList(context.Background())
	if err != nil {
		t.Fatalf("GetABTTeamList: %v", err)
	}
	if fake.teamsCalls != 2 {
		t.Fatalf("expected 2 pages fetched, got %d", fake.teamsCalls)
	}
	if len(got) != 1 || got[0] != "Draco" {
		t.Fatalf("got %v, want [Draco]", got)
	}
}
