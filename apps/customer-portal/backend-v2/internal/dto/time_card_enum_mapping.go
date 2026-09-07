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

import "strings"

// timeCardStateToEnum maps a time-card state value the frontend can send onto
// entity-service's TimeCardState enum.
//
// The frontend does not send the enum. It resolves a state from the choice list
// this backend serves on GET /projects/{id}/filters as `timeCardStates`
// (findApprovedTimeCardStateId in features/project-details/utils/timeTrackingPage.ts
// picks the entry whose label is "Approved"), then sends that value as
// filters.states. Those choice-list values come from ServiceNow, so they arrive
// title-cased — "Approved", not "approved".
//
// entity-service validates strictly against pending/submitted/approved/rejected/
// processed (validTimeCardStates in sn_time_card_service.go) and returns
// 400 "states contains invalid value: Approved" for anything else, which
// mapUpstreamError passes through and the time-tracking page renders as
// "Error loading time tracking details." Forwarding the value unchanged is
// therefore never correct.
//
// The table is 1:1 with the enum today because ServiceNow's labels happen to be
// the title-cased enum values. It is still an explicit table rather than a
// strings.ToLower call, for the same reason the other six *_enum_mapping.go
// files are: it documents the accepted vocabulary, and it keeps working if a
// choice-list label ever stops matching its enum.
var timeCardStateToEnum = map[string]string{
	"pending":   "pending",
	"submitted": "submitted",
	"approved":  "approved",
	"rejected":  "rejected",
	"processed": "processed",
	"recalled":  "recalled",
}

// timeCardStatesToEnums translates the portal's time-card state filter values
// into entity-service's enum vocabulary.
//
// Unrecognised values are dropped **only when at least one value translated**,
// which is a deliberate departure from the "silently drop unknown filter ids"
// convention this backend applies to case search. That convention is safe there
// because dropping one of several status ids still leaves a filter in place. It
// is not safe here: dropping the *only* value removes the state filter entirely,
// and entity-service then returns time cards in every state — so the page would
// quietly show unapproved cards as though they were approved, which is worse
// than an error.
//
// So when nothing translates, the original values are passed through unchanged
// and entity-service's own validation rejects them with a 400. A wrong filter
// fails loudly instead of showing wrong data. The ids come from ServiceNow via
// GET /projects/{id}/filters and snFlexibleID accepts numbers as well as
// strings, so an environment whose choice list uses numeric ids is a real
// possibility, not a hypothetical.
//
// Returns nil for genuinely empty input so the field is omitted from the
// upstream request rather than sent as an empty array.
func timeCardStatesToEnums(states []string) []string {
	if len(states) == 0 {
		return nil
	}
	// Blank entries carry no intent, so they neither translate nor get forwarded
	// as a bogus filter — a states array of only blanks means "no filter".
	supplied := make([]string, 0, len(states))
	for _, s := range states {
		if t := strings.TrimSpace(s); t != "" {
			supplied = append(supplied, t)
		}
	}
	if len(supplied) == 0 {
		return nil
	}

	out := make([]string, 0, len(supplied))
	seen := make(map[string]struct{}, len(supplied))
	for _, s := range supplied {
		enum, ok := timeCardStateToEnum[strings.ToLower(s)]
		if !ok {
			continue
		}
		if _, dup := seen[enum]; dup {
			continue
		}
		seen[enum] = struct{}{}
		out = append(out, enum)
	}
	if len(out) == 0 {
		// Nothing recognised. Forward as-is so the request fails visibly rather
		// than silently widening to every state.
		return supplied
	}
	return out
}
