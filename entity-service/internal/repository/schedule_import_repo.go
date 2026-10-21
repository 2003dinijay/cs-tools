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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// RotaImportWrite is a rota import file, resolved and authorised, ready to write.
type RotaImportWrite struct {
	RotaCode string
	// TeamKeys and From..To bound what a replace clears: every assignment of these teams on these days.
	TeamKeys []string
	From     string
	To       string
	Replace  bool
	// DryRun runs every write and rolls the transaction back, so the answer is exactly what a real import would do.
	DryRun      bool
	Assignments []domain.ApplyScheduleRangeRequest
	Absences    []domain.ApplyScheduleAbsenceRequest
}

// RotaImportResult is what each entry did, in the file's order.
type RotaImportResult struct {
	Cleared     int
	Assignments []domain.ApplyScheduleRangeResponse
	Absences    []domain.ApplyScheduleAbsenceResponse
}

// ImportRota implements ScheduleRepository: the whole file in one transaction, all or nothing.
func (r *scheduleRepository) ImportRota(ctx context.Context, w RotaImportWrite, actorEmail string) (RotaImportResult, error) {
	var out RotaImportResult
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return out, fmt.Errorf("begin rota import: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// The generator's own lock, so an import and a "Generate month" of the same rota never interleave.
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, RotaGeneratorOwner(w.RotaCode)); err != nil {
		return out, fmt.Errorf("lock rota import: %w", err)
	}
	if err := nameTheActor(ctx, tx, actorEmail); err != nil {
		return out, err
	}

	if w.Replace {
		tag, err := tx.Exec(ctx, `
			DELETE FROM team_schedule_assignment
			 WHERE team_key = ANY($1::text[])
			   AND rota_date BETWEEN $2::date AND $3::date`,
			w.TeamKeys, w.From, w.To)
		if err != nil {
			return out, fmt.Errorf("clear rota for import: %w", err)
		}
		out.Cleared = int(tag.RowsAffected())
	}

	for i, req := range w.Assignments {
		res, err := applyRangeTx(ctx, tx, req, actorEmail)
		if err != nil {
			return RotaImportResult{}, apierror.Prefixed(fmt.Sprintf("assignments[%d]", i), err)
		}
		out.Assignments = append(out.Assignments, res)
	}
	for i, req := range w.Absences {
		res, err := applyAbsenceTx(ctx, tx, req, actorEmail)
		if err != nil {
			return RotaImportResult{}, apierror.Prefixed(fmt.Sprintf("absences[%d]", i), err)
		}
		out.Absences = append(out.Absences, res)
	}

	if w.DryRun {
		return out, nil
	}
	if err := tx.Commit(ctx); err != nil {
		return RotaImportResult{}, fmt.Errorf("commit rota import: %w", err)
	}
	return out, nil
}
