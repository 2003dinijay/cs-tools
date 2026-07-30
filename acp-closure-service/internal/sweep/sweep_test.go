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

package sweep

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/acp-closure-service/internal/notify"
)

func TestProcessProject_NoEndDateIsNoOp(t *testing.T) {
	reader := &mockEntityReader{}
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{}

	proj := project{ID: "p1", Account: &projectAccountRef{ID: "a1"}, EndDate: nil}

	err := processProject(context.Background(), reader, updater, ntf, time.Now(), proj)
	if err != nil {
		t.Fatalf("processProject() error = %v, want nil", err)
	}
	if len(updater.calls) != 0 {
		t.Errorf("updater.calls = %d, want 0", len(updater.calls))
	}
	if len(ntf.sent) != 0 {
		t.Errorf("ntf.sent = %d, want 0", len(ntf.sent))
	}
}

// TestProcessProject_InternalOnlyWindowSkipsCustomerContactLookup covers a
// 90-day window: internal-only per the confirmed audience matrix. Only one
// notify.Send should occur (internal), and no contact-search calls should
// happen at all, since the customer side isn't consulted for this window.
func TestProcessProject_InternalOnlyWindowSkipsCustomerContactLookup(t *testing.T) {
	reader := &mockEntityReader{
		searchProjectContactsFn: func(ctx context.Context, projectID string, body []byte) ([]byte, error) {
			t.Fatal("SearchProjectContacts should not be called for an internal-only window")
			return nil, nil
		},
		searchAccountContactsFn: func(ctx context.Context, accountID string, body []byte) ([]byte, error) {
			t.Fatal("SearchAccountContacts should not be called for an internal-only window")
			return nil, nil
		},
	}
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{}

	now := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	endDate := now.AddDate(0, 0, 89) // fires the 90-day window
	proj := project{ID: "p1", Account: &projectAccountRef{ID: "a1"}, EndDate: &endDate}

	err := processProject(context.Background(), reader, updater, ntf, now, proj)
	if err != nil {
		t.Fatalf("processProject() error = %v, want nil", err)
	}

	if len(ntf.sent) != 1 {
		t.Fatalf("ntf.sent = %d, want 1", len(ntf.sent))
	}
	if ntf.sent[0].Kind != notify.KindInternal {
		t.Errorf("notice.Kind = %v, want %v", ntf.sent[0].Kind, notify.KindInternal)
	}

	if len(updater.calls) != 1 {
		t.Fatalf("updater.calls = %d, want 1", len(updater.calls))
	}
	var body struct {
		SuspensionProcessState struct {
			BasedOnSubscriptionEndDate struct {
				EventType string `json:"event_type"`
			} `json:"based_on_subscription_end_date"`
		} `json:"suspensionProcessState"`
	}
	if err := json.Unmarshal(updater.calls[0].body, &body); err != nil {
		t.Fatalf("parse update body: %v", err)
	}
	if got := body.SuspensionProcessState.BasedOnSubscriptionEndDate.EventType; got != "90_days_notice" {
		t.Errorf("event_type = %q, want %q", got, "90_days_notice")
	}
}

// TestProcessProject_CustomerAudienceWindowNotifiesBusinessContact covers a
// 7-day window: both internal and customer per the confirmed audience
// matrix. A project contact with the business-contact role should receive
// the customer notice, alongside the internal notice.
func TestProcessProject_CustomerAudienceWindowNotifiesBusinessContact(t *testing.T) {
	reader := &mockEntityReader{
		searchProjectContactsFn: func(ctx context.Context, projectID string, body []byte) ([]byte, error) {
			return []byte(`{"contacts":[{"name":"Bob","email":"bob@customer.example","roles":["business_contact"]}]}`), nil
		},
	}
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{}

	now := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	endDate := now.AddDate(0, 0, 6) // fires the 7-day window
	proj := project{ID: "p1", Account: &projectAccountRef{ID: "a1"}, EndDate: &endDate}

	err := processProject(context.Background(), reader, updater, ntf, now, proj)
	if err != nil {
		t.Fatalf("processProject() error = %v, want nil", err)
	}

	if len(ntf.sent) != 2 {
		t.Fatalf("ntf.sent = %d, want 2", len(ntf.sent))
	}
	var sawInternal, sawCustomer bool
	for _, n := range ntf.sent {
		switch n.Kind {
		case notify.KindInternal:
			sawInternal = true
		case notify.KindCustomer:
			sawCustomer = true
			if n.Recipient != "bob@customer.example" {
				t.Errorf("customer notice recipient = %q, want %q", n.Recipient, "bob@customer.example")
			}
		}
	}
	if !sawInternal {
		t.Error("expected an internal notice, got none")
	}
	if !sawCustomer {
		t.Error("expected a customer notice, got none")
	}

	if len(updater.calls) != 1 {
		t.Fatalf("updater.calls = %d, want 1", len(updater.calls))
	}
}

// TestProcessProject_CustomerAudienceWindowNudgesAMWhenNoContactFound covers
// the three-tier fallback's last resort: no business contact, no primary
// contact. The customer notice is replaced entirely by an AM-nudge email —
// not sent in addition to a (nonexistent) customer notice.
func TestProcessProject_CustomerAudienceWindowNudgesAMWhenNoContactFound(t *testing.T) {
	reader := &mockEntityReader{}
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{}

	now := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	endDate := now.AddDate(0, 0, 6) // fires the 7-day window
	proj := project{ID: "p1", Account: &projectAccountRef{ID: "a1"}, EndDate: &endDate}

	err := processProject(context.Background(), reader, updater, ntf, now, proj)
	if err != nil {
		t.Fatalf("processProject() error = %v, want nil", err)
	}

	if len(ntf.sent) != 2 {
		t.Fatalf("ntf.sent = %d, want 2", len(ntf.sent))
	}
	var sawInternal, sawNudge, sawCustomer bool
	for _, n := range ntf.sent {
		switch n.Kind {
		case notify.KindInternal:
			sawInternal = true
		case notify.KindAMNudge:
			sawNudge = true
		case notify.KindCustomer:
			sawCustomer = true
		}
	}
	if !sawInternal {
		t.Error("expected an internal notice, got none")
	}
	if !sawNudge {
		t.Error("expected an AM-nudge notice, got none")
	}
	if sawCustomer {
		t.Error("expected no customer notice when no contact resolved, got one")
	}
}

// TestProcessProject_NotifyFailureBlocksStateWrite verifies that when
// notify.Send fails, no suspensionProcessState write happens — leaving
// lastNoticeWindow unchanged so the same window is retried on the next run,
// with no separate FAILED-marker bookkeeping needed.
func TestProcessProject_NotifyFailureBlocksStateWrite(t *testing.T) {
	reader := &mockEntityReader{}
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{
		sendFn: func(ctx context.Context, n notify.Notice) error {
			return errors.New("smtp relay unreachable")
		},
	}

	now := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	endDate := now.AddDate(0, 0, 89) // fires the 90-day window (internal-only)
	proj := project{ID: "p1", Account: &projectAccountRef{ID: "a1"}, EndDate: &endDate}

	err := processProject(context.Background(), reader, updater, ntf, now, proj)
	if err == nil {
		t.Fatal("processProject() error = nil, want non-nil")
	}
	if len(updater.calls) != 0 {
		t.Errorf("updater.calls = %d, want 0", len(updater.calls))
	}
}

// TestProcessProject_Day0SuccessfulNotifyThenSuspend verifies the day-0
// ordering: when the final notice email succeeds, suspend is attempted
// afterward — two separate UpdateProject calls, notice-state first.
func TestProcessProject_Day0SuccessfulNotifyThenSuspend(t *testing.T) {
	reader := &mockEntityReader{
		searchProjectContactsFn: func(ctx context.Context, projectID string, body []byte) ([]byte, error) {
			return []byte(`{"contacts":[{"name":"Bob","email":"bob@customer.example","roles":["business_contact"]}]}`), nil
		},
	}
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{}

	now := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	endDate := now.AddDate(0, 0, -3) // 3 days past due
	open := "Open"
	proj := project{ID: "p1", Account: &projectAccountRef{ID: "a1"}, EndDate: &endDate, ClosureState: &open}

	err := processProject(context.Background(), reader, updater, ntf, now, proj)
	if err != nil {
		t.Fatalf("processProject() error = %v, want nil", err)
	}

	if len(ntf.sent) != 2 {
		t.Fatalf("ntf.sent = %d, want 2 (internal + customer)", len(ntf.sent))
	}

	if len(updater.calls) != 2 {
		t.Fatalf("updater.calls = %d, want 2 (record notice, then suspend)", len(updater.calls))
	}

	var noticeBody struct {
		SuspensionProcessState struct {
			BasedOnSubscriptionEndDate struct {
				EventType string `json:"event_type"`
			} `json:"based_on_subscription_end_date"`
		} `json:"suspensionProcessState"`
	}
	if err := json.Unmarshal(updater.calls[0].body, &noticeBody); err != nil {
		t.Fatalf("parse first update body: %v", err)
	}
	if got := noticeBody.SuspensionProcessState.BasedOnSubscriptionEndDate.EventType; got != "suspend" {
		t.Errorf("first call event_type = %q, want %q", got, "suspend")
	}

	var suspendBody struct {
		EndDateClosureState string `json:"endDateClosureState"`
	}
	if err := json.Unmarshal(updater.calls[1].body, &suspendBody); err != nil {
		t.Fatalf("parse second update body: %v", err)
	}
	if suspendBody.EndDateClosureState != "Suspended" {
		t.Errorf("second call endDateClosureState = %q, want %q", suspendBody.EndDateClosureState, "Suspended")
	}
}

// TestProcessProject_Day0RetrySkipsNotifyWhenAlreadyRecorded covers the
// retry case: a prior run already recorded the terminal marker (email
// done), but suspend itself previously failed. This run should skip notify
// entirely and go straight to suspend.
func TestProcessProject_Day0RetrySkipsNotifyWhenAlreadyRecorded(t *testing.T) {
	reader := &mockEntityReader{
		searchProjectContactsFn: func(ctx context.Context, projectID string, body []byte) ([]byte, error) {
			t.Fatal("SearchProjectContacts should not be called when notify is already recorded")
			return nil, nil
		},
	}
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{}

	now := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	endDate := now.AddDate(0, 0, -3)
	open := "Open"
	proj := project{
		ID:                     "p1",
		Account:                &projectAccountRef{ID: "a1"},
		EndDate:                &endDate,
		ClosureState:           &open,
		SuspensionProcessState: []byte(`{"based_on_subscription_end_date":{"event_type":"suspend"}}`),
	}

	err := processProject(context.Background(), reader, updater, ntf, now, proj)
	if err != nil {
		t.Fatalf("processProject() error = %v, want nil", err)
	}

	if len(ntf.sent) != 0 {
		t.Errorf("ntf.sent = %d, want 0 (notify already recorded)", len(ntf.sent))
	}
	if len(updater.calls) != 1 {
		t.Fatalf("updater.calls = %d, want 1 (suspend only)", len(updater.calls))
	}
	var suspendBody struct {
		EndDateClosureState string `json:"endDateClosureState"`
	}
	if err := json.Unmarshal(updater.calls[0].body, &suspendBody); err != nil {
		t.Fatalf("parse update body: %v", err)
	}
	if suspendBody.EndDateClosureState != "Suspended" {
		t.Errorf("endDateClosureState = %q, want %q", suspendBody.EndDateClosureState, "Suspended")
	}
}

// TestProcessProject_SuspendGuardSkipsAlreadySuspendedProject verifies the
// suspend guard: if closureState already reads "Suspended" (from this run's
// already-fetched data), no UpdateProject call happens at all for suspend.
func TestProcessProject_SuspendGuardSkipsAlreadySuspendedProject(t *testing.T) {
	reader := &mockEntityReader{}
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{}

	now := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	endDate := now.AddDate(0, 0, -3)
	suspended := "Suspended"
	proj := project{
		ID:                     "p1",
		Account:                &projectAccountRef{ID: "a1"},
		EndDate:                &endDate,
		ClosureState:           &suspended,
		SuspensionProcessState: []byte(`{"based_on_subscription_end_date":{"event_type":"suspend"}}`),
	}

	err := processProject(context.Background(), reader, updater, ntf, now, proj)
	if err != nil {
		t.Fatalf("processProject() error = %v, want nil", err)
	}
	if len(updater.calls) != 0 {
		t.Errorf("updater.calls = %d, want 0 (already suspended, no-op)", len(updater.calls))
	}
}

func TestProcessProject_NothingDueIsNoOp(t *testing.T) {
	reader := &mockEntityReader{}
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{}

	now := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	endDate := now.AddDate(0, 0, 200) // far beyond the 90-day window
	proj := project{ID: "p1", Account: &projectAccountRef{ID: "a1"}, EndDate: &endDate}

	err := processProject(context.Background(), reader, updater, ntf, now, proj)
	if err != nil {
		t.Fatalf("processProject() error = %v, want nil", err)
	}
	if len(updater.calls) != 0 {
		t.Errorf("updater.calls = %d, want 0", len(updater.calls))
	}
	if len(ntf.sent) != 0 {
		t.Errorf("ntf.sent = %d, want 0", len(ntf.sent))
	}
}

// TestProcessProject_InternalNoticeUsesRealAccountManagerEmail is a
// regression test using the real GetAccount response shape confirmed via
// direct Postman testing against the dedicated test account (trimmed to the
// field this component reads): a populated accountManager with a real email.
// The internal notice's Recipient must be that email, and GetAccount must be
// called with the project's account ID.
func TestProcessProject_InternalNoticeUsesRealAccountManagerEmail(t *testing.T) {
	const realGetAccountResponse = `{
		"id": "f213fdd1-1b4b-a650-a002-c9d3604bcbac",
		"name": "ACP Test Partner Account",
		"technicalOwner": {
			"id": "tech-1",
			"name": "Dinithi Nanayakkara (Intern)",
			"email": "dinithi@wso2.example"
		},
		"accountManager": {
			"id": "am-1",
			"name": "Rukshan Kuruppu (Intern)",
			"email": "rukshan@wso2.example"
		},
		"renewalAccountManager": {
			"id": "ram-1",
			"name": "Ishan Hansaka Silva",
			"email": "ishan@wso2.example"
		}
	}`

	var gotAccountID string
	reader := &mockEntityReader{
		getAccountFn: func(ctx context.Context, id string) ([]byte, error) {
			gotAccountID = id
			return []byte(realGetAccountResponse), nil
		},
	}
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{}

	now := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	endDate := now.AddDate(0, 0, 89) // fires the 90-day (internal-only) window
	proj := project{
		ID:      "p1",
		Account: &projectAccountRef{ID: "f213fdd1-1b4b-a650-a002-c9d3604bcbac"},
		EndDate: &endDate,
	}

	err := processProject(context.Background(), reader, updater, ntf, now, proj)
	if err != nil {
		t.Fatalf("processProject() error = %v, want nil", err)
	}

	if gotAccountID != "f213fdd1-1b4b-a650-a002-c9d3604bcbac" {
		t.Errorf("GetAccount called with id = %q, want the project's account ID", gotAccountID)
	}
	if len(ntf.sent) != 1 {
		t.Fatalf("ntf.sent = %d, want 1", len(ntf.sent))
	}
	if got := ntf.sent[0].Recipient; got != "rukshan@wso2.example" {
		t.Errorf("internal notice Recipient = %q, want %q", got, "rukshan@wso2.example")
	}
}

// TestProcessProject_InternalNoticeHasEmptyRecipientWhenNoAccountManager
// covers the legitimate-absence case: an account with no accountManager
// assigned at all (nested key entirely missing, not just empty). The
// internal notice must still be sent, with an empty Recipient — this is not
// an error, per recipients.AccountManagerEmail's contract.
func TestProcessProject_InternalNoticeHasEmptyRecipientWhenNoAccountManager(t *testing.T) {
	reader := &mockEntityReader{
		getAccountFn: func(ctx context.Context, id string) ([]byte, error) {
			return []byte(`{"id": "a1", "name": "Some Account"}`), nil
		},
	}
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{}

	now := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	endDate := now.AddDate(0, 0, 89)
	proj := project{ID: "p1", Account: &projectAccountRef{ID: "a1"}, EndDate: &endDate}

	err := processProject(context.Background(), reader, updater, ntf, now, proj)
	if err != nil {
		t.Fatalf("processProject() error = %v, want nil", err)
	}
	if len(ntf.sent) != 1 {
		t.Fatalf("ntf.sent = %d, want 1", len(ntf.sent))
	}
	if got := ntf.sent[0].Recipient; got != "" {
		t.Errorf("internal notice Recipient = %q, want \"\" (no account manager assigned)", got)
	}
}
