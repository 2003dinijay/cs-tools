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
	"errors"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// tierKeyPrefix namespaces this engine's Redis keys — one plain string key
// per (caseID, clockType) pair, value = the highest tier (0/50/75/100) this
// engine has already alerted for (or seeded as a baseline — see Engine.
// processStatus in engine.go). Replaces the old wake-index ZSET design: with
// no due date of our own to schedule against anymore (see client.go's
// package doc comment for why), there's nothing to schedule — only a
// per-clock "have we already alerted for this" cursor to remember between
// polls.
const tierKeyPrefix = "sla:tier:"

// tierTTL bounds how long a clock's cursor survives with no further Tick
// touching it — entity-service's GET /sla-status only ever returns
// currently-active clocks, so a clock that completes/closes simply stops
// appearing and this engine has no explicit "clock finished" signal to react
// to. A generous TTL (refreshed on every Tick that still sees the clock —
// see setTier's caller) lets a stale cursor for a long-finished case expire
// on its own rather than accumulating in Redis forever; it comfortably
// outlives any realistic case lifetime, so it never fires while a clock is
// still genuinely active.
const tierTTL = 90 * 24 * time.Hour

// TierStore wraps the small set of Redis operations this engine needs —
// first Redis dependency in this repo (see this package's own CLAUDE.md
// section) — local for now (REDIS_ADDR), Azure Cache for Redis later via
// the same protocol/client, only a connection-string/TLS change.
type TierStore struct {
	rdb *redis.Client
}

// NewTierStore constructs a TierStore. Connecting is lazy — go-redis dials
// on first use, not here — so a wrong addr only surfaces as an error from
// the first call below, matching every other lazy-connect client in this
// repo (e.g. eventbus.NewProducer).
func NewTierStore(rdb *redis.Client) *TierStore {
	return &TierStore{rdb: rdb}
}

func tierKey(caseID, clockType string) string {
	return tierKeyPrefix + caseID + "|" + clockType
}

// GetTier returns the last tier recorded for (caseID, clockType), and
// whether a cursor exists at all — found=false means this engine has never
// seen this clock before (or its cursor expired), which Engine.processStatus
// treats as "seed a baseline, don't alert" rather than "alert for
// everything up to its current tier."
func (s *TierStore) GetTier(ctx context.Context, caseID, clockType string) (tier int, found bool, err error) {
	val, err := s.rdb.Get(ctx, tierKey(caseID, clockType)).Result()
	if errors.Is(err, redis.Nil) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	tier, err = strconv.Atoi(val)
	if err != nil {
		return 0, false, err
	}
	return tier, true, nil
}

// SetTier records tier as the last tier reached for (caseID, clockType),
// refreshing tierTTL. Called both to seed/reseed a baseline (no alert sent)
// and to record a tier this call just alerted for.
func (s *TierStore) SetTier(ctx context.Context, caseID, clockType string, tier int) error {
	return s.rdb.Set(ctx, tierKey(caseID, clockType), strconv.Itoa(tier), tierTTL).Err()
}
