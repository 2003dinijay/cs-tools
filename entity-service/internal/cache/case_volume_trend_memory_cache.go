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
	"sync"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// caseVolumeMemoryEntry is one cached day in InMemoryCaseVolumeTrendCache.
type caseVolumeMemoryEntry struct {
	count   domain.CaseVolumeDayCount
	expires time.Time
}

// InMemoryCaseVolumeTrendCache is CaseVolumeTrendCache's fallback when no
// Redis is configured (REDIS_URL/REDIS_ADDR unset) -- a plain, per-process
// map, following the same lazy-expiry-sweep-on-access shape as this
// backend's own paging_phones.go cache. Safe to use even with several
// replicas each running their own copy: this cache only ever holds
// read-only, deployment-wide case counts that are identical regardless of
// which replica (or which caller) asks (see service.CaseVolumeTrendCache's
// own doc comment), so each replica simply computing and caching
// independently costs nothing beyond that replica's own first, uncached
// request for a given day.
type InMemoryCaseVolumeTrendCache struct {
	mu      sync.Mutex
	entries map[string]caseVolumeMemoryEntry
}

// NewInMemoryCaseVolumeTrendCache constructs an empty InMemoryCaseVolumeTrendCache.
func NewInMemoryCaseVolumeTrendCache() *InMemoryCaseVolumeTrendCache {
	return &InMemoryCaseVolumeTrendCache{entries: make(map[string]caseVolumeMemoryEntry)}
}

func caseVolumeMemoryKey(filtersKey, day string) string {
	return filtersKey + "|" + day
}

// GetMany returns whichever of the given days are currently cached (and not
// yet expired) for filtersKey, sweeping expired entries for the requested
// keys as it goes.
func (c *InMemoryCaseVolumeTrendCache) GetMany(_ context.Context, filtersKey string, days []string) map[string]domain.CaseVolumeDayCount {
	out := make(map[string]domain.CaseVolumeDayCount, len(days))
	if len(days) == 0 {
		return out
	}
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, d := range days {
		key := caseVolumeMemoryKey(filtersKey, d)
		entry, ok := c.entries[key]
		if !ok {
			continue
		}
		if now.After(entry.expires) {
			delete(c.entries, key)
			continue
		}
		out[d] = entry.count
	}
	return out
}

// Set caches one day's counts for filtersKey, for ttl.
func (c *InMemoryCaseVolumeTrendCache) Set(_ context.Context, filtersKey, day string, dc domain.CaseVolumeDayCount, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[caseVolumeMemoryKey(filtersKey, day)] = caseVolumeMemoryEntry{count: dc, expires: time.Now().Add(ttl)}
}
