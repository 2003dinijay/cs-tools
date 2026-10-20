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

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// RunbookRepository reads and writes the runbook catalog (migrations 0241-0243).
type RunbookRepository interface {
	// List is every runbook with its alerts, ordered by number.
	List(ctx context.Context) ([]domain.Runbook, error)
	// Create adds a runbook with the next number; a ConflictError when one of its alerts is taken.
	Create(ctx context.Context, actor string, req domain.SaveRunbookRequest) (domain.Runbook, error)
	// Update replaces a runbook's link and alerts; nil when there is none with that id, a ConflictError when an alert is taken.
	Update(ctx context.Context, actor, id string, req domain.SaveRunbookRequest) (*domain.Runbook, error)
	// Delete removes a runbook and its alerts; false when there was none.
	Delete(ctx context.Context, id string) (bool, error)
	// Import applies every entry in one transaction: an entry replaces the runbook that holds its first listed alert, or is
	// created; its alerts leave any other runbook, and a runbook left without alerts is removed.
	Import(ctx context.Context, actor string, reqs []domain.SaveRunbookRequest) (domain.ImportRunbooksResponse, error)
	// Merge moves every alert of ids[1:] into ids[0], removes the others and relinks ids[0]; nil when an id is unknown.
	Merge(ctx context.Context, actor string, ids []string, runbookURL *string) (*domain.Runbook, error)
	// Unmerge moves each alert out of the runbook into a new runbook of its own, with the same link, and returns
	// the runbook followed by the new ones; nil when there is none with that id.
	Unmerge(ctx context.Context, actor, id string, alerts []string) ([]domain.Runbook, error)
}

type runbookRepo struct {
	db *pgxpool.Pool
}

// NewRunbookRepository constructs a Postgres-backed RunbookRepository.
func NewRunbookRepository(db *pgxpool.Pool) RunbookRepository {
	return &runbookRepo{db: db}
}

const runbookSelect = `SELECT r.id::text, r.number, r.runbook_url,
	COALESCE((SELECT array_agg(a.alert_name ORDER BY lower(a.alert_name)) FROM runbook_alert a WHERE a.runbook_id = r.id), '{}'),
	r.created_on, r.created_by, r.updated_on, r.updated_by
	FROM runbook r`

func scanRunbook(row pgx.Row) (domain.Runbook, error) {
	var rb domain.Runbook
	err := row.Scan(&rb.ID, &rb.Number, &rb.RunbookURL, &rb.Alerts, &rb.CreatedOn, &rb.CreatedBy, &rb.UpdatedOn, &rb.UpdatedBy)
	return rb, err
}

func (r *runbookRepo) List(ctx context.Context) ([]domain.Runbook, error) {
	rows, err := r.db.Query(ctx, runbookSelect+` ORDER BY r.number`)
	if err != nil {
		return nil, fmt.Errorf("list runbooks: %w", err)
	}
	defer rows.Close()
	out := []domain.Runbook{}
	for rows.Next() {
		rb, err := scanRunbook(rows)
		if err != nil {
			return nil, fmt.Errorf("list runbooks: scan: %w", err)
		}
		out = append(out, rb)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list runbooks: %w", err)
	}
	return out, nil
}

func (r *runbookRepo) Create(ctx context.Context, actor string, req domain.SaveRunbookRequest) (domain.Runbook, error) {
	var out domain.Runbook
	err := r.inTx(ctx, "create runbook", func(tx pgx.Tx) error {
		var id string
		if err := tx.QueryRow(ctx,
			`INSERT INTO runbook (runbook_url, created_by, updated_by) VALUES ($1, $2, $2) RETURNING id::text`,
			req.RunbookURL, actor).Scan(&id); err != nil {
			return err
		}
		if err := setRunbookAlertsTx(ctx, tx, id, req.Alerts); err != nil {
			return err
		}
		rb, err := scanRunbook(tx.QueryRow(ctx, runbookSelect+` WHERE r.id = $1::uuid`, id))
		out = rb
		return err
	})
	return out, err
}

func (r *runbookRepo) Update(ctx context.Context, actor, id string, req domain.SaveRunbookRequest) (*domain.Runbook, error) {
	var out *domain.Runbook
	err := r.inTx(ctx, "update runbook", func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx,
			`UPDATE runbook SET runbook_url = $2, updated_by = $3, updated_on = NOW() WHERE id = $1::uuid`,
			id, req.RunbookURL, actor)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return nil
		}
		if err := setRunbookAlertsTx(ctx, tx, id, req.Alerts); err != nil {
			return err
		}
		rb, err := scanRunbook(tx.QueryRow(ctx, runbookSelect+` WHERE r.id = $1::uuid`, id))
		out = &rb
		return err
	})
	return out, err
}

func (r *runbookRepo) Delete(ctx context.Context, id string) (bool, error) {
	tag, err := r.db.Exec(ctx, `DELETE FROM runbook WHERE id = $1::uuid`, id)
	if err != nil {
		return false, fmt.Errorf("delete runbook: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

func (r *runbookRepo) Import(ctx context.Context, actor string, reqs []domain.SaveRunbookRequest) (domain.ImportRunbooksResponse, error) {
	var out domain.ImportRunbooksResponse
	err := r.inTx(ctx, "import runbooks", func(tx pgx.Tx) error {
		for _, req := range reqs {
			// The runbook holding the entry's first listed alert takes the entry; an entry none of whose alerts is listed is new.
			var id string
			err := tx.QueryRow(ctx,
				`SELECT a.runbook_id::text FROM unnest($1::text[]) WITH ORDINALITY e(alert, pos)
				 JOIN runbook_alert a ON lower(btrim(a.alert_name)) = e.alert
				 ORDER BY e.pos LIMIT 1`,
				lowerAll(req.Alerts)).Scan(&id)
			switch {
			case errors.Is(err, pgx.ErrNoRows):
				if err := tx.QueryRow(ctx,
					`INSERT INTO runbook (runbook_url, created_by, updated_by) VALUES ($1, $2, $2) RETURNING id::text`,
					req.RunbookURL, actor).Scan(&id); err != nil {
					return err
				}
				out.Created++
			case err != nil:
				return err
			default:
				if _, err := tx.Exec(ctx,
					`UPDATE runbook SET runbook_url = $2, updated_by = $3, updated_on = NOW() WHERE id = $1::uuid`,
					id, req.RunbookURL, actor); err != nil {
					return err
				}
				out.Updated++
			}
			// The entry's alerts are the runbook's alerts now, wherever they were listed before.
			if _, err := tx.Exec(ctx,
				`DELETE FROM runbook_alert WHERE runbook_id = $1::uuid OR lower(btrim(alert_name)) = ANY($2::text[])`,
				id, lowerAll(req.Alerts)); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx,
				`INSERT INTO runbook_alert (runbook_id, alert_name) SELECT $1::uuid, unnest($2::text[])`, id, req.Alerts); err != nil {
				return err
			}
		}
		tag, err := tx.Exec(ctx,
			`DELETE FROM runbook r WHERE NOT EXISTS (SELECT 1 FROM runbook_alert a WHERE a.runbook_id = r.id)`)
		if err != nil {
			return err
		}
		out.Removed = int(tag.RowsAffected())
		return nil
	})
	return out, err
}

// setRunbookAlertsTx replaces a runbook's alerts; an alert listed under another runbook is a ConflictError.
func setRunbookAlertsTx(ctx context.Context, tx pgx.Tx, id string, alerts []string) error {
	if _, err := tx.Exec(ctx, `DELETE FROM runbook_alert WHERE runbook_id = $1::uuid`, id); err != nil {
		return err
	}
	var alert, owner string
	err := tx.QueryRow(ctx,
		`SELECT a.alert_name, r.number FROM runbook_alert a JOIN runbook r ON r.id = a.runbook_id
		 WHERE lower(btrim(a.alert_name)) = ANY($1::text[]) ORDER BY lower(a.alert_name) LIMIT 1`,
		lowerAll(alerts)).Scan(&alert, &owner)
	if err == nil {
		return &apierror.ConflictError{Msg: "alert " + alert + " already belongs to runbook " + owner}
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO runbook_alert (runbook_id, alert_name) SELECT $1::uuid, unnest($2::text[])`, id, alerts); err != nil {
		if isUniqueViolation(err) {
			return &apierror.ConflictError{Msg: "one of the alerts already belongs to another runbook"}
		}
		return err
	}
	return nil
}

func (r *runbookRepo) Merge(ctx context.Context, actor string, ids []string, runbookURL *string) (*domain.Runbook, error) {
	var out *domain.Runbook
	err := r.inTx(ctx, "merge runbooks", func(tx pgx.Tx) error {
		var locked int
		if err := tx.QueryRow(ctx,
			`SELECT count(*) FROM (SELECT id FROM runbook WHERE id = ANY($1::uuid[]) FOR UPDATE) l`, ids).Scan(&locked); err != nil {
			return err
		}
		if locked != len(ids) {
			return nil
		}
		target, others := ids[0], ids[1:]
		if _, err := tx.Exec(ctx,
			`UPDATE runbook_alert SET runbook_id = $1::uuid WHERE runbook_id = ANY($2::uuid[])`, target, others); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM runbook WHERE id = ANY($1::uuid[])`, others); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			`UPDATE runbook SET runbook_url = $2, updated_by = $3, updated_on = NOW() WHERE id = $1::uuid`,
			target, runbookURL, actor); err != nil {
			return err
		}
		rb, err := scanRunbook(tx.QueryRow(ctx, runbookSelect+` WHERE r.id = $1::uuid`, target))
		out = &rb
		return err
	})
	return out, err
}

func (r *runbookRepo) Unmerge(ctx context.Context, actor, id string, alerts []string) ([]domain.Runbook, error) {
	var out []domain.Runbook
	err := r.inTx(ctx, "unmerge runbook alerts", func(tx pgx.Tx) error {
		var url *string
		if err := tx.QueryRow(ctx, `SELECT runbook_url FROM runbook WHERE id = $1::uuid FOR UPDATE`, id).Scan(&url); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return err
		}
		var held, moving int
		if err := tx.QueryRow(ctx,
			`SELECT count(*), count(*) FILTER (WHERE lower(btrim(alert_name)) = ANY($2::text[]))
			 FROM runbook_alert WHERE runbook_id = $1::uuid`, id, lowerAll(alerts)).Scan(&held, &moving); err != nil {
			return err
		}
		if moving != len(alerts) {
			return &apierror.ValidationError{Msg: "every alert to unmerge must belong to this runbook"}
		}
		if moving == held {
			return &apierror.ValidationError{Msg: "a runbook must keep at least one alert: leave one alert in it"}
		}
		if _, err := tx.Exec(ctx,
			`UPDATE runbook SET updated_by = $2, updated_on = NOW() WHERE id = $1::uuid`, id, actor); err != nil {
			return err
		}
		created := make([]string, 0, len(alerts))
		for _, requested := range alerts {
			// The alert moves as stored, not as the request spelled it.
			var alert string
			if err := tx.QueryRow(ctx,
				`SELECT alert_name FROM runbook_alert WHERE runbook_id = $1::uuid AND lower(btrim(alert_name)) = lower(btrim($2))`,
				id, requested).Scan(&alert); err != nil {
				return err
			}
			var newID string
			if err := tx.QueryRow(ctx,
				`INSERT INTO runbook (runbook_url, created_by, updated_by) VALUES ($1, $2, $2) RETURNING id::text`,
				url, actor).Scan(&newID); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx,
				`UPDATE runbook_alert SET runbook_id = $2::uuid WHERE runbook_id = $1::uuid AND lower(btrim(alert_name)) = lower(btrim($3))`,
				id, newID, alert); err != nil {
				return err
			}
			created = append(created, newID)
		}
		rows, err := tx.Query(ctx, runbookSelect+` WHERE r.id = ANY($1::uuid[]) ORDER BY r.id = $2::uuid DESC, r.number`,
			append([]string{id}, created...), id)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			rb, err := scanRunbook(rows)
			if err != nil {
				return err
			}
			out = append(out, rb)
		}
		return rows.Err()
	})
	return out, err
}

// inTx runs fn in a transaction; a ConflictError or ValidationError passes through unwrapped so the handler can map it.
func (r *runbookRepo) inTx(ctx context.Context, op string, fn func(tx pgx.Tx) error) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("%s: begin tx: %w", op, err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
		var conflict *apierror.ConflictError
		var invalid *apierror.ValidationError
		if errors.As(err, &conflict) || errors.As(err, &invalid) {
			return err
		}
		return fmt.Errorf("%s: %w", op, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("%s: commit: %w", op, err)
	}
	return nil
}
