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

// Runs against a live database like schedule_repo_integration_test.go, and is skipped unless ENTITY_TEST_DATABASE_URL is set.

package repository

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

const (
	importSunday    = "2026-09-27"
	importNextMon   = "2026-09-28"
	importWednesday = "2026-09-23"
	importThursday  = "2026-09-24"
)

// seedImportRows puts the other engineer on the fixture team inside the week and on the Monday after it, and the lead on the other team inside the week.
func seedImportRows(t *testing.T, repo ScheduleRepository, shift string) {
	t.Helper()
	ctx := context.Background()
	for _, r := range []domain.ApplyScheduleRangeRequest{
		{UserID: schedOtherID, TeamKey: schedTeamKey, ShiftCode: shift, From: schedMonday, To: schedMonday},
		{UserID: schedOtherID, TeamKey: schedTeamKey, ShiftCode: shift, From: importNextMon, To: importNextMon},
		{UserID: schedLeadID, TeamKey: schedOtherTeam, ShiftCode: shift, From: schedMonday, To: schedMonday},
	} {
		if _, err := repo.ApplyRange(ctx, r, schedLeadEmail); err != nil {
			t.Fatalf("seed %+v: %v", r, err)
		}
	}
}

func importWeek(shift string) RotaImportWrite {
	return RotaImportWrite{
		RotaCode:      "SRE_SAAS",
		TeamKeys:      []string{schedTeamKey},
		From:          schedMonday,
		To:            importSunday,
		WritableTeams: []string{schedTeamKey},
		Assignments: []domain.ApplyScheduleRangeRequest{
			{UserID: schedMemberID, TeamKey: schedTeamKey, ShiftCode: shift, From: schedMonday, To: importSunday},
		},
		Absences: []domain.ApplyScheduleAbsenceRequest{
			{UserID: schedLeadID, TeamKey: schedTeamKey, KindCode: "ANNUAL_LEAVE", From: importWednesday, To: importThursday},
		},
	}
}

func assignmentsOn(t *testing.T, pool *pgxpool.Pool, user, team, from, to string) int {
	t.Helper()
	return countRows(t, pool, `
		SELECT count(*) FROM team_schedule_assignment
		 WHERE user_id = $1::uuid AND team_key = $2 AND rota_date BETWEEN $3::date AND $4::date`,
		user, team, from, to)
}

func writtenOverActivity(t *testing.T, pool *pgxpool.Pool, user string) int {
	t.Helper()
	return countRows(t, pool, `
		SELECT count(*) FROM team_schedule_assignment_activity
		 WHERE user_id = $1::uuid AND action = 'DELETED' AND note = $2`, user, rotaImportWriteOverNote)
}

func TestScheduleIntegration_ImportRotaWritesTheFileAndKeepsWhatItDoesNotMention(t *testing.T) {
	repo, pool := newScheduleIntegrationRepo(t)
	shift := shiftWithScope(t, pool, "CRE", "WEEKDAY")
	seedImportRows(t, repo, shift)

	res, err := repo.ImportRota(context.Background(), importWeek(shift), schedLeadEmail)
	if err != nil {
		t.Fatalf("ImportRota: %v", err)
	}
	if res.Cleared != 0 {
		t.Errorf("cleared %d, want nothing: the file does not mention the seeded rows", res.Cleared)
	}
	if len(res.Assignments) != 1 || res.Assignments[0].Applied != 5 || res.Assignments[0].Skipped != 2 {
		t.Errorf("assignment result %+v, want the 5 weekdays applied and the weekend skipped", res.Assignments)
	}
	if len(res.Absences) != 1 || res.Absences[0].Created != 1 {
		t.Errorf("absence result %+v", res.Absences)
	}
	if n := assignmentsOn(t, pool, schedOtherID, schedTeamKey, schedMonday, importSunday); n != 1 {
		t.Errorf("somebody the file does not mention lost their row (%d left)", n)
	}
	if n := assignmentsOn(t, pool, schedLeadID, schedOtherTeam, schedMonday, schedMonday); n != 1 {
		t.Errorf("a team outside the import was touched (%d left)", n)
	}
	if n := assignmentsOn(t, pool, schedMemberID, schedTeamKey, schedMonday, importSunday); n != 5 {
		t.Errorf("%d imported rows, want 5", n)
	}
	if n := countRows(t, pool, `
		SELECT count(*) FROM team_schedule_absence
		 WHERE user_id = $1::uuid AND starts_on = $2::date AND ends_on = $3::date`,
		schedLeadID, importWednesday, importThursday); n != 1 {
		t.Errorf("%d imported absences, want 1", n)
	}
}

func TestScheduleIntegration_ImportRotaWritesOverAClashOnAnotherWritableTeam(t *testing.T) {
	repo, pool := newScheduleIntegrationRepo(t)
	shift := shiftWithScope(t, pool, "CRE", "WEEKDAY")
	seedImportRows(t, repo, shift)

	w := importWeek(shift)
	w.Absences = nil
	w.Assignments = []domain.ApplyScheduleRangeRequest{
		{UserID: schedLeadID, TeamKey: schedTeamKey, ShiftCode: shift, From: schedMonday, To: schedMonday},
	}

	// The lead's same window on the other team blocks the insert, and that team is not the caller's to write.
	_, err := repo.ImportRota(context.Background(), w, schedLeadEmail)
	var conflict *apierror.ConflictError
	if !errors.As(err, &conflict) || !strings.HasPrefix(conflict.Msg, "assignments[0]: ") {
		t.Fatalf("a clash on a team the caller may not write: want a ConflictError naming assignments[0], got %v", err)
	}

	w.TeamKeys = []string{schedTeamKey, schedOtherTeam}
	w.WritableTeams = w.TeamKeys
	res, err := repo.ImportRota(context.Background(), w, schedLeadEmail)
	if err != nil {
		t.Fatalf("ImportRota over a writable clash: %v", err)
	}
	if res.Cleared != 1 {
		t.Errorf("cleared %d, want the lead's row on the other team", res.Cleared)
	}
	if n := assignmentsOn(t, pool, schedLeadID, schedOtherTeam, schedMonday, schedMonday); n != 0 {
		t.Errorf("the clashing row is still there (%d)", n)
	}
	if n := assignmentsOn(t, pool, schedLeadID, schedTeamKey, schedMonday, schedMonday); n != 1 {
		t.Errorf("%d rows where the file put the lead, want 1", n)
	}
	if n := writtenOverActivity(t, pool, schedLeadID); n != 1 {
		t.Errorf("%d DELETED activity rows for the written-over row, want 1", n)
	}
}

func TestScheduleIntegration_ImportRotaWritesOverTheTierHolder(t *testing.T) {
	repo, pool := newScheduleIntegrationRepo(t)
	ctx := context.Background()
	if _, err := repo.ApplyRange(ctx, domain.ApplyScheduleRangeRequest{
		UserID: schedOtherID, TeamKey: schedTeamKey, ShiftCode: "SRE_TZ1_L1", From: schedMonday, To: schedMonday,
	}, schedLeadEmail); err != nil {
		t.Fatalf("seed the L1 holder: %v", err)
	}

	w := importWeek("SRE_TZ1_L1")
	w.Absences = nil
	w.Assignments = []domain.ApplyScheduleRangeRequest{
		{UserID: schedMemberID, TeamKey: schedTeamKey, ShiftCode: "SRE_TZ1_L1", From: schedMonday, To: schedMonday},
	}
	res, err := repo.ImportRota(ctx, w, schedLeadEmail)
	if err != nil {
		t.Fatalf("ImportRota onto a held tier: %v", err)
	}
	if res.Cleared != 1 {
		t.Errorf("cleared %d, want the earlier L1 holder", res.Cleared)
	}
	if n := assignmentsOn(t, pool, schedOtherID, schedTeamKey, schedMonday, schedMonday); n != 0 {
		t.Errorf("the earlier holder still holds L1 (%d rows)", n)
	}
	if n := assignmentsOn(t, pool, schedMemberID, schedTeamKey, schedMonday, schedMonday); n != 1 {
		t.Errorf("%d rows for the file's L1, want 1", n)
	}
}

func TestScheduleIntegration_ImportRotaTwoEntriesForOneTierAreRefused(t *testing.T) {
	repo, pool := newScheduleIntegrationRepo(t)

	w := importWeek("SRE_TZ1_L1")
	w.Absences = nil
	w.Assignments = []domain.ApplyScheduleRangeRequest{
		{UserID: schedMemberID, TeamKey: schedTeamKey, ShiftCode: "SRE_TZ1_L1", From: schedMonday, To: schedMonday},
		{UserID: schedOtherID, TeamKey: schedTeamKey, ShiftCode: "SRE_TZ1_L1", From: schedMonday, To: schedMonday},
	}
	_, err := repo.ImportRota(context.Background(), w, schedLeadEmail)
	var conflict *apierror.ConflictError
	if !errors.As(err, &conflict) || !strings.HasPrefix(conflict.Msg, "assignments[1]: ") {
		t.Fatalf("the file giving one tier to two people: want a ConflictError naming assignments[1], got %v", err)
	}
	if n := assignmentsOn(t, pool, schedMemberID, schedTeamKey, schedMonday, schedMonday); n != 0 {
		t.Errorf("a refused file left %d rows", n)
	}
}

func TestScheduleIntegration_ImportRotaWritesOverTheLeaveUnderAnAssignment(t *testing.T) {
	repo, pool := newScheduleIntegrationRepo(t)
	ctx := context.Background()
	shift := shiftWithScope(t, pool, "CRE", "WEEKDAY")
	// Leave from the Monday to the next Monday: the week's weekdays are written over, the weekend and the next Monday stay.
	if _, err := repo.ApplyAbsence(ctx, domain.ApplyScheduleAbsenceRequest{
		UserID: schedMemberID, TeamKey: schedTeamKey, KindCode: "ANNUAL_LEAVE", From: schedMonday, To: importNextMon,
	}, schedLeadEmail); err != nil {
		t.Fatalf("seed leave: %v", err)
	}
	// Leave filed on a team the caller may not write is left alone.
	if _, err := repo.ApplyAbsence(ctx, domain.ApplyScheduleAbsenceRequest{
		UserID: schedOtherID, TeamKey: schedOtherTeam, KindCode: "ANNUAL_LEAVE", From: schedMonday, To: schedMonday,
	}, schedLeadEmail); err != nil {
		t.Fatalf("seed leave on the other team: %v", err)
	}

	w := importWeek(shift)
	w.Absences = nil
	w.Assignments = append(w.Assignments, domain.ApplyScheduleRangeRequest{
		UserID: schedOtherID, TeamKey: schedTeamKey, ShiftCode: shift, From: schedMonday, To: schedMonday,
	})
	res, err := repo.ImportRota(ctx, w, schedLeadEmail)
	if err != nil {
		t.Fatalf("ImportRota over leave: %v", err)
	}
	trimmed := 0
	for _, a := range res.Absences {
		trimmed += a.Trimmed
	}
	if trimmed != 1 {
		t.Errorf("trimmed %d, want the member's leave cut back to the weekend", trimmed)
	}
	if n := countRows(t, pool, `
		SELECT count(*) FROM team_schedule_absence
		 WHERE user_id = $1::uuid AND starts_on = $2::date AND ends_on = $3::date`,
		schedMemberID, "2026-09-26", importNextMon); n != 1 {
		t.Errorf("the member's leave was not cut to the weekend and the next Monday (%d)", n)
	}
	if n := countRows(t, pool, `SELECT count(*) FROM team_schedule_absence WHERE user_id = $1::uuid`, schedOtherID); n != 1 {
		t.Errorf("leave on a team the caller may not write was touched (%d left)", n)
	}
}

func TestScheduleIntegration_ImportRotaIsAllOrNothing(t *testing.T) {
	repo, pool := newScheduleIntegrationRepo(t)
	shift := shiftWithScope(t, pool, "CRE", "WEEKDAY")
	seedImportRows(t, repo, shift)

	w := importWeek(shift)
	// An escalation window with no tier is refused by the apply rules, after the first entry has been written.
	w.Assignments = append(w.Assignments, domain.ApplyScheduleRangeRequest{
		UserID: schedLeadID, TeamKey: schedTeamKey, ShiftCode: "SRE_TZ1", From: schedMonday, To: schedMonday,
	})
	_, err := repo.ImportRota(context.Background(), w, schedLeadEmail)
	var invalid *apierror.ValidationError
	if !errors.As(err, &invalid) || !strings.HasPrefix(invalid.Msg, "assignments[1]: ") {
		t.Fatalf("want a ValidationError naming assignments[1], got %v", err)
	}
	if n := assignmentsOn(t, pool, schedMemberID, schedTeamKey, schedMonday, importSunday); n != 0 {
		t.Errorf("the first entry was kept although a later one failed (%d rows)", n)
	}
	if n := countRows(t, pool, `SELECT count(*) FROM team_schedule_absence WHERE user_id = $1::uuid`, schedLeadID); n != 0 {
		t.Errorf("%d absences written by a failed import", n)
	}
}

func TestScheduleIntegration_ImportRotaDryRunWritesNothing(t *testing.T) {
	repo, pool := newScheduleIntegrationRepo(t)
	shift := shiftWithScope(t, pool, "CRE", "WEEKDAY")
	seedImportRows(t, repo, shift)

	w := importWeek(shift)
	w.DryRun = true
	res, err := repo.ImportRota(context.Background(), w, schedLeadEmail)
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if res.Assignments[0].Applied != 5 || res.Absences[0].Created != 1 {
		t.Errorf("dry run reported %+v, want what the real import would do", res)
	}
	if n := assignmentsOn(t, pool, schedMemberID, schedTeamKey, schedMonday, importSunday); n != 0 {
		t.Errorf("a dry run wrote %d rows", n)
	}
}

func TestScheduleIntegration_ImportRotaKeepsTheShiftsAMoveDerives(t *testing.T) {
	repo, pool := newScheduleIntegrationRepo(t)
	shift := shiftWithScope(t, pool, "CRE", "WEEKDAY")
	seedImportRows(t, repo, shift)
	// The lead's row on the other team stands in for a shift a move absence derives.
	mustExec(t, pool, `
		UPDATE team_schedule_assignment SET source = 'MOVE'
		 WHERE user_id = $1::uuid AND team_key = $2 AND rota_date = $3::date`,
		schedLeadID, schedOtherTeam, schedMonday)

	w := importWeek(shift)
	w.Absences = nil
	w.TeamKeys = []string{schedTeamKey, schedOtherTeam}
	w.WritableTeams = w.TeamKeys
	w.Assignments = []domain.ApplyScheduleRangeRequest{
		{UserID: schedLeadID, TeamKey: schedTeamKey, ShiftCode: shift, From: schedMonday, To: schedMonday},
	}
	_, err := repo.ImportRota(context.Background(), w, schedLeadEmail)
	var conflict *apierror.ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("a clash with a move's shift: want it refused, not written over, got %v", err)
	}
	if n := countRows(t, pool, `
		SELECT count(*) FROM team_schedule_assignment
		 WHERE user_id = $1::uuid AND team_key = $2 AND rota_date = $3::date AND source = 'MOVE'`,
		schedLeadID, schedOtherTeam, schedMonday); n != 1 {
		t.Errorf("an import removed a shift its move still derives (%d left)", n)
	}
	if n := writtenOverActivity(t, pool, schedLeadID); n != 0 {
		t.Errorf("%d DELETED activity rows for a shift the import kept", n)
	}
}

func TestScheduleIntegration_ImportRotaKeepsATierAMoveHolds(t *testing.T) {
	repo, pool := newScheduleIntegrationRepo(t)
	ctx := context.Background()
	if _, err := repo.ApplyRange(ctx, domain.ApplyScheduleRangeRequest{
		UserID: schedOtherID, TeamKey: schedTeamKey, ShiftCode: "SRE_TZ1_L1", From: schedMonday, To: schedMonday,
	}, schedLeadEmail); err != nil {
		t.Fatalf("seed the L1 holder: %v", err)
	}
	// The holder's row stands in for a tiered shift a move derives.
	mustExec(t, pool, `UPDATE team_schedule_assignment SET source = 'MOVE' WHERE user_id = $1::uuid AND team_key = $2`,
		schedOtherID, schedTeamKey)

	w := importWeek("SRE_TZ1_L1")
	w.Absences = nil
	w.Assignments = []domain.ApplyScheduleRangeRequest{
		{UserID: schedMemberID, TeamKey: schedTeamKey, ShiftCode: "SRE_TZ1_L1", From: schedMonday, To: schedMonday},
	}
	_, err := repo.ImportRota(ctx, w, schedLeadEmail)
	var conflict *apierror.ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("a tier a move holds: want it refused, not written over, got %v", err)
	}
	if n := assignmentsOn(t, pool, schedOtherID, schedTeamKey, schedMonday, schedMonday); n != 1 {
		t.Errorf("an import removed a tiered shift its move derives (%d left)", n)
	}
}
