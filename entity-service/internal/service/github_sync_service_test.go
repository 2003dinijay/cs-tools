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
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/github"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const ghIssueURL = "https://github.com/wso2/choreo/issues/42"

type fakeGhRepo struct {
	mapping *repository.RepoMapping
	cr      *repository.GithubChangeRequest
}

func (f *fakeGhRepo) RepoMapping(context.Context, string, string) (*repository.RepoMapping, error) {
	return f.mapping, nil
}
func (f *fakeGhRepo) ChangeRequestByGitReference(context.Context, string) (*repository.GithubChangeRequest, error) {
	return f.cr, nil
}
func (f *fakeGhRepo) ClaimDelivery(context.Context, string, string, string) error { return nil }
func (f *fakeGhRepo) ReleaseDelivery(context.Context, string) error               { return nil }
func (f *fakeGhRepo) LinkDelivery(context.Context, string, string) error          { return nil }

type fakeGhClient struct {
	comments []string
	states   []github.State
	labels   [][]string
	removed  []string
}

func (f *fakeGhClient) CreateComment(_ context.Context, _ github.Issue, body string) (*github.Comment, error) {
	f.comments = append(f.comments, body)
	return &github.Comment{}, nil
}
func (f *fakeGhClient) SetLabels(_ context.Context, _ github.Issue, l []string) error {
	f.labels = append(f.labels, l)
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

func ghDelivery(event, action string, labels []string) Delivery {
	var p IssuePayload
	p.Action = action
	p.Issue.HTMLURL = ghIssueURL
	p.Issue.Number = 42
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

func crLabels() []string {
	return []string{labelChangeRequest, "CRType/Normal", labelScopeApp}
}

func newGhSvc(r *fakeGhRepo, c *fakeGhClient) GithubSyncService {
	return NewGithubSyncService(r, c, "wso2-integration-bot")
}

func mapped() *repository.RepoMapping {
	return &repository.RepoMapping{ProductID: "p1", ProductName: "Choreo", TeamID: "t1"}
}

// Our own writes come back as webhooks; dropping them by sender identity is
// what stops an endless loop.
func TestGithubSync_DropsOwnEvents(t *testing.T) {
	d := ghDelivery("issues", "labeled", crLabels())
	d.Payload.Sender.Login = "WSO2-Integration-Bot" // case-insensitive
	got, err := newGhSvc(&fakeGhRepo{mapping: mapped()}, &fakeGhClient{}).HandleWebhook(context.Background(), d)
	if err != nil {
		t.Fatalf("HandleWebhook: %v", err)
	}
	if !strings.Contains(got.Skipped, "integration account") {
		t.Fatalf("skipped = %q, want the integration-account reason", got.Skipped)
	}
}

// The mapping table is the allow-list. An unmapped repository is not an error.
func TestGithubSync_UnmappedRepositoryIsSkipped(t *testing.T) {
	got, err := newGhSvc(&fakeGhRepo{mapping: nil}, &fakeGhClient{}).
		HandleWebhook(context.Background(), ghDelivery("issues", "labeled", crLabels()))
	if err != nil {
		t.Fatalf("HandleWebhook: %v", err)
	}
	if !strings.Contains(got.Skipped, "not mapped") {
		t.Fatalf("skipped = %q", got.Skipped)
	}
}

// A change request is created by labelling, never by opening an issue.
func TestGithubSync_CreationRequiresLabelling(t *testing.T) {
	for _, action := range []string{"opened", "edited"} {
		t.Run(action, func(t *testing.T) {
			got, _ := newGhSvc(&fakeGhRepo{mapping: mapped(), cr: nil}, &fakeGhClient{}).
				HandleWebhook(context.Background(), ghDelivery("issues", action, crLabels()))
			if got.Action != "" {
				t.Fatalf("action = %q, want no action", got.Action)
			}
		})
	}
	got, _ := newGhSvc(&fakeGhRepo{mapping: mapped(), cr: nil}, &fakeGhClient{}).
		HandleWebhook(context.Background(), ghDelivery("issues", "labeled", crLabels()))
	if got.Action != "would_create" {
		t.Fatalf("action = %q, want would_create", got.Action)
	}
}

// The full label set gates everything.
func TestGithubSync_LabelGate(t *testing.T) {
	cases := map[string]struct {
		labels []string
		want   bool
	}{
		"complete":        {crLabels(), true},
		"no type label":   {[]string{"CRType/Normal", labelScopeApp}, false},
		"no CRType":       {[]string{labelChangeRequest, labelScopeApp}, false},
		"no scope":        {[]string{labelChangeRequest, "CRType/Normal"}, false},
		"two scopes":      {[]string{labelChangeRequest, "CRType/Normal", labelScopeApp, labelScopeInfra}, false},
		"unrelated extra": {append(crLabels(), "bug"), true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := hasChangeRequestLabels(tc.labels); got != tc.want {
				t.Fatalf("hasChangeRequestLabels(%v) = %v, want %v", tc.labels, got, tc.want)
			}
		})
	}
}

// THE GUARD WORTH KEEPING: closing is only allowed from Review. Anything else
// reopens the issue and explains why.
func TestGithubSync_CloseRefusedOutsideReview(t *testing.T) {
	client := &fakeGhClient{}
	repo := &fakeGhRepo{mapping: mapped(), cr: &repository.GithubChangeRequest{ID: "cr1", Number: "CHG1", State: "ASSESS"}}

	got, err := newGhSvc(repo, client).HandleWebhook(context.Background(), ghDelivery("issues", "closed", crLabels()))
	if err != nil {
		t.Fatalf("HandleWebhook: %v", err)
	}
	if got.Action != "close_refused" {
		t.Fatalf("action = %q, want close_refused", got.Action)
	}
	if len(client.states) != 1 || client.states[0] != github.StateOpen {
		t.Fatalf("issue was not reopened: %v", client.states)
	}
	if len(client.comments) != 1 || !strings.Contains(client.comments[0], "ASSESS") {
		t.Fatalf("comment did not name the state: %v", client.comments)
	}
}

func TestGithubSync_CloseAllowedFromReview(t *testing.T) {
	client := &fakeGhClient{}
	repo := &fakeGhRepo{mapping: mapped(), cr: &repository.GithubChangeRequest{ID: "cr1", State: stateReview}}

	got, err := newGhSvc(repo, client).HandleWebhook(context.Background(), ghDelivery("issues", "closed", crLabels()))
	if err != nil {
		t.Fatalf("HandleWebhook: %v", err)
	}
	if got.Action != "closed" {
		t.Fatalf("action = %q, want closed", got.Action)
	}
	if len(client.states) != 0 {
		t.Fatalf("issue should not have been reopened: %v", client.states)
	}
}

// Comments mirror only on creation, and only for a linked issue.
func TestGithubSync_Comments(t *testing.T) {
	repo := &fakeGhRepo{mapping: mapped(), cr: &repository.GithubChangeRequest{ID: "cr1"}}
	d := ghDelivery("issue_comment", "created", crLabels())
	d.Payload.Comment = &struct {
		Body    string      `json:"body"`
		HTMLURL string      `json:"html_url"`
		User    github.User `json:"user"`
	}{Body: "hello"}

	got, _ := newGhSvc(repo, &fakeGhClient{}).HandleWebhook(context.Background(), d)
	if got.Action != "comment_mirrored" {
		t.Fatalf("action = %q, want comment_mirrored", got.Action)
	}

	edited := ghDelivery("issue_comment", "edited", crLabels())
	got, _ = newGhSvc(repo, &fakeGhClient{}).HandleWebhook(context.Background(), edited)
	if got.Action != "" {
		t.Fatalf("edited comment should not mirror, got %q", got.Action)
	}

	unlinked := &fakeGhRepo{mapping: mapped(), cr: nil}
	got, _ = newGhSvc(unlinked, &fakeGhClient{}).HandleWebhook(context.Background(), d)
	if got.Action != "" {
		t.Fatalf("unlinked issue should not mirror, got %q", got.Action)
	}
}

// State comes from the label's name, not from ServiceNow's numeric codes --
// whose own table disagreed with its own constants.
func TestGithubSync_StateByLabelName(t *testing.T) {
	want := map[string]string{
		"Assessed": "ASSESS", "Authorized": "AUTHORIZE", "Scheduled": "SCHEDULED",
		"Implemented": "IMPLEMENT", "Reviewed": "REVIEW",
	}
	for label, state := range want {
		got, ok := githubStateForLabel(label)
		if !ok || got != state {
			t.Errorf("githubStateForLabel(%q) = %q,%v want %q", label, got, ok, state)
		}
	}
	// Closed and Canceled must not be label-driven: closing has a guard that a
	// label would bypass.
	for _, label := range []string{"Closed", "Canceled"} {
		if _, ok := githubStateForLabel(label); ok {
			t.Errorf("%q should not be label-driven", label)
		}
	}
}

func TestGithubAttributes(t *testing.T) {
	impact, likelihood, crType := githubAttributes([]string{
		labelChangeRequest, "CRType/Normal", labelScopeInfra, labelImpactHigh, labelLikelihoodMed,
	})
	if impact != "HIGH" {
		t.Errorf("impact = %q, want HIGH", impact)
	}
	if likelihood != "MEDIUM" {
		t.Errorf("likelihood = %q, want MEDIUM", likelihood)
	}
	if crType != "INFRA" {
		t.Errorf("crType = %q, want INFRA", crType)
	}

	// Absent labels leave fields empty rather than defaulting to the lowest
	// value, which would be indistinguishable from a deliberate choice.
	impact, likelihood, crType = githubAttributes([]string{labelChangeRequest, "CRType/Normal", labelScopeApp})
	if impact != "" || likelihood != "" {
		t.Errorf("absent labels defaulted: impact=%q likelihood=%q", impact, likelihood)
	}
	if crType != "GENERAL" {
		t.Errorf("crType = %q, want GENERAL", crType)
	}
}

func TestGithubSync_UnhandledEvents(t *testing.T) {
	for _, event := range []string{"push", "pull_request", "star"} {
		got, err := newGhSvc(&fakeGhRepo{mapping: mapped()}, &fakeGhClient{}).
			HandleWebhook(context.Background(), ghDelivery(event, "created", crLabels()))
		if err != nil {
			t.Fatalf("HandleWebhook(%s): %v", event, err)
		}
		if got.Skipped == "" {
			t.Fatalf("event %s should have been skipped", event)
		}
	}
}
