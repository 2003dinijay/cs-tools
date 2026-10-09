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

package dispatch

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/events"
)

// handleStatusPageDue posts one cloud status webhook to the public status
// dashboard and reports the outcome to entity-service.
//
// ONE ATTEMPT, AND NEVER AN ERROR BACK TO THE CONSUMER. csm-scheduled-tasks
// already retries undelivered webhooks (every five minutes, up to five
// attempts) from entity-service's ledger. If this consumer retried too -- and
// then the DLQ consumer replayed -- two retriers would race on one row and the
// dashboard could be told the same thing twice. So a failed post is reported
// as a failed attempt, which ends the row's lease and makes it the scheduled
// task's to retry, and the record is acknowledged.
//
// A report that itself fails is logged and dropped: the row's lease then runs
// out (15 minutes) and the scheduled task picks it up, posting it again if the
// post had in fact succeeded. That is the one double-post window, and it needs
// entity-service to be unreachable at the moment of the report.
func (d *Dispatcher) handleStatusPageDue(ctx context.Context, raw json.RawMessage) error {
	var p events.OutageStatusPageDuePayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("dispatch: decode %s payload: %w", events.TypeOutageStatusPageDue, err)
	}
	if d.statusPageReports == nil {
		// Not configured at all: nothing can be posted or reported. The lease
		// runs out and the scheduled task posts it.
		slog.WarnContext(ctx, "dispatch: status page not configured; leaving outage.status_page_due to the scheduled task",
			"webhookId", p.WebhookID, "outageId", p.OutageID)
		return nil
	}

	var postErr error
	if d.statusPage == nil {
		postErr = fmt.Errorf("status page webhooks are not configured on csm-notification-service")
	} else {
		postErr = d.statusPage.Post(ctx, p.Cloud, p.Event, p.Timestamp)
	}
	errMsg := ""
	if postErr != nil {
		errMsg = postErr.Error()
	}
	if err := d.statusPageReports.RecordCloudStatusDelivery(ctx, p.WebhookID, postErr == nil, errMsg); err != nil {
		slog.ErrorContext(ctx, "dispatch: reporting status page delivery failed; the lease will hand it to the scheduled task",
			"webhookId", p.WebhookID, "outageId", p.OutageID, "delivered", postErr == nil, "err", err)
		return nil
	}
	if postErr != nil {
		slog.WarnContext(ctx, "dispatch: status page webhook failed; left for the scheduled task",
			"webhookId", p.WebhookID, "number", p.Number, "cloud", p.Cloud, "event", p.Event, "err", errMsg)
		return nil
	}
	slog.InfoContext(ctx, "dispatch: status page webhook delivered",
		"webhookId", p.WebhookID, "number", p.Number, "cloud", p.Cloud, "event", p.Event)
	return nil
}
