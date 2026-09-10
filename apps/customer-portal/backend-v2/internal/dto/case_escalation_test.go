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
	"encoding/json"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
)

// TestCaseEscalationLevelRef_BuildsTheIDLabelPair covers the translation the
// frontend depends on: it reads escalationLevel.id, while entity-service exposes
// only the bare level id per its no-{id,label} response convention.
func TestCaseEscalationLevelRef_BuildsTheIDLabelPair(t *testing.T) {
	for id, want := range map[string]string{
		"0": "EL0", "1": "EL1", "2": "EL2", "3": "EL3", "4": "EL4", "5": "EL5",
	} {
		v := id
		got := caseEscalationLevelRef(&v)
		if got == nil {
			t.Errorf("caseEscalationLevelRef(%q) = nil", id)
			continue
		}
		if got.ID != id || got.Label != want {
			t.Errorf("caseEscalationLevelRef(%q) = {%s, %s}, want {%s, %s}", id, got.ID, got.Label, id, want)
		}
	}
}

// TestCaseEscalationLevelRef_EdgeCases keeps an absent value absent and lets an
// unrecognised id through as its own label, matching caseStatusRef/caseSeverityRef.
func TestCaseEscalationLevelRef_EdgeCases(t *testing.T) {
	if got := caseEscalationLevelRef(nil); got != nil {
		t.Errorf("nil id: got %+v, want nil", got)
	}
	blank := "   "
	if got := caseEscalationLevelRef(&blank); got != nil {
		t.Errorf("blank id: got %+v, want nil", got)
	}
	future := "9"
	got := caseEscalationLevelRef(&future)
	if got == nil || got.ID != "9" || got.Label != "9" {
		t.Errorf("unknown id: got %+v, want it passed through as its own label", got)
	}
	padded := " 2 "
	if got := caseEscalationLevelRef(&padded); got == nil || got.ID != "2" || got.Label != "EL2" {
		t.Errorf("padded id: got %+v, want {2, EL2}", got)
	}
}

// TestMapCaseDetails_EmitsEscalationAndDuration is the regression guard: all
// three fields were declared on no Go struct anywhere in the chain — not the
// Choreo decode, not the domain view, not the mirror, not the DTO — so
// encoding/json dropped them silently while Ballerina's Case record returns all
// three and the portal reads them.
func TestMapCaseDetails_EmitsEscalationAndDuration(t *testing.T) {
	level := "0"
	dur := "247 Days 4 Hours 31 Minutes"
	esc := false

	b, err := json.Marshal(MapCaseDetails(entity.CaseView{
		ID:              "9fe85754-3ba2-cb10-3e1e-088aa4e45aae",
		Number:          "CS0441080",
		Duration:        &dur,
		EscalationLevel: &level,
		IsEscalated:     &esc,
	}))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out struct {
		Duration        *string `json:"duration"`
		EscalationLevel *struct {
			ID    string `json:"id"`
			Label string `json:"label"`
		} `json:"escalationLevel"`
		IsEscalated *bool `json:"isEscalated"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Duration == nil || *out.Duration != dur {
		t.Errorf("duration = %v, want %q", out.Duration, dur)
	}
	if out.EscalationLevel == nil {
		t.Fatal("escalationLevel absent; CaseDetailsContent reads escalationLevel.id")
	}
	if out.EscalationLevel.ID != "0" || out.EscalationLevel.Label != "EL0" {
		t.Errorf("escalationLevel = %+v, want {0, EL0}", out.EscalationLevel)
	}
	// A false isEscalated must survive rather than being omitted as a zero value:
	// "not escalated" is a real answer, distinct from "unknown".
	if out.IsEscalated == nil {
		t.Error("isEscalated absent for an explicit false; it must be sent")
	} else if *out.IsEscalated {
		t.Error("isEscalated = true, want false")
	}
}

// TestMapCaseDetails_OmitsAbsentEscalationFields keeps an unknown state absent
// rather than fabricating a default.
func TestMapCaseDetails_OmitsAbsentEscalationFields(t *testing.T) {
	b, err := json.Marshal(MapCaseDetails(entity.CaseView{ID: "x", Number: "CS1"}))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var probe map[string]any
	if err := json.Unmarshal(b, &probe); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, key := range []string{"duration", "escalationLevel", "isEscalated"} {
		if _, present := probe[key]; present {
			t.Errorf("%s present when absent upstream; want it omitted", key)
		}
	}
}
