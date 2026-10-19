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

package domain

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// product_version.current_support_status and release_date are nullable columns,
// so a version without them must serialize them as null, not an empty string or
// the zero time.
func TestProductVersion_NullableFieldsSerializeAsNull(t *testing.T) {
	out, err := json.Marshal(ProductVersion{ID: "v1", Version: "4.0.0"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, want := range []string{`"currentSupportStatus":null`, `"releaseDate":null`} {
		if !strings.Contains(string(out), want) {
			t.Errorf("output missing %s: %s", want, out)
		}
	}

	status, released := SupportStatusAvailable, time.Date(2025, 3, 11, 0, 0, 0, 0, time.UTC)
	out, _ = json.Marshal(ProductVersion{ID: "v1", Version: "4.0.0", CurrentSupportStatus: &status, ReleaseDate: &released})
	for _, want := range []string{`"currentSupportStatus":"available"`, `"releaseDate":"2025-03-11T00:00:00Z"`} {
		if !strings.Contains(string(out), want) {
			t.Errorf("output missing %s: %s", want, out)
		}
	}
}
