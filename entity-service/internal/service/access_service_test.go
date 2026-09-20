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
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

type fakeAccessRepo struct {
	users    []repository.AccessUser
	projects []string
	// projectCalls counts RegisteredProjectIDs lookups.
	projectCalls int
}

func (f *fakeAccessRepo) UsersByEmail(context.Context, string) ([]repository.AccessUser, error) {
	return f.users, nil
}
func (f *fakeAccessRepo) RegisteredProjectIDs(context.Context, string) ([]string, error) {
	f.projectCalls++
	return f.projects, nil
}

func idCtx(id auth.Identity) context.Context { return auth.WithIdentity(context.Background(), id) }

func userOf(t string, active bool) repository.AccessUser {
	return repository.AccessUser{UserType: t, Active: active}
}

var testClientRoles = map[string]string{"integration": "internal", "portal": "delegate"}

func TestAccessService_ResolveScope(t *testing.T) {
	const email = "jane@example.com"
	tests := []struct {
		name         string
		id           auth.Identity
		users        []repository.AccessUser
		projects     []string
		wantErr      any // nil, or a pointer to the expected apierror type
		wantAll      bool
		wantProjects []string
	}{
		{name: "identity not validated -> refuse (503), never trust the token",
			id: auth.Identity{Validated: false, UserEmail: email}, wantErr: &apierror.ServiceUnavailableError{}},

		{name: "internal user sees everything",
			id: auth.Identity{Validated: true, UserEmail: email}, users: []repository.AccessUser{userOf("INTERNAL", true)}, wantAll: true},
		{name: "customer sees only registered projects",
			id: auth.Identity{Validated: true, UserEmail: email}, users: []repository.AccessUser{userOf("EXTERNAL", true)},
			projects: []string{"p1", "p2"}, wantProjects: []string{"p1", "p2"}},
		{name: "customer with no registered projects gets an EMPTY scope, not everything",
			id: auth.Identity{Validated: true, UserEmail: email}, users: []repository.AccessUser{userOf("EXTERNAL", true)},
			projects: []string{}, wantProjects: []string{}},
		{name: "email shared by an internal and an external row -> customer scope (less access)",
			id: auth.Identity{Validated: true, UserEmail: email}, users: []repository.AccessUser{userOf("INTERNAL", true), userOf("EXTERNAL", true)},
			projects: []string{"p1"}, wantProjects: []string{"p1"}},
		{name: "inactive internal row does not count",
			id: auth.Identity{Validated: true, UserEmail: email}, users: []repository.AccessUser{userOf("INTERNAL", false)}, wantErr: &apierror.ForbiddenError{}},
		{name: "inactive internal + active customer -> customer",
			id: auth.Identity{Validated: true, UserEmail: email}, users: []repository.AccessUser{userOf("INTERNAL", false), userOf("EXTERNAL", true)},
			projects: []string{"p9"}, wantProjects: []string{"p9"}},
		{name: "internal row alongside a NOT_AVAILABLE row -> denied",
			id: auth.Identity{Validated: true, UserEmail: email}, users: []repository.AccessUser{userOf("INTERNAL", true), userOf("NOT_AVAILABLE", true)}, wantErr: &apierror.ForbiddenError{}},
		{name: "system user is not a person -> denied",
			id: auth.Identity{Validated: true, UserEmail: email}, users: []repository.AccessUser{userOf("SYSTEM", true)}, wantErr: &apierror.ForbiddenError{}},
		{name: "NULL user_type -> denied",
			id: auth.Identity{Validated: true, UserEmail: email}, users: []repository.AccessUser{userOf("", true)}, wantErr: &apierror.ForbiddenError{}},
		{name: "unknown email -> denied",
			id: auth.Identity{Validated: true, UserEmail: email}, users: nil, wantErr: &apierror.ForbiddenError{}},
		{name: "unknown email, but forwarded by an internal client -> rescued as internal (everything)",
			id: auth.Identity{Validated: true, ClientID: "integration", UserEmail: email}, users: nil, wantAll: true},
		{name: "unknown email forwarded by a non-internal (delegate) client -> still denied",
			id: auth.Identity{Validated: true, ClientID: "portal", UserEmail: email}, users: nil, wantErr: &apierror.ForbiddenError{}},
		{name: "unknown email forwarded by an unlisted client -> still denied",
			id: auth.Identity{Validated: true, ClientID: "stranger", UserEmail: email}, users: nil, wantErr: &apierror.ForbiddenError{}},
		{name: "the internal-client rescue does NOT apply to a KNOWN customer: still scoped, never widened",
			id: auth.Identity{Validated: true, ClientID: "integration", UserEmail: email}, users: []repository.AccessUser{userOf("EXTERNAL", true)},
			projects: []string{"p1"}, wantProjects: []string{"p1"}},
		{name: "the internal-client rescue does NOT apply to a KNOWN-but-insufficient user (inactive) -- that's a real state, not \"unknown\"",
			id: auth.Identity{Validated: true, ClientID: "integration", UserEmail: email}, users: []repository.AccessUser{userOf("INTERNAL", false)}, wantErr: &apierror.ForbiddenError{}},
		{name: "the internal-client rescue does NOT apply to a KNOWN SYSTEM user",
			id: auth.Identity{Validated: true, ClientID: "integration", UserEmail: email}, users: []repository.AccessUser{userOf("SYSTEM", true)}, wantErr: &apierror.ForbiddenError{}},

		{name: "m2m: internal client with no user -> everything",
			id: auth.Identity{Validated: true, ClientID: "integration"}, wantAll: true},
		{name: "m2m: delegate client with no user must forward one (401)",
			id: auth.Identity{Validated: true, ClientID: "portal"}, wantErr: &apierror.UnauthorizedError{}},
		{name: "m2m: unlisted client -> 403",
			id: auth.Identity{Validated: true, ClientID: "stranger"}, wantErr: &apierror.ForbiddenError{}},
		{name: "no user and no client -> 401",
			id: auth.Identity{Validated: true}, wantErr: &apierror.UnauthorizedError{}},

		{name: "a user token wins over an internal client: the user's scope applies",
			id: auth.Identity{Validated: true, ClientID: "integration", UserEmail: email}, users: []repository.AccessUser{userOf("EXTERNAL", true)},
			projects: []string{"p1"}, wantProjects: []string{"p1"}},
		{name: "portal client + customer user -> customer scope",
			id: auth.Identity{Validated: true, ClientID: "portal", UserEmail: email}, users: []repository.AccessUser{userOf("EXTERNAL", true)},
			projects: []string{"p3"}, wantProjects: []string{"p3"}},
	}
	for _, tt := range tests {
		repo := &fakeAccessRepo{users: tt.users, projects: tt.projects}
		scope, err := NewAccessService(repo, testClientRoles).ResolveScope(idCtx(tt.id))

		if tt.wantErr != nil {
			if err == nil {
				t.Errorf("%s: got scope %+v, want an error", tt.name, scope)
				continue
			}
			var ok bool
			switch tt.wantErr.(type) {
			case *apierror.ServiceUnavailableError:
				var e *apierror.ServiceUnavailableError
				ok = errors.As(err, &e)
			case *apierror.ForbiddenError:
				var e *apierror.ForbiddenError
				ok = errors.As(err, &e)
			case *apierror.UnauthorizedError:
				var e *apierror.UnauthorizedError
				ok = errors.As(err, &e)
			}
			if !ok {
				t.Errorf("%s: got %T (%v), want %T", tt.name, err, err, tt.wantErr)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: unexpected error %v", tt.name, err)
			continue
		}
		if scope.Unrestricted != tt.wantAll {
			t.Errorf("%s: Unrestricted = %v, want %v", tt.name, scope.Unrestricted, tt.wantAll)
		}
		if !tt.wantAll {
			if scope.ProjectIDs == nil || len(scope.ProjectIDs) != len(tt.wantProjects) {
				t.Errorf("%s: ProjectIDs = %#v, want %v (non-nil)", tt.name, scope.ProjectIDs, tt.wantProjects)
			}
		}
	}
}

// Internal users must not trigger a project lookup at all.
func TestAccessService_InternalSkipsProjectLookup(t *testing.T) {
	repo := &fakeAccessRepo{users: []repository.AccessUser{userOf("INTERNAL", true)}}
	_, _ = NewAccessService(repo, nil).ResolveScope(idCtx(auth.Identity{Validated: true, UserEmail: "a@b.c"}))
	if repo.projectCalls != 0 {
		t.Fatalf("project lookups = %d, want 0", repo.projectCalls)
	}
}
