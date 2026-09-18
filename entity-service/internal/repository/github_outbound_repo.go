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
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// OutboundItem is one push waiting to go to GitHub.
type OutboundItem struct {
	ID    int64
	Event string
	// WorkItemID is the change request for the CR events and the case itself
	// for the case ones -- both are work items, which is why the column is not
	// named for either.
	WorkItemID   string
	GitReference string
	Payload      map[string]any
	Attempts     int
}

// GithubOutboundRepository is the queue the outbound worker drains.
type GithubOutboundRepository interface {
	// ClaimDue takes up to limit items whose retry time has arrived.
	//
	// FOR UPDATE SKIP LOCKED so more than one replica is safe: two workers
	// racing get disjoint sets rather than blocking or double-pushing.
	ClaimDue(ctx context.Context, limit int) ([]OutboundItem, error)
	// MarkDelivered records success.
	MarkDelivered(ctx context.Context, id int64) error
	// Reschedule records a failure and sets the next attempt, or gives up once
	// maxAttempts is reached.
	Reschedule(ctx context.Context, id int64, attempts, maxAttempts int, backoff time.Duration, reason string) error
}

type githubOutboundRepository struct {
	db *pgxpool.Pool
}

// NewGithubOutboundRepository constructs the queue reader.
func NewGithubOutboundRepository(db *pgxpool.Pool) GithubOutboundRepository {
	return &githubOutboundRepository{db: db}
}

// maxErrorChars bounds what we store from a failure. GitHub's error bodies can
// carry content this service is not allowed to keep, so the column is small
// enough to hold a classification and not a payload.
const maxErrorChars = 300

func (r *githubOutboundRepository) ClaimDue(ctx context.Context, limit int) ([]OutboundItem, error) {
	// Claimed by moving next_attempt_on forward rather than by deleting: if
	// this worker dies mid-push the row becomes due again on its own, instead
	// of being lost the way a claim-and-forget would lose it.
	const query = `
		WITH due AS (
			SELECT id FROM github_outbound_queue
			WHERE status = 'PENDING' AND next_attempt_on <= NOW()
			ORDER BY id
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		UPDATE github_outbound_queue q
		SET next_attempt_on = NOW() + INTERVAL '5 minutes'
		FROM due
		WHERE q.id = due.id
		RETURNING q.id, q.event, q.work_item_id::text, q.git_reference, q.payload, q.attempts`

	rows, err := r.db.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("github outbound: claim due: %w", err)
	}
	defer rows.Close()

	var out []OutboundItem
	for rows.Next() {
		var it OutboundItem
		var raw []byte
		if err := rows.Scan(&it.ID, &it.Event, &it.WorkItemID, &it.GitReference, &raw, &it.Attempts); err != nil {
			return nil, fmt.Errorf("github outbound: scan: %w", err)
		}
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &it.Payload); err != nil {
				return nil, fmt.Errorf("github outbound: decode payload for %d: %w", it.ID, err)
			}
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func (r *githubOutboundRepository) MarkDelivered(ctx context.Context, id int64) error {
	const query = `
		UPDATE github_outbound_queue
		SET status = 'DELIVERED', delivered_on = NOW(), last_error = NULL
		WHERE id = $1`
	if _, err := r.db.Exec(ctx, query, id); err != nil {
		return fmt.Errorf("github outbound: mark delivered %d: %w", id, err)
	}
	return nil
}

func (r *githubOutboundRepository) Reschedule(ctx context.Context, id int64, attempts, maxAttempts int, backoff time.Duration, reason string) error {
	if len(reason) > maxErrorChars {
		reason = reason[:maxErrorChars]
	}
	// Past the limit the row stops being retried and stays visible as FAILED,
	// which is a dead letter someone can look at rather than a silent drop.
	const query = `
		UPDATE github_outbound_queue
		SET attempts        = $2,
		    last_error      = $3,
		    status          = CASE WHEN $2 >= $4 THEN 'FAILED' ELSE 'PENDING' END,
		    next_attempt_on = NOW() + ($5 || ' seconds')::interval
		WHERE id = $1`
	_, err := r.db.Exec(ctx, query, id, attempts, reason, maxAttempts, int(backoff.Seconds()))
	if err != nil {
		return fmt.Errorf("github outbound: reschedule %d: %w", id, err)
	}
	return nil
}
