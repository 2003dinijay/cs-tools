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
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

func TestBuildHandoffIssueTitle(t *testing.T) {
	snap := repository.SpecialistHandoffSnapshot{Number: "INC0087812", Subject: "OpenSearch Alert:\n  ApiServer 5xx"}
	if got := buildHandoffIssueTitle(snap); got != "[INC0087812] OpenSearch Alert: ApiServer 5xx" {
		t.Errorf("no priority: %q", got)
	}
	snap.Priority = hoStr("moderate")
	if got := buildHandoffIssueTitle(snap); got != "[INC0087812] [P3] OpenSearch Alert: ApiServer 5xx" {
		t.Errorf("with priority: %q", got)
	}
	snap.Subject = strings.Repeat("é", 300)
	got := []rune(buildHandoffIssueTitle(snap))
	if len(got) != githubIssueTitleMax || got[len(got)-1] != '…' {
		t.Errorf("long subject: %d runes ending %q, want %d ending …", len(got), string(got[len(got)-1]), githubIssueTitleMax)
	}
}

// The whole body for an alert-born incident with every field set: the table,
// the alert details as the description, the latest notes as plain text, and
// the footer.
func TestBuildHandoffIssueBody_Full(t *testing.T) {
	at := func(s string) time.Time {
		v, err := time.Parse(time.RFC3339, s)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	alert := repository.AlertCreationNotePrefix + " ALT000000436</p><table><tr><td>service</td><td>Choreo</td></tr></table>"
	body := buildHandoffIssueBody(handoffIssueInput{
		Incident: repository.SpecialistHandoffSnapshot{
			IncidentID: "11111111-1111-4111-8111-111111111111", Number: "INC0087812", Subject: "ApiServer 5xx",
			AlertNote: &alert, Priority: hoStr("CRITICAL"), Impact: hoStr("HIGH"), Urgency: hoStr("HIGH"),
			Category: hoStr("SERVICE_INTERRUPTION"), Environment: hoStr("prod"), ServiceName: hoStr("Choreo"),
			OpenedOn: at("2026-10-07T07:39:00+05:30"),
			RecentWorkNotes: []repository.SpecialistHandoffNote{
				{CreatedOn: at("2026-10-07T02:13:00Z"), Author: "A. Engineer", Content: "<p>Restarted the pods.</p><p>5xx rate &amp; latency unchanged.</p>"},
				{CreatedOn: at("2026-10-07T02:10:00Z"), Author: "a.engineer@example.com", Content: "Followed the runbook."},
			},
		},
		TeamLabel: "Choreo Runtime Team", Reason: "Runbook doesn't solve the incident", ActorName: "A. Engineer",
		TaskNumber: "TASK1000123", EscalatedOn: at("2026-10-07T02:15:00Z"), PortalBaseURL: "https://csm.example/",
	})
	want := `## Special Ops escalation: INC0087812

| | |
|---|---|
| **Incident** | [INC0087812](https://csm.example/operations/incidents/11111111-1111-4111-8111-111111111111) |
| **Priority** | P1 Critical (impact High, urgency High) |
| **Service** | Choreo |
| **Environment** | prod |
| **Category** | Service interruption |
| **Escalated to** | Choreo Runtime Team |
| **Reason** | Runbook doesn't solve the incident |
| **Escalated by** | A. Engineer |
| **Runbook task** | TASK1000123 |
| **Incident opened** | 2026-10-07 02:09 UTC |
| **Escalated** | 2026-10-07 02:15 UTC |

### Description

` + alert + `

### Recent work notes

- **2026-10-07 02:13 UTC** A. Engineer: Restarted the pods. 5xx rate & latency unchanged.
- **2026-10-07 02:10 UTC** a.engineer@example.com: Followed the runbook.

---
<sub>Raised from the CSM portal when INC0087812 was escalated to Special Ops. This issue isn't linked to the incident: comments here don't reach the CSM portal.</sub>
`
	if body != want {
		t.Errorf("body:\n%s\nwant:\n%s", body, want)
	}
}

// Rows without a value are left out, the number is unlinked without a portal
// address, there is no notes section without notes, and values can't break
// the table.
func TestBuildHandoffIssueBody_Sparse(t *testing.T) {
	body := buildHandoffIssueBody(handoffIssueInput{
		Incident:  repository.SpecialistHandoffSnapshot{IncidentID: "x", Number: "INC0000001", Subject: "s", Urgency: hoStr("LOW")},
		TeamLabel: "Team | A\nB", Reason: "r",
	})
	for _, absent := range []string{"**Service**", "**Environment**", "**Category**", "**Escalated by**", "**Runbook task**",
		"**Incident opened**", "**Escalated**", "Recent work notes", "](", "| **Priority** | P"} {
		if strings.Contains(body, absent) {
			t.Errorf("body has %q:\n%s", absent, body)
		}
	}
	for _, present := range []string{"| **Incident** | INC0000001 |", "| **Priority** | urgency Low |",
		`| **Escalated to** | Team \| A B |`, "### Description\n\n_No description._\n"} {
		if !strings.Contains(body, present) {
			t.Errorf("body lacks %q:\n%s", present, body)
		}
	}
}

func TestIssueNoteText(t *testing.T) {
	if got := issueNoteText("<p>line one<br/>line   two</p>\n<ul><li>a</li><li>b</li></ul>"); got != "line one line two a b" {
		t.Errorf("html note: %q", got)
	}
	long := issueNoteText(strings.Repeat("word ", 200))
	if r := []rune(long); len(r) > handoffNoteMax+1 || !strings.HasSuffix(long, "…") {
		t.Errorf("long note: %d runes, %q…", len(r), long[:20])
	}
	if got := issueNoteText("<p> </p>"); got != "" {
		t.Errorf("empty note: %q", got)
	}
}
