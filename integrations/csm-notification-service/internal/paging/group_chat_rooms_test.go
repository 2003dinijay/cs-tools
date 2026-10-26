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

package paging

import (
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/notifications"
)

const (
	iaasGroupID  = "77777777-7777-4777-8777-000000000001"
	iaasGroupURL = "https://chat.googleapis.com/v1/spaces/IAAS/messages?key=k&token=t"
)

// A saved SRE group room becomes sre.groupChats under the group's lower-cased
// name, its URL in a variable named by the group's id; the file's rooms,
// aliases and teams are untouched.
func TestMergeCaseAlertSettings_GroupRooms(t *testing.T) {
	cfg := loadConfigText(t, smeShapedConfig)
	pristine := loadConfigText(t, smeShapedConfig)
	saved := CaseAlertSettings{Rooms: []CaseAlertRoom{
		{Ladder: "SRE", GroupID: iaasGroupID, GroupName: " WSO2 IaaS Team ", Label: "WSO2 IaaS Team", WebhookURL: iaasGroupURL},
		{Ladder: "SRE", GroupID: "77777777-7777-4777-8777-000000000002", GroupName: "Kept without a URL", Label: "x"},
		{Ladder: "CRE", GroupID: "77777777-7777-4777-8777-000000000003", GroupName: "A CRE group", Label: "y", WebhookURL: iaasGroupURL},
	}}
	merged, env, changes, err := MergeCaseAlertSettings(cfg, saved)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg, pristine) {
		t.Fatal("the merge wrote into the caller's configuration")
	}
	want := map[string]Chat{"wso2 iaas team": {Audience: "WSO2 IaaS Team", WebhookURLEnv: "CASE_ALERTS_ROOM_SRE_GROUP_77777777_7777_4777_8777_000000000001"}}
	if !maps.Equal(merged.SRE.GroupChats, want) {
		t.Fatalf("group chats = %+v, want %+v", merged.SRE.GroupChats, want)
	}
	if env[want["wso2 iaas team"].WebhookURLEnv] != iaasGroupURL || len(env) != 1 {
		t.Fatalf("env = %v", env)
	}
	if !reflect.DeepEqual(merged.SRE.TeamChats, cfg.SRE.TeamChats) || !reflect.DeepEqual(merged.SRE.Teams, cfg.SRE.Teams) || merged.SRE.Chat != cfg.SRE.Chat {
		t.Error("a team room, alias or the shared room changed when only a group room was saved")
	}
	joined := strings.Join(changes, "\n")
	if !strings.Contains(joined, `sre.groupChats[wso2 iaas team]: saved room "WSO2 IaaS Team"`) || !strings.Contains(joined, `skipped a CRE room for group "a cre group"`) {
		t.Errorf("changes:\n%s", joined)
	}
	if strings.Contains(joined, "googleapis") || strings.Contains(joined, "token=") {
		t.Error("a change line quotes a URL")
	}
	routing := CaseAlertGroupRouting(merged)["SRE"]
	if !slices.Equal(routing.GroupRooms, []string{"wso2 iaas team"}) {
		t.Errorf("reported group rooms = %v", routing.GroupRooms)
	}
}

// A group's own room wins over the room of the team it is routed to, and over
// the teamless fan-out of a default group; its calls are not this table's.
func TestBuildIncidentRoutes_GroupRoomWins(t *testing.T) {
	env := map[string]string{
		"CASE_ALERTS_ROOM_SRE_ARTEMIS":       "https://chat.googleapis.com/artemis",
		"CASE_ALERTS_ROOM_SRE_APOLLO":        "https://chat.googleapis.com/apollo",
		"CASE_ALERTS_ROOM_SRE":               "https://chat.googleapis.com/sre",
		"CASE_ALERTS_ROOM_SRE_GROUP_U2":      "https://chat.googleapis.com/u2-group",
		"CASE_ALERTS_ROOM_SRE_GROUP_DEFAULT": "https://chat.googleapis.com/default-group",
	}
	resolve := func(c Chat) (string, bool) {
		return ChatWebhookURL(c, nil, func(k string) string { return env[k] })
	}
	l := LadderConfig{
		Chat: Chat{WebhookURLEnv: "CASE_ALERTS_ROOM_SRE"},
		TeamChats: map[string]Chat{
			"artemis": {WebhookURLEnv: "CASE_ALERTS_ROOM_SRE_ARTEMIS"},
			"apollo":  {WebhookURLEnv: "CASE_ALERTS_ROOM_SRE_APOLLO"},
		},
		TeamlessChats: []string{"apollo", "artemis"},
		GroupChats: map[string]Chat{
			"u2 sre group":      {WebhookURLEnv: "CASE_ALERTS_ROOM_SRE_GROUP_U2"},
			"default sre group": {WebhookURLEnv: "CASE_ALERTS_ROOM_SRE_GROUP_DEFAULT"},
			"no url group":      {WebhookURLEnv: "CASE_ALERTS_ROOM_SRE_GROUP_UNSET"},
		},
	}
	l.Teams.Aliases = map[string]string{"U2 SRE Group": "artemis", "Artemis SRE Group": "artemis"}
	l.Teams.DefaultGroups = []string{"Default SRE Group", "Other Default"}

	got := BuildIncidentRoutes(l, resolve)

	checks := map[string]string{
		"U2 SRE Group":      "https://chat.googleapis.com/u2-group",
		"Artemis SRE Group": "https://chat.googleapis.com/artemis",
		"Default SRE Group": "https://chat.googleapis.com/default-group",
	}
	for group, want := range checks {
		if u := got.Spaces[notifications.IncidentAudience(group)]; u != want {
			t.Errorf("%s -> %q, want %q", group, u, want)
		}
	}
	if _, ok := got.Spaces[notifications.IncidentAudience("no url group")]; ok {
		t.Error("a group room with no URL was registered")
	}
	if _, ok := got.Fanout["default sre group"]; ok {
		t.Error("a default group with its own room still fans out to the teamless rooms")
	}
	if !slices.Equal(got.Fanout["other default"], []string{"apollo", "artemis"}) {
		t.Errorf("other default fan-out = %v", got.Fanout["other default"])
	}

	// The card and the replies resolve through the same audiences.
	routes := &notifications.IncidentRoutes{}
	routes.Set(got.Fanout)
	has := func(a string) bool { _, ok := got.Spaces[a]; return ok }
	for group, want := range map[string][]string{
		"Default SRE Group": {notifications.IncidentAudience("default sre group")},
		"Other Default":     {notifications.IncidentAudience("apollo"), notifications.IncidentAudience("artemis")},
		"u2 sre group":      {notifications.IncidentAudience("u2 sre group")},
	} {
		if a, _ := notifications.IncidentAudiences(routes, nil, group, has); !slices.Equal(a, want) {
			t.Errorf("%s audiences = %v, want %v", group, a, want)
		}
	}
}
