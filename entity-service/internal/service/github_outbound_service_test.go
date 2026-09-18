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
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/github"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const obOwner, obRepo, obIssue = "wso2", "choreo", 42

func obItem(event string, payload map[string]any) repository.OutboundItem {
	return repository.OutboundItem{
		ID: 1, Event: event, WorkItemID: "cr-1",
		Owner: obOwner, Repository: obRepo, IssueNumber: obIssue, Payload: payload,
	}
}

func obSvc(c *fakeGhClient) GithubOutboundService {
	return NewGithubOutboundService(c, "https://csm.example", DefaultCommentSkipAuthors())
}

func TestOutbound_CommentIsRelayed(t *testing.T) {
	c := &fakeGhClient{}
	err := obSvc(c).Deliver(context.Background(), obItem(outboundCommentAdded, map[string]any{
		"content": "we have scheduled this for Friday", "createdBy": "nimal@wso2.com", "type": "COMMENT",
	}))
	if err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	if len(c.comments) != 1 {
		t.Fatalf("posted %d comments, want 1", len(c.comments))
	}
	if !strings.Contains(c.comments[0], "we have scheduled this for Friday") {
		t.Errorf("body missing the comment: %q", c.comments[0])
	}
	// The author is stored as an email; a public issue must not carry it.
	if strings.Contains(c.comments[0], "@wso2.com") {
		t.Errorf("the author's email address reached a public issue: %q", c.comments[0])
	}
	if !strings.Contains(c.comments[0], "nimal") {
		t.Errorf("the author's name was lost: %q", c.comments[0])
	}
}

// A work note is internal by definition. Relaying it would disclose something
// written on the assumption it stayed inside.
func TestOutbound_WorkNoteIsNotRelayed(t *testing.T) {
	c := &fakeGhClient{}
	err := obSvc(c).Deliver(context.Background(), obItem(outboundCommentAdded, map[string]any{
		"content": "customer is threatening to escalate", "createdBy": "x@wso2.com", "type": "WORK_NOTE",
	}))
	if err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	if len(c.comments) != 0 {
		t.Fatalf("a work note was posted to GitHub: %q", c.comments)
	}
}

func TestOutbound_EmptyCommentIsNotRelayed(t *testing.T) {
	c := &fakeGhClient{}
	if err := obSvc(c).Deliver(context.Background(), obItem(outboundCommentAdded, map[string]any{
		"content": "   ", "type": "COMMENT",
	})); err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	if len(c.comments) != 0 {
		t.Fatal("an empty comment was posted")
	}
}

// Only the columns on the allow-list are reported. A column added to the table
// later must be silent by default, not leak onto a public issue.
func TestOutbound_OnlyReportableChanges(t *testing.T) {
	c := &fakeGhClient{}
	err := obSvc(c).Deliver(context.Background(), obItem(outboundCRUpdated, map[string]any{
		"changes": map[string]any{
			"state":                map[string]any{"from": "NEW", "to": "ASSESS"},
			"justification":        map[string]any{"from": "a", "to": "b"},
			"risk_impact_analysis": map[string]any{"from": "x", "to": "y"},
		},
	}))
	if err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	if len(c.comments) != 1 {
		t.Fatalf("posted %d comments, want 1", len(c.comments))
	}
	body := c.comments[0]
	if !strings.Contains(body, "NEW → ASSESS") {
		t.Errorf("the state change was not reported: %q", body)
	}
	for _, leaked := range []string{"justification", "risk_impact_analysis", "\"a\"", "\"x\""} {
		if strings.Contains(body, leaked) {
			t.Errorf("an off-list column reached GitHub (%s): %q", leaked, body)
		}
	}
}

// An update touching nothing reportable says nothing at all, rather than
// posting an empty notice.
func TestOutbound_NothingReportableIsSilent(t *testing.T) {
	c := &fakeGhClient{}
	if err := obSvc(c).Deliver(context.Background(), obItem(outboundCRUpdated, map[string]any{
		"changes": map[string]any{"justification": map[string]any{"from": "a", "to": "b"}},
	})); err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	if len(c.comments) != 0 {
		t.Fatalf("posted a comment for an unreportable change: %q", c.comments)
	}
}

func TestOutbound_CRCreated(t *testing.T) {
	c := &fakeGhClient{}
	if err := obSvc(c).Deliver(context.Background(), obItem(outboundCRCreated, nil)); err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	if len(c.comments) != 1 || !strings.Contains(c.comments[0], "change request") {
		t.Fatalf("comments = %q", c.comments)
	}
	if !strings.Contains(c.comments[0], "csm.example/operations/change-requests/cr-1") {
		t.Errorf("the link back to the record is missing: %q", c.comments[0])
	}
}

// Never set labels or state outbound: those are what DRIVE the change request
// from the inbound side, so writing them back is how a loop starts.
func TestOutbound_NeverWritesLabelsOrState(t *testing.T) {
	c := &fakeGhClient{}
	for _, ev := range []string{outboundCRCreated, outboundCRUpdated, outboundCommentAdded} {
		_ = obSvc(c).Deliver(context.Background(), obItem(ev, map[string]any{
			"content": "x", "type": "COMMENT",
			"changes": map[string]any{"state": map[string]any{"from": "NEW", "to": "ASSESS"}},
		}))
	}
	if len(c.labels) != 0 || len(c.removed) != 0 {
		t.Errorf("labels were written outbound: set=%v removed=%v", c.labels, c.removed)
	}
	if len(c.states) != 0 {
		t.Errorf("issue state was written outbound: %v", c.states)
	}
}

// A row with no issue on it cannot be posted anywhere, and no amount of
// retrying will put one there. The trigger will not write such a row -- all
// three columns are NOT NULL -- so this guards the case where one is inserted
// by hand or by a future caller that skips the trigger.
func TestOutbound_RowWithoutAnIssueIsPermanent(t *testing.T) {
	for name, mangle := range map[string]func(*repository.OutboundItem){
		"no owner":      func(i *repository.OutboundItem) { i.Owner = "" },
		"no repository": func(i *repository.OutboundItem) { i.Repository = "" },
		"no number":     func(i *repository.OutboundItem) { i.IssueNumber = 0 },
	} {
		t.Run(name, func(t *testing.T) {
			c := &fakeGhClient{}
			item := obItem(outboundCommentAdded, map[string]any{"content": "x", "type": "COMMENT"})
			mangle(&item)
			err := obSvc(c).Deliver(context.Background(), item)
			if err == nil {
				t.Fatal("want an error")
			}
			if !OutboundPermanent(err) {
				t.Fatal("a row with no issue should be permanent, not retried")
			}
			if len(c.comments) != 0 {
				t.Fatalf("posted anyway: %q", c.comments)
			}
		})
	}
}

func TestOutboundBackoff(t *testing.T) {
	prev := time.Duration(0)
	for attempt := 1; attempt <= 8; attempt++ {
		d := OutboundBackoff(attempt)
		if d < prev {
			t.Fatalf("backoff went backwards at attempt %d: %v after %v", attempt, d, prev)
		}
		if d > outboundMaxBackoff {
			t.Fatalf("backoff exceeded the cap at attempt %d: %v", attempt, d)
		}
		prev = d
	}
	if OutboundBackoff(1) != outboundBaseBackoff {
		t.Errorf("first attempt = %v, want %v", OutboundBackoff(1), outboundBaseBackoff)
	}
}

func TestOutboundPermanent(t *testing.T) {
	cases := map[string]struct {
		err  error
		want bool
	}{
		"404 gone":         {&github.Error{StatusCode: 404}, true},
		"410 gone":         {&github.Error{StatusCode: 410}, true},
		"403 permissions":  {&github.Error{StatusCode: 403}, true},
		"403 rate limited": {&github.Error{StatusCode: 403, RetryAfter: time.Minute}, false},
		"429 rate limited": {&github.Error{StatusCode: 429, RetryAfter: time.Minute}, false},
		"502 transient":    {&github.Error{StatusCode: 502}, false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := OutboundPermanent(tc.err); got != tc.want {
				t.Fatalf("OutboundPermanent(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

// GitHub knows when its limit resets; our backoff is a guess. Its answer wins.
func TestOutboundRetryAfter(t *testing.T) {
	d, ok := OutboundRetryAfter(&github.Error{StatusCode: 429, RetryAfter: 90 * time.Second})
	if !ok || d != 90*time.Second {
		t.Fatalf("got %v,%v want 90s,true", d, ok)
	}
	if _, ok := OutboundRetryAfter(&github.Error{StatusCode: 502}); ok {
		t.Fatal("a plain 502 should not report a retry-after")
	}
}

// The two halves of "SN Case Updates -> GitHub".

func TestOutbound_CaseClosedCarriesResolutionNotes(t *testing.T) {
	c := &fakeGhClient{}
	err := obSvc(c).Deliver(context.Background(), obItem(outboundCaseClosed, map[string]any{
		"resolutionNotes": "Root cause was a stale cache entry; fixed in 2.4.1.",
	}))
	if err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	if len(c.comments) != 1 {
		t.Fatalf("posted %d comments, want 1", len(c.comments))
	}
	if !strings.Contains(c.comments[0], "stale cache entry") {
		t.Errorf("resolution notes missing: %q", c.comments[0])
	}
	if !strings.Contains(c.comments[0], "closed") {
		t.Errorf("closure not stated: %q", c.comments[0])
	}
}

// A case closed with nothing written in the notes field still gets the notice.
// ServiceNow sent one, and "this issue's case is closed" is the part the
// reader of the issue actually needs.
func TestOutbound_CaseClosedWithoutNotesStillPosts(t *testing.T) {
	c := &fakeGhClient{}
	if err := obSvc(c).Deliver(context.Background(), obItem(outboundCaseClosed, map[string]any{})); err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	if len(c.comments) != 1 {
		t.Fatalf("posted %d comments, want 1", len(c.comments))
	}
}

func TestOutbound_CaseAssignedNamesThePerson(t *testing.T) {
	c := &fakeGhClient{}
	err := obSvc(c).Deliver(context.Background(), obItem(outboundCaseAssigned, map[string]any{
		"assignedTo": "Nimal Perera",
	}))
	if err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	if len(c.comments) != 1 {
		t.Fatalf("posted %d comments, want 1", len(c.comments))
	}
	if !strings.Contains(c.comments[0], "Nimal Perera") {
		t.Errorf("assignee missing: %q", c.comments[0])
	}
}

// Unassignment resolves to no name, and "assigned to nobody" is not a comment
// worth posting on a customer's issue.
func TestOutbound_CaseUnassignedPostsNothing(t *testing.T) {
	c := &fakeGhClient{}
	if err := obSvc(c).Deliver(context.Background(), obItem(outboundCaseAssigned, map[string]any{
		"assignedTo": "",
	})); err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	if len(c.comments) != 0 {
		t.Fatalf("posted %d comments, want 0: %q", len(c.comments), c.comments)
	}
}

// Comments sync from the case, so the link in one must be a case link. This
// shipped pointing at /operations/change-requests/<case id>, which 404s.
func TestOutbound_CommentLinksToTheCaseNotTheChangeRequest(t *testing.T) {
	c := &fakeGhClient{}
	err := obSvc(c).Deliver(context.Background(), obItem(outboundCommentAdded, map[string]any{
		"content": "scheduled for Friday", "createdBy": "nimal@wso2.com", "type": "COMMENT",
	}))
	if err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	if strings.Contains(c.comments[0], "change-requests") {
		t.Errorf("a case comment linked to the change-request route: %q", c.comments[0])
	}
	if !strings.Contains(c.comments[0], "/cases/") {
		t.Errorf("no case link in a case comment: %q", c.comments[0])
	}
}

// The case journal carries machine-written entries -- auto-closure reminders
// and the CR notices this service already posts itself. They are dropped by
// AUTHOR, never by matching the text, so rewording a template cannot silently
// start leaking them onto a customer's issue.
func TestOutbound_MachineAuthoredCommentsAreNotMirrored(t *testing.T) {
	for _, author := range []string{"system", "github_integration", "github_pipeline", "SYSTEM"} {
		t.Run(author, func(t *testing.T) {
			c := &fakeGhClient{}
			err := obSvc(c).Deliver(context.Background(), obItem(outboundCommentAdded, map[string]any{
				"content":   "Hi team, This case is in Solution Proposed state and is being monitored.",
				"createdBy": author, "type": "COMMENT",
			}))
			if err != nil {
				t.Fatalf("Deliver: %v", err)
			}
			if len(c.comments) != 0 {
				t.Fatalf("machine comment reached the issue: %q", c.comments)
			}
		})
	}
}

// A person's comment still syncs -- the filter must not swallow the real ones.
func TestOutbound_PeopleComentsStillSync(t *testing.T) {
	c := &fakeGhClient{}
	err := obSvc(c).Deliver(context.Background(), obItem(outboundCommentAdded, map[string]any{
		"content": "Scheduled for Friday.", "createdBy": "nimal@wso2.com", "type": "COMMENT",
	}))
	if err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	if len(c.comments) != 1 {
		t.Fatalf("a person's comment was dropped")
	}
}

// An empty-but-present override restores the old mirror-everything behaviour.
func TestOutbound_EmptySkipListMirrorsEverything(t *testing.T) {
	c := &fakeGhClient{}
	svc := NewGithubOutboundService(c, "https://csm.example", nil)
	err := svc.Deliver(context.Background(), obItem(outboundCommentAdded, map[string]any{
		"content": "Auto closure notice.", "createdBy": "system", "type": "COMMENT",
	}))
	if err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	if len(c.comments) != 1 {
		t.Fatalf("an empty skip list should mirror everything")
	}
}
