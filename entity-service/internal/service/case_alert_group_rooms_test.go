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
	"reflect"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const roomIaaSGroup = "88888888-8888-4888-8888-000000000001"

func TestCaseAlertSaveRoom_GroupRoom(t *testing.T) {
	yes, chat := true, "chat"
	iaas := caseAlertAssignmentGroups[0]
	admin := asPerson("someone@wso2.com", "cs_admin")
	cases := []struct {
		name string
		req  domain.SaveCaseAlertRoomRequest
		want string // "", "forbidden", "invalid"
	}{
		{"an SRE group's room, labelled by its name", domain.SaveCaseAlertRoomRequest{Ladder: "sre", GroupID: " " + iaas.ID + " "}, ""},
		{"an SRE group's room, own label", domain.SaveCaseAlertRoomRequest{Ladder: "SRE", GroupID: iaas.ID, Label: "IaaS chat"}, ""},
		{"a CRE group's room", domain.SaveCaseAlertRoomRequest{Ladder: "CRE", GroupID: iaas.ID}, "invalid"},
		{"an SME group's room", domain.SaveCaseAlertRoomRequest{Ladder: "SME", GroupID: iaas.ID}, "invalid"},
		{"a group and a team", domain.SaveCaseAlertRoomRequest{Ladder: "SRE", GroupID: iaas.ID, TeamKey: "apollo"}, "invalid"},
		{"a group's room, teamless", domain.SaveCaseAlertRoomRequest{Ladder: "SRE", GroupID: iaas.ID, Teamless: &yes}, "invalid"},
		{"a group's room with a channel", domain.SaveCaseAlertRoomRequest{Ladder: "SRE", GroupID: iaas.ID, Channel: &chat}, "invalid"},
		{"not a uuid", domain.SaveCaseAlertRoomRequest{Ladder: "SRE", GroupID: "wso2-iaas"}, "invalid"},
		{"not an assignment group", domain.SaveCaseAlertRoomRequest{Ladder: "SRE", GroupID: "77777777-7777-4777-8777-0000000000ff"}, "invalid"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo := newFakeCaseAlertRepo()
			svc, _ := newCaseAlerts(repo, testCaseAlertKey)
			_, err := svc.SaveRoom(admin, c.req)
			if c.want == "invalid" {
				wantAPIError[*apierror.ValidationError](t, err, c.name)
				if repo.saved != nil {
					t.Fatalf("a refused save was written: %+v", repo.saved)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			want := repository.CaseAlertRoomWrite{Ladder: "SRE", GroupID: iaas.ID, Label: iaas.Name, Enabled: true}
			if c.req.Label != "" {
				want.Label = c.req.Label
			}
			if !reflect.DeepEqual(*repo.saved, want) {
				t.Fatalf("saved %+v, want %+v", *repo.saved, want)
			}
		})
	}

	// The SRE rota admin may save one; the SME rota admin may not.
	repo := newFakeCaseAlertRepo()
	svc, _ := newCaseAlerts(repo, testCaseAlertKey)
	if _, err := svc.SaveRoom(asPerson("sre.rota@wso2.com"), domain.SaveCaseAlertRoomRequest{Ladder: "SRE", GroupID: iaas.ID}); err != nil {
		t.Fatal(err)
	}
	_, err := svc.SaveRoom(asPerson("sme.rota@wso2.com"), domain.SaveCaseAlertRoomRequest{Ladder: "SRE", GroupID: iaas.ID})
	wantAPIError[*apierror.ForbiddenError](t, err, "the SME rota admin saving an SRE group's room")
}

func TestCaseAlertRuntime_GroupRoom(t *testing.T) {
	repo := newFakeCaseAlertRepo()
	iaas := caseAlertAssignmentGroups[0]
	repo.rooms[roomIaaSGroup] = &repository.CaseAlertRoomRow{ID: roomIaaSGroup, Ladder: "SRE", GroupID: iaas.ID, GroupName: iaas.Name, Label: iaas.Name, Enabled: true}
	svc, _ := newCaseAlerts(repo, testCaseAlertKey)
	admin := asPerson("someone@wso2.com", "cs_admin")

	got, err := svc.Runtime(asService("csm-notification-service"))
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range got.Rooms {
		if r.GroupID != "" {
			t.Fatalf("a group's room with no URL reached the alert service: %+v", r)
		}
	}

	if _, err := svc.SetWebhook(admin, roomIaaSGroup, domain.SetCaseAlertWebhookRequest{URL: testChatWebhook}); err != nil {
		t.Fatal(err)
	}
	if got, err = svc.Runtime(asService("csm-notification-service")); err != nil {
		t.Fatal(err)
	}
	want := domain.CaseAlertRuntimeRoom{Ladder: "SRE", GroupID: iaas.ID, GroupName: iaas.Name, Label: iaas.Name, WebhookURL: testChatWebhook}
	found := false
	for _, r := range got.Rooms {
		if r.GroupID != "" {
			found = reflect.DeepEqual(r, want)
		}
	}
	if !found {
		t.Fatalf("rooms = %+v, want %+v among them", got.Rooms, want)
	}

	// With its own room the group is no longer one whose cards go to the
	// shared SRE room.
	settings, err := svc.Get(admin)
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range settings.UnmappedGroups {
		if g.ID == iaas.ID {
			t.Fatalf("%s still listed as unmapped: %+v", iaas.Name, settings.UnmappedGroups)
		}
	}
}
