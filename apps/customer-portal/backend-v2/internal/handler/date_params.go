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
	"net/http"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/dto"
)

// dateParam pairs a caller-facing field name with the value supplied for it, so
// a rejection can name the field the caller actually sent.
type dateParam struct {
	Name  string
	Value string
}

// validateDateParams writes a 400 and returns false when any supplied date is
// not a plain YYYY-MM-DD value.
//
// An empty value is skipped rather than rejected: these are optional filters,
// and absent means "no bound", which every affected upstream already handles.
// Order is significant — the slice is walked in order so the message is
// deterministic when more than one date is malformed.
func validateDateParams(w http.ResponseWriter, params ...dateParam) bool {
	for _, p := range params {
		if p.Value == "" {
			continue
		}
		if !dto.IsValidDateOnly(p.Value) {
			writeError(w, http.StatusBadRequest, "Invalid "+p.Name+": expected a date in YYYY-MM-DD format.")
			return false
		}
	}
	return true
}

// derefString returns the pointed-to value, or "" when the pointer is nil, so an
// optional filter can be fed to validateDateParams without a nil check at each
// call site.
func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
