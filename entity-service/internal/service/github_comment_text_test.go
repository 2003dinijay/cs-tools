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
)

// The inputs here are real comment bodies taken from the staging database, not
// invented ones: the markup this has to survive is whatever ServiceNow wrote.
func TestSNToMarkdown(t *testing.T) {
	cases := map[string]struct{ in, want string }{
		"plain text is untouched": {
			in:   "Scheduling the upgrade for Friday 22:00 UTC.",
			want: "Scheduling the upgrade for Friday 22:00 UTC.",
		},
		"code wrapper and paragraph": {
			in:   "[code]<br><p>Priority was updated from  to 1 - Critical</p>[/code]",
			want: "Priority was updated from  to 1 - Critical",
		},
		"heading becomes bold": {
			in:   "[code]<br><b> <u>Description</u> </b><br><p>Test Description goes here</p>[/code]",
			want: "**Description**\nTest Description goes here",
		},
		"strong is bold too": {
			in:   "<p><strong>TEMPLATE MARKER:</strong><br />SR General</p>",
			want: "**TEMPLATE MARKER:**\nSR General",
		},
		"entities are decoded": {
			in:   "<p>Cert expires &amp; must be rotated &lt;soon&gt;</p>",
			want: "Cert expires & must be rotated <soon>",
		},
		"markup with no text yields nothing": {
			in:   "[code]<br><br></p>[/code]",
			want: "",
		},
		"empty stays empty": {in: "", want: ""},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if got := snToMarkdown(c.in); got != c.want {
				t.Errorf("snToMarkdown(%q)\n got: %q\nwant: %q", c.in, got, c.want)
			}
		})
	}
}

// A body that is only markup renders to nothing, and a comment with nothing in
// it must not be posted at all.
func TestOutbound_CommentThatIsOnlyMarkupPostsNothing(t *testing.T) {
	c := &fakeGhClient{}
	err := obSvc(c).Deliver(tCtx(), obItem(outboundCommentAdded, map[string]any{
		"content": "[code]<br><br></p>[/code]", "createdBy": "x@wso2.com", "type": "COMMENT",
	}))
	if err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	if len(c.comments) != 0 {
		t.Fatalf("posted an empty comment: %q", c.comments)
	}
}

// No HTML tag may survive into a comment on a public issue.
func TestOutbound_NoRawTagsReachTheIssue(t *testing.T) {
	c := &fakeGhClient{}
	err := obSvc(c).Deliver(tCtx(), obItem(outboundCommentAdded, map[string]any{
		"content":   "[code]<br><b> <u>Description</u> </b><br><p>Upgrade &amp; restart</p>[/code]",
		"createdBy": "nimal@wso2.com", "type": "COMMENT",
	}))
	if err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	body := c.comments[0]
	for _, bad := range []string{"<br", "<b>", "<u>", "<p>", "[code]", "&amp;"} {
		if contains(body, bad) {
			t.Errorf("%q survived into the issue comment: %q", bad, body)
		}
	}
}

func tCtx() context.Context { return context.Background() }

func contains(s, sub string) bool { return strings.Contains(s, sub) }
