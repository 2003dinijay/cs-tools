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

package service

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	importUser1 = "10000000-0000-4000-8000-000000000001"
	importUser2 = "10000000-0000-4000-8000-000000000002"
	importUser3 = "10000000-0000-4000-8000-000000000003"
	importUser4 = "10000000-0000-4000-8000-000000000004"
)

func importStr(s string) *string { return &s }

// importCatalogue is a small SRE catalogue: SaaS with Apollo and Artemis, IaaS beside it, and a CRE team and window.
func importCatalogue() domain.ScheduleCatalogue {
	saas, iaas := importStr("SRE_SAAS"), importStr("SRE_IAAS")
	sre, cre := importStr("SRE"), importStr("CRE")
	member := func(id, email string) domain.ScheduleTeamMember {
		return domain.ScheduleTeamMember{ScheduleEngineer: domain.ScheduleEngineer{UserID: id, Email: email}}
	}
	return domain.ScheduleCatalogue{
		Rotas: []domain.ScheduleRota{
			{Code: "SRE_SAAS", Family: "SRE"},
			{Code: "SRE_IAAS", Family: "SRE"},
			{Code: "SME_MOESIF", Family: "SME"},
		},
		Zones: []domain.ScheduleZone{
			{Code: "TZ1", RotaCode: saas},
			{Code: "TZ3", RotaCode: saas},
			{Code: "IAAS_D", RotaCode: iaas},
		},
		Shifts: []domain.ScheduleShift{
			{Code: "SRE_TZ1_L1", Family: "SRE", ZoneCode: importStr("TZ1"), Tier: importStr("L1")},
			{Code: "SRE_TZ1", Family: "SRE", ZoneCode: importStr("TZ1")},
			{Code: "SRE_TZ3", Family: "SRE", ZoneCode: importStr("TZ3")},
			{Code: "SRE_WE_TZ1", Family: "SRE", ZoneCode: importStr("TZ1")},
			{Code: "SRE_IAAS_DAY", Family: "SRE", ZoneCode: importStr("IAAS_D")},
			{Code: "CRE_REGULAR", Family: "CRE"},
		},
		AbsenceKinds: []domain.ScheduleAbsenceKind{
			{Code: "ANNUAL_LEAVE"},
			{Code: "ALLO_EXT"},
			{Code: "SRE_NIGHT_COVER", Family: sre},
			{Code: "MIGRATION", Family: cre},
			{Code: "CUSTOMER_ON_SITE", Retired: true},
		},
		Teams: []domain.ScheduleTeam{
			{Key: "apollo", Family: "SRE", RotaCode: saas, Members: []domain.ScheduleTeamMember{
				member(importUser1, "user1@example.com"), member(importUser2, "user2@example.com"),
			}},
			{Key: "artemis", Family: "SRE", RotaCode: saas, Members: []domain.ScheduleTeamMember{
				member(importUser3, "user3@example.com"),
			}},
			{Key: "iaas", Family: "SRE", RotaCode: iaas, Members: []domain.ScheduleTeamMember{
				member(importUser4, "user4@example.com"),
			}},
			{Key: "castor", Family: "CRE"},
		},
	}
}

// importFile is the standard structure, as a lead would write it.
func importFile() domain.ImportRotaRequest {
	return domain.ImportRotaRequest{
		SchemaVersion: 1,
		Family:        "SRE",
		RotaCode:      "SRE_SAAS",
		Period:        domain.ImportRotaPeriod{From: "2026-11-01", To: "2026-11-30"},
		Mode:          "replace",
		Assignments: []domain.ImportRotaAssignment{
			{Email: "user1@example.com", TeamKey: "apollo", ShiftCode: "SRE_TZ1_L1", From: "2026-11-02", To: "2026-11-06"},
			{Email: "User2@Example.com", TeamKey: "Apollo", ShiftCode: "sre_tz1", Tier: importStr("l2"), From: "2026-11-02", To: "2026-11-06"},
			{Email: "user3@example.com", TeamKey: "artemis", ShiftCode: "SRE_TZ3", Tier: importStr("L1"), From: "2026-11-02", To: "2026-11-08", Note: importStr("  night  ")},
		},
		Absences: []domain.ImportRotaAbsence{
			{Email: "user2@example.com", TeamKey: "apollo", KindCode: "ANNUAL_LEAVE", From: "2026-11-16", To: "2026-11-20"},
			{Email: "user3@example.com", TeamKey: "artemis", KindCode: "ALLO_EXT", AllocatedTo: importStr("Example Customer"), From: "2026-11-09", To: "2026-11-13"},
		},
	}
}

func newImportService(repo *fakeScheduleRepo) ScheduleService {
	repo.catalogue = importCatalogue()
	repo.ledTeams = []string{"apollo", "artemis"}
	return NewScheduleService(repo, alwaysUnrestrictedAccess{})
}

func TestImportRota_HandsTheResolvedFileToTheRepository(t *testing.T) {
	repo := &fakeScheduleRepo{}
	svc := newImportService(repo)

	res, err := svc.ImportRota(leadCtx("lead@example.com"), "sre_saas", importFile(), false)
	if err != nil {
		t.Fatalf("ImportRota: %v", err)
	}
	w := repo.gotImport
	if w == nil {
		t.Fatal("nothing reached the repository")
	}
	if repo.gotActorEml != "lead@example.com" {
		t.Errorf("actor %q, want the caller", repo.gotActorEml)
	}
	if w.RotaCode != "SRE_SAAS" || w.From != "2026-11-01" || w.To != "2026-11-30" || !w.Replace || w.DryRun {
		t.Errorf("write header %+v", *w)
	}
	if !reflect.DeepEqual(w.TeamKeys, []string{"apollo", "artemis"}) {
		t.Errorf("team keys %v, want the rota's two teams and no other", w.TeamKeys)
	}

	got := w.Assignments
	if len(got) != 3 {
		t.Fatalf("%d assignments, want 3", len(got))
	}
	if got[0].UserID != importUser1 || got[0].TeamKey != "apollo" || got[0].ShiftCode != "SRE_TZ1_L1" || got[0].Tier != nil {
		t.Errorf("assignments[0] %+v", got[0])
	}
	if got[1].UserID != importUser2 || got[1].TeamKey != "apollo" || got[1].ShiftCode != "SRE_TZ1" || got[1].Tier == nil || *got[1].Tier != "L2" {
		t.Errorf("assignments[1] was not normalised: %+v", got[1])
	}
	if got[2].Note == nil || *got[2].Note != "night" {
		t.Errorf("assignments[2] note %v, want it trimmed", got[2].Note)
	}

	abs := w.Absences
	if len(abs) != 2 || abs[0].UserID != importUser2 || abs[0].KindCode != "ANNUAL_LEAVE" || abs[0].HomeTeamKey != "apollo" {
		t.Errorf("absences %+v", abs)
	}
	if abs[1].AllocatedTo == nil || *abs[1].AllocatedTo != "Example Customer" {
		t.Errorf("absences[1] lost who the allocation is for: %+v", abs[1])
	}

	if res.Applied != 3 || res.AbsencesCreated != 2 || res.Mode != "replace" || res.DryRun || res.SkippedDays == nil {
		t.Errorf("response %+v", res)
	}
}

func TestImportRota_DryRunReachesTheRepository(t *testing.T) {
	repo := &fakeScheduleRepo{}
	svc := newImportService(repo)
	res, err := svc.ImportRota(leadCtx("lead@example.com"), "SRE_SAAS", importFile(), true)
	if err != nil {
		t.Fatalf("ImportRota: %v", err)
	}
	if repo.gotImport == nil || !repo.gotImport.DryRun || !res.DryRun {
		t.Fatal("a dry run was not passed to the repository as one")
	}
}

func TestImportRota_ListsEveryProblemInTheFile(t *testing.T) {
	repo := &fakeScheduleRepo{}
	svc := newImportService(repo)
	file := importFile()
	file.Assignments = []domain.ImportRotaAssignment{
		{Email: "nobody@example.com", TeamKey: "apollo", ShiftCode: "SRE_TZ1_L1", From: "2026-11-02", To: "2026-11-02"},
		{Email: "user4@example.com", TeamKey: "iaas", ShiftCode: "SRE_TZ1_L1", From: "2026-11-02", To: "2026-11-02"},
		{Email: "user1@example.com", TeamKey: "apollo", ShiftCode: "SRE_IAAS_DAY", Tier: importStr("L1"), From: "2026-11-02", To: "2026-11-02"},
		{Email: "user1@example.com", TeamKey: "apollo", ShiftCode: "CRE_REGULAR", From: "2026-11-02", To: "2026-11-02"},
		{Email: "user1@example.com", TeamKey: "apollo", ShiftCode: "SRE_TZ1", Tier: importStr("L4"), From: "2026-11-02", To: "2026-11-02"},
		{Email: "user1@example.com", TeamKey: "apollo", ShiftCode: "SRE_TZ1_L1", From: "2026-10-31", To: "2026-11-02"},
		{Email: "user1@example.com", TeamKey: "apollo", ShiftCode: "SRE_TZ1_L1", From: "2026-11-05", To: "2026-11-02"},
	}
	file.Absences = []domain.ImportRotaAbsence{
		{Email: "user1@example.com", TeamKey: "apollo", KindCode: "MIGRATION", From: "2026-11-02", To: "2026-11-02"},
		{Email: "user1@example.com", TeamKey: "apollo", KindCode: "CUSTOMER_ON_SITE", From: "2026-11-02", To: "2026-11-02"},
		{Email: "user1@example.com", TeamKey: "apollo", KindCode: "ANNUAL_LEAVE", From: "2026-12-01", To: "2026-12-02"},
		{Email: "", TeamKey: "apollo", KindCode: "ANNUAL_LEAVE", From: "2026-11-02", To: "2026-11-02"},
	}

	_, err := svc.ImportRota(leadCtx("lead@example.com"), "SRE_SAAS", file, false)
	var invalid *apierror.ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("want ValidationError, got %v", err)
	}
	for _, want := range []string{
		"assignments[0]: nobody on the team schedule has the email nobody@example.com",
		`assignments[1]: "iaas" is not a team on the SRE_SAAS rota`,
		`assignments[2]: "SRE_IAAS_DAY" is not a window of the SRE_SAAS rota`,
		`assignments[3]: "CRE_REGULAR" is not a window of the SRE_SAAS rota`,
		"assignments[4]: tier must be L1, L2 or L3",
		"assignments[5]: 2026-10-31 to 2026-11-02 is outside the period",
		"assignments[6]: to is before from",
		`absences[0]: "MIGRATION" is not a kind of time away offered on the SRE rota`,
		`absences[1]: "CUSTOMER_ON_SITE" is not a kind of time away`,
		"absences[2]: 2026-12-01 to 2026-12-02 does not overlap the period",
		"absences[3]: email is required",
	} {
		if !strings.Contains(invalid.Msg, want) {
			t.Errorf("refusal does not say %q:\n%s", want, invalid.Msg)
		}
	}
	if repo.gotImport != nil {
		t.Fatal("a file with problems reached the repository")
	}
}

func TestImportRota_AnAbsenceMayRunPastThePeriod(t *testing.T) {
	repo := &fakeScheduleRepo{}
	svc := newImportService(repo)
	file := importFile()
	file.Absences = []domain.ImportRotaAbsence{
		{Email: "user1@example.com", TeamKey: "apollo", KindCode: "SRE_NIGHT_COVER", From: "2026-11-01", To: "2027-01-31"},
	}
	if _, err := svc.ImportRota(leadCtx("lead@example.com"), "SRE_SAAS", file, false); err != nil {
		t.Fatalf("a Night cover move running past the period was refused: %v", err)
	}
	if got := repo.gotImport.Absences[0]; got.From != "2026-11-01" || got.To != "2027-01-31" {
		t.Errorf("span %s to %s, want it kept whole", got.From, got.To)
	}
}

func TestImportRota_RefusesTheFileItself(t *testing.T) {
	tooMany := importFile()
	for len(tooMany.Assignments) <= rotaImportMaxEntries {
		tooMany.Assignments = append(tooMany.Assignments, tooMany.Assignments[0])
	}
	cases := []struct {
		name     string
		code     string
		mutate   func(*domain.ImportRotaRequest)
		file     *domain.ImportRotaRequest
		want     string
		notFound bool
	}{
		{name: "unknown schema version", code: "SRE_SAAS", mutate: func(f *domain.ImportRotaRequest) { f.SchemaVersion = 2 }, want: "schemaVersion must be 1"},
		{name: "rota in the body differs from the path", code: "SRE_IAAS", want: "does not match the rota being imported"},
		{name: "unknown rota", code: "SRE_PAAS", mutate: func(f *domain.ImportRotaRequest) { f.RotaCode = "SRE_PAAS" }, notFound: true},
		{name: "not an SRE rota", code: "SME_MOESIF", mutate: func(f *domain.ImportRotaRequest) { f.RotaCode = "SME_MOESIF"; f.Family = "SME" }, want: "only SRE rotas can be imported"},
		{name: "family does not match", code: "SRE_SAAS", mutate: func(f *domain.ImportRotaRequest) { f.Family = "CRE" }, want: "family must be SRE"},
		{name: "unknown mode", code: "SRE_SAAS", mutate: func(f *domain.ImportRotaRequest) { f.Mode = "upsert" }, want: `mode must be "replace" or "merge"`},
		{name: "period is not a date", code: "SRE_SAAS", mutate: func(f *domain.ImportRotaRequest) { f.Period.From = "01/11/2026" }, want: "period: from must be YYYY-MM-DD"},
		{name: "period runs backwards", code: "SRE_SAAS", mutate: func(f *domain.ImportRotaRequest) { f.Period.To = "2026-10-01" }, want: "period: to is before from"},
		{name: "period longer than a year", code: "SRE_SAAS", mutate: func(f *domain.ImportRotaRequest) { f.Period.To = "2027-12-31" }, want: "period: longer than a year"},
		{name: "nothing in the file", code: "SRE_SAAS", mutate: func(f *domain.ImportRotaRequest) { f.Assignments, f.Absences = nil, nil }, want: "no assignments and no absences"},
		{name: "too many entries", code: "SRE_SAAS", file: &tooMany, want: "at most 2000 can be imported"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeScheduleRepo{}
			svc := newImportService(repo)
			file := importFile()
			if tc.file != nil {
				file = *tc.file
			}
			if tc.mutate != nil {
				tc.mutate(&file)
			}
			_, err := svc.ImportRota(leadCtx("lead@example.com"), tc.code, file, false)
			if tc.notFound {
				var nf *apierror.NotFoundError
				if !errors.As(err, &nf) {
					t.Fatalf("want NotFoundError, got %v", err)
				}
			} else {
				var invalid *apierror.ValidationError
				if !errors.As(err, &invalid) || !strings.Contains(invalid.Msg, tc.want) {
					t.Fatalf("want a ValidationError saying %q, got %v", tc.want, err)
				}
			}
			if repo.gotImport != nil {
				t.Fatal("a refused file reached the repository")
			}
		})
	}
}

func TestImportRota_EveryEntryNeedsAWriterOfItsTeam(t *testing.T) {
	file := importFile()
	file.Mode = "merge"

	repo := &fakeScheduleRepo{}
	svc := newImportService(repo)
	repo.ledTeams = []string{"apollo"}
	_, err := svc.ImportRota(leadCtx("apollo.lead@example.com"), "SRE_SAAS", file, false)
	var forbidden *apierror.ForbiddenError
	if !errors.As(err, &forbidden) || !strings.HasPrefix(forbidden.Msg, "assignments[2]: ") {
		t.Fatalf("an Apollo lead importing an Artemis turn: want a ForbiddenError naming assignments[2], got %v", err)
	}
	if repo.gotImport != nil {
		t.Fatal("the import was written although one entry was not the caller's to change")
	}
}

func TestImportRota_AReplaceNeedsAWriterOfEveryTeam(t *testing.T) {
	file := importFile()
	file.Assignments = file.Assignments[:2]
	file.Absences = file.Absences[:1]

	repo := &fakeScheduleRepo{}
	svc := newImportService(repo)
	repo.ledTeams = []string{"apollo"}
	if _, err := svc.ImportRota(leadCtx("apollo.lead@example.com"), "SRE_SAAS", file, false); err == nil {
		t.Fatal("an Apollo lead replaced the whole SaaS rota, Artemis included")
	}
	if repo.gotImport != nil {
		t.Fatal("the replace was written")
	}

	// The same file merged touches only Apollo, which this lead may change.
	file.Mode = "merge"
	if _, err := svc.ImportRota(leadCtx("apollo.lead@example.com"), "SRE_SAAS", file, false); err != nil {
		t.Fatalf("an Apollo-only merge by the Apollo lead: %v", err)
	}
}

func TestImportRota_NeedsAUserToken(t *testing.T) {
	repo := &fakeScheduleRepo{}
	svc := newImportService(repo)
	_, err := svc.ImportRota(context.Background(), "SRE_SAAS", importFile(), false)
	var forbidden *apierror.ForbiddenError
	if !errors.As(err, &forbidden) {
		t.Fatalf("want ForbiddenError without a user, got %v", err)
	}
	if repo.gotImport != nil {
		t.Fatal("an import with no user reached the repository")
	}
}

func TestImportRota_RefusesARestrictedCaller(t *testing.T) {
	repo := &fakeScheduleRepo{catalogue: importCatalogue(), ledTeams: []string{"apollo", "artemis"}}
	svc := NewScheduleService(repo, restrictedAccess{})
	if _, err := svc.ImportRota(leadCtx("user1@example.com"), "SRE_SAAS", importFile(), false); err == nil {
		t.Fatal("a caller who is not internal staff imported a rota")
	}
	if repo.gotImport != nil {
		t.Fatal("the import reached the repository")
	}
}

func TestImportRota_RefusesAnEmailTwoUsersShare(t *testing.T) {
	repo := &fakeScheduleRepo{}
	svc := newImportService(repo)
	repo.catalogue.Teams[1].Members = append(repo.catalogue.Teams[1].Members, domain.ScheduleTeamMember{
		ScheduleEngineer: domain.ScheduleEngineer{UserID: importUser4, Email: "USER1@example.com"},
	})
	_, err := svc.ImportRota(leadCtx("lead@example.com"), "SRE_SAAS", importFile(), false)
	var invalid *apierror.ValidationError
	if !errors.As(err, &invalid) || !strings.Contains(invalid.Msg, "user1@example.com belongs to more than one user") {
		t.Fatalf("want the shared email refused, got %v", err)
	}
}

// failingImportRepo answers every read as the fake does and refuses the import itself.
type failingImportRepo struct {
	*fakeScheduleRepo
	importErr error
}

func (f *failingImportRepo) ImportRota(context.Context, repository.RotaImportWrite, string) (repository.RotaImportResult, error) {
	return repository.RotaImportResult{}, f.importErr
}

func TestImportRota_RepositoryRefusalComesBackAsIs(t *testing.T) {
	repo := &fakeScheduleRepo{catalogue: importCatalogue(), ledTeams: []string{"apollo", "artemis"}}
	refusal := &apierror.ConflictError{Msg: "assignments[2]: TZ3 L1 is already held"}
	svc := NewScheduleService(&failingImportRepo{fakeScheduleRepo: repo, importErr: refusal}, alwaysUnrestrictedAccess{})
	_, err := svc.ImportRota(leadCtx("lead@example.com"), "SRE_SAAS", importFile(), false)
	if !errors.Is(err, refusal) {
		t.Fatalf("want the repository's ConflictError unchanged, got %v", err)
	}
}
