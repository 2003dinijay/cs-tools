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

package notify

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/migrationreport"
)

// migrationCaseReportTimezone is the zone the scan's timestamps are
// written in: the one the team reading it works in, and the one
// entity-service takes the age cut-off in.
const migrationCaseReportTimezone = "Asia/Colombo"

// migrationCaseColumns are the scan's table headings, in the order the
// mail has always had them. The last two are the additions the CS team
// asked for; everything before them is unchanged.
var migrationCaseColumns = []string{
	"Case ID", "WSO2 ID", "Created Date", "Last Customer Comment", "Assignee", "WSO2 Product", "Integration CS Team",
	"Sales Region", "Account Manager / Technical Owner",
}

// RenderMigrationCaseReport writes the weekly migration case scan in the
// layout its readers have always received: a short plain introduction,
// then three headed tables with a grey header row and plain borders. No
// banner, colours or branding. caseLinkBase is the support portal's case
// page prefix; when set, each row's WSO2 ID links to <caseLinkBase>/<case
// number>, else it is plain text.
func RenderMigrationCaseReport(r migrationreport.Report, caseLinkBase string) string {
	loc, err := time.LoadLocation(migrationCaseReportTimezone)
	if err != nil {
		loc = time.UTC
	}
	days := strconv.Itoa(r.OlderThanDays)
	var b strings.Builder
	b.WriteString("<html><body>\n")
	fmt.Fprintf(&b, "<p>Hi Team,<br/>The following migration cases have been identified, which are opened for over %s days without a conclusion. Kindly review and take the necessary actions to drive them toward completion and help accelerate a successful migration.</p>\n", days)
	renderMigrationTable(&b, "Migrations Customer Requested to Hold", r.OnHold, caseLinkBase, loc)
	renderMigrationTable(&b, "Migrations Waiting on Accounts Team", r.WaitingOnAccounts, caseLinkBase, loc)
	renderMigrationTable(&b, "Migrations in Progress for Over "+days+" Days That Don't Fall Under Either of the Above Categories", r.InProgress, caseLinkBase, loc)
	b.WriteString("</body></html>\n")
	return b.String()
}

// renderMigrationTable writes one heading and its table. An empty table
// is a heading over its header row alone, as it always was: the reader
// sees that the category was checked and had nothing in it.
func renderMigrationTable(b *strings.Builder, title string, rows []migrationreport.Row, caseLinkBase string, loc *time.Location) {
	fmt.Fprintf(b, "<h3>%s</h3>\n", escapeHTML(title))
	b.WriteString(`<table border="1" cellpadding="5" cellspacing="0" style="border-collapse:collapse;">` + "\n")
	b.WriteString(`<tr style="background-color:#f2f2f2;">`)
	for _, c := range migrationCaseColumns {
		fmt.Fprintf(b, "<th>%s</th>", escapeHTML(c))
	}
	b.WriteString("</tr>\n")
	const stamp = "2006-01-02 15:04:05"
	for _, row := range rows {
		lastComment := ""
		if row.LastCustomerCommentOn != nil {
			lastComment = row.LastCustomerCommentOn.In(loc).Format(stamp)
		}
		owners := strings.Join(nonEmpty(row.AccountManager, row.TechnicalOwner), " / ")
		b.WriteString("<tr>")
		for _, cell := range []string{
			escapeHTML(row.Number),
			migrationCaseLink(caseLinkBase, row),
			escapeHTML(row.CreatedOn.In(loc).Format(stamp)),
			escapeHTML(lastComment),
			escapeHTML(row.AssignedTo),
			escapeHTML(row.Product),
			escapeHTML(row.CsTeam),
			escapeHTML(row.SalesRegion),
			escapeHTML(owners),
		} {
			fmt.Fprintf(b, "<td>%s</td>", cell)
		}
		b.WriteString("</tr>\n")
	}
	b.WriteString("</table><br/>\n")
}

// migrationCaseLink is the row's WSO2 ID, linked to the case page in the
// support portal (<caseLinkBase>/<number>) when a base URL is configured.
// The key falls back to the number so the cell is never blank; the number
// is path-escaped because it is data.
func migrationCaseLink(caseLinkBase string, row migrationreport.Row) string {
	text := row.IssueKey
	if text == "" {
		text = row.Number
	}
	if caseLinkBase == "" || row.Number == "" {
		return escapeHTML(text)
	}
	href := strings.TrimRight(caseLinkBase, "/") + "/" + url.PathEscape(row.Number)
	return fmt.Sprintf(`<a href="%s" target="_blank">%s</a>`, escapeHTML(href), escapeHTML(text))
}

// nonEmpty drops the blank strings so a missing owner leaves no stray
// separator.
func nonEmpty(values ...string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			out = append(out, v)
		}
	}
	return out
}
