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
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// RotaReminderRequestRepository queues rota reminders asked for by hand
// (team_schedule_reminder_request, migration 0243).
type RotaReminderRequestRepository interface {
	// Create queues a request, unless one of the same family and kind is
	// still waiting (ConflictError) or was asked for within cooldown
	// (TooManyRequestsError). Checked and written under one lock, so two
	// presses at once queue one request.
	Create(ctx context.Context, actor, family, kind string, cooldown time.Duration) (domain.RotaReminderRequest, error)
	// List is the family's latest requests, newest first.
	List(ctx context.Context, family string, limit int) ([]domain.RotaReminderRequest, error)
	// Claim marks the family's oldest waiting request SENDING and returns it:
	// one QUEUED, or one SENDING but not reported within staleAfter. Nil when
	// there is none.
	Claim(ctx context.Context, actor, family string, staleAfter time.Duration) (*domain.RotaReminderRequest, error)
	// Report records how a SENDING request went.
	Report(ctx context.Context, actor, family, id, status, result string) (domain.RotaReminderRequest, error)
}

type rotaReminderRequestRepository struct{ db *pgxpool.Pool }

// NewRotaReminderRequestRepository constructs a RotaReminderRequestRepository.
func NewRotaReminderRequestRepository(db *pgxpool.Pool) RotaReminderRequestRepository {
	return &rotaReminderRequestRepository{db: db}
}

const rotaReminderRequestColumns = `id::text, family, kind, status, requested_by, requested_on, claimed_on, handled_on, COALESCE(result, '')`

func scanRotaReminderRequest(row pgx.Row) (domain.RotaReminderRequest, error) {
	var out domain.RotaReminderRequest
	err := row.Scan(&out.ID, &out.Family, &out.Kind, &out.Status, &out.RequestedBy, &out.RequestedOn, &out.ClaimedOn, &out.HandledOn, &out.Result)
	return out, err
}

func (r *rotaReminderRequestRepository) inTx(ctx context.Context, actor string, fn func(tx pgx.Tx) error) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := nameTheActor(ctx, tx, actor); err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *rotaReminderRequestRepository) Create(ctx context.Context, actor, family, kind string, cooldown time.Duration) (domain.RotaReminderRequest, error) {
	var out domain.RotaReminderRequest
	err := r.inTx(ctx, actor, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('rota-reminder-request:' || $1 || ':' || $2, 0))`, family, kind); err != nil {
			return fmt.Errorf("lock: %w", err)
		}
		last, err := scanRotaReminderRequest(tx.QueryRow(ctx, `SELECT `+rotaReminderRequestColumns+`
			FROM team_schedule_reminder_request WHERE family = $1 AND kind = $2
			ORDER BY requested_on DESC LIMIT 1`, family, kind))
		switch {
		case errors.Is(err, pgx.ErrNoRows):
		case err != nil:
			return fmt.Errorf("read the last request: %w", err)
		case last.Status == "QUEUED" || last.Status == "SENDING":
			return &apierror.ConflictError{Msg: fmt.Sprintf("a %s reminder asked for by %s is already waiting to be sent", kind, last.RequestedBy)}
		case time.Since(last.RequestedOn) < cooldown:
			return &apierror.TooManyRequestsError{Msg: fmt.Sprintf("a %s reminder was asked for less than %d minutes ago; try again later", kind, int(cooldown.Minutes()))}
		}
		out, err = scanRotaReminderRequest(tx.QueryRow(ctx, `
			INSERT INTO team_schedule_reminder_request (family, kind, requested_by, created_by, updated_by)
			VALUES ($1, $2, $3, $3, $3)
			RETURNING `+rotaReminderRequestColumns, family, kind, actor))
		return err
	})
	var conflict *apierror.ConflictError
	var tooMany *apierror.TooManyRequestsError
	if err != nil && !errors.As(err, &conflict) && !errors.As(err, &tooMany) {
		return domain.RotaReminderRequest{}, fmt.Errorf("queue a rota reminder: %w", err)
	}
	return out, err
}

func (r *rotaReminderRequestRepository) List(ctx context.Context, family string, limit int) ([]domain.RotaReminderRequest, error) {
	rows, err := r.db.Query(ctx, `SELECT `+rotaReminderRequestColumns+`
		FROM team_schedule_reminder_request WHERE family = $1 ORDER BY requested_on DESC LIMIT $2`, family, limit)
	if err != nil {
		return nil, fmt.Errorf("list rota reminder requests: %w", err)
	}
	defer rows.Close()
	out := []domain.RotaReminderRequest{}
	for rows.Next() {
		req, err := scanRotaReminderRequest(rows)
		if err != nil {
			return nil, fmt.Errorf("list rota reminder requests: %w", err)
		}
		out = append(out, req)
	}
	return out, rows.Err()
}

func (r *rotaReminderRequestRepository) Claim(ctx context.Context, actor, family string, staleAfter time.Duration) (*domain.RotaReminderRequest, error) {
	var out *domain.RotaReminderRequest
	err := r.inTx(ctx, actor, func(tx pgx.Tx) error {
		req, err := scanRotaReminderRequest(tx.QueryRow(ctx, `
			UPDATE team_schedule_reminder_request
			   SET status = 'SENDING', claimed_on = now(), updated_on = now(), updated_by = $3
			 WHERE id = (SELECT id FROM team_schedule_reminder_request
			              WHERE family = $1
			                AND (status = 'QUEUED' OR (status = 'SENDING' AND claimed_on < now() - make_interval(secs => $2)))
			              ORDER BY requested_on
			              LIMIT 1
			              FOR UPDATE SKIP LOCKED)
			RETURNING `+rotaReminderRequestColumns, family, staleAfter.Seconds(), actor))
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		out = &req
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("claim a rota reminder request: %w", err)
	}
	return out, nil
}

func (r *rotaReminderRequestRepository) Report(ctx context.Context, actor, family, id, status, result string) (domain.RotaReminderRequest, error) {
	var out domain.RotaReminderRequest
	err := r.inTx(ctx, actor, func(tx pgx.Tx) error {
		var err error
		out, err = scanRotaReminderRequest(tx.QueryRow(ctx, `
			UPDATE team_schedule_reminder_request
			   SET status = $3, result = NULLIF($4, ''), handled_on = now(), updated_on = now(), updated_by = $5
			 WHERE id = $1::uuid AND family = $2 AND status = 'SENDING'
			RETURNING `+rotaReminderRequestColumns, id, family, status, result, actor))
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.RotaReminderRequest{}, &apierror.NotFoundError{Msg: "no rota reminder request is being sent with that id"}
	}
	if err != nil {
		return domain.RotaReminderRequest{}, fmt.Errorf("report a rota reminder request: %w", err)
	}
	return out, nil
}
