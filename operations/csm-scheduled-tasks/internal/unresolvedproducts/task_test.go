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
package unresolvedproducts

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type fakeLister struct {
	nodes     []Node
	truncated bool
	err       error
	since     time.Time
	calls     int
}

func (f *fakeLister) ListSince(_ context.Context, since time.Time) (Listing, error) {
	f.calls++
	f.since = since
	return Listing{Nodes: f.nodes, Truncated: f.truncated}, f.err
}

type fakeEmail struct {
	to, cc        []string
	subject, body string
	sent          int
}

func (f *fakeEmail) SendEmail(_ context.Context, to, cc []string, subject, body string) error {
	f.sent++
	f.to, f.cc, f.subject, f.body = to, cc, subject, body
	return nil
}

var fixedNow = func() time.Time { return time.Date(2026, 10, 22, 6, 0, 0, 0, time.UTC) }

func ptr(s string) *string { return &s }

func TestSendDigest_SendsTheNodesSeenInTheWindow(t *testing.T) {
	list := &fakeLister{nodes: []Node{
		{NodeID: strings.Repeat("a", 64), ProjectKey: ptr("SCTPSUB"), DeploymentNumber: ptr("DEP000002519"),
			ReportedProduct: "wso2am", ReportedVersion: ptr("9.9.9"), UnresolvedSince: fixedNow(), LastSeenOn: fixedNow()},
		{NodeID: "node-2", ReportedProduct: "wso2am", ReportedVersion: ptr("9.9.9"), UnresolvedSince: fixedNow(), LastSeenOn: fixedNow()},
		{NodeID: "node-3", ReportedProduct: "wso2zz<script>", UnresolvedSince: fixedNow(), LastSeenOn: fixedNow()},
	}}
	email := &fakeEmail{}
	err := SendDigest(list, email, 24*time.Hour, []string{"catalogue@wso2.com"}, []string{"cc@wso2.com"}, true, fixedNow)(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !list.since.Equal(fixedNow().Add(-24 * time.Hour)) {
		t.Errorf("since = %v, want 24 hours before now", list.since)
	}
	if email.sent != 1 || email.to[0] != "catalogue@wso2.com" || email.cc[0] != "cc@wso2.com" {
		t.Fatalf("email sent %d to %v cc %v", email.sent, email.to, email.cc)
	}
	if !strings.Contains(email.subject, "2 product version(s)") {
		t.Errorf("subject %q should count distinct product versions (wso2am 9.9.9 twice is one)", email.subject)
	}
	for _, want := range []string{"SCTPSUB", "DEP000002519", "9.9.9", "aaaaaaaaaaaa", "&lt;script&gt;", "24 hours"} {
		if !strings.Contains(email.body, want) {
			t.Errorf("body is missing %q", want)
		}
	}
	if strings.Contains(email.body, strings.Repeat("a", 64)) {
		t.Error("the full 64-character node id should be shortened")
	}
	if strings.Contains(email.body, "<script>") {
		t.Error("reported text must be escaped")
	}
}

func TestSendDigest_NothingToReportSendsNothing(t *testing.T) {
	list := &fakeLister{}
	email := &fakeEmail{}
	if err := SendDigest(list, email, 24*time.Hour, []string{"x@wso2.com"}, nil, true, fixedNow)(context.Background()); err != nil {
		t.Fatal(err)
	}
	if list.calls != 1 || email.sent != 0 {
		t.Errorf("listed %d times, sent %d emails; want 1 and 0", list.calls, email.sent)
	}
}

func TestSendDigest_NoRecipientsOrEmailsOffSkipsTheLookup(t *testing.T) {
	for name, tc := range map[string]struct {
		to      []string
		enabled bool
	}{"no recipients": {nil, true}, "emails off": {[]string{"x@wso2.com"}, false}} {
		list := &fakeLister{}
		email := &fakeEmail{}
		if err := SendDigest(list, email, 24*time.Hour, tc.to, nil, tc.enabled, fixedNow)(context.Background()); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if list.calls != 0 || email.sent != 0 {
			t.Errorf("%s: listed %d, sent %d; want neither", name, list.calls, email.sent)
		}
	}
}

func TestSendDigest_ListingFailureFailsTheRun(t *testing.T) {
	list := &fakeLister{err: errors.New("boom")}
	err := SendDigest(list, &fakeEmail{}, 24*time.Hour, []string{"x@wso2.com"}, nil, true, fixedNow)(context.Background())
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("error = %v", err)
	}
}

func TestSendDigest_ShowsWhenEachNodeWentUnresolvedAndWhenTheListIsCut(t *testing.T) {
	since := time.Date(2026, 9, 3, 10, 0, 0, 0, time.UTC)
	list := &fakeLister{truncated: true, nodes: []Node{
		{NodeID: "node-1", ReportedProduct: "wso2am", ReportedVersion: ptr("9.9.9"), UnresolvedSince: since, LastSeenOn: fixedNow()},
	}}
	email := &fakeEmail{}
	if err := SendDigest(list, email, 24*time.Hour, []string{"x@wso2.com"}, nil, true, fixedNow)(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"2026-09-03", "Unresolved Since", "This list is incomplete"} {
		if !strings.Contains(email.body, want) {
			t.Errorf("body is missing %q", want)
		}
	}

	list.truncated = false
	if err := SendDigest(list, email, 24*time.Hour, []string{"x@wso2.com"}, nil, true, fixedNow)(context.Background()); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(email.body, "This list is incomplete") {
		t.Error("a complete list must not be marked incomplete")
	}
}
