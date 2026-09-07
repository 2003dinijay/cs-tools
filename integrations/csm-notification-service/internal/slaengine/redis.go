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

package slaengine

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// wakeKey is the single Redis sorted-set key this engine uses as its
// scheduling index: member = "<caseId>|<clockType>|<tier>", score = the Unix
// timestamp that member becomes due at. One key for the whole engine (not
// one per case) — the ZRANGE ... BYSCORE query below scans the whole set in
// one round trip per tick regardless of how many clocks are registered.
const wakeKey = "sla:wake"

// WakeIndex wraps the Redis ZSET operations this engine needs — a direct
// port of the POC's internal/schedule package, renamed to sit alongside the
// entity-service client above rather than as its own top-level package,
// since nothing outside this engine has a reason to use it. First Redis
// dependency in this repo (see this package's own CLAUDE.md section) — local
// for now (REDIS_ADDR), Azure Cache for Redis later via the same
// protocol/client, only a connection-string/TLS change.
type WakeIndex struct {
	rdb *redis.Client
}

// NewWakeIndex constructs a WakeIndex. Connecting is lazy — go-redis dials
// on first use, not here — so a wrong addr only surfaces as an error from
// the first call below, matching every other lazy-connect client in this
// repo (e.g. eventbus.NewProducer).
func NewWakeIndex(rdb *redis.Client) *WakeIndex {
	return &WakeIndex{rdb: rdb}
}

// AddWake schedules member to become due at at.
func (w *WakeIndex) AddWake(ctx context.Context, member string, at time.Time) error {
	return w.rdb.ZAdd(ctx, wakeKey, redis.Z{Score: float64(at.Unix()), Member: member}).Err()
}

// RemoveWake drops member from the index — call only once whatever fired for
// it has been durably recorded (entity-service's SetTierReachedIfUnset) and
// successfully published (events.TypeSLATierReached), never before: see
// Tick's doc comment in engine.go for why the ordering matters.
func (w *WakeIndex) RemoveWake(ctx context.Context, member string) error {
	return w.rdb.ZRem(ctx, wakeKey, member).Err()
}

// DueMembers returns every member whose score (epoch seconds) is <= now.
// Uses ZRangeArgs (ByScore) rather than the deprecated ZRangeByScore —
// same query, current API.
func (w *WakeIndex) DueMembers(ctx context.Context, now time.Time) ([]string, error) {
	return w.rdb.ZRangeArgs(ctx, redis.ZRangeArgs{
		Key:     wakeKey,
		Start:   0,
		Stop:    now.Unix(),
		ByScore: true,
	}).Result()
}
