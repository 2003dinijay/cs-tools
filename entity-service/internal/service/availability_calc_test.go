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
	"math"
	"testing"
	"time"
)

func availAt(h, m int) time.Time {
	return time.Date(2026, 10, 1, h, m, 0, 0, time.UTC)
}

func availDay() (time.Time, time.Time) {
	return time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
}

// businessHours covers 09:00-17:00 UTC each day. Used to prove the scheduled
// side actually differs from the absolute one — with the 24x7 schedule every
// commitment on the instance uses, the two are identical and a port could get
// the scheduled arm completely wrong without a single test noticing.
type businessHours struct{}

func (businessHours) Covered(begin, end time.Time) time.Duration {
	var total time.Duration
	for d := begin.Truncate(24 * time.Hour); d.Before(end); d = d.AddDate(0, 0, 1) {
		ws := time.Date(d.Year(), d.Month(), d.Day(), 9, 0, 0, 0, d.Location())
		we := time.Date(d.Year(), d.Month(), d.Day(), 17, 0, 0, 0, d.Location())
		s, e := ws, we
		if begin.After(s) {
			s = begin
		}
		if end.Before(e) {
			e = end
		}
		if e.After(s) {
			total += e.Sub(s)
		}
	}
	return total
}

func approx(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.000001 {
		t.Errorf("%s = %.6f, want %.6f", name, got, want)
	}
}

// *** THE TEST THAT CATCHES THE MOST LIKELY WRONG PORT. ***
// Every piece of product documentation says a planned outage "reduces agreed
// service time". The V2 source does no such thing: the accumulator skips any
// type that is not exactly "outage", so a planned outage changes NOTHING —
// not downtime, not AST, not the count.
//
// An implementation that subtracted planned time from AST would produce
// availability BELOW 100 here, and one that treated it as downtime would too.
// Both are the obvious reading of the field names.
func TestCalculateAvailability_PlannedOutageChangesNothing(t *testing.T) {
	begin, end := availDay()

	withPlanned := CalculateAvailability(AvailabilityInputs{
		Begin: begin, End: end, TargetPercent: 100,
		Outages: []AvailabilityOutage{
			{Begin: availAt(2, 0), End: availAt(6, 0), Type: OutageTypePlanned},
		},
	})
	withNothing := CalculateAvailability(AvailabilityInputs{
		Begin: begin, End: end, TargetPercent: 100,
	})

	if withPlanned != withNothing {
		t.Fatalf("a planned outage changed the result.\n with planned: %+v\n with none:    %+v",
			withPlanned, withNothing)
	}
	approx(t, "absoluteAvailability", withPlanned.AbsoluteAvailability, 100)
	if withPlanned.ScheduledTotal != 24*time.Hour {
		t.Errorf("AST = %v, want 24h — a planned outage must not shrink it", withPlanned.ScheduledTotal)
	}
	if withPlanned.AbsoluteCount != 0 || withPlanned.ScheduledCount != 0 {
		t.Errorf("counts = %d/%d, want 0/0", withPlanned.AbsoluteCount, withPlanned.ScheduledCount)
	}
}

// A degradation must be inert for the same reason, and it is worth asserting
// separately: 115 of the 635 outages on the instance are degradations, and a
// port that mapped the WSO2 three-value enum onto the two-value ServiceNow
// one by treating DEGRADATION as an outage would change 18% of the inputs.
func TestCalculateAvailability_DegradationIsInert(t *testing.T) {
	begin, end := availDay()
	got := CalculateAvailability(AvailabilityInputs{
		Begin: begin, End: end, TargetPercent: 100,
		Outages: []AvailabilityOutage{
			{Begin: availAt(2, 0), End: availAt(6, 0), Type: "degradation"},
		},
	})
	approx(t, "absoluteAvailability", got.AbsoluteAvailability, 100)
	if got.AbsoluteDowntime != 0 {
		t.Errorf("absoluteDowntime = %v, want 0", got.AbsoluteDowntime)
	}
}

func TestCalculateAvailability_SingleOutage(t *testing.T) {
	begin, end := availDay()
	// 1h26m24s is 6% of a day, chosen so the expected answer is exact and a
	// rounding error cannot hide inside it.
	got := CalculateAvailability(AvailabilityInputs{
		Begin: begin, End: end, TargetPercent: 100,
		Outages: []AvailabilityOutage{
			{Begin: availAt(10, 0), End: availAt(10, 0).Add(time.Hour + 26*time.Minute + 24*time.Second),
				Type: OutageTypeOutage},
		},
	})

	if got.AbsoluteDowntime != time.Hour+26*time.Minute+24*time.Second {
		t.Errorf("absoluteDowntime = %v", got.AbsoluteDowntime)
	}
	approx(t, "absoluteAvailability", got.AbsoluteAvailability, 94)
	approx(t, "scheduledAvailability", got.ScheduledAvailability, 94)
	if got.AbsoluteCount != 1 || got.ScheduledCount != 1 {
		t.Errorf("counts = %d/%d, want 1/1", got.AbsoluteCount, got.ScheduledCount)
	}
	// target 100 -> zero tolerance, which is the live configuration on all
	// seven commitments.
	if got.AllowedDowntime != 0 {
		t.Errorf("allowedDowntime = %v, want 0", got.AllowedDowntime)
	}
	if got.CommitmentMet {
		t.Error("commitmentMet = true, but target is 100% and there was downtime")
	}
}

// Trimming. An outage spanning the period boundary must contribute only its
// overlap — otherwise a multi-day outage is counted in full against every day
// it touches, which inflates downtime by a multiple rather than a little.
func TestCalculateAvailability_OutageIsTrimmedToThePeriod(t *testing.T) {
	begin, end := availDay()
	got := CalculateAvailability(AvailabilityInputs{
		Begin: begin, End: end, TargetPercent: 100,
		Outages: []AvailabilityOutage{{
			Begin: time.Date(2026, 9, 30, 22, 0, 0, 0, time.UTC), // 2h before
			End:   time.Date(2026, 10, 1, 3, 0, 0, 0, time.UTC),  // 3h into
			Type:  OutageTypeOutage,
		}},
	})
	if got.AbsoluteDowntime != 3*time.Hour {
		t.Fatalf("absoluteDowntime = %v, want 3h (the overlap only)", got.AbsoluteDowntime)
	}
}

// With a real schedule the absolute and scheduled arms must diverge. Every
// commitment on the instance is 24x7, so without this test the scheduled arm
// is never actually exercised.
func TestCalculateAvailability_ScheduleSeparatesTheTwoArms(t *testing.T) {
	begin, end := availDay()
	got := CalculateAvailability(AvailabilityInputs{
		Begin: begin, End: end, Schedule: businessHours{}, TargetPercent: 99,
		Outages: []AvailabilityOutage{
			// 04:00-06:00, entirely OUTSIDE business hours.
			{Begin: availAt(4, 0), End: availAt(6, 0), Type: OutageTypeOutage},
			// 10:00-11:00, entirely INSIDE.
			{Begin: availAt(10, 0), End: availAt(11, 0), Type: OutageTypeOutage},
		},
	})

	if got.AbsoluteDowntime != 3*time.Hour {
		t.Errorf("absoluteDowntime = %v, want 3h", got.AbsoluteDowntime)
	}
	if got.ScheduledDowntime != time.Hour {
		t.Errorf("scheduledDowntime = %v, want 1h", got.ScheduledDowntime)
	}
	if got.ScheduledTotal != 8*time.Hour {
		t.Errorf("AST = %v, want 8h", got.ScheduledTotal)
	}

	// *** THE OUT-OF-HOURS OUTAGE MUST NOT BE COUNTED EITHER. ***
	// V2 increments the counter only when the duration is positive, so the
	// 04:00 outage scores zero seconds AND zero count. Counting it would
	// change MTBF and MTRS, which divide by it.
	if got.AbsoluteCount != 2 {
		t.Errorf("absoluteCount = %d, want 2", got.AbsoluteCount)
	}
	if got.ScheduledCount != 1 {
		t.Errorf("scheduledCount = %d, want 1 — an outage outside the schedule is not a counted failure", got.ScheduledCount)
	}

	approx(t, "absoluteAvailability", got.AbsoluteAvailability, 100*21.0/24.0)
	approx(t, "scheduledAvailability", got.ScheduledAvailability, 100*7.0/8.0)

	// MTBF/MTRS divide by the SCHEDULED count (1), never the absolute (2).
	if got.MTBF != 7*time.Hour {
		t.Errorf("MTBF = %v, want 7h", got.MTBF)
	}
	if got.MTRS != time.Hour {
		t.Errorf("MTRS = %v, want 1h", got.MTRS)
	}

	// target 99 over an 8h AST -> 1% of 8h = 4m48s allowed. 1h exceeds it.
	if got.AllowedDowntime != 4*time.Minute+48*time.Second {
		t.Errorf("allowedDowntime = %v, want 4m48s", got.AllowedDowntime)
	}
	if got.CommitmentMet {
		t.Error("commitmentMet = true, but 1h > 4m48s")
	}
}

func TestCalculateAvailability_CommitmentMetWhenInsideTolerance(t *testing.T) {
	begin, end := availDay()
	// target 99 over 24h -> 14m24s allowed.
	got := CalculateAvailability(AvailabilityInputs{
		Begin: begin, End: end, TargetPercent: 99,
		Outages: []AvailabilityOutage{
			{Begin: availAt(3, 0), End: availAt(3, 10), Type: OutageTypeOutage},
		},
	})
	if got.AllowedDowntime != 14*time.Minute+24*time.Second {
		t.Fatalf("allowedDowntime = %v, want 14m24s", got.AllowedDowntime)
	}
	if !got.CommitmentMet {
		t.Errorf("commitmentMet = false, but 10m <= 14m24s")
	}
}

// Zero outages is the overwhelmingly common case — 212,684 of the instance's
// 212,904 rows. MTBF and MTRS must stay zero rather than divide by zero.
func TestCalculateAvailability_NoOutages(t *testing.T) {
	begin, end := availDay()
	got := CalculateAvailability(AvailabilityInputs{Begin: begin, End: end, TargetPercent: 100})

	approx(t, "absoluteAvailability", got.AbsoluteAvailability, 100)
	approx(t, "scheduledAvailability", got.ScheduledAvailability, 100)
	if got.MTBF != 0 || got.MTRS != 0 {
		t.Errorf("MTBF/MTRS = %v/%v, want 0/0", got.MTBF, got.MTRS)
	}
	if !got.CommitmentMet {
		t.Error("commitmentMet = false with no downtime at all")
	}
}

// A schedule that covers none of the period. V2 guards this one explicitly
// and returns 100 rather than NaN.
func TestCalculateAvailability_EmptyScheduleYields100NotNaN(t *testing.T) {
	// A Sunday, where businessHours still covers 09:00-17:00 — so instead
	// use a period wholly outside the window.
	begin := time.Date(2026, 10, 1, 1, 0, 0, 0, time.UTC)
	end := time.Date(2026, 10, 1, 5, 0, 0, 0, time.UTC)
	got := CalculateAvailability(AvailabilityInputs{
		Begin: begin, End: end, Schedule: businessHours{}, TargetPercent: 100,
		Outages: []AvailabilityOutage{
			{Begin: availAt(2, 0), End: availAt(3, 0), Type: OutageTypeOutage},
		},
	})

	if got.ScheduledTotal != 0 {
		t.Fatalf("AST = %v, want 0", got.ScheduledTotal)
	}
	if math.IsNaN(got.ScheduledAvailability) {
		t.Fatal("scheduledAvailability is NaN; V2 guards this case and returns 100")
	}
	approx(t, "scheduledAvailability", got.ScheduledAvailability, 100)
	// The absolute arm still sees it: 1h down in a 4h period.
	approx(t, "absoluteAvailability", got.AbsoluteAvailability, 75)
}

// Overlapping outages are summed, NOT merged. ServiceNow runs a separate
// AvailabilityOutageProcessor to coalesce overlaps before the accumulator
// ever sees them; this function is the accumulator, so it must double-count
// and the caller must coalesce. Asserted so that a later change which
// "helpfully" merges here is caught, because the merge belongs upstream
// where it can be tested against the processor's own rules.
func TestCalculateAvailability_OverlapsAreNotMergedHere(t *testing.T) {
	begin, end := availDay()
	got := CalculateAvailability(AvailabilityInputs{
		Begin: begin, End: end, TargetPercent: 100,
		Outages: []AvailabilityOutage{
			{Begin: availAt(1, 0), End: availAt(3, 0), Type: OutageTypeOutage},
			{Begin: availAt(2, 0), End: availAt(4, 0), Type: OutageTypeOutage},
		},
	})
	if got.AbsoluteDowntime != 4*time.Hour {
		t.Fatalf("absoluteDowntime = %v, want 4h (2+2, summed not merged — "+
			"coalescing is AvailabilityOutageProcessor's job upstream)", got.AbsoluteDowntime)
	}
}
