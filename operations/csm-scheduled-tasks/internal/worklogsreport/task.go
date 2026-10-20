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

// Package worklogsreport is the CS Worklogs daily and weekly report
// sub-crons. It holds the tasks; internal/worklogreport holds the
// entity-service client and the wire types, and internal/notify renders
// them -- the same split every other report task here uses, and for the
// same mechanical reason: notify imports the client package, so a task
// that imports notify cannot live beside it without a cycle.
package worklogsreport

import (
	"context"
	"fmt"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/notify"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/worklogreport"
)

// ReportFetcher is the subset of *worklogreport.Client these tasks use.
type ReportFetcher interface {
	Report(ctx context.Context, cadence worklogreport.Cadence, from, to time.Time) (worklogreport.Report, error)
}

// EmailSender is the subset of *notify.Client these tasks use.
type EmailSender interface {
	SendEmail(ctx context.Context, to, cc []string, subject, htmlBody string) error
}

// ReportTimezone is the zone the reported day and week are measured in.
//
// It matters, and it is not the zone the cron fires in. ServiceNow runs
// WeeklyReportAutomation V2 at 16:00 UTC on a Monday, which is 21:30 in
// Asia/Colombo -- the timestamp on the delivered mail. The job then asks
// for the seven days before its own run date. Computing "the week that
// just ended" in UTC from a cron that fires at 16:00 UTC gives the same
// answer here, but the daily report fires at 03:55 UTC, which is already
// the next day in Colombo, so the two zones disagree about which day
// "yesterday" is. The reported range is therefore always derived in this
// zone, never in UTC and never in the host's local time.
const ReportTimezone = "Asia/Colombo"

// WeeklyRange returns the Monday-to-Sunday window a weekly run reports
// on: the seven days ending the day before now.
//
// ServiceNow's DateRange action walks back from its trigger date one day
// at a time -- `to` is the previous day and `from` six further back --
// so a Monday run covers the Monday to Sunday just gone. Its own output
// names are off by one (outputs.sat holds Sunday), which affects only
// the column headings, not the range.
func WeeklyRange(now time.Time, loc *time.Location) (from, to time.Time) {
	day := now.In(loc)
	to = time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -1)
	from = to.AddDate(0, 0, -6)
	return from, to
}

// DailyDate returns the single day a daily run reports on: yesterday.
func DailyDate(now time.Time, loc *time.Location) time.Time {
	day := now.In(loc)
	return time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -1)
}

// SendDaily returns the daily sub-cron handler.
func SendDaily(reports ReportFetcher, email EmailSender, to, cc []string, emailsEnabled bool) func(ctx context.Context) error {
	return send(reports, email, worklogreport.CadenceDaily, to, cc, emailsEnabled)
}

// SendWeekly returns the weekly sub-cron handler.
func SendWeekly(reports ReportFetcher, email EmailSender, to, cc []string, emailsEnabled bool) func(ctx context.Context) error {
	return send(reports, email, worklogreport.CadenceWeekly, to, cc, emailsEnabled)
}

// send fetches, renders and sends one report.
//
// *** RECIPIENTS COME FROM CONFIGURATION, NOT FROM THE DATA. ***
//
// entity-service derives an audience from the team registry and returns
// it on the report, but it is carried for comparison only. The delivered
// mails are not derivable: their To and Cc roles are inverted between
// the two cadences, two addresses appear in both fields of the daily
// one, and several recipients hold no team membership at all. The
// ServiceNow flow hardcodes both lines in its Send Email action, and its
// GetLeadsEmails step feeds a getLeadsInfo() that V2 overrides to return
// nothing. Deriving the audience here would mail the wrong thirty-four
// people.
//
// Drift between the configured lists and the derived one is logged so a
// lead who joins and never receives the report, or one who left and
// still does, surfaces somewhere other than an inbox.
//
// An empty report is still sent. "Nobody logged time" is a real answer,
// and a day where the mail simply does not arrive is indistinguishable
// from a day where the job broke.
func send(reports ReportFetcher, email EmailSender, cadence worklogreport.Cadence,
	to, cc []string, emailsEnabled bool) func(ctx context.Context) error {

	return func(ctx context.Context) error {
		if !emailsEnabled || len(to) == 0 {
			return nil
		}

		loc, err := time.LoadLocation(ReportTimezone)
		if err != nil {
			return fmt.Errorf("worklogsreport: load %s: %w", ReportTimezone, err)
		}

		var from, until time.Time
		if cadence == worklogreport.CadenceWeekly {
			from, until = WeeklyRange(time.Now(), loc)
		} else {
			from = DailyDate(time.Now(), loc)
			until = from
		}

		report, err := reports.Report(ctx, cadence, from, until)
		if err != nil {
			return err
		}

		audience := worklogreport.Resolve(report.Recipients,
			worklogreport.Audience{To: to, Cc: cc}, nil)

		return email.SendEmail(ctx, audience.To, audience.Cc,
			notify.WorklogsReportSubject(report), notify.RenderWorklogsReport(report))
	}
}
