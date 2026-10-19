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
	"errors"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

type fakeRunbookLister struct {
	runbooks []domain.Runbook
	err      error
}

func (f fakeRunbookLister) List(context.Context) ([]domain.Runbook, error) {
	return f.runbooks, f.err
}

var testRunbooks = []domain.Runbook{
	{AlertName: "example-gw-CriticalHTTP5xxCountLimitExceeded-prod-001", RunbookURL: strp("https://docs.example.com/gw-5xx")},
	{AlertName: "example-gw-CriticalHTTP5xx", RunbookURL: strp("https://docs.example.com/short")},
	{AlertName: "Icinga Alert: demo-node-01 / JVM Heap"},
}

func TestRunbookNote(t *testing.T) {
	cases := []struct {
		name    string
		subject string
		corrID  *string
		want    string
	}{
		{"longest match wins, inside a longer subject", "[Alert]Fired:Sev0 Azure Monitor Alert EXAMPLE-gw-CriticalHTTP5xxCountLimitExceeded-prod-001 on log-002", nil, "Runbook for this alert: https://docs.example.com/gw-5xx"},
		{"shorter entry still matches on its own", "example-gw-CriticalHTTP5xx spike", nil, "Runbook for this alert: https://docs.example.com/short"},
		{"known alert without a link", "Icinga Alert: demo-node-01 / JVM Heap", nil, noRunbookNoteText},
		{"alert-born incident with no match", "unknown alert", strp("fp-1"), noRunbookNoteText},
		{"customer incident with no match gets nothing", "Login page is slow", nil, ""},
		{"blank correlation id is not alert-born", "Login page is slow", strp("  "), ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := runbookNote(testRunbooks, domain.CreateIncidentRequest{Subject: tc.subject, CorrelationID: tc.corrID})
			if got != tc.want {
				t.Fatalf("runbookNote = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestWithRunbookNote_AppendsAfterCallerNotes(t *testing.T) {
	req := domain.CreateIncidentRequest{Subject: "Icinga Alert: demo-node-01 / JVM Heap", WorkNotes: strp("Created by alert-core.")}
	got := withRunbookNote(context.Background(), req, fakeRunbookLister{runbooks: testRunbooks})
	want := "Created by alert-core.\n\n" + noRunbookNoteText
	if got.WorkNotes == nil || *got.WorkNotes != want {
		t.Fatalf("WorkNotes = %v, want %q", got.WorkNotes, want)
	}
}

func TestWithRunbookNote_CatalogFailureLeavesRequestAlone(t *testing.T) {
	req := domain.CreateIncidentRequest{Subject: "Icinga Alert: demo-node-01 / JVM Heap", WorkNotes: strp("keep")}
	got := withRunbookNote(context.Background(), req, fakeRunbookLister{err: errors.New("db down")})
	if got.WorkNotes == nil || *got.WorkNotes != "keep" {
		t.Fatalf("WorkNotes = %v, want unchanged", got.WorkNotes)
	}
}

func TestWithRunbookNote_NoCatalogIsNoOp(t *testing.T) {
	req := domain.CreateIncidentRequest{Subject: "x", CorrelationID: strp("fp")}
	if got := withRunbookNote(context.Background(), req, nil); got.WorkNotes != nil {
		t.Fatalf("WorkNotes = %v, want nil", *got.WorkNotes)
	}
}

func TestNormalizeRunbook(t *testing.T) {
	got, err := normalizeRunbook(domain.SaveRunbookRequest{AlertName: "  a  ", RunbookURL: strp("  ")})
	if err != nil || got.AlertName != "a" || got.RunbookURL != nil {
		t.Fatalf("normalizeRunbook = %+v, %v; want trimmed name and nil link", got, err)
	}
	for _, bad := range []domain.SaveRunbookRequest{
		{AlertName: " "},
		{AlertName: "a", RunbookURL: strp("javascript:alert(1)")},
		{AlertName: "a", RunbookURL: strp("docs.example.com/x")},
	} {
		if _, err := normalizeRunbook(bad); err == nil {
			t.Fatalf("normalizeRunbook(%+v) accepted, want a validation error", bad)
		}
	}
}
