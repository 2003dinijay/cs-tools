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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package service

import (
	"fmt"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// currentUserFilterPlaceholder is the literal `values` entry that a
// createdBy+eq filter must carry to mean "the authenticated caller", mirroring
// the old createdByMe:true request field. There is no way to express "eq this
// literal email" today: a single literal creator email is already covered by
// createdBy+in with one value, so eq is reserved for this placeholder.
const currentUserFilterPlaceholder = "__current_user_email__"

// caseFilterFieldSet is the exact set of CaseFieldFilter.Field values accepted
// by case search. Anything else is rejected outright.
var caseFilterFieldSet = map[string]bool{
	"type": true, "state": true, "severity": true, "engagementType": true,
	"issueType": true, "workState": true, "tag": true, "projectId": true,
	"deploymentId": true, "assignedUserId": true, "createdBy": true,
	"createdOn": true, "updatedOn": true, "closedOn": true, "product": true,
	"projectOnboardingStatus": true, "projectType": true, "integrationCsTeam": true,
	"resolutionNotes": true, "parentId": true,
}

// caseFilterOpSet is the exact set of CaseFieldFilter.Op values accepted by
// case search, independent of field. Field/op compatibility is enforced
// separately in ParseCaseFieldFilters.
var caseFilterOpSet = map[string]bool{
	"eq": true, "in": true, "notIn": true, "isEmpty": true, "isNotEmpty": true,
	"gte": true, "lte": true,
}

// requireCaseFilterValues rejects a filter entry whose op needs a non-empty
// values array but doesn't have one.
func requireCaseFilterValues(f domain.CaseFieldFilter) error {
	if len(f.Values) == 0 {
		return &apierror.ValidationError{Msg: fmt.Sprintf("filters: field %q op %q requires a non-empty values array", f.Field, f.Op)}
	}
	return nil
}

// badCaseFilterCombo reports a field/op combination that is not supported.
func badCaseFilterCombo(f domain.CaseFieldFilter) error {
	return &apierror.ValidationError{Msg: fmt.Sprintf("filters: field %q does not support op %q", f.Field, f.Op)}
}

// parseCaseFilterDate parses a single filter value into a date/time, accepting
// either a full RFC3339 timestamp or a plain YYYY-MM-DD date (interpreted as
// UTC midnight), since callers may reasonably send either for a date-range
// bound.
func parseCaseFilterDate(f domain.CaseFieldFilter, value string) (*time.Time, error) {
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return &t, nil
	}
	if t, err := time.Parse("2006-01-02", value); err == nil {
		return &t, nil
	}
	return nil, &apierror.ValidationError{Msg: fmt.Sprintf("filters: field %q op %q value %q must be an RFC3339 timestamp or YYYY-MM-DD date", f.Field, f.Op, value)}
}

// ParseCaseFieldFilters translates the case-search wire contract's generic
// filter array (domain.CaseFieldFilter) into domain.ParsedCaseFilters, the
// internal named-field representation both CaseService backends (ServiceNow
// and Postgres) and the Postgres repository build their queries/payloads
// from. This is the one place field/op validation and the field→op meaning
// table live; everything downstream keeps its pre-existing, already-verified
// per-field logic, just reading from ParsedCaseFilters instead of the old
// named request fields.
//
// callerEmail/callerEmailErr resolve the authenticated caller's identity
// (via the same JWT helper callers already use elsewhere in this package,
// e.g. case_service.go's prior createdByMe handling): pass a non-empty
// callerEmail when resolution succeeded, or callerEmailErr describing why it
// didn't. Both are only consulted if the array actually contains a
// createdBy+eq current-user filter, so callers can resolve them
// unconditionally without changing behavior for requests that don't need it.
func ParseCaseFieldFilters(filters []domain.CaseFieldFilter, callerEmail string, callerEmailErr error) (domain.ParsedCaseFilters, error) {
	var p domain.ParsedCaseFilters

	for _, f := range filters {
		if !caseFilterFieldSet[f.Field] {
			return domain.ParsedCaseFilters{}, &apierror.ValidationError{Msg: "filters: unsupported field: " + f.Field}
		}
		if !caseFilterOpSet[f.Op] {
			return domain.ParsedCaseFilters{}, &apierror.ValidationError{Msg: "filters: unsupported op: " + f.Op}
		}

		switch f.Field {
		case "type":
			if f.Op != "in" {
				return domain.ParsedCaseFilters{}, badCaseFilterCombo(f)
			}
			if err := requireCaseFilterValues(f); err != nil {
				return domain.ParsedCaseFilters{}, err
			}
			p.Types = append(p.Types, f.Values...)

		case "state":
			if f.Op != "in" {
				return domain.ParsedCaseFilters{}, badCaseFilterCombo(f)
			}
			if err := requireCaseFilterValues(f); err != nil {
				return domain.ParsedCaseFilters{}, err
			}
			for _, v := range f.Values {
				p.States = append(p.States, domain.CaseState(v))
			}

		case "severity":
			if f.Op != "in" {
				return domain.ParsedCaseFilters{}, badCaseFilterCombo(f)
			}
			if err := requireCaseFilterValues(f); err != nil {
				return domain.ParsedCaseFilters{}, err
			}
			for _, v := range f.Values {
				p.Severities = append(p.Severities, domain.CaseSeverity(v))
			}

		case "engagementType":
			if f.Op != "in" {
				return domain.ParsedCaseFilters{}, badCaseFilterCombo(f)
			}
			if err := requireCaseFilterValues(f); err != nil {
				return domain.ParsedCaseFilters{}, err
			}
			for _, v := range f.Values {
				p.EngagementTypes = append(p.EngagementTypes, domain.EngagementType(v))
			}

		case "issueType":
			if f.Op != "in" {
				return domain.ParsedCaseFilters{}, badCaseFilterCombo(f)
			}
			if err := requireCaseFilterValues(f); err != nil {
				return domain.ParsedCaseFilters{}, err
			}
			for _, v := range f.Values {
				p.IssueTypes = append(p.IssueTypes, domain.CaseIssueType(v))
			}

		case "workState":
			if f.Op != "in" {
				return domain.ParsedCaseFilters{}, badCaseFilterCombo(f)
			}
			if err := requireCaseFilterValues(f); err != nil {
				return domain.ParsedCaseFilters{}, err
			}
			for _, v := range f.Values {
				p.WorkStates = append(p.WorkStates, domain.CaseWorkState(v))
			}

		case "tag":
			switch f.Op {
			case "in":
				if err := requireCaseFilterValues(f); err != nil {
					return domain.ParsedCaseFilters{}, err
				}
				p.Tags = append(p.Tags, f.Values...)
			case "notIn":
				if err := requireCaseFilterValues(f); err != nil {
					return domain.ParsedCaseFilters{}, err
				}
				p.ExcludeTags = append(p.ExcludeTags, f.Values...)
			default:
				return domain.ParsedCaseFilters{}, badCaseFilterCombo(f)
			}

		case "projectId":
			if f.Op != "in" {
				return domain.ParsedCaseFilters{}, badCaseFilterCombo(f)
			}
			if err := requireCaseFilterValues(f); err != nil {
				return domain.ParsedCaseFilters{}, err
			}
			p.ProjectIDs = append(p.ProjectIDs, f.Values...)

		case "deploymentId":
			if f.Op != "in" {
				return domain.ParsedCaseFilters{}, badCaseFilterCombo(f)
			}
			if err := requireCaseFilterValues(f); err != nil {
				return domain.ParsedCaseFilters{}, err
			}
			p.DeploymentIDs = append(p.DeploymentIDs, f.Values...)

		case "assignedUserId":
			switch f.Op {
			case "in":
				if err := requireCaseFilterValues(f); err != nil {
					return domain.ParsedCaseFilters{}, err
				}
				p.AssignedUserIDs = append(p.AssignedUserIDs, f.Values...)
			case "isEmpty":
				p.Unassigned = true
			default:
				// isNotEmpty (or anything else) has no prior equivalent: nothing
				// today expresses "assigned to someone specific but no one in
				// particular" -- reject rather than invent new behavior.
				return domain.ParsedCaseFilters{}, badCaseFilterCombo(f)
			}

		case "createdBy":
			switch f.Op {
			case "in":
				if err := requireCaseFilterValues(f); err != nil {
					return domain.ParsedCaseFilters{}, err
				}
				p.CreatedBy = append(p.CreatedBy, f.Values...)
			case "eq":
				if err := requireCaseFilterValues(f); err != nil {
					return domain.ParsedCaseFilters{}, err
				}
				if len(f.Values) != 1 || f.Values[0] != currentUserFilterPlaceholder {
					return domain.ParsedCaseFilters{}, &apierror.ValidationError{
						Msg: fmt.Sprintf("filters: createdBy eq only supports values: [%q] (the current-user placeholder); use op \"in\" for literal creator emails", currentUserFilterPlaceholder),
					}
				}
				if callerEmailErr != nil {
					return domain.ParsedCaseFilters{}, callerEmailErr
				}
				if callerEmail == "" {
					return domain.ParsedCaseFilters{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required for the createdBy current-user filter"}
				}
				p.CreatedByMe = true
			default:
				return domain.ParsedCaseFilters{}, badCaseFilterCombo(f)
			}

		case "createdOn":
			if err := requireCaseFilterValues(f); err != nil {
				return domain.ParsedCaseFilters{}, err
			}
			t, err := parseCaseFilterDate(f, f.Values[0])
			if err != nil {
				return domain.ParsedCaseFilters{}, err
			}
			switch f.Op {
			case "gte":
				p.StartCreatedDate = t
			case "lte":
				p.EndCreatedDate = t
			default:
				return domain.ParsedCaseFilters{}, badCaseFilterCombo(f)
			}

		case "updatedOn":
			if err := requireCaseFilterValues(f); err != nil {
				return domain.ParsedCaseFilters{}, err
			}
			t, err := parseCaseFilterDate(f, f.Values[0])
			if err != nil {
				return domain.ParsedCaseFilters{}, err
			}
			switch f.Op {
			case "gte":
				p.StartUpdatedDate = t
			case "lte":
				p.EndUpdatedDate = t
			default:
				return domain.ParsedCaseFilters{}, badCaseFilterCombo(f)
			}

		case "closedOn":
			if err := requireCaseFilterValues(f); err != nil {
				return domain.ParsedCaseFilters{}, err
			}
			t, err := parseCaseFilterDate(f, f.Values[0])
			if err != nil {
				return domain.ParsedCaseFilters{}, err
			}
			switch f.Op {
			case "gte":
				p.ClosedStartDate = t
			case "lte":
				p.ClosedEndDate = t
			default:
				return domain.ParsedCaseFilters{}, badCaseFilterCombo(f)
			}

		case "product":
			if f.Op != "in" {
				return domain.ParsedCaseFilters{}, badCaseFilterCombo(f)
			}
			if err := requireCaseFilterValues(f); err != nil {
				return domain.ParsedCaseFilters{}, err
			}
			p.ProductNames = append(p.ProductNames, f.Values...)

		case "projectOnboardingStatus":
			if f.Op != "in" {
				return domain.ParsedCaseFilters{}, badCaseFilterCombo(f)
			}
			if err := requireCaseFilterValues(f); err != nil {
				return domain.ParsedCaseFilters{}, err
			}
			p.ProjectOnboardingStatuses = append(p.ProjectOnboardingStatuses, f.Values...)

		case "projectType":
			if f.Op != "in" {
				return domain.ParsedCaseFilters{}, badCaseFilterCombo(f)
			}
			if err := requireCaseFilterValues(f); err != nil {
				return domain.ParsedCaseFilters{}, err
			}
			p.ProjectTypeIDs = append(p.ProjectTypeIDs, f.Values...)

		case "integrationCsTeam":
			if f.Op != "in" {
				return domain.ParsedCaseFilters{}, badCaseFilterCombo(f)
			}
			if err := requireCaseFilterValues(f); err != nil {
				return domain.ParsedCaseFilters{}, err
			}
			p.IntegrationCsTeamIDs = append(p.IntegrationCsTeamIDs, f.Values...)

		case "resolutionNotes":
			// isNotEmpty has no prior equivalent: false and omitted were
			// explicitly documented as identical for resolutionNotesEmpty, so
			// there is no distinct "resolution notes present" SN-layer
			// behavior to translate to -- reject rather than invent one.
			if f.Op != "isEmpty" {
				return domain.ParsedCaseFilters{}, badCaseFilterCombo(f)
			}
			p.ResolutionNotesEmpty = true

		case "parentId":
			if f.Op != "eq" {
				return domain.ParsedCaseFilters{}, badCaseFilterCombo(f)
			}
			if err := requireCaseFilterValues(f); err != nil {
				return domain.ParsedCaseFilters{}, err
			}
			if len(f.Values) != 1 {
				return domain.ParsedCaseFilters{}, &apierror.ValidationError{Msg: "filters: parentId eq requires exactly one value"}
			}
			id := f.Values[0]
			p.ParentID = &id
		}
	}

	return p, nil
}

// resolveCaseFilterCallerEmail resolves the authenticated caller's email from
// the request's forwarded x-user-id-token, for use as ParseCaseFieldFilters'
// callerEmail/callerEmailErr pair. Safe to call unconditionally: the result is
// only consulted by ParseCaseFieldFilters when the filter array actually
// contains a createdBy+eq current-user filter.
func resolveCaseFilterCallerEmail(token string) (string, error) {
	if token == "" {
		return "", &apierror.UnauthorizedError{Msg: "x-user-id-token header is required for the createdBy current-user filter"}
	}
	email, err := emailFromJWT(token)
	if err != nil {
		return "", &apierror.ValidationError{Msg: "x-user-id-token: " + err.Error()}
	}
	return email, nil
}
