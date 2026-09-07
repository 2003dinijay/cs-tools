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

package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestValidateDateParams_RejectsMalformedAndNamesTheField checks the caller gets
// an actionable 400 naming the field and the expected format, instead of the
// upstream's schema language leaking through:
//
//	Validation failed for '$.filters.startDate:pattern' constraint(s).
func TestValidateDateParams_RejectsMalformedAndNamesTheField(t *testing.T) {
	w := httptest.NewRecorder()
	ok := validateDateParams(w, dateParam{"filters.startDate", "2026-6-16"})

	if ok {
		t.Fatal("validateDateParams accepted an unpadded date")
	}
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
	var body struct{ Message string }
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !strings.Contains(body.Message, "filters.startDate") {
		t.Errorf("message %q does not name the offending field", body.Message)
	}
	if !strings.Contains(body.Message, "YYYY-MM-DD") {
		t.Errorf("message %q does not state the expected format", body.Message)
	}
	// The rejected value must not be echoed back — it is caller input.
	if strings.Contains(body.Message, "2026-6-16") {
		t.Errorf("message %q echoes the caller's value back", body.Message)
	}
}

// TestValidateDateParams_SkipsEmptyValues keeps these filters optional: absent
// means "no bound", which every affected upstream already handles.
func TestValidateDateParams_SkipsEmptyValues(t *testing.T) {
	w := httptest.NewRecorder()
	if !validateDateParams(w, dateParam{"startDate", ""}, dateParam{"endDate", ""}) {
		t.Error("empty dates were rejected; they must be treated as absent")
	}
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want no response written", w.Code)
	}
}

// TestValidateDateParams_ReportsTheFirstMalformedInOrder pins deterministic
// messaging when more than one date is bad — the slice is walked in order.
func TestValidateDateParams_ReportsTheFirstMalformedInOrder(t *testing.T) {
	w := httptest.NewRecorder()
	validateDateParams(w, dateParam{"startDate", "bad"}, dateParam{"endDate", "also-bad"})

	var body struct{ Message string }
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !strings.Contains(body.Message, "startDate") || strings.Contains(body.Message, "endDate") {
		t.Errorf("message %q should name startDate only, the first supplied", body.Message)
	}
}

// TestValidateDateParams_AcceptsValidDates is the pass-through case.
func TestValidateDateParams_AcceptsValidDates(t *testing.T) {
	w := httptest.NewRecorder()
	if !validateDateParams(w, dateParam{"startDate", "2026-06-16"}, dateParam{"endDate", "2026-08-24"}) {
		t.Error("valid dates were rejected")
	}
}

// TestDerefString covers the nil-pointer path used for optional filters.
func TestDerefString(t *testing.T) {
	if got := derefString(nil); got != "" {
		t.Errorf("derefString(nil) = %q, want empty", got)
	}
	v := "2026-06-16"
	if got := derefString(&v); got != v {
		t.Errorf("derefString(&v) = %q, want %q", got, v)
	}
}
