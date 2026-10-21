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
	"fmt"
	"html"
	"regexp"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// handoffIssueInput is what the internal GitHub issue of a specialist handoff
// is built from: the incident as the handoff found it, and the handoff
// itself.
type handoffIssueInput struct {
	Incident    repository.SpecialistHandoffSnapshot
	TeamLabel   string
	Reason      string
	ActorName   string
	TaskNumber  string
	EscalatedOn time.Time
	// PortalBaseURL is the CSM portal's address (CSM_PORTAL_BASE_URL); empty
	// shows the incident number without a link.
	PortalBaseURL string
}

// githubIssueTitleMax is GitHub's limit on an issue title.
const githubIssueTitleMax = 256

// handoffIssueTimeLayout is how the issue shows times, always in UTC.
const handoffIssueTimeLayout = "2006-01-02 15:04 UTC"

// handoffNoteMax bounds one work note in the issue's list.
const handoffNoteMax = 400

// incidentPriorityShort and incidentPriorityLabel are how the issue names an
// incident priority: "P1" in the title, "P1 Critical" in the table.
var (
	incidentPriorityShort = map[string]string{
		string(domain.IncidentPriorityCritical): "P1", string(domain.IncidentPriorityHigh): "P2",
		string(domain.IncidentPriorityModerate): "P3", string(domain.IncidentPriorityLow): "P4",
		string(domain.IncidentPriorityPlanning): "P5",
	}
	incidentPriorityLabel = map[string]string{
		string(domain.IncidentPriorityCritical): "P1 Critical", string(domain.IncidentPriorityHigh): "P2 High",
		string(domain.IncidentPriorityModerate): "P3 Moderate", string(domain.IncidentPriorityLow): "P4 Low",
		string(domain.IncidentPriorityPlanning): "P5 Planning",
	}
	incidentCategoryLabel = map[string]string{
		string(domain.IncidentCategoryInquiry):             "Inquiry",
		string(domain.IncidentCategoryServiceInterruption): "Service interruption",
		string(domain.IncidentCategorySecurity):            "Security",
	}
)

// buildHandoffIssueTitle is "[INC…] [P1] <subject>": the incident number and
// priority first, so the issue list can be searched by incident and triaged
// at a glance. No priority leaves its bracket out.
func buildHandoffIssueTitle(inc repository.SpecialistHandoffSnapshot) string {
	title := "[" + inc.Number + "] "
	if p := incidentPriorityShort[enumKey(inc.Priority)]; p != "" {
		title += "[" + p + "] "
	}
	title += issueOneLine(inc.Subject)
	if r := []rune(title); len(r) > githubIssueTitleMax {
		title = string(r[:githubIssueTitleMax-1]) + "…"
	}
	return title
}

// buildHandoffIssueBody is the issue body: a table of what the receiving team
// needs at a glance (rows without a value are left out), the description --
// the incident's, or for an alert-born incident with none the alert details
// (handoffIssueBody) -- the latest work notes, and a footer saying the issue
// is not linked back to the incident.
func buildHandoffIssueBody(in handoffIssueInput) string {
	inc := in.Incident
	var b strings.Builder
	fmt.Fprintf(&b, "## Special Ops escalation: %s\n\n| | |\n|---|---|\n", inc.Number)
	row := func(name, value string) {
		if value = issueCell(value); value != "" {
			fmt.Fprintf(&b, "| **%s** | %s |\n", name, value)
		}
	}

	incident := issueCell(inc.Number)
	if base := strings.TrimRight(strings.TrimSpace(in.PortalBaseURL), "/"); base != "" && inc.IncidentID != "" {
		incident = "[" + incident + "](" + base + "/operations/incidents/" + inc.IncidentID + ")"
	}
	fmt.Fprintf(&b, "| **Incident** | %s |\n", incident)
	row("Priority", issuePriorityText(inc))
	row("Service", derefString(inc.ServiceName))
	row("Environment", derefString(inc.Environment))
	row("Category", incidentCategoryLabel[enumKey(inc.Category)])
	row("Escalated to", in.TeamLabel)
	row("Reason", in.Reason)
	row("Escalated by", in.ActorName)
	row("Runbook task", in.TaskNumber)
	if !inc.OpenedOn.IsZero() {
		row("Incident opened", inc.OpenedOn.UTC().Format(handoffIssueTimeLayout))
	}
	if !in.EscalatedOn.IsZero() {
		row("Escalated", in.EscalatedOn.UTC().Format(handoffIssueTimeLayout))
	}

	b.WriteString("\n### Description\n\n")
	if d := strings.TrimSpace(handoffIssueBody(inc)); d != "" {
		b.WriteString(d)
	} else {
		b.WriteString("_No description._")
	}
	b.WriteString("\n")

	if len(inc.RecentWorkNotes) > 0 {
		b.WriteString("\n### Recent work notes\n\n")
		for _, n := range inc.RecentWorkNotes {
			text := issueNoteText(n.Content)
			if text == "" {
				continue
			}
			fmt.Fprintf(&b, "- **%s** %s: %s\n", n.CreatedOn.UTC().Format(handoffIssueTimeLayout), issueOneLine(n.Author), text)
		}
	}

	fmt.Fprintf(&b, "\n---\n<sub>Raised from the CSM portal when %s was escalated to Special Ops. "+
		"This issue isn't linked to the incident: comments here don't reach the CSM portal.</sub>\n", inc.Number)
	return b.String()
}

// issuePriorityText is "P1 Critical (impact High, urgency High)", with whatever
// parts the incident has.
func issuePriorityText(inc repository.SpecialistHandoffSnapshot) string {
	p := incidentPriorityLabel[enumKey(inc.Priority)]
	var iu []string
	if v := enumTitle(derefString(inc.Impact)); v != "" {
		iu = append(iu, "impact "+v)
	}
	if v := enumTitle(derefString(inc.Urgency)); v != "" {
		iu = append(iu, "urgency "+v)
	}
	if len(iu) == 0 {
		return p
	}
	if p == "" {
		return strings.Join(iu, ", ")
	}
	return p + " (" + strings.Join(iu, ", ") + ")"
}

var (
	noteBreakTags = regexp.MustCompile(`(?i)<\s*(br|/p|/li|/div|/tr)\s*/?>`)
	noteTags      = regexp.MustCompile(`<[^>]*>`)
	issueSpaces   = regexp.MustCompile(`\s+`)
)

// issueNoteText is a work note as one line of plain text for the issue's list:
// the portal stores notes as HTML, and a list item can't hold its markup.
// Long notes are cut, as the full note is in the CSM portal.
func issueNoteText(content string) string {
	t := noteBreakTags.ReplaceAllString(content, " ")
	t = noteTags.ReplaceAllString(t, "")
	t = issueOneLine(html.UnescapeString(t))
	if r := []rune(t); len(r) > handoffNoteMax {
		t = strings.TrimSpace(string(r[:handoffNoteMax])) + "…"
	}
	return t
}

// issueCell makes a value safe inside a Markdown table cell: one line, with the
// column separator escaped.
func issueCell(s string) string {
	return strings.ReplaceAll(issueOneLine(s), "|", `\|`)
}

// issueOneLine collapses all whitespace, line breaks included, to single issueSpaces.
func issueOneLine(s string) string {
	return strings.TrimSpace(issueSpaces.ReplaceAllString(s, " "))
}

func enumKey(s *string) string { return strings.ToUpper(strings.TrimSpace(derefString(s))) }

// enumTitle turns an enum label such as "HIGH" into "High".
func enumTitle(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[:1]) + strings.ToLower(s[1:])
}
