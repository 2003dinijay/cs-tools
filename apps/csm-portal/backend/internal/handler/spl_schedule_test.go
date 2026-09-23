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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/servicenow"
)

type mockSplScheduleClient struct {
	schedule                                              servicenow.ABTTeamScheduleData
	err                                                   error
	gotFrom, gotDuration, gotTeamID, gotEventType, gotURL string
}

func (m *mockSplScheduleClient) GetABTTeamSchedule(ctx context.Context, from, duration, teamID, eventType, teamScheduleURL string) (servicenow.ABTTeamScheduleData, error) {
	m.gotFrom, m.gotDuration, m.gotTeamID, m.gotEventType, m.gotURL = from, duration, teamID, eventType, teamScheduleURL
	return m.schedule, m.err
}

func TestGetABTTeamSchedule_PassesParamsAndConfiguredURL(t *testing.T) {
	mock := &mockSplScheduleClient{schedule: servicenow.ABTTeamScheduleData{SnURL: "https://sn.example.com"}}
	h := NewSplScheduleHandler(mock, []string{"csm-agents"}, "https://sn.example.com")
	req := withUser(httptest.NewRequest(http.MethodGet, "/spl/abt-team-schedule?from=2024-01-01&duration=7d&teamId=team-1&eventType=oncall", nil))
	w := httptest.NewRecorder()

	h.GetABTTeamSchedule(w, req)

	assertStatus(t, w, http.StatusOK)
	if mock.gotFrom != "2024-01-01" || mock.gotTeamID != "team-1" || mock.gotURL != "https://sn.example.com" {
		t.Errorf("client called with from=%q teamID=%q url=%q", mock.gotFrom, mock.gotTeamID, mock.gotURL)
	}
}

func TestGetABTTeamSchedule_AllParamsOptional(t *testing.T) {
	mock := &mockSplScheduleClient{}
	h := NewSplScheduleHandler(mock, []string{"csm-agents"}, "https://sn.example.com")
	req := withUser(httptest.NewRequest(http.MethodGet, "/spl/abt-team-schedule", nil))
	w := httptest.NewRecorder()

	h.GetABTTeamSchedule(w, req)

	assertStatus(t, w, http.StatusOK)
}

func TestGetABTTeamSchedule_RejectsUnsafeTeamID(t *testing.T) {
	h := NewSplScheduleHandler(&mockSplScheduleClient{}, []string{"csm-agents"}, "")
	req := withUser(httptest.NewRequest(http.MethodGet, "/spl/abt-team-schedule?teamId=team%5E1", nil))
	w := httptest.NewRecorder()

	h.GetABTTeamSchedule(w, req)

	assertStatus(t, w, http.StatusBadRequest)
}

func TestGetABTTeamSchedule_RejectsUnauthorizedGroup(t *testing.T) {
	h := NewSplScheduleHandler(&mockSplScheduleClient{}, []string{"other-group"}, "")
	req := withUser(httptest.NewRequest(http.MethodGet, "/spl/abt-team-schedule", nil))
	w := httptest.NewRecorder()

	h.GetABTTeamSchedule(w, req)

	assertStatus(t, w, http.StatusForbidden)
}
