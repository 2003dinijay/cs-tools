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

// Package notify defines the shape of an ACP notice and a logging
// implementation of sending one. Real email sending is not implemented on
// either side yet — deferred pending message-queue design (per the 2026-07-17
// meeting notes) — so LoggingNotifier is not a temporary stand-in for this
// component specifically; it is genuinely the only option available today.
package notify

import (
	"context"
	"log/slog"

	"github.com/wso2-open-operations/cs-tools/acp-closure-service/internal/closure"
	"github.com/wso2-open-operations/cs-tools/acp-closure-service/internal/recipients"
)

// Kind distinguishes the purpose of a Notice, since the same window can
// produce different recipients and content depending on audience.
type Kind string

const (
	// KindCustomer is the customer-facing closure notice for a given window.
	KindCustomer Kind = "customer"
	// KindInternal is the Account Manager notice sent for every firing window.
	KindInternal Kind = "internal"
	// KindAMNudge is a distinct "please configure a business contact" email,
	// sent to the Account Manager instead of (not in addition to) a customer
	// notice when recipients.Resolution.NeedsAMNudge is true.
	KindAMNudge Kind = "am_nudge"
)

// Notice is everything a Notifier needs to send (or, today, log) one ACP
// notification.
type Notice struct {
	Kind      Kind
	Window    closure.NoticeWindow
	ProjectID string
	Recipient string
	// ResolvedVia records which tier of the three-tier customer-contact
	// fallback produced Recipient (see recipients.ResolveCustomerContact).
	// Only meaningful for KindCustomer/KindAMNudge — left at its zero value
	// ("") for KindInternal, which never goes through that fallback chain at
	// all. Deliberately distinct from recipients.ResolvedViaNone ("none"),
	// which means "every tier was tried and none resolved" — a different,
	// more specific fact than "this notice doesn't use tiers."
	ResolvedVia recipients.ResolvedVia
}

// LoggingNotifier logs what would have been sent instead of sending it.
type LoggingNotifier struct {
	Logger *slog.Logger
}

// Send logs the notice and always succeeds.
func (n *LoggingNotifier) Send(ctx context.Context, notice Notice) error {
	n.Logger.InfoContext(ctx, "notice",
		"kind", notice.Kind,
		"window", notice.Window,
		"projectID", notice.ProjectID,
		"recipient", notice.Recipient,
		"resolvedVia", notice.ResolvedVia,
	)
	return nil
}
