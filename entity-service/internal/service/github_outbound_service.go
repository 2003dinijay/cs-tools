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
	"fmt"
	"log/slog"
	"math"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/github"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// Outbound event names, matching what the database triggers enqueue.
const (
	outboundCRCreated    = "cr_created"
	outboundCRUpdated    = "cr_updated"
	outboundCommentAdded = "comment_added"
	// The two halves of "[GitHub Integration] SN Case Updates -> GitHub".
	// That flow is one trigger with a branch on the state's display value; the
	// branch is in the database triggers here, so each arm gets its own event.
	outboundCaseClosed   = "case_closed"
	outboundCaseAssigned = "case_assigned"
)

const (
	// outboundMaxAttempts bounds retrying. Past this the row is FAILED and
	// stays visible rather than being retried forever against something no
	// retry can fix -- a deleted issue, a revoked token.
	outboundMaxAttempts = 6
	outboundBaseBackoff = 30 * time.Second
	outboundMaxBackoff  = 2 * time.Hour
)

// GithubOutboundService pushes change-request activity to the linked issue.
//
// This is the outbound half of the sync, replacing ServiceNow's
// [GitHub Integration] flows. Everything it does is a comment, a label or a
// state change on the issue named by the change request's git_reference.
type GithubOutboundService interface {
	// Deliver pushes one queued item. A nil error means delivered.
	Deliver(ctx context.Context, item repository.OutboundItem) error
}

type githubOutboundService struct {
	gh githubIssueClient
	// skipAuthors are comment authors whose comments are never mirrored.
	//
	// The case journal carries system-generated text as well as what people
	// write: auto-closure reminders addressed to "Hi team", and the "Change
	// request (CHGxxxxxxx) is created." notices this service already posts
	// itself from its own triggers. On staging that is 7,915 and 278 case
	// comments respectively. Neither belongs on a page a customer reads.
	//
	// CONFIGURABLE, WITH A DEFAULT, because what ServiceNow did here could not
	// be established: the flow has no sys_hub_trigger_instance row, its
	// snapshot carries no readable condition, and the linked issues sit in a
	// private repository. Rather than guess at fidelity, the default excludes
	// the obvious machine authors and one env var restores the old behaviour
	// without a deploy.
	skipAuthors map[string]bool
	// assignedLabel is put on the issue when the case is assigned and taken
	// off when it closes, matching the workflow this replaces. Empty disables
	// both, for a repository that does not use the label.
	assignedLabel string
	// portalBaseURL builds the link back to the change request, so someone
	// reading the issue can reach the record. Empty omits the link rather than
	// rendering a broken one.
	portalBaseURL string
}

// NewGithubOutboundService constructs the outbound pusher.
func NewGithubOutboundService(gh githubIssueClient, portalBaseURL string, skipAuthors []string, assignedLabel string) GithubOutboundService {
	skip := make(map[string]bool, len(skipAuthors))
	for _, a := range skipAuthors {
		if a = strings.ToLower(strings.TrimSpace(a)); a != "" {
			skip[a] = true
		}
	}
	return &githubOutboundService{gh: gh, portalBaseURL: strings.TrimRight(portalBaseURL, "/"), skipAuthors: skip, assignedLabel: strings.TrimSpace(assignedLabel)}
}

// Deliver implements GithubOutboundService.
func (s *githubOutboundService) Deliver(ctx context.Context, item repository.OutboundItem) error {
	// Taken from the row, not parsed out of a URL. The trigger resolved these
	// against the mapping as it stood when the change happened, which is the
	// issue this row is about even if the case has since been re-linked.
	if item.Owner == "" || item.Repository == "" || item.IssueNumber <= 0 {
		return fmt.Errorf("%w: queue row %d has no issue to post to", ErrOutboundPermanent, item.ID)
	}
	issue := github.Issue{Owner: item.Owner, Repository: item.Repository, Number: item.IssueNumber}

	body, err := s.render(item)
	if err != nil {
		return err
	}
	if body == "" {
		// Nothing worth saying. Not a failure: most change-request updates
		// touch columns nobody reading a GitHub issue cares about.
		return nil
	}

	if _, err := s.gh.CreateComment(ctx, issue, body); err != nil {
		return err
	}
	return s.applyIssueState(ctx, issue, item)
}

// applyIssueState mirrors the case's state onto the issue itself.
//
// WHY THIS EXISTS, HAVING ONCE BEEN REFUSED. An earlier version of this file
// said "comments only, deliberately", reasoning that writing labels or state
// back would fight the inbound half, since label and state changes on an issue
// are what drive the change request. That was overcautious: our own writes
// return as webhooks whose sender is the integration account, and the loop
// guard drops them by identity before any of that logic runs.
//
// Reading the GitHub Actions workflows ServiceNow dispatched to settled it.
// "ServiceNow Case Updates -> GitHub" does three things on closure -- comments,
// removes the assigned label, and CLOSES THE ISSUE -- and two on assignment.
// Posting only the comment left every issue open forever after its case closed.
//
// Failures here are returned, so the row is retried. The comment is already
// posted, and a retry re-posts it; that is the accepted trade, because an issue
// left open after its case closed is the worse outcome and the queue has no
// per-step resume.
func (s *githubOutboundService) applyIssueState(ctx context.Context, issue github.Issue, item repository.OutboundItem) error {
	switch item.Event {
	case outboundCaseAssigned:
		if s.assignedLabel == "" {
			return nil
		}
		// Added, not set: SetLabels replaces the whole set and would drop
		// every label a person put on the issue by hand.
		return s.gh.AddLabel(ctx, issue, s.assignedLabel)

	case outboundCaseClosed:
		if s.assignedLabel != "" {
			// Already absent is success -- RemoveLabel treats 404 that way.
			if err := s.gh.RemoveLabel(ctx, issue, s.assignedLabel); err != nil {
				return err
			}
		}
		return s.gh.SetState(ctx, issue, github.StateClosed)
	}
	return nil
}

// ErrOutboundPermanent marks a failure that retrying cannot fix.
var ErrOutboundPermanent = errors.New("github outbound: permanent failure")

// render turns a queued item into the comment to post, or "" for nothing.
//
// COMMENTS ONLY, DELIBERATELY. ServiceNow's outbound flows also drove issue
// labels and state, but doing that from here would fight the inbound half:
// label and state changes on the issue are what DRIVE the change request, so
// writing them back is how a sync loop starts. Our own events are dropped by
// sender identity, but a loop that depends on one guard is worse than one that
// cannot form.
func (s *githubOutboundService) render(item repository.OutboundItem) (string, error) {
	switch item.Event {
	case outboundCommentAdded:
		content, _ := item.Payload["content"].(string)
		author, _ := item.Payload["createdBy"].(string)
		// Work notes never reach here: the database trigger only enqueues
		// COMMENT-type rows, matching ServiceNow hardcoding note_type to
		// 'additional_comments' -- the customer-visible journal. This second
		// check is belt and braces, since the cost of being wrong is internal
		// text on a public issue.
		if kind, ok := item.Payload["type"].(string); ok && strings.EqualFold(kind, "WORK_NOTE") {
			return "", nil
		}
		// Machine-written journal entries. Dropped by AUTHOR rather than by
		// matching the text: the spec's own rule, and a body match would break
		// the moment somebody reworded the auto-closure template.
		if s.skipAuthors[strings.ToLower(strings.TrimSpace(author))] {
			return "", nil
		}
		// ServiceNow stored these as HTML fragments; a GitHub comment is
		// Markdown. Converted before the emptiness check, since a body that is
		// nothing but markup has no text in it to post.
		content = snToMarkdown(content)
		if strings.TrimSpace(content) == "" {
			return "", nil
		}
		// The CASE, not the change request: the trigger for this event fires on
		// comments against the case and enqueues the case's id. Linking it as a
		// change request produced a /operations/change-requests/ URL with a
		// case id in it -- a 404 for anyone who clicked it.
		return fmt.Sprintf("**%s** commented on %s:\n\n%s",
			displayAuthor(author), s.caseLink(item.WorkItemID), content), nil

	case outboundCRCreated:
		// The whole change picture for the case, matching the workflow this
		// replaces: it called back into ServiceNow for the sibling list rather
		// than announcing one CR in isolation.
		msg := fmt.Sprintf("A change request has been raised for this issue: %s",
			s.changeRequestLink(item.WorkItemID))
		if list := describeSiblings(item.Payload["siblings"]); list != "" {
			msg += "\n\n**All change requests for this case:**\n" + list
		}
		return msg, nil

	case outboundCRUpdated:
		changes, _ := item.Payload["changes"].(map[string]any)
		lines := describeChanges(changes)
		if len(lines) == 0 {
			return "", nil
		}
		verb := "was updated"
		if outboundAction(changes) == "state_changed" {
			verb = "changed state"
		}
		return fmt.Sprintf("%s %s:\n\n%s",
			s.changeRequestLink(item.WorkItemID), verb, strings.Join(lines, "\n")), nil

	case outboundCaseClosed:
		// ServiceNow posted the resolution notes verbatim and nothing when they
		// were empty. An issue closed with no explanation still deserves the
		// notice, so the closure line always goes; the notes are what is
		// conditional.
		notes, _ := item.Payload["resolutionNotes"].(string)
		msg := fmt.Sprintf("%s has been closed.", s.caseLink(item.WorkItemID))
		if strings.TrimSpace(notes) != "" {
			msg += "\n\n" + notes
		}
		return msg, nil

	case outboundCaseAssigned:
		// The assignee is a name, never an address: this comment lands on an
		// issue a customer can read.
		assignee, _ := item.Payload["assignedTo"].(string)
		if strings.TrimSpace(assignee) == "" {
			return "", nil
		}
		return fmt.Sprintf("%s has been assigned to **%s**.",
			s.caseLink(item.WorkItemID), assignee), nil
	}
	return "", fmt.Errorf("%w: unknown event %q", ErrOutboundPermanent, item.Event)
}

// outboundReportable is the set of columns worth telling GitHub about.
//
// EXACTLY THE FOUR SERVICENOW WATCHED, taken from the trigger on
// "[GitHub Integration] SN CR Updates -> GitHub":
//
//	State changes, or Assigned to changes, or Planned start date changes,
//	or Planned end date changes; and Parent is not empty
//
// Impact, likelihood and risk are deliberately absent -- an earlier guess
// included them, and they would put field changes on a customer-visible issue
// that the integration being replaced never sent.
//
// An allow-list rather than a deny-list, so a column added to change_request
// later is silent by default rather than leaking because nobody remembered to
// exclude it.
var outboundReportable = map[string]string{
	"state":              "State",
	"assigned_to_id":     "Assigned to",
	"planned_start_date": "Planned start",
	"planned_end_date":   "Planned end",
}

// outboundAction classifies an update the way ServiceNow's "Read CR Update"
// step did: a state move takes priority, and anything else reportable is a
// date change.
func outboundAction(changes map[string]any) string {
	if _, ok := changes["state"]; ok {
		return "state_changed"
	}
	return "dates_updated"
}

func describeChanges(changes map[string]any) []string {
	var out []string
	for column, label := range outboundReportable {
		raw, ok := changes[column]
		if !ok {
			continue
		}
		diff, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		from := displayValue(diff["from"])
		to := displayValue(diff["to"])
		if from == to {
			continue
		}
		out = append(out, fmt.Sprintf("- **%s**: %s → %s", label, from, to))
	}
	return out
}

func displayValue(v any) string {
	if v == nil {
		return "_empty_"
	}
	s := strings.TrimSpace(fmt.Sprintf("%v", v))
	if s == "" {
		return "_empty_"
	}
	return s
}

// displayAuthor keeps an address out of a public issue: a comment author is
// stored as an email, and GitHub issues are read by people outside WSO2.
func displayAuthor(author string) string {
	if at := strings.Index(author, "@"); at > 0 {
		return author[:at]
	}
	if author == "" {
		return "A WSO2 engineer"
	}
	return author
}

func (s *githubOutboundService) changeRequestLink(id string) string {
	return s.portalLink(id, "change request", "operations/change-requests")
}

func (s *githubOutboundService) caseLink(id string) string {
	return s.portalLink(id, "case", "cases")
}

// portalLink renders a markdown link into the portal, or bare prose when no
// base URL is configured -- a comment with the wrong host in it is worse than
// one without a link, and this runs against a public issue.
func (s *githubOutboundService) portalLink(id, noun, path string) string {
	if s.portalBaseURL == "" {
		return "The " + noun
	}
	return fmt.Sprintf("[The %s](%s/%s/%s)", noun, s.portalBaseURL, path, id)
}

// OutboundBackoff is how long to wait before attempt n, doubling and capped.
func OutboundBackoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	d := time.Duration(float64(outboundBaseBackoff) * math.Pow(2, float64(attempt-1)))
	if d > outboundMaxBackoff {
		return outboundMaxBackoff
	}
	return d
}

// OutboundRetryAfter reports how long GitHub asked us to wait, when it did.
// Honouring it is what keeps us inside the rate limit rather than hammering
// through it.
func OutboundRetryAfter(err error) (time.Duration, bool) {
	var apiErr *github.Error
	if errors.As(err, &apiErr) && apiErr.RetryAfter > 0 {
		return apiErr.RetryAfter, true
	}
	return 0, false
}

// OutboundPermanent reports whether retrying is pointless.
func OutboundPermanent(err error) bool {
	if errors.Is(err, ErrOutboundPermanent) {
		return true
	}
	var apiErr *github.Error
	if errors.As(err, &apiErr) {
		// 404: the issue is gone or the token cannot see it. 403 without a
		// Retry-After is a permissions problem, not a rate limit.
		if apiErr.StatusCode == 404 || apiErr.StatusCode == 410 {
			return true
		}
		if apiErr.StatusCode == 403 && !apiErr.RateLimited() {
			return true
		}
	}
	return false
}

var _ = slog.Default

// describeSiblings renders the case's other change requests as a list. Empty
// when there are none to show, so a case with a single CR does not get a
// one-item list restating what the sentence above it already said.
func describeSiblings(raw any) string {
	rows, ok := raw.([]any)
	if !ok || len(rows) < 2 {
		return ""
	}
	var b strings.Builder
	for _, r := range rows {
		m, ok := r.(map[string]any)
		if !ok {
			continue
		}
		number, _ := m["number"].(string)
		state, _ := m["state"].(string)
		if number == "" {
			continue
		}
		fmt.Fprintf(&b, "- %s — %s\n", number, displayValue(state))
	}
	return strings.TrimRight(b.String(), "\n")
}
