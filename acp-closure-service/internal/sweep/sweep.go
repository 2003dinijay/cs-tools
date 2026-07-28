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

// Package sweep orchestrates one full ACP evaluation pass: fetch open
// projects, decide what's due, resolve recipients, notify, and write back
// state. Dry-run is not a flag branched on inside this package — it is
// entirely a property of which projectUpdater/notifier implementation the
// caller injects (see types.go).
package sweep

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/wso2-open-operations/cs-tools/acp-closure-service/internal/closure"
	"github.com/wso2-open-operations/cs-tools/acp-closure-service/internal/notify"
	"github.com/wso2-open-operations/cs-tools/acp-closure-service/internal/recipients"
	"github.com/wso2-open-operations/cs-tools/acp-closure-service/internal/suspensionstate"
)

// processProject evaluates and, if anything is due, acts on a single
// project. Notify happens before suspend is ever attempted, and an error
// from notify returns immediately — this ordering, not a separate flag, is
// what guarantees suspend never proceeds after a failed notify (the day-0
// "email first, stop on failure" contract).
func processProject(ctx context.Context, reader entityReader, updater projectUpdater, ntf notifier, now time.Time, proj project) error {
	if proj.EndDate == nil {
		return nil
	}

	lastWindow, err := suspensionstate.LastNoticeWindow(proj.SuspensionProcessState)
	if err != nil {
		return fmt.Errorf("sweep: parse suspensionProcessState for project %s: %w", proj.ID, err)
	}

	decision := closure.Decide(now, *proj.EndDate, lastWindow)
	if !decision.Fires {
		return nil
	}

	if decision.ShouldNotify {
		if err := notifyForWindow(ctx, reader, ntf, proj, decision.Window); err != nil {
			return fmt.Errorf("sweep: notify project %s: %w", proj.ID, err)
		}
		if err := recordNoticeSent(ctx, updater, proj, decision.Window); err != nil {
			return fmt.Errorf("sweep: record notice for project %s: %w", proj.ID, err)
		}
	}

	if decision.ShouldSuspend {
		if err := suspend(ctx, updater, proj); err != nil {
			return fmt.Errorf("sweep: suspend project %s: %w", proj.ID, err)
		}
	}

	return nil
}

// needsCustomerAudience reports whether window's confirmed audience matrix
// includes the customer, not just the Account Manager. 90/60/30 are
// internal-only; 15/7/0 are both.
func needsCustomerAudience(window closure.NoticeWindow) bool {
	switch window {
	case closure.NoticeWindow15, closure.NoticeWindow7, closure.NoticeWindow0:
		return true
	default:
		return false
	}
}

// notifyForWindow sends the internal (Account Manager) notice — due for
// every firing window — and, only when the audience matrix calls for it,
// the customer-facing notice (or the distinct AM-nudge email when no
// customer contact resolves).
func notifyForWindow(ctx context.Context, reader entityReader, ntf notifier, proj project, window closure.NoticeWindow) error {
	amEmail, err := recipients.StubOwnerEmail(proj.AccountID)
	if err != nil {
		return fmt.Errorf("resolve AM email: %w", err)
	}

	if err := ntf.Send(ctx, notify.Notice{
		Kind:      notify.KindInternal,
		Window:    window,
		ProjectID: proj.ID,
		Recipient: amEmail,
	}); err != nil {
		return fmt.Errorf("send internal notice: %w", err)
	}

	if !needsCustomerAudience(window) {
		return nil
	}

	projectContacts, accountContacts, err := fetchContacts(ctx, reader, proj)
	if err != nil {
		return err
	}
	resolution := recipients.ResolveCustomerContact(projectContacts, accountContacts)

	if resolution.NeedsAMNudge {
		return ntf.Send(ctx, notify.Notice{
			Kind:      notify.KindAMNudge,
			Window:    window,
			ProjectID: proj.ID,
			Recipient: amEmail,
		})
	}

	return ntf.Send(ctx, notify.Notice{
		Kind:      notify.KindCustomer,
		Window:    window,
		ProjectID: proj.ID,
		Recipient: resolution.CustomerContact.Email,
	})
}

func fetchContacts(ctx context.Context, reader entityReader, proj project) ([]recipients.ProjectContact, []recipients.AccountContact, error) {
	pcRaw, err := reader.SearchProjectContacts(ctx, proj.ID, []byte(`{}`))
	if err != nil {
		return nil, nil, fmt.Errorf("search project contacts: %w", err)
	}
	var pcResp projectContactSearchResponse
	if err := json.Unmarshal(pcRaw, &pcResp); err != nil {
		return nil, nil, fmt.Errorf("parse project contacts: %w", err)
	}

	acRaw, err := reader.SearchAccountContacts(ctx, proj.AccountID, []byte(`{}`))
	if err != nil {
		return nil, nil, fmt.Errorf("search account contacts: %w", err)
	}
	var acResp accountContactSearchResponse
	if err := json.Unmarshal(acRaw, &acResp); err != nil {
		return nil, nil, fmt.Errorf("parse account contacts: %w", err)
	}

	projectContacts := make([]recipients.ProjectContact, len(pcResp.Contacts))
	for i, c := range pcResp.Contacts {
		projectContacts[i] = recipients.ProjectContact{Name: c.Name, Email: c.Email, Roles: c.Roles}
	}
	accountContacts := make([]recipients.AccountContact, len(acResp.Contacts))
	for i, c := range acResp.Contacts {
		accountContacts[i] = recipients.AccountContact{Name: c.Name, Email: c.Email, IsPrimary: c.IsPrimary}
	}
	return projectContacts, accountContacts, nil
}

// recordNoticeSent writes the new window into suspensionProcessState's
// based_on_subscription_end_date key, preserving every other key untouched.
func recordNoticeSent(ctx context.Context, updater projectUpdater, proj project, window closure.NoticeWindow) error {
	newState, err := suspensionstate.WithSubscriptionEndDateState(proj.SuspensionProcessState, window, map[string]string{
		"actionSendEmailNotification": "SUCCESSFUL",
	})
	if err != nil {
		return fmt.Errorf("build suspensionProcessState: %w", err)
	}

	body, err := json.Marshal(map[string]json.RawMessage{"suspensionProcessState": newState})
	if err != nil {
		return fmt.Errorf("marshal update request: %w", err)
	}

	_, err = updater.UpdateProject(ctx, proj.ID, body)
	return err
}

// suspend writes endDateClosureState=Suspended, unless the project is
// already suspended (checked against the already-fetched closureState — no
// extra round-trip), mirroring legacy's checkForOpenProject guard.
func suspend(ctx context.Context, updater projectUpdater, proj project) error {
	if proj.ClosureState != nil && *proj.ClosureState == "Suspended" {
		return nil
	}

	body, err := json.Marshal(map[string]string{"endDateClosureState": "Suspended"})
	if err != nil {
		return fmt.Errorf("marshal update request: %w", err)
	}

	_, err = updater.UpdateProject(ctx, proj.ID, body)
	return err
}
