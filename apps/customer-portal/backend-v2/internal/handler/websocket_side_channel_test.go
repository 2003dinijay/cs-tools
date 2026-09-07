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
	"testing"
)

// TestStampRequestedBy_UsesTheSessionNotTheBrowser is the security assertion for
// the token-increase audit trail: requestedBy becomes the authenticated user's
// email regardless of what the browser claimed.
func TestStampRequestedBy_UsesTheSessionNotTheBrowser(t *testing.T) {
	parsed := map[string]any{
		"type":           msgTypeTokenIncreaseRequest,
		"conversationId": "11111111-1111-1111-1111-111111111111",
		// A caller trying to write someone else into the audit row.
		"requestedBy": "attacker@evil.invalid",
	}

	payload, dropped, err := stampRequestedBy(parsed, "real.user@wso2.com")
	if err != nil {
		t.Fatalf("stampRequestedBy: %v", err)
	}
	if dropped {
		t.Error("dropped = true, want false: there was a session email to stamp")
	}

	var out map[string]any
	if err := json.Unmarshal(payload, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got := out[requestedByField]; got != "real.user@wso2.com" {
		t.Errorf("requestedBy = %v, want the session email; the browser's value must never survive", got)
	}
}

// TestStampRequestedBy_RemovesTheFieldWhenThereIsNoSessionEmail covers the one
// path that would otherwise let a caller name themselves: with no email to
// stamp, a client-supplied requestedBy must be removed, never forwarded. The
// upstream falls back to the account when the field is absent.
func TestStampRequestedBy_RemovesTheFieldWhenThereIsNoSessionEmail(t *testing.T) {
	parsed := map[string]any{
		"type":        msgTypeTokenIncreaseRequest,
		"requestedBy": "attacker@evil.invalid",
	}

	payload, dropped, err := stampRequestedBy(parsed, "")
	if err != nil {
		t.Fatalf("stampRequestedBy: %v", err)
	}
	if !dropped {
		t.Error("dropped = false, want true: a client-supplied value was discarded")
	}

	var out map[string]any
	if err := json.Unmarshal(payload, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if v, present := out[requestedByField]; present {
		t.Errorf("requestedBy is still present (%v); it must be absent rather than client-supplied", v)
	}
}

// TestStampRequestedBy_PreservesEverythingElse checks the rest of the payload is
// forwarded verbatim — only the requester is rewritten.
func TestStampRequestedBy_PreservesEverythingElse(t *testing.T) {
	parsed := map[string]any{
		"type":           msgTypeFeedback,
		"conversationId": "11111111-1111-1111-1111-111111111111",
		"rating":         "up",
		"messageId":      "abc-123",
	}

	payload, _, err := stampRequestedBy(parsed, "real.user@wso2.com")
	if err != nil {
		t.Fatalf("stampRequestedBy: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(payload, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for k, want := range map[string]any{
		"type":           msgTypeFeedback,
		"conversationId": "11111111-1111-1111-1111-111111111111",
		"rating":         "up",
		"messageId":      "abc-123",
	} {
		if out[k] != want {
			t.Errorf("%s = %v, want %v", k, out[k], want)
		}
	}
}

// TestIsConvertedState guards the rule that CONVERTED outranks RESOLVED: the
// agent reporting an issue solved must not silently overwrite the state of a
// conversation a case was created from, or the chat stops being attributable to
// the case it produced.
func TestIsConvertedState(t *testing.T) {
	converted := "CONVERTED"
	lower := "converted"
	padded := "  Converted  "
	resolved := "RESOLVED"
	empty := ""

	for name, tc := range map[string]struct {
		in   *string
		want bool
	}{
		"exact":             {&converted, true},
		"lowercase":         {&lower, true},
		"padded mixed case": {&padded, true},
		"resolved":          {&resolved, false},
		"empty":             {&empty, false},
		"nil":               {nil, false},
	} {
		if got := isConvertedState(tc.in); got != tc.want {
			t.Errorf("%s: isConvertedState = %v, want %v", name, got, tc.want)
		}
	}
}
