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

package notify

import (
	"context"
	"log/slog"
	"testing"

	"github.com/wso2-open-operations/cs-tools/integrations/acp-closure-service/internal/recipients"
)

// capturingHandler records every log record passed to it, so tests can
// assert on structured attributes directly rather than parsing formatted
// text output.
type capturingHandler struct {
	records []slog.Record
}

func (h *capturingHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *capturingHandler) Handle(_ context.Context, r slog.Record) error {
	h.records = append(h.records, r)
	return nil
}

func (h *capturingHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *capturingHandler) WithGroup(_ string) slog.Handler      { return h }

func attrValue(t *testing.T, r slog.Record, key string) (string, bool) {
	t.Helper()
	var val string
	var found bool
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == key {
			val = a.Value.String()
			found = true
			return false
		}
		return true
	})
	return val, found
}

// TestLoggingNotifier_Send_LogsResolvedViaForCustomerNotice verifies a
// customer notice's log line carries which fallback tier resolved the
// recipient — the exact signal needed to observe, from real run logs, how
// often each tier of the three-tier fallback actually fires (e.g. to gather
// evidence toward confirming the still-unconfirmed business-contact role
// string).
func TestLoggingNotifier_Send_LogsResolvedViaForCustomerNotice(t *testing.T) {
	h := &capturingHandler{}
	n := &LoggingNotifier{Logger: slog.New(h)}

	err := n.Send(context.Background(), Notice{
		Kind:        KindCustomer,
		ProjectID:   "p1",
		Recipient:   "bob@customer.example",
		ResolvedVia: recipients.ResolvedViaBusinessContact,
	})
	if err != nil {
		t.Fatalf("Send() error = %v, want nil", err)
	}
	if len(h.records) != 1 {
		t.Fatalf("records = %d, want 1", len(h.records))
	}

	got, found := attrValue(t, h.records[0], "resolvedVia")
	if !found {
		t.Fatal("resolvedVia attribute not present in log record")
	}
	if got != string(recipients.ResolvedViaBusinessContact) {
		t.Errorf("resolvedVia = %q, want %q", got, recipients.ResolvedViaBusinessContact)
	}
}

// TestLoggingNotifier_Send_LogsEmptyResolvedViaForInternalNotice covers the
// internal (Account Manager) notice, which never goes through
// recipients.ResolveCustomerContact's fallback chain at all — its
// resolvedVia should log as empty, distinguishable from
// recipients.ResolvedViaNone ("none"), which specifically means "all
// fallback tiers were tried and none resolved."
func TestLoggingNotifier_Send_LogsEmptyResolvedViaForInternalNotice(t *testing.T) {
	h := &capturingHandler{}
	n := &LoggingNotifier{Logger: slog.New(h)}

	err := n.Send(context.Background(), Notice{
		Kind:      KindInternal,
		ProjectID: "p1",
		Recipient: "am@wso2.example",
	})
	if err != nil {
		t.Fatalf("Send() error = %v, want nil", err)
	}
	if len(h.records) != 1 {
		t.Fatalf("records = %d, want 1", len(h.records))
	}

	got, found := attrValue(t, h.records[0], "resolvedVia")
	if !found {
		t.Fatal("resolvedVia attribute not present in log record")
	}
	if got != "" {
		t.Errorf("resolvedVia = %q, want \"\" (not %q)", got, recipients.ResolvedViaNone)
	}
}
