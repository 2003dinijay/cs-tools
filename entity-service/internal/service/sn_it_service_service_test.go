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

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// fakeServiceNowServices stands in for ServiceNow's POST /services/search the
// way the real one behaves: `name CONTAINS searchQuery`, matches in the order
// the records are listed (creation order, newest first), at most maxLimit rows
// per page, and a totalRecords that counts every match. It counts the calls it
// serves and fails any page whose offset is in failOffsets.
type fakeServiceNowServices struct {
	records     []snITService
	calls       atomic.Int32
	failOffsets map[int]bool
}

func (f *fakeServiceNowServices) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.calls.Add(1)
	var body snITServiceSearchPayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if body.Pagination.Limit > maxLimit {
		http.Error(w, "limit above the upstream's ceiling", http.StatusBadRequest)
		return
	}
	if f.failOffsets[body.Pagination.Offset] {
		http.Error(w, "upstream failure", http.StatusInternalServerError)
		return
	}

	q := strings.ToLower(body.Filters.SearchQuery)
	var matches []snITService
	for _, rec := range f.records {
		if q == "" || strings.Contains(strings.ToLower(derefString(rec.Name)), q) {
			matches = append(matches, rec)
		}
	}
	start := min(body.Pagination.Offset, len(matches))
	end := min(start+body.Pagination.Limit, len(matches))

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(snITServicesResponse{
		Services:     matches[start:end],
		TotalRecords: len(matches),
		Offset:       body.Pagination.Offset,
		Limit:        body.Pagination.Limit,
	})
}

func fakeService(n int, name, class string) snITService {
	return snITService{ID: fmt.Sprintf("%032x", n), Name: &name, Class: &class}
}

// choreoRecords reproduces what production returned for "Choreo": 55 matches,
// the real "Choreo" service last (it is the oldest), 45 service offerings, four
// "client-...choreo-alert-integration" services first (the newest), and a few
// services whose names do not contain the query at all.
func choreoRecords() []snITService {
	var recs []snITService
	n := 1
	add := func(name, class string) {
		recs = append(recs, fakeService(n, name, class))
		n++
	}
	add("Unrelated Billing Service", "cmdb_ci_service")
	add("client-uoechoreosub-alert-integration", "cmdb_ci_service")
	add("client-btchoreo-alert-integration", "cmdb_ci_service")
	add("client-uoexeterchoreo-alert-integration", "cmdb_ci_service")
	add("client-devrychoreo-alert-integration", "cmdb_ci_service")
	for i := 0; i < 20; i++ {
		add(fmt.Sprintf("Choreo EU - DP Thing %02d Service Offering", i), "service_offering")
	}
	add("Choreo EU - Data Plane Service", "cmdb_ci_service")
	add("Choreo EU - Control Plane Service", "cmdb_ci_service")
	add("Another Unrelated Service", "cmdb_ci_service")
	for i := 0; i < 25; i++ {
		add(fmt.Sprintf("Choreo EU - CP Thing %02d Service Offering", i), "service_offering")
	}
	add("Choreo Data Plane Service - EU", "cmdb_ci_service")
	add("Choreo Data Plane Service - US", "cmdb_ci_service")
	add("Choreo Control Plane Service - US", "cmdb_ci_service")
	add("Choreo", "cmdb_ci_service")
	return recs
}

func searchITServices(t *testing.T, fake *fakeServiceNowServices, query string, offset, limit int) domain.SearchITServicesResponse {
	t.Helper()
	svc := NewServiceNowITServiceService(newTestSNClient(t, fake))
	resp, err := svc.SearchITServices(context.Background(), domain.SearchITServicesRequest{
		Filters:    &domain.SearchITServicesFilters{SearchQuery: query},
		Pagination: domain.Pagination{Offset: offset, Limit: limit},
	})
	if err != nil {
		t.Fatalf("SearchITServices(%q, offset %d, limit %d): %v", query, offset, limit, err)
	}
	return resp
}

func itServiceNames(resp domain.SearchITServicesResponse) []string {
	names := make([]string, len(resp.Services))
	for i, s := range resp.Services {
		names[i] = derefString(s.Name)
	}
	return names
}

// The reported bug: "Choreo" is the 55th of 55 matches, so a page of 20 in
// upstream order never shows it. It must come first, then the other services
// that start with the query, then the offerings that do, then the services
// that only contain it.
func TestSNITServiceSearch_ExactMatchComesFirstAmongManyMatches(t *testing.T) {
	for _, query := range []string{"Choreo", "choreo", "  CHOREO "} {
		fake := &fakeServiceNowServices{records: choreoRecords()}
		resp := searchITServices(t, fake, query, 0, 20)
		names := itServiceNames(resp)

		if len(names) != 20 {
			t.Fatalf("query %q: got %d rows, want a full page of 20", query, len(names))
		}
		if resp.Total != 55 {
			t.Errorf("query %q: Total = %d, want the upstream's 55", query, resp.Total)
		}
		if names[0] != "Choreo" {
			t.Errorf("query %q: first row = %q, want the exact match \"Choreo\"; rows: %q", query, names[0], names)
		}
		// Next: the five other services whose name starts with the query, then offerings.
		wantNext := []string{
			"Choreo EU - Data Plane Service",
			"Choreo EU - Control Plane Service",
			"Choreo Data Plane Service - EU",
			"Choreo Data Plane Service - US",
			"Choreo Control Plane Service - US",
		}
		for i, want := range wantNext {
			if got := names[1+i]; got != want {
				t.Errorf("query %q: row %d = %q, want %q (services before offerings, upstream order kept)", query, 1+i, got, want)
			}
		}
		for i := 1 + len(wantNext); i < len(names); i++ {
			if !strings.HasSuffix(names[i], "Service Offering") {
				t.Errorf("query %q: row %d = %q, want a service offering after every service", query, i, names[i])
			}
		}
	}
}

// Every page of a ranked search is a slice of one order: walking the pages
// returns each match once, "Choreo" first and the substring-only matches last.
func TestSNITServiceSearch_PagesPartitionTheRankedMatches(t *testing.T) {
	fake := &fakeServiceNowServices{records: choreoRecords()}

	var all []string
	for offset := 0; offset < 60; offset += 20 {
		resp := searchITServices(t, fake, "choreo", offset, 20)
		if resp.Offset != offset || resp.Limit != 20 {
			t.Errorf("offset %d: response echoes offset %d limit %d", offset, resp.Offset, resp.Limit)
		}
		all = append(all, itServiceNames(resp)...)
	}

	if len(all) != 55 {
		t.Fatalf("pages returned %d rows in all, want 55", len(all))
	}
	seen := map[string]bool{}
	for _, name := range all {
		if seen[name] {
			t.Errorf("%q appears on more than one page", name)
		}
		seen[name] = true
	}
	if all[0] != "Choreo" {
		t.Errorf("first row overall = %q, want \"Choreo\"", all[0])
	}
	// The four "...choreo-alert-integration" services contain the query only inside a word.
	for _, name := range all[len(all)-4:] {
		if !strings.HasPrefix(name, "client-") {
			t.Errorf("last rows should be the substring-only matches, got %q", name)
		}
	}
}

// 55 matches is two upstream pages of 50, fetched once each per search; a search
// that fits one page costs one call.
func TestSNITServiceSearch_UpstreamCallCount(t *testing.T) {
	cases := []struct {
		name      string
		query     string
		wantCalls int32
	}{
		{"55 matches need two pages", "choreo", 2},
		{"a handful of matches fit the first page", "unrelated", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeServiceNowServices{records: choreoRecords()}
			searchITServices(t, fake, tc.query, 0, 20)
			if got := fake.calls.Load(); got != tc.wantCalls {
				t.Errorf("upstream calls = %d, want %d", got, tc.wantCalls)
			}
		})
	}
}

// With nothing typed there is nothing to rank by: one call, upstream order.
func TestSNITServiceSearch_EmptyQueryIsPassedThrough(t *testing.T) {
	fake := &fakeServiceNowServices{records: choreoRecords()}
	resp := searchITServices(t, fake, "", 0, 20)

	if got := fake.calls.Load(); got != 1 {
		t.Errorf("upstream calls = %d, want 1", got)
	}
	names := itServiceNames(resp)
	if names[0] != "Unrelated Billing Service" || names[1] != "client-uoechoreosub-alert-integration" {
		t.Errorf("rows are not in the upstream's order: %q", names[:3])
	}
	if resp.Total != len(choreoRecords()) {
		t.Errorf("Total = %d, want %d", resp.Total, len(choreoRecords()))
	}
}

// A caller that walks a huge result page by page (the SRE alert service does)
// must still reach every match: the ranked window holds the first
// snITServiceRankWindow upstream matches and the pages after it are the
// upstream's own, so nothing is skipped or repeated across the boundary.
func TestSNITServiceSearch_WalkingPastTheRankedWindowStillReachesEveryMatch(t *testing.T) {
	const matches = snITServiceRankWindow + 70
	var recs []snITService
	for i := 0; i < matches; i++ {
		recs = append(recs, fakeService(i+1, fmt.Sprintf("Widget %03d", i), "cmdb_ci_service"))
	}
	fake := &fakeServiceNowServices{records: recs}
	svc := NewServiceNowITServiceService(newTestSNClient(t, fake))

	seen := map[string]int{}
	offset := 0
	for offset < matches {
		resp, err := svc.SearchITServices(context.Background(), domain.SearchITServicesRequest{
			Filters:    &domain.SearchITServicesFilters{SearchQuery: "widget"},
			Pagination: domain.Pagination{Offset: offset, Limit: 50},
		})
		if err != nil {
			t.Fatalf("offset %d: %v", offset, err)
		}
		if len(resp.Services) == 0 {
			t.Fatalf("offset %d: empty page before the %d matches were reached (got %d)", offset, matches, len(seen))
		}
		for _, s := range resp.Services {
			seen[derefString(s.Name)]++
		}
		offset += len(resp.Services)
	}

	if len(seen) != matches {
		t.Errorf("walk reached %d distinct matches, want %d", len(seen), matches)
	}
	for name, n := range seen {
		if n != 1 {
			t.Errorf("%q returned %d times", name, n)
		}
	}
}

// A page that starts past the ranked window is one upstream call, not a rescan.
func TestSNITServiceSearch_PagePastTheWindowIsOneUpstreamCall(t *testing.T) {
	var recs []snITService
	for i := 0; i < snITServiceRankWindow+30; i++ {
		recs = append(recs, fakeService(i+1, fmt.Sprintf("Widget %03d", i), "cmdb_ci_service"))
	}
	fake := &fakeServiceNowServices{records: recs}
	resp := searchITServices(t, fake, "widget", snITServiceRankWindow, 20)

	if got := fake.calls.Load(); got != 1 {
		t.Errorf("upstream calls = %d, want 1", got)
	}
	if len(resp.Services) != 20 || derefString(resp.Services[0].Name) != fmt.Sprintf("Widget %03d", snITServiceRankWindow) {
		t.Errorf("page past the window = %q, want the upstream's rows starting at %d", itServiceNames(resp), snITServiceRankWindow)
	}
}

// A failed upstream page fails the search; it must not return a partial,
// differently ranked list.
func TestSNITServiceSearch_FailedUpstreamPageIsAnError(t *testing.T) {
	fake := &fakeServiceNowServices{records: choreoRecords(), failOffsets: map[int]bool{maxLimit: true}}
	svc := NewServiceNowITServiceService(newTestSNClient(t, fake))

	_, err := svc.SearchITServices(context.Background(), domain.SearchITServicesRequest{
		Filters:    &domain.SearchITServicesFilters{SearchQuery: "choreo"},
		Pagination: domain.Pagination{Limit: 20},
	})
	if err == nil {
		t.Fatal("expected an error when the second upstream page fails")
	}
}

func TestSNITServiceSearch_MapsUpstreamFields(t *testing.T) {
	name, class := "Choreo", "cmdb_ci_service"
	groupSysid := fmt.Sprintf("%032x", 999)
	fake := &fakeServiceNowServices{records: []snITService{{
		ID:                    fmt.Sprintf("%032x", 1),
		Name:                  &name,
		Class:                 &class,
		BusinessCriticality:   &snITServiceLabel{ID: "1"},
		ServiceClassification: &snITServiceLabel{Label: "Business Service"},
		SupportGroup:          &snITServiceLabel{ID: groupSysid, Label: "Artemis SRE Group"},
	}}}

	resp := searchITServices(t, fake, "choreo", 0, 20)
	if len(resp.Services) != 1 {
		t.Fatalf("got %d services, want 1", len(resp.Services))
	}
	got := resp.Services[0]
	if got.ID != sysidToUUID(fmt.Sprintf("%032x", 1)) {
		t.Errorf("ID = %q, want the sysid converted to a UUID", got.ID)
	}
	if got.SupportGroup == nil || got.SupportGroup.Name != "Artemis SRE Group" || got.SupportGroup.ID != sysidToUUID(groupSysid) {
		t.Errorf("SupportGroup = %+v, want Artemis SRE Group with a UUID id", got.SupportGroup)
	}
	if got.BusinessCriticality == nil || *got.BusinessCriticality != domain.BusinessCriticalityMostCritical {
		t.Errorf("BusinessCriticality = %v, want most critical", got.BusinessCriticality)
	}
	if got.ServiceClassification == nil || *got.ServiceClassification != domain.ServiceClassificationBusinessService {
		t.Errorf("ServiceClassification = %v, want business service", got.ServiceClassification)
	}
}

func TestSNITServiceSearch_RejectsALimitAboveTheCeiling(t *testing.T) {
	svc := NewServiceNowITServiceService(newTestSNClient(t, &fakeServiceNowServices{}))
	_, err := svc.SearchITServices(context.Background(), domain.SearchITServicesRequest{
		Filters:    &domain.SearchITServicesFilters{SearchQuery: "x"},
		Pagination: domain.Pagination{Limit: maxLimit + 1},
	})
	if err == nil {
		t.Fatal("expected a validation error")
	}
}

func TestItServiceMatchTier(t *testing.T) {
	cases := []struct {
		name, query string
		want        int
	}{
		{"choreo", "choreo", 0},
		{"choreo eu - dp metrics service offering", "choreo", 1},
		{"data choreo plane", "choreo", 2},
		{"client-choreo-alert", "choreo", 2},
		{"client-btchoreo-alert-integration", "choreo", 3},
		{"unrelated", "choreo", 3},
	}
	for _, tc := range cases {
		if got := itServiceMatchTier(tc.name, tc.query); got != tc.want {
			t.Errorf("itServiceMatchTier(%q, %q) = %d, want %d", tc.name, tc.query, got, tc.want)
		}
	}
}

// One real offering is spelled "Choreo  EU" with two spaces; whitespace and case
// must not decide whether a name is an exact or a prefix match.
func TestNormalizeITServiceName(t *testing.T) {
	if got := normalizeITServiceName("  Choreo   EU - DP  "); got != "choreo eu - dp" {
		t.Errorf("normalizeITServiceName = %q", got)
	}
	if itServiceMatchTier(normalizeITServiceName("Choreo  EU - DP API Proxies"), normalizeITServiceName("choreo eu")) != 1 {
		t.Error("a double-spaced name should still match its single-spaced prefix")
	}
}
