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
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// snGithubFixtureBody is a Service Request-Generic issue body. The expected
// fields and description below were produced by the repositories' own
// extractFields.js and servicenow_create_case.yml's jq step on this body, so
// these tests hold the Go port to the original.
const snGithubFixtureBody = `### ServiceNow Catalog Item

General Requests

### Request Details

Rotate the gateway certificate

### Description

Line one
Line two with [runbook](https://example.com/runbook)

### Priority

High (P2)

### Environment Details

Development,Production

### Template Marker

sr-generic

### Extra Notes

_No response_

### My Custom (Field)

value`

const snGithubFixtureDescription = `<span style="color:red;font-weight:bold;font-size:2em;">ENVIRONMENT: Development, Production</span><br/><br/><strong>CATALOG ITEM:</strong><br/>General Requests<br/><br/><strong>REQUEST DETAILS:</strong><br/>Rotate the gateway certificate<br/><br/><strong>DESCRIPTION:</strong><br/>Line one<br/>Line two with <a href="https://example.com/runbook" target="_blank" rel="noopener">runbook</a><br/><br/><strong>PRIORITY:</strong><br/>2 - High<br/><br/><strong>MY CUSTOM FIELD:</strong><br/>value<br/><br/><br/><strong>─────────────────────────────────────</strong><br/><strong>SOURCE:</strong> GitHub Issue #42<br/><strong>URL:</strong> https://github.com/wso2/choreo/issues/42<br/><strong>Repository:</strong> wso2/choreo<br/><strong>User:</strong> octocat<br/><strong>Labels:</strong> Type/ServiceRequest, validation-passed<br/>`

func snGithubFixtureIssue() snGithubIssue {
	return snGithubIssue{
		Number: 42, Title: "Rotate the gateway certificate", Body: snGithubFixtureBody,
		URL: "https://github.com/wso2/choreo/issues/42", User: "octocat",
		Labels: []string{"Type/ServiceRequest", "validation-passed"}, RepoFull: "wso2/choreo",
	}
}

func TestExtractSNIssueFields_MatchesExtractFieldsJS(t *testing.T) {
	f := extractSNIssueFields(snGithubFixtureBody)
	var keys []string
	for _, e := range f.order {
		keys = append(keys, e.key)
	}
	// Known labels mapped, "Template Marker" and "_No response_" dropped, an
	// unknown label keyed u_<words>, priority mapped where it stood.
	if got, want := strings.Join(keys, ","), "catalog_item,u_request_details,description,priority,u_project_environment,u_my_custom_field"; got != want {
		t.Errorf("keys = %s\nwant   %s", got, want)
	}
	for key, want := range map[string]string{
		"priority":              "2 - High",
		"u_project_environment": "Development, Production",
		"description":           "Line one\nLine two with <a href=\"https://example.com/runbook\" target=\"_blank\" rel=\"noopener\">runbook</a>",
	} {
		if got, _ := f.get(key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}

	// No priority at all defaults to Moderate, appended last.
	f = extractSNIssueFields("### Request Details\n\nx")
	if got, _ := f.get("priority"); got != "3 - Moderate" || f.order[len(f.order)-1].key != "priority" {
		t.Errorf("default priority = %q at the end of %v, want 3 - Moderate", got, f.order)
	}
}

func TestSNGithubCaseDescription_MatchesTheWorkflow(t *testing.T) {
	issue := snGithubFixtureIssue()
	if got := snGithubCaseDescription(extractSNIssueFields(issue.Body), issue); got != snGithubFixtureDescription {
		t.Errorf("description differs from servicenow_create_case.yml's\n got: %s\nwant: %s", got, snGithubFixtureDescription)
	}
}

func TestSNGithubCaseCreate_Payload(t *testing.T) {
	p := snGithubCaseCreate(snGithubFixtureIssue(),
		"3587f161-c3ba-8f10-af2f-404599013160", "01b7f961-c3ba-8f10-af2f-40459901313d",
		"Issue", "General", CatalogGeneralRequests, "Type/ServiceRequest")
	for key, want := range map[string]string{
		// sys_ids, whichever form the repository's file used
		"account":              "3587f161c3ba8f10af2f404599013160",
		"project":              "01b7f961c3ba8f10af2f40459901313d",
		"category":             "Issue",
		"announcement_type":    "General",
		"title":                "Rotate the gateway certificate",
		"issue_number":         "42",
		"issue_url":            "https://github.com/wso2/choreo/issues/42",
		"u_github_issue_url":   "https://github.com/wso2/choreo/issues/42",
		"github_user":          "octocat",
		"repository_full_name": "wso2/choreo",
		"u_github_issue_key":   "wso2/choreo#42",
		"labels":               "Type/ServiceRequest, validation-passed",
		"catalog_item":         "General Requests",
		"case_type":            "Service Request",
		"catalog_prefix":       "Type/ServiceRequest",
		"priority":             "2 - High",
		"u_request_details":    "Rotate the gateway certificate",
		"description":          snGithubFixtureDescription,
	} {
		if got := p[key]; got != want {
			t.Errorf("%s = %v, want %q", key, got, want)
		}
	}
}

func TestSNGithubCommentText(t *testing.T) {
	got := snGithubCommentText("octocat", "https://github.com/wso2/choreo/issues/42#issuecomment-1",
		"see ![shot](https://x.test/a.png) and <img width=\"10\" src=\"https://x.test/b.png\" alt=\"b\">")
	want := "@octocat (GitHub Comment) [https://github.com/wso2/choreo/issues/42#issuecomment-1]\n\nsee https://x.test/a.png and https://x.test/b.png"
	if got != want {
		t.Errorf("comment text\n got: %q\nwant: %q", got, want)
	}
}

// snGithubTestServer is ServiceNow's side over TLS, as the client requires
// https.
func snGithubTestServer(t *testing.T, h http.HandlerFunc) *SNGithubCaseClient {
	t.Helper()
	srv := httptest.NewTLSServer(h)
	t.Cleanup(srv.Close)
	c, err := NewSNGithubCaseClient(srv.URL+"/api/wso2/gh_integration/case", "gh.integration", "s3cret")
	if err != nil {
		t.Fatal(err)
	}
	c.http = srv.Client()
	return c
}

func TestSNGithubCaseClient_Create(t *testing.T) {
	var gotBody map[string]any
	c := snGithubTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if r.Method != http.MethodPost || r.URL.Path != "/api/wso2/gh_integration/case" || !ok ||
			user != "gh.integration" || pass != "s3cret" || r.Header.Get("User-Agent") != snGithubUserAgent {
			t.Errorf("unexpected request %s %s auth=%v ua=%q", r.Method, r.URL.Path, ok, r.Header.Get("User-Agent"))
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"result":{"case_number":"CS0441400","sys_id":"0a1b2c3d4e5f60718293a4b5c6d7e8f9","u_wso2_case_id":"WCSSUB-10"}}`))
	})
	res, err := c.CreateCase(context.Background(), map[string]any{"u_github_issue_key": "wso2/choreo#42"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Number != "CS0441400" || res.SysID != "0a1b2c3d4e5f60718293a4b5c6d7e8f9" || res.WSO2CaseID != "WCSSUB-10" || res.Existing {
		t.Errorf("result = %+v", res)
	}
	if gotBody["u_github_issue_key"] != "wso2/choreo#42" {
		t.Errorf("ServiceNow got %v", gotBody)
	}
}

func TestSNGithubCaseClient_CreateAnswers(t *testing.T) {
	for name, tc := range map[string]struct {
		status   int
		body     string
		existing bool
		wantErr  bool
	}{
		"already exists, unwrapped": {200, `{"message":"Case already exists","case_number":"CS1","case_sys_id":"0a1b2c3d4e5f60718293a4b5c6d7e8f9","u_wso2_case_id":"K-1"}`, true, false},
		"server error":              {500, `{"error":"boom"}`, false, true},
		"no case in the answer":     {201, `{"result":{"message":"ok"}}`, false, true},
		"not JSON":                  {201, `<html>`, false, true},
	} {
		c := snGithubTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(tc.status)
			_, _ = w.Write([]byte(tc.body))
		})
		res, err := c.CreateCase(context.Background(), map[string]any{})
		if (err != nil) != tc.wantErr || (err == nil && res.Existing != tc.existing) {
			t.Errorf("%s: res=%+v err=%v", name, res, err)
		}
	}
}

func TestSNGithubCaseClient_UpdateAndComment(t *testing.T) {
	var commentPath, commentBody string
	c := snGithubTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPatch && r.URL.Path == "/api/wso2/gh_integration/case":
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/api/now/table/sn_customerservice_case/"):
			commentPath = r.URL.Path
			raw, _ := io.ReadAll(r.Body)
			commentBody = string(raw)
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})
	if err := c.UpdateCase(context.Background(), map[string]any{}); !errors.Is(err, errSNGithubCaseNotFound) {
		t.Errorf("update of an unknown case: err = %v, want errSNGithubCaseNotFound", err)
	}
	if err := c.AddComment(context.Background(), "0a1b2c3d4e5f60718293a4b5c6d7e8f9", "hello"); err != nil {
		t.Fatal(err)
	}
	if commentPath != "/api/now/table/sn_customerservice_case/0a1b2c3d4e5f60718293a4b5c6d7e8f9" || commentBody != `{"comments":"hello"}` {
		t.Errorf("comment went to %s with %s", commentPath, commentBody)
	}
}

func TestNewSNGithubCaseClient_RequiresHTTPS(t *testing.T) {
	for _, u := range []string{"http://x.service-now.com/api/wso2/gh_integration/case", "not a url", ""} {
		if _, err := NewSNGithubCaseClient(u, "u", "p"); err == nil {
			t.Errorf("%q: want an error", u)
		}
	}
}

// fakeSNCases is ServiceNow's GitHub case API for the sync tests.
type fakeSNCases struct {
	createErr error
	updateErr error
	created   []map[string]any
	updates   []map[string]any
	comments  []string // "<sysid>|<text>"
}

func (f *fakeSNCases) CreateCase(_ context.Context, p map[string]any) (snGithubCaseResult, error) {
	f.created = append(f.created, p)
	if f.createErr != nil {
		return snGithubCaseResult{}, f.createErr
	}
	return snGithubCaseResult{SysID: "0a1b2c3d4e5f60718293a4b5c6d7e8f9", Number: "CS0441400", WSO2CaseID: "WCSSUB-10"}, nil
}

func (f *fakeSNCases) UpdateCase(_ context.Context, p map[string]any) error {
	f.updates = append(f.updates, p)
	return f.updateErr
}

func (f *fakeSNCases) AddComment(_ context.Context, sysID, text string) error {
	f.comments = append(f.comments, sysID+"|"+text)
	return f.updateErr
}

func mappedWithProject() *repository.RepoMapping {
	m := mapped()
	m.ProjectID = "01b7f961-c3ba-8f10-af2f-40459901313d"
	m.Category, m.AnnouncementType = "Issue", "General"
	return m
}

func snFirstSvc(r *fakeGhRepo, m *fakeGhMutations, sn *fakeSNCases, n *recordingSRNotifier) GithubSyncService {
	svc := WithServiceNowGithubCases(writingSvc(r, m, &fakeGhClient{}), sn)
	if n != nil {
		svc.(*githubSyncService).srNotices = n
	}
	return svc
}

func srDelivery(action string) Delivery {
	d := ghDelivery("issues", action, "Rotate the gateway certificate", []string{"Type/ServiceRequest"})
	d.Payload.Issue.Body = snGithubFixtureBody
	return d
}

// Under dual-write the SR is created in ServiceNow first and the Postgres row
// takes ServiceNow's sys_id, CS number and WSO2 Case ID, with ServiceNow's
// description. ServiceNow's own flow acknowledges and announces it, so ours
// does not.
func TestGithubSNFirst_CreatesInServiceNowAndTakesItsIDs(t *testing.T) {
	mut, sn, n := &fakeGhMutations{}, &fakeSNCases{}, &recordingSRNotifier{}
	out, err := snFirstSvc(&fakeGhRepo{mapping: mappedWithProject()}, mut, sn, n).HandleWebhook(context.Background(), srDelivery("labeled"))
	if err != nil {
		t.Fatal(err)
	}
	if out.Action != "created" || len(sn.created) != 1 {
		t.Fatalf("outcome %+v, ServiceNow creates %d", out, len(sn.created))
	}
	p := sn.created[0]
	if p["project"] != "01b7f961c3ba8f10af2f40459901313d" || p["catalog_item"] != "General Requests" ||
		p["catalog_prefix"] != "Type/ServiceRequest" || p["u_github_issue_key"] != "wso2/choreo#42" || p["category"] != "Issue" {
		t.Errorf("ServiceNow payload: %v", p)
	}
	if mut.lastSR.ServiceNowID != sysidToUUID("0a1b2c3d4e5f60718293a4b5c6d7e8f9") ||
		mut.lastSR.ServiceNowNumber != "CS0441400" || mut.lastSR.ServiceNowWSO2ID != "WCSSUB-10" {
		t.Errorf("Postgres row did not take ServiceNow's ids: %+v", mut.lastSR)
	}
	if !strings.HasPrefix(mut.lastSR.Description, `<span style="color:red;font-weight:bold;font-size:2em;">ENVIRONMENT: Development, Production</span>`) {
		t.Errorf("Postgres description is not ServiceNow's: %q", mut.lastSR.Description)
	}
	if len(n.created) != 0 {
		t.Errorf("our SR automation ran for %v; ServiceNow's flow handles a ServiceNow-first SR", n.created)
	}
}

// ServiceNow refusing the create writes nothing and fails the delivery, which
// releases its claim so a redelivery can retry.
func TestGithubSNFirst_ServiceNowFailureWritesNothing(t *testing.T) {
	mut, repo := &fakeGhMutations{}, &fakeGhRepo{mapping: mappedWithProject()}
	_, err := snFirstSvc(repo, mut, &fakeSNCases{createErr: errors.New("503")}, nil).HandleWebhook(context.Background(), srDelivery("labeled"))
	if err == nil {
		t.Fatal("want an error when ServiceNow refuses the create")
	}
	if mut.created != 0 {
		t.Errorf("Postgres got %d records with no ServiceNow case", mut.created)
	}
	if repo.claimed["d1"] {
		t.Error("the failed delivery kept its claim; a redelivery would be refused")
	}
}

// A repository mapped without a project (account_github_repo) cannot go
// through ServiceNow's API; it keeps the GitHub series and our automation.
func TestGithubSNFirst_NoProjectKeepsTheGitHubSeries(t *testing.T) {
	mut, sn, n := &fakeGhMutations{}, &fakeSNCases{}, &recordingSRNotifier{}
	if _, err := snFirstSvc(&fakeGhRepo{mapping: mapped()}, mut, sn, n).HandleWebhook(context.Background(), srDelivery("labeled")); err != nil {
		t.Fatal(err)
	}
	if len(sn.created) != 0 || mut.lastSR.ServiceNowID != "" || len(n.created) != 1 {
		t.Errorf("sn creates=%d snID=%q automation=%v", len(sn.created), mut.lastSR.ServiceNowID, n.created)
	}
}

// An edit reaches ServiceNow (servicenow_update_case.yml's PATCH); a label the
// workflows apply does not. Postgres keeps ServiceNow's text for the record.
func TestGithubSNFirst_EditReachesServiceNow(t *testing.T) {
	mut, sn := &fakeGhMutations{}, &fakeSNCases{}
	svc := snFirstSvc(&fakeGhRepo{mapping: mappedWithProject(), caseID: "sr-1"}, mut, sn, nil)
	if _, err := svc.HandleWebhook(context.Background(), srDelivery("labeled")); err != nil {
		t.Fatal(err)
	}
	if len(sn.updates) != 0 {
		t.Errorf("a label change was sent to ServiceNow: %v", sn.updates)
	}
	d := srDelivery("edited")
	d.ID = "d2"
	if _, err := svc.HandleWebhook(context.Background(), d); err != nil {
		t.Fatal(err)
	}
	if len(sn.updates) != 1 || sn.updates[0]["u_github_issue_key"] != "wso2/choreo#42" {
		t.Errorf("ServiceNow updates: %v", sn.updates)
	}
	if !strings.HasPrefix(mut.lastUpdate.ServiceNowDescription, "<span ") {
		t.Errorf("update carried no ServiceNow description: %q", mut.lastUpdate.ServiceNowDescription)
	}

	// ServiceNow not having the case (a record of the GitHub series) is not a failure.
	sn.updateErr = errSNGithubCaseNotFound
	d.ID = "d3"
	if _, err := svc.HandleWebhook(context.Background(), d); err != nil {
		t.Errorf("a case ServiceNow does not have failed the delivery: %v", err)
	}
}

// An issue comment reaches the case's ServiceNow journal as
// github_comment_to_sn.yml wrote it.
func TestGithubSNFirst_CommentReachesTheServiceNowJournal(t *testing.T) {
	sn := &fakeSNCases{}
	caseID := sysidToUUID("0a1b2c3d4e5f60718293a4b5c6d7e8f9")
	d := commentDelivery("Any update?", "octocat")
	d.Payload.Comment.HTMLURL = "https://github.com/wso2/choreo/issues/42#issuecomment-7"
	if _, err := snFirstSvc(&fakeGhRepo{mapping: mappedWithProject(), caseID: caseID}, &fakeGhMutations{}, sn, nil).
		HandleWebhook(context.Background(), d); err != nil {
		t.Fatal(err)
	}
	want := "0a1b2c3d4e5f60718293a4b5c6d7e8f9|@octocat (GitHub Comment) [https://github.com/wso2/choreo/issues/42#issuecomment-7]\n\nAny update?"
	if len(sn.comments) != 1 || sn.comments[0] != want {
		t.Errorf("ServiceNow comments %q, want %q", sn.comments, want)
	}
}
