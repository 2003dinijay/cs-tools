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

package cache

import (
	"context"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

func TestInMemoryCaseVolumeTrendCache_SetThenGet(t *testing.T) {
	c := NewInMemoryCaseVolumeTrendCache()
	ctx := context.Background()
	c.Set(ctx, "{}", "2026-10-01", domain.CaseVolumeDayCount{Opened: 3, Closed: 1}, time.Hour)

	got := c.GetMany(ctx, "{}", []string{"2026-10-01", "2026-10-02"})
	if len(got) != 1 {
		t.Fatalf("got %d entries, want 1 (only the cached day)", len(got))
	}
	if dc := got["2026-10-01"]; dc.Opened != 3 || dc.Closed != 1 {
		t.Errorf("got %+v, want Opened=3 Closed=1", dc)
	}
}

func TestInMemoryCaseVolumeTrendCache_ExpiredEntryIsAMiss(t *testing.T) {
	c := NewInMemoryCaseVolumeTrendCache()
	ctx := context.Background()
	c.Set(ctx, "{}", "2026-10-01", domain.CaseVolumeDayCount{Opened: 3}, -time.Second)

	got := c.GetMany(ctx, "{}", []string{"2026-10-01"})
	if _, ok := got["2026-10-01"]; ok {
		t.Error("expected an already-expired entry to be a miss")
	}
}

func TestInMemoryCaseVolumeTrendCache_DistinctFiltersKeysDoNotCollide(t *testing.T) {
	c := NewInMemoryCaseVolumeTrendCache()
	ctx := context.Background()
	c.Set(ctx, `{"a":1}`, "2026-10-01", domain.CaseVolumeDayCount{Opened: 1}, time.Hour)
	c.Set(ctx, `{"a":2}`, "2026-10-01", domain.CaseVolumeDayCount{Opened: 2}, time.Hour)

	got1 := c.GetMany(ctx, `{"a":1}`, []string{"2026-10-01"})
	got2 := c.GetMany(ctx, `{"a":2}`, []string{"2026-10-01"})
	if got1["2026-10-01"].Opened != 1 || got2["2026-10-01"].Opened != 2 {
		t.Errorf("two distinct filtersKeys collided: got1=%+v got2=%+v", got1, got2)
	}
}
