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
	"log/slog"
	"time"
)

// Roller is the subset of *Client this package's handlers depend on, declared
// here so a test can substitute a fake.
type Roller interface {
	RollupDaily(ctx context.Context, day string) (int, error)
	RollupMonthly(ctx context.Context, year, month int) (int, error)
}

// DailyRollup returns a registry.Task.Handler that rebuilds the daily usage
// rows of the last lookbackDays UTC days before today, newest first.
//
// Today is left out on purpose: ingestion keeps the current day's rows up to
// date as reports arrive, so this run is the reconcile that settles each day
// once it is over. Several days are rebuilt rather than only yesterday so a
// report that reaches PCT late, or a night this task did not run, is picked up
// by the next run. The rollup is idempotent, so the overlap costs nothing.
//
// One failing day does not stop the others; the handler returns every failure
// together, so the engine retries the whole (idempotent) run.
func DailyRollup(r Roller, lookbackDays int, now func() time.Time) func(ctx context.Context) error {
	if lookbackDays < 1 {
		lookbackDays = 1
	}
	return func(ctx context.Context) error {
		today := now().UTC()
		var errs []error
		for i := 1; i <= lookbackDays; i++ {
			day := today.AddDate(0, 0, -i).Format("2006-01-02")
			written, err := r.RollupDaily(ctx, day)
			if err != nil {
				errs = append(errs, fmt.Errorf("daily usage rollup for %s: %w", day, err))
				continue
			}
			slog.InfoContext(ctx, "usage rollup: daily rows rebuilt", "day", day, "rowsWritten", written)
		}
		return errors.Join(errs...)
	}
}

// MonthlyRollup returns a registry.Task.Handler that rebuilds the monthly usage
// rows of the previous calendar month and then the current one (so a month in
// progress already shows its running figure). Previous month first: it is the
// one that must end up final. A failure in one month does not stop the other.
func MonthlyRollup(r Roller, now func() time.Time) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		current := now().UTC()
		firstOfCurrent := time.Date(current.Year(), current.Month(), 1, 0, 0, 0, 0, time.UTC)
		previous := firstOfCurrent.AddDate(0, -1, 0)

		var errs []error
		for _, month := range []time.Time{previous, firstOfCurrent} {
			written, err := r.RollupMonthly(ctx, month.Year(), int(month.Month()))
			if err != nil {
				errs = append(errs, fmt.Errorf("monthly usage rollup for %s: %w", month.Format("2006-01"), err))
				continue
			}
			slog.InfoContext(ctx, "usage rollup: monthly rows rebuilt", "month", month.Format("2006-01"), "rowsWritten", written)
		}
		return errors.Join(errs...)
	}
}
