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
	"net/http"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// TestSNProjectService_SearchProjects_MapsAccountRef verifies that the account
// reference newly added to ServiceNow's project search response is mapped into
// domain.ProjectView.Account, and that a project with no linked account (blank
// id/name) maps to a nil Account rather than a zero-valued ref.
func TestSNProjectService_SearchProjects_MapsAccountRef(t *testing.T) {
	const accountSysid = "4a6fc0623b16c31091404c6aa5e45a09"

	client := newTestSNClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"projects": []map[string]any{
				{
					"id": "11111111111111111111111111111111", "name": "With account", "key": "WA",
					"type":    map[string]any{"name": "Subscription"},
					"endDate": "", "createdOn": "2026-01-01 00:00:00",
					"account": map[string]any{"id": accountSysid, "name": "Automation Test Customer Account"},
				},
				{
					"id": "22222222222222222222222222222222", "name": "No account", "key": "NA",
					"type":    map[string]any{"name": "Subscription"},
					"endDate": "", "createdOn": "2026-01-01 00:00:00",
					"account": map[string]any{"id": "", "name": ""},
				},
			},
			"totalRecords": 2, "offset": 0, "limit": 10,
		})
	}))

	svc := NewServiceNowProjectService(client, nil)
	resp, err := svc.SearchProjects(contextWithUserIDToken("token"), domain.SearchProjectsRequest{
		Pagination: domain.Pagination{Limit: 10},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Projects) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(resp.Projects))
	}

	withAccount := resp.Projects[0]
	if withAccount.Account == nil {
		t.Fatalf("expected non-nil Account for project with a linked account")
	}
	if withAccount.Account.ID != sysidToUUID(accountSysid) || withAccount.Account.Name != "Automation Test Customer Account" {
		t.Fatalf("unexpected Account: %+v", withAccount.Account)
	}

	noAccount := resp.Projects[1]
	if noAccount.Account != nil {
		t.Fatalf("expected nil Account for project with no linked account, got %+v", noAccount.Account)
	}
}

// TestSNProjectService_SearchProjects_MapsStartDate verifies that the date-only
// startDate from ServiceNow's project search response is parsed into
// domain.ProjectView.StartDate, and that a null or absent startDate maps to a
// nil pointer rather than a zero time (which would serialize as year 0001).
func TestSNProjectService_SearchProjects_MapsStartDate(t *testing.T) {
	client := newTestSNClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"projects": []map[string]any{
				{
					"id": "11111111111111111111111111111111", "name": "With start date", "key": "WSD",
					"type":      map[string]any{"name": "Subscription"},
					"startDate": "2026-03-15", "endDate": "2027-03-14",
					"createdOn": "2024-01-01 00:00:00",
					"account":   map[string]any{"id": "", "name": ""},
				},
				{
					"id": "22222222222222222222222222222222", "name": "Null start date", "key": "NSD",
					"type":      map[string]any{"name": "Subscription"},
					"startDate": nil, "endDate": "",
					"createdOn": "2024-01-01 00:00:00",
					"account":   map[string]any{"id": "", "name": ""},
				},
				{
					// startDate key omitted entirely.
					"id": "33333333333333333333333333333333", "name": "Absent start date", "key": "ASD",
					"type":    map[string]any{"name": "Subscription"},
					"endDate": "", "createdOn": "2024-01-01 00:00:00",
					"account": map[string]any{"id": "", "name": ""},
				},
			},
			"totalRecords": 3, "offset": 0, "limit": 10,
		})
	}))

	svc := NewServiceNowProjectService(client, nil)
	resp, err := svc.SearchProjects(contextWithUserIDToken("token"), domain.SearchProjectsRequest{
		Pagination: domain.Pagination{Limit: 10},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Projects) != 3 {
		t.Fatalf("expected 3 projects, got %d", len(resp.Projects))
	}

	withStart := resp.Projects[0]
	if withStart.StartDate == nil {
		t.Fatalf("expected non-nil StartDate for project with a start date")
	}
	want := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	if !withStart.StartDate.Equal(want) {
		t.Fatalf("unexpected StartDate: got %v, want %v", *withStart.StartDate, want)
	}
	// StartDate must stay distinct from EndDate and CreatedOn.
	if withStart.EndDate == nil || !withStart.EndDate.Equal(time.Date(2027, 3, 14, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected EndDate: %v", withStart.EndDate)
	}
	if !withStart.CreatedOn.Equal(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected CreatedOn: %v", withStart.CreatedOn)
	}

	if got := resp.Projects[1].StartDate; got != nil {
		t.Fatalf("expected nil StartDate for null startDate, got %v", *got)
	}
	if got := resp.Projects[2].StartDate; got != nil {
		t.Fatalf("expected nil StartDate for absent startDate, got %v", *got)
	}
}

// The contact id is optional upstream: absent on an instance that predates the field, and
// null for a row with no linked contact record. Neither case may produce a bogus id — the
// caller uses emptiness to decide whether the row is clickable.
func TestSNProjectContactService_SearchProjectContacts_OptionalContactID(t *testing.T) {
	projectUUID := sysidToUUID(sysid32('7'))
	contactSysid := sysid32('8')

	client := newTestSNClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"contacts":[
			{"id":"` + contactSysid + `","name":"Linked","email":"linked@example.com",
			 "registrationState":"REGISTERED","notificationsEnabled":true,"roles":["r"]},
			{"id":null,"name":"Orphaned","email":"orphan@example.com",
			 "registrationState":"INVITED","notificationsEnabled":false,"roles":[]},
			{"name":"OldInstance","email":"old@example.com",
			 "registrationState":"INVITED","notificationsEnabled":false,"roles":[]}
		],"totalRecords":3,"offset":0,"limit":10}`))
	}))

	svc := NewServiceNowProjectContactService(client)

	got, err := svc.SearchProjectContacts(contextWithUserIDToken("token"), projectUUID,
		domain.SearchProjectContactsRequest{Pagination: domain.Pagination{Limit: 10}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Contacts) != 3 {
		t.Fatalf("got %d contacts, want 3", len(got.Contacts))
	}

	if want := sysidToUUID(contactSysid); got.Contacts[0].ID != want {
		t.Errorf("linked contact ID = %q, want %q", got.Contacts[0].ID, want)
	}
	if got.Contacts[1].ID != "" {
		t.Errorf("null upstream id produced %q, want empty", got.Contacts[1].ID)
	}
	if got.Contacts[2].ID != "" {
		t.Errorf("absent upstream id produced %q, want empty", got.Contacts[2].ID)
	}
}
