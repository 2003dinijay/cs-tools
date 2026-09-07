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

package dto

import (
	"regexp"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/registry"
)

// robotNameRE matches this API's own constraint on
// RegistryTokenCreateRequest.RobotName exactly: alphanumeric and dashes only.
var robotNameRE = regexp.MustCompile(`^[a-zA-Z0-9\-]+$`)

// ValidRobotName reports whether name satisfies the registry service's robot
// name constraint.
func ValidRobotName(name string) bool {
	return robotNameRE.MatchString(name)
}

// RegistryTokenCreateRequest is the portal's request body for
// POST /projects/{id}/registry-tokens. The server derives accountName,
// projectKey, snAccountId, snProjectId, and createdBy itself — the client
// only supplies robotName/tokenType/createdFor.
type RegistryTokenCreateRequest struct {
	RobotName  string             `json:"robotName"`
	TokenType  registry.TokenType `json:"tokenType"`
	CreatedFor *string            `json:"createdFor,omitempty"`
}

// RegistryTokenCreateResponse is the portal's response for
// POST /projects/{id}/registry-tokens and POST /registry-tokens/{id}/regenerate.
// Name is read by both GenerateTokenModal.tsx and RegenerateTokenModal.tsx
// to display the server-assigned token name after creation.
type RegistryTokenCreateResponse struct {
	Secret string  `json:"secret"`
	Name   *string `json:"name,omitempty"`
}

// MapRegistryTokenCreateResponse builds the portal response from the
// registry service's TokenCreationResponse.
func MapRegistryTokenCreateResponse(r registry.TokenCreationResponse) RegistryTokenCreateResponse {
	return RegistryTokenCreateResponse{Secret: r.Secret, Name: r.Name}
}

// RegistryPermission is one access grant on a registry token.
type RegistryPermission struct {
	Namespace string `json:"namespace"`
}

// RegistryToken is a registry token as returned to the portal. ID and
// ExpiresAt are numeric (not string) — the real registry service sends
// them as JSON numbers (confirmed against the old Ballerina backend's
// registry:Token{int id?; ...; int expiresAt?; ...}), and the frontend's
// RegistryToken type reads them as numbers directly (registryTokens.ts:
// registryTokenExpiresWithinDays does `token.expiresAt < nowSec`). A
// *string here would fail to unmarshal the real response entirely.
type RegistryToken struct {
	ID           *int64               `json:"id,omitempty"`
	Name         string               `json:"name"`
	DisplayName  *string              `json:"displayName,omitempty"`
	CreationTime *string              `json:"creationTime,omitempty"`
	TokenType    *registry.TokenType  `json:"tokenType,omitempty"`
	CreatedFor   *string              `json:"createdFor,omitempty"`
	CreatedBy    *string              `json:"createdBy,omitempty"`
	ExpiresAt    *int64               `json:"expiresAt,omitempty"`
	Disable      bool                 `json:"disable"`
	Duration     int                  `json:"duration"`
	Permissions  []RegistryPermission `json:"permissions"`
}

// MapRegistryToken builds the portal response from a registry.Token. Note:
// Description is deliberately omitted — it's an internal encoding the
// backend uses to recover project/type/owner for authorization, not
// something the frontend should render.
func MapRegistryToken(t registry.Token) RegistryToken {
	perms := make([]RegistryPermission, 0, len(t.Permissions))
	for _, p := range t.Permissions {
		perms = append(perms, RegistryPermission{Namespace: p.Namespace})
	}
	return RegistryToken{
		ID:           t.ID,
		Name:         t.Name,
		DisplayName:  t.DisplayName,
		CreationTime: t.CreationTime,
		TokenType:    t.TokenType,
		CreatedFor:   t.CreatedFor,
		CreatedBy:    t.CreatedBy,
		ExpiresAt:    t.ExpiresAt,
		Disable:      t.Disable,
		Duration:     t.Duration,
		Permissions:  perms,
	}
}

// MapRegistryTokens builds the portal response list from registry.Tokens.
func MapRegistryTokens(tokens []registry.Token) []RegistryToken {
	out := make([]RegistryToken, 0, len(tokens))
	for _, t := range tokens {
		out = append(out, MapRegistryToken(t))
	}
	return out
}

// IntegrationUser is a service-account user associated with a project.
type IntegrationUser struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

// MapIntegrationUsers builds the portal response from registry.IntegrationUsers.
func MapIntegrationUsers(users []registry.IntegrationUser) []IntegrationUser {
	out := make([]IntegrationUser, 0, len(users))
	for _, u := range users {
		out = append(out, IntegrationUser{ID: u.ID, Email: u.Email})
	}
	return out
}
