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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/config"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// errUnknownUser marks "no user row at all for this email" specifically, so
// ResolveScope can tell it apart from a known user who simply isn't INTERNAL
// (inactive, SYSTEM, or otherwise) -- only the former gets the internal-client
// fallback below.
var errUnknownUser = errors.New("no user row for this email")

// AccessScope is the set of projects (and, through them, cases) a caller may
// see. Cases are scoped by their project, so there is no separate case list.
type AccessScope struct {
	// Unrestricted means every project and case.
	Unrestricted bool
	// ProjectIDs is the allowed set when Unrestricted is false. It may be empty,
	// which means no access -- never "no filter".
	ProjectIDs []string
}

// AccessService turns the verified caller identity into an AccessScope.
type AccessService interface {
	// ResolveScope decides what the caller of ctx may see:
	//   - a validated user token: an internal user sees everything; an external
	//     (customer) user sees only projects they are a REGISTERED contact of;
	//     any other KNOWN user type or an inactive user is denied.
	//   - a validated user token whose email has NO row in "user" at all: denied,
	//     UNLESS the request also carries a validated client-credentials token
	//     whose client id has the "internal" role in AUTH_CLIENT_ROLES -- then
	//     it's treated as an internal user (everything). This covers a caller
	//     forwarding a real WSO2 staff member's token for someone not yet
	//     synced into this data source; it does NOT apply to a customer (a
	//     known EXTERNAL row still only sees their own registered projects,
	//     however trusted the client is), nor to a known-but-not-internal row
	//     (inactive/SYSTEM/etc. is a real state, not "unknown").
	//   - no user token, but a validated client-credentials token whose client id
	//     has the "internal" role in AUTH_CLIENT_ROLES: a system caller, sees
	//     everything.
	// Everything else is refused, and so is any request carrying an
	// unvalidated identity: an unverified identity is never used to scope.
	// Token validation is always on, so that should only happen if the auth
	// middleware was somehow left out of the chain -- a bug, not a deployment
	// choice.
	ResolveScope(ctx context.Context) (AccessScope, error)
}

type accessService struct {
	repo        repository.AccessRepository
	clientRoles map[string]string
}

// NewAccessService constructs an AccessService. clientRoles is
// config.Config.AuthClientRoles (client id -> "internal" | "delegate").
func NewAccessService(repo repository.AccessRepository, clientRoles map[string]string) AccessService {
	return &accessService{repo: repo, clientRoles: clientRoles}
}

// ResolveScope implements AccessService.
func (s *accessService) ResolveScope(ctx context.Context) (AccessScope, error) {
	id := auth.IdentityFromContext(ctx)
	if !id.Validated {
		return AccessScope{}, &apierror.ServiceUnavailableError{Msg: "results cannot be scoped to the caller: no verified identity on this request"}
	}

	// A user token always decides, even when a trusted client sent it: the
	// system role only applies when the request carries no user at all --
	// except to rescue an otherwise-unknown user, see errUnknownUser below.
	if id.UserEmail != "" {
		scope, err := s.scopeForUser(ctx, id.UserEmail)
		if errors.Is(err, errUnknownUser) {
			if s.clientRoles[id.ClientID] == config.ClientRoleInternal {
				return AccessScope{Unrestricted: true}, nil
			}
			return AccessScope{}, &apierror.ForbiddenError{Msg: "no access for this user"}
		}
		return scope, err
	}

	if id.ClientID == "" {
		return AccessScope{}, &apierror.UnauthorizedError{Msg: "a user token (x-user-id-token) or an authorized client credential is required"}
	}
	switch s.clientRoles[id.ClientID] {
	case config.ClientRoleInternal:
		return AccessScope{Unrestricted: true}, nil
	case config.ClientRoleDelegate:
		return AccessScope{}, &apierror.UnauthorizedError{Msg: "this client acts on behalf of users and must forward a user token (x-user-id-token)"}
	default:
		return AccessScope{}, &apierror.ForbiddenError{Msg: "client is not authorized"}
	}
}

// scopeForUser maps a user's type to a scope. user.email is not unique, so the
// active rows for the email are combined conservatively: internal access needs
// every active row to be INTERNAL. An email that is also (or only) an EXTERNAL
// customer is scoped like a customer -- less access, never more, when the data
// is ambiguous.
func (s *accessService) scopeForUser(ctx context.Context, email string) (AccessScope, error) {
	users, err := s.repo.UsersByEmail(ctx, email)
	if err != nil {
		return AccessScope{}, err
	}
	if len(users) == 0 {
		return AccessScope{}, errUnknownUser
	}

	var internal, external, other bool
	for _, u := range users {
		if !u.Active {
			continue
		}
		switch u.UserType {
		case "INTERNAL":
			internal = true
		case "EXTERNAL":
			external = true
		default:
			other = true
		}
	}

	switch {
	case external:
		ids, err := s.repo.RegisteredProjectIDs(ctx, email)
		if err != nil {
			return AccessScope{}, err
		}
		return AccessScope{ProjectIDs: ids}, nil
	case internal && !other:
		return AccessScope{Unrestricted: true}, nil
	default:
		return AccessScope{}, &apierror.ForbiddenError{Msg: "no access for this user"}
	}
}
