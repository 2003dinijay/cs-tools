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

package entry

import (
	"html"
	"io"
	"strings"

	"github.com/microcosm-cc/bluemonday"
	xhtml "golang.org/x/net/html"
)

// policy is the formatting the "What did you learn?" editor offers, and
// nothing else. The webapp sanitises the same way, but an entry is written by
// one employee and read by every other, so the backend never relies on that.
// Disallowed tags are dropped with their text kept; script and style lose
// their content too. Entries are text only, so <img> goes like any other tag.
var policy = func() *bluemonday.Policy {
	p := bluemonday.NewPolicy()
	p.AllowElements("p", "br", "strong", "em", "u", "ol", "ul", "li")
	p.AllowAttrs("href", "target").OnElements("a")
	p.AllowURLSchemes("http", "https", "mailto", "tel")
	p.AllowRelativeURLs(true)
	return p
}()

// Sanitize returns html with everything outside the editor's formatting
// removed.
func Sanitize(html string) string {
	return policy.Sanitize(html)
}

// PlainText is an entry's text without markup: what search matches and what
// the length limit counts. A paragraph, list item or line break ends with a
// space, so "<p>One</p><p>Two</p>" reads "One Two", the way the webapp's own
// character counter reads it.
func PlainText(sanitized string) string {
	var b strings.Builder
	z := xhtml.NewTokenizer(strings.NewReader(sanitized))
	for {
		switch z.Next() {
		case xhtml.ErrorToken:
			if z.Err() != io.EOF {
				return ""
			}
			return strings.TrimSpace(strings.ReplaceAll(b.String(), " ", " "))
		case xhtml.TextToken:
			b.Write(z.Text())
		case xhtml.EndTagToken:
			if name, _ := z.TagName(); string(name) == "p" || string(name) == "li" {
				b.WriteByte(' ')
			}
		case xhtml.StartTagToken, xhtml.SelfClosingTagToken:
			if name, _ := z.TagName(); string(name) == "br" {
				b.WriteByte(' ')
			}
		}
	}
}

// ForChat converts an entry's sanitised HTML to the subset a Google Chat card
// paragraph understands: <b>, <i>, <u>, <a href> and <br>. Paragraphs end in a
// line break and list items become bullet lines.
func ForChat(sanitized string) string {
	var b strings.Builder
	// Whether each open <a> was written out: one without an href isn't, so
	// its closing tag mustn't be either.
	var links []bool
	z := xhtml.NewTokenizer(strings.NewReader(sanitized))
	for {
		tt := z.Next()
		if tt == xhtml.ErrorToken {
			break
		}
		token := z.Token()
		switch tt {
		case xhtml.TextToken:
			b.WriteString(html.EscapeString(token.Data))
		case xhtml.StartTagToken, xhtml.SelfClosingTagToken:
			switch token.Data {
			case "strong":
				b.WriteString("<b>")
			case "em":
				b.WriteString("<i>")
			case "u":
				b.WriteString("<u>")
			case "br":
				b.WriteString("<br>")
			case "li":
				b.WriteString("• ")
			case "a":
				written := false
				for _, attr := range token.Attr {
					if attr.Key == "href" {
						b.WriteString(`<a href="` + html.EscapeString(attr.Val) + `">`)
						written = true
						break
					}
				}
				links = append(links, written)
			}
		case xhtml.EndTagToken:
			switch token.Data {
			case "strong":
				b.WriteString("</b>")
			case "em":
				b.WriteString("</i>")
			case "u":
				b.WriteString("</u>")
			case "p", "li":
				b.WriteString("<br>")
			case "a":
				if n := len(links); n > 0 {
					if links[n-1] {
						b.WriteString("</a>")
					}
					links = links[:n-1]
				}
			}
		}
	}
	out := strings.TrimSpace(b.String())
	for strings.HasSuffix(out, "<br>") {
		out = strings.TrimSpace(strings.TrimSuffix(out, "<br>"))
	}
	return out
}
