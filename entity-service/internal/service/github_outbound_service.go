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
	// portalBaseURL builds the link back to the change request, so someone
	// reading the issue can reach the record. Empty omits the link rather than
	// rendering a broken one.
	portalBaseURL string
}

// NewGithubOutboundService constructs the outbound pusher.
func NewGithubOutboundService(gh githubIssueClient, portalBaseURL string) GithubOutboundService {
	return &githubOutboundService{gh: gh, portalBaseURL: strings.TrimRight(portalBaseURL, "/")}
}

// Deliver implements GithubOutboundService.
func (s *githubOutboundService) Deliver(ctx context.Context, item repository.OutboundItem) error {
	issue, err := github.ParseIssueURL(item.GitReference)
	if err != nil {
		// A reference we cannot parse will never become parseable. Surfacing
		// it as a permanent failure is more useful than retrying six times.
		return fmt.Errorf("%w: %v", ErrOutboundPermanent, err)
	}

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
		kind, _ := item.Payload["type"].(string)
		// A work note is internal by definition. Relaying it to a public issue
		// would disclose something written on the assumption it stayed inside.
		if strings.EqualFold(kind, "WORK_NOTE") {
			return "", nil
		}
		if strings.TrimSpace(content) == "" {
			return "", nil
		}
		return fmt.Sprintf("**%s** commented on %s:\n\n%s",
			displayAuthor(author), s.changeRequestLink(item.ChangeRequestID), content), nil

	case outboundCRCreated:
		return fmt.Sprintf("A change request has been raised for this issue: %s",
			s.changeRequestLink(item.ChangeRequestID)), nil

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
			s.changeRequestLink(item.ChangeRequestID), verb, strings.Join(lines, "\n")), nil
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
	if s.portalBaseURL == "" {
		return "the change request"
	}
	return fmt.Sprintf("[the change request](%s/operations/change-requests/%s)", s.portalBaseURL, id)
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
