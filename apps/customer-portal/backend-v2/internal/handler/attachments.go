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
	"errors"
	"log/slog"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/dto"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/middleware"
)

// entityAttachmentClient abstracts the entity-service attachment operations
// used by AttachmentHandler.
type entityAttachmentClient interface {
	CreateAttachment(ctx context.Context, req entity.CreateAttachmentRequest) (entity.CreateAttachmentResponse, error)
	SearchAttachments(ctx context.Context, req entity.SearchAttachmentsRequest) (entity.SearchAttachmentsResponse, error)
	GetAttachmentContent(ctx context.Context, id string) (body []byte, contentType string, err error)
	DeleteAttachment(ctx context.Context, id string) (entity.DeleteAttachmentResponse, error)
	GetAttachment(ctx context.Context, id string) (entity.AttachmentDetails, error)
	// GetAttachmentCase backs both authorizeAttachmentAccess (every route in
	// this file) and DeleteAttachment's closed-case guard — this handler
	// serves no case route of its own (see caseIsClosed in cases.go). It is
	// GET /attachments/{id}/case (id is the attachment id), not the plain
	// GetCase/GET /cases/{id}: the plain route is always DATA_SOURCE-scoped,
	// so it 404s on an attachment whose parent case lives only in ServiceNow
	// under ATTACHMENT_DATA_SOURCE=servicenow (entity-service's own
	// attachmentReadHandler doc comment, internal/server/routes.go) — this
	// check must stay sourced the same way the attachment content itself is.
	GetAttachmentCase(ctx context.Context, id string) (entity.CaseView, error)
	// SearchDeployments backs authorizeAttachmentAccess's deployment branch
	// (see deploymentAttachmentIsVisible below).
	SearchDeployments(ctx context.Context, req entity.SearchDeploymentsRequest) (entity.SearchDeploymentsResponse, error)
	// GetCase backs commentAttachmentIsVisible's access check on the
	// caller-supplied caseID hint — deliberately the plain, general GetCase
	// here (not GetAttachmentCase): this is an ordinary "can the caller see
	// this case" check, unrelated to attachment sourcing.
	GetCase(ctx context.Context, id string) (entity.CaseView, error)
	// SearchComments backs commentAttachmentIsVisible (see below): an
	// attachment pasted inline into a case comment has its own ReferenceID
	// set to the COMMENT's id, not the case's, so GetAttachmentCase can never
	// resolve it (entity-service has no comment->case lookup at all). This
	// searches the case's own comments instead and checks whether the
	// attachment id appears in one of their inlineAttachments.
	SearchComments(ctx context.Context, req entity.SearchCommentsRequest) (entity.SearchCommentsResponse, error)
}

// deploymentAttachmentIsVisible reports whether deploymentID is visible to
// the calling user, the same way DeploymentHandler.deploymentBelongsToProject
// does: entity-service's SearchDeployments is evaluated under the caller's
// own row-level-security scope (deployment has RLS, migration 0176), so a
// result actually matching deploymentID already proves access — no second
// project lookup needed. Unlike deploymentBelongsToProject, this doesn't need
// to know the project up front: SearchDeployments' ids filter (entity-service's
// SearchDeploymentsRequest.IDs) resolves the single deployment directly, which
// is all an attachment's own ReferenceID ever carries.
//
// Checks each returned DeploymentView.ID against deploymentID explicitly,
// rather than trusting a non-empty result alone: the ServiceNow-backed
// SearchDeployments adapter (snDeploymentService, plain DATA_SOURCE=servicenow)
// doesn't forward the ids filter at all (see entity-service's own
// SearchDeploymentsRequest.IDs doc comment — only the Postgres data source
// applies it), so an unfiltered search could return an unrelated deployment
// the caller happens to have access to. A bare len(resp.Deployments) > 0
// check would then authorize against that unrelated deployment instead of
// the one actually being asked about.
func deploymentAttachmentIsVisible(ctx context.Context, client entityAttachmentClient, deploymentID string) (bool, error) {
	resp, err := client.SearchDeployments(ctx, entity.SearchDeploymentsRequest{
		IDs:        []string{deploymentID},
		Pagination: entity.Pagination{Limit: 1},
	})
	if err != nil {
		return false, err
	}
	for _, deployment := range resp.Deployments {
		if deployment.ID == deploymentID {
			return true, nil
		}
	}
	return false, nil
}

// commentAttachmentMaxPages/commentAttachmentPageSize bound
// commentAttachmentIsVisible's comment search: a generous but finite scan,
// not full pagination to the end of arbitrarily long threads. A false
// negative here just means this fallback doesn't find it and the request
// 404s same as before this check existed -- fails closed, never open.
const (
	commentAttachmentPageSize = 50
	commentAttachmentMaxPages = 10
)

// commentAttachmentIsVisible reports whether attachmentID is embedded as an
// inline image in one of caseID's own customer-visible comments, which also
// proves the caller may see caseID at all.
//
// Exists because an attachment pasted inline into a case comment has its own
// ReferenceID set to the ServiceNow COMMENT's (journal entry's) id, not the
// case's -- entity-service has no way to resolve a comment id back to its
// parent case (there is no working GetCommentByID for the ServiceNow data
// source at all, see snCommentSearchService.GetComment's own doc comment), so
// GetAttachmentCase can never succeed for these and always 404s.
//
// Safe despite caseID being caller-supplied: (1) GetCase independently proves
// the caller may see caseID -- the same check every other case-scoped route
// in this backend relies on, not something this function takes on faith; (2)
// attachmentID must then actually turn up inside one of caseID's own
// inlineAttachments, not just be asserted -- finding it there proves it was
// already visible to the caller the moment they fetched the comment thread.
// Filters to type=comment only (never work_note): a customer must not gain
// access to an inline image merely because it happens to be embedded in an
// internal note they were never shown.
func commentAttachmentIsVisible(ctx context.Context, client entityAttachmentClient, caseID, attachmentID string) (bool, error) {
	if _, err := client.GetCase(ctx, caseID); err != nil {
		var apiErr *apierror.Error
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			return false, nil
		}
		return false, err
	}

	commentType := entity.CommentTypeComment
	offset := 0
	for range commentAttachmentMaxPages {
		resp, err := client.SearchComments(ctx, entity.SearchCommentsRequest{
			ReferenceID:   caseID,
			ReferenceType: entity.ReferenceTypeCase,
			Pagination:    entity.Pagination{Limit: commentAttachmentPageSize, Offset: offset},
			Filters:       &entity.CommentFilters{Type: &commentType},
		})
		if err != nil {
			return false, err
		}
		for _, c := range resp.Comments {
			for _, ia := range c.InlineAttachments {
				if ia.ID == attachmentID {
					return true, nil
				}
			}
		}
		if !resp.HasMore || len(resp.Comments) == 0 {
			break
		}
		offset += commentAttachmentPageSize
	}
	return false, nil
}

// authorizeAttachmentAccess verifies the caller may see an attachment's
// underlying entity before GetAttachmentContent/GetAttachment/DeleteAttachment
// act on it, and returns the case view (zero-valued for a non-case reference)
// so DeleteAttachment's own closed-case guard can reuse it instead of a
// second lookup. None of those three routes is nested under a project/case
// path, so unlike every other authorization check in this backend there is
// no path segment to trust — the attachment's own opaque UUID is the only
// thing identifying the resource, and without this check any authenticated
// caller could read or delete any other customer's attachment just by
// guessing or observing its id.
//
// Does NOT reliably branch on attachment.ReferenceType — an earlier revision
// of this function did, and it was wrong in practice, confirmed live: under
// DATA_SOURCE=postgres-servicenow-dual-write, entity-service's
// GetAttachmentByID reports ReferenceType "case" when it reads its own
// Postgres case_attachment table (which hardcodes that value on every row —
// see that repository method's own doc comment) and reports it nil when it
// falls back to the ServiceNow mirror, which it always does for a
// deployment-referenced attachment specifically (case_attachment.case_id has
// a hard FK into "case", so such a row can never exist there in the first
// place — see CreateCaseAttachmentFromServiceNow's own doc comment). So
// ReferenceType is never actually "deployment" on any path this backend can
// observe, live-dual-write or not.
//
// What actually happens here instead: try the case-based check
// (GetAttachmentCase) first, since that is the common case and entity-service
// already scopes it correctly. On a 404-shaped failure — which an
// out-of-scope case and a deployment- or comment-referenced attachment's
// ReferenceID all produce, and this backend cannot tell apart from the
// response alone — fall back to deploymentAttachmentIsVisible (RLS-scoped
// SearchDeployments by id), then, only when the caller supplied caseHint (the
// case they're viewing the attachment from -- see GetAttachmentContent/
// GetAttachment's own doc comments), commentAttachmentIsVisible. Still fails
// closed on every type neither check can confirm (conversation/
// change_request/incident — none of which has a scoped ownership check
// anywhere in this codebase yet — see entity-service's own CLAUDE.md, "Where
// this is actually enforced").
func authorizeAttachmentAccess(ctx context.Context, client entityAttachmentClient, attachment entity.AttachmentDetails, caseHint string) (entity.CaseView, error) {
	if attachment.ReferenceID == "" {
		return entity.CaseView{}, &apierror.Error{StatusCode: http.StatusNotFound}
	}

	// GetAttachmentCase takes the attachment's own id, not ReferenceID --
	// entity-service resolves ReferenceID internally, from the same
	// CaseService this resolves the case from, so it can never disagree with
	// a ReferenceID fetched from a different data source.
	caseView, err := client.GetAttachmentCase(ctx, attachment.ID)
	if err == nil {
		return caseView, nil
	}
	var apiErr *apierror.Error
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusNotFound {
		return entity.CaseView{}, err
	}

	visible, derr := deploymentAttachmentIsVisible(ctx, client, attachment.ReferenceID)
	if derr != nil {
		return entity.CaseView{}, derr
	}
	if visible {
		return entity.CaseView{}, nil
	}

	if caseHint != "" {
		ok, cerr := commentAttachmentIsVisible(ctx, client, caseHint, attachment.ID)
		if cerr != nil {
			return entity.CaseView{}, cerr
		}
		if ok {
			return entity.CaseView{}, nil
		}
	}

	// Nothing resolved it -- report the original GetAttachmentCase error,
	// not the deployment or comment one, since GetAttachmentCase is the
	// common case and its 404 is the more informative of the three to
	// log/map from.
	return entity.CaseView{}, err
}

// AttachmentHandler handles HTTP requests for attachment operations.
type AttachmentHandler struct {
	entity entityAttachmentClient
}

// NewAttachmentHandler creates an AttachmentHandler backed by the given entity client.
func NewAttachmentHandler(entity entityAttachmentClient) *AttachmentHandler {
	return &AttachmentHandler{entity: entity}
}

// CreateAttachment handles POST /attachments.
func (h *AttachmentHandler) CreateAttachment(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	body, ok := readJSONBodyWithLimit(w, r, maxAttachmentBodyBytes)
	if !ok {
		return
	}

	var req entity.CreateAttachmentRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.CreateAttachment(r.Context(), req)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity CreateAttachment failed", "userID", user.UserID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to create attachment.")
		return
	}

	writeJSONValue(w, http.StatusCreated, dto.MapAttachmentCreate(result))
}

// SearchAttachments handles POST /attachments/search.
func (h *AttachmentHandler) SearchAttachments(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var req entity.SearchAttachmentsRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.SearchAttachments(r.Context(), req)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchAttachments failed", "userID", user.UserID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to search attachments.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapSearchAttachments(result))
}

// GetAttachmentContent handles GET /attachments/{id}/content. The response is
// the raw file content, not JSON. Content-Disposition: attachment is always
// set (mirroring entity-service's own XSS mitigation for this endpoint) so
// browsers never render an attachment inline. Rejected with 404 unless the
// caller can see the attachment's own case (see authorizeAttachmentAccess).
//
// Accepts an optional ?caseId= query param: the case the caller is viewing
// this attachment from, used only as a hint for authorizeAttachmentAccess's
// commentAttachmentIsVisible fallback (an inline-comment-image attachment
// cannot otherwise be resolved to a case at all — see that function's doc
// comment). Ignored when absent, malformed, or when the case-based/deployment
// checks already succeed -- never itself a trust boundary, since
// commentAttachmentIsVisible independently re-verifies both the caller's
// access to caseId and that this attachment id genuinely appears in one of
// its comments.
func (h *AttachmentHandler) GetAttachmentContent(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	id := r.PathValue("id")
	if id == "" || !isAttachmentID(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}
	caseHint := r.URL.Query().Get("caseId")
	if caseHint != "" && !uuidRe.MatchString(caseHint) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	attachment, err := h.entity.GetAttachment(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetAttachment failed", "userID", user.UserID, "attachmentID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to download attachment.")
		return
	}
	if _, err := authorizeAttachmentAccess(r.Context(), h.entity, attachment, caseHint); err != nil {
		slog.WarnContext(r.Context(), "rejected attachment access outside caller's scope", "userID", user.UserID, "attachmentID", id)
		mapUpstreamError(w, err, "Failed to download attachment.")
		return
	}

	content, contentType, err := h.entity.GetAttachmentContent(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetAttachmentContent failed", "userID", user.UserID, "attachmentID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to download attachment.")
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", "attachment")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content) // #nosec G705 -- Content-Type set from entity-service's own sanitized value; Content-Disposition forces download, never inline rendering
}

// DeleteAttachment handles DELETE /attachments/{id}. Rejected with 404 unless
// the caller can see the attachment's own case (see authorizeAttachmentAccess),
// and with 400 when that case is closed.
//
// The closed-case check here deliberately does NOT reuse caseIsClosed
// (used by CreateCaseAttachment/PatchCaseAttachment, both nested under an
// already-trusted /cases/{caseId}/... path) — caseIsClosed fails OPEN on an
// entity-service error, which is the right call there since it's a pure
// business-rule check layered on top of an already-authorized request. Here
// the GetAttachmentCase call IS the authorization check (see
// authorizeAttachmentAccess) and must fail closed, so this reuses its
// already-fetched CaseView directly
// instead of a second, separately-failing-open lookup. A previous version of
// this handler only ran its closed-case check when a fetch of the attachment
// succeeded AND its referenceId was non-empty — silently skipping the check
// entirely otherwise, which is the same class of bug this rewrite closes:
// every path now either resolves a real case and checks its state, or denies
// outright.
func (h *AttachmentHandler) DeleteAttachment(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	id := r.PathValue("id")
	if id == "" || !isAttachmentID(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	attachment, err := h.entity.GetAttachment(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetAttachment failed", "userID", user.UserID, "attachmentID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to delete attachment.")
		return
	}
	caseView, err := authorizeAttachmentAccess(r.Context(), h.entity, attachment, "")
	if err != nil {
		slog.WarnContext(r.Context(), "rejected attachment access outside caller's scope", "userID", user.UserID, "attachmentID", id)
		mapUpstreamError(w, err, "Failed to delete attachment.")
		return
	}
	if dto.IsCaseStateClosed(caseView.State) {
		slog.WarnContext(r.Context(), "rejected attachment delete on a closed case", "userID", user.UserID, "attachmentID", id, "caseID", attachment.ReferenceID)
		writeError(w, http.StatusBadRequest, ErrMsgCaseClosedForAttachmentDelete)
		return
	}

	result, err := h.entity.DeleteAttachment(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity DeleteAttachment failed", "userID", user.UserID, "attachmentID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to delete attachment.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapDeleteAttachment(result))
}

// GetAttachment handles GET /attachments/{id} — metadata plus base64-encoded
// content, distinct from GetAttachmentContent's raw binary stream. Rejected
// with 404 unless the caller can see the attachment's own case (see
// authorizeAttachmentAccess). Accepts the same optional ?caseId= hint as
// GetAttachmentContent, for the same reason (see that handler's doc comment).
func (h *AttachmentHandler) GetAttachment(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	id := r.PathValue("id")
	if id == "" || !isAttachmentID(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}
	caseHint := r.URL.Query().Get("caseId")
	if caseHint != "" && !uuidRe.MatchString(caseHint) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	result, err := h.entity.GetAttachment(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetAttachment failed", "userID", user.UserID, "attachmentID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to retrieve attachment.")
		return
	}
	if _, err := authorizeAttachmentAccess(r.Context(), h.entity, result, caseHint); err != nil {
		slog.WarnContext(r.Context(), "rejected attachment access outside caller's scope", "userID", user.UserID, "attachmentID", id)
		mapUpstreamError(w, err, "Failed to retrieve attachment.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapAttachmentDetails(result))
}
