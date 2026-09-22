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
	labelChangeRequest  = "Type/ChangeRequest"
	labelPrefixCRType   = "CRType/"
	labelPrefixCRScope  = "CRScope/"
	labelScopeApp       = "CRScope/Application"
	labelScopeInfra     = "CRScope/Infrastructure"
	labelImpactHigh     = "Impact 1"
	labelImpactMedium   = "Impact 2"
	labelLikelihoodHigh = "Likelihood 1"
	labelLikelihoodMed  = "Likelihood 2"
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
	// labels is the vocabulary this deployment recognises. Defaults to
	// ServiceNow's values; overridable because a repository's labels are a
	// deployment's business and dev need not match production.
	labels GithubLabels
	// mutate writes the change request. Nil leaves the sync read-only, which
	// is how it behaved before the mutation layer existed and is still useful
	// for a dry run against a live repository.
	mutate repository.GithubMutationRepository
	// integrationLogin is our own GitHub account. Events it sent are our own
	// writes coming back and are dropped -- identity, not string-matching the
	// comment body the way the case webhook does.
	integrationLogins []string
}

// githubIssueClient is the slice of *github.Client this service needs.
type githubIssueClient interface {
	CreateComment(ctx context.Context, issue github.Issue, body string) (*github.Comment, error)
	SetLabels(ctx context.Context, issue github.Issue, labels []string) error
	RemoveLabel(ctx context.Context, issue github.Issue, label string) error
	AddLabel(ctx context.Context, issue github.Issue, label string) error
	SetState(ctx context.Context, issue github.Issue, state github.State) error
}

// NewGithubSyncService constructs the webhook policy.
func NewGithubSyncService(repo repository.GithubSyncRepository, gh githubIssueClient, integrationLogin string) GithubSyncService {
	return NewGithubSyncServiceWithLabels(repo, gh, integrationLogin, DefaultGithubLabels())
}

// NewGithubSyncServiceWithLabels is NewGithubSyncService with an explicit
// label vocabulary.
func NewGithubSyncServiceWithLabels(repo repository.GithubSyncRepository, gh githubIssueClient, integrationLogin string, labels GithubLabels) GithubSyncService {
	return &githubSyncService{repo: repo, gh: gh, integrationLogins: integrationLoginSet(integrationLogin), labels: labels}
}

// WithMutations returns the service able to write change requests. Without it
// the sync recognises and reports but changes nothing.
func (s *githubSyncService) WithMutations(m repository.GithubMutationRepository) GithubSyncService {
	s.mutate = m
	return s
}

// NewGithubSyncServiceWriting is the full service: recognises, writes, and
// pushes the resulting label changes back to the issue.
func NewGithubSyncServiceWriting(repo repository.GithubSyncRepository, mutate repository.GithubMutationRepository, gh githubIssueClient, integrationLogin string, labels GithubLabels) GithubSyncService {
	return &githubSyncService{repo: repo, gh: gh, integrationLogins: integrationLoginSet(integrationLogin), labels: labels, mutate: mutate}
}

func skip(reason string) (Outcome, error) { return Outcome{Skipped: reason}, nil }

// HandleWebhook implements GithubSyncService.
//
// THE DELIVERY CLAIM WRAPS EVERY WRITE. GitHub retries any delivery it did not
// get a 2xx for, and an issue must not become two change requests.
func (s *githubSyncService) HandleWebhook(ctx context.Context, d Delivery) (Outcome, error) {
	p := d.Payload

	// The cheap guards first, outside the claim: they touch nothing, so
	// recording a delivery we are going to ignore protects nothing.
	//
	// Our own writes come back as webhooks. Dropping them by sender identity is
	// what stops a comment we posted being synced back as a new one.
	//
	// THE IDENTITY IS THE WORKFLOW'S, NOT OURS. We dispatch; a GitHub Actions
	// workflow does the posting, so the sender on the way back is
	// github-actions[bot] and never the integration account. Before the
	// dispatch rewrite this service posted directly and the two were the same,
	// which is why the default outlived its meaning: a CSM comment went out,
	// came back as a bot comment, and was written onto the case as a new one.
	//
	// The text marker below is the second line of defence, matching
	// sn_comment_to_github.yml's own guard. Identity alone should be enough;
	// the marker catches a repository whose workflow posts under some other
	// account.
	if s.isOwnEvent(p) {
		return skip("event was sent by the integration account")
	}
	if d.Event != "issues" && d.Event != "issue_comment" {
		return skip("event " + d.Event + " is not handled")
	}

	if err := s.repo.ClaimDelivery(ctx, d.ID, d.Event, p.Action); err != nil {
		// ErrDeliverySeen travels up to the handler, which answers 200 so
		// GitHub stops retrying something already applied.
		return Outcome{}, err
	}

	out, err := s.handleClaimed(ctx, d)
	if err != nil {
		if rerr := s.repo.ReleaseDelivery(ctx, d.ID); rerr != nil {
			slog.ErrorContext(ctx, "github: could not release delivery claim",
				"delivery", d.ID, "err", rerr)
		}
		return Outcome{}, err
	}

	if out.ChangeRequestID != "" {
		if lerr := s.repo.LinkDelivery(ctx, d.ID, out.ChangeRequestID); lerr != nil {
			slog.WarnContext(ctx, "github: could not link delivery to change request",
				"delivery", d.ID, "err", lerr)
		}
	}
	return out, nil
}

// handleClaimed is the body of HandleWebhook, run with the delivery claimed.
func (s *githubSyncService) handleClaimed(ctx context.Context, d Delivery) (Outcome, error) {
	p := d.Payload

	// An unmapped repository is not ours. The mapping table IS the allow-list,
	// so routing and permission cannot drift apart.
	mapping, err := s.repo.RepoMapping(ctx, p.Repository.Owner.Login, p.Repository.Name)
	if err != nil {
		return Outcome{}, err
	}
	if mapping == nil {
		return skip(fmt.Sprintf("repository %s/%s is not mapped to an account",
			p.Repository.Owner.Login, p.Repository.Name))
	}

	cr, err := s.repo.ChangeRequestByGitReference(ctx, p.Issue.HTMLURL)
	if err != nil {
		return Outcome{}, err
	}

	if d.Event == "issue_comment" {
		return s.handleComment(ctx, p, mapping, cr)
	}
	return s.handleIssue(ctx, p, cr)
}

// handleIssue creates or updates the change request an issue represents.
//
// RECOGNITION IS BY TITLE, NOT BY LABEL. issue_servicenow.yml says it plainly:
// "Change requests carry no template label -- the [CR]:/[ECR]: title is the
// only signal." An earlier version of this file gated on a Type/ChangeRequest
// label that nothing in any product repository applies.
func (s *githubSyncService) handleIssue(ctx context.Context, p IssuePayload, cr *repository.GithubChangeRequest) (Outcome, error) {
	if !IsChangeRequestTitle(p.Issue.Title) {
		return skip("issue title is not prefixed [CR]: or [ECR]:, so it is not a change request")
	}

	labels := p.LabelNames()

	// The class label is applied by the repository's validation workflow. Its
	// absence means validation has not passed yet, and acting early would
	// create a record from a half-filled template.
	class, ok := s.labels.ClassOf(labels)
	if !ok {
		return skip("exactly one CR class label is required; found none or several")
	}

	if s.mutate == nil {
		return Outcome{Action: "creation_prepared"}, nil
	}

	if cr == nil {
		id, number, err := s.mutate.CreateFromIssue(ctx, repository.NewChangeRequestFromIssue{
			Subject:      p.Issue.Title,
			Description:  p.Issue.Body,
			GitReference: p.Issue.HTMLURL,
			Type:         class,
		})
		if err != nil {
			return Outcome{}, err
		}
		slog.InfoContext(ctx, "github: change request created from issue",
			"changeRequestId", id, "number", number, "issue", p.Issue.Number)
		return Outcome{Action: "created", ChangeRequestID: id}, nil
	}

	// An edited issue overwrites the record's subject, description and class.
	// servicenow_update_case.yml does the same: the issue is the source of
	// truth for the text, which is why it re-sends all of it rather than a diff.
	if err := s.mutate.UpdateFromIssue(ctx, cr.ID, repository.NewChangeRequestFromIssue{
		Subject:      p.Issue.Title,
		Description:  p.Issue.Body,
		GitReference: p.Issue.HTMLURL,
		Type:         class,
	}); err != nil {
		return Outcome{}, err
	}
	return Outcome{Action: "updated", ChangeRequestID: cr.ID}, nil
}

// handleComment mirrors a GitHub comment onto the change request.
func (s *githubSyncService) handleComment(ctx context.Context, p IssuePayload, mapping *repository.RepoMapping, cr *repository.GithubChangeRequest) (Outcome, error) {
	if p.Action != "created" && p.Action != "edited" {
		return skip("comment action " + p.Action + " is not handled")
	}
	// Comment is a pointer: an issue_comment delivery that carries no comment
	// object is malformed, and dereferencing it would panic the handler rather
	// than answer GitHub.
	if p.Comment == nil {
		return skip("issue_comment delivery carried no comment")
	}
	// THE COMMENT GOES ON THE CASE, NOT THE CHANGE REQUEST.
	//
	// github_comment_to_sn.yml PATCHes sn_customerservice_case, and the
	// outbound trigger only watches case comments. Attaching a relayed comment
	// to the change request instead produced a conversation that could not be
	// answered: a reply on the change request synced nowhere, and a reply on
	// the case reached GitHub detached from the thread that started it.
	caseID, err := s.repo.CaseByIssueNumber(ctx, mapping.AccountID, p.Issue.Number)
	if err != nil {
		return Outcome{}, err
	}
	if caseID == "" {
		return skip("no case is linked to this issue")
	}
	body := strings.TrimSpace(p.Comment.Body)
	if body == "" {
		return skip("comment is empty")
	}
	// A command, not a note. The repository's own workflows own these.
	if strings.HasPrefix(body, "/") {
		return skip("comment is a slash command")
	}
	if s.mutate == nil {
		return Outcome{Action: "comment_pending_write"}, nil
	}

	// The "(GitHub Comment)" marker is what github_comment_to_sn.yml stamps on
	// what it relays, and what sn_comment_to_github.yml checks for before
	// posting back. Keeping the same marker keeps the same loop closed.
	author := p.Comment.User.Login
	if author == "" {
		author = "a GitHub user"
	}
	text := fmt.Sprintf("@%s (GitHub Comment) %s\n\n%s", author, p.Issue.HTMLURL, body)

	if err := s.mutate.AddComment(ctx, caseID, text, author); err != nil {
		return Outcome{}, err
	}
	// The change request id, when there is one, is only for the delivery log.
	var crID string
	if cr != nil {
		crID = cr.ID
	}
	return Outcome{Action: "comment_relayed", ChangeRequestID: crID}, nil
}

// isOwnEvent reports whether this delivery is an echo of something we caused.
func (s *githubSyncService) isOwnEvent(p IssuePayload) bool {
	for _, login := range s.integrationLogins {
		if login != "" && strings.EqualFold(p.Sender.Login, login) {
			return true
		}
	}
	// A comment we relayed carries this marker, and so does one the workflow
	// posted on our behalf. Either way it originated here.
	if p.Comment != nil && strings.Contains(p.Comment.Body, "(GitHub Comment)") {
		return true
	}
	return false
}

// integrationLoginSet is every account whose events are our own coming back.
//
// github-actions[bot] is always included: with the dispatch architecture it is
// the account that actually posts, whatever the configured integration login
// is. Leaving it to configuration would make a loop the default.
func integrationLoginSet(configured string) []string {
	out := []string{"github-actions[bot]"}
	if c := strings.TrimSpace(configured); c != "" {
		out = append(out, c)
	}
	return out
}
