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

package notifications

import (
	"strings"
	"testing"
)

// TestPlainTextFromHTML is a regression test for a real bug: ServiceNow
// returns case descriptions and comments as rich-text HTML (e.g.
// `<p><span style="white-space: pre-wrap;">some text</span></p>`), and
// escapeMultiline used to HTML-escape that HTML directly, so the source's
// own tags showed up as literal, visible "<p>..." clutter in the rendered
// email instead of just the text.
func TestPlainTextFromHTML(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "servicenow's rich-text wrapper",
			input: `<p><span style="white-space: pre-wrap;">Test comment</span></p>`,
			want:  "Test comment",
		},
		{
			name:  "multiple paragraphs become newlines, not run together",
			input: `<p>First paragraph.</p><p>Second paragraph.</p>`,
			want:  "First paragraph.\nSecond paragraph.",
		},
		{
			name:  "br becomes a newline",
			input: "Line one<br>Line two<br/>Line three",
			want:  "Line one\nLine two\nLine three",
		},
		{
			name:  "heading followed by a paragraph stays separated",
			input: "<h2>Summary</h2><p>Details</p>",
			want:  "Summary\nDetails",
		},
		{
			name:  "plain text with no markup passes through unchanged",
			input: "just plain text, no html here",
			want:  "just plain text, no html here",
		},
		{
			name:  "html entities are decoded",
			input: "<p>Salt &amp; pepper</p>",
			want:  "Salt & pepper",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := plainTextFromHTML(tt.input); got != tt.want {
				t.Errorf("plainTextFromHTML(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestEscapeMultiline_StripsSourceHTMLThenEscapes verifies the full pipeline
// escapeMultiline actually runs: strip the source's own HTML down to plain
// text first (plainTextFromHTML), then HTML-escape the result and convert
// newlines to <br> — so literal "<p>" tags from the source never survive
// into the rendered output, but something a user actually typed (e.g.
// "<3") still gets safely escaped rather than interpreted.
func TestEscapeMultiline_StripsSourceHTMLThenEscapes(t *testing.T) {
	got := escapeMultiline(`<p><span style="white-space: pre-wrap;">Test comment</span></p>`)
	if strings.Contains(got, "&lt;p&gt;") || strings.Contains(got, "<p>") {
		t.Errorf("escapeMultiline(...) = %q, source's own <p> tag leaked into the output one way or another", got)
	}
	if got != "Test comment" {
		t.Errorf("escapeMultiline(...) = %q, want %q", got, "Test comment")
	}

	// A literal "<" a user actually typed must still come out escaped, not
	// stripped as if it were a real tag with nothing after it.
	got = escapeMultiline("I <3 this")
	if !strings.Contains(got, "&lt;3") {
		t.Errorf("escapeMultiline(%q) = %q, want the literal \"<\" escaped, not stripped", "I <3 this", got)
	}
}

// TestEscapeHTML_EncodesNonASCIIAsNumericEntity verifies the fix for a real
// reported bug: entity-service sometimes returns a display name with a
// trailing marker like "Jane Doe Ⓦ" — a browser renders that raw UTF-8
// rune fine, but the external email-sending service this client calls
// doesn't reliably preserve non-ASCII bytes through its own send path, so it
// arrived as a literal "?" in the recipient's inbox. escapeHTML must convert
// it to a numeric HTML character reference instead, which survives as plain
// ASCII regardless and still renders as "Ⓦ" in any HTML-capable mail client.
func TestEscapeHTML_EncodesNonASCIIAsNumericEntity(t *testing.T) {
	got := escapeHTML("Jane Doe Ⓦ")
	want := "Jane Doe &#9420;"
	if got != want {
		t.Errorf("escapeHTML(...) = %q, want %q", got, want)
	}

	// Ordinary ASCII and the standard HTML-escaped characters must still
	// come out exactly as html.EscapeString alone would produce them.
	got = escapeHTML(`Tom & Jerry <script>`)
	want = "Tom &amp; Jerry &lt;script&gt;"
	if got != want {
		t.Errorf("escapeHTML(...) = %q, want %q", got, want)
	}
}

// TestRenderCommentAddedEmail_UsesCaseNumberNotRawID verifies the "commented
// on" strap line renders caseNumber (a human-readable reference like
// "CS0023001"), not some other, meaningless identifier — a real bug this
// caught: the line used to substitute a raw UUID (project id) there
// instead.
func TestRenderCommentAddedEmail_UsesCaseNumberNotRawID(t *testing.T) {
	out := RenderCommentAddedEmail("Jane Doe", "CS0023001", "Something broke", "Working on it", "https://x/comment", "https://x/case")
	if !strings.Contains(out, "CS0023001") {
		t.Error("rendered email doesn't contain the case number")
	}
}

// TestRenderInternalNoteEmail_NoReplyStrapAndUsesWorkNoteWording verifies
// the internal-note layout doesn't carry RenderCommentAddedEmail's
// "Re: <title>" strap (an internal note isn't "about" the case title the
// way a reply is) and uses "added work note" wording instead of
// "commented on case" — matching an existing internal WSO2-support email
// format recipients (always wso2.com staff) are already used to.
func TestRenderInternalNoteEmail_NoReplyStrapAndUsesWorkNoteWording(t *testing.T) {
	out := RenderInternalNoteEmail("Jane Doe", "WSO2-1000", "Something broke", "Internal only", "https://x/comment", "https://x/case")
	if !strings.Contains(out, "added work note") {
		t.Error("rendered email doesn't use the internal-note wording")
	}
	if strings.Contains(out, "Re: Something broke") {
		t.Error("rendered email carries a \"Re: <title>\" strap, which the internal-note layout must not have")
	}
	if !strings.Contains(out, "WSO2-1000") {
		t.Error("rendered email doesn't contain the case reference")
	}
	if !strings.Contains(out, "Internal only") {
		t.Error("rendered email doesn't contain the note's own content")
	}
}

// TestRenderSeverityChangedEmail_ContainsOldAndNewSeverity verifies both
// severities render, distinctly, in the output — a real bug the analogous
// RenderCommentAddedEmail test above caught for a different placeholder,
// so this checks the same class of mistake can't happen here (e.g. the
// old severity accidentally substituted into the new severity's slot).
func TestRenderSeverityChangedEmail_ContainsOldAndNewSeverity(t *testing.T) {
	out := RenderSeverityChangedEmail("CS0023001", "High (P2)", "Low (P4)", "https://x/case", "https://x/comment")
	if !strings.Contains(out, "CS0023001") {
		t.Error("rendered email doesn't contain the case number")
	}
	if !strings.Contains(out, "High (P2)") {
		t.Error("rendered email doesn't contain the old severity")
	}
	if !strings.Contains(out, "Low (P4)") {
		t.Error("rendered email doesn't contain the new severity")
	}
}
