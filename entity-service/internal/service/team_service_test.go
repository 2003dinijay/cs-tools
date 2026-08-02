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

package service

import (
	"context"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// TestSearchTeams_GroupID covers the new optional groupId pass-through: a team
// configured with a 4th (groupSysID) registry field must come back with
// GroupID populated as this platform's UUID, converted via sysidToUUID; a
// team with no configured groupSysID must come back with GroupID empty
// rather than erroring, so the filter-scoping capability degrades gracefully.
func TestSearchTeams_GroupID(t *testing.T) {
	withTeamRegistry(t, "alpha|Alpha Team|CRE|d1e42a1234567890abcdef1234567890,beta|Beta Team|SRE")

	svc := NewTeamService()
	resp, err := svc.SearchTeams(context.Background(), domain.SearchTeamsRequest{
		Pagination: domain.Pagination{Limit: 10, Offset: 0},
	})
	if err != nil {
		t.Fatalf("SearchTeams: %v", err)
	}
	if len(resp.Teams) != 2 {
		t.Fatalf("got %d teams, want 2: %+v", len(resp.Teams), resp.Teams)
	}

	byID := make(map[string]domain.Team, len(resp.Teams))
	for _, team := range resp.Teams {
		byID[team.ID] = team
	}

	alpha, ok := byID["alpha"]
	if !ok {
		t.Fatalf("expected an \"alpha\" team in %+v", resp.Teams)
	}
	wantGroupID := sysidToUUID("d1e42a1234567890abcdef1234567890")
	if alpha.GroupID != wantGroupID {
		t.Fatalf("alpha.GroupID = %q, want %q", alpha.GroupID, wantGroupID)
	}

	beta, ok := byID["beta"]
	if !ok {
		t.Fatalf("expected a \"beta\" team in %+v", resp.Teams)
	}
	if beta.GroupID != "" {
		t.Fatalf("beta.GroupID = %q, want empty (no groupSysID configured)", beta.GroupID)
	}
}
