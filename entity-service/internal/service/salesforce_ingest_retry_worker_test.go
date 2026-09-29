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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package service

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

type fakeReingester struct {
	mu    sync.Mutex
	calls []string
	errs  map[string]error
}

func (f *fakeReingester) RetryMembershipIngest(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, id)
	return f.errs[id]
}

func (f *fakeReingester) called() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string{}, f.calls...)
}

func failedDatabaseStep(membershipSfID, lastError string, attempts int) domain.OnboardingStep {
	return domain.OnboardingStep{
		MembershipSfID: membershipSfID, Step: domain.OnboardingStepDatabase, Status: domain.OnboardingStepFailed,
		LastError: sampleStr(lastError), AttemptCount: attempts, EventType: domain.SalesforceEventCreated,
		EventModifiedOn: time.Now().Add(-time.Hour), UpdatedOn: time.Now().Add(-time.Hour),
	}
}

func newRetryWorker(steps *fakeStepRepo, re *fakeReingester, states *fakeIngestStateRepo) *SalesforceIngestRetryWorker {
	w := NewSalesforceIngestRetryWorker(steps, re, nil, time.Minute)
	if states != nil {
		w.States = states
	}
	return w
}

// TestRetryWorker_ReRunsOnlyMissingParentFailures: of the FAILED DATABASE
// steps, only the ones whose error is a missing project / account and that
// are under the attempt cap are re-run; other failures are someone else's
// problem and SUCCEEDED rows are not touched.
func TestRetryWorker_ReRunsOnlyMissingParentFailures(t *testing.T) {
	steps := &fakeStepRepo{existing: []domain.OnboardingStep{
		failedDatabaseStep("m-project", `project not found for key "ACME" / sfId "a0p1"`, 3),
		failedDatabaseStep("m-account", `account not found for sfId "0011"`, 1),
		failedDatabaseStep("m-other", "project contact a0e1 has no email", 1),
		failedDatabaseStep("m-capped", `project not found for key "X" / sfId "Y"`, salesforceIngestRetryMaxAttempts),
		{MembershipSfID: "m-ok", Step: domain.OnboardingStepDatabase, Status: domain.OnboardingStepSucceeded},
		{MembershipSfID: "m-email", Step: domain.OnboardingStepEmail, Status: domain.OnboardingStepFailed, LastError: sampleStr("project not found")},
	}}
	re := &fakeReingester{errs: map[string]error{"m-account": errors.New("still not there")}}
	w := newRetryWorker(steps, re, nil)

	w.RunOnce(context.Background())

	if got, want := re.called(), []string{"m-project", "m-account"}; !reflect.DeepEqual(got, want) {
		t.Errorf("re-ran %v, want %v (a failing re-run must not stop the batch)", got, want)
	}
}

// TestRetryWorker_ListErrorIsLoggedNotFatal: a database error listing the
// steps ends the tick quietly; the next tick tries again.
func TestRetryWorker_ListErrorIsLoggedNotFatal(t *testing.T) {
	steps := &fakeStepRepo{listErr: errors.New("db down"), existing: []domain.OnboardingStep{failedDatabaseStep("m-1", "project not found", 1)}}
	re := &fakeReingester{}
	newRetryWorker(steps, re, nil).RunOnce(context.Background())
	if len(re.called()) != 0 {
		t.Errorf("re-ran %v, want nothing after a list error", re.called())
	}
}

// TestRetryWorker_NothingWiredIsNoOp: a worker without its membership
// dependencies does nothing rather than nil-pointer.
func TestRetryWorker_NothingWiredIsNoOp(t *testing.T) {
	w := NewSalesforceIngestRetryWorker(nil, nil, nil, time.Minute)
	w.RunOnce(context.Background())
}

// TestRetryWorker_Ledger: FAILED ledger rows go to the retrier registered for
// their entity, under the same missing-parent and attempt-cap filter; an
// entity with no retrier (nothing records into the ledger yet) is skipped.
func TestRetryWorker_Ledger(t *testing.T) {
	states := &fakeIngestStateRepo{failed: []domain.SalesforceIngestState{
		{Entity: "widget", SfID: "w-1", Status: domain.SalesforceIngestFailed, LastError: sampleStr(`account not found for sfId "0011"`), AttemptCount: 2},
		{Entity: "widget", SfID: "w-other", Status: domain.SalesforceIngestFailed, LastError: sampleStr("customer is missing Name"), AttemptCount: 1},
		{Entity: "widget", SfID: "w-capped", Status: domain.SalesforceIngestFailed, LastError: sampleStr("project not found"), AttemptCount: salesforceIngestRetryMaxAttempts},
		{Entity: domain.SalesforceIngestEntityAccount, SfID: "no-retrier", Status: domain.SalesforceIngestFailed, LastError: sampleStr("project not found"), AttemptCount: 1},
	}}
	var retried []string
	w := newRetryWorker(&fakeStepRepo{}, &fakeReingester{}, states)
	w.EntityRetriers["widget"] = func(_ context.Context, sfID string) error {
		retried = append(retried, sfID)
		return nil
	}

	w.RunOnce(context.Background())

	if want := []string{"w-1"}; !reflect.DeepEqual(retried, want) {
		t.Errorf("retried %v, want %v", retried, want)
	}
}

// TestRetryWorker_LedgerListError mirrors the step case for the ledger.
func TestRetryWorker_LedgerListError(t *testing.T) {
	states := &fakeIngestStateRepo{listErr: errors.New("db down")}
	w := newRetryWorker(&fakeStepRepo{}, &fakeReingester{}, states)
	called := false
	w.EntityRetriers["widget"] = func(context.Context, string) error { called = true; return nil }
	w.RunOnce(context.Background())
	if called {
		t.Error("retrier called after a list error")
	}
}

// TestRetryWorker_RunStopsOnCancel: the loop waits one interval before its
// first tick, ticks, and returns when the context is cancelled.
func TestRetryWorker_RunStopsOnCancel(t *testing.T) {
	steps := &fakeStepRepo{existing: []domain.OnboardingStep{failedDatabaseStep("m-1", "project not found", 1)}}
	re := &fakeReingester{}
	w := newRetryWorker(steps, re, nil)
	w.Interval = 5 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { w.Run(ctx); close(done) }()

	deadline := time.Now().Add(2 * time.Second)
	for len(re.called()) == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after cancel")
	}
	if len(re.called()) == 0 {
		t.Error("no tick ran before cancel")
	}
}

// TestRetryMembershipIngest_ReRunsAsUpdated: the service's re-run is the
// ordinary ingest with UPDATED as the event type — the FAILED step does not
// block it — and a service built without the membership ingest refuses.
func TestRetryMembershipIngest_ReRunsAsUpdated(t *testing.T) {
	h := newIngestHarness(sampleProjectContact(domain.MembershipStateInvited, "Portal user"), sampleContact(), false)
	h.steps.existing = []domain.OnboardingStep{failedDatabaseStep(testMembershipID, `project not found for key "ACMEPROD" / sfId "a0p1"`, 2)}

	if err := h.svc.(*salesforceEventService).RetryMembershipIngest(context.Background(), testMembershipID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(h.repo.steps) != 1 || h.repo.steps[0].EventType != domain.SalesforceEventUpdated || h.repo.steps[0].Status != domain.OnboardingStepSucceeded {
		t.Errorf("recorded steps = %+v, want one SUCCEEDED step stamped UPDATED", h.repo.steps)
	}
	if len(h.repo.upserts) != 1 || h.repo.upserts[0].MembershipSfID != testMembershipID {
		t.Errorf("upserts = %+v, want the one membership", h.repo.upserts)
	}

	off := NewSalesforceEventService(&stubSalesforceAccountRepo{}, &stubSalesEntityClient{}, SalesforceIngestSupport{}).(*salesforceEventService)
	if err := off.RetryMembershipIngest(context.Background(), testMembershipID); !errors.Is(err, errMembershipIngestDisabled) {
		t.Errorf("disabled service: err = %v, want errMembershipIngestDisabled", err)
	}
}
