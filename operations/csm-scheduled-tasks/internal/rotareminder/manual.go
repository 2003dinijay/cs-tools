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

package rotareminder

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/notify"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/teamschedule"
)

// RequestQueue is the subset of *teamschedule.Client that hands out the
// "Send now" requests made on the Rota Access tab.
type RequestQueue interface {
	ClaimReminderRequest(ctx context.Context, family string) (*teamschedule.ReminderRequest, error)
	ReportReminderRequest(ctx context.Context, family, id, status, result string) error
}

// manualPerRun bounds the requests one run sends, so a backlog cannot keep a
// run going; the rest wait for the next.
const manualPerRun = 5

// ManualReminder returns the handler of rota_manual_reminder: at every run it
// claims the family's waiting "Send now" requests and sends each with the
// automatic reminders' own code -- weekday is the next working day's windows,
// weekend the coming weekend (this one, from a Saturday or a Sunday), sent
// even when not fully marked, as on Friday. Each request is reported SENT or
// FAILED with a short result for the card. A request that cannot be sent is
// reported, not retried: somebody pressed a button and should see why. Only a
// failure to claim or report fails the run.
func ManualReminder(rota RotaReader, store ReminderStore, queue RequestQueue, email EmailSender, weekdayCfg, weekendCfg Config) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		for i := 0; i < manualPerRun; i++ {
			req, err := queue.ClaimReminderRequest(ctx, weekendCfg.Family)
			if err != nil {
				return fmt.Errorf("rotareminder: claim a rota reminder request: %w", err)
			}
			if req == nil {
				return nil
			}
			status, result := "SENT", ""
			weekday, weekend := weekdayCfg, weekendCfg
			if req.Test {
				weekday, weekend = asTest(weekday, req.RequestedBy), asTest(weekend, req.RequestedBy)
			}
			sent, err := sendManual(ctx, rota, store, email, weekday, weekend, req.Kind)
			switch {
			case err != nil:
				status, result = "FAILED", err.Error()
			case sent == 0:
				result = "Nothing was sent: nobody on these rotations has an email address, or the rotations are empty."
			case sent == 1:
				result = "Sent 1 email."
			default:
				result = fmt.Sprintf("Sent %d emails.", sent)
			}
			if req.Test && err == nil && sent > 0 {
				result = strings.TrimSuffix(result, ".") + " to " + req.RequestedBy + " (test)."
			}
			slog.InfoContext(ctx, "rota reminder: sent by hand", "kind", req.Kind, "test", req.Test, "requestedBy", req.RequestedBy, "status", status, "result", result)
			if err := queue.ReportReminderRequest(ctx, weekendCfg.Family, req.ID, status, result); err != nil {
				return fmt.Errorf("rotareminder: report rota reminder request %s: %w", req.ID, err)
			}
		}
		return nil
	}
}

// sendManual sends one request and counts the emails that went out.
func sendManual(ctx context.Context, rota RotaReader, store ReminderStore, email EmailSender, weekdayCfg, weekendCfg Config, kind string) (int, error) {
	counted := &countingSender{EmailSender: email}
	switch kind {
	case "weekday":
		cfg := weekdayCfg
		if !cfg.EmailsEnabled {
			return 0, errors.New("emails are turned off in this environment (ALERTS_ENABLED)")
		}
		cfg.Manual, cfg.Anchor = true, cfg.Now
		err := WeekdayReminder(rota, counted, cfg)(ctx)
		return counted.n, err
	case "weekend":
		cfg := weekendCfg
		if !cfg.EmailsEnabled {
			return 0, errors.New("emails are turned off in this environment (ALERTS_ENABLED)")
		}
		now := cfg.Now()
		cfg.Manual, cfg.Anchor = true, func() time.Time { return manualWeekendAnchor(now, cfg.Zone) }
		err := weekend(ctx, rota, store, counted, cfg, true)
		return counted.n, err
	}
	return 0, fmt.Errorf("unknown reminder kind %q", kind)
}

// manualWeekendAnchor is the day a "Send now" weekend is counted from: today,
// except from a Saturday or a Sunday, when it is the Friday before, so the
// weekend is this one and not the next.
func manualWeekendAnchor(now time.Time, zone *time.Location) time.Time {
	switch day := dayOf(now, zone); day.Weekday() {
	case time.Saturday:
		return day.AddDate(0, 0, -1)
	case time.Sunday:
		return day.AddDate(0, 0, -2)
	default:
		return now
	}
}

// countingSender counts the emails the reminders send. Only the rota's own
// emails count: an alert about a failure is not a reminder.
type countingSender struct {
	EmailSender
	n int
}

func (c *countingSender) SendEmail(ctx context.Context, to, cc []string, subject, body string) error {
	return c.SendEmailWithAttachments(ctx, to, cc, subject, body, nil)
}

func (c *countingSender) SendEmailWithAttachments(ctx context.Context, to, cc []string, subject, body string, a []notify.Attachment) error {
	err := c.EmailSender.SendEmailWithAttachments(ctx, to, cc, subject, body, a)
	if err == nil && strings.Contains(subject, " Rota]") {
		c.n++
	}
	return err
}

// asTest is cfg for "Send a test to me": everything, alerts included, goes
// only to the person who asked, and nothing is copied or recorded.
func asTest(cfg Config, to string) Config {
	cfg.Test = true
	cfg.RedirectTo = []string{to}
	cfg.Copy = nil
	cfg.AlertTo, cfg.AlertCc = []string{to}, nil
	return cfg
}
