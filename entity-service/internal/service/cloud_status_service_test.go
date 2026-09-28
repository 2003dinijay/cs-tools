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
	"errors"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// fakeCloudStatusRepo records what the service asked it to do.
type fakeCloudStatusRepo struct {
	candidates []repository.CloudStatusCandidate
	candErr    error

	recorded  []repository.CloudStatusCandidate
	conflicts map[string]bool // key -> already present, so Record reports false

	monitors     map[string][]string
	monitorsErr  error
	statusWrites []statusWrite

	pending    []domain.PendingCloudStatusWebhook
	pendingErr error

	deliveries []struct {
		id        string
		delivered bool
		errMsg    string
	}
}

func (f *fakeCloudStatusRepo) Candidates(_ context.Context, _ []string) ([]repository.CloudStatusCandidate, error) {
	return f.candidates, f.candErr
}

func (f *fakeCloudStatusRepo) Record(_ context.Context, c repository.CloudStatusCandidate) (bool, error) {
	f.recorded = append(f.recorded, c)
	if f.conflicts[c.OutageID+string(c.Event)] {
		return false, nil
	}
	return true, nil
}

func (f *fakeCloudStatusRepo) AffectedMonitors(_ context.Context, outageID string) ([]string, error) {
	return f.monitors[outageID], f.monitorsErr
}

func (f *fakeCloudStatusRepo) SetMonitorStatus(_ context.Context, ids []string, status domain.CloudMonitorStatus) (int64, error) {
	f.statusWrites = append(f.statusWrites, statusWrite{ids: ids, status: status})
	return int64(len(ids)), nil
}

func (f *fakeCloudStatusRepo) Pending(_ context.Context, _, _ int) ([]domain.PendingCloudStatusWebhook, error) {
	return f.pending, f.pendingErr
}

func (f *fakeCloudStatusRepo) RecordDelivery(_ context.Context, id string, delivered bool, errMsg string) error {
	f.deliveries = append(f.deliveries, struct {
		id        string
		delivered bool
		errMsg    string
	}{id, delivered, errMsg})
	return nil
}

type statusWrite struct {
	ids    []string
	status domain.CloudMonitorStatus
}

const testServiceID = "11111111-1111-1111-1111-111111111111"

// TestCloudStatusSweep_RecordsRoutableTransitions is the ordinary path: two
// in-scope outages, both routable, both new.
func TestCloudStatusSweep_RecordsRoutableTransitions(t *testing.T) {
	repo := &fakeCloudStatusRepo{candidates: []repository.CloudStatusCandidate{
		{OutageID: "a", Number: "OUT001", Cloud: "CHOREO", Event: domain.CloudStatusEventOutageBegin, Timestamp: "2026-09-28T10:00:00Z"},
		{OutageID: "b", Number: "OUT002", Cloud: "ASGARDEO", Event: domain.CloudStatusEventOutageEnd, Timestamp: "2026-09-28T11:00:00Z"},
	}}
	svc := NewCloudStatusService(repo, []string{testServiceID})

	got, err := svc.Sweep(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Scanned != 2 || got.Recorded != 2 || got.SkippedNoCloud != 0 {
		t.Errorf("got %+v, want scanned 2 recorded 2 skipped 0", got)
	}
}

// TestCloudStatusSweep_AlreadyRecordedIsNotCountedAgain is the steady state:
// the sweep runs on a schedule and mostly finds work it has already done.
//
// This is the behaviour that replaces ServiceNow's re-posting. There, every
// update to an ongoing outage re-fired the flow and posted another identical
// begin webhook; here the second sweep records nothing.
func TestCloudStatusSweep_AlreadyRecordedIsNotCountedAgain(t *testing.T) {
	repo := &fakeCloudStatusRepo{
		candidates: []repository.CloudStatusCandidate{
			{OutageID: "a", Cloud: "CHOREO", Event: domain.CloudStatusEventOutageBegin},
		},
		conflicts: map[string]bool{"a" + string(domain.CloudStatusEventOutageBegin): true},
	}
	svc := NewCloudStatusService(repo, []string{testServiceID})

	got, err := svc.Sweep(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Scanned != 1 {
		t.Errorf("scanned: got %d, want 1", got.Scanned)
	}
	if got.Recorded != 0 {
		t.Errorf("recorded: got %d, want 0 -- a repeat sweep must record nothing", got.Recorded)
	}
}

// TestCloudStatusSweep_SkipsUnroutableAndKeepsGoing covers the divergence from
// ServiceNow's Look Up Record step, which failed the whole execution when the
// monitor was missing. The sweep handles many outages per run, so one
// unroutable record must not stop the rest -- but it must be counted.
func TestCloudStatusSweep_SkipsUnroutableAndKeepsGoing(t *testing.T) {
	repo := &fakeCloudStatusRepo{candidates: []repository.CloudStatusCandidate{
		{OutageID: "a", Cloud: "", Event: domain.CloudStatusEventOutageBegin},            // no monitor at all
		{OutageID: "b", Cloud: "NOT_A_CLOUD", Event: domain.CloudStatusEventOutageBegin}, // unknown enum value
		{OutageID: "c", Cloud: "DEVANT", Event: domain.CloudStatusEventOutageBegin},
	}}
	svc := NewCloudStatusService(repo, []string{testServiceID})

	got, err := svc.Sweep(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.SkippedNoCloud != 2 {
		t.Errorf("skippedNoCloud: got %d, want 2", got.SkippedNoCloud)
	}
	if got.Recorded != 1 {
		t.Errorf("recorded: got %d, want 1 -- the routable outage must still be recorded", got.Recorded)
	}
	if len(repo.recorded) != 1 || repo.recorded[0].OutageID != "c" {
		t.Errorf("wrong outage recorded: %+v", repo.recorded)
	}
}

// TestCloudStatusSweep_NoConfiguredScopeIsANoOp guards the safe default. An
// unconfigured deployment must post nothing to a public status page.
func TestCloudStatusSweep_NoConfiguredScopeIsANoOp(t *testing.T) {
	repo := &fakeCloudStatusRepo{candidates: []repository.CloudStatusCandidate{
		{OutageID: "a", Cloud: "CHOREO", Event: domain.CloudStatusEventOutageBegin},
	}}
	svc := NewCloudStatusService(repo, nil)

	got, err := svc.Sweep(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Scanned != 0 || got.Recorded != 0 {
		t.Errorf("got %+v, want an empty sweep", got)
	}
	if len(repo.recorded) != 0 {
		t.Errorf("recorded %d transitions with no scope configured", len(repo.recorded))
	}
}

// TestCloudOfferingSlug_CoversEveryStoredEnumValue is the regression guard for
// Defect 1.
//
// The list here is cloud_monitor_cloud_offering_enum from csm-sync-service
// migration 0079, in full. ServiceNow's script covered five of these seven and
// returned undefined for the other two. If the sync ever adds an eighth value,
// this test is what should fail -- a missing slug means a cloud whose outages
// are silently never published.
func TestCloudOfferingSlug_CoversEveryStoredEnumValue(t *testing.T) {
	want := map[string]string{
		"ASGARDEO":      "asgardeo",
		"BIJIRA":        "bijira",
		"CHOREO":        "choreo",
		"DEVANT":        "devant",
		"MOESIF":        "moesif",
		"CHOREO_EU":     "choreo-eu",
		"AGENT_MANAGER": "agent-manager",
	}
	for stored, slug := range want {
		if got := domain.CloudOfferingSlug(stored); got != slug {
			t.Errorf("CloudOfferingSlug(%q) = %q, want %q", stored, got, slug)
		}
	}
	if got := domain.CloudOfferingSlug("SOMETHING_NEW"); got != "" {
		t.Errorf("an unknown offering must be unroutable, got %q", got)
	}
}

// TestCloudStatusEventWireValues pins the strings that go on the wire. The
// dashboard switches on these, so a rename in Go must not change them.
func TestCloudStatusEventWireValues(t *testing.T) {
	if got := domain.CloudStatusEventOutageBegin.WireValue(); got != "outage_begin" {
		t.Errorf("begin wire value: got %q, want %q (confirmed from the flow's step 15)", got, "outage_begin")
	}
	// NOTE: "outage_end" is INFERRED, not confirmed -- step 8's Event input
	// was never captured. This test pins what the port currently sends so the
	// value is changed deliberately, not so it is known to be right.
	if got := domain.CloudStatusEventOutageEnd.WireValue(); got != "outage_end" {
		t.Errorf("end wire value: got %q, want %q", got, "outage_end")
	}
}

// TestCloudStatusPendingWebhooks_TranslatesCloudToSlug checks the dashboard
// sees slugs, never the stored enum spelling.
func TestCloudStatusPendingWebhooks_TranslatesCloudToSlug(t *testing.T) {
	repo := &fakeCloudStatusRepo{pending: []domain.PendingCloudStatusWebhook{
		{ID: "1", Cloud: "CHOREO_EU", Event: domain.CloudStatusEventOutageBegin},
		{ID: "2", Cloud: "UNKNOWN", Event: domain.CloudStatusEventOutageBegin},
	}}
	svc := NewCloudStatusService(repo, []string{testServiceID})

	got, err := svc.PendingWebhooks(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Count != 1 {
		t.Fatalf("count: got %d, want 1 -- the unmappable row must not be dispatched", got.Count)
	}
	if got.Webhooks[0].Cloud != "choreo-eu" {
		t.Errorf("cloud: got %q, want %q", got.Webhooks[0].Cloud, "choreo-eu")
	}
}

// TestCloudStatusRecordDelivery_FailureNeedsAReason keeps the failure path
// from recording a silent nothing.
func TestCloudStatusRecordDelivery_FailureNeedsAReason(t *testing.T) {
	repo := &fakeCloudStatusRepo{}
	svc := NewCloudStatusService(repo, []string{testServiceID})

	err := svc.RecordDelivery(context.Background(), domain.RecordCloudStatusDeliveryRequest{
		ID: testServiceID, Delivered: false,
	})
	var verr *apierror.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("got %v, want a ValidationError", err)
	}
	if len(repo.deliveries) != 0 {
		t.Errorf("a rejected report must not reach the repository")
	}
}

// TestCloudStatusRecordDelivery_SuccessNeedsNoReason is the companion.
func TestCloudStatusRecordDelivery_SuccessNeedsNoReason(t *testing.T) {
	repo := &fakeCloudStatusRepo{}
	svc := NewCloudStatusService(repo, []string{testServiceID})

	if err := svc.RecordDelivery(context.Background(), domain.RecordCloudStatusDeliveryRequest{
		ID: testServiceID, Delivered: true,
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.deliveries) != 1 || !repo.deliveries[0].delivered {
		t.Errorf("delivery not recorded: %+v", repo.deliveries)
	}
}

// TestCloudStatusSweep_OngoingOutageSetsSeverityByType is the ongoing arm's
// mapping, straight from the flow's script. Note that type "outage" produces
// PARTIAL_OUTAGE, not MAJOR_OUTAGE -- that is what the flow did.
func TestCloudStatusSweep_OngoingOutageSetsSeverityByType(t *testing.T) {
	cases := []struct {
		outageType string
		want       domain.CloudMonitorStatus
	}{
		{"PLANNED", domain.CloudMonitorStatusMaintenance},
		{"DEGRADATION", domain.CloudMonitorStatusDegraded},
		{"OUTAGE", domain.CloudMonitorStatusPartialOutage},
	}
	for _, tc := range cases {
		t.Run(tc.outageType, func(t *testing.T) {
			repo := &fakeCloudStatusRepo{
				candidates: []repository.CloudStatusCandidate{{
					OutageID: "o1", Cloud: "CHOREO",
					Event: domain.CloudStatusEventOutageBegin, Type: tc.outageType,
				}},
				monitors: map[string][]string{"o1": {"m1", "m2"}},
			}
			svc := NewCloudStatusService(repo, []string{testServiceID})

			got, err := svc.Sweep(context.Background())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(repo.statusWrites) != 1 || repo.statusWrites[0].status != tc.want {
				t.Fatalf("status writes: %+v, want %s", repo.statusWrites, tc.want)
			}
			if got.MonitorsUpdated != 2 {
				t.Errorf("monitorsUpdated: got %d, want 2", got.MonitorsUpdated)
			}
			if got.UnknownOutageType != 0 {
				t.Errorf("a known type must not be counted as unknown")
			}
		})
	}
}

// TestCloudStatusSweep_EndedOutageReturnsMonitorsToOperational is the
// completed arm's literal 0.
func TestCloudStatusSweep_EndedOutageReturnsMonitorsToOperational(t *testing.T) {
	repo := &fakeCloudStatusRepo{
		candidates: []repository.CloudStatusCandidate{{
			OutageID: "o1", Cloud: "CHOREO",
			Event: domain.CloudStatusEventOutageEnd, Type: "OUTAGE",
		}},
		monitors: map[string][]string{"o1": {"m1"}},
	}
	svc := NewCloudStatusService(repo, []string{testServiceID})

	if _, err := svc.Sweep(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.statusWrites) != 1 {
		t.Fatalf("status writes: %+v", repo.statusWrites)
	}
	// The outage's own type is OUTAGE and must be ignored on this arm: a
	// completed outage is Operational regardless of what it was.
	if repo.statusWrites[0].status != domain.CloudMonitorStatusOperational {
		t.Errorf("status: got %s, want OPERATIONAL", repo.statusWrites[0].status)
	}
}

// TestCloudStatusSweep_MissingTypeFallsBackAndIsCounted covers the defect
// ServiceNow had: its script fell off the end and wrote undefined. A public
// status page must not be left asserting all is well during an incident.
func TestCloudStatusSweep_MissingTypeFallsBackAndIsCounted(t *testing.T) {
	repo := &fakeCloudStatusRepo{
		candidates: []repository.CloudStatusCandidate{{
			OutageID: "o1", Cloud: "CHOREO",
			Event: domain.CloudStatusEventOutageBegin, Type: "",
		}},
		monitors: map[string][]string{"o1": {"m1"}},
	}
	svc := NewCloudStatusService(repo, []string{testServiceID})

	got, err := svc.Sweep(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.UnknownOutageType != 1 {
		t.Errorf("unknownOutageType: got %d, want 1", got.UnknownOutageType)
	}
	if len(repo.statusWrites) != 1 || repo.statusWrites[0].status != domain.CloudMonitorStatusUnknownType {
		t.Fatalf("a missing type must still write a non-operational status: %+v", repo.statusWrites)
	}
	if repo.statusWrites[0].status == domain.CloudMonitorStatusOperational {
		t.Error("a missing type must never leave the page claiming Operational")
	}
}

// TestCloudStatusSweep_StatusIsReassertedEvenWhenAlreadyRecorded is the
// self-healing property: recording is once-only, the status is a desired end
// state and is re-asserted every sweep.
func TestCloudStatusSweep_StatusIsReassertedEvenWhenAlreadyRecorded(t *testing.T) {
	repo := &fakeCloudStatusRepo{
		candidates: []repository.CloudStatusCandidate{{
			OutageID: "o1", Cloud: "CHOREO",
			Event: domain.CloudStatusEventOutageBegin, Type: "OUTAGE",
		}},
		conflicts: map[string]bool{"o1" + string(domain.CloudStatusEventOutageBegin): true},
		monitors:  map[string][]string{"o1": {"m1"}},
	}
	svc := NewCloudStatusService(repo, []string{testServiceID})

	got, err := svc.Sweep(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Recorded != 0 {
		t.Errorf("the webhook must not be re-recorded, got %d", got.Recorded)
	}
	if len(repo.statusWrites) != 1 {
		t.Errorf("the status must still be re-asserted: %+v", repo.statusWrites)
	}
}

// TestCloudStatusSweep_NoAffectedMonitorsIsNotAnError: most outages name no
// affected CIs, and the webhook still goes out regardless.
func TestCloudStatusSweep_NoAffectedMonitorsIsNotAnError(t *testing.T) {
	repo := &fakeCloudStatusRepo{
		candidates: []repository.CloudStatusCandidate{{
			OutageID: "o1", Cloud: "CHOREO",
			Event: domain.CloudStatusEventOutageBegin, Type: "OUTAGE",
		}},
		monitors: map[string][]string{},
	}
	svc := NewCloudStatusService(repo, []string{testServiceID})

	got, err := svc.Sweep(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Recorded != 1 {
		t.Errorf("the webhook must still be recorded, got %d", got.Recorded)
	}
	if len(repo.statusWrites) != 0 {
		t.Errorf("nothing to write, got %+v", repo.statusWrites)
	}
}
