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
	"html"
	"regexp"
	"strings"
)

// Comment bodies in the case journal are not plain text. They came from
// ServiceNow, which stores them as fragments of HTML wrapped in its own
// [code]...[/code] markers:
//
//	[code]<br><b> <u>Description</u> </b><br><p>Priority was updated</p>[/code]
//
// 25,298 of the 47,109 customer-visible comments on staging carry markup like
// this. Posting it verbatim puts raw tags on a page the customer reads, so it
// is converted to the Markdown a GitHub comment is actually written in.
//
// CONVERTED, NOT STRIPPED. Deleting the tags would run the description and the
// heading together into one line, losing the paragraph breaks that make a long
// comment readable. The structural tags become their Markdown equivalents and
// everything else is dropped.
var (
	snCodeWrapper = regexp.MustCompile(`(?is)\[/?code\]`)
	snBreak       = regexp.MustCompile(`(?i)<br\s*/?>`)
	snParaClose   = regexp.MustCompile(`(?i)</p\s*>|</div\s*>`)
	snBold        = regexp.MustCompile(`(?is)<(b|strong)\s*>(.*?)</(b|strong)\s*>`)
	snItalic      = regexp.MustCompile(`(?is)<(i|em)\s*>(.*?)</(i|em)\s*>`)
	snImage       = regexp.MustCompile(`(?is)<img[^>]*?\ssrc=["']([^"']+)["'][^>]*>`)
	snAnyTag      = regexp.MustCompile(`(?s)<[^>]*>`)
	snBlankRuns   = regexp.MustCompile(`\n{3,}`)
	snTrailWS     = regexp.MustCompile(`[ \t]+\n`)
)

// snMetaHeader matches the metadata line ServiceNow puts at the top of a
// journal entry, e.g. "Additional comments - Nimal Perera (Work notes)".
var snMetaHeader = regexp.MustCompile(`^[^\n]*\((?:Work notes|Additional comments|GitHub Comment|GitHub Actions)\)[^\n]*\n`)

// snToMarkdown converts a ServiceNow comment body into GitHub Markdown.
//
// A body that carries no markup is returned unchanged apart from trimming, so
// a comment typed in the portal today is not reformatted on its way out.
func snToMarkdown(s string) string {
	if s == "" {
		return ""
	}

	s = snCodeWrapper.ReplaceAllString(s, "")
	// ServiceNow's first line is the entry's own metadata header, not content.
	// The workflow dropped it unconditionally; we drop it only when it looks
	// like a header, so a one-line comment is never swallowed.
	s = snMetaHeader.ReplaceAllString(s, "")
	// Underline has no Markdown equivalent and bold is the closest intent;
	// emitting <u> would leave a tag on the page, which is what we are fixing.
	s = regexp.MustCompile(`(?is)<u\s*>(.*?)</u\s*>`).ReplaceAllString(s, "$1")
	// The inner text is trimmed before the markers go on: ServiceNow writes
	// "<b> <u>Description</u> </b>", and "** Description **" is not bold in
	// Markdown -- a space next to the marker stops it binding, so it renders
	// as literal asterisks.
	s = emphasise(s, snBold, "**")
	s = emphasise(s, snItalic, "_")
	// An <img> carries the only copy of an attachment's URL. Stripping the tag
	// with everything else threw the link away and left the reader with a gap
	// where a screenshot had been; the workflow kept the URL as text, so we do.
	s = snImage.ReplaceAllString(s, "$1")
	s = snParaClose.ReplaceAllString(s, "\n\n")
	s = snBreak.ReplaceAllString(s, "\n")
	// Whatever is left is presentational: opening <p>, <span style=…>, tables.
	s = snAnyTag.ReplaceAllString(s, "")

	// After the tags are gone, &amp; and friends are just text.
	s = html.UnescapeString(s)

	// Bold markers can end up wrapping only spaces once the tags around them
	// are removed ("**  **"), which renders as literal asterisks.
	s = regexp.MustCompile(`\*\*\s*\*\*`).ReplaceAllString(s, "")
	s = snTrailWS.ReplaceAllString(s, "\n")
	s = snBlankRuns.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

// emphasise wraps the inner text of each match in marker, trimming the
// whitespace that would otherwise stop the marker binding.
func emphasise(s string, re *regexp.Regexp, marker string) string {
	return re.ReplaceAllStringFunc(s, func(m string) string {
		inner := strings.TrimSpace(re.ReplaceAllString(m, "$2"))
		if inner == "" {
			return ""
		}
		return marker + inner + marker
	})
}
