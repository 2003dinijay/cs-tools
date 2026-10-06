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
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// wakeKey is the single Redis sorted-set key this engine uses as its
// scheduling index: member = "<caseId>|<clockType>|<tier>", score = the Unix
// timestamp that member becomes due at. One key for the whole engine (not
// one per case) — the ZRANGE ... BYSCORE query below scans the whole set in
// one round trip per tick regardless of how many clocks are registered.
// Ported from the pre-poll design's own WakeIndex (see this package's own
// CLAUDE.md section for the history) — ClockMeta/the alerted-tier cursor
// below are new, replacing that design's entity-service-backed durable
// clock row.
const wakeKey = "sla:wake"

// clockKeyPrefix namespaces one HASH per (caseID, clockType) pair, holding
// everything this engine needs to know about that clock: the Chat card's
// own display fields (set once at RegisterClocks, read back unchanged at
// alert time — there is no live entity-service lookup to refresh them from
// any more), the paused flag ApplyStateEffects toggles, and the
// alertedTier cursor Tick/CompleteResponseClock/ApplyStateEffects advance.
const clockKeyPrefix = "sla:clock:"

// clockTTL bounds how long a clock's hash survives with no further write
// touching it. This engine has no explicit "case closed for good, delete
// everything" signal of its own (ApplyStateEffects' CLOSED branch still
// writes a completion, which refreshes this same TTL) — a generous TTL,
// refreshed on every touch, lets a long-finished case's hash expire on its
// own rather than accumulating forever, the same reasoning the removed
// poll design's own tierTTL gave for its cursor keys.
const clockTTL = 90 * 24 * time.Hour

// tierClaimKeyPrefix namespaces one key per (caseID, clockType, tier) —
// claimed via ClaimTier's Redis SETNX before Engine.alertTier ever runs, so
// that if this service is ever deployed with more than one replica, only
// the replica that wins the SETNX race sends that tier's alert. Ported
// unchanged from the removed poll design's own TierStore — the reasoning
// is identical: two replicas racing on the same due wake member must not
// both alert.
const tierClaimKeyPrefix = "sla:tier-claimed:"

// ClockMeta is one (caseID, clockType) clock's full Redis-held state —
// RegisterClocks writes CaseNumber..StartedAt once; Paused/AlertedTier are
// mutated by ApplyStateEffects/CompleteResponseClock/Tick afterward.
type ClockMeta struct {
	CaseNumber string
	WSO2CaseID string
	CaseTitle  string
	CaseType   string
	Product    string
	Team       string
	Priority   string
	// State is the case's own display-label status (e.g. "Work In
	// Progress"), refreshed by ApplyStateEffects on every case.status_changed
	// — "" until the first status change, since case.created carries no
	// status field of its own (a brand-new case is always freshly Open).
	State     string
	StartedAt time.Time
	Paused    bool
	// AlertedTier is the highest tier (0/50/75/100) already alerted for, OR
	// force-completed via CompleteResponseClock/ApplyStateEffects' CLOSED
	// branch — Tick drops a due wake member outright once its own tier is
	// at or below this value, the same "already handled, don't re-alert"
	// cursor the removed poll design's own TierStore kept, just stored
	// alongside the clock's own metadata instead of as a separate key.
	AlertedTier int
}

// Store wraps every Redis operation this engine needs — first Redis
// dependency in this repo (see this package's own CLAUDE.md section),
// local for now (REDIS_ADDR), Azure Cache for Redis later via the same
// protocol/client, only a connection-string/TLS change.
type Store struct {
	rdb *redis.Client
}

// NewStore constructs a Store. Connecting is lazy — go-redis dials on first
// use, not here — so a wrong addr only surfaces as an error from the first
// call below, matching every other lazy-connect client in this repo (e.g.
// eventbus.NewProducer).
func NewStore(rdb *redis.Client) *Store {
	return &Store{rdb: rdb}
}

func clockKey(caseID, clockType string) string {
	return clockKeyPrefix + caseID + "|" + clockType
}

func tierClaimKey(caseID, clockType string, tier int) string {
	return tierClaimKeyPrefix + caseID + "|" + clockType + "|" + strconv.Itoa(tier)
}

// AddWake schedules member to become due at at.
func (s *Store) AddWake(ctx context.Context, member string, at time.Time) error {
	return s.rdb.ZAdd(ctx, wakeKey, redis.Z{Score: float64(at.Unix()), Member: member}).Err()
}

// RemoveWake drops member from the index — Tick's processDueMember calls
// this exactly once per member it examines, regardless of outcome (alerted,
// already handled, paused, or malformed): see that function's own doc
// comment for why a member must never be left to be re-examined on every
// future tick forever once its due time has passed.
func (s *Store) RemoveWake(ctx context.Context, member string) error {
	return s.rdb.ZRem(ctx, wakeKey, member).Err()
}

// DueMembers returns every member whose score (epoch seconds) is <= now.
func (s *Store) DueMembers(ctx context.Context, now time.Time) ([]string, error) {
	return s.rdb.ZRangeArgs(ctx, redis.ZRangeArgs{
		Key:     wakeKey,
		Start:   0,
		Stop:    now.Unix(),
		ByScore: true,
	}).Result()
}

// SetClock writes a clock's full display-field set — called once, from
// RegisterClocks, when the clock is first created. AlertedTier/Paused are
// always written 0/false here: a freshly registered clock has never been
// alerted for and is never born paused.
func (s *Store) SetClock(ctx context.Context, caseID, clockType string, meta ClockMeta) error {
	key := clockKey(caseID, clockType)
	if err := s.rdb.HSet(ctx, key,
		"caseNumber", meta.CaseNumber,
		"wso2CaseId", meta.WSO2CaseID,
		"caseTitle", meta.CaseTitle,
		"caseType", meta.CaseType,
		"product", meta.Product,
		"team", meta.Team,
		"priority", meta.Priority,
		"state", meta.State,
		"startedAt", meta.StartedAt.Unix(),
		"paused", boolString(meta.Paused),
		"alertedTier", meta.AlertedTier,
	).Err(); err != nil {
		return err
	}
	return s.rdb.Expire(ctx, key, clockTTL).Err()
}

// GetClock reads one clock's full state back. found=false means this
// engine has no record of this (caseID, clockType) pair at all — either it
// was never registered (e.g. a case that already existed before this
// feature deployed — see RegisterClocks' own doc comment on backfill), or
// its hash expired.
func (s *Store) GetClock(ctx context.Context, caseID, clockType string) (meta ClockMeta, found bool, err error) {
	res, err := s.rdb.HGetAll(ctx, clockKey(caseID, clockType)).Result()
	if err != nil {
		return ClockMeta{}, false, err
	}
	if len(res) == 0 {
		return ClockMeta{}, false, nil
	}
	meta = ClockMeta{
		CaseNumber: res["caseNumber"],
		WSO2CaseID: res["wso2CaseId"],
		CaseTitle:  res["caseTitle"],
		CaseType:   res["caseType"],
		Product:    res["product"],
		Team:       res["team"],
		Priority:   res["priority"],
		State:      res["state"],
		Paused:     res["paused"] == "1",
	}
	if v, err := strconv.ParseInt(res["startedAt"], 10, 64); err == nil {
		meta.StartedAt = time.Unix(v, 0)
	}
	if v, err := strconv.Atoi(res["alertedTier"]); err == nil {
		meta.AlertedTier = v
	}
	return meta, true, nil
}

// SetPaused toggles one clock's paused flag — ApplyStateEffects' only write
// for the AWAITING_INFO/SOLUTION_PROPOSED/resume branches. A no-op (HSET
// creates a near-empty hash) against a clock this engine never registered —
// harmless, since without a RegisterClocks call there is also no wake entry
// for Tick to ever examine against it.
func (s *Store) SetPaused(ctx context.Context, caseID, clockType string, paused bool) error {
	key := clockKey(caseID, clockType)
	if err := s.rdb.HSet(ctx, key, "paused", boolString(paused)).Err(); err != nil {
		return err
	}
	return s.rdb.Expire(ctx, key, clockTTL).Err()
}

// SetState refreshes one clock's own display State field — called from
// ApplyStateEffects on every case.status_changed, so a breach alert fired
// later shows the case's current status, not a stale "" from registration.
func (s *Store) SetState(ctx context.Context, caseID, clockType, state string) error {
	key := clockKey(caseID, clockType)
	if err := s.rdb.HSet(ctx, key, "state", state).Err(); err != nil {
		return err
	}
	return s.rdb.Expire(ctx, key, clockTTL).Err()
}

// advanceAlertedTierScript atomically sets a clock hash's alertedTier field
// to ARGV[1] only if it is currently absent or lower than ARGV[1] — never
// moving it backward. Mirrors the removed poll design's own TierStore
// advanceTierScript exactly, just against a hash field instead of a plain
// string key — see that script's own doc comment (preserved in git
// history) for the full concurrency reasoning: two callers (a Tick claim
// and CompleteResponseClock/ApplyStateEffects' CLOSED branch, say) must
// never let whichever writes second silently move the cursor backward.
var advanceAlertedTierScript = redis.NewScript(`
local current = redis.call('HGET', KEYS[1], 'alertedTier')
local candidate = tonumber(ARGV[1])
if (not current) or (candidate > tonumber(current)) then
	redis.call('HSET', KEYS[1], 'alertedTier', ARGV[1])
end
redis.call('EXPIRE', KEYS[1], ARGV[2])
return 1
`)

// AdvanceAlertedTier atomically records tier as the highest tier
// alerted/completed for (caseID, clockType) — but only if the currently
// stored value is absent or lower. Used both by Tick (right after a
// successful alert) and by CompleteResponseClock/ApplyStateEffects' CLOSED
// branch (passing 100 to force-complete every tier at once, matching the
// removed pre-poll design's "mark all three tiers reached" semantics for an
// early completion).
func (s *Store) AdvanceAlertedTier(ctx context.Context, caseID, clockType string, tier int) error {
	return advanceAlertedTierScript.Run(ctx, s.rdb, []string{clockKey(caseID, clockType)}, tier, int(clockTTL.Seconds())).Err()
}

// ClaimTier atomically claims (caseID, clockType, tier) via Redis SETNX —
// claimed=true means this call is the one that just claimed it and should
// go on to alert; claimed=false means some other call (a concurrent
// replica, or an earlier attempt) already holds the claim.
func (s *Store) ClaimTier(ctx context.Context, caseID, clockType string, tier int) (claimed bool, err error) {
	return s.rdb.SetNX(ctx, tierClaimKey(caseID, clockType, tier), 1, clockTTL).Result()
}

// ReleaseTier gives back a claim made by ClaimTier — called when a claimed
// tier's alert fails to send (the Kafka publish specifically — see
// Engine.alertTier's own doc comment for why a Chat-send failure doesn't
// trigger this), so a later tick can retry it instead of losing it for
// good.
func (s *Store) ReleaseTier(ctx context.Context, caseID, clockType string, tier int) error {
	return s.rdb.Del(ctx, tierClaimKey(caseID, clockType, tier)).Err()
}

func boolString(b bool) string {
	if b {
		return "1"
	}
	return "0"
}
