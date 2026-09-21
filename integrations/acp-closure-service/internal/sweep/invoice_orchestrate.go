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
	"fmt"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/acp-closure-service/internal/closure"
	"github.com/wso2-open-operations/cs-tools/integrations/acp-closure-service/internal/suspensionstate"
)

// processProjectInvoice evaluates and, if anything is due, acts on the
// invoice-based closure reason for a single project — the Phase 2 sibling
// of processProject's own subscription end-date evaluation, entirely
// independent (its own suspensionProcessState track, its own
// InvoiceDueDateClosureState dimension).
//
// Two preconditions gate this cascade before any invoice data is even
// fetched, per DecideInvoice's own documented contract:
//   - isPartner (confirmed account-level via the real API — see
//     closure.DecideInvoice's doc comment) disables the cascade entirely.
//   - No eligible due invoice (resolveDueInvoice returns nil) — a
//     legitimate, common state, not an error.
func processProjectInvoice(ctx context.Context, reader entityReader, updater projectUpdater, ntf notifier, now time.Time, proj project) error {
	if proj.Account != nil && proj.Account.IsPartner != nil && *proj.Account.IsPartner {
		return nil
	}

	invoice, err := resolveDueInvoice(ctx, reader, proj)
	if err != nil {
		return fmt.Errorf("resolve due invoice for project %s: %w", proj.ID, err)
	}
	if invoice == nil {
		return nil
	}

	hasPrimaryPartner, err := resolveHasPrimaryPartner(ctx, reader, proj.accountID())
	if err != nil {
		return fmt.Errorf("resolve hasPrimaryPartner for project %s: %w", proj.ID, err)
	}

	lastWindow, err := suspensionstate.LastNoticeWindowForInvoices(proj.SuspensionProcessState)
	if err != nil {
		return fmt.Errorf("parse suspensionProcessState for project %s: %w", proj.ID, err)
	}

	decision := closure.DecideInvoice(now, invoice.InvoiceDate, invoice.DueDate, invoice.EULAVersionDecimal, hasPrimaryPartner, lastWindow)
	if !decision.Fires {
		return nil
	}

	resolvedForNotice := dueInvoice{
		ID:          invoice.ID,
		Opportunity: invoice.Opportunity,
		DueDate:     invoice.DueDate,
		SuspendDate: closure.InvoiceSuspendDate(invoice.InvoiceDate, invoice.DueDate, invoice.EULAVersionDecimal, hasPrimaryPartner),
	}

	if decision.ShouldNotify {
		delivered := false
		if !alreadyClosedForAnyReason(proj) {
			delivered, err = notifyForWindow(ctx, reader, ntf, proj, decision.Window,
				func(w closure.NoticeWindow, p project, accountOwnerName string) string {
					return internalInvoiceNoticeBody(w, p, accountOwnerName, resolvedForNotice)
				},
				customerInvoiceNoticeSubject,
				func(w closure.NoticeWindow, p project) string {
					return customerInvoiceNoticeBody(w, p, resolvedForNotice)
				},
			)
			if err != nil {
				return fmt.Errorf("sweep: notify invoice for project %s: %w", proj.ID, err)
			}
		}
		if err := recordInvoiceNoticeSent(ctx, updater, proj, decision.Window, delivered); err != nil {
			return fmt.Errorf("sweep: record invoice notice for project %s: %w", proj.ID, err)
		}
	}

	if decision.ShouldSuspend {
		if err := suspendInvoice(ctx, updater, proj); err != nil {
			return fmt.Errorf("sweep: suspend invoice for project %s: %w", proj.ID, err)
		}
	}

	return nil
}

// resolveHasPrimaryPartner fetches the account's hasPrimaryPartner flag —
// confirmed via the real API to only be present on GetAccount's full
// response, not the shortened account summary embedded in a project
// response (unlike isPartner, which is present there). "" accountID (no
// linked account) and an absent field both resolve to false, matching
// usesGracePeriod's own "not a primary partner" default.
func resolveHasPrimaryPartner(ctx context.Context, reader entityReader, accountID string) (bool, error) {
	if accountID == "" {
		return false, nil
	}
	raw, err := reader.GetAccount(ctx, accountID)
	if err != nil {
		return false, fmt.Errorf("get account: %w", err)
	}
	var acc accountDTO
	if err := json.Unmarshal(raw, &acc); err != nil {
		return false, fmt.Errorf("parse account: %w", err)
	}
	return acc.HasPrimaryPartner != nil && *acc.HasPrimaryPartner, nil
}

// recordInvoiceNoticeSent mirrors recordNoticeSent exactly, writing
// based_on_due_invoices instead of based_on_subscription_end_date — its
// own independent idempotency track.
func recordInvoiceNoticeSent(ctx context.Context, updater projectUpdater, proj project, window closure.NoticeWindow, delivered bool) error {
	action := "IGNORED"
	if delivered {
		action = "SUCCESSFUL"
	}
	newState, err := suspensionstate.WithDueInvoicesState(proj.SuspensionProcessState, window, map[string]string{
		"actionSendEmailNotification": action,
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

// suspendInvoice mirrors suspend exactly, writing/reading
// InvoiceDueDateClosureState instead of EndDateClosureState — its own
// per-dimension idempotency guard, entirely separate from the subscription
// cascade's.
func suspendInvoice(ctx context.Context, updater projectUpdater, proj project) error {
	if proj.InvoiceDueDateClosureState != nil && *proj.InvoiceDueDateClosureState != "Open" {
		return nil
	}

	body, err := json.Marshal(map[string]string{"invoiceDueDateClosureState": "Suspended"})
	if err != nil {
		return fmt.Errorf("marshal update request: %w", err)
	}

	_, err = updater.UpdateProject(ctx, proj.ID, body)
	return err
}
