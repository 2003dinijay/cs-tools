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
	"net/http/httptest"
	"strings"
	"testing"
)

// TestDecodeRequestWithLimit_OversizedTrailingData reproduces a case where the
// first JSON object fits within the size cap, but trailing data after it pushes
// the body over the limit. The trailing-data check must report this as a
// size-limit error (the caller-supplied tooLargeMsg), not the generic "must
// contain a single JSON object" message — the two errors mean different things
// to the caller.
func TestDecodeRequestWithLimit_OversizedTrailingData(t *testing.T) {
	const limit = 1024
	const tooLargeMsg = "attachment exceeds the maximum allowed size of 10 MB"

	body := `{"x":"ok"}` + strings.Repeat(" ", limit*2)
	req := httptest.NewRequest("POST", "/", strings.NewReader(body))
	rec := httptest.NewRecorder()

	var dst struct {
		X string `json:"x"`
	}
	ok := decodeRequestWithLimit(rec, req, &dst, limit, tooLargeMsg)

	if ok {
		t.Fatal("expected decodeRequestWithLimit to return false for oversized trailing data")
	}
	if rec.Code != 400 {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), tooLargeMsg) {
		t.Errorf("response body = %q, want it to contain the size-limit message %q", rec.Body.String(), tooLargeMsg)
	}
	if strings.Contains(rec.Body.String(), "must contain a single JSON object") {
		t.Errorf("response body = %q, should not fall back to the generic trailing-data message for a size-limit case", rec.Body.String())
	}
}
