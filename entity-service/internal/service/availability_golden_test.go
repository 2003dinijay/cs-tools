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
	"bufio"
	"encoding/json"
	"math"
	"os"
	"strings"
	"testing"
	"time"
)

// Replays real ServiceNow cases through the calculator.
//
// *** THE UNIT TESTS PROVE THE PORT MATCHES MY READING OF THE SOURCE. THIS
// PROVES IT MATCHES THE SOURCE. *** Those are not the same thing, and the
// difference has already cost this port once: the first version skipped
// AvailabilityOutageProcessor, so overlapping outages were summed instead
// of merged and planned windows excused nothing. Every unit test passed,
// because they were written from the same misreading.
//
// The data comes from discovery script 49, which dumps real periods, the
// real outages that fed them, and ServiceNow's own computed answers.
//
// ── WHY v1 ROWS CAN JUDGE A v2 PORT ─────────────────────────────────────
// The instance runs v1 and this is a v2 port, which looks disqualifying.
// It is not, for FIXED period types, because the engines agree where it
// counts: v1 merges overlapping same-type outages (_mergeAfter /
// _mergeBefore), v1 splits an outage around a planned one ("OUTAGE IS SPLIT
// BY EXISTING PLANNED OUTAGE"), and both use the same formulas. The one
// known divergence is rolling-window length — v1 spans N-1 days under
// PRB1304264 — so script 49 emits fixed types only.
//
// ── IF THE FILE IS ABSENT ───────────────────────────────────────────────
// The test skips. That is deliberate: the dump contains real service
// offering identifiers and is pasted in by hand after a run, so CI must not
// fail for its absence. It must also not quietly pass as though verified —
// hence the explicit skip message rather than silence.

const goldenPath = "testdata/availability_golden.jsonl"

// *** ROWS WHERE SERVICENOW DISAGREES WITH ITSELF. ***
// Not tolerated discrepancies — rows proven stale by ServiceNow's own other
// rows. Each needs the evidence written out, because "the golden test has
// an exclusion list" is how a real bug gets parked.
//
// The suite would otherwise be 67/68, and the one failure is not the port.
var knownStaleRows = map[string]string{
	// Four rows are fed by one identical outage, 1758077536..1758089278
	// (11742s). Three agree with this calculator to five decimals:
	//
	//   daily  2025-09-17 00:00   SN 86.40972   mine 86.40972
	//   daily  2025-09-16 18:30   SN 86.40972   mine 86.40972
	//   weekly 2025-09-14 18:30   SN 98.05853   mine 98.05853
	//   weekly 2025-09-15 00:00   SN 98.80456   mine 98.05853  <- this row
	//
	// The two weekly rows cover the same seven-day length and the same
	// single outage, which is wholly inside both — so they CANNOT
	// legitimately differ, and ServiceNow holds both. This row's figure is
	// exactly what an outage ending 4512s earlier would produce, so it is a
	// snapshot taken before the outage was extended and never recalculated.
	// "Recalculate Availability" fires on an end change; it evidently did
	// not reach this row.
	//
	// Worth carrying beyond this test: some of the 212,904 rows being
	// migrated are stale in the same way, so the mirrored history is not
	// self-consistent.
	"db394b511b483a500bb3da47b04bcbf9": "stale: its sibling weekly row, same outage and same window length, agrees with this calculator; this one matches an outage 4512s shorter",
}

type goldenCase struct {
	Kind       string  `json:"kind"`
	Row        string  `json:"row"`
	Type       string  `json:"type"`
	Start      string  `json:"start"`
	End        string  `json:"end"`
	StartEpoch int64   `json:"startEpoch"`
	EndEpoch   int64   `json:"endEpoch"`
	TZ         string  `json:"tz"`
	Target     float64 `json:"target"`
	Schedule   string  `json:"schedule"`
	Want       struct {
		AbsDownSecs   int64   `json:"absDownSecs"`
		SchedDownSecs int64   `json:"schedDownSecs"`
		ASTSecs       int64   `json:"astSecs"`
		AllowedSecs   int64   `json:"allowedSecs"`
		MTBFSecs      int64   `json:"mtbfSecs"`
		MTRSSecs      int64   `json:"mtrsSecs"`
		AbsAvail      float64 `json:"absAvail"`
		SchedAvail    float64 `json:"schedAvail"`
		AbsCount      int     `json:"absCount"`
		SchedCount    int     `json:"schedCount"`
		Met           bool    `json:"met"`
	} `json:"want"`
	Outages []struct {
		B int64  `json:"b"`
		E int64  `json:"e"`
		T string `json:"t"`
	} `json:"outages"`
}

func loadGoldenCases(t *testing.T) []goldenCase {
	t.Helper()
	f, err := os.Open(goldenPath)
	if os.IsNotExist(err) {
		t.Skipf("no golden data at %s — run discovery script 49 against "+
			"ServiceNow and paste its JSON lines in. UNTIL THEN THIS PORT IS "+
			"VERIFIED ONLY AGAINST A READING OF THE SOURCE, NOT AGAINST "+
			"PRODUCTION.", goldenPath)
	}
	if err != nil {
		t.Fatalf("open %s: %v", goldenPath, err)
	}
	defer func() { _ = f.Close() }()

	var cases []goldenCase
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 1<<22)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		// The script's own header lines start with '#'.
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var c goldenCase
		if err := json.Unmarshal([]byte(line), &c); err != nil {
			t.Fatalf("malformed golden line: %v\n%s", err, line)
		}
		cases = append(cases, c)
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("read %s: %v", goldenPath, err)
	}
	if len(cases) == 0 {
		t.Fatalf("%s has no cases; a file with only comments is not verification", goldenPath)
	}
	return cases
}

func TestAvailability_GoldenAgainstServiceNow(t *testing.T) {
	cases := loadGoldenCases(t)

	var downtimeCases int
	for _, c := range cases {
		if c.Kind == "downtime" {
			downtimeCases++
		}
	}
	// *** A SUITE OF ALL-CLEAR ROWS WOULD PASS AGAINST A CALCULATOR THAT
	// ALWAYS RETURNED 100. *** 212,684 of the instance's 212,904 rows are
	// exactly that, so a dump that happened to contain only those would be
	// worthless and would look like a clean run.
	if downtimeCases == 0 {
		t.Fatal("golden data contains no cases with downtime; it cannot " +
			"distinguish this calculator from one that always returns 100")
	}
	var withDurations int
	for _, c := range cases {
		if c.Want.ASTSecs > 0 {
			withDurations++
		}
	}
	t.Logf("replaying %d cases (%d with downtime) from ServiceNow", len(cases), downtimeCases)
	if withDurations == 0 {
		t.Logf("*** DURATIONS NOT VERIFIED: every case has astSecs=0, which is " +
			"script 49's durSecs() helper failing, not real data. " +
			"Percentages, counts and met_commitment ARE verified, and " +
			"absolute_availability pins the downtime to five decimals. " +
			"Fix durSecs and re-dump to compare the raw durations too. ***")
	} else if withDurations < len(cases) {
		t.Logf("durations verified on %d of %d cases", withDurations, len(cases))
	}

	for _, c := range cases {
		t.Run(c.Type+"/"+c.Row, func(t *testing.T) {
			if why, stale := knownStaleRows[c.Row]; stale {
				t.Skipf("KNOWN-STALE SERVICENOW ROW — %s", why)
			}
			outages := make([]AvailabilityOutage, 0, len(c.Outages))
			for _, o := range c.Outages {
				outages = append(outages, AvailabilityOutage{
					Begin: time.Unix(o.B, 0).UTC(),
					End:   time.Unix(o.E, 0).UTC(),
					Type:  o.T,
				})
			}

			// The schedule is NOT reconstructed here. Every commitment on
			// the instance points at the stock "24 x 7" record, so AlwaysOn
			// is exact for this data — and the stored ast confirms it per
			// case, asserted below. A case whose ast is not the full period
			// would mean a narrower schedule appeared and this harness
			// needs the spans; it fails rather than guessing.
			got := CalculateAvailability(AvailabilityInputs{
				Begin:         time.Unix(c.StartEpoch, 0).UTC(),
				End:           time.Unix(c.EndEpoch, 0).UTC(),
				Outages:       outages,
				TargetPercent: c.Target,
			})

			// *** THE DUMP'S DURATION FIELDS CAME BACK ZERO. ***
			// Script 49's durSecs() helper failed against this instance —
			// every astSecs/absDownSecs/mtbfSecs is 0, including on a case
			// reading 79.35% availability, which is impossible. So the
			// stored durations are NOT usable as expectations.
			//
			// The first version of this test skipped any case whose ast did
			// not equal the period. With every ast zero that skipped all 68
			// and reported a clean pass having verified nothing — the exact
			// failure mode a golden harness exists to prevent. Now the
			// durations are simply not compared, loudly, and everything
			// else still is.
			//
			// *** WHAT IS STILL VERIFIED IS THE PART THAT MATTERS. ***
			// absolute_availability is 100*(period-downtime)/period, so
			// matching it to five decimals pins the downtime to within
			// a fraction of a second. The counts pin the merge behaviour.
			// Those two are also exactly what the dashboard publishes.
			period := c.EndEpoch - c.StartEpoch
			if c.Want.ASTSecs > 0 {
				if c.Want.ASTSecs != period {
					t.Skipf("stored ast is %ds for a %ds period, so this "+
						"commitment has a narrower schedule than 24x7; the "+
						"golden harness does not reconstruct spans", c.Want.ASTSecs, period)
				}
				checkSecs(t, "absolute_downtime", got.AbsoluteDowntime, c.Want.AbsDownSecs)
				checkSecs(t, "scheduled_downtime", got.ScheduledDowntime, c.Want.SchedDownSecs)
				checkSecs(t, "ast", got.ScheduledTotal, c.Want.ASTSecs)
				checkSecs(t, "allowed_downtime", got.AllowedDowntime, c.Want.AllowedSecs)
				checkSecs(t, "mtbf", got.MTBF, c.Want.MTBFSecs)
				checkSecs(t, "mtrs", got.MTRS, c.Want.MTRSSecs)
			}

			checkPct(t, "absolute_availability", got.AbsoluteAvailability, c.Want.AbsAvail)
			checkPct(t, "scheduled_availability", got.ScheduledAvailability, c.Want.SchedAvail)

			if got.AbsoluteCount != c.Want.AbsCount {
				t.Errorf("absolute_count = %d, ServiceNow says %d", got.AbsoluteCount, c.Want.AbsCount)
			}
			if got.ScheduledCount != c.Want.SchedCount {
				t.Errorf("scheduled_count = %d, ServiceNow says %d", got.ScheduledCount, c.Want.SchedCount)
			}
			if got.CommitmentMet != c.Want.Met {
				t.Errorf("met_commitment = %v, ServiceNow says %v", got.CommitmentMet, c.Want.Met)
			}
		})
	}
}

func checkSecs(t *testing.T, name string, got time.Duration, wantSecs int64) {
	t.Helper()
	// One second of slack. ServiceNow stores durations to the second and
	// its period boundaries carry the "+1 second to roll to midnight"
	// adjustment, so an exact-nanosecond comparison would fail on rounding
	// rather than on arithmetic.
	gotSecs := int64(got.Round(time.Second) / time.Second)
	if diff := gotSecs - wantSecs; diff > 1 || diff < -1 {
		t.Errorf("%s = %ds, ServiceNow says %ds (off by %ds)", name, gotSecs, wantSecs, diff)
	}
}

func checkPct(t *testing.T, name string, got, want float64) {
	t.Helper()
	// ServiceNow stores these at five decimal places.
	if math.Abs(got-want) > 0.00002 {
		t.Errorf("%s = %.6f, ServiceNow says %.6f", name, got, want)
	}
}
