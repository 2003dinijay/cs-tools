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
	"log/slog"
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// runbookLister is the part of the runbook catalog a new incident's work note needs.
type runbookLister interface {
	List(ctx context.Context) ([]domain.Runbook, error)
}

const (
	runbookNotePrefix = "Runbook for this alert: "
	noRunbookNoteText = "No runbook available for this alert."
)

// matchRunbook is the entry whose alert name the subject contains, ignoring case; the longest name wins.
func matchRunbook(runbooks []domain.Runbook, subject string) *domain.Runbook {
	haystack := strings.ToLower(subject)
	var best *domain.Runbook
	bestLen := 0
	for i := range runbooks {
		needle := strings.ToLower(strings.TrimSpace(runbooks[i].AlertName))
		if needle == "" || len(needle) <= bestLen || !strings.Contains(haystack, needle) {
			continue
		}
		best = &runbooks[i]
		bestLen = len(needle)
	}
	return best
}

// runbookNote is the work note for a new incident, or "" when it gets none: a catalog match, or an alert-born incident with no match.
func runbookNote(runbooks []domain.Runbook, req domain.CreateIncidentRequest) string {
	if rb := matchRunbook(runbooks, req.Subject); rb != nil {
		if rb.RunbookURL != nil && strings.TrimSpace(*rb.RunbookURL) != "" {
			return runbookNotePrefix + *rb.RunbookURL
		}
		return noRunbookNoteText
	}
	if req.CorrelationID != nil && strings.TrimSpace(*req.CorrelationID) != "" {
		return noRunbookNoteText
	}
	return ""
}

// withRunbookNote appends the runbook work note after the caller's own; a catalog read failure is logged and leaves the request as it was.
func withRunbookNote(ctx context.Context, req domain.CreateIncidentRequest, catalog runbookLister) domain.CreateIncidentRequest {
	if catalog == nil {
		return req
	}
	runbooks, err := catalog.List(ctx)
	if err != nil {
		slog.WarnContext(ctx, "incident create: runbook catalog unavailable; creating the incident without a runbook note", "error", err)
		return req
	}
	note := runbookNote(runbooks, req)
	if note == "" {
		return req
	}
	if req.WorkNotes != nil && strings.TrimSpace(*req.WorkNotes) != "" {
		note = *req.WorkNotes + "\n\n" + note
	}
	req.WorkNotes = &note
	return req
}

// WithRunbookCatalog sets the runbook catalog new incidents are matched against; applies to the Postgres and the ServiceNow incident services.
func WithRunbookCatalog(svc IncidentService, catalog runbookLister) IncidentService {
	switch s := svc.(type) {
	case *incidentService:
		s.runbooks = catalog
	case *snIncidentService:
		s.runbooks = catalog
	}
	return svc
}
