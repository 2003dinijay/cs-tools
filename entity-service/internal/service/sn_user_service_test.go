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
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// testAbtUserSysid is the caller's ServiceNow sys_id used across the GetMe
// ABT-team-resolution tests.
var testAbtUserSysid = sysid32('9')

// abtTeamsFixtureJSON is a representative (not real) flat ABT registry: a
// CRE team (Castor), a flat IAM-US team (no sub-team nesting), an SRE team
// (Apollo SRE Group), and an unclassified team with no "family" field.
const abtTeamsFixtureJSON = `{
	"teams": [
		{"teamKey": "castor", "displayName": "Castor", "family": "CRE"},
		{"teamKey": "iam_us", "displayName": "IAM-US", "family": "CRE"},
		{"teamKey": "apollo", "displayName": "Apollo SRE Group", "family": "SRE"},
		{"teamKey": "customer_onboarding", "displayName": "Customer Onboarding"}
	]
}`

// snUserMeJSON is a minimal ServiceNow GET /users/me payload for the given
// caller sys_id.
func snUserMeJSON(id string) string {
	return `{
		"id": "` + id + `",
		"email": "agent@example.com",
		"lastName": "Agent",
		"roles": ["wso2_agent"]
	}`
}

// membershipsJSON builds a group-members/search response body with the given
// groupName as the caller's single membership, or an empty memberships list
// if groupName is "".
func membershipsJSON(userID, groupName string) string {
	if groupName == "" {
		return `{"memberships": [], "totalRecords": 0}`
	}
	return `{"memberships": [{"userId": "` + userID + `", "groupId": "irrelevant-sysid", "groupName": "` + groupName + `"}], "totalRecords": 1}`
}

func TestSNUserService_GetMe_TeamMatch(t *testing.T) {
	var capturedBody []byte

	mux := http.NewServeMux()
	mux.HandleFunc("/users/me", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(snUserMeJSON(testAbtUserSysid)))
	})
	mux.HandleFunc("/teams", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(abtTeamsFixtureJSON))
	})
	mux.HandleFunc("/group-members/search", func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(membershipsJSON(testAbtUserSysid, "Castor")))
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowUserService(client)

	got, err := svc.GetMe(contextWithUserIDToken("token"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Team == nil {
		t.Fatalf("Team = nil, want Castor")
	}
	if got.Team.TeamKey != "castor" || got.Team.TeamName != "Castor" || got.Team.Family != "cre" {
		t.Fatalf("Team = %+v, want {castor Castor cre}", got.Team)
	}

	// The request body must send groupNames, never groupIds.
	var reqBody struct {
		Filters struct {
			GroupNames []string `json:"groupNames"`
			GroupIDs   []string `json:"groupIds"`
			UserID     string   `json:"userId"`
		} `json:"filters"`
	}
	if err := json.Unmarshal(capturedBody, &reqBody); err != nil {
		t.Fatalf("unmarshal captured request body: %v", err)
	}
	if reqBody.Filters.GroupIDs != nil {
		t.Fatalf("request body carried groupIds %v, want none (name-based lookup only)", reqBody.Filters.GroupIDs)
	}
	if len(reqBody.Filters.GroupNames) == 0 {
		t.Fatalf("request body carried no groupNames, want the cached registry's display names")
	}
	found := false
	for _, n := range reqBody.Filters.GroupNames {
		if n == "Castor" {
			found = true
		}
	}
	if !found {
		t.Fatalf("groupNames = %v, want it to include \"Castor\"", reqBody.Filters.GroupNames)
	}
	if reqBody.Filters.UserID != testAbtUserSysid {
		t.Fatalf("userId = %q, want %q", reqBody.Filters.UserID, testAbtUserSysid)
	}
}

// TestSNUserService_GetMe_FlatTeamMatch_IAMUS verifies IAM-US resolves as its
// own flat team -- the old sub-team nesting is gone, so this is just a
// normal name match like any other team.
func TestSNUserService_GetMe_FlatTeamMatch_IAMUS(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/users/me", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(snUserMeJSON(testAbtUserSysid)))
	})
	mux.HandleFunc("/teams", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(abtTeamsFixtureJSON))
	})
	mux.HandleFunc("/group-members/search", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(membershipsJSON(testAbtUserSysid, "IAM-US")))
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowUserService(client)

	got, err := svc.GetMe(contextWithUserIDToken("token"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Team == nil {
		t.Fatalf("Team = nil, want IAM-US")
	}
	if got.Team.TeamKey != "iam_us" || got.Team.TeamName != "IAM-US" || got.Team.Family != "cre" {
		t.Fatalf("Team = %+v, want {iam_us IAM-US cre}", got.Team)
	}
}

// TestSNUserService_GetMe_UnclassifiedTeamMatch verifies a team with no
// "family" set still resolves correctly, with an empty Family rather than an
// error.
func TestSNUserService_GetMe_UnclassifiedTeamMatch(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/users/me", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(snUserMeJSON(testAbtUserSysid)))
	})
	mux.HandleFunc("/teams", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(abtTeamsFixtureJSON))
	})
	mux.HandleFunc("/group-members/search", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(membershipsJSON(testAbtUserSysid, "Customer Onboarding")))
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowUserService(client)

	got, err := svc.GetMe(contextWithUserIDToken("token"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Team == nil {
		t.Fatalf("Team = nil, want Customer Onboarding")
	}
	if got.Team.TeamKey != "customer_onboarding" || got.Team.TeamName != "Customer Onboarding" || got.Team.Family != "" {
		t.Fatalf("Team = %+v, want {customer_onboarding \"Customer Onboarding\" \"\"}", got.Team)
	}
}

func TestSNUserService_GetMe_NoMatch(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/users/me", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(snUserMeJSON(testAbtUserSysid)))
	})
	mux.HandleFunc("/teams", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(abtTeamsFixtureJSON))
	})
	mux.HandleFunc("/group-members/search", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(membershipsJSON(testAbtUserSysid, "")))
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowUserService(client)

	got, err := svc.GetMe(contextWithUserIDToken("token"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Team != nil {
		t.Fatalf("Team = %+v, want nil (no membership)", got.Team)
	}
}

// TestSNUserService_GetMe_GroupMembershipCallErrors_IdentityStillReturned
// verifies that a downstream failure on the group-members/search call never
// fails the overall /users/me response -- identity/roles must still come
// back, with Team simply nil.
func TestSNUserService_GetMe_GroupMembershipCallErrors_IdentityStillReturned(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/users/me", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(snUserMeJSON(testAbtUserSysid)))
	})
	mux.HandleFunc("/teams", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(abtTeamsFixtureJSON))
	})
	mux.HandleFunc("/group-members/search", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowUserService(client)

	got, err := svc.GetMe(contextWithUserIDToken("token"))
	if err != nil {
		t.Fatalf("unexpected error: %v (identity must still be returned)", err)
	}
	if got.Email != "agent@example.com" {
		t.Fatalf("Email = %q, want agent@example.com even though group lookup failed", got.Email)
	}
	if got.Team != nil {
		t.Fatalf("Team = %+v, want nil when group-membership call errors", got.Team)
	}
}

// TestSNUserService_GetMe_RegistryFetchFails_IdentityStillReturned verifies
// that a failure fetching the ABT team registry itself (a distinct failure
// mode from the group-membership call failing) also degrades gracefully:
// identity/roles still come back, Team nil.
func TestSNUserService_GetMe_RegistryFetchFails_IdentityStillReturned(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/users/me", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(snUserMeJSON(testAbtUserSysid)))
	})
	mux.HandleFunc("/teams", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	groupMembersCalled := false
	mux.HandleFunc("/group-members/search", func(w http.ResponseWriter, r *http.Request) {
		groupMembersCalled = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(membershipsJSON(testAbtUserSysid, "Castor")))
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowUserService(client)

	got, err := svc.GetMe(contextWithUserIDToken("token"))
	if err != nil {
		t.Fatalf("unexpected error: %v (identity must still be returned)", err)
	}
	if got.Email != "agent@example.com" {
		t.Fatalf("Email = %q, want agent@example.com even though registry fetch failed", got.Email)
	}
	if got.Team != nil {
		t.Fatalf("Team = %+v, want nil when registry fetch fails", got.Team)
	}
	// With an empty registry, AbtGroupNames() is empty, so GetMe should
	// short-circuit and never even call group-members/search.
	if groupMembersCalled {
		t.Fatalf("group-members/search was called despite an empty (failed-to-load) registry")
	}
}

// TestGetUserMeResponse_TeamOmittedWhenNil locks in the JSON field names and
// casing a downstream CSM-backend consumer depends on: "team" (camelCase),
// omitted entirely when the caller has no resolved ABT team.
func TestGetUserMeResponse_TeamOmittedWhenNil(t *testing.T) {
	resp := domain.GetUserMeResponse{ID: "u1", Email: "agent@example.com", Roles: []string{}}
	raw, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var asMap map[string]json.RawMessage
	if err := json.Unmarshal(raw, &asMap); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, present := asMap["team"]; present {
		t.Fatalf("expected \"team\" to be omitted when Team is nil, got: %s", raw)
	}
}

// TestGetUserMeResponse_TeamFieldShape locks in the exact field names/casing
// on the populated Team object: teamKey, teamName, family. This shape is
// unchanged by the sys_id -> name-based resolution switch.
func TestGetUserMeResponse_TeamFieldShape(t *testing.T) {
	resp := domain.GetUserMeResponse{
		ID: "u1", Email: "agent@example.com", Roles: []string{},
		Team: &domain.UserTeam{TeamKey: "castor", TeamName: "Castor", Family: "cre"},
	}
	raw, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var asMap map[string]json.RawMessage
	if err := json.Unmarshal(raw, &asMap); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	teamRaw, present := asMap["team"]
	if !present {
		t.Fatalf("expected \"team\" to be present, got: %s", raw)
	}
	var team map[string]string
	if err := json.Unmarshal(teamRaw, &team); err != nil {
		t.Fatalf("unmarshal team: %v", err)
	}
	want := map[string]string{"teamKey": "castor", "teamName": "Castor", "family": "cre"}
	for k, v := range want {
		if team[k] != v {
			t.Fatalf("team[%q] = %q, want %q (full team object: %s)", k, team[k], v, teamRaw)
		}
	}
}
