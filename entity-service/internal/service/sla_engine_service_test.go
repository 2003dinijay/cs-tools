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
	"fmt"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// recordingSLAEngineRepo is an in-memory repository.SLAEngineRepository
// fake that records every call, backed by the same fixed policy set
// fakePolicyLookupRepo uses (sla_policy_resolver_test.go), so
// SLAEngineService tests exercise the real resolver, not a stub.
type recordingSLAEngineRepo struct {
	fakePolicyLookupRepo
	registered []string // "workItemID|policyID"
	revised    []string // "workItemID|target|policyID"
	completed  []string // "workItemID|target"
	paused     []string // "workItemID|target|true" or "...|false"

	registerErr error
	registerOK  bool // if false, RegisterClock reports "already registered"

	reviseErr error
	// reviseOK, keyed by "workItemID|target", reports whether RevisePolicy
	// found an existing active row to update for that pair -- absent
	// entries default to false (nothing to revise), matching a clock type
	// that was never registered under the case's old severity.
	reviseOK map[string]bool

	everExistedErr error
	// everExisted, keyed by "workItemID|target", simulates ClockEverExisted's
	// answer for that pair -- absent entries default to false ("truly never
	// existed", the newly-applicable-clock-type case), while a true entry
	// simulates "a clock of this type was registered before but has since
	// reached a terminal stage" (the resurrection bug this fix guards
	// against).
	everExisted map[string]bool
}

func newRecordingSLAEngineRepo() *recordingSLAEngineRepo {
	return &recordingSLAEngineRepo{fakePolicyLookupRepo: *newFakePolicyLookupRepo(), registerOK: true, reviseOK: map[string]bool{}, everExisted: map[string]bool{}}
}

func (r *recordingSLAEngineRepo) RegisterClock(_ context.Context, workItemID string, policy repository.SLAPolicyRef) (bool, error) {
	if r.registerErr != nil {
		return false, r.registerErr
	}
	r.registered = append(r.registered, workItemID+"|"+policy.ID)
	return r.registerOK, nil
}

func (r *recordingSLAEngineRepo) RevisePolicy(_ context.Context, workItemID string, policy repository.SLAPolicyRef) (bool, error) {
	if r.reviseErr != nil {
		return false, r.reviseErr
	}
	r.revised = append(r.revised, workItemID+"|"+policy.Target+"|"+policy.ID)
	return r.reviseOK[workItemID+"|"+policy.Target], nil
}

func (r *recordingSLAEngineRepo) ClockEverExisted(_ context.Context, workItemID, target string) (bool, error) {
	if r.everExistedErr != nil {
		return false, r.everExistedErr
	}
	return r.everExisted[workItemID+"|"+target], nil
}

func (r *recordingSLAEngineRepo) CompleteClock(_ context.Context, workItemID, target string) (bool, error) {
	r.completed = append(r.completed, workItemID+"|"+target)
	return true, nil
}

func (r *recordingSLAEngineRepo) SetPaused(_ context.Context, workItemID, target string, paused bool) (bool, error) {
	r.paused = append(r.paused, fmt.Sprintf("%s|%s|%v", workItemID, target, paused))
	return true, nil
}

func TestSLAEngineService_RegisterCaseClocks_CatastrophicRegistersAllThree(t *testing.T) {
	repo := newRecordingSLAEngineRepo()
	svc := NewSLAEngineService(repo, nil)

	sev := domain.CaseSeverityCatastrophic
	svc.RegisterCaseClocks(context.Background(), "case-1", &sev, "")

	// plan defaults to Open Source (nil projectSvc) but P0 only exists
	// under Managed Services -- the resolver's own cross-plan fallback
	// must still find all three P0 policies.
	want := []string{"case-1|p0-r-ms", "case-1|p0-w-ms", "case-1|p0-res-ms"}
	if len(repo.registered) != len(want) {
		t.Fatalf("registered = %v, want %v", repo.registered, want)
	}
	for i, w := range want {
		if repo.registered[i] != w {
			t.Errorf("registered[%d] = %q, want %q", i, repo.registered[i], w)
		}
	}
}

func TestSLAEngineService_RegisterCaseClocks_LowSeverityRegistersResponseOnly(t *testing.T) {
	repo := newRecordingSLAEngineRepo()
	svc := NewSLAEngineService(repo, nil)

	sev := domain.CaseSeverityLow
	svc.RegisterCaseClocks(context.Background(), "case-2", &sev, "")

	if len(repo.registered) != 1 || repo.registered[0] != "case-2|q-r-os" {
		t.Errorf("registered = %v, want exactly [case-2|q-r-os] (Query/LOW: response only, matching the old sla_clocks design's LOW-severity behavior)", repo.registered)
	}
}

func TestSLAEngineService_RegisterCaseClocks_NilSeverityRegistersNothing(t *testing.T) {
	repo := newRecordingSLAEngineRepo()
	svc := NewSLAEngineService(repo, nil)

	svc.RegisterCaseClocks(context.Background(), "case-3", nil, "")

	if len(repo.registered) != 0 {
		t.Errorf("registered = %v, want none for a case with no severity", repo.registered)
	}
}

func TestSLAEngineService_RegisterCaseClocks_MissingPolicySkipsThatClockTypeOnly(t *testing.T) {
	repo := newRecordingSLAEngineRepo()
	// Remove the workaround policy so CATASTROPHIC's middle clock type has
	// nothing to resolve -- response and resolution must still register.
	delete(repo.policies, "P0 - Workaround (Managed Services)|WORKAROUND")
	svc := NewSLAEngineService(repo, nil)

	sev := domain.CaseSeverityCatastrophic
	svc.RegisterCaseClocks(context.Background(), "case-4", &sev, "")

	want := []string{"case-4|p0-r-ms", "case-4|p0-res-ms"}
	if len(repo.registered) != len(want) {
		t.Fatalf("registered = %v, want %v", repo.registered, want)
	}
}

func TestSLAEngineService_CompleteResponseClock(t *testing.T) {
	repo := newRecordingSLAEngineRepo()
	svc := NewSLAEngineService(repo, nil)

	svc.CompleteResponseClock(context.Background(), "case-5")

	if len(repo.completed) != 1 || repo.completed[0] != "case-5|RESPONSE" {
		t.Errorf("completed = %v, want [case-5|RESPONSE]", repo.completed)
	}
}

func TestSLAEngineService_ApplyCaseStateEffects(t *testing.T) {
	tests := []struct {
		name          string
		state         domain.CaseState
		wantPaused    []string
		wantCompleted []string
	}{
		{
			"awaiting info pauses both",
			domain.CaseStateAwaitingInfo,
			[]string{"case-6|WORKAROUND|true", "case-6|RESOLUTION|true"},
			nil,
		},
		{
			"solution proposed pauses both",
			domain.CaseStateSolutionProposed,
			[]string{"case-6|WORKAROUND|true", "case-6|RESOLUTION|true"},
			nil,
		},
		{
			"closed resumes+completes resolution, pauses workaround only",
			domain.CaseStateClosed,
			[]string{"case-6|RESOLUTION|false", "case-6|WORKAROUND|true"},
			[]string{"case-6|RESOLUTION"},
		},
		{
			"work in progress resumes both",
			domain.CaseStateWorkInProgress,
			[]string{"case-6|WORKAROUND|false", "case-6|RESOLUTION|false"},
			nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newRecordingSLAEngineRepo()
			svc := NewSLAEngineService(repo, nil)

			svc.ApplyCaseStateEffects(context.Background(), "case-6", tt.state)

			if len(repo.paused) != len(tt.wantPaused) {
				t.Fatalf("paused = %v, want %v", repo.paused, tt.wantPaused)
			}
			for i, w := range tt.wantPaused {
				if repo.paused[i] != w {
					t.Errorf("paused[%d] = %q, want %q", i, repo.paused[i], w)
				}
			}
			if len(repo.completed) != len(tt.wantCompleted) {
				t.Fatalf("completed = %v, want %v", repo.completed, tt.wantCompleted)
			}
			for i, w := range tt.wantCompleted {
				if repo.completed[i] != w {
					t.Errorf("completed[%d] = %q, want %q", i, repo.completed[i], w)
				}
			}
		})
	}
}

// TestSLAEngineService_ReviseCaseClocks_RevisesExistingClockInPlace is this
// fix's core regression test: a case already has a response clock
// registered (simulating a case created at one severity), its severity then
// changes, and ReviseCaseClocks must revise that existing clock's policy in
// place -- not register a duplicate alongside it (RegisterClock is never
// called at all here, since RevisePolicy reports a row was found for every
// clock type CATASTROPHIC applies to).
func TestSLAEngineService_ReviseCaseClocks_RevisesExistingClockInPlace(t *testing.T) {
	repo := newRecordingSLAEngineRepo()
	// Every P0 clock type already has an active row for this case (as if
	// RegisterCaseClocks had run earlier under a different severity).
	repo.reviseOK["case-8|RESPONSE"] = true
	repo.reviseOK["case-8|WORKAROUND"] = true
	repo.reviseOK["case-8|RESOLUTION"] = true
	svc := NewSLAEngineService(repo, nil)

	sev := domain.CaseSeverityCatastrophic
	svc.ReviseCaseClocks(context.Background(), "case-8", &sev, "")

	wantRevised := []string{"case-8|RESPONSE|p0-r-ms", "case-8|WORKAROUND|p0-w-ms", "case-8|RESOLUTION|p0-res-ms"}
	if len(repo.revised) != len(wantRevised) {
		t.Fatalf("revised = %v, want %v", repo.revised, wantRevised)
	}
	for i, w := range wantRevised {
		if repo.revised[i] != w {
			t.Errorf("revised[%d] = %q, want %q", i, repo.revised[i], w)
		}
	}
	if len(repo.registered) != 0 {
		t.Errorf("registered = %v, want none -- every clock type already had an active row to revise", repo.registered)
	}
}

// TestSLAEngineService_ReviseCaseClocks_FallsBackToRegisterForNewlyApplicableType
// covers a severity INCREASE: the case was created at LOW (response clock
// only) and is revised up to CATASTROPHIC, which also applies "workaround"/
// "resolution" -- clock types that were never registered before. RevisePolicy
// reports no existing row for those two (reviseOK unset, defaults false), so
// ReviseCaseClocks must fall back to RegisterClock for them, while still
// revising the pre-existing response clock in place.
func TestSLAEngineService_ReviseCaseClocks_FallsBackToRegisterForNewlyApplicableType(t *testing.T) {
	repo := newRecordingSLAEngineRepo()
	repo.reviseOK["case-9|RESPONSE"] = true // the only clock type LOW ever registered

	svc := NewSLAEngineService(repo, nil)

	sev := domain.CaseSeverityCatastrophic
	svc.ReviseCaseClocks(context.Background(), "case-9", &sev, "")

	// RevisePolicy is attempted for every applicable clock type regardless
	// of outcome (that's how the fallback is even discovered) -- only
	// RESPONSE actually finds an existing row to update; the other two
	// attempts report "nothing to revise" and fall back to RegisterClock
	// below.
	wantRevised := []string{"case-9|RESPONSE|p0-r-ms", "case-9|WORKAROUND|p0-w-ms", "case-9|RESOLUTION|p0-res-ms"}
	if len(repo.revised) != len(wantRevised) {
		t.Fatalf("revised = %v, want %v", repo.revised, wantRevised)
	}
	for i, w := range wantRevised {
		if repo.revised[i] != w {
			t.Errorf("revised[%d] = %q, want %q", i, repo.revised[i], w)
		}
	}
	wantRegistered := []string{"case-9|p0-w-ms", "case-9|p0-res-ms"}
	if len(repo.registered) != len(wantRegistered) {
		t.Fatalf("registered = %v, want %v", repo.registered, wantRegistered)
	}
	for i, w := range wantRegistered {
		if repo.registered[i] != w {
			t.Errorf("registered[%d] = %q, want %q", i, repo.registered[i], w)
		}
	}
}

// TestSLAEngineService_ReviseCaseClocks_DoesNotResurrectTerminalClock is the
// regression test for this fix: a case's response clock was already marked
// terminal (e.g. ACHIEVED by CompleteResponseClock once the first reply went
// out), so RevisePolicy correctly reports "nothing to revise" for RESPONSE
// (no ACTIVE row exists any more) -- but that must NOT be read as "RESPONSE
// was never applicable," which would incorrectly resurrect it via
// RegisterClock. ClockEverExisted reporting true for RESPONSE is what tells
// ReviseCaseClocks to leave it alone. Meanwhile a genuinely new clock type
// for the same call (WORKAROUND/RESOLUTION, ClockEverExisted false -- truly
// never existed) must still correctly fall back to RegisterClock, same as
// TestSLAEngineService_ReviseCaseClocks_FallsBackToRegisterForNewlyApplicableType.
func TestSLAEngineService_ReviseCaseClocks_DoesNotResurrectTerminalClock(t *testing.T) {
	repo := newRecordingSLAEngineRepo()
	// RESPONSE has no active row to revise (already terminal, not merely
	// "never registered") -- reviseOK left at its default false, but
	// everExisted true marks it as "existed before, now terminal."
	repo.everExisted["case-11|RESPONSE"] = true
	// WORKAROUND/RESOLUTION are truly new: reviseOK and everExisted both
	// left at their default false.

	svc := NewSLAEngineService(repo, nil)

	sev := domain.CaseSeverityCatastrophic
	svc.ReviseCaseClocks(context.Background(), "case-11", &sev, "")

	// RevisePolicy is still attempted for every applicable clock type
	// regardless of outcome, same as the sibling test above.
	wantRevised := []string{"case-11|RESPONSE|p0-r-ms", "case-11|WORKAROUND|p0-w-ms", "case-11|RESOLUTION|p0-res-ms"}
	if len(repo.revised) != len(wantRevised) {
		t.Fatalf("revised = %v, want %v", repo.revised, wantRevised)
	}
	for i, w := range wantRevised {
		if repo.revised[i] != w {
			t.Errorf("revised[%d] = %q, want %q", i, repo.revised[i], w)
		}
	}

	// RESPONSE must NOT be registered (it's terminal, not newly applicable);
	// WORKAROUND/RESOLUTION must still register, exactly as the
	// newly-applicable-clock-type path already does.
	wantRegistered := []string{"case-11|p0-w-ms", "case-11|p0-res-ms"}
	if len(repo.registered) != len(wantRegistered) {
		t.Fatalf("registered = %v, want %v -- a terminal RESPONSE clock must not be resurrected", repo.registered, wantRegistered)
	}
	for i, w := range wantRegistered {
		if repo.registered[i] != w {
			t.Errorf("registered[%d] = %q, want %q", i, repo.registered[i], w)
		}
	}
	for _, r := range repo.registered {
		if r == "case-11|p0-r-ms" {
			t.Fatalf("registered = %v, want no RESPONSE registration -- a completed/terminal clock was incorrectly resurrected", repo.registered)
		}
	}
}

// TestSLAEngineService_ReviseCaseClocks_NilSeverityRevisesNothing mirrors
// RegisterCaseClocks' own nil-severity handling.
func TestSLAEngineService_ReviseCaseClocks_NilSeverityRevisesNothing(t *testing.T) {
	repo := newRecordingSLAEngineRepo()
	svc := NewSLAEngineService(repo, nil)

	svc.ReviseCaseClocks(context.Background(), "case-10", nil, "")

	if len(repo.revised) != 0 || len(repo.registered) != 0 {
		t.Errorf("revised = %v, registered = %v, want none for a case with no severity", repo.revised, repo.registered)
	}
}

// TestSLAEngineService_RegisterCaseClocks_UsesResolvedPlan confirms
// RegisterCaseClocks actually threads the project-derived plan through to
// the resolver (rather than always defaulting) for a non-P0 severity,
// where the plan choice actually changes which policy matches.
func TestSLAEngineService_RegisterCaseClocks_UsesResolvedPlan(t *testing.T) {
	repo := newRecordingSLAEngineRepo()
	projectSvc := fakeProjectSvc{project: domain.ProjectDetailsView{SubscriptionType: domain.SubscriptionTypeManagedCloudSubscription}}
	svc := NewSLAEngineService(repo, projectSvc)

	sev := domain.CaseSeverityMedium // P3
	svc.RegisterCaseClocks(context.Background(), "case-7", &sev, "proj-1")

	// Only P3 - Resolution (Managed Services) is faked; response/workaround
	// for P3 aren't in the fake set at all, so only resolution registers.
	if len(repo.registered) != 1 || repo.registered[0] != "case-7|p3-res-ms" {
		t.Errorf("registered = %v, want [case-7|p3-res-ms]", repo.registered)
	}
}
