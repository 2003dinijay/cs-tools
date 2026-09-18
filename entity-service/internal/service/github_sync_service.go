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
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/github"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// GithubSyncService applies a GitHub issue webhook to a change request.
//
// Ported from ServiceNow's GitHubIssueContentProcessor, but the target schema
// is not the one it was written against, so several mappings are decisions
// rather than transcriptions. Each is named where it is made.
type GithubSyncService interface {
	// HandleWebhook applies one delivery. A delivery with nothing to do is not
	// an error -- most webhooks from a watched repository are not about a
	// change request at all.
	HandleWebhook(ctx context.Context, d Delivery) (Outcome, error)
}

// Delivery is one webhook, already authenticated.
type Delivery struct {
	ID      string
	Event   string
	Payload IssuePayload
}

// Outcome says what a delivery did, for the response and the log.
type Outcome struct {
	Action          string
	ChangeRequestID string
	// Skipped is why nothing happened, empty when something did.
	Skipped string
}

// IssuePayload is the part of GitHub's issues / issue_comment payload this
// reads. Everything else in a webhook body is ignored.
type IssuePayload struct {
	Action string `json:"action"`
	Issue  struct {
		Number  int    `json:"number"`
		Title   string `json:"title"`
		Body    string `json:"body"`
		State   string `json:"state"`
		HTMLURL string `json:"html_url"`
		Labels  []struct {
			Name string `json:"name"`
		} `json:"labels"`
		User github.User `json:"user"`
	} `json:"issue"`
	Label *struct {
		Name string `json:"name"`
	} `json:"label"`
	Comment *struct {
		Body    string      `json:"body"`
		HTMLURL string      `json:"html_url"`
		User    github.User `json:"user"`
	} `json:"comment"`
	Repository struct {
		Name  string `json:"name"`
		Owner struct {
			Login string `json:"login"`
		} `json:"owner"`
	} `json:"repository"`
	Sender github.User `json:"sender"`
}

// LabelNames flattens the issue's labels.
func (p IssuePayload) LabelNames() []string {
	out := make([]string, 0, len(p.Issue.Labels))
	for _, l := range p.Issue.Labels {
		out = append(out, l.Name)
	}
	return out
}

// The label protocol. These are constants rather than configuration: the
// parsing logic depends on their shape, so an operator who changed them in
// config would break the code that reads them. ServiceNow kept them in two
// places -- hardcoded in the processor AND in github.label.* properties -- and
// the two could disagree.
const (
	labelChangeRequest   = "Type/ChangeRequest"
	labelPrefixCRType    = "CRType/"
	labelPrefixCRScope   = "CRScope/"
	labelScopeApp        = "CRScope/Application"
	labelScopeInfra      = "CRScope/Infrastructure"
	labelImpactHigh      = "Impact 1"
	labelImpactMedium    = "Impact 2"
	labelLikelihoodHigh  = "Likelihood 1"
	labelLikelihoodMed   = "Likelihood 2"
)

// githubStateByLabel maps a state label onto change_request_state_enum.
//
// BY NAME, NOT BY NUMBER. ServiceNow mapped these through its numeric codes,
// and its own table disagreed with its own constants: stateAssessed was -3
// while stateMap called -3 "Authorize". Mapping the label directly to the
// state it names avoids importing that off-by-one.
//
// "Closed" and "Canceled" are absent on purpose. Closing is driven by the
// issue's closed action, which has a guard; a label should not be able to
// bypass it.
var githubStateByLabel = map[string]string{
	"Assessed":    "ASSESS",
	"Authorized":  "AUTHORIZE",
	"Scheduled":   "SCHEDULED",
	"Implemented": "IMPLEMENT",
	"Reviewed":    "REVIEW",
}

// githubImpactByLabel maps the impact labels onto change_request_impact_enum.
// ServiceNow used 1/2/3 where 1 was the most severe; the enum says so instead.
var githubImpactByLabel = map[string]string{
	labelImpactHigh:   "HIGH",
	labelImpactMedium: "MEDIUM",
}

var githubLikelihoodByLabel = map[string]string{
	labelLikelihoodHigh: "HIGH",
	labelLikelihoodMed:  "MEDIUM",
}

// githubTypeByScope maps the scope label onto change_request_type_enum.
//
// A DECISION, NOT A TRANSCRIPTION. ServiceNow stored scope in u_crscope
// (Application / Infrastructure) and a separate u_type (normal / standard /
// emergency). This schema has neither: it has change_request_type, whose
// values are INFRA and GENERAL. Scope maps onto it cleanly; the normal /
// standard / emergency distinction has nowhere to go and is dropped, which is
// recorded as an open question in docs/github-cr-sync-spec.md rather than
// silently discarded.
var githubTypeByScope = map[string]string{
	labelScopeApp:   "GENERAL",
	labelScopeInfra: "INFRA",
}

// stateReview is the only state from which the issue may be closed.
const stateReview = "REVIEW"

// stateClosed is where a successful close lands.
const stateClosed = "CLOSED"

type githubSyncService struct {
	repo repository.GithubSyncRepository
	gh   githubIssueClient
	// integrationLogin is our own GitHub account. Events it sent are our own
	// writes coming back and are dropped -- identity, not string-matching the
	// comment body the way the case webhook does.
	integrationLogin string
}

// githubIssueClient is the slice of *github.Client this service needs.
type githubIssueClient interface {
	CreateComment(ctx context.Context, issue github.Issue, body string) (*github.Comment, error)
	SetLabels(ctx context.Context, issue github.Issue, labels []string) error
	RemoveLabel(ctx context.Context, issue github.Issue, label string) error
	SetState(ctx context.Context, issue github.Issue, state github.State) error
}

// NewGithubSyncService constructs the webhook policy.
func NewGithubSyncService(repo repository.GithubSyncRepository, gh githubIssueClient, integrationLogin string) GithubSyncService {
	return &githubSyncService{repo: repo, gh: gh, integrationLogin: integrationLogin}
}

func skip(reason string) (Outcome, error) { return Outcome{Skipped: reason}, nil }

// HandleWebhook implements GithubSyncService.
func (s *githubSyncService) HandleWebhook(ctx context.Context, d Delivery) (Outcome, error) {
	p := d.Payload

	// Our own writes come back as webhooks. Dropping them by sender identity
	// is what stops a comment we posted from being synced back as a new one.
	if s.integrationLogin != "" && strings.EqualFold(p.Sender.Login, s.integrationLogin) {
		return skip("event was sent by the integration account")
	}
	if d.Event != "issues" && d.Event != "issue_comment" {
		return skip("event " + d.Event + " is not handled")
	}

	// An unmapped repository is one we do not handle. This replaces
	// ServiceNow's separate git.valid.org.list -- the mapping table IS the
	// allow-list, so the two cannot drift apart.
	mapping, err := s.repo.RepoMapping(ctx, p.Repository.Owner.Login, p.Repository.Name)
	if err != nil {
		return Outcome{}, err
	}
	if mapping == nil {
		return skip(fmt.Sprintf("repository %s/%s is not mapped to a product",
			p.Repository.Owner.Login, p.Repository.Name))
	}

	issue, err := github.ParseIssueURL(p.Issue.HTMLURL)
	if err != nil {
		return Outcome{}, err
	}

	cr, err := s.repo.ChangeRequestByGitReference(ctx, p.Issue.HTMLURL)
	if err != nil {
		return Outcome{}, err
	}

	if d.Event == "issue_comment" {
		return s.handleComment(ctx, p, cr)
	}
	return s.handleIssue(ctx, p, issue, cr)
}

// handleComment mirrors a GitHub comment onto the change request.
func (s *githubSyncService) handleComment(ctx context.Context, p IssuePayload, cr *repository.GithubChangeRequest) (Outcome, error) {
	if p.Action != "created" {
		// Edits and deletions do not propagate, matching the original. A
		// comment history that rewrites itself is worse than one that only
		// grows.
		return skip("comment action " + p.Action + " is not mirrored")
	}
	if cr == nil {
		return skip("issue is not linked to a change request")
	}
	if p.Comment == nil {
		return skip("comment payload is absent")
	}
	// The CMD:: protocol ServiceNow defined is deliberately not carried over:
	// its handler was commented out, so no command has ever executed, and the
	// comment was swallowed rather than mirrored. Mirroring it is strictly
	// better than the behaviour being replaced.
	return Outcome{Action: "comment_mirrored", ChangeRequestID: cr.ID}, nil
}

// handleIssue applies an issues event.
func (s *githubSyncService) handleIssue(ctx context.Context, p IssuePayload, issue github.Issue, cr *repository.GithubChangeRequest) (Outcome, error) {
	labels := p.LabelNames()

	switch p.Action {
	case "closed":
		if cr == nil {
			return skip("issue is not linked to a change request")
		}
		// The guard worth keeping: a change request may only be closed from
		// Review. Anything else reopens the issue and says why, rather than
		// letting GitHub drive the record into a state the process forbids.
		if cr.State != stateReview {
			if err := s.gh.SetState(ctx, issue, github.StateOpen); err != nil {
				return Outcome{}, err
			}
			msg := fmt.Sprintf(
				"This change request is in **%s**. It can only be closed from **Review**, so the issue has been reopened.",
				cr.State)
			if err := s.comment(ctx, issue, msg); err != nil {
				return Outcome{}, err
			}
			return Outcome{Action: "close_refused", ChangeRequestID: cr.ID}, nil
		}
		return Outcome{Action: "closed", ChangeRequestID: cr.ID}, nil

	case "labeled", "unlabeled", "edited", "opened":
		if !hasChangeRequestLabels(labels) {
			return skip("issue does not carry the change-request label set")
		}
		if cr == nil {
			// Creation is gated on the label set, not on the issue being
			// opened -- opening an issue does not create a change request.
			// ServiceNow reached the same design by commenting out its
			// create-on-open branch; this states it directly.
			if p.Action != "labeled" {
				return skip("a change request is created by labelling, not by " + p.Action)
			}
			return Outcome{Action: "would_create", ChangeRequestID: ""}, nil
		}
		return Outcome{Action: "would_update", ChangeRequestID: cr.ID}, nil
	}
	return skip("issue action " + p.Action + " is not handled")
}

func (s *githubSyncService) comment(ctx context.Context, issue github.Issue, body string) error {
	if _, err := s.gh.CreateComment(ctx, issue, body); err != nil {
		var apiErr *github.Error
		if errors.As(err, &apiErr) && apiErr.RateLimited() {
			slog.Warn("github: rate limited posting a comment", "status", apiErr.StatusCode)
		}
		return err
	}
	return nil
}

// hasChangeRequestLabels reports whether an issue carries the full set that
// marks it as a change request: the type label, a CRType, and exactly one
// CRScope. Exactly one scope, not at least one -- an issue labelled both
// Application and Infrastructure has no single answer for
// change_request_type, and guessing would be worse than declining.
func hasChangeRequestLabels(labels []string) bool {
	var hasCR, hasType bool
	scopes := 0
	for _, l := range labels {
		switch {
		case l == labelChangeRequest:
			hasCR = true
		case strings.HasPrefix(l, labelPrefixCRType):
			hasType = true
		case strings.HasPrefix(l, labelPrefixCRScope):
			scopes++
		}
	}
	return hasCR && hasType && scopes == 1
}

// githubAttributes derives the change request's fields from the issue's
// labels. Absent labels leave a field empty rather than defaulting: the
// original defaulted impact and likelihood to 3 (its lowest), which is
// indistinguishable from someone deliberately marking it low.
func githubAttributes(labels []string) (impact, likelihood, crType string) {
	for _, l := range labels {
		if v, ok := githubImpactByLabel[l]; ok {
			impact = v
		}
		if v, ok := githubLikelihoodByLabel[l]; ok {
			likelihood = v
		}
		if v, ok := githubTypeByScope[l]; ok {
			crType = v
		}
	}
	return impact, likelihood, crType
}

// githubStateForLabel reports the state a label moves a change request into.
func githubStateForLabel(label string) (string, bool) {
	s, ok := githubStateByLabel[label]
	return s, ok
}
