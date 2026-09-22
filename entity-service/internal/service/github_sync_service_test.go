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
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/github"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const ghIssueURL = "https://github.com/wso2/choreo/issues/42"

type fakeGhRepo struct {
	claimed    map[string]bool
	linked     map[string]string
	mappingErr error
	accountID  string
	linkErr    error
	mapping    *repository.RepoMapping
	cr         *repository.GithubChangeRequest
}

func (f *fakeGhRepo) RepoMapping(context.Context, string, string) (*repository.RepoMapping, error) {
	return f.mapping, f.mappingErr
}
func (f *fakeGhRepo) ChangeRequestByGitReference(context.Context, string) (*repository.GithubChangeRequest, error) {
	return f.cr, nil
}

// Behaves like the table it stands in for: a second claim on the same id is
// refused. A fake that always returned nil is why the missing claim call went
// unnoticed.
func (f *fakeGhRepo) RepoForAccount(context.Context, string) (*repository.RepoMapping, error) {
	return f.mapping, f.mappingErr
}
func (f *fakeGhRepo) AccountForCase(context.Context, string) (string, error) {
	return f.accountID, nil
}
func (f *fakeGhRepo) SetCaseGithubIssueNumber(_ context.Context, caseID string, n int) (bool, error) {
	if f.linkErr != nil {
		return false, f.linkErr
	}
	if f.linked == nil {
		f.linked = map[string]string{}
	}
	f.linked[caseID] = fmt.Sprint(n)
	return true, nil
}

func (f *fakeGhRepo) ClaimDelivery(_ context.Context, id, _, _ string) error {
	if f.claimed == nil {
		f.claimed = map[string]bool{}
	}
	if f.claimed[id] {
		return repository.ErrDeliverySeen
	}
	f.claimed[id] = true
	return nil
}

func (f *fakeGhRepo) ReleaseDelivery(_ context.Context, id string) error {
	delete(f.claimed, id)
	return nil
}

func (f *fakeGhRepo) LinkDelivery(_ context.Context, id, crID string) error {
	if f.linked == nil {
		f.linked = map[string]string{}
	}
	f.linked[id] = crID
	return nil
}

type fakeGhClient struct {
	comments []string
	states   []github.State
	labels   [][]string
	removed  []string
	added    []string
}

func (f *fakeGhClient) CreateComment(_ context.Context, _ github.Issue, body string) (*github.Comment, error) {
	f.comments = append(f.comments, body)
	return &github.Comment{}, nil
}
func (f *fakeGhClient) SetLabels(_ context.Context, _ github.Issue, l []string) error {
	f.labels = append(f.labels, l)
	return nil
}
func (f *fakeGhClient) AddLabel(_ context.Context, _ github.Issue, l string) error {
	f.added = append(f.added, l)
	return nil
}
func (f *fakeGhClient) RemoveLabel(_ context.Context, _ github.Issue, l string) error {
	f.removed = append(f.removed, l)
	return nil
}
func (f *fakeGhClient) SetState(_ context.Context, _ github.Issue, s github.State) error {
	f.states = append(f.states, s)
	return nil
}

// fakeGhMutations records what the sync would have written.
type fakeGhMutations struct {
	created     int
	lastCreate  repository.NewChangeRequestFromIssue
	lastUpdate  repository.NewChangeRequestFromIssue
	lastComment string
}

func (f *fakeGhMutations) CreateFromIssue(_ context.Context, in repository.NewChangeRequestFromIssue) (string, string, error) {
	f.created++
	f.lastCreate = in
	return "cr-new", "CHG-GH-000001", nil
}
func (f *fakeGhMutations) UpdateFromIssue(_ context.Context, _ string, in repository.NewChangeRequestFromIssue) error {
	f.lastUpdate = in
	return nil
}
func (f *fakeGhMutations) SetState(context.Context, string, string) (bool, error) { return true, nil }
func (f *fakeGhMutations) AddComment(_ context.Context, _, content, _ string) error {
	f.lastComment = content
	return nil
}
func (f *fakeGhMutations) SetAssignee(context.Context, string, string) (bool, error) {
	return true, nil
}
func (f *fakeGhMutations) UserIDForGithubLogin(context.Context, string) (string, error) {
	return "", nil
}

// ghDelivery builds one webhook. The title carries the [CR]: prefix because
// that -- not a label -- is what marks an issue as a change request.
func ghDelivery(event, action, title string, labels []string) Delivery {
	var p IssuePayload
	p.Action = action
	p.Issue.HTMLURL = ghIssueURL
	p.Issue.Number = 42
	p.Issue.Title = title
	p.Issue.Body = "Certificates expire on the 30th."
	p.Repository.Name = "choreo"
	p.Repository.Owner.Login = "wso2"
	p.Sender.Login = "a-human"
	for _, l := range labels {
		p.Issue.Labels = append(p.Issue.Labels, struct {
			Name string `json:"name"`
		}{Name: l})
	}
	return Delivery{ID: "d1", Event: event, Payload: p}
}

// commentDelivery builds an issue_comment webhook with the comment object
// actually present -- it is a pointer on the payload, so a test that only sets
// fields on it dereferences nil.
func commentDelivery(body, author string) Delivery {
	d := ghDelivery("issue_comment", "created", "[CR]: x", []string{"CR/NormalChange"})
	d.Payload.Comment = &struct {
		Body    string      `json:"body"`
		HTMLURL string      `json:"html_url"`
		User    github.User `json:"user"`
	}{Body: body}
	d.Payload.Comment.User.Login = author
	return d
}

func crDelivery() Delivery {
	return ghDelivery("issues", "labeled", "[CR]: rotate gateway certificates",
		[]string{"CR/NormalChange"})
}

func newGhSvc(r *fakeGhRepo, c *fakeGhClient) GithubSyncService {
	return NewGithubSyncService(r, c, "wso2-integration-bot")
}

func writingSvc(r *fakeGhRepo, m *fakeGhMutations, c *fakeGhClient) GithubSyncService {
	return NewGithubSyncServiceWriting(r, m, c, "wso2-integration-bot", DefaultGithubLabels())
}

func mapped() *repository.RepoMapping {
	return &repository.RepoMapping{
		AccountID: "a1", AccountName: "Choreo Customer",
		CredentialRef: "gh-choreo", Owner: "wso2", Repository: "choreo",
	}
}

// RECOGNITION IS BY TITLE. issue_servicenow.yml: "Change requests carry no
// template label -- the [CR]:/[ECR]: title is the only signal." An earlier
// version of this service gated on a Type/ChangeRequest label that nothing in
// any product repository applies, so it would never have matched a real issue.
func TestHandleWebhook_RecognisesByTitlePrefix(t *testing.T) {
	cases := map[string]struct {
		title   string
		labels  []string
		creates bool
	}{
		"[CR]: is a change request":     {"[CR]: rotate certs", []string{"CR/NormalChange"}, true},
		"[ECR]: is one too":             {"[ECR]: restore the gateway", []string{"CR/EmergencyChange"}, true},
		"no prefix is not":              {"rotate certs", []string{"CR/NormalChange"}, false},
		"an incident is not":            {"gateway down", []string{"Type/Incident"}, false},
		"a service request is not":      {"please add a user", []string{"Type/ServiceRequest"}, false},
		"prefix without a class waits":  {"[CR]: rotate certs", nil, false},
		"prefix with two classes waits": {"[CR]: rotate certs", []string{"CR/NormalChange", "CR/StandardChange"}, false},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			r := &fakeGhRepo{mapping: mapped()}
			m := &fakeGhMutations{}
			_, err := writingSvc(r, m, &fakeGhClient{}).
				HandleWebhook(context.Background(), ghDelivery("issues", "labeled", c.title, c.labels))
			if err != nil {
				t.Fatalf("HandleWebhook: %v", err)
			}
			if got := m.created > 0; got != c.creates {
				t.Errorf("created = %v, want %v", got, c.creates)
			}
		})
	}
}

// The class label decides the change request's type.
func TestHandleWebhook_ClassLabelSetsTheType(t *testing.T) {
	for label, want := range map[string]string{
		"CR/NormalChange":    "Normal Change",
		"CR/StandardChange":  "Standard Change",
		"CR/EmergencyChange": "Emergency Change",
	} {
		t.Run(label, func(t *testing.T) {
			r := &fakeGhRepo{mapping: mapped()}
			m := &fakeGhMutations{}
			_, err := writingSvc(r, m, &fakeGhClient{}).HandleWebhook(context.Background(),
				ghDelivery("issues", "labeled", "[CR]: x", []string{label}))
			if err != nil {
				t.Fatalf("HandleWebhook: %v", err)
			}
			if m.lastCreate.Type != want {
				t.Errorf("type = %q, want %q", m.lastCreate.Type, want)
			}
		})
	}
}

// An unmapped repository is not ours. The mapping table is the allow-list.
func TestHandleWebhook_UnmappedRepositoryIsIgnored(t *testing.T) {
	r := &fakeGhRepo{mapping: nil}
	m := &fakeGhMutations{}
	out, err := writingSvc(r, m, &fakeGhClient{}).HandleWebhook(context.Background(), crDelivery())
	if err != nil {
		t.Fatalf("HandleWebhook: %v", err)
	}
	if out.Skipped == "" || m.created != 0 {
		t.Errorf("acted on an unmapped repository: %+v", out)
	}
}

// Our own writes come back as webhooks; dropping them by sender identity is
// what stops a comment we posted syncing back as a new one.
func TestHandleWebhook_OwnEventsAreDropped(t *testing.T) {
	d := crDelivery()
	d.Payload.Sender.Login = "wso2-integration-bot"
	r := &fakeGhRepo{mapping: mapped()}
	m := &fakeGhMutations{}
	out, err := writingSvc(r, m, &fakeGhClient{}).HandleWebhook(context.Background(), d)
	if err != nil {
		t.Fatalf("HandleWebhook: %v", err)
	}
	if out.Skipped == "" || m.created != 0 {
		t.Errorf("acted on our own event: %+v", out)
	}
}

func TestHandleWebhook_ReplayedDeliveryIsRefused(t *testing.T) {
	r := &fakeGhRepo{mapping: mapped()}
	svc := writingSvc(r, &fakeGhMutations{}, &fakeGhClient{})
	if _, err := svc.HandleWebhook(context.Background(), crDelivery()); err != nil {
		t.Fatalf("first delivery: %v", err)
	}
	if _, err := svc.HandleWebhook(context.Background(), crDelivery()); !errors.Is(err, repository.ErrDeliverySeen) {
		t.Fatalf("replay was processed again; err = %v", err)
	}
}

func TestHandleWebhook_FailedDeliveryReleasesItsClaim(t *testing.T) {
	r := &fakeGhRepo{mappingErr: errors.New("database is down")}
	d := crDelivery()
	if _, err := writingSvc(r, &fakeGhMutations{}, &fakeGhClient{}).
		HandleWebhook(context.Background(), d); err == nil {
		t.Fatal("want the underlying failure")
	}
	if r.claimed[d.ID] {
		t.Error("the claim survived a failure, so GitHub's retry would be refused as a replay")
	}
}

// A comment is relayed with the same "(GitHub Comment)" marker
// github_comment_to_sn.yml uses, which is what sn_comment_to_github.yml checks
// before posting back. Same marker, same loop closed.
func TestHandleWebhook_CommentIsRelayedWithTheLoopMarker(t *testing.T) {
	r := &fakeGhRepo{mapping: mapped(), cr: &repository.GithubChangeRequest{ID: "cr-1"}}
	m := &fakeGhMutations{}
	d := commentDelivery("Scheduled for Friday.", "nimal")

	if _, err := writingSvc(r, m, &fakeGhClient{}).HandleWebhook(context.Background(), d); err != nil {
		t.Fatalf("HandleWebhook: %v", err)
	}
	if !strings.Contains(m.lastComment, "(GitHub Comment)") {
		t.Errorf("loop marker missing: %q", m.lastComment)
	}
	if !strings.Contains(m.lastComment, "Scheduled for Friday.") {
		t.Errorf("comment body lost: %q", m.lastComment)
	}
}

// Slash commands belong to the repository's own workflows.
func TestHandleWebhook_SlashCommandsAreNotRelayed(t *testing.T) {
	r := &fakeGhRepo{mapping: mapped(), cr: &repository.GithubChangeRequest{ID: "cr-1"}}
	m := &fakeGhMutations{}
	d := commentDelivery("/close", "nimal")
	if _, err := writingSvc(r, m, &fakeGhClient{}).HandleWebhook(context.Background(), d); err != nil {
		t.Fatalf("HandleWebhook: %v", err)
	}
	if m.lastComment != "" {
		t.Errorf("relayed a slash command: %q", m.lastComment)
	}
}
