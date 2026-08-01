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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package service

import "github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"

// snUserRef mirrors the ServiceNow `createdByUser` object, supplied alongside the
// flat createdBy/createdByFullName fields on comment and attachment rows. Those
// are the only two places ServiceNow hands over the author's sys_user sys_id,
// because they are the only two where it already resolved the sys_user record
// for another reason; see domain.UserReference for why the remaining actor sites
// deliberately stay id-less.
//
// It is null upstream when the author does not resolve to a sys_user record at
// all (a comment written by "system", or by an integration account), and absent
// entirely against a ServiceNow deployment that predates the field — both
// unmarshal to a nil *snUserRef, which snUserReference treats identically.
type snUserRef struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

// snUserReference builds the canonical user reference for an actor. email and
// name come from the flat fields the row always carries; ref is the upstream
// user object where ServiceNow supplies one and nil everywhere else.
//
// The id is taken only from ref, and only when ref carries a non-empty sys_id,
// so an absent or null upstream object can never produce a garbage id. It is
// converted to the same UUID form every other id in this service uses, so
// GET /users/{id} accepts it unchanged.
func snUserReference(ref *snUserRef, email, name string) *domain.UserReference {
	if ref == nil {
		return domain.NewUserReference("", email, name)
	}
	if ref.Email != "" {
		email = ref.Email
	}
	if ref.Name != "" {
		name = ref.Name
	}
	var id string
	if ref.ID != "" {
		id = sysidToUUID(ref.ID)
	}
	return domain.NewUserReference(id, email, name)
}

// snStr dereferences an optional upstream string, yielding "" for nil.
func snStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
