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

package sweep

import (
	"encoding/json"
	"testing"
)

// TestProject_ParsesNestedAccountFromRealGetProjectResponse is a regression
// test for a real bug found via direct Postman testing against staging:
// GetProject's response (GET /projects/e3e87599-1bc7-6650-182c-0dc5604bcb68)
// nests the account reference as "account": {"id": ..., "name": ...}, not a
// flat top-level "accountId" string. The project struct originally assumed
// the latter, so accountID() silently returned "" — which produced a real
// 404 on SearchAccountContacts, initially misdiagnosed as bad test data.
// This uses the actual response shape returned for that project (trimmed to
// the fields this component reads, plus one sibling field on the nested
// account object to prove extra fields don't break parsing).
func TestProject_ParsesNestedAccountFromRealGetProjectResponse(t *testing.T) {
	const realGetProjectResponse = `{
		"id": "e3e87599-1bc7-6650-182c-0dc5604bcb68",
		"account": {
			"id": "f213fdd1-1b4b-a650-a002-c9d3604bcbac",
			"name": "ACP Test Partner Account"
		},
		"endDate": null
	}`

	var proj project
	if err := json.Unmarshal([]byte(realGetProjectResponse), &proj); err != nil {
		t.Fatalf("unmarshal real GetProject response: %v", err)
	}

	if proj.Account == nil {
		t.Fatal("Account = nil, want populated from the nested \"account\" object")
	}

	const wantAccountID = "f213fdd1-1b4b-a650-a002-c9d3604bcbac"
	if got := proj.accountID(); got != wantAccountID {
		t.Errorf("accountID() = %q, want %q", got, wantAccountID)
	}
}

// TestProject_AccountIDIsEmptyWhenAccountIsAbsent covers the other real
// shape: SearchProjects's response items (entity-service's ProjectView) carry
// no account reference at all, nested or flat — confirmed against
// entity-service's own domain type. accountID() must degrade to "" rather
// than panic on a nil Account.
func TestProject_AccountIDIsEmptyWhenAccountIsAbsent(t *testing.T) {
	const realSearchProjectsItem = `{"id": "p1", "endDate": null}`

	var proj project
	if err := json.Unmarshal([]byte(realSearchProjectsItem), &proj); err != nil {
		t.Fatalf("unmarshal real SearchProjects item: %v", err)
	}

	if got := proj.accountID(); got != "" {
		t.Errorf("accountID() = %q, want \"\"", got)
	}
}
