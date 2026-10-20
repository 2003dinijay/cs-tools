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

package worklogsreport_test

import (
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/worklogsreport"
)

// The range these reports cover is derived in Asia/Colombo, not UTC, and
// the daily cron fires at an hour where the two zones disagree about the
// date. That disagreement has cost this codebase real bugs before, so it
// is pinned here rather than left to reasoning.
func TestReportRangesAreDerivedInColombo(t *testing.T) {
	loc, err := time.LoadLocation(worklogsreport.ReportTimezone)
	if err != nil {
		t.Fatalf("load %s: %v", worklogsreport.ReportTimezone, err)
	}

	t.Run("weekly covers the Monday-to-Sunday just gone", func(t *testing.T) {
		// 16:00 UTC on Monday 2026-10-12 -- when ServiceNow's
		// WeeklyReportAutomation V2 fires. The delivered mail for that
		// run is subject-lined [2026-10-05 - 2026-10-11].
		run := time.Date(2026, 10, 12, 16, 0, 0, 0, time.UTC)
		from, to := worklogsreport.WeeklyRange(run, loc)
		if got := from.Format("2006-01-02"); got != "2026-10-05" {
			t.Errorf("from = %s, the delivered mail says 2026-10-05", got)
		}
		if got := to.Format("2006-01-02"); got != "2026-10-11" {
			t.Errorf("to = %s, the delivered mail says 2026-10-11", got)
		}
		if d := to.Sub(from).Hours() / 24; d != 6 {
			t.Errorf("span = %v days between endpoints, want 6", d)
		}
	})

	t.Run("daily reports yesterday in Colombo, not in UTC", func(t *testing.T) {
		// 03:55 UTC on 2026-10-19 is 09:25 on the 19th in Colombo, which
		// is when the delivered daily mail arrives. Its subject is
		// [2026-10-18] -- yesterday in Colombo. Deriving it in UTC from
		// this instant gives the same answer, so the case that actually
		// separates the two zones is below.
		run := time.Date(2026, 10, 19, 3, 55, 0, 0, time.UTC)
		if got := worklogsreport.DailyDate(run, loc).Format("2006-01-02"); got != "2026-10-18" {
			t.Errorf("daily date = %s, the delivered mail says 2026-10-18", got)
		}
	})

	t.Run("a late-evening UTC run is already tomorrow in Colombo", func(t *testing.T) {
		// 19:00 UTC on the 18th is 00:30 on the 19th in Colombo. A UTC
		// derivation would report the 17th; Colombo's answer is the
		// 18th, and Colombo is the zone the business day is measured in.
		run := time.Date(2026, 10, 18, 19, 0, 0, 0, time.UTC)
		if got := worklogsreport.DailyDate(run, loc).Format("2006-01-02"); got != "2026-10-18" {
			t.Errorf("daily date = %s, want 2026-10-18 (UTC would wrongly say 2026-10-17)", got)
		}
		from, to := worklogsreport.WeeklyRange(run, loc)
		if from.Format("2006-01-02") != "2026-10-12" || to.Format("2006-01-02") != "2026-10-18" {
			t.Errorf("weekly = %s..%s, want 2026-10-12..2026-10-18",
				from.Format("2006-01-02"), to.Format("2006-01-02"))
		}
	})

	t.Run("the returned dates carry no time-of-day", func(t *testing.T) {
		// work_date is a bare DATE upstream; a range carrying a clock
		// time would make the boundary depend on when the cron ran.
		from, to := worklogsreport.WeeklyRange(time.Now(), loc)
		for _, d := range []time.Time{from, to, worklogsreport.DailyDate(time.Now(), loc)} {
			if d.Hour() != 0 || d.Minute() != 0 || d.Second() != 0 {
				t.Errorf("%v carries a time of day", d)
			}
		}
	})
}
