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
	"fmt"
	"log/slog"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/events"
)

// outageNoticePublisher is the slice of EventPublisherService the drainer needs.
type outageNoticePublisher interface {
	Publish(ctx context.Context, eventType events.Type, entityID string, payload json.RawMessage) error
}

// OutageNoticeDrainer sends the two outage emails within seconds of the change
// that makes them due, as ServiceNow's record-triggered flows do.
//
// *** IT RUNS THE SAME DECISIONS THE SCHEDULED TASKS USED TO. *** Both emails
// are state-based -- "this outage is opted in and has ended, and no resolution
// has been sent" -- so the existing sweeps already say exactly what is owed and
// record it. What changes is cadence and delivery: every Interval instead of
// every scheduler tick, and an event on the outage topic for
// csm-notification-service instead of an email sent by the caller. No outbox
// row is claimed, so the cloud status drainer's use of event_outbox for the
// same outages is untouched.
//
// *** A FLOW WITH NO RECIPIENTS IS NOT SWEPT. *** A sweep records what it
// decides, so sweeping with nobody to send to would mark emails sent that no
// one received. Leaving a recipient list empty is how a deployment keeps that
// email off.
type OutageNoticeDrainer struct {
	Notifications           OutageNotificationService
	Communications          OutageCommunicationService
	Publisher               outageNoticePublisher
	NotificationRecipients  []string
	CommunicationRecipients []string
	Interval                time.Duration
}

// Run drains until ctx is cancelled. ctx must carry the system identity: both
// sweeps are for internal callers only.
func (d *OutageNoticeDrainer) Run(ctx context.Context) {
	slog.InfoContext(ctx, "outagenotice: drainer started", "interval", d.Interval,
		"notificationRecipients", len(d.NotificationRecipients),
		"communicationRecipients", len(d.CommunicationRecipients))
	for {
		d.drainOnce(ctx)
		select {
		case <-ctx.Done():
			slog.InfoContext(ctx, "outagenotice: drainer stopped")
			return
		case <-time.After(d.Interval):
		}
	}
}

// drainOnce runs each enabled flow once and publishes what it decided.
//
// One flow failing never stops the other, and one publish failing never stops
// the rest: each decision is already recorded, so returning early would drop
// the remainder silently. A failed publish is kept by EventPublisherService's
// failure store for replay, and logged here with the outage it belongs to.
func (d *OutageNoticeDrainer) drainOnce(ctx context.Context) (published int) {
	if len(d.NotificationRecipients) > 0 {
		res, err := d.Notifications.Sweep(ctx, 0)
		if err != nil {
			slog.ErrorContext(ctx, "outagenotice: internal-notification sweep failed", "err", err)
		} else {
			for _, dec := range res.Decisions {
				if d.publish(ctx, events.TypeOutageNotificationDue, events.OutageNoticePayload{
					OutageID: dec.OutageID, Number: dec.Number, Kind: string(dec.Kind),
					Subject: dec.Subject, Body: dec.Body, Recipients: d.NotificationRecipients,
				}) {
					published++
				}
			}
			for id, msg := range res.Errors {
				slog.ErrorContext(ctx, "outagenotice: internal notification could not be recorded; not sent",
					"outageId", id, "err", msg)
			}
		}
	}
	if len(d.CommunicationRecipients) > 0 {
		res, err := d.Communications.Sweep(ctx, 0)
		if err != nil {
			slog.ErrorContext(ctx, "outagenotice: outage-communication sweep failed", "err", err)
		} else {
			for _, dec := range res.Decisions {
				if d.publish(ctx, events.TypeOutageCommunicationDue, events.OutageNoticePayload{
					OutageID: dec.OutageID, Number: dec.Number, Kind: string(dec.Kind),
					Subject: dec.Subject, Body: dec.Body, Recipients: d.CommunicationRecipients,
				}) {
					published++
				}
			}
		}
	}
	return published
}

func (d *OutageNoticeDrainer) publish(ctx context.Context, t events.Type, p events.OutageNoticePayload) bool {
	raw, err := json.Marshal(p)
	if err == nil {
		err = d.Publisher.Publish(ctx, t, p.OutageID, raw)
	}
	if err != nil {
		slog.ErrorContext(ctx, "outagenotice: publish failed; email recorded as sent but not delivered",
			"type", string(t), "outageId", p.OutageID, "number", p.Number, "kind", p.Kind,
			"err", fmt.Errorf("publish %s: %w", t, err))
		return false
	}
	slog.InfoContext(ctx, "outagenotice: published", "type", string(t),
		"outageId", p.OutageID, "number", p.Number, "kind", p.Kind)
	return true
}
