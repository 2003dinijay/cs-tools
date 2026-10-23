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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// rotaImportWriteOverNote is the note on the activity row of a row a rota import wrote over.
const rotaImportWriteOverNote = "Written over by a rota import"

// RotaImportWrite is a rota import file, resolved and authorised, ready to write.
type RotaImportWrite struct {
	RotaCode string
	// TeamKeys are the rota's teams, From..To the period the file covers.
	TeamKeys []string
	From     string
	To       string
	// WritableTeams are the rota's teams the caller may write: the file writes over clashing rows on these only, and a clash on any other team is refused.
	WritableTeams []string
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
// The file writes over the roster where it speaks: before an assignment goes in, whatever would refuse it on a writable team is removed, and so is the person's time away on the days it is worked.
// Assignments the file itself wrote (created in this transaction) are never written over, so two entries that collide are refused rather than one silently replacing the other.
// The file's own absences go in after every assignment, so nothing written over is ever the file's.
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

	writable := lowerKeys(w.WritableTeams)
	for i, req := range w.Assignments {
		cleared, removed, trimmed, err := writeOverForAssignment(ctx, tx, req, writable, actorEmail)
		if err != nil {
			return RotaImportResult{}, apierror.Prefixed(fmt.Sprintf("assignments[%d]", i), err)
		}
		res, err := applyRangeTx(ctx, tx, req, actorEmail)
		if err != nil {
			return RotaImportResult{}, apierror.Prefixed(fmt.Sprintf("assignments[%d]", i), err)
		}
		out.Cleared += cleared
		out.Assignments = append(out.Assignments, res)
		if removed+trimmed > 0 {
			out.Absences = append(out.Absences, domain.ApplyScheduleAbsenceResponse{Removed: removed, Trimmed: trimmed})
		}
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

// writeOverForAssignment clears, on each day the entry's window is worked, what the picker would refuse it over: the person's overlapping turn or regular hours on another writable team, and another person's hold on the tier slot it takes.
// It then takes the person's time away off those days, except a move, which is where they work rather than time off.
func writeOverForAssignment(ctx context.Context, tx pgx.Tx, req domain.ApplyScheduleRangeRequest,
	writable []string, actorEmail string) (cleared, removed, trimmed int, err error) {
	from, ferr := time.Parse(time.DateOnly, req.From)
	to, terr := time.Parse(time.DateOnly, req.To)
	if ferr != nil || terr != nil || req.ShiftCode == "" {
		// applyRangeTx refuses it in its own words.
		return 0, 0, 0, nil
	}

	var scope, family string
	var isEscalation, shared bool
	var zoneID, fixedTier *string
	err = tx.QueryRow(ctx, `
		SELECT day_scope::text, family::text, is_escalation, shared_across_teams, zone_id::text, tier::text
		  FROM team_schedule_shift WHERE code = $1`, req.ShiftCode).
		Scan(&scope, &family, &isEscalation, &shared, &zoneID, &fixedTier)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, 0, 0, nil
	}
	if err != nil {
		return 0, 0, 0, fmt.Errorf("read the window to write over: %w", err)
	}
	tier := req.Tier
	if tier == nil {
		tier = fixedTier
	}
	holdsTier := family == "SRE" && isEscalation && zoneID != nil && tier != nil

	var runFrom *time.Time
	cutRun := func(end time.Time) error {
		if runFrom == nil {
			return nil
		}
		rm, tr, err := cutAbsencesTx(ctx, tx, req.UserID, req.TeamKey, *runFrom, end, actorEmail, ptrTo(rotaImportWriteOverNote), writable)
		removed, trimmed, runFrom = removed+rm, trimmed+tr, nil
		return err
	}
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		weekend := d.Weekday() == time.Saturday || d.Weekday() == time.Sunday
		if (scope == "WEEKDAY" && weekend) || (scope == "WEEKEND" && !weekend) {
			if err := cutRun(d.AddDate(0, 0, -1)); err != nil {
				return cleared, removed, trimmed, err
			}
			continue
		}
		if runFrom == nil {
			day := d
			runFrom = &day
		}
		iso := d.Format(time.DateOnly)

		// The person's own clashing rows on another team; the picker clears their rows on this team itself.
		rows, err := tx.Query(ctx, `SELECT `+assignmentColumns+assignmentFrom+`
			  JOIN team_schedule_shift n ON n.code = $3
			 WHERE a.user_id = $1::uuid
			   AND lower(a.team_key) <> lower($4)
			   AND lower(a.team_key) = ANY($5::text[])
			   AND a.source <> 'MOVE'
			   AND a.created_on < now()
			   AND a.is_rotation = n.is_rotation
			   AND tstzrange(a.starts_at, a.ends_at, '[)') && tstzrange(
			         ($2::date::timestamp + make_interval(mins => n.start_minute)) AT TIME ZONE n.authoring_time_zone,
			         ($2::date::timestamp + make_interval(mins => n.end_minute))   AT TIME ZONE n.authoring_time_zone,
			         '[)')`,
			req.UserID, iso, req.ShiftCode, req.TeamKey, writable)
		if err != nil {
			return cleared, removed, trimmed, fmt.Errorf("read clashing assignments: %w", err)
		}
		over, err := scanAssignments(rows)
		rows.Close()
		if err != nil {
			return cleared, removed, trimmed, err
		}

		if holdsTier {
			// The same lock tierHolderConflict takes, so a lead setting this slot meanwhile waits for the import.
			if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text, 0::bigint))`,
				"team-schedule-tier-slot:"+*zoneID+":"+*tier+":"+iso); err != nil {
				return cleared, removed, trimmed, fmt.Errorf("lock the tier slot on %s: %w", iso, err)
			}
			rows, err := tx.Query(ctx, `SELECT `+assignmentColumns+assignmentFrom+`
				 WHERE a.zone_id = $1::uuid
				   AND COALESCE(a.tier, s.tier) = $2::team_schedule_tier_enum
				   AND a.rota_date = $3::date
				   AND a.user_id <> $4::uuid
				   AND ($6 OR lower(a.team_key) = lower($5))
				   AND lower(a.team_key) = ANY($7::text[])
				   AND a.source <> 'MOVE'
				   AND a.created_on < now()`,
				*zoneID, *tier, iso, req.UserID, req.TeamKey, shared, writable)
			if err != nil {
				return cleared, removed, trimmed, fmt.Errorf("read the tier holder: %w", err)
			}
			holders, err := scanAssignments(rows)
			rows.Close()
			if err != nil {
				return cleared, removed, trimmed, err
			}
			over = append(over, holders...)
		}

		if len(over) > 0 {
			ids := make([]string, 0, len(over))
			for _, old := range over {
				if err := recordActivity(ctx, tx, old, "DELETED", actorEmail, nil, nil, nil, ptrTo(rotaImportWriteOverNote)); err != nil {
					return cleared, removed, trimmed, err
				}
				ids = append(ids, old.ID)
			}
			if _, err := tx.Exec(ctx, `DELETE FROM team_schedule_assignment WHERE id = ANY($1::uuid[])`, ids); err != nil {
				return cleared, removed, trimmed, fmt.Errorf("write over assignments: %w", err)
			}
			cleared += len(ids)
		}
	}
	return cleared, removed, trimmed, cutRun(to)
}

// ptrTo is a pointer to v.
func ptrTo(v string) *string { return &v }
