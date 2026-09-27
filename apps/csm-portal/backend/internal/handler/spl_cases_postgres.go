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

package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/servicenow"
)

// entityCasesClient is the subset of internal/entity.CustomerEntityClient
// this file needs, so this package depends on a small interface rather than
// that package's concrete type — the same shape main.go's customerEntityClient
// already satisfies, so no new client/credentials are needed to construct
// postgresSplCaseClient; see main.go's wiring.
type entityCasesClient interface {
	SearchCases(ctx context.Context, body []byte) ([]byte, error)
	SearchCaseComments(ctx context.Context, caseID string, body []byte) ([]byte, error)
	CreateCaseComment(ctx context.Context, caseID string, body []byte) ([]byte, error)
}

// splAttachmentsInfoClient is the one splCaseClient method
// postgresSplCaseClient does NOT implement itself — see its own doc comment
// for why. Distinct from spl_attachments.go's splAttachmentsClient (that
// one downloads attachment content; this one lists attachment metadata) —
// same upstream, different operations, already separate interfaces before
// this file existed.
type splAttachmentsInfoClient interface {
	GetAttachmentsInfo(ctx context.Context, caseNumber string, offset, limit int) ([]servicenow.AttachmentInfo, error)
}

// postgresSplCaseClient implements splCaseClient for GetCases/GetCaseByNumber/
// GetCommentsAndWorknotes by calling entity-service (Postgres) instead of
// ServiceNow directly — the SPL ServiceNow-removal work's first slice (see
// the transition plan's "Layer 1"). GetAttachmentsInfo is NOT migrated:
// entity-service's attachment table only covers new uploads through a
// different storage system (SFTPGo) and has no path for attachments already
// sitting in ServiceNow — a real backfill problem, not wired up here — so
// that one method still delegates to the wrapped ServiceNow client.
//
// Known, real gap carried over from entity-service itself, not introduced
// here: AccountNumber/AccountName/ProjectKey come back empty for every case
// read through this path. entity-service's own SearchCaseView doc comment
// says plainly: "Populated for the ServiceNow data source only; null
// otherwise." SPL's case list/detail views will show blank Account/Project
// columns for Postgres-sourced cases until entity-service closes that gap
// itself — that's outside this file's/this repo's control.
type postgresSplCaseClient struct {
	entity entityCasesClient
	sn     splAttachmentsInfoClient
}

// NewPostgresSplCaseClient builds a postgresSplCaseClient. entity is
// typically the same *entity.CustomerEntityClient every other CS Portal
// handler already uses (main.go's customerEntityClient) — SPL's Postgres
// data is the same entity-service, not a separate one. sn is the existing
// ServiceNow client, kept only for attachments.
func NewPostgresSplCaseClient(entity entityCasesClient, sn splAttachmentsInfoClient) *postgresSplCaseClient {
	return &postgresSplCaseClient{entity: entity, sn: sn}
}

// caseStateToDisplay/caseStateFromDisplay translate between entity-service's
// domain.CaseState wire values (lowercase snake_case, e.g. "work_in_progress")
// and the six display labels SPL's UI has always used, which are exactly
// ServiceNow's own state labels (e.g. "Work In Progress") — see
// CaseStateCard.tsx/SplCasesPage.tsx on the frontend, which are NOT changing
// as part of this. entity-service's "closed" has no SPL summary card and is
// deliberately not in caseStateFromDisplay (a caller can never ask to filter
// by a label SPL's own UI never offers).
var caseStateToDisplay = map[string]string{
	"open":              "Open",
	"work_in_progress":  "Work In Progress",
	"awaiting_info":     "Awaiting Info",
	"solution_proposed": "Solution Proposed",
	"waiting_on_wso2":   "Waiting on WSO2",
	"reopened":          "Reopened",
	"closed":            "Closed",
}

var caseStateFromDisplay = map[string]string{
	"Open":              "open",
	"Work In Progress":  "work_in_progress",
	"Awaiting Info":     "awaiting_info",
	"Solution Proposed": "solution_proposed",
	"Waiting on WSO2":   "waiting_on_wso2",
	"Reopened":          "reopened",
}

// --- entity-service wire types (this backend's own copy of the JSON
// contract — entity-service is a separate Go module, so its domain types
// can't be imported directly; every existing entity-service caller in this
// codebase already works this way, see internal/entity/customer.go). Field
// names/JSON tags mirror entity-service's internal/domain package exactly. ---

type entityCaseFieldFilter struct {
	Field  string   `json:"field"`
	Op     string   `json:"op"`
	Values []string `json:"values,omitempty"`
}

type entitySearchCasesFilters struct {
	SearchQuery string                  `json:"searchQuery,omitempty"`
	Filters     []entityCaseFieldFilter `json:"filters,omitempty"`
}

type entityPagination struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

type entityCaseSort struct {
	Field string `json:"field"`
	Order string `json:"order"`
}

type entitySearchCasesRequest struct {
	Filters    entitySearchCasesFilters `json:"filters"`
	SortBy     entityCaseSort           `json:"sortBy"`
	Pagination entityPagination         `json:"pagination"`
}

type entityRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type entityUserReference struct {
	ID    *string `json:"id"`
	Email string  `json:"email"`
	Name  string  `json:"name"`
}

// entityAccountRef mirrors entity-service's own domain.AccountRef exactly
// (id/name/type only — corrected from an earlier version of this file that
// invented a "number" field AccountRef does not have; harmless in practice
// since AccountDetails itself is always nil outside the ServiceNow data
// source, but wrong regardless).
type entityAccountRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type entitySearchCaseView struct {
	ID               string               `json:"id"`
	InternalID       string               `json:"internalId"`
	Number           string               `json:"number"`
	CreatedOn        string               `json:"createdOn"`
	CreatedBy        *entityUserReference `json:"createdBy"`
	Subject          *string              `json:"subject"`
	Description      *string              `json:"description"`
	State            *string              `json:"state"`
	Product          *entityRef           `json:"product"`
	Project          *entityRef           `json:"project"`
	ProjectKey       *string              `json:"projectKey"`
	AssignedEngineer *entityUserReference `json:"assignedEngineer"`
	AccountDetails   *entityAccountRef    `json:"account"`
}

type entitySearchCasesResponse struct {
	Cases []entitySearchCaseView `json:"cases"`
	Total int                    `json:"total"`
}

type entityCaseComment struct {
	Type      string               `json:"type"`
	Content   string               `json:"content"`
	CreatedBy *entityUserReference `json:"createdBy"`
	CreatedOn string               `json:"createdOn"`
}

type entitySearchCaseCommentsResponse struct {
	Comments []entityCaseComment `json:"comments"`
	Total    int                 `json:"total"`
}

// entityCreateCommentRequest mirrors entity-service's own
// domain.CreateCaseCommentRequest (POST /cases/{id}/comments — a distinct,
// case-scoped request type from the generic POST /comments one, handled by
// CaseHandler.CreateCaseComment, not CommentHandler.CreateComment). The case
// id travels in the URL path (see CreateCaseComment's ctx.entity.CreateCaseComment
// call), not the body — entity-service rejects a "referenceId"/"referenceType"
// field here with "unknown field" (confirmed against a real running instance).
type entityCreateCommentRequest struct {
	Type    string `json:"type"`
	Content string `json:"content"`
}

// entityCaseCommentDetail mirrors entity-service's domain.CaseCommentDetail
// (the CreateCaseComment response's "comment" object) — a distinct, smaller
// shape from entityCaseComment (the search-comments list item): CreatedBy is
// a plain free-text string here, not a {id,email,name} reference, so this
// can't reuse entityCaseComment (confirmed against a real running instance —
// reusing it fails to unmarshal with "cannot unmarshal string into ...
// createdBy of type handler.entityUserReference").
type entityCaseCommentDetail struct {
	ID        string `json:"id"`
	CreatedOn string `json:"createdOn"`
	CreatedBy string `json:"createdBy"`
}

type entityCreateCommentResponse struct {
	Comment entityCaseCommentDetail `json:"comment"`
}

// deref returns "" for a nil pointer, matching servicenow.CaseDetails's own
// convention of empty-string-not-omitted for a field ServiceNow itself never
// leaves absent.
func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefRefName(r *entityRef) string {
	if r == nil {
		return ""
	}
	return r.Name
}

// toServiceNowTimestamp reformats entity-service's RFC3339 timestamp
// ("2026-09-24T08:13:06Z") into ServiceNow's own bare "YYYY-MM-DD HH:MM:SS"
// shape (no "T", no trailing offset) — the format the frontend's SPL
// components have always assumed for a CaseDetails.OpenedAt/Comment.CreatedOn
// value, e.g. by appending their own "Z" before parsing it as a Date, or by
// splitting on the space to isolate the date. Passing entity-service's own
// RFC3339 string straight through breaks both of those. Falls back to the
// raw string, unchanged, if it doesn't parse as RFC3339.
func toServiceNowTimestamp(rfc3339 string) string {
	t, err := time.Parse(time.RFC3339, rfc3339)
	if err != nil {
		return rfc3339
	}
	return t.UTC().Format("2006-01-02 15:04:05")
}

// toCaseDetails maps one entity-service SearchCaseView onto SPL's existing
// servicenow.CaseDetails wire shape, unchanged from what the frontend has
// always received — see this file's own doc comment on which fields come
// back empty for a Postgres-sourced case.
func toCaseDetails(v entitySearchCaseView) servicenow.CaseDetails {
	state := derefStr(v.State)
	displayState, ok := caseStateToDisplay[state]
	if !ok {
		displayState = state
	}

	assignedTo := ""
	if v.AssignedEngineer != nil {
		assignedTo = v.AssignedEngineer.Name
	}
	openedBy := ""
	if v.CreatedBy != nil {
		openedBy = v.CreatedBy.Name
	}
	// accountNumber has no entity-service equivalent to populate from here
	// (domain.AccountRef carries id/name/type, not a ServiceNow-style
	// number) — always empty on this path, same as AccountDetails itself
	// always being nil outside the ServiceNow data source.
	accountNumber, accountName := "", ""
	if v.AccountDetails != nil {
		accountName = v.AccountDetails.Name
	}

	return servicenow.CaseDetails{
		Number:           v.Number,
		SysID:            v.ID,
		CaseID:           v.InternalID,
		Description:      derefStr(v.Description),
		ShortDescription: derefStr(v.Subject),
		AssignedTo:       assignedTo,
		CaseType:         "case",
		State:            displayState,
		OpenedBy:         openedBy,
		OpenedAt:         toServiceNowTimestamp(v.CreatedOn),
		AccountNumber:    accountNumber,
		AccountName:      accountName,
		ProjectNumber:    derefRefName(v.Project),
		ProjectKey:       derefStr(v.ProjectKey),
		ProductName:      derefRefName(v.Product),
	}
}

func (c *postgresSplCaseClient) searchCases(ctx context.Context, req entitySearchCasesRequest) (entitySearchCasesResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return entitySearchCasesResponse{}, fmt.Errorf("marshal entity-service search request: %w", err)
	}
	raw, err := c.entity.SearchCases(ctx, body)
	if err != nil {
		return entitySearchCasesResponse{}, err
	}
	var resp entitySearchCasesResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return entitySearchCasesResponse{}, fmt.Errorf("unmarshal entity-service search response: %w", err)
	}
	return resp, nil
}

// GetCases implements splCaseClient.
func (c *postgresSplCaseClient) GetCases(ctx context.Context, searchString, stateFilter *string, offset, limit int) (servicenow.CaseDetailsWithCount, error) {
	req := entitySearchCasesRequest{
		SortBy:     entityCaseSort{Field: "createdOn", Order: "desc"},
		Pagination: entityPagination{Limit: limit, Offset: offset},
	}
	if searchString != nil && *searchString != "" {
		req.Filters.SearchQuery = *searchString
	}
	if stateFilter != nil && *stateFilter != "" {
		wireState, ok := caseStateFromDisplay[*stateFilter]
		if !ok {
			// Unknown label -- filter to nothing rather than silently
			// ignoring the caller's stateFilter and returning every case.
			return servicenow.CaseDetailsWithCount{Count: 0, Cases: []servicenow.CaseDetails{}}, nil
		}
		req.Filters.Filters = append(req.Filters.Filters, entityCaseFieldFilter{
			Field: "state", Op: "in", Values: []string{wireState},
		})
	}

	resp, err := c.searchCases(ctx, req)
	if err != nil {
		return servicenow.CaseDetailsWithCount{}, err
	}
	cases := make([]servicenow.CaseDetails, 0, len(resp.Cases))
	for _, v := range resp.Cases {
		cases = append(cases, toCaseDetails(v))
	}
	return servicenow.CaseDetailsWithCount{Count: resp.Total, Cases: cases}, nil
}

// resolveCaseByNumber searches for the single case with this exact number.
// entity-service's GET /cases/{id} only accepts the internal UUID, not
// SPL's case number, so every by-number lookup goes through search instead
// of a direct get — see this file's own doc comment.
func (c *postgresSplCaseClient) resolveCaseByNumber(ctx context.Context, caseNumber string) (entitySearchCaseView, error) {
	resp, err := c.searchCases(ctx, entitySearchCasesRequest{
		Filters:    entitySearchCasesFilters{Filters: []entityCaseFieldFilter{{Field: "number", Op: "eq", Values: []string{caseNumber}}}},
		SortBy:     entityCaseSort{Field: "createdOn", Order: "desc"},
		Pagination: entityPagination{Limit: 1, Offset: 0},
	})
	if err != nil {
		return entitySearchCaseView{}, err
	}
	if len(resp.Cases) == 0 {
		return entitySearchCaseView{}, servicenow.ErrCaseNotFound
	}
	return resp.Cases[0], nil
}

// GetCaseByNumber implements splCaseClient.
func (c *postgresSplCaseClient) GetCaseByNumber(ctx context.Context, caseNumber string) (servicenow.CaseDetails, error) {
	v, err := c.resolveCaseByNumber(ctx, caseNumber)
	if err != nil {
		return servicenow.CaseDetails{}, err
	}
	return toCaseDetails(v), nil
}

// GetCommentsAndWorknotes implements splCaseClient.
func (c *postgresSplCaseClient) GetCommentsAndWorknotes(ctx context.Context, caseNumber string, offset, limit int) (servicenow.CommentsResponse, error) {
	v, err := c.resolveCaseByNumber(ctx, caseNumber)
	if err != nil {
		return servicenow.CommentsResponse{}, err
	}

	body, err := json.Marshal(struct {
		Pagination entityPagination `json:"pagination"`
	}{Pagination: entityPagination{Limit: limit, Offset: offset}})
	if err != nil {
		return servicenow.CommentsResponse{}, fmt.Errorf("marshal entity-service comments request: %w", err)
	}
	raw, err := c.entity.SearchCaseComments(ctx, v.ID, body)
	if err != nil {
		return servicenow.CommentsResponse{}, err
	}
	var resp entitySearchCaseCommentsResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return servicenow.CommentsResponse{}, fmt.Errorf("unmarshal entity-service comments response: %w", err)
	}

	comments := make([]servicenow.Comment, 0, len(resp.Comments))
	for _, cm := range resp.Comments {
		createdBy := ""
		if cm.CreatedBy != nil {
			createdBy = cm.CreatedBy.Name
		}
		// entity-service's "work_note"/"comment" -- capitalized to match
		// what ServiceNow's own sys_journal_field.element ("work_notes" /
		// "comments") was already being displayed as by the frontend.
		commentType := "Comment"
		if cm.Type == "work_note" {
			commentType = "Work note"
		}
		comments = append(comments, servicenow.Comment{
			CreatedOn: toServiceNowTimestamp(cm.CreatedOn),
			Value:     cm.Content,
			CreatedBy: createdBy,
			Type:      commentType,
		})
	}
	return servicenow.CommentsResponse{Total: resp.Total, Comments: comments}, nil
}

// PostWorkNote implements splWorknotesClient by creating a "work_note"-typed
// comment on the case through entity-service (Postgres) — entity-service's
// commentService.CreateComment already exists natively, so this is a
// straight pass-through, the same shape as GetCommentsAndWorknotes' read
// side. Unlike ServiceNow's PostWorkNote, entity-service's create-comment
// path does not itself reject a closed case (no ErrCaseClosed equivalent
// exists there yet) — a real, known gap: posting a work note on a closed
// Postgres-sourced case currently succeeds where the old ServiceNow path
// would have rejected it with "Case is closed."
func (c *postgresSplCaseClient) PostWorkNote(ctx context.Context, caseNumber, worknote, _ string) (servicenow.WorkNoteResponse, error) {
	v, err := c.resolveCaseByNumber(ctx, caseNumber)
	if err != nil {
		return servicenow.WorkNoteResponse{}, err
	}

	body, err := json.Marshal(entityCreateCommentRequest{
		Type:    "work_note",
		Content: worknote,
	})
	if err != nil {
		return servicenow.WorkNoteResponse{}, fmt.Errorf("marshal entity-service create-comment request: %w", err)
	}
	raw, err := c.entity.CreateCaseComment(ctx, v.ID, body)
	if err != nil {
		return servicenow.WorkNoteResponse{}, err
	}
	var resp entityCreateCommentResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return servicenow.WorkNoteResponse{}, fmt.Errorf("unmarshal entity-service create-comment response: %w", err)
	}
	return servicenow.WorkNoteResponse{
		Number:    caseNumber,
		UpdatedOn: toServiceNowTimestamp(resp.Comment.CreatedOn),
	}, nil
}

// GetAttachmentsInfo implements splCaseClient by delegating to the wrapped
// ServiceNow client — see this file's own doc comment for why this one
// method isn't migrated.
func (c *postgresSplCaseClient) GetAttachmentsInfo(ctx context.Context, caseNumber string, offset, limit int) ([]servicenow.AttachmentInfo, error) {
	return c.sn.GetAttachmentsInfo(ctx, caseNumber, offset, limit)
}
