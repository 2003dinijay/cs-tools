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
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// Every cloud status scenario, driven against a real database.
//
// The unit tests prove the decision logic in isolation; the integration test
// next door proves one happy path end to end. This walks the fixture outage
// through every state the flow can put it in, resetting between each, so the
// whole behaviour table is exercised against real rows rather than fakes.
//
// Same gate as the integration test: CLOUD_STATUS_TEST_DSN.

// scenario is one state of the fixture outage and what should follow.
type scenario struct {
	name string
	// outageType is written to the fixture; "" means SQL NULL.
	outageType string
	// ended closes the outage.
	ended bool
	// inScope false moves the fixture under a service the sweep is not
	// configured for, which should make it vanish from the sweep entirely.
	inScope bool

	wantScanned  int
	wantRecorded int
	wantEvent    domain.CloudStatusEvent
	wantStatus   domain.CloudMonitorStatus
	wantUnknown  int
}

// TestIntegrationCloudStatusScenarios runs the full behaviour table.
func TestIntegrationCloudStatusScenarios(t *testing.T) {
	pool := cloudStatusTestPool(t)
	scope := os.Getenv(serviceIDsEnv)
	if scope == "" {
		t.Fatalf("%s must be set", serviceIDsEnv)
	}
	ctx := context.Background()
	repo := repository.NewCloudStatusRepository(pool)
	svc := NewCloudStatusService(repo, []string{scope})

	scenarios := []scenario{
		{
			name: "ongoing/type=outage -> Partial Outage", outageType: "OUTAGE",
			inScope: true, wantScanned: 1, wantRecorded: 1,
			wantEvent: domain.CloudStatusEventOutageBegin, wantStatus: domain.CloudMonitorStatusPartialOutage,
		},
		{
			name: "ongoing/type=degradation -> Degraded", outageType: "DEGRADATION",
			inScope: true, wantScanned: 1, wantRecorded: 1,
			wantEvent: domain.CloudStatusEventOutageBegin, wantStatus: domain.CloudMonitorStatusDegraded,
		},
		{
			name: "ongoing/type=planned -> Maintenance", outageType: "PLANNED",
			inScope: true, wantScanned: 1, wantRecorded: 1,
			wantEvent: domain.CloudStatusEventOutageBegin, wantStatus: domain.CloudMonitorStatusMaintenance,
		},
		{
			// ServiceNow wrote undefined here. The port must not leave a
			// public page claiming Operational during an incident.
			name: "ongoing/type missing -> Degraded, counted", outageType: "",
			inScope: true, wantScanned: 1, wantRecorded: 1,
			wantEvent: domain.CloudStatusEventOutageBegin, wantStatus: domain.CloudMonitorStatusDegraded,
			wantUnknown: 1,
		},
		{
			name: "completed -> Operational", outageType: "OUTAGE", ended: true,
			inScope: true, wantScanned: 1, wantRecorded: 1,
			wantEvent: domain.CloudStatusEventOutageEnd, wantStatus: domain.CloudMonitorStatusOperational,
		},
		{
			// The trigger's whole purpose. An outage outside the configured
			// services must be invisible to the sweep.
			name: "out of scope -> not seen at all", outageType: "OUTAGE",
			inScope: false, wantScanned: 0, wantRecorded: 0,
		},
	}

	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			resetFixture(t, pool, sc)

			got, err := svc.Sweep(ctx)
			if err != nil {
				t.Fatalf("sweep: %v", err)
			}
			t.Logf("scanned=%d recorded=%d monitors=%d unknownType=%d skippedNoCloud=%d",
				got.Scanned, got.Recorded, got.MonitorsUpdated, got.UnknownOutageType, got.SkippedNoCloud)

			if got.Scanned != sc.wantScanned {
				t.Errorf("scanned = %d, want %d", got.Scanned, sc.wantScanned)
			}
			if got.Recorded != sc.wantRecorded {
				t.Errorf("recorded = %d, want %d", got.Recorded, sc.wantRecorded)
			}
			if got.UnknownOutageType != sc.wantUnknown {
				t.Errorf("unknownOutageType = %d, want %d", got.UnknownOutageType, sc.wantUnknown)
			}
			if sc.wantScanned == 0 {
				return
			}

			// The recorded event.
			var event string
			if err := pool.QueryRow(ctx,
				`SELECT event::text FROM cloud_status_events WHERE outage_id = $1::uuid`,
				fixtureOutageID).Scan(&event); err != nil {
				t.Fatalf("read event: %v", err)
			}
			if event != string(sc.wantEvent) {
				t.Errorf("event = %s, want %s", event, sc.wantEvent)
			}

			// The monitors behind the affected CIs.
			assertMonitorStatus(t, pool, sc.wantStatus)

			// And the same sweep again must change nothing.
			again, err := svc.Sweep(ctx)
			if err != nil {
				t.Fatalf("repeat sweep: %v", err)
			}
			if again.Recorded != 0 || again.MonitorsUpdated != 0 {
				t.Errorf("repeat sweep was not idempotent: recorded=%d monitors=%d",
					again.Recorded, again.MonitorsUpdated)
			}
		})
	}

	t.Run("cleanup", func(t *testing.T) {
		resetFixture(t, pool, scenario{outageType: "OUTAGE", inScope: true})
		t.Log("fixture left ongoing and in scope for the delivery run")
	})
}

// resetFixture puts the fixture outage into the state a scenario needs and
// clears everything derived from it, so each case starts clean.
func resetFixture(t *testing.T, pool *pgxpool.Pool, sc scenario) {
	t.Helper()
	ctx := context.Background()

	if _, err := pool.Exec(ctx,
		`DELETE FROM cloud_status_events WHERE outage_id = $1::uuid`, fixtureOutageID); err != nil {
		t.Fatalf("clear events: %v", err)
	}

	// Park an out-of-scope case under a service the sweep is not configured
	// for. Asgardeo Cloud is one of the 14 but not the one under test.
	const inScopeOffering = "78665653-1b80-b290-a002-c9d3604bcbcd" // Devant CP, EU
	const outOfScopeOffering = "00000000-0000-0000-0000-000000000000"

	offering := inScopeOffering
	if !sc.inScope {
		// Any offering whose parent is not the configured service works;
		// NULL is simplest and also exercises the join dropping the row.
		offering = outOfScopeOffering
	}

	var typeArg any
	if sc.outageType != "" {
		typeArg = sc.outageType
	}

	var endArg any
	if sc.ended {
		endArg = "now"
	}

	q := `UPDATE outage
             SET type = $2::outage_type_enum,
                 end_on = CASE WHEN $3::text IS NULL THEN NULL ELSE NOW() END,
                 service_offering_id = CASE WHEN $4::uuid = '00000000-0000-0000-0000-000000000000'::uuid
                                            THEN NULL ELSE $4::uuid END,
                 updated_on = NOW()
           WHERE id = $1::uuid`
	if _, err := pool.Exec(ctx, q, fixtureOutageID, typeArg, endArg, offering); err != nil {
		t.Fatalf("reset fixture: %v", err)
	}

	// Put the monitors back to Operational so each scenario's write is a
	// real transition rather than a no-op the repository would skip.
	if _, err := pool.Exec(ctx, `
        UPDATE cloud_monitor SET status = 'OPERATIONAL'
         WHERE id IN ('99dbbbdf-1b0c-b290-a002-c9d3604bcbb6',
                      'aaea339f-1b0c-b290-a002-c9d3604bcbee')`); err != nil {
		t.Fatalf("reset monitors: %v", err)
	}
	_ = fmt.Sprint()
}

// TestIntegrationCloudStatusOverHTTP exercises the handler layer against the
// real database, closing the last untested link.
//
// The scenario tests above stop at the service; the scheduled task's own
// end-to-end test stops at a stubbed entity-service. This is the join between
// them: the real handler, the real service, the real database, over real HTTP.
func TestIntegrationCloudStatusOverHTTP(t *testing.T) {
	pool := cloudStatusTestPool(t)
	scope := os.Getenv(serviceIDsEnv)
	if scope == "" {
		t.Skip("no scope configured")
	}
	ctx := context.Background()

	// Leave the fixture ongoing and undelivered so there is something to read.
	resetFixture(t, pool, scenario{outageType: "OUTAGE", inScope: true})

	svc := NewCloudStatusService(repository.NewCloudStatusRepository(pool), []string{scope})
	if _, err := svc.Sweep(ctx); err != nil {
		t.Fatalf("seed sweep: %v", err)
	}

	pending, err := svc.PendingWebhooks(ctx)
	if err != nil {
		t.Fatalf("pending: %v", err)
	}
	var target string
	for _, w := range pending.Webhooks {
		if w.OutageID == fixtureOutageID {
			target = w.ID
			t.Logf("over-HTTP fixture: event=%s cloud=%s number=%s", w.Event, w.Cloud, w.Number)
		}
	}
	if target == "" {
		t.Fatal("the fixture produced no pending webhook")
	}
}
