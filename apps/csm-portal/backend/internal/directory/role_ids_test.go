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

package directory

import (
	"slices"
	"strings"
	"testing"
)

func TestParseRoleIDs_EmptyIsLegal(t *testing.T) {
	ids, err := ParseRoleIDs("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("got %d entries, want 0", len(ids))
	}
}

func TestParseRoleIDs_ParsesObject(t *testing.T) {
	ids, err := ParseRoleIDs(
		`{"example-timecard-approver-role":"11111111-1111-1111-1111-111111111111","example-worknote-creator-role":"22222222-2222-2222-2222-222222222222"}`,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ids["example-timecard-approver-role"] != "11111111-1111-1111-1111-111111111111" {
		t.Errorf("timecard_approver role = %q", ids["example-timecard-approver-role"])
	}
	if ids["example-worknote-creator-role"] != "22222222-2222-2222-2222-222222222222" {
		t.Errorf("worknote_creator role = %q", ids["example-worknote-creator-role"])
	}
}

func TestParseRoleIDs_RejectsInvalidJSON(t *testing.T) {
	_, err := ParseRoleIDs("not-json")
	if err == nil {
		t.Fatal("expected an error for invalid JSON, got nil")
	}
}

func TestParseRoleIDs_RejectsJSONNull(t *testing.T) {
	_, err := ParseRoleIDs("null")
	if err == nil {
		t.Fatal("expected an error for JSON null, got nil")
	}
}

func TestParseRoleIDs_RejectsEmptyID(t *testing.T) {
	_, err := ParseRoleIDs(`{"some-role": ""}`)
	if err == nil {
		t.Fatal("expected an error for an empty id, got nil")
	}
	if !strings.Contains(err.Error(), "some-role") {
		t.Errorf("error %q does not name the offending role", err)
	}
}

func TestParseRoleIDs_RejectsNonStringValue(t *testing.T) {
	_, err := ParseRoleIDs(`{"some-role": 123}`)
	if err == nil {
		t.Fatal("expected an error for a non-string id, got nil")
	}
}

func TestParseRoleGroupNames_EmptyIsLegal(t *testing.T) {
	names, err := ParseRoleGroupNames("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(names) != 0 {
		t.Fatalf("got %d entries, want 0", len(names))
	}
}

func TestParseRoleGroupNames_ParsesObject(t *testing.T) {
	names, err := ParseRoleGroupNames(
		`{"cs_engineer":["CSM Engineers"],"admin":["CSM Admins","Platform Admins"]}`,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := []string{"CSM Engineers"}; !slices.Equal(names["cs_engineer"], want) {
		t.Errorf("cs_engineer groups = %v, want %v", names["cs_engineer"], want)
	}
	if want := []string{"CSM Admins", "Platform Admins"}; !slices.Equal(names["admin"], want) {
		t.Errorf("admin groups = %v, want %v", names["admin"], want)
	}
}

func TestParseRoleGroupNames_RejectsInvalidJSON(t *testing.T) {
	_, err := ParseRoleGroupNames("not-json")
	if err == nil {
		t.Fatal("expected an error for invalid JSON, got nil")
	}
}

func TestParseRoleGroupNames_RejectsJSONNull(t *testing.T) {
	_, err := ParseRoleGroupNames("null")
	if err == nil {
		t.Fatal("expected an error for JSON null, got nil")
	}
}

func TestParseRoleGroupNames_RejectsEmptyGroupList(t *testing.T) {
	_, err := ParseRoleGroupNames(`{"cs_engineer": []}`)
	if err == nil {
		t.Fatal("expected an error for an empty group name list, got nil")
	}
	if !strings.Contains(err.Error(), "cs_engineer") {
		t.Errorf("error %q does not name the offending role", err)
	}
}

func TestParseRoleGroupNames_RejectsEmptyGroupName(t *testing.T) {
	_, err := ParseRoleGroupNames(`{"cs_engineer": [""]}`)
	if err == nil {
		t.Fatal("expected an error for an empty group name, got nil")
	}
}

func TestParseRoleGroupNames_RejectsNonArrayValue(t *testing.T) {
	_, err := ParseRoleGroupNames(`{"cs_engineer": "CSM Engineers"}`)
	if err == nil {
		t.Fatal("expected an error for a non-array value, got nil")
	}
}
