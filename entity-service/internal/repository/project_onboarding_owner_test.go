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

package repository

import (
	"strings"
	"testing"
)

// TestOnboardingOwnerRef covers the NULL-handling of the joined owner columns:
// a project with no owner must map to a nil PersonRef (JSON null), and an owner
// with a blank name or NULL email must still be returned.
func TestOnboardingOwnerRef(t *testing.T) {
	const id = "00000000-0000-0000-0000-000000000001"

	if got := onboardingOwnerRef(nil, nil, nil); got != nil {
		t.Errorf("all-NULL owner = %+v, want nil", got)
	}
	if got := onboardingOwnerRef(strp(""), strp("Jane Doe"), nil); got != nil {
		t.Errorf("empty owner id = %+v, want nil", got)
	}

	full := onboardingOwnerRef(strp(id), strp("Jane Doe"), strp("jane.doe@example.com"))
	if full == nil || full.ID != id || full.Name != "Jane Doe" || full.Email == nil || *full.Email != "jane.doe@example.com" {
		t.Errorf("full owner = %+v", full)
	}

	noEmail := onboardingOwnerRef(strp(id), strp("Jane Doe"), nil)
	if noEmail == nil || noEmail.Email != nil || noEmail.Name != "Jane Doe" {
		t.Errorf("null-email owner = %+v, want owner with nil email", noEmail)
	}

	blankName := onboardingOwnerRef(strp(id), nil, strp("jane.doe@example.com"))
	if blankName == nil || blankName.ID != id || blankName.Name != "" {
		t.Errorf("blank-name owner = %+v, want owner with empty name", blankName)
	}
}

// TestProjectOnboardingOwnerSQL pins that the owner is resolved with a LEFT
// JOIN (so ownerless projects keep their row), on the project's single FK (so
// no row multiplication), and that the name falls back to first + last name.
func TestProjectOnboardingOwnerSQL(t *testing.T) {
	if !strings.HasPrefix(projectOnboardingOwnerJoin, "LEFT JOIN ") {
		t.Errorf("owner join must be a LEFT JOIN: %s", projectOnboardingOwnerJoin)
	}
	if !strings.Contains(projectOnboardingOwnerJoin, "p.onboarding_owner_id") {
		t.Errorf("owner join must key on project.onboarding_owner_id: %s", projectOnboardingOwnerJoin)
	}
	for _, want := range []string{"onb.id", "onb.name", "onb.first_name", "onb.last_name", "onb.email"} {
		if !strings.Contains(projectOnboardingOwnerColumns, want) {
			t.Errorf("owner columns missing %s: %s", want, projectOnboardingOwnerColumns)
		}
	}
	if strings.Contains(projectOnboardingOwnerJoin, "INNER") {
		t.Errorf("owner join must not be an inner join")
	}
}
