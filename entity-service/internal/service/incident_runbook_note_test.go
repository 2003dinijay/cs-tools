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
	"strings"
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
	linked := runbookNote(testRunbooks, domain.CreateIncidentRequest{
		Subject: "[Alert]Fired:Sev0 Azure Monitor Alert EXAMPLE-gw-CriticalHTTP5xxCountLimitExceeded-prod-001 on log-002"})
	want := `<p><strong>Runbook</strong></p><p>Matched alert: <code>example-gw-CriticalHTTP5xxCountLimitExceeded-prod-001</code></p>` +
		`<p><a href="https://docs.example.com/gw-5xx" target="_blank" rel="noopener noreferrer">Open runbook</a></p>`
	if linked != want {
		t.Fatalf("linked note = %q, want %q", linked, want)
	}

	short := runbookNote(testRunbooks, domain.CreateIncidentRequest{Subject: "example-gw-CriticalHTTP5xx spike"})
	if !strings.Contains(short, `href="https://docs.example.com/short"`) {
		t.Fatalf("shorter entry did not match on its own: %q", short)
	}

	unlinked := runbookNote(testRunbooks, domain.CreateIncidentRequest{Subject: "Icinga Alert: demo-node-01 / JVM Heap"})
	if want := `<p><strong>Runbook</strong></p><p>Matched alert: <code>Icinga Alert: demo-node-01 / JVM Heap</code></p><p>` + noRunbookNoteText + `</p>`; unlinked != want {
		t.Fatalf("unlinked note = %q, want %q", unlinked, want)
	}

	if got := runbookNote(testRunbooks, domain.CreateIncidentRequest{Subject: "unknown alert", CorrelationID: strp("fp-1")}); got != "<p><strong>Runbook</strong></p><p>"+noRunbookNoteText+"</p>" {
		t.Fatalf("alert-born no-match note = %q", got)
	}
	for _, corr := range []*string{nil, strp("  ")} {
		if got := runbookNote(testRunbooks, domain.CreateIncidentRequest{Subject: "Login page is slow", CorrelationID: corr}); got != "" {
			t.Fatalf("customer incident got a note: %q", got)
		}
	}
}

func TestRunbookNote_EscapesCatalogValues(t *testing.T) {
	rbs := []domain.Runbook{{AlertName: "<script>x</script>", RunbookURL: strp(`https://docs.example.com/a?x="><img src=y>`)}}
	got := runbookNote(rbs, domain.CreateIncidentRequest{Subject: "fired <script>x</script> now"})
	if strings.Contains(got, "<script>") || strings.Contains(got, "<img") || strings.Contains(got, `"><`) {
		t.Fatalf("catalog values reached the note unescaped: %q", got)
	}
}

func TestWithRunbookNote_IsASeparateNote(t *testing.T) {
	req := domain.CreateIncidentRequest{Subject: "Icinga Alert: demo-node-01 / JVM Heap", WorkNotes: strp("<p>Created by alert-core.</p>")}
	got := withRunbookNote(context.Background(), req, fakeRunbookLister{runbooks: testRunbooks})
	if got.WorkNotes == nil || *got.WorkNotes != "<p>Created by alert-core.</p>" {
		t.Fatalf("WorkNotes = %v, want the caller's note untouched", got.WorkNotes)
	}
	if got.RunbookWorkNote == nil || !strings.HasPrefix(*got.RunbookWorkNote, "<p><strong>Runbook</strong></p>") {
		t.Fatalf("RunbookWorkNote = %v, want the runbook note", got.RunbookWorkNote)
	}
}

func TestWithRunbookNote_CatalogFailureLeavesRequestAlone(t *testing.T) {
	req := domain.CreateIncidentRequest{Subject: "Icinga Alert: demo-node-01 / JVM Heap", WorkNotes: strp("keep")}
	got := withRunbookNote(context.Background(), req, fakeRunbookLister{err: errors.New("db down")})
	if got.WorkNotes == nil || *got.WorkNotes != "keep" || got.RunbookWorkNote != nil {
		t.Fatalf("WorkNotes = %v, RunbookWorkNote = %v; want unchanged and no runbook note", got.WorkNotes, got.RunbookWorkNote)
	}
}

func TestWithRunbookNote_NoCatalogIsNoOp(t *testing.T) {
	req := domain.CreateIncidentRequest{Subject: "x", CorrelationID: strp("fp")}
	if got := withRunbookNote(context.Background(), req, nil); got.WorkNotes != nil || got.RunbookWorkNote != nil {
		t.Fatalf("WorkNotes = %v, RunbookWorkNote = %v; want both nil", got.WorkNotes, got.RunbookWorkNote)
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
