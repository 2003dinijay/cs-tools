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
package usagerollup

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

type fakeRoller struct {
	days    []string
	months  []string
	failDay string
	failMon string
}

func (f *fakeRoller) RollupDaily(_ context.Context, day string) (int, error) {
	f.days = append(f.days, day)
	if day == f.failDay {
		return 0, errors.New("boom")
	}
	return 3, nil
}

func (f *fakeRoller) RollupMonthly(_ context.Context, year, month int) (int, error) {
	key := fmt.Sprintf("%04d-%02d", year, month)
	f.months = append(f.months, key)
	if key == f.failMon {
		return 0, errors.New("boom")
	}
	return 2, nil
}

func clock(t string) func() time.Time {
	return func() time.Time {
		v, err := time.Parse(time.RFC3339, t)
		if err != nil {
			panic(err)
		}
		return v
	}
}

func TestDailyRollup_RebuildsTheLastDaysNewestFirstAndLeavesTodayOut(t *testing.T) {
	r := &fakeRoller{}
	if err := DailyRollup(r, 3, clock("2026-10-22T00:30:00Z"))(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := strings.Join(r.days, ","); got != "2026-10-21,2026-10-20,2026-10-19" {
		t.Errorf("days = %s", got)
	}
}

func TestDailyRollup_CrossesMonthAndYearBoundaries(t *testing.T) {
	r := &fakeRoller{}
	if err := DailyRollup(r, 2, clock("2027-01-01T00:30:00Z"))(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := strings.Join(r.days, ","); got != "2026-12-31,2026-12-30" {
		t.Errorf("days = %s", got)
	}
}

func TestDailyRollup_UsesTheUTCDateNotTheLocalOne(t *testing.T) {
	// 23:30 on the 21st in UTC-5 is already 04:30 on the 22nd UTC.
	zone := time.FixedZone("UTC-5", -5*3600)
	now := func() time.Time { return time.Date(2026, 10, 21, 23, 30, 0, 0, zone) }
	r := &fakeRoller{}
	if err := DailyRollup(r, 1, now)(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(r.days) != 1 || r.days[0] != "2026-10-21" {
		t.Errorf("days = %v, want [2026-10-21]", r.days)
	}
}

func TestDailyRollup_OneFailingDayDoesNotStopTheOthers(t *testing.T) {
	r := &fakeRoller{failDay: "2026-10-20"}
	err := DailyRollup(r, 3, clock("2026-10-22T00:30:00Z"))(context.Background())
	if err == nil {
		t.Fatal("a failed day reported success")
	}
	if !strings.Contains(err.Error(), "2026-10-20") || strings.Contains(err.Error(), "2026-10-21") {
		t.Errorf("error should name only the failed day: %v", err)
	}
	if len(r.days) != 3 {
		t.Errorf("days attempted = %v, want all three", r.days)
	}
}

func TestDailyRollup_LookbackBelowOneStillDoesYesterday(t *testing.T) {
	r := &fakeRoller{}
	if err := DailyRollup(r, 0, clock("2026-10-22T00:30:00Z"))(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(r.days) != 1 || r.days[0] != "2026-10-21" {
		t.Errorf("days = %v", r.days)
	}
}

func TestMonthlyRollup_PreviousMonthThenCurrent(t *testing.T) {
	r := &fakeRoller{}
	if err := MonthlyRollup(r, clock("2026-10-22T01:30:00Z"))(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := strings.Join(r.months, ","); got != "2026-09,2026-10" {
		t.Errorf("months = %s", got)
	}
}

func TestMonthlyRollup_JanuaryLooksBackToDecemberOfLastYear(t *testing.T) {
	r := &fakeRoller{}
	if err := MonthlyRollup(r, clock("2027-01-03T01:30:00Z"))(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := strings.Join(r.months, ","); got != "2026-12,2027-01" {
		t.Errorf("months = %s", got)
	}
}

func TestMonthlyRollup_OneFailingMonthDoesNotStopTheOther(t *testing.T) {
	r := &fakeRoller{failMon: "2026-09"}
	err := MonthlyRollup(r, clock("2026-10-22T01:30:00Z"))(context.Background())
	if err == nil || !strings.Contains(err.Error(), "2026-09") {
		t.Fatalf("error = %v, want one naming 2026-09", err)
	}
	if len(r.months) != 2 {
		t.Errorf("months attempted = %v, want both", r.months)
	}
}
