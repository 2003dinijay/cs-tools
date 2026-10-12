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
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

const (
	someoneID = "33333333-3333-3333-3333-333333333333"
)

// fakeRotaAccessRepo answers the rota access reads from its fields and
// records the last write.
type fakeRotaAccessRepo struct {
	storedAdmin  bool
	internal     bool
	teamFamily   map[string]string // rostered team -> family
	listedFamily []string
	listed       bool
	wrote        string // "grant-admin:sre_rota_admin", "revoke-marker:lyra", ...
	actor        string
}

func (f *fakeRotaAccessRepo) ListRotaAdmins(context.Context) ([]domain.RotaAdminGrant, error) {
	f.listed = true
	return []domain.RotaAdminGrant{{Family: "SRE", UserID: someoneID}}, nil
}

func (f *fakeRotaAccessRepo) ListRotaMarkers(_ context.Context, families []string) ([]domain.RotaMarkerGrant, error) {
	f.listed = true
	f.listedFamily = families
	return []domain.RotaMarkerGrant{}, nil
}

func (f *fakeRotaAccessRepo) GrantRotaAdmin(_ context.Context, _, role, actor string) (bool, error) {
	f.wrote, f.actor = "grant-admin:"+role, actor
	return true, nil
}

func (f *fakeRotaAccessRepo) RevokeRotaAdmin(_ context.Context, _, role, actor string) (bool, error) {
	f.wrote, f.actor = "revoke-admin:"+role, actor
	return true, nil
}

func (f *fakeRotaAccessRepo) GrantRotaMarker(_ context.Context, _, team, actor string) (bool, error) {
	f.wrote, f.actor = "grant-marker:"+team, actor
	return true, nil
}

func (f *fakeRotaAccessRepo) RevokeRotaMarker(_ context.Context, _, team, actor string) (bool, error) {
	f.wrote, f.actor = "revoke-marker:"+team, actor
	return true, nil
}

func (f *fakeRotaAccessRepo) UserIsInternal(context.Context, string) (bool, error) {
	return f.internal, nil
}

func (f *fakeRotaAccessRepo) HoldsAdminRole(context.Context, string) (bool, error) {
	return f.storedAdmin, nil
}

func (f *fakeRotaAccessRepo) RosteredTeamFamily(_ context.Context, team string) (string, bool, error) {
	fam, ok := f.teamFamily[team]
	return fam, ok, nil
}

// rotaAdminOf is a schedule repository whose caller runs the given families.
type rotaAdminOf struct {
	fakeScheduleRepo
	families []string
	member   bool
}

func (r *rotaAdminOf) RotaAdminFamilies(context.Context, string) ([]string, error) {
	return r.families, nil
}

func (r *rotaAdminOf) UserInTeam(context.Context, string, string) (bool, error) {
	return r.member, nil
}

func callerWithRoles(email string, roles ...string) context.Context {
	return auth.WithIdentity(context.Background(), auth.Identity{Validated: true, UserEmail: email, Roles: roles})
}

func newRotaAccess(repo *fakeRotaAccessRepo, families ...string) (RotaAccessService, *rotaAdminOf) {
	schedule := &rotaAdminOf{families: families, member: true}
	return NewRotaAccessService(repo, schedule, alwaysUnrestrictedAccess{}, []string{"cs_admin"}), schedule
}

func wantForbidden(t *testing.T, err error, what string) {
	t.Helper()
	var forbidden *apierror.ForbiddenError
	if !errors.As(err, &forbidden) {
		t.Fatalf("%s: want ForbiddenError, got %v", what, err)
	}
}

func wantInvalid(t *testing.T, err error, what string) {
	t.Helper()
	var invalid *apierror.ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("%s: want ValidationError, got %v", what, err)
	}
}

// Somebody who may change nothing gets an empty tab, not a refusal, and the
// grants are not even read for them.
func TestRotaAccessIsEmptyForSomebodyWhoMayChangeNothing(t *testing.T) {
	repo := &fakeRotaAccessRepo{}
	svc, _ := newRotaAccess(repo)
	got, err := svc.Get(callerWithRoles("engineer@example.com", "cs_engineer"))
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.CanManage.RotaAdmins || len(got.CanManage.MarkerFamilies) != 0 || repo.listed {
		t.Fatalf("got %+v (listed=%v), want nothing", got, repo.listed)
	}
}

// The CSM admin is the portal's admin role on the token, or the stored admin
// role the Case Paging tab already treats as one. Either may change it all.
func TestCSMAdminMayChangeEveryFamily(t *testing.T) {
	for name, tc := range map[string]struct {
		ctx    context.Context
		stored bool
	}{
		"by the token's role": {callerWithRoles("head@example.com", "CS_ADMIN"), false},
		"by the stored role":  {callerWithRoles("head@example.com"), true},
	} {
		t.Run(name, func(t *testing.T) {
			repo := &fakeRotaAccessRepo{storedAdmin: tc.stored, internal: true}
			svc, _ := newRotaAccess(repo)
			got, err := svc.Get(tc.ctx)
			if err != nil {
				t.Fatalf("Get: %v", err)
			}
			if !got.CanManage.RotaAdmins || !reflect.DeepEqual(got.CanManage.MarkerFamilies, []string{"CRE", "SRE", "SME"}) {
				t.Fatalf("CanManage = %+v, want everything", got.CanManage)
			}
			if err := svc.GrantRotaAdmin(tc.ctx, "sre", someoneID); err != nil {
				t.Fatalf("GrantRotaAdmin: %v", err)
			}
			if repo.wrote != "grant-admin:sre_rota_admin" || repo.actor != "head@example.com" {
				t.Fatalf("wrote %q as %q", repo.wrote, repo.actor)
			}
		})
	}
}

// A rota admin runs one family: they name that family's markers and nothing
// else -- not another family's, and not the rota admins themselves.
func TestRotaAdminNamesMarkersInTheirOwnFamilyOnly(t *testing.T) {
	repo := &fakeRotaAccessRepo{internal: true, teamFamily: map[string]string{"apollo": "SRE", "lyra": "CRE"}}
	svc, _ := newRotaAccess(repo, "SRE")
	ctx := callerWithRoles("sre.rota.admin@example.com", "cs_engineer")

	got, err := svc.Get(ctx)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.CanManage.RotaAdmins || !reflect.DeepEqual(repo.listedFamily, []string{"SRE"}) {
		t.Fatalf("CanManage = %+v, markers listed for %v; want SRE only", got.CanManage, repo.listedFamily)
	}
	if err := svc.GrantRotaMarker(ctx, "apollo", someoneID); err != nil {
		t.Fatalf("own family: %v", err)
	}
	if repo.wrote != "grant-marker:apollo" {
		t.Fatalf("wrote %q", repo.wrote)
	}
	repo.wrote = ""
	wantForbidden(t, svc.GrantRotaMarker(ctx, "lyra", someoneID), "a CRE team")
	wantForbidden(t, svc.RevokeRotaMarker(ctx, "lyra", someoneID), "removing a CRE team's marker")
	wantForbidden(t, svc.GrantRotaAdmin(ctx, "SRE", someoneID), "granting rota admin")
	if repo.wrote != "" {
		t.Fatalf("a refused change was written: %q", repo.wrote)
	}
}

// A marker marks their own team, and a rota role is only for internal staff:
// both are refused up front rather than stored as a grant that does nothing.
func TestRotaMarkerMustBeAnInternalMemberOfTheTeam(t *testing.T) {
	admin := callerWithRoles("head@example.com", "cs_admin")

	repo := &fakeRotaAccessRepo{internal: true, teamFamily: map[string]string{"lyra": "CRE"}}
	svc, schedule := newRotaAccess(repo)
	schedule.member = false
	wantInvalid(t, svc.GrantRotaMarker(admin, "lyra", someoneID), "not on the team")

	repo = &fakeRotaAccessRepo{internal: false, teamFamily: map[string]string{"lyra": "CRE"}}
	svc, _ = newRotaAccess(repo)
	wantInvalid(t, svc.GrantRotaMarker(admin, "lyra", someoneID), "not internal")
	wantInvalid(t, svc.GrantRotaAdmin(admin, "CRE", someoneID), "rota admin, not internal")
	if repo.wrote != "" {
		t.Fatalf("a refused grant was written: %q", repo.wrote)
	}
}

// Removing a marker who has since left the team is exactly what removal is
// for, so it is not held to the membership a grant needs.
func TestRemovingAMarkerWhoLeftTheTeam(t *testing.T) {
	repo := &fakeRotaAccessRepo{internal: true, teamFamily: map[string]string{"lyra": "CRE"}}
	svc, schedule := newRotaAccess(repo)
	schedule.member = false
	if err := svc.RevokeRotaMarker(callerWithRoles("head@example.com", "cs_admin"), "lyra", someoneID); err != nil {
		t.Fatalf("RevokeRotaMarker: %v", err)
	}
	if repo.wrote != "revoke-marker:lyra" {
		t.Fatalf("wrote %q", repo.wrote)
	}
}

func TestRotaAccessRefusesWhatItCannotName(t *testing.T) {
	repo := &fakeRotaAccessRepo{internal: true, teamFamily: map[string]string{"lyra": "CRE"}}
	svc, _ := newRotaAccess(repo)
	admin := callerWithRoles("head@example.com", "cs_admin")

	wantInvalid(t, svc.GrantRotaAdmin(admin, "QA", someoneID), "unknown family")
	wantInvalid(t, svc.GrantRotaAdmin(admin, "CRE", ""), "no user")
	wantInvalid(t, svc.GrantRotaMarker(admin, "lyra", "not-a-uuid"), "bad user id")
	var notFound *apierror.NotFoundError
	if err := svc.GrantRotaMarker(admin, "cab", someoneID); !errors.As(err, &notFound) {
		t.Fatalf("a team not on the schedule: want NotFoundError, got %v", err)
	}
	if repo.wrote != "" {
		t.Fatalf("a refused grant was written: %q", repo.wrote)
	}
}

// A service credential names nobody, so it can grant nothing.
func TestRotaAccessNeedsAUserToken(t *testing.T) {
	svc, _ := newRotaAccess(&fakeRotaAccessRepo{})
	ctx := auth.WithIdentity(context.Background(), auth.Identity{Validated: true, ClientID: "m2m"})
	if _, err := svc.Get(ctx); err == nil {
		t.Fatal("a service credential was answered")
	}
}
