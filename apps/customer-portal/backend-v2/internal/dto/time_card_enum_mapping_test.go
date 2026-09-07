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
	"reflect"
	"testing"
)

// TestTimeCardStatesToEnums_TranslatesServiceNowLabels is the regression guard
// for the time-tracking page failing with "Error loading time tracking details".
//
// The frontend sends the ServiceNow choice-list value ("Approved"), and
// entity-service validates strictly against its lowercase enum, returning
// 400 "states contains invalid value: Approved".
func TestTimeCardStatesToEnums_TranslatesServiceNowLabels(t *testing.T) {
	for name, tc := range map[string]struct {
		in   []string
		want []string
	}{
		"the failing case":  {[]string{"Approved"}, []string{"approved"}},
		"all six labels":    {[]string{"Pending", "Submitted", "Approved", "Rejected", "Processed", "Recalled"}, []string{"pending", "submitted", "approved", "rejected", "processed", "recalled"}},
		"already enum":      {[]string{"approved"}, []string{"approved"}},
		"mixed case":        {[]string{"aPpRoVeD"}, []string{"approved"}},
		"surrounding space": {[]string{"  Approved  "}, []string{"approved"}},
		"duplicates":        {[]string{"Approved", "approved"}, []string{"approved"}},
	} {
		got := timeCardStatesToEnums(tc.in)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: timeCardStatesToEnums(%v) = %v, want %v", name, tc.in, got, tc.want)
		}
	}
}

// TestTimeCardStatesToEnums_OmitsWhenNoIntent checks the field is omitted rather
// than sent as an empty array when the caller supplied nothing meaningful.
// entity-service treats an absent filter as "no state restriction"; an empty
// array would be a different, pointless request shape.
func TestTimeCardStatesToEnums_OmitsWhenNoIntent(t *testing.T) {
	for name, in := range map[string][]string{
		"nil":           nil,
		"empty":         {},
		"blank strings": {"", "   "},
		"blanks only":   {"\t"},
	} {
		if got := timeCardStatesToEnums(in); got != nil {
			t.Errorf("%s: got %v, want nil so the field is omitted upstream", name, got)
		}
	}
}

// TestTimeCardStatesToEnums_UnknownOnlyFailsLoudly is the deliberate departure
// from the "silently drop unknown filter ids" convention used for case search.
//
// Dropping one of several case status ids still leaves a filter in place.
// Dropping the *only* time-card state removes the filter entirely, and
// entity-service then returns cards in every state — the page would quietly show
// unapproved cards as approved. Forwarding the unrecognised value instead makes
// entity-service reject it with a 400, so a wrong filter fails visibly rather
// than displaying wrong data. ServiceNow choice ids can be numeric
// (snFlexibleID accepts numbers), so this path is reachable in practice.
func TestTimeCardStatesToEnums_UnknownOnlyFailsLoudly(t *testing.T) {
	for name, in := range map[string][]string{
		"unknown labels": {"Draft", "In Review"},
		"numeric ids":    {"3"},
	} {
		got := timeCardStatesToEnums(in)
		if len(got) == 0 {
			t.Errorf("%s: got %v — dropping every value silently widens the search to all states", name, got)
		}
	}
}

// TestTimeCardStatesToEnums_KeepsKnownDropsUnknown documents the mixed case: a
// recognised value survives alongside an unrecognised one, so one bad label does
// not discard the whole filter.
func TestTimeCardStatesToEnums_KeepsKnownDropsUnknown(t *testing.T) {
	got := timeCardStatesToEnums([]string{"Approved", "Not A State"})
	want := []string{"approved"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TestTimeCardStateTableCoversEntityServiceEnum pins the table against
// entity-service's validTimeCardStates. If that service adds a state, this fails
// and points at the table to update — the alternative is a 400 in production for
// whichever filter value maps to the new state.
func TestTimeCardStateTableCoversEntityServiceEnum(t *testing.T) {
	// Verbatim from validTimeCardStates in
	// entity-service/internal/service/sn_time_card_service.go. "recalled" was
	// missed on the first pass; with unknown values now forwarded rather than
	// dropped, omitting a state entity-service actually accepts would turn a
	// legitimate filter into a 400.
	entityServiceEnum := []string{"pending", "submitted", "approved", "rejected", "processed", "recalled"}

	for _, want := range entityServiceEnum {
		got, ok := timeCardStateToEnum[want]
		if !ok {
			t.Errorf("entity-service accepts %q but the table has no entry for it", want)
			continue
		}
		if got != want {
			t.Errorf("timeCardStateToEnum[%q] = %q, want %q", want, got, want)
		}
	}
	if len(timeCardStateToEnum) != len(entityServiceEnum) {
		t.Errorf("table has %d entries, entity-service accepts %d — a value here would be rejected upstream",
			len(timeCardStateToEnum), len(entityServiceEnum))
	}
}
