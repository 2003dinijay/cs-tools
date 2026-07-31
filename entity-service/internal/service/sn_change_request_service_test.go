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

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
)

// TestToDownstreamUTCDateTime covers the create-path datetime conversion: the
// platform's API accepts one datetime format everywhere, and the downstream
// create endpoint requires a different one than its own update endpoint.
func TestToDownstreamUTCDateTime(t *testing.T) {
	t.Parallel()

	t.Run("converts platform format to the downstream format", func(t *testing.T) {
		got, err := toDownstreamUTCDateTime("plannedStartDate", "2026-08-01 10:00:00")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if want := "2026-08-01T10:00:00Z"; got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("rejects bad input with a validation error naming the field", func(t *testing.T) {
		for _, in := range []string{
			"2026-08-01T10:00:00Z", // already UTC form: not the platform's format
			"2026-08-01",
			"01-08-2026 10:00:00",
			"not a date",
			"",
		} {
			_, err := toDownstreamUTCDateTime("plannedEndDate", in)
			if err == nil {
				t.Errorf("input %q: expected an error, got none", in)
				continue
			}
			var ve *apierror.ValidationError
			if !errors.As(err, &ve) {
				t.Errorf("input %q: expected *apierror.ValidationError, got %T", in, err)
				continue
			}
			if want := "plannedEndDate must follow the format: YYYY-MM-DD HH:mm:ss"; ve.Msg != want {
				t.Errorf("input %q: got msg %q, want %q", in, ve.Msg, want)
			}
		}
	})
}

// TestPatchResponseToleratesSlimReceipt pins the behaviour at the boundary where
// a committed write was being reported as a total failure. The downstream layer
// may answer a change-request write with a slim receipt (identifier plus a few
// fields) rather than the full detail payload. Decoding that must not fail, and
// mapping it must not panic on the absent fields.
func TestPatchResponseToleratesSlimReceipt(t *testing.T) {
	t.Parallel()

	const slimReceipt = `{
		"message": "Change request updated successfully.",
		"changeRequest": {
			"id": "0123456789abcdef0123456789abcdef",
			"state": {"label": "Assess"},
			"updatedOn": "2026-07-30 11:22:33",
			"updatedBy": "engineer@example.com"
		}
	}`

	var resp snPatchChangeRequestResponse
	if err := json.Unmarshal([]byte(slimReceipt), &resp); err != nil {
		t.Fatalf("slim receipt failed to decode: %v", err)
	}

	view := mapSNChangeRequestDetailToView(resp.ChangeRequest)

	if want := "01234567-89ab-cdef-0123-456789abcdef"; view.ID != want {
		t.Errorf("ID: got %q, want %q", view.ID, want)
	}
	if view.State == nil {
		t.Error("State: got nil, want a mapped value")
	}
	if want := "2026-07-30 11:22:33"; view.UpdatedOn != want {
		t.Errorf("UpdatedOn: got %q, want %q", view.UpdatedOn, want)
	}
	// Absent optional references must map to nil, not panic and not fabricate.
	if view.Case != nil || view.Deployment != nil || view.AssignedEngineer != nil || view.AssignedTeam != nil {
		t.Error("absent optional references should map to nil")
	}
	// An absent required-in-the-full-payload reference degrades to a zero value.
	if view.Project.ID != "" {
		t.Errorf("Project.ID: got %q, want empty", view.Project.ID)
	}
}

// TestNormalizePaginationCapMatchesDownstream pins the cap at the single choke
// point every search normalizes through. The downstream layer rejects a limit
// above 50 with an opaque error, so exceeding it must be caught here with a
// named validation error instead.
func TestNormalizePaginationCapMatchesDownstream(t *testing.T) {
	t.Parallel()

	if maxLimit != 50 {
		t.Fatalf("maxLimit is %d; the downstream layer rejects anything above 50", maxLimit)
	}
}
