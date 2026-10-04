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
	_ "embed"
	"html"
	"strconv"
	"strings"
	"time"
)

// The two outage emails. Moved here from csm-scheduled-tasks, which sent them
// on its tick; they now arrive as outage-events and are sent within seconds.
// entity-service decides and words both, so these only wrap the text.

//go:embed templates/outage_notification.html
var outageNotificationTemplateRaw string

var outageNotificationTemplate = bakeLogo(outageNotificationTemplateRaw)

// OutageNotificationEmailData is what RenderOutageNotificationEmail substitutes.
type OutageNotificationEmailData struct {
	// PhaseWord is Declared / Update / Resolved, for the banner.
	PhaseWord string
	Number    string
	// Message is entity-service's body text, verbatim.
	Message string
	// Link is the outage's CSM portal page; empty renders no link.
	Link string
}

// OutagePhaseWord maps an outage notice kind to its banner word.
func OutagePhaseWord(kind string) string {
	switch kind {
	case "DECLARED":
		return "Declared"
	case "RESOLVED":
		return "Resolved"
	case "UPDATE":
		return "Update"
	default:
		return kind
	}
}

// RenderOutageNotificationEmail wraps the internal-stakeholder notification in
// the house shell. Its body is deliberately thin -- ServiceNow's flow sends one
// fixed sentence per phase -- so the link is what makes it actionable.
func RenderOutageNotificationEmail(d OutageNotificationEmailData) string {
	return strings.NewReplacer(
		"<!-- [PHASE_WORD] -->", escapeHTML(d.PhaseWord),
		"<!-- [NUMBER] -->", escapeHTML(d.Number),
		"<!-- [MESSAGE] -->", escapeHTML(d.Message),
		"<!-- [OUTAGE_LINK] -->", outageLinkHTML(d.Link),
		"<!-- [YEAR] -->", strconv.Itoa(time.Now().Year()),
	).Replace(outageNotificationTemplate)
}

// RenderOutageCommunicationEmail renders the SRE declaration/resolution email:
// entity-service's full plain-text message, escaped then line-broken (in that
// order, so the breaks survive), followed by the link.
//
// The body is escaped, never interpolated: it carries the outage's short
// description, impact and state, all operator-typed free text.
func RenderOutageCommunicationEmail(body, link string) string {
	withBreaks := strings.ReplaceAll(html.EscapeString(body), "\n", "<br>\n")
	return `<div style="font-family:system-ui,-apple-system,Segoe UI,Roboto,sans-serif;font-size:14px;line-height:1.6;color:#17191e">` +
		withBreaks +
		outageLinkHTML(link) +
		`</div>`
}

// outageLinkHTML is the "View outage" line both outage emails carry.
func outageLinkHTML(link string) string {
	if link == "" {
		return ""
	}
	return `<p style="margin:24px 0 0;"><a href="` + escapeHTML(link) +
		`" style="font-size:14px; font-weight:600; color:rgb(71,96,146); text-decoration:underline;">View outage in the CSM portal</a></p>`
}
