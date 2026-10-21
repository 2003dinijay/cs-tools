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
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// RunbookRepository reads and writes the runbook catalog (migration 0241).
type RunbookRepository interface {
	// List is every entry, ordered by alert name.
	List(ctx context.Context) ([]domain.Runbook, error)
	// Create adds an entry; a ConflictError when its alert name is taken.
	Create(ctx context.Context, actor string, req domain.SaveRunbookRequest) (domain.Runbook, error)
	// Update changes an entry; nil when there is none with that id, a ConflictError when the new alert name is taken.
	Update(ctx context.Context, actor, id string, req domain.SaveRunbookRequest) (*domain.Runbook, error)
	// Delete removes an entry; false when there was none.
	Delete(ctx context.Context, id string) (bool, error)
	// Import creates or updates every entry by alert name in one transaction.
	Import(ctx context.Context, actor string, reqs []domain.SaveRunbookRequest) (domain.ImportRunbooksResponse, error)
}

type runbookRepo struct {
	db *pgxpool.Pool
}

// NewRunbookRepository constructs a Postgres-backed RunbookRepository.
func NewRunbookRepository(db *pgxpool.Pool) RunbookRepository {
	return &runbookRepo{db: db}
}

const runbookColumns = `id::text, alert_name, runbook_url, created_on, created_by, updated_on, updated_by`

func scanRunbook(row pgx.Row) (domain.Runbook, error) {
	var rb domain.Runbook
	err := row.Scan(&rb.ID, &rb.AlertName, &rb.RunbookURL, &rb.CreatedOn, &rb.CreatedBy, &rb.UpdatedOn, &rb.UpdatedBy)
	return rb, err
}

func runbookConflict(err error, alertName string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return &apierror.ConflictError{Msg: "a runbook entry already exists for alert name: " + alertName}
	}
	return nil
}

func (r *runbookRepo) List(ctx context.Context) ([]domain.Runbook, error) {
	rows, err := r.db.Query(ctx, `SELECT `+runbookColumns+` FROM runbook ORDER BY lower(alert_name), id`)
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
	rb, err := scanRunbook(r.db.QueryRow(ctx,
		`INSERT INTO runbook (alert_name, runbook_url, created_by, updated_by)
		 VALUES ($1, $2, $3, $3)
		 RETURNING `+runbookColumns,
		req.AlertName, req.RunbookURL, actor))
	if err != nil {
		if conflict := runbookConflict(err, req.AlertName); conflict != nil {
			return domain.Runbook{}, conflict
		}
		return domain.Runbook{}, fmt.Errorf("create runbook: %w", err)
	}
	return rb, nil
}

func (r *runbookRepo) Update(ctx context.Context, actor, id string, req domain.SaveRunbookRequest) (*domain.Runbook, error) {
	rb, err := scanRunbook(r.db.QueryRow(ctx,
		`UPDATE runbook
		 SET alert_name = $2, runbook_url = $3, updated_by = $4, updated_on = NOW()
		 WHERE id = $1::uuid
		 RETURNING `+runbookColumns,
		id, req.AlertName, req.RunbookURL, actor))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		if conflict := runbookConflict(err, req.AlertName); conflict != nil {
			return nil, conflict
		}
		return nil, fmt.Errorf("update runbook: %w", err)
	}
	return &rb, nil
}

func (r *runbookRepo) Delete(ctx context.Context, id string) (bool, error) {
	tag, err := r.db.Exec(ctx, `DELETE FROM runbook WHERE id = $1::uuid`, id)
	if err != nil {
		return false, fmt.Errorf("delete runbook: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

func (r *runbookRepo) Import(ctx context.Context, actor string, reqs []domain.SaveRunbookRequest) (domain.ImportRunbooksResponse, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return domain.ImportRunbooksResponse{}, fmt.Errorf("import runbooks: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var out domain.ImportRunbooksResponse
	for _, req := range reqs {
		var inserted bool
		err := tx.QueryRow(ctx,
			`INSERT INTO runbook (alert_name, runbook_url, created_by, updated_by)
			 VALUES ($1, $2, $3, $3)
			 ON CONFLICT (lower(btrim(alert_name))) DO UPDATE
			 SET runbook_url = EXCLUDED.runbook_url, updated_by = EXCLUDED.updated_by, updated_on = NOW()
			 RETURNING (xmax = 0)`,
			req.AlertName, req.RunbookURL, actor).Scan(&inserted)
		if err != nil {
			return domain.ImportRunbooksResponse{}, fmt.Errorf("import runbooks: %w", err)
		}
		if inserted {
			out.Created++
		} else {
			out.Updated++
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.ImportRunbooksResponse{}, fmt.Errorf("import runbooks: commit: %w", err)
	}
	return out, nil
}
