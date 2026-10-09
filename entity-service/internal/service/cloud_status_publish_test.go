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
	"encoding/json"
	"errors"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/events"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

func publishingSvc(repo *fakeCloudStatusRepo, pub *mockEventPublisher) CloudStatusService {
	return WithCloudStatusPublisher(NewCloudStatusService(repo, []string{testServiceID}), pub)
}

// A transition recorded straight after an outage write is published at once,
// already decided: dashboard slug, wire event and instant. It is leased, and
// left undelivered -- the consumer reports the outcome.
func TestHandleOutages_PublishesStatusPageDue(t *testing.T) {
	repo := &fakeCloudStatusRepo{candidates: []repository.CloudStatusCandidate{
		{OutageID: "o1", Number: "OUT0010021", Cloud: "CHOREO", Event: domain.CloudStatusEventOutageBegin, Timestamp: "2026-10-09T06:54:00.000Z"},
	}}
	pub := &mockEventPublisher{}

	if err := publishingSvc(repo, pub).HandleOutages(context.Background(), []string{"o1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	call, ok := findPublishCall(pub.calls, events.TypeOutageStatusPageDue)
	if !ok {
		t.Fatalf("want outage.status_page_due, got %v", publishedTypes(pub.calls))
	}
	if call.entityID != "o1" {
		t.Errorf("keyed by %q, want the outage id so begin and end stay ordered", call.entityID)
	}
	var p events.OutageStatusPageDuePayload
	if err := json.Unmarshal(call.payload, &p); err != nil {
		t.Fatal(err)
	}
	want := events.OutageStatusPageDuePayload{WebhookID: repo.claimed[0], OutageID: "o1", Number: "OUT0010021",
		Cloud: "choreo", Event: "outage_begin", Timestamp: "2026-10-09T06:54:00.000Z"}
	if p != want {
		t.Errorf("payload = %+v, want %+v", p, want)
	}
	if len(repo.deliveries) != 0 || len(repo.released) != 0 {
		t.Errorf("a published row stays leased for the consumer; got deliveries=%v released=%v", repo.deliveries, repo.released)
	}
}

// A row that cannot be published is handed straight back to the scheduled
// task, without an attempt counted against it: nothing was sent.
func TestHandleOutages_PublishFailureReleasesTheLease(t *testing.T) {
	repo := &fakeCloudStatusRepo{candidates: []repository.CloudStatusCandidate{
		{OutageID: "o1", Cloud: "CHOREO", Event: domain.CloudStatusEventOutageEnd, Timestamp: "2026-10-09T07:10:00.000Z"},
	}}
	pub := &mockEventPublisher{err: errors.New("broker down")}

	if err := publishingSvc(repo, pub).HandleOutages(context.Background(), []string{"o1"}); err != nil {
		t.Fatalf("a failed publish must not fail the outage write's follow-up: %v", err)
	}
	if len(repo.released) != 1 || repo.released[0] != repo.claimed[0] {
		t.Errorf("want the lease released, got released=%v claimed=%v", repo.released, repo.claimed)
	}
	if len(repo.deliveries) != 0 {
		t.Errorf("no attempt may be counted for an event never sent, got %v", repo.deliveries)
	}
}

// The unique key is still the send-once guarantee: an outage edited while
// ongoing does not publish its begin again.
func TestHandleOutages_AlreadyRecordedIsNotPublishedAgain(t *testing.T) {
	repo := &fakeCloudStatusRepo{
		candidates: []repository.CloudStatusCandidate{{OutageID: "o1", Cloud: "CHOREO", Event: domain.CloudStatusEventOutageBegin}},
		conflicts:  map[string]bool{"o1" + string(domain.CloudStatusEventOutageBegin): true},
	}
	pub := &mockEventPublisher{}

	if err := publishingSvc(repo, pub).HandleOutages(context.Background(), []string{"o1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pub.calls) != 0 {
		t.Errorf("want nothing published, got %v", publishedTypes(pub.calls))
	}
}

// The sweep only records: csm-scheduled-tasks calls it and posts the pending
// rows itself.
func TestSweep_NeverPublishes(t *testing.T) {
	repo := &fakeCloudStatusRepo{candidates: []repository.CloudStatusCandidate{
		{OutageID: "o1", Cloud: "CHOREO", Event: domain.CloudStatusEventOutageBegin},
	}}
	pub := &mockEventPublisher{}

	if _, err := publishingSvc(repo, pub).Sweep(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pub.calls) != 0 || len(repo.claimed) != 0 {
		t.Errorf("the sweep must not publish or lease, got calls=%v claimed=%v", publishedTypes(pub.calls), repo.claimed)
	}
}

type stubOutageService struct {
	OutageService
	createErr error
}

func (s *stubOutageService) CreateOutage(context.Context, domain.CreateOutageRequest) (domain.CreateOutageResponse, error) {
	if s.createErr != nil {
		return domain.CreateOutageResponse{}, s.createErr
	}
	return domain.CreateOutageResponse{Outage: domain.Outage{ID: "new-outage"}}, nil
}

func (s *stubOutageService) UpdateOutage(context.Context, domain.PatchOutageRequest) (domain.PatchOutageResponse, error) {
	return domain.PatchOutageResponse{}, nil
}

type recordingCloudStatus struct {
	CloudStatusService
	handled [][]string
}

func (r *recordingCloudStatus) HandleOutages(_ context.Context, ids []string) error {
	r.handled = append(r.handled, ids)
	return nil
}

// Declaring and ending an outage both reach the status page from the write
// itself; a failed write reaches nothing.
func TestOutageCloudStatus_WritesHandOverTheOutage(t *testing.T) {
	cs := &recordingCloudStatus{}
	svc := WithOutageCloudStatus(&stubOutageService{}, cs)

	if _, err := svc.CreateOutage(context.Background(), domain.CreateOutageRequest{}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpdateOutage(context.Background(), domain.PatchOutageRequest{ID: "existing"}); err != nil {
		t.Fatal(err)
	}
	if len(cs.handled) != 2 || cs.handled[0][0] != "new-outage" || cs.handled[1][0] != "existing" {
		t.Errorf("handled = %v, want [[new-outage] [existing]]", cs.handled)
	}

	cs.handled = nil
	failing := WithOutageCloudStatus(&stubOutageService{createErr: errors.New("boom")}, cs)
	if _, err := failing.CreateOutage(context.Background(), domain.CreateOutageRequest{}); err == nil {
		t.Fatal("the write's own error must come back")
	}
	if len(cs.handled) != 0 {
		t.Errorf("a failed write must not reach the status page, got %v", cs.handled)
	}
}
