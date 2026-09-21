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

package sweep

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// resolvedInvoice is resolveDueInvoice's result — the "immediate" due
// invoice for a project (the one with the soonest due date, among the
// eligible ones), plus its opportunity's EULA version, which is what
// closure.DecideInvoice/InvoiceSuspendDate need alongside it.
type resolvedInvoice struct {
	ID                 string
	Opportunity        string
	InvoiceDate        time.Time
	DueDate            time.Time
	EULAVersionDecimal float64
}

// excludedInvoiceClassifications mirrors the legacy fetchDueInvoicesByProject
// exclusion list verbatim (PP/CO/TAM — Cloud support type invoices). Real
// sample data hasn't shown any of these codes yet (CL/LS/PS observed
// instead), but that's staging test data, not confirmed representative of
// production — kept exactly as legacy specifies rather than adjusted
// against unverified staging behavior (see decide_invoice.go's sibling
// eligibility note on the same caution).
var excludedInvoiceClassifications = map[string]bool{"PP": true, "CO": true, "TAM": true}

// resolveDueInvoice finds the "immediate" due invoice for a project —
// ported from the legacy ACPInvoiceUtils.fetchDueInvoicesByProject, minus
// the opportunity-stage=Closed-Won check (the new Opportunity API doesn't
// expose a stage field yet — see the epic issue's follow-up request to
// Sajith; deliberately not faked here, this filter is simply not applied
// until that field exists). Returns (nil, nil) when the project has no
// eligible due invoice — a legitimate, common state, not an error.
func resolveDueInvoice(ctx context.Context, reader entityReader, proj project) (*resolvedInvoice, error) {
	linksRaw, err := reader.SearchProjectOpportunityLinks(ctx, []byte(`{"projectId":"`+proj.ID+`"}`))
	if err != nil {
		return nil, fmt.Errorf("search project-opportunity links: %w", err)
	}
	var linksResp searchProjectOpportunityLinksResponse
	if err := json.Unmarshal(linksRaw, &linksResp); err != nil {
		return nil, fmt.Errorf("parse project-opportunity links: %w", err)
	}

	var best *resolvedInvoice

	for _, link := range linksResp.Links {
		if link.Opportunity == nil || link.Opportunity.ID == "" {
			continue
		}

		oppRaw, err := reader.GetOpportunity(ctx, link.Opportunity.ID)
		if err != nil {
			return nil, fmt.Errorf("get opportunity %s: %w", link.Opportunity.ID, err)
		}
		var opp opportunityDTO
		if err := json.Unmarshal(oppRaw, &opp); err != nil {
			return nil, fmt.Errorf("parse opportunity %s: %w", link.Opportunity.ID, err)
		}
		if !eligibleOpportunity(opp) {
			continue
		}
		eulaDecimal, err := parseEULAVersionDecimal(opp.EulaVersionDecimal)
		if err != nil {
			return nil, fmt.Errorf("parse eulaVersionDecimal for opportunity %s: %w", link.Opportunity.ID, err)
		}

		invoicesRaw, err := reader.SearchInvoices(ctx, []byte(`{"opportunityId":"`+link.Opportunity.ID+`"}`))
		if err != nil {
			return nil, fmt.Errorf("search invoices for opportunity %s: %w", link.Opportunity.ID, err)
		}
		var invResp searchInvoicesResponse
		if err := json.Unmarshal(invoicesRaw, &invResp); err != nil {
			return nil, fmt.Errorf("parse invoices for opportunity %s: %w", link.Opportunity.ID, err)
		}

		oppName := ""
		if opp.Name != nil {
			oppName = *opp.Name
		}

		for _, inv := range invResp.Invoices {
			if !eligibleInvoice(inv, proj.StartDate) {
				continue
			}
			dueDate, err := parseInvoiceDate(*inv.InvoicedDueDate)
			if err != nil {
				return nil, fmt.Errorf("parse invoicedDueDate for invoice %s: %w", inv.ID, err)
			}
			invoiceDate := time.Time{}
			if inv.InvoiceDate != nil {
				invoiceDate, err = parseInvoiceDate(*inv.InvoiceDate)
				if err != nil {
					return nil, fmt.Errorf("parse invoiceDate for invoice %s: %w", inv.ID, err)
				}
			}

			candidate := &resolvedInvoice{
				ID:                 inv.ID,
				Opportunity:        oppName,
				InvoiceDate:        invoiceDate,
				DueDate:            dueDate,
				EULAVersionDecimal: eulaDecimal,
			}
			if best == nil || candidate.DueDate.Before(best.DueDate) {
				best = candidate
			}
		}
	}

	return best, nil
}

// eligibleOpportunity mirrors the legacy fetchDueInvoicesByProject
// eligibility check on the opportunity's text EULA field — non-null and
// not "Customer contract". Kept literal (see excludedInvoiceClassifications'
// doc comment) rather than adjusted against unverified staging data.
func eligibleOpportunity(opp opportunityDTO) bool {
	return opp.EulaVersion != nil && *opp.EulaVersion != "Customer contract"
}

// eligibleInvoice mirrors the legacy fetchDueInvoicesByProject invoice
// filters: unpaid, not an excluded classification, not the
// "Auto Created PS" placeholder name, and due on/after the project's start
// date (when both are known). An invoice with no due date at all can't be
// scheduled against, so it's excluded too — the legacy query implicitly
// required this by filtering/sorting on the field directly.
func eligibleInvoice(inv invoiceDTO, projectStart *time.Time) bool {
	if inv.InvoicedDueDate == nil || *inv.InvoicedDueDate == "" {
		return false
	}
	if inv.InvoicedPaidDate != nil && *inv.InvoicedPaidDate != "" {
		return false
	}
	if inv.Classification != nil && excludedInvoiceClassifications[*inv.Classification] {
		return false
	}
	if inv.Name != nil && *inv.Name == "Auto Created PS" {
		return false
	}
	if projectStart != nil {
		due, err := parseInvoiceDate(*inv.InvoicedDueDate)
		if err == nil && due.Before(*projectStart) {
			return false
		}
	}
	return true
}

// parseInvoiceDate parses an Invoice date field — date-only ("2026-09-01"),
// not the full RFC3339 timestamps project.StartDate/EndDate use.
func parseInvoiceDate(s string) (time.Time, error) {
	return time.Parse("2006-01-02", s)
}

// parseEULAVersionDecimal parses Opportunity.EulaVersionDecimal — a string
// on the wire (confirmed via the real API, not a float like its name might
// suggest) — into the float64 closure.DecideInvoice's math needs. nil
// (absent) parses to 0, matching usesGracePeriod's own "eulaVersion <= 0
// means unset" convention rather than erroring on an absent-but-otherwise-
// eligible opportunity.
func parseEULAVersionDecimal(s *string) (float64, error) {
	if s == nil || *s == "" {
		return 0, nil
	}
	return strconv.ParseFloat(*s, 64)
}
