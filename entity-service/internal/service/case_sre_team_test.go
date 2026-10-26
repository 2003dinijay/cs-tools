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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// The case view's own account SRE team wins; Postgres is asked only when the case response leaves it out.
func TestCaseSRETeam_FallsBackToTheAccountInPostgres(t *testing.T) {
	project := &domain.EntityRef{ID: "p1"}
	asked := ""
	resolve := func(_ context.Context, projectID string) (string, string, error) {
		asked = projectID
		return "g2", "MS/PC SRE Group", nil
	}

	withTeam := domain.CaseView{ProjectDetails: project, AccountDetails: &domain.AccountRef{SreTeam: &domain.EntityRef{ID: "g1", Name: "Apollo SRE Group"}}}
	if id, name := caseSRETeam(context.Background(), resolve, withTeam, "test"); id != "g1" || name != "Apollo SRE Group" || asked != "" {
		t.Errorf("case view with a team = %q/%q (asked %q); want g1/Apollo SRE Group without asking Postgres", id, name, asked)
	}

	withoutTeam := domain.CaseView{ProjectDetails: project, AccountDetails: &domain.AccountRef{}}
	if id, name := caseSRETeam(context.Background(), resolve, withoutTeam, "test"); id != "g2" || name != "MS/PC SRE Group" || asked != "p1" {
		t.Errorf("case view without a team = %q/%q (asked %q); want g2/MS/PC SRE Group from project p1", id, name, asked)
	}

	failing := func(context.Context, string) (string, string, error) { return "x", "y", errors.New("down") }
	if id, name := caseSRETeam(context.Background(), failing, withoutTeam, "test"); id != "" || name != "" {
		t.Errorf("failed lookup = %q/%q; want none", id, name)
	}
	if id, name := caseSRETeam(context.Background(), nil, withoutTeam, "test"); id != "" || name != "" {
		t.Errorf("no resolver = %q/%q; want none", id, name)
	}
}
