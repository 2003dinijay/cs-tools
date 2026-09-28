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
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// CloudStatusCandidate is one in-scope outage as the sweep sees it, together
// with the transition it currently implies.
type CloudStatusCandidate struct {
	OutageID string
	Number   string
	// Cloud is empty when the outage's configuration item has no cloud
	// monitor. The sweep counts and skips those rather than guessing a route.
	Cloud string
	Event domain.CloudStatusEvent
	// Timestamp is the begin or the end instant, matching Event.
	Timestamp string
}

// CloudStatusRepository reads which outages owe the status dashboard a
// webhook, and records what was sent.
type CloudStatusRepository interface {
	Candidates(ctx context.Context, parentServiceIDs []string) ([]CloudStatusCandidate, error)
	Record(ctx context.Context, c CloudStatusCandidate) (bool, error)
	Pending(ctx context.Context, limit, maxAttempts int) ([]domain.PendingCloudStatusWebhook, error)
	RecordDelivery(ctx context.Context, id string, delivered bool, errMsg string) error
}

type cloudStatusRepository struct {
	db *pgxpool.Pool
}

// NewCloudStatusRepository constructs the cloud status webhook store.
func NewCloudStatusRepository(db *pgxpool.Pool) CloudStatusRepository {
	return &cloudStatusRepository{db: db}
}

// candidatesSQL finds every outage in the flow's scope and states the single
// transition it currently implies.
//
// THE SCOPE FILTER reproduces the ServiceNow trigger exactly, and the exactness
// matters. The trigger reads
//
//	Configuration Item -> Parent [Service Offering] -> Sys ID  is one of 14
//
// i.e. it dot-walks the outage's CI *as a service offering* and compares that
// offering's PARENT. An outage whose CI is a business service directly can
// never match, because the dot-walk yields nothing. So this joins through
// service_offering and filters on parent_id -- deliberately NOT on
// outage.service_id, which would widen the scope past what ServiceNow fires
// for and start posting webhooks nobody has ever seen.
//
// THE TRANSITION RULE mirrors the flow's conditional check, including its one
// non-obvious consequence:
//
//	begin set, end null  ->  OUTAGE_BEGIN
//	begin set, end set   ->  OUTAGE_END
//	begin null           ->  neither arm; the flow does nothing
//
// An outage that is created already-ended therefore yields ONLY the end event.
// That is not an oversight to tidy up: ServiceNow evaluates the record as it
// stands, so it too would send only the end, and a port that helpfully
// back-filled a begin would flip the public dashboard to degraded for an
// outage that was over before anyone heard of it.
//
// The ordinary case still produces both, in order, because the row is seen
// twice: once while ongoing and again once ended.
const candidatesSQL = `
    SELECT o.id::text,
           COALESCE(o.number, ''),
           COALESCE(cm.cloud_offering::text, ''),
           CASE WHEN o.end_on IS NULL THEN 'OUTAGE_BEGIN' ELSE 'OUTAGE_END' END,
           to_char(
               CASE WHEN o.end_on IS NULL THEN o.start_on ELSE o.end_on END
               AT TIME ZONE 'UTC',
               'YYYY-MM-DD"T"HH24:MI:SS"Z"'
           )
      FROM outage o
      JOIN service_offering so ON so.id = o.service_offering_id
      LEFT JOIN cloud_monitor cm ON cm.service_offering_id = o.service_offering_id
     WHERE so.parent_id = ANY($1::uuid[])
       AND o.start_on IS NOT NULL
     ORDER BY COALESCE(o.end_on, o.start_on)
`

// Candidates returns the transition currently implied by every in-scope
// outage. Callers pass the 14 configured parent service ids.
func (r *cloudStatusRepository) Candidates(ctx context.Context, parentServiceIDs []string) ([]CloudStatusCandidate, error) {
	rows, err := r.db.Query(ctx, candidatesSQL, parentServiceIDs)
	if err != nil {
		return nil, fmt.Errorf("query cloud status candidates: %w", err)
	}
	defer rows.Close()

	var out []CloudStatusCandidate
	for rows.Next() {
		var c CloudStatusCandidate
		if err := rows.Scan(&c.OutageID, &c.Number, &c.Cloud, &c.Event, &c.Timestamp); err != nil {
			return nil, fmt.Errorf("scan cloud status candidate: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate cloud status candidates: %w", err)
	}
	return out, nil
}

// Record claims one transition, returning whether this call is the one that
// claimed it.
//
// DO NOTHING rather than DO UPDATE: the unique key is the whole idempotency
// guarantee. ServiceNow re-ran this flow on every qualifying update to the
// outage and re-posted the same event each time -- an outage edited five times
// while ongoing produced five identical begin webhooks. Doing nothing on
// conflict is a deliberate divergence, and the better behaviour: the dashboard
// is being told a fact, and repeating it carries no information.
const recordSQL = `
    INSERT INTO cloud_status_events (outage_id, event, cloud)
    VALUES ($1::uuid, $2::cloud_status_event_enum, $3)
    ON CONFLICT (outage_id, event) DO NOTHING
    RETURNING id
`

// Record inserts the transition if it is new. The bool reports whether a row
// was created.
func (r *cloudStatusRepository) Record(ctx context.Context, c CloudStatusCandidate) (bool, error) {
	var id string
	err := r.db.QueryRow(ctx, recordSQL, c.OutageID, string(c.Event), c.Cloud).Scan(&id)
	if err == pgx.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("record cloud status event: %w", err)
	}
	return true, nil
}

// pendingSQL reads transitions still owed a successful delivery.
//
// The outage is re-joined for its number and its instants rather than copying
// them onto the event row. The event row records the DECISION; the outage
// remains the source of truth for the facts, and a corrected begin time should
// be reported correctly by a webhook that has not gone out yet.
const pendingSQL = `
    SELECT e.id::text,
           e.outage_id::text,
           COALESCE(o.number, ''),
           e.event::text,
           e.cloud,
           to_char(
               CASE WHEN e.event = 'OUTAGE_BEGIN' THEN o.start_on ELSE o.end_on END
               AT TIME ZONE 'UTC',
               'YYYY-MM-DD"T"HH24:MI:SS"Z"'
           ),
           e.attempt_count,
           COALESCE(e.last_error, '')
      FROM cloud_status_events e
      JOIN outage o ON o.id = e.outage_id
     WHERE e.delivered IS FALSE
       AND e.attempt_count < $2
     ORDER BY e.created_on
     LIMIT $1
`

// Pending returns up to limit undelivered webhooks that have not yet exhausted
// maxAttempts.
func (r *cloudStatusRepository) Pending(ctx context.Context, limit, maxAttempts int) ([]domain.PendingCloudStatusWebhook, error) {
	rows, err := r.db.Query(ctx, pendingSQL, limit, maxAttempts)
	if err != nil {
		return nil, fmt.Errorf("query pending cloud status webhooks: %w", err)
	}
	defer rows.Close()

	out := make([]domain.PendingCloudStatusWebhook, 0)
	for rows.Next() {
		var w domain.PendingCloudStatusWebhook
		if err := rows.Scan(&w.ID, &w.OutageID, &w.Number, &w.Event, &w.Cloud,
			&w.Timestamp, &w.AttemptCount, &w.LastError); err != nil {
			return nil, fmt.Errorf("scan pending cloud status webhook: %w", err)
		}
		out = append(out, w)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pending cloud status webhooks: %w", err)
	}
	return out, nil
}

// recordDeliverySQL stamps the outcome of one attempt.
//
// attempt_count increments on both outcomes, success included. It counts what
// was tried, not what failed; a webhook that succeeded on its third attempt
// should still read as having taken three.
const recordDeliverySQL = `
    UPDATE cloud_status_events
       SET delivered       = $2,
           attempt_count   = attempt_count + 1,
           last_error      = CASE WHEN $2 THEN NULL ELSE NULLIF($3, '') END,
           last_attempt_on = NOW(),
           delivered_on    = CASE WHEN $2 THEN NOW() ELSE delivered_on END,
           updated_on      = NOW()
     WHERE id = $1::uuid
`

// RecordDelivery stamps the outcome of one webhook attempt.
func (r *cloudStatusRepository) RecordDelivery(ctx context.Context, id string, delivered bool, errMsg string) error {
	tag, err := r.db.Exec(ctx, recordDeliverySQL, id, delivered, errMsg)
	if err != nil {
		return fmt.Errorf("record cloud status delivery: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
