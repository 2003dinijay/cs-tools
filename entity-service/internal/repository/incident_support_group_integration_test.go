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

package repository

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestSupportGroupOfServiceLive(t *testing.T) {
	dsn := os.Getenv("PG_OUTAGE_TEST_DSN")
	if dsn == "" {
		t.Skip("PG_OUTAGE_TEST_DSN not set; this test needs a real database")
	}
	ctx := WithSystemIdentity(context.Background())
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()
	repo := NewIncidentRepository(NewScoped(pool))

	var withGroup, group, without string
	if err := pool.QueryRow(ctx, `SELECT id::text, support_group_id::text FROM service WHERE support_group_id IS NOT NULL LIMIT 1`).Scan(&withGroup, &group); err != nil {
		t.Skipf("no service with a support group in this database: %v", err)
	}
	if got, err := repo.SupportGroupOfService(ctx, withGroup); err != nil || got != group {
		t.Errorf("service with a support group: got %q err %v, want %q", got, err, group)
	}
	if err := pool.QueryRow(ctx, `SELECT id::text FROM service WHERE support_group_id IS NULL LIMIT 1`).Scan(&without); err == nil {
		if got, err := repo.SupportGroupOfService(ctx, without); err != nil || got != "" {
			t.Errorf("service without one: got %q err %v, want \"\"", got, err)
		}
	}
	if got, err := repo.SupportGroupOfService(ctx, "00000000-0000-0000-0000-000000000001"); err != nil || got != "" {
		t.Errorf("unknown service: got %q err %v, want \"\"", got, err)
	}
}
