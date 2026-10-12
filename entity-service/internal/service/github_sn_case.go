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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// SERVICENOW-FIRST GITHUB SERVICE REQUESTS (DUAL-WRITE).
//
// Under dual-write a portal service request is created in ServiceNow first and
// takes ServiceNow's numbers: its CS number and its WSO2 Case ID (<KEY>-<n>).
// A service request raised from a GitHub issue had none of that -- it was
// written to Postgres only and numbered SR-GH-<n> / WSO2-GH-<n>, and ServiceNow
// never saw it.
//
// This creates it the way the repositories did before the cutover:
// servicenow_create_case.yml posted the issue to ServiceNow's GitHub case API
// (a scripted REST API run as an integration user, so no user token is
// needed), which answers with the case's sys_id, number and WSO2 Case ID and
// treats an issue it already has as "Case already exists". The payload below
// is that workflow's, field for field; the Postgres record then takes
// ServiceNow's ids, as a portal-created one does. Edits go to the same API
// (servicenow_update_case.yml's PATCH) and issue comments to the case's
// journal (github_comment_to_sn.yml's Table API PATCH).

// snGithubCaseAPI is ServiceNow's side of a GitHub-raised service request.
type snGithubCaseAPI interface {
	CreateCase(ctx context.Context, payload map[string]any) (snGithubCaseResult, error)
	UpdateCase(ctx context.Context, payload map[string]any) error
	AddComment(ctx context.Context, caseSysID, text string) error
}

// snGithubCaseResult is what ServiceNow's GitHub case API answers on create.
type snGithubCaseResult struct {
	SysID      string
	Number     string
	WSO2CaseID string
	// Existing is ServiceNow's "Case already exists": the issue had a case,
	// and these are its ids.
	Existing bool
}

// snGithubUserAgent is the workflows' own (servicenow-config.yml's
// constants.user_agent), kept in case anything on ServiceNow's side keys on it.
const snGithubUserAgent = "GitHub-Actions-WSO2/1.0"

// SNGithubCaseClient calls ServiceNow's GitHub case API and the case table
// with the integration user's basic auth.
type SNGithubCaseClient struct {
	apiURL   string
	baseURL  string
	username string
	password string
	http     *http.Client
}

// NewSNGithubCaseClient builds the client. apiURL is the GitHub case API
// endpoint (the repositories' SERVICENOW_URL); the instance's base URL, for
// the case table, is taken from it.
func NewSNGithubCaseClient(apiURL, username, password string) (*SNGithubCaseClient, error) {
	u, err := url.Parse(strings.TrimSpace(apiURL))
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return nil, errors.New("the ServiceNow GitHub case API URL must be an absolute https URL")
	}
	return &SNGithubCaseClient{
		apiURL:   u.String(),
		baseURL:  u.Scheme + "://" + u.Host,
		username: username,
		password: password,
		http:     &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (c *SNGithubCaseClient) send(ctx context.Context, method, target string, payload any) (int, []byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, nil, fmt.Errorf("sn github case: encode payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(body))
	if err != nil {
		return 0, nil, fmt.Errorf("sn github case: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", snGithubUserAgent)
	req.SetBasicAuth(c.username, c.password)
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("sn github case: %s: %w", method, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return resp.StatusCode, raw, nil
}

// CreateCase posts a new issue (servicenow_create_case.yml's POST).
func (c *SNGithubCaseClient) CreateCase(ctx context.Context, payload map[string]any) (snGithubCaseResult, error) {
	status, raw, err := c.send(ctx, http.MethodPost, c.apiURL, payload)
	if err != nil {
		return snGithubCaseResult{}, err
	}
	if status != http.StatusOK && status != http.StatusCreated {
		return snGithubCaseResult{}, fmt.Errorf("sn github case: create returned %d: %s", status, excerpt(raw))
	}
	return parseSNGithubCaseResult(raw)
}

// parseSNGithubCaseResult reads the create answer as the workflow did:
// result-wrapped or not, sys_id or case_sys_id.
func parseSNGithubCaseResult(raw []byte) (snGithubCaseResult, error) {
	var outer map[string]json.RawMessage
	if err := json.Unmarshal(raw, &outer); err != nil {
		return snGithubCaseResult{}, fmt.Errorf("sn github case: unreadable create response: %s", excerpt(raw))
	}
	body := raw
	if r, ok := outer["result"]; ok && len(r) > 0 && r[0] == '{' {
		body = r
	}
	var r struct {
		CaseNumber string `json:"case_number"`
		SysID      string `json:"sys_id"`
		CaseSysID  string `json:"case_sys_id"`
		WSO2CaseID string `json:"u_wso2_case_id"`
		Message    string `json:"message"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return snGithubCaseResult{}, fmt.Errorf("sn github case: unreadable create response: %s", excerpt(raw))
	}
	sysID := r.SysID
	if sysID == "" {
		sysID = r.CaseSysID
	}
	if sysID == "" || r.CaseNumber == "" {
		return snGithubCaseResult{}, fmt.Errorf("sn github case: create response carries no case: %s", excerpt(raw))
	}
	return snGithubCaseResult{
		SysID: sysID, Number: r.CaseNumber, WSO2CaseID: r.WSO2CaseID,
		Existing: strings.EqualFold(strings.TrimSpace(r.Message), "Case already exists"),
	}, nil
}

// UpdateCase applies an edited issue (servicenow_update_case.yml's PATCH).
func (c *SNGithubCaseClient) UpdateCase(ctx context.Context, payload map[string]any) error {
	status, raw, err := c.send(ctx, http.MethodPatch, c.apiURL, payload)
	if err != nil {
		return err
	}
	if status == http.StatusNotFound {
		return errSNGithubCaseNotFound
	}
	if status != http.StatusOK {
		return fmt.Errorf("sn github case: update returned %d: %s", status, excerpt(raw))
	}
	return nil
}

// AddComment adds an issue comment to the case's customer-visible journal
// (github_comment_to_sn.yml's Table API PATCH).
func (c *SNGithubCaseClient) AddComment(ctx context.Context, caseSysID, text string) error {
	target := c.baseURL + "/api/now/table/sn_customerservice_case/" + url.PathEscape(caseSysID)
	status, raw, err := c.send(ctx, http.MethodPatch, target, map[string]string{"comments": text})
	if err != nil {
		return err
	}
	if status == http.StatusNotFound {
		return errSNGithubCaseNotFound
	}
	if status != http.StatusOK && status != http.StatusNoContent {
		return fmt.Errorf("sn github case: comment returned %d: %s", status, excerpt(raw))
	}
	return nil
}

// errSNGithubCaseNotFound is ServiceNow answering 404: it has no case for
// the issue or the id, as for a record of the GitHub series.
var errSNGithubCaseNotFound = errors.New("sn github case: not found")

func excerpt(raw []byte) string {
	s := strings.TrimSpace(string(raw))
	if len(s) > 300 {
		s = s[:300] + "..."
	}
	return s
}

// snIssueField is one "### Label" of an issue form, in the order the body has it.
type snIssueField struct{ key, value string }

// snIssueFields is extractFields.js's extractAllFields: an ordered set of
// fields, keyed as ServiceNow expects, in the issue body's order. A later
// label mapping to the same key (Severity after Priority) replaces the value
// where the first one stood, as a JavaScript object does.
type snIssueFields struct {
	order []snIssueField
}

func (f *snIssueFields) set(key, value string) {
	for i := range f.order {
		if f.order[i].key == key {
			f.order[i].value = value
			return
		}
	}
	f.order = append(f.order, snIssueField{key, value})
}

func (f *snIssueFields) get(key string) (string, bool) {
	for _, e := range f.order {
		if e.key == key {
			return e.value, true
		}
	}
	return "", false
}

// snKnownIssueFields is extractFields.js's knownFields.
var snKnownIssueFields = map[string]string{
	"servicenow catalog item":                 "catalog_item",
	"catalog item":                            "catalog_item",
	"case type":                               "case_type",
	"priority":                                "priority",
	"severity":                                "priority",
	"short description":                       "u_short_description",
	"description":                             "description",
	"detailed description":                    "description",
	"issue description":                       "description",
	"impact":                                  "u_impact",
	"impact description (overall)":            "u_impact_description_overall",
	"impact description (customer)":           "u_impact_description_customer",
	"environment details":                     "u_project_environment",
	"environment":                             "u_project_environment",
	"affected environments":                   "u_project_environment",
	"issue summary":                           "u_request_details",
	"affected component":                      "u_affected_component",
	"affected services":                       "u_affected_services",
	"service outage/downtime":                 "u_service_outage",
	"is a maintenance window required or not": "u_maintenance_window",
	"maintenance window":                      "u_maintenance_window",
	"implementation plan":                     "u_implementation_plan",
	"implementation steps":                    "u_implementation_plan",
	"test plan":                               "u_test_plan",
	"monitoring checks":                       "u_monitoring_checks",
	"request details":                         "u_request_details",
}

var (
	snNonWordRe       = regexp.MustCompile(`[^\w\s-]`)
	snSpaceRunRe      = regexp.MustCompile(`\s+`)
	snLeadingUPrefRe  = regexp.MustCompile(`^u_+`)
	snImageLinkRe     = regexp.MustCompile(`!\[([^\]]*)\]\((https?://[^\s)]+)\)`)
	snMarkdownLinkRe  = regexp.MustCompile(`\[([^\]]*)\]\((https?://[^\s)]+)\)`)
	snParenthesisedRe = regexp.MustCompile(`\s*\(.*?\)`)
	snHTMLImageRe     = regexp.MustCompile(`<img[^>]*?\ssrc="([^"]+)"[^>]*>`)
	snMarkdownImageRe = regexp.MustCompile(`!\[[^\]]*\]\(([^\s)]+)\)`)
)

// snFieldKey is extractFields.js's key for a label it does not know.
func snFieldKey(label string) string {
	k := strings.ToLower(strings.TrimSpace(snNonWordRe.ReplaceAllString(label, "")))
	k = snSpaceRunRe.ReplaceAllString(k, "_")
	return "u_" + snLeadingUPrefRe.ReplaceAllString(k, "")
}

// snLinkify is extractFields.js's linkifyAttachments.
func snLinkify(v string) string {
	anchor := func(re *regexp.Regexp) func(string) string {
		return func(m string) string {
			sub := re.FindStringSubmatch(m)
			text := sub[1]
			if text == "" {
				text = sub[2]
			}
			return `<a href="` + sub[2] + `" target="_blank" rel="noopener">` + text + `</a>`
		}
	}
	v = snImageLinkRe.ReplaceAllStringFunc(v, anchor(snImageLinkRe))
	return snMarkdownLinkRe.ReplaceAllStringFunc(v, anchor(snMarkdownLinkRe))
}

var snPriorityLabels = map[string]string{
	"Critical": "1 - Critical", "High": "2 - High", "Moderate": "3 - Moderate", "Low": "4 - Low",
}

// extractSNIssueFields ports extractFields.js's extractAllFields.
func extractSNIssueFields(body string) *snIssueFields {
	f := &snIssueFields{}
	var label string
	var value []string
	flush := func() {
		if label == "" {
			return
		}
		v := strings.TrimSpace(strings.Join(value, "\n"))
		l := strings.TrimSpace(label)
		label, value = "", nil
		if v == "" || v == "_No response_" || v == "No response" {
			return
		}
		normalized := strings.ToLower(l)
		// The retired "Template Marker" field is never forwarded.
		if normalized == "template marker" {
			return
		}
		key, ok := snKnownIssueFields[normalized]
		if !ok {
			key = snFieldKey(l)
		}
		f.set(key, snLinkify(v))
	}
	for _, line := range strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n") {
		if rest, ok := strings.CutPrefix(line, "###"); ok && len(rest) > 0 && (rest[0] == ' ' || rest[0] == '\t') {
			flush()
			label = rest
			continue
		}
		if label != "" {
			value = append(value, line)
		}
	}
	flush()

	raw, _ := f.get("priority")
	if raw == "" {
		raw = "Moderate"
	}
	raw = strings.TrimSpace(snParenthesisedRe.ReplaceAllString(raw, ""))
	if p, ok := snPriorityLabels[raw]; ok {
		f.set("priority", p)
	} else {
		f.set("priority", "3 - Moderate")
	}

	if env, ok := f.get("u_project_environment"); ok {
		var parts []string
		for _, e := range strings.Split(env, ",") {
			if e = strings.TrimSpace(e); e != "" {
				parts = append(parts, e)
			}
		}
		f.set("u_project_environment", strings.Join(parts, ", "))
	}
	return f
}

// snGithubIssue is what the payloads describe.
type snGithubIssue struct {
	Number   int
	Title    string
	Body     string
	URL      string
	User     string
	Labels   []string
	RepoFull string // owner/repository
}

// snGithubCaseDescription is the workflows' FORMATTED_DESCRIPTION: the
// environment banner, every field but the environment as a bold heading, and
// the source block.
func snGithubCaseDescription(f *snIssueFields, issue snGithubIssue) string {
	env, ok := f.get("u_project_environment")
	if !ok {
		env = "Not specified"
	}
	var b strings.Builder
	b.WriteString(`<span style="color:red;font-weight:bold;font-size:2em;">ENVIRONMENT: ` + env + `</span><br/><br/>`)
	for _, e := range f.order {
		if e.key == "u_project_environment" {
			continue
		}
		heading := strings.ToUpper(strings.ReplaceAll(strings.TrimPrefix(e.key, "u_"), "_", " "))
		b.WriteString("<strong>" + heading + ":</strong><br/>" + strings.ReplaceAll(e.value, "\n", "<br/>") + "<br/><br/>")
	}
	b.WriteString("<br/><strong>─────────────────────────────────────</strong><br/>")
	b.WriteString("<strong>SOURCE:</strong> GitHub Issue #" + strconv.Itoa(issue.Number) + "<br/>")
	b.WriteString("<strong>URL:</strong> " + issue.URL + "<br/>")
	b.WriteString("<strong>Repository:</strong> " + issue.RepoFull + "<br/>")
	b.WriteString("<strong>User:</strong> " + issue.User + "<br/>")
	b.WriteString("<strong>Labels:</strong> " + strings.Join(issue.Labels, ", ") + "<br/>")
	return b.String()
}

// snGithubCasePayloadBase is every extracted field plus the issue's identity,
// the part the create and update payloads share.
func snGithubCasePayloadBase(f *snIssueFields, issue snGithubIssue) map[string]any {
	p := make(map[string]any, len(f.order)+8)
	for _, e := range f.order {
		p[e.key] = e.value
	}
	p["title"] = issue.Title
	p["issue_number"] = strconv.Itoa(issue.Number)
	p["issue_url"] = issue.URL
	p["github_user"] = issue.User
	p["repository_full_name"] = issue.RepoFull
	p["u_github_issue_key"] = issue.RepoFull + "#" + strconv.Itoa(issue.Number)
	p["description"] = snGithubCaseDescription(f, issue)
	return p
}

// snGithubCaseCreate is servicenow_create_case.yml's payload. account and
// project go as sys_ids, whichever form the repository's file used.
func snGithubCaseCreate(issue snGithubIssue, accountID, projectID, category, announcementType, catalog, catalogPrefix string) map[string]any {
	f := extractSNIssueFields(issue.Body)
	p := snGithubCasePayloadBase(f, issue)
	p["project"] = uuidToSysid(projectID)
	p["account"] = uuidToSysid(accountID)
	p["category"] = category
	p["announcement_type"] = announcementType
	p["u_github_issue_url"] = issue.URL
	p["labels"] = strings.Join(issue.Labels, ", ")
	p["catalog_item"] = catalog
	p["case_type"] = "Service Request"
	p["catalog_prefix"] = catalogPrefix
	return p
}

// snGithubCaseUpdate is servicenow_update_case.yml's payload.
func snGithubCaseUpdate(issue snGithubIssue) map[string]any {
	return snGithubCasePayloadBase(extractSNIssueFields(issue.Body), issue)
}

// snGithubCommentText is github_comment_to_sn.yml's journal note: images
// reduced to their URL, under "@user (GitHub Comment) [url]".
func snGithubCommentText(user, commentURL, body string) string {
	body = snHTMLImageRe.ReplaceAllString(body, "$1")
	body = snMarkdownImageRe.ReplaceAllString(body, "$1")
	return "@" + user + " (GitHub Comment) [" + commentURL + "]\n\n" + body
}
