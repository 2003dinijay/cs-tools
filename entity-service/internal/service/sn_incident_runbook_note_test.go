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
	"strings"
	"sync"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// runbookNoteSNStub records the create body and every work note PATCHed onto the new incident.
type runbookNoteSNStub struct {
	mu          sync.Mutex
	createBody  map[string]any
	patchedNote []string
	patchStatus int
}

func (s *runbookNoteSNStub) handler(t *testing.T) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/services/search", snServicesStub(nil))
	mux.HandleFunc("/incidents", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		s.mu.Lock()
		_ = json.Unmarshal(body, &s.createBody)
		s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message": "Incident created successfully.",
			"incident": {"id": "` + testIncidentSysid + `", "number": "INC0001", "createdOn": "2026-01-01 00:00:00", "createdBy": "engineer@example.com"}}`))
	})
	mux.HandleFunc("/incidents/"+testIncidentSysid, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var body struct {
			WorkNotes *string `json:"workNotes"`
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		s.mu.Lock()
		if body.WorkNotes != nil {
			s.patchedNote = append(s.patchedNote, *body.WorkNotes)
		}
		status := s.patchStatus
		s.mu.Unlock()
		if status != 0 {
			w.WriteHeader(status)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message": "Incident updated successfully.", "incident": {"id": "` + testIncidentSysid + `"}}`))
	})
	return mux
}

func TestSNIncidentService_CreateIncident_RunbookNoteIsASeparateWorkNote(t *testing.T) {
	stub := &runbookNoteSNStub{}
	svc := WithRunbookCatalog(NewServiceNowIncidentService(newTestSNClient(t, stub.handler(t)), nil),
		fakeRunbookLister{runbooks: []domain.Runbook{{AlertName: "gw-5xx", RunbookURL: strp("https://docs.example.com/gw-5xx")}}})

	req := validCreateIncidentRequest()
	req.Subject = "Fired gw-5xx on prod"
	req.WorkNotes = strp("<p>Created by alert-core.</p>")
	if _, err := svc.CreateIncident(contextWithUserIDToken("token"), req); err != nil {
		t.Fatalf("CreateIncident: %v", err)
	}

	stub.mu.Lock()
	defer stub.mu.Unlock()
	if got, _ := stub.createBody["workNotes"].(string); strings.Contains(got, "Runbook") {
		t.Fatalf("create work notes carry the runbook note: %q", got)
	}
	if len(stub.patchedNote) != 1 || !strings.Contains(stub.patchedNote[0], `<a href="https://docs.example.com/gw-5xx"`) ||
		!strings.Contains(stub.patchedNote[0], ">Open runbook</a>") {
		t.Fatalf("separate runbook work note = %q, want one note with the Open runbook link", stub.patchedNote)
	}
}

func TestSNIncidentService_CreateIncident_RunbookNoteFailureDoesNotFailCreate(t *testing.T) {
	stub := &runbookNoteSNStub{patchStatus: http.StatusInternalServerError}
	svc := WithRunbookCatalog(NewServiceNowIncidentService(newTestSNClient(t, stub.handler(t)), nil),
		fakeRunbookLister{runbooks: []domain.Runbook{{AlertName: "gw-5xx", RunbookURL: strp("https://docs.example.com/gw-5xx")}}})

	req := validCreateIncidentRequest()
	req.Subject = "Fired gw-5xx on prod"
	resp, err := svc.CreateIncident(contextWithUserIDToken("token"), req)
	if err != nil {
		t.Fatalf("CreateIncident failed because of the runbook note: %v", err)
	}
	if resp.Incident.Number != "INC0001" {
		t.Fatalf("incident number = %q, want INC0001", resp.Incident.Number)
	}
}
