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

package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"golang.org/x/sync/errgroup"
)

// parentRefTypeCase is the CaseNumberRef.Type value for a parent that is itself a case.
// Addressable because CaseNumberRef.Type is an optional (pointer) field.
var parentRefTypeCase = "case"

// CaseRepository defines the persistence operations for the case entity,
// split across work_item (migration 000016, fields common to every
// work_item type) and "case" (migration 000018, a shared-PK extension
// carrying case-specific fields -- "case".id IS work_item.id).
type CaseRepository interface {
	// CreateCase inserts a new case row (both work_item and "case").
	CreateCase(ctx context.Context, req domain.CreateCaseRequest) (domain.Case, error)
	// GetCaseByID returns the enriched case view for the given UUID, or a
	// NotFoundError if no matching row exists.
	GetCaseByID(ctx context.Context, id string) (domain.CaseView, error)
	// SearchCases returns a filtered, paginated slice of enriched case views
	// together with the total count of matching rows before pagination.
	// COUNT and SELECT are executed concurrently on separate pool connections.
	SearchCases(ctx context.Context, req domain.SearchCasesRequest) ([]domain.SearchCaseView, int, error)
	// CreateCaseComment inserts a new comment row for the given case.
	CreateCaseComment(ctx context.Context, req domain.CreateCaseCommentRequest) (domain.CaseComment, error)
	// SearchCaseComments returns a paginated slice of comments for the given case
	// together with the total count of matching rows before pagination.
	SearchCaseComments(ctx context.Context, req domain.SearchCaseCommentsRequest) ([]domain.CaseComment, int, error)
	// UpdateCase updates the state and/or priority of the case identified by
	// req.ID. "case".closed_on is set to NOW() when transitioning to closed.
	//
	// previousSeverity is the case's severity as it stood immediately before
	// this update. When req.Severity is nil, severity can't have changed at
	// all, so this just equals the returned domain.Case.Severity — no extra
	// work needed. When req.Severity is set, this runs inside a transaction
	// that locks the row (SELECT ... FOR UPDATE) before reading its prior
	// severity and applying the update, so previousSeverity is accurate even
	// under a concurrent update to the same case: a plain separate
	// read-then-write (what this used to do) could either miss a genuine
	// LOW-severity-boundary crossing or double-detect one, depending on how
	// two concurrent updates interleave — see caseService.
	// detectBillableStatusChange, the sole caller that needs this value, and
	// the CodeRabbit finding on PR #1683 this fixes.
	//
	// Returns a NotFoundError if no matching row exists.
	UpdateCase(ctx context.Context, req domain.UpdateCaseRequest) (c domain.Case, previousSeverity domain.CaseSeverity, err error)
	// CreateCaseAttachment inserts a new attachment metadata row for the case
	// identified by req.ReferenceID. req.StorageKey must be non-nil: this data
	// source stores file bytes externally in SFTPGo, never inline in Postgres.
	// Returns a ValidationError if req.ReferenceID does not match an existing case.
	CreateCaseAttachment(ctx context.Context, req domain.CreateAttachmentRequest) (domain.Attachment, error)
	// SearchCaseAttachments returns a paginated slice of attachments for the given
	// case, most recently created first, together with the total matching count.
	SearchCaseAttachments(ctx context.Context, caseID string, pagination domain.Pagination) ([]domain.Attachment, int, error)
	// GetCaseAttachmentByID returns the attachment identified by id.
	// Returns a NotFoundError if no matching row exists.
	GetCaseAttachmentByID(ctx context.Context, id string) (domain.Attachment, error)
	// DeleteCaseAttachment permanently removes the attachment metadata row
	// identified by id. It does not delete the backing SFTPGo file -- that
	// remains the downstream CSM backend's responsibility.
	// Returns a NotFoundError if no matching row exists.
	DeleteCaseAttachment(ctx context.Context, id string) error
	// UpdateCaseAttachmentName renames the attachment identified by id and
	// records who did it. Returns a NotFoundError if no matching row exists.
	UpdateCaseAttachmentName(ctx context.Context, id, name, updatedBy string) (updatedOn time.Time, err error)
	// ConfirmCaseAttachment atomically transitions the attachment identified
	// by id from status 'pending' to 'complete'. The WHERE clause's
	// "AND status = 'pending'" guard is the concurrency safety net: if two
	// confirm calls race, only one affects a row. Returns a ConflictError
	// (not a silent no-op) if the row is not currently 'pending' -- including
	// when it doesn't exist, since by the time this is called the caller has
	// already resolved the row via GetCaseAttachmentByID and any mismatch
	// here means it changed state concurrently.
	ConfirmCaseAttachment(ctx context.Context, id string) (domain.Attachment, error)
	// AddCaseTag finds or creates a tag named label (case-insensitively) and
	// attaches it to the case's underlying work_item, unless it is already
	// attached (idempotent: a second call for an already-attached label
	// returns the existing tag, not an error). callerEmail is recorded as
	// created_by/updated_by; the "no fine-grained ACL beyond authenticated
	// caller" model matches caseRepo's existing convention -- callerEmail
	// is threaded down to this layer against a future authorization
	// decision, not checked here yet. Returns a ValidationError if caseID
	// does not reference an existing case.
	AddCaseTag(ctx context.Context, caseID, label, callerEmail string) (domain.Tag, error)
	// RemoveCaseTag detaches the tag identified by tagID from the case
	// identified by caseID. Returns a NotFoundError if that pairing does
	// not exist (the tag might exist but not be on this case, or not exist
	// at all -- both are "not found" from the caller's perspective).
	RemoveCaseTag(ctx context.Context, caseID, tagID, callerEmail string) error
	// SearchTags returns tags (not scoped to any case) whose name matches
	// searchQuery case-insensitively (all tags when searchQuery is empty),
	// most recently created first, capped at limit. callerEmail is threaded
	// down for the same future-authorization reason as AddCaseTag; tags are
	// global vocabulary with no per-case or per-caller scope today.
	SearchTags(ctx context.Context, searchQuery, callerEmail string, limit int) ([]domain.Tag, error)
	// SetCaseWatchList replaces the case's watch list (work_item_watcher
	// rows keyed by the case's own id, which is also its work_item id)
	// wholesale with userIDs, and bumps the case's underlying work_item
	// row's updated_on/updated_by the same way every other UpdateCase
	// branch does -- callerEmail is that updated_by. Returns the resolved
	// watcher list and the new updated_on. Returns a NotFoundError if
	// caseID does not exist; a ValidationError if any userID does not
	// exist.
	SetCaseWatchList(ctx context.Context, caseID string, userIDs []string, callerEmail string) ([]domain.WatchListUser, time.Time, error)
}

type caseRepo struct {
	db *pgxpool.Pool
}

// NewCaseRepository constructs a CaseRepository backed by the given connection pool.
func NewCaseRepository(db *pgxpool.Pool) CaseRepository {
	return &caseRepo{db: db}
}

// CreateCase implements CaseRepository.
// CreateCase implements CaseRepository.
//
// STILL BROKEN, DELIBERATELY NOT FIXED HERE: unlike every other method in
// this file, this one can't be repaired with a table/column rename alone.
// work_item.number and work_item.wso2_id are both UNIQUE with no DB default
// and no backing sequence anywhere in migrations/ (CLAUDE.md documents the
// intended design -- "generated from dedicated sequences via column
// defaults" -- but no CREATE SEQUENCE for either one was ever actually
// added), so something has to generate them on every insert, and the exact
// format is undefined (ServiceNow's own case numbers look like "CS0023001",
// but that's not proven to be the intended Postgres-native format either).
// Explicitly deferred per product decision rather than guessed at -- see
// this repository's own package doc / CLAUDE.md for the options considered.
// The query below still references the nonexistent "cases" table (the same
// class of bug this file's other methods had) and will fail at runtime.
func (r *caseRepo) CreateCase(ctx context.Context, req domain.CreateCaseRequest) (domain.Case, error) {
	const query = `
		INSERT INTO cases (
			created_by, project_id, deployment_id, deployed_product_id,
			type, subject, description, severity, issue_type, state
		)
		VALUES (
			$1, $2, $3, $4,
			$5::case_type_enum, $6, $7,
			$8::case_severity_enum, $9::case_issue_type_enum,
			'open'::case_state_enum
		)
		RETURNING id, number, internal_id, created_by, project_id, deployment_id, deployed_product_id,
		          subject, description, severity, issue_type, state, created_at, updated_at, closed_at`

	var c domain.Case
	err := r.db.QueryRow(ctx, query,
		req.CreatedBy, req.ProjectID, req.DeploymentID, req.DeployedProductID,
		req.Type, req.Subject, req.Description, string(req.Severity), string(req.IssueType),
	).Scan(
		&c.ID, &c.Number, &c.InternalID, &c.CreatedBy,
		&c.ProjectID, &c.DeploymentID, &c.DeployedProductID,
		&c.Subject, &c.Description, &c.Severity, &c.IssueType, &c.State,
		&c.CreatedOn, &c.UpdatedOn, &c.ClosedOn,
	)
	if err != nil {
		if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23503": // foreign_key_violation — one of the referenced IDs does not exist
				return domain.Case{}, &apierror.ValidationError{Msg: "one or more referenced IDs do not exist: " + pgErr.Detail}
			case "P0001": // raise_exception from integrity triggers (deployment/project, deployed_product/deployment, catastrophic priority)
				return domain.Case{}, &apierror.ValidationError{Msg: pgErr.Message}
			}
		}
		return domain.Case{}, fmt.Errorf("create case: %w", err)
	}
	return c, nil
}

// GetCaseByID implements CaseRepository.
//
// case.id IS work_item.id (a shared-PK extension, migration 000018) -- the
// case-specific fields (severity/issue_type/state/work_state/closed_on) come
// from "case", everything else (number, subject, created_on/updated_on,
// created_by, project/deployment/deployed-product/account ids, assignee,
// parent) from work_item, since those are common to every work_item type,
// not just cases.
func (r *caseRepo) GetCaseByID(ctx context.Context, id string) (domain.CaseView, error) {
	var cv domain.CaseView
	var (
		aeID, aeName           *string
		pcID, pcNum, pcType    *string
		rcID, rcNum            *string
		accountID, accountName *string
		workState              *string
		description            *string
		depID, depName         string
		dpID, dpDisplayName    string
		prodID, prodName       string
		creatorEmail           string
		creatorID, creatorName *string
	)
	err := r.db.QueryRow(ctx,
		`SELECT wi.id, wi.number, wi.wso2_id,
		        wi.description, c.severity, c.issue_type, c.state, c.work_state,
		        wi.created_on, wi.updated_on, c.closed_on,
		        wi.subject,
		        wi.created_by, creator.id, COALESCE(creator.name, NULLIF(TRIM(CONCAT_WS(' ', creator.first_name, creator.last_name)), '')),
		        p.id, p.name,
		        d.id, d.name,
		        dp.id, prod.name || COALESCE(' ' || pv.version, ''),
		        prod.id, prod.name,
		        a.id, a.name,
		        ae.id, COALESCE(ae.name, NULLIF(TRIM(CONCAT_WS(' ', ae.first_name, ae.last_name)), '')),
		        pw.id, pw.number, pw.type::TEXT,
		        rc_wi.id, rc_wi.number
		 FROM work_item wi
		 JOIN "case" c ON c.id = wi.id
		 LEFT JOIN "user" creator ON LOWER(creator.email) = LOWER(wi.created_by)
		 JOIN project p ON p.id = wi.project_id
		 LEFT JOIN account a ON a.id = wi.account_id
		 JOIN deployment d ON d.id = wi.deployment_id
		 JOIN deployed_product dp ON dp.id = wi.deployed_product_id
		 JOIN product prod ON prod.id = dp.product_id
		 LEFT JOIN product_version pv ON pv.id = dp.version_id
		 LEFT JOIN "user" ae ON ae.id = wi.assigned_to_id
		 LEFT JOIN work_item pw ON pw.id = wi.parent_id
		 LEFT JOIN "case" rc ON rc.id = c.related_case_id
		 LEFT JOIN work_item rc_wi ON rc_wi.id = rc.id
		 WHERE wi.id = $1 AND wi.type = 'CASE'`, id,
	).Scan(
		&cv.ID, &cv.Number, &cv.InternalID,
		&description, &cv.Severity, &cv.IssueType, &cv.State, &workState,
		&cv.CreatedOn, &cv.UpdatedOn, &cv.ClosedOn,
		&cv.Subject,
		&creatorEmail, &creatorID, &creatorName,
		&cv.ProjectDetails.ID, &cv.ProjectDetails.Name,
		&depID, &depName,
		&dpID, &dpDisplayName,
		&prodID, &prodName,
		&accountID, &accountName,
		&aeID, &aeName,
		&pcID, &pcNum, &pcType,
		&rcID, &rcNum,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.CaseView{}, &apierror.NotFoundError{Msg: "case not found"}
	}
	if err != nil {
		return domain.CaseView{}, fmt.Errorf("get case by id: %w", err)
	}
	// work_item.description (migration 000035) has no NOT NULL constraint,
	// unlike subject; CaseView.Description is a required (non-pointer)
	// string, so a NULL column becomes "" rather than left unset.
	if description != nil {
		cv.Description = *description
	}
	// wi.type = 'CASE' is the query's own WHERE clause, so hardcoding this
	// is accurate, not a guess -- cheaper than adding another SELECT/Scan
	// column for a value the query already guarantees.
	caseType := "case"
	cv.Type = &caseType
	cv.DeploymentDetails = &domain.EntityRef{ID: depID, Name: depName}
	cv.DeployedProductDetails = &domain.DeployedProductRef{
		ID:          &dpID,
		DisplayName: &dpDisplayName,
		// The catalogue product the deployed instance was created from. Both
		// joins are inner, so this data source always has one when it has a
		// deployed product.
		Product: &domain.EntityRef{ID: prodID, Name: prodName},
	}
	if accountID != nil {
		name := ""
		if accountName != nil {
			name = *accountName
		}
		// Type (support tier) has no real column anywhere in the migrations
		// -- account has no tier-like column at all -- so it is left "".
		cv.AccountDetails = &domain.AccountRef{ID: *accountID, Name: name}
	}
	if workState != nil {
		ws := domain.CaseWorkState(*workState)
		cv.WorkState = &ws
	}
	// work_item.created_by is a free-text VARCHAR (an email), not a UUID FK
	// -- resolved by email match against "user" for a real id/name when
	// possible, same pattern as deployment_repo.go's own created_by fix.
	name := ""
	if creatorName != nil {
		name = *creatorName
	}
	id2 := ""
	if creatorID != nil {
		id2 = *creatorID
	}
	cv.CreatedBy = domain.NewUserReference(id2, creatorEmail, name)
	if aeID != nil {
		aName := ""
		if aeName != nil {
			aName = *aeName
		}
		cv.AssignedEngineer = domain.NewUserReference(*aeID, "", aName)
	}
	if pcID != nil {
		// work_item.parent_id (migration 000036) is a generic self-reference
		// across every work_item type, not case-specific -- unlike
		// RelatedCase below, Type reflects the parent's own real type
		// rather than being hardcoded, so a non-case parent isn't
		// misrepresented as one.
		var t *string
		if pcType != nil {
			lower := strings.ToLower(*pcType)
			t = &lower
		}
		cv.ParentCase = &domain.CaseNumberRef{ID: *pcID, Number: *pcNum, Type: t}
	}
	if rcID != nil {
		// "case".related_case_id (migration 000038) is a foreign key into
		// "case" specifically, so a resolved related record is always
		// another case.
		cv.RelatedCase = &domain.CaseNumberRef{ID: *rcID, Number: *rcNum, Type: &parentRefTypeCase}
	}
	watchers, err := fetchCaseWatchers(ctx, r.db, id)
	if err != nil {
		return domain.CaseView{}, err
	}
	cv.WatchList = watchers
	return cv, nil
}

// caseCommentTypeEnum maps a domain.CommentType to its comment_type_enum
// label (migration 000037: APPROVAL_HISTORY, COMMENT, WORK_NOTE) for the
// case-scoped comment methods below. Mirrors commentTypeToEnum in
// internal/service/comment_service.go -- kept as a small local map rather
// than importing the service package (repository must not depend on
// service, per this repo's own layering convention).
var caseCommentTypeEnum = map[domain.CommentType]string{
	domain.CommentTypeComment:  "COMMENT",
	domain.CommentTypeWorkNote: "WORK_NOTE",
	domain.CommentTypeActivity: "APPROVAL_HISTORY",
}

var caseCommentEnumType = map[string]domain.CommentType{
	"COMMENT":          domain.CommentTypeComment,
	"WORK_NOTE":        domain.CommentTypeWorkNote,
	"APPROVAL_HISTORY": domain.CommentTypeActivity,
}

// CreateCaseComment implements CaseRepository.
func (r *caseRepo) CreateCaseComment(ctx context.Context, req domain.CreateCaseCommentRequest) (domain.CaseComment, error) {
	// APPROVAL_HISTORY only ever arises from ServiceNow's own audit trail,
	// never a caller-authored comment -- see commentService.CreateComment's
	// identical restriction in the generic comment path.
	if req.Type == domain.CommentTypeActivity {
		return domain.CaseComment{}, &apierror.ValidationError{Msg: `type "activity" is not writable through this endpoint`}
	}
	typeEnum, ok := caseCommentTypeEnum[req.Type]
	if !ok {
		return domain.CaseComment{}, &apierror.ValidationError{Msg: "type contains invalid value: " + string(req.Type)}
	}

	// comment.work_item_id references work_item(id), which is also
	// "case".id -- INSERT ... SELECT confirms the case exists in the same
	// round trip, RETURNING zero rows (rather than a hard-to-attribute FK
	// error) when it doesn't.
	const query = `
		INSERT INTO comment (id, created_on, created_by, type, work_item_id, content)
		SELECT gen_random_uuid(), NOW(), $1, $2::comment_type_enum, c.id, $4
		FROM "case" c
		WHERE c.id = $3
		RETURNING id, work_item_id, type, content, created_by, created_on`

	var c domain.CaseComment
	var typeRaw, createdByEmail string
	err := r.db.QueryRow(ctx, query,
		req.CreatedBy, typeEnum, req.CaseID, req.Content,
	).Scan(&c.ID, &c.CaseID, &typeRaw, &c.Content, &createdByEmail, &c.CreatedOn)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.CaseComment{}, &apierror.ValidationError{Msg: "case not found: " + req.CaseID}
	}
	if err != nil {
		return domain.CaseComment{}, fmt.Errorf("create case comment: %w", err)
	}
	c.Type = caseCommentEnumType[typeRaw]
	// comment.created_by is a free-text VARCHAR (an email, by this data
	// source's own convention -- see caseService.CreateCaseComment), not a
	// UUID FK, so the reference carries no id here, matching this file's
	// other email-only CreatedBy references (e.g. SearchCaseView.CreatedBy).
	c.CreatedBy = domain.NewUserReference("", createdByEmail, "")
	return c, nil
}

// SearchCaseComments implements CaseRepository.
func (r *caseRepo) SearchCaseComments(ctx context.Context, req domain.SearchCaseCommentsRequest) ([]domain.CaseComment, int, error) {
	args := []any{req.CaseID}
	typeFilter := ""
	if req.Filters != nil && req.Filters.Type != nil {
		args = append(args, caseCommentTypeEnum[*req.Filters.Type])
		typeFilter = fmt.Sprintf(" AND cc.type = $%d::comment_type_enum", len(args))
	}

	countQuery := `SELECT COUNT(*) FROM comment cc WHERE cc.work_item_id = $1` + typeFilter
	// LEFT JOIN "user" by email match: comment.created_by is a free-text
	// VARCHAR (see CreateCaseComment above), not a FK, so a real user id/name
	// is only available when it happens to match a known user's email.
	dataQuery := fmt.Sprintf(`
		SELECT cc.id, cc.work_item_id, cc.type, cc.content, cc.created_by, cc.created_on,
		       u.id, COALESCE(u.name, NULLIF(TRIM(CONCAT_WS(' ', u.first_name, u.last_name)), ''))
		FROM comment cc
		LEFT JOIN "user" u ON LOWER(u.email) = LOWER(cc.created_by)
		WHERE cc.work_item_id = $1%s
		ORDER BY cc.created_on DESC, cc.id
		LIMIT $%d OFFSET $%d`, typeFilter, len(args)+1, len(args)+2)

	dataArgs := append(args, req.Pagination.Limit, req.Pagination.Offset)

	var total int
	var comments []domain.CaseComment

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		if err := r.db.QueryRow(egCtx, countQuery, args...).Scan(&total); err != nil {
			return fmt.Errorf("count case comments: %w", err)
		}
		return nil
	})

	eg.Go(func() error {
		rows, err := r.db.Query(egCtx, dataQuery, dataArgs...)
		if err != nil {
			return fmt.Errorf("query case comments: %w", err)
		}
		defer rows.Close()

		result := make([]domain.CaseComment, 0, req.Pagination.Limit)
		for rows.Next() {
			var c domain.CaseComment
			var typeRaw, authorEmail string
			var authorID, authorName *string
			if err := rows.Scan(
				&c.ID, &c.CaseID, &typeRaw, &c.Content, &authorEmail, &c.CreatedOn,
				&authorID, &authorName,
			); err != nil {
				return fmt.Errorf("scan case comment: %w", err)
			}
			c.Type = caseCommentEnumType[typeRaw]
			if authorID != nil {
				name := ""
				if authorName != nil {
					name = *authorName
				}
				c.CreatedBy = domain.NewUserReference(*authorID, authorEmail, name)
			} else {
				c.CreatedBy = domain.NewUserReference("", authorEmail, "")
			}
			result = append(result, c)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate case comments: %w", err)
		}
		comments = result
		return nil
	})

	if err := eg.Wait(); err != nil {
		return nil, 0, err
	}

	return comments, total, nil
}

// updateCaseQuery is shared by both branches of UpdateCase below. case.id IS
// work_item.id (migration 000018), so this updates both tables in one round
// trip via a CTE: "case" carries state/severity/work_state/closed_on,
// work_item carries everything else (including updated_on, bumped
// unconditionally). The work_item UPDATE's "AND EXISTS (SELECT 1 FROM
// updated_case)" guard means it only actually touches a row when the case
// update did -- so a nonexistent id updates nothing anywhere and the final
// join returns zero rows, not a partial update.
const updateCaseQuery = `
	WITH updated_case AS (
		UPDATE "case"
		SET state      = CASE WHEN $2 <> '' THEN $2::case_state_enum ELSE state END,
		    severity   = CASE WHEN $3 <> '' THEN $3::case_severity_enum ELSE severity END,
		    work_state = CASE WHEN $4 <> '' THEN $4::case_work_state_enum ELSE work_state END,
		    closed_on  = CASE WHEN $2 = 'closed' THEN NOW() WHEN $2 <> '' AND $2 <> 'closed' THEN NULL ELSE closed_on END
		WHERE id = $1
		RETURNING id, severity, issue_type, state, work_state, closed_on
	),
	updated_work_item AS (
		UPDATE work_item
		SET updated_on = NOW()
		WHERE id = $1 AND EXISTS (SELECT 1 FROM updated_case)
		RETURNING id, number, wso2_id, created_by, project_id, deployment_id, deployed_product_id,
		          subject, description, created_on, updated_on
	)
	SELECT uwi.id, uwi.number, uwi.wso2_id, uwi.created_by, uwi.project_id, uwi.deployment_id, uwi.deployed_product_id,
	       uwi.subject, uwi.description, uc.severity, uc.issue_type, uc.state, uc.work_state,
	       uwi.created_on, uwi.updated_on, uc.closed_on
	FROM updated_work_item uwi
	JOIN updated_case uc ON uc.id = uwi.id`

// scanUpdatedCase is shared by both branches of UpdateCase below.
func scanUpdatedCase(row pgx.Row) (domain.Case, error) {
	var c domain.Case
	var workStateRaw *string
	if err := row.Scan(
		&c.ID, &c.Number, &c.InternalID, &c.CreatedBy,
		&c.ProjectID, &c.DeploymentID, &c.DeployedProductID,
		&c.Subject, &c.Description, &c.Severity, &c.IssueType, &c.State, &workStateRaw,
		&c.CreatedOn, &c.UpdatedOn, &c.ClosedOn,
	); err != nil {
		return domain.Case{}, err
	}
	if workStateRaw != nil {
		ws := domain.CaseWorkState(*workStateRaw)
		c.WorkState = &ws
	}
	return c, nil
}

// UpdateCase implements CaseRepository.
func (r *caseRepo) UpdateCase(ctx context.Context, req domain.UpdateCaseRequest) (domain.Case, domain.CaseSeverity, error) {
	state := ""
	if req.State != nil {
		state = string(*req.State)
	}
	severity := ""
	if req.Severity != nil {
		severity = string(*req.Severity)
	}
	workState := ""
	if req.WorkState != nil {
		workState = string(*req.WorkState)
	}

	// req.Severity == nil: severity can't change, so there's nothing to
	// race on — skip the transaction/lock overhead entirely.
	if req.Severity == nil {
		c, err := scanUpdatedCase(r.db.QueryRow(ctx, updateCaseQuery, req.ID, state, severity, workState))
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Case{}, "", &apierror.NotFoundError{Msg: "case not found"}
		}
		if err != nil {
			return domain.Case{}, "", fmt.Errorf("update case: %w", err)
		}
		return c, c.Severity, nil
	}

	// req.Severity != nil: lock the row first so the previous severity this
	// returns is accurate even under a concurrent update to the same case —
	// see this method's own interface doc comment for why that matters.
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return domain.Case{}, "", fmt.Errorf("update case: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var previousSeverity domain.CaseSeverity
	err = tx.QueryRow(ctx, `SELECT severity FROM "case" WHERE id = $1 FOR UPDATE`, req.ID).Scan(&previousSeverity)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Case{}, "", &apierror.NotFoundError{Msg: "case not found"}
	}
	if err != nil {
		return domain.Case{}, "", fmt.Errorf("update case: lock row: %w", err)
	}

	c, err := scanUpdatedCase(tx.QueryRow(ctx, updateCaseQuery, req.ID, state, severity, workState))
	if err != nil {
		return domain.Case{}, "", fmt.Errorf("update case: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Case{}, "", fmt.Errorf("update case: commit tx: %w", err)
	}
	return c, previousSeverity, nil
}

// CreateCaseAttachment implements CaseRepository.
func (r *caseRepo) CreateCaseAttachment(ctx context.Context, req domain.CreateAttachmentRequest) (domain.Attachment, error) {
	const query = `
		INSERT INTO case_attachments (case_id, storage_key, filename, mime_type, size_bytes, description, uploaded_by, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, case_id, storage_key, filename, mime_type, size_bytes, description, uploaded_by, created_at, status`

	var (
		a            domain.Attachment
		storageKey   string
		uploadedByID string
	)
	err := r.db.QueryRow(ctx, query,
		req.ReferenceID, req.StorageKey, req.Name, req.Type, req.SizeBytes, req.Description, req.CreatedBy, req.Status,
	).Scan(
		&a.ID, &a.ReferenceID, &storageKey, &a.Name, &a.Type, &a.SizeBytes, &a.Description,
		&uploadedByID, &a.CreatedOn, &a.Status,
	)
	if err != nil {
		if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23503": // foreign_key_violation — case_id or uploaded_by does not exist
				return domain.Attachment{}, &apierror.ValidationError{Msg: "one or more referenced IDs do not exist: " + pgErr.Detail}
			case "23514": // check_violation — e.g. size_bytes <= 0 or an invalid status
				return domain.Attachment{}, &apierror.ValidationError{Msg: pgErr.Message}
			}
		}
		return domain.Attachment{}, fmt.Errorf("create case attachment: %w", err)
	}
	a.ReferenceType = domain.ReferenceTypeCase
	a.StorageKey = &storageKey
	// The insert returns only the uploader's id; email and display name would
	// need a further join, so the reference carries the id alone, same as
	// CreateCaseComment above.
	a.CreatedBy = domain.NewUserReference(uploadedByID, "", "")
	return a, nil
}

// ConfirmCaseAttachment implements CaseRepository.
func (r *caseRepo) ConfirmCaseAttachment(ctx context.Context, id string) (domain.Attachment, error) {
	const query = `
		UPDATE case_attachments
		SET status = 'complete', updated_at = NOW()
		WHERE id = $1 AND status = 'pending'
		RETURNING id, case_id, storage_key, filename, mime_type, size_bytes, description, uploaded_by, created_at, status`

	var (
		a            domain.Attachment
		storageKey   string
		uploadedByID string
	)
	err := r.db.QueryRow(ctx, query, id).Scan(
		&a.ID, &a.ReferenceID, &storageKey, &a.Name, &a.Type, &a.SizeBytes, &a.Description,
		&uploadedByID, &a.CreatedOn, &a.Status,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Attachment{}, &apierror.ConflictError{Msg: "attachment is not pending (it may already be confirmed, or was confirmed/deleted concurrently)"}
	}
	if err != nil {
		return domain.Attachment{}, fmt.Errorf("confirm case attachment: %w", err)
	}
	a.ReferenceType = domain.ReferenceTypeCase
	a.StorageKey = &storageKey
	a.CreatedBy = domain.NewUserReference(uploadedByID, "", "")
	return a, nil
}

// SearchCaseAttachments implements CaseRepository.
//
// Both queries filter to status = 'complete' -- see the doc comment on
// CaseService.SearchCaseAttachments for why pending (still-uploading) rows
// are excluded from the default list/search response rather than shown with
// a visible status.
func (r *caseRepo) SearchCaseAttachments(ctx context.Context, caseID string, pagination domain.Pagination) ([]domain.Attachment, int, error) {
	const countQuery = `SELECT COUNT(*) FROM case_attachments WHERE case_id = $1 AND status = 'complete'`
	const dataQuery = `
		SELECT ca.id, ca.case_id, ca.filename, ca.mime_type, ca.size_bytes, ca.description,
		       u.id, u.email, TRIM(u.first_name || ' ' || u.last_name) AS full_name,
		       ca.created_at, ca.storage_key, ca.status
		FROM case_attachments ca
		JOIN "user" u ON u.id = ca.uploaded_by
		WHERE ca.case_id = $1 AND ca.status = 'complete'
		ORDER BY ca.created_at DESC, ca.id
		LIMIT $2 OFFSET $3`

	var total int
	var attachments []domain.Attachment

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		if err := r.db.QueryRow(egCtx, countQuery, caseID).Scan(&total); err != nil {
			return fmt.Errorf("count case attachments: %w", err)
		}
		return nil
	})

	eg.Go(func() error {
		rows, err := r.db.Query(egCtx, dataQuery, caseID, pagination.Limit, pagination.Offset)
		if err != nil {
			return fmt.Errorf("query case attachments: %w", err)
		}
		defer rows.Close()

		result := make([]domain.Attachment, 0, pagination.Limit)
		for rows.Next() {
			var (
				a                         domain.Attachment
				uploaderID, uploaderEmail string
				uploaderName, storageKey  string
			)
			if err := rows.Scan(
				&a.ID, &a.ReferenceID, &a.Name, &a.Type, &a.SizeBytes, &a.Description,
				&uploaderID, &uploaderEmail, &uploaderName, &a.CreatedOn, &storageKey, &a.Status,
			); err != nil {
				return fmt.Errorf("scan case attachment: %w", err)
			}
			a.ReferenceType = domain.ReferenceTypeCase
			a.CreatedBy = domain.NewUserReference(uploaderID, uploaderEmail, uploaderName)
			a.StorageKey = &storageKey
			result = append(result, a)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate case attachments: %w", err)
		}
		attachments = result
		return nil
	})

	if err := eg.Wait(); err != nil {
		return nil, 0, err
	}

	return attachments, total, nil
}

// GetCaseAttachmentByID implements CaseRepository.
//
// Unlike SearchCaseAttachments, this intentionally does not filter by status:
// a direct id lookup must return a 'pending' row too, since that's how the
// confirm step resolves the row it's about to transition, and how an
// uploader can check on their own in-flight upload. See the doc comment on
// CaseService.SearchCaseAttachments for the read-path status decision.
func (r *caseRepo) GetCaseAttachmentByID(ctx context.Context, id string) (domain.Attachment, error) {
	const query = `
		SELECT ca.id, ca.case_id, ca.filename, ca.mime_type, ca.size_bytes, ca.description,
		       u.id, u.email, TRIM(u.first_name || ' ' || u.last_name) AS full_name,
		       ca.created_at, ca.storage_key, ca.status
		FROM case_attachments ca
		JOIN "user" u ON u.id = ca.uploaded_by
		WHERE ca.id = $1`

	var (
		a                         domain.Attachment
		uploaderID, uploaderEmail string
		uploaderName, storageKey  string
	)
	err := r.db.QueryRow(ctx, query, id).Scan(
		&a.ID, &a.ReferenceID, &a.Name, &a.Type, &a.SizeBytes, &a.Description,
		&uploaderID, &uploaderEmail, &uploaderName, &a.CreatedOn, &storageKey, &a.Status,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Attachment{}, &apierror.NotFoundError{Msg: "attachment not found"}
	}
	if err != nil {
		return domain.Attachment{}, fmt.Errorf("get case attachment by id: %w", err)
	}
	a.ReferenceType = domain.ReferenceTypeCase
	a.CreatedBy = domain.NewUserReference(uploaderID, uploaderEmail, uploaderName)
	a.StorageKey = &storageKey
	return a, nil
}

// DeleteCaseAttachment implements CaseRepository.
func (r *caseRepo) DeleteCaseAttachment(ctx context.Context, id string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM case_attachments WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete case attachment: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return &apierror.NotFoundError{Msg: "attachment not found"}
	}
	return nil
}

// UpdateCaseAttachmentName implements CaseRepository.
func (r *caseRepo) UpdateCaseAttachmentName(ctx context.Context, id, name, updatedBy string) (time.Time, error) {
	const query = `
		UPDATE case_attachments
		SET filename = $2, updated_at = NOW(), updated_by = $3
		WHERE id = $1
		RETURNING updated_at`

	var updatedOn time.Time
	err := r.db.QueryRow(ctx, query, id, name, updatedBy).Scan(&updatedOn)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, &apierror.NotFoundError{Msg: "attachment not found"}
	}
	if err != nil {
		if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return time.Time{}, &apierror.ValidationError{Msg: "one or more referenced IDs do not exist: " + pgErr.Detail}
		}
		return time.Time{}, fmt.Errorf("update case attachment name: %w", err)
	}
	return updatedOn, nil
}

// pgSortColMap maps domain CaseSortField values to Postgres column expressions.
var pgSortColMap = map[domain.CaseSortField]string{
	domain.CaseSortFieldCreatedOn: "wi.created_on",
	domain.CaseSortFieldUpdatedOn: "wi.updated_on",
	domain.CaseSortFieldSeverity:  "c.severity",
	domain.CaseSortFieldState:     "c.state",
}

// SearchCases implements CaseRepository.
func (r *caseRepo) SearchCases(ctx context.Context, req domain.SearchCasesRequest) ([]domain.SearchCaseView, int, error) {
	filterArgs := []any{}
	argIdx := 1

	where := "WHERE 1=1"

	if len(req.Parsed.Types) > 0 {
		// req.Parsed.Types holds validCaseType's lowercase values
		// ("case", "service_request", ...); work_item_type_enum's labels are
		// uppercase and match 1:1 once uppercased.
		typeStrings := make([]string, len(req.Parsed.Types))
		for i, t := range req.Parsed.Types {
			typeStrings[i] = strings.ToUpper(t)
		}
		where += fmt.Sprintf(" AND wi.type = ANY($%d::work_item_type_enum[])", argIdx)
		filterArgs = append(filterArgs, typeStrings)
		argIdx++
	}

	if len(req.Parsed.ProjectIDs) > 0 {
		where += fmt.Sprintf(" AND wi.project_id = ANY($%d::uuid[])", argIdx)
		filterArgs = append(filterArgs, req.Parsed.ProjectIDs)
		argIdx++
	}

	if len(req.Parsed.DeploymentIDs) > 0 {
		where += fmt.Sprintf(" AND wi.deployment_id = ANY($%d::uuid[])", argIdx)
		filterArgs = append(filterArgs, req.Parsed.DeploymentIDs)
		argIdx++
	}

	// States/Severities/IssueTypes/WorkStates all live on "case" (migration
	// 000018), joined LEFT below since not every matched work_item type has
	// one -- applying any of these filters therefore implicitly narrows the
	// result to case-type rows, since a non-case row's joined c.* columns
	// are always NULL and can never equal a non-NULL filter value.
	if len(req.Parsed.States) > 0 {
		stateStrings := make([]string, len(req.Parsed.States))
		for i, s := range req.Parsed.States {
			stateStrings[i] = string(s)
		}
		where += fmt.Sprintf(" AND c.state = ANY($%d::case_state_enum[])", argIdx)
		filterArgs = append(filterArgs, stateStrings)
		argIdx++
	}

	if len(req.Parsed.Severities) > 0 {
		severityStrings := make([]string, len(req.Parsed.Severities))
		for i, s := range req.Parsed.Severities {
			severityStrings[i] = string(s)
		}
		where += fmt.Sprintf(" AND c.severity = ANY($%d::case_severity_enum[])", argIdx)
		filterArgs = append(filterArgs, severityStrings)
		argIdx++
	}

	if len(req.Parsed.IssueTypes) > 0 {
		issueTypeStrings := make([]string, len(req.Parsed.IssueTypes))
		for i, it := range req.Parsed.IssueTypes {
			issueTypeStrings[i] = string(it)
		}
		where += fmt.Sprintf(" AND c.issue_type = ANY($%d::case_issue_type_enum[])", argIdx)
		filterArgs = append(filterArgs, issueTypeStrings)
		argIdx++
	}

	if len(req.Parsed.EngagementTypes) > 0 {
		// engagement.type (migration 000019) -- a column on the separate
		// engagement work_item-subtype table, joined LEFT below, not a
		// "case" column at all. Applying this filter implicitly narrows the
		// result to engagement-type rows, same reasoning as the case-only
		// filters above.
		engTypeStrings := make([]string, len(req.Parsed.EngagementTypes))
		for i, et := range req.Parsed.EngagementTypes {
			engTypeStrings[i] = string(et)
		}
		where += fmt.Sprintf(" AND eng.type = ANY($%d::engagement_type_enum[])", argIdx)
		filterArgs = append(filterArgs, engTypeStrings)
		argIdx++
	}

	if len(req.Parsed.CreatedBy) > 0 {
		// work_item.created_by is already a free-text email (not a UUID FK
		// needing a join) -- see this file's other created_by fixes.
		where += fmt.Sprintf(" AND wi.created_by = ANY($%d)", argIdx)
		filterArgs = append(filterArgs, req.Parsed.CreatedBy)
		argIdx++
	}

	if len(req.Parsed.WorkStates) > 0 {
		workStateStrings := make([]string, len(req.Parsed.WorkStates))
		for i, ws := range req.Parsed.WorkStates {
			workStateStrings[i] = string(ws)
		}
		where += fmt.Sprintf(" AND c.work_state = ANY($%d::case_work_state_enum[])", argIdx)
		filterArgs = append(filterArgs, workStateStrings)
		argIdx++
	}

	if len(req.Parsed.AssignedUserIDs) > 0 {
		where += fmt.Sprintf(" AND wi.assigned_to_id = ANY($%d::uuid[])", argIdx)
		filterArgs = append(filterArgs, req.Parsed.AssignedUserIDs)
		argIdx++
	}

	if req.Parsed.ClosedStartDate != nil {
		where += fmt.Sprintf(" AND c.closed_on >= $%d", argIdx)
		filterArgs = append(filterArgs, req.Parsed.ClosedStartDate)
		argIdx++
	}
	if req.Parsed.ClosedEndDate != nil {
		where += fmt.Sprintf(" AND c.closed_on <= $%d", argIdx)
		filterArgs = append(filterArgs, req.Parsed.ClosedEndDate)
		argIdx++
	}
	if req.Parsed.StartCreatedDate != nil {
		where += fmt.Sprintf(" AND wi.created_on >= $%d", argIdx)
		filterArgs = append(filterArgs, req.Parsed.StartCreatedDate)
		argIdx++
	}
	if req.Parsed.EndCreatedDate != nil {
		where += fmt.Sprintf(" AND wi.created_on <= $%d", argIdx)
		filterArgs = append(filterArgs, req.Parsed.EndCreatedDate)
		argIdx++
	}
	if req.Parsed.StartUpdatedDate != nil {
		where += fmt.Sprintf(" AND wi.updated_on >= $%d", argIdx)
		filterArgs = append(filterArgs, req.Parsed.StartUpdatedDate)
		argIdx++
	}
	if req.Parsed.EndUpdatedDate != nil {
		where += fmt.Sprintf(" AND wi.updated_on <= $%d", argIdx)
		filterArgs = append(filterArgs, req.Parsed.EndUpdatedDate)
		argIdx++
	}

	if req.Filters.SearchQuery != "" {
		escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(req.Filters.SearchQuery)
		pattern := "%" + escaped + "%"
		where += fmt.Sprintf(` AND (wi.subject ILIKE $%d ESCAPE '\' OR wi.number ILIKE $%d ESCAPE '\' OR wi.wso2_id ILIKE $%d ESCAPE '\')`, argIdx, argIdx, argIdx)
		filterArgs = append(filterArgs, pattern)
		argIdx++
	}

	sortCol := pgSortColMap[req.SortBy.Field]
	sortDir := string(req.SortBy.Order)

	// Deployment/deployed-product/product are LEFT joins here (unlike
	// GetCaseByID's INNER joins): SearchCases can return non-case work_item
	// types too (service_request, engagement, security_report_analysis),
	// and "announcement" rows have no deployment/deployed-product at all
	// (see validateCreateCaseRequest) -- an INNER join would silently drop
	// them from every search result.
	joins := `LEFT JOIN "case" c ON c.id = wi.id
		 LEFT JOIN engagement eng ON eng.id = wi.id
		 JOIN project p ON p.id = wi.project_id
		 LEFT JOIN deployment d ON d.id = wi.deployment_id
		 LEFT JOIN deployed_product dp ON dp.id = wi.deployed_product_id
		 LEFT JOIN product prod ON prod.id = dp.product_id
		 LEFT JOIN product_version pv ON pv.id = dp.version_id
		 LEFT JOIN "user" ae ON ae.id = wi.assigned_to_id
		 LEFT JOIN work_item pw ON pw.id = wi.parent_id
		 LEFT JOIN "case" rc ON rc.id = c.related_case_id
		 LEFT JOIN work_item rc_wi ON rc_wi.id = rc.id`

	countQuery := "SELECT COUNT(*) FROM work_item wi " + joins + " " + where

	dataQuery := fmt.Sprintf(
		`SELECT wi.id, wi.number, wi.wso2_id,
		        wi.type::TEXT, wi.subject, wi.description, c.severity, c.issue_type, c.state,
		        eng.type::TEXT, c.work_state, wi.created_on, wi.updated_on,
		        wi.created_by,
		        p.id, p.name,
		        d.id, d.name,
		        dp.id, prod.name || COALESCE(' ' || pv.version, ''),
		        prod.id, prod.name,
		        ae.id, COALESCE(ae.name, NULLIF(TRIM(CONCAT_WS(' ', ae.first_name, ae.last_name)), '')),
		        pw.id, pw.number,
		        rc_wi.id, rc_wi.number
		 FROM work_item wi %s %s
		 ORDER BY %s %s NULLS LAST, wi.id
		 LIMIT $%d OFFSET $%d`,
		joins, where, sortCol, sortDir, argIdx, argIdx+1,
	)
	dataArgs := append(append([]any{}, filterArgs...), req.Pagination.Limit, req.Pagination.Offset)

	var total int
	var cases []domain.SearchCaseView

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		if err := r.db.QueryRow(egCtx, countQuery, filterArgs...).Scan(&total); err != nil {
			return fmt.Errorf("count cases: %w", err)
		}
		return nil
	})

	eg.Go(func() error {
		rows, err := r.db.Query(egCtx, dataQuery, dataArgs...)
		if err != nil {
			return fmt.Errorf("query cases: %w", err)
		}
		defer rows.Close()

		result := make([]domain.SearchCaseView, 0, req.Pagination.Limit)
		for rows.Next() {
			var cv domain.SearchCaseView
			var caseType, subject string
			var description *string
			var severity, issueType, engagementType, workState, state *string
			var createdAt, updatedAt time.Time
			var aeID, aeName *string
			var pcID, pcNumber *string
			var rcID, rcNumber *string
			var prodID, prodName *string
			var depID, depName *string
			var dpID, dpName *string
			var creatorEmail string
			if err := rows.Scan(
				&cv.ID, &cv.Number, &cv.InternalID,
				&caseType, &subject, &description, &severity, &issueType, &state,
				&engagementType, &workState, &createdAt, &updatedAt,
				&creatorEmail,
				&cv.Project.ID, &cv.Project.Name,
				&depID, &depName,
				&dpID, &dpName,
				&prodID, &prodName,
				&aeID, &aeName,
				&pcID, &pcNumber,
				&rcID, &rcNumber,
			); err != nil {
				return fmt.Errorf("scan case: %w", err)
			}
			if depID != nil {
				cv.Deployment = &domain.EntityRef{ID: *depID, Name: stringOrEmpty(depName)}
			}
			if dpID != nil {
				cv.DeployedProduct = &domain.EntityRef{ID: *dpID, Name: stringOrEmpty(dpName)}
			}
			cv.Type = strings.ToLower(caseType)
			cv.Subject = &subject
			cv.Description = description
			cv.Severity = severity
			cv.IssueType = issueType
			cv.EngagementType = engagementType
			cv.WorkState = workState
			if state != nil {
				cv.State = *state
			}
			cv.CreatedOn = createdAt.UTC().Format(time.RFC3339)
			cv.UpdatedOn = updatedAt.UTC().Format(time.RFC3339)
			if prodID != nil {
				cv.Product = &domain.EntityRef{ID: *prodID, Name: stringOrEmpty(prodName)}
			}
			// The search projection carries only the creator's email, no id or
			// display name, so the canonical reference keeps a null id.
			cv.CreatedBy = domain.NewUserReference("", creatorEmail, "")
			if aeID != nil {
				cv.AssignedEngineer = domain.NewUserReference(*aeID, "", stringOrEmpty(aeName))
			}
			if pcID != nil {
				cv.ParentCase = &domain.EntityRef{ID: *pcID, Name: stringOrEmpty(pcNumber)}
			}
			if rcID != nil {
				cv.RelatedCase = &domain.EntityRef{ID: *rcID, Name: stringOrEmpty(rcNumber)}
			}
			result = append(result, cv)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate cases: %w", err)
		}
		cases = result
		return nil
	})

	if err := eg.Wait(); err != nil {
		return nil, 0, err
	}

	return cases, total, nil
}

// rowsQuerier is satisfied by both *pgxpool.Pool and pgx.Tx, letting
// fetchCaseWatchers run either directly against the pool (GetCaseByID) or
// inside an existing transaction (SetCaseWatchList), without duplicating the
// query.
type rowsQuerier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// fetchCaseWatchers reads the resolved watch list for the case (== work_item)
// identified by caseID, newest-added-last ordering is not available (see
// work_item_watcher's own migration comment: no per-row audit trail to sort
// by), so results are ordered by user_name for a stable, deterministic
// response instead.
func fetchCaseWatchers(ctx context.Context, q rowsQuerier, caseID string) ([]domain.WatchListUser, error) {
	rows, err := q.Query(ctx, `
		SELECT u.id, u.user_name, COALESCE(u.name, CONCAT_WS(' ', u.first_name, u.last_name)), u.email
		FROM work_item_watcher w
		JOIN "user" u ON u.id = w.user_id
		WHERE w.work_item_id = $1
		ORDER BY u.user_name`, caseID)
	if err != nil {
		return nil, fmt.Errorf("query case watch list: %w", err)
	}
	defer rows.Close()

	var watchers []domain.WatchListUser
	for rows.Next() {
		var id, userName, name, email string
		if err := rows.Scan(&id, &userName, &name, &email); err != nil {
			return nil, fmt.Errorf("scan case watcher: %w", err)
		}
		watchers = append(watchers, domain.WatchListUser{
			ID:       id,
			UserName: userName,
			Name:     name,
			Email:    email,
			// User.ID is always null by contract -- see WatchListUser.User's
			// own doc comment ("its id is always null: a watch-list entry is
			// not guaranteed to point at a user record"). Pass "" rather
			// than id so NewUserReference omits it, even though this
			// particular row is known to resolve to a real user.
			User: domain.NewUserReference("", email, name),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate case watch list: %w", err)
	}
	return watchers, nil
}

// SetCaseWatchList implements CaseRepository.
func (r *caseRepo) SetCaseWatchList(ctx context.Context, caseID string, userIDs []string, callerEmail string) ([]domain.WatchListUser, time.Time, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("set case watch list: begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var updatedOn time.Time
	err = tx.QueryRow(ctx, `UPDATE work_item SET updated_on = NOW(), updated_by = $2 WHERE id = $1 RETURNING updated_on`, caseID, callerEmail).Scan(&updatedOn)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, time.Time{}, &apierror.NotFoundError{Msg: "case not found"}
	}
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("touch work_item for watch list update: %w", err)
	}

	if _, err := tx.Exec(ctx, `DELETE FROM work_item_watcher WHERE work_item_id = $1`, caseID); err != nil {
		return nil, time.Time{}, fmt.Errorf("clear case watch list: %w", err)
	}

	for _, userID := range userIDs {
		if _, err := tx.Exec(ctx,
			`INSERT INTO work_item_watcher (id, work_item_id, user_id) VALUES (gen_random_uuid(), $1, $2)`,
			caseID, userID,
		); err != nil {
			if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) && pgErr.Code == "23503" {
				return nil, time.Time{}, &apierror.ValidationError{Msg: "one or more watch list user IDs do not exist: " + pgErr.Detail}
			}
			return nil, time.Time{}, fmt.Errorf("insert case watcher: %w", err)
		}
	}

	watchers, err := fetchCaseWatchers(ctx, tx, caseID)
	if err != nil {
		return nil, time.Time{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, time.Time{}, fmt.Errorf("set case watch list: commit tx: %w", err)
	}

	return watchers, updatedOn, nil
}

// scanTag scans a single (id, name) row into a domain.Tag. tag has no
// "color" column (migration 000021), unlike ServiceNow's label table, so
// Color is always nil for a Postgres-sourced tag.
func scanTag(row interface{ Scan(...any) error }) (domain.Tag, error) {
	var t domain.Tag
	err := row.Scan(&t.ID, &t.Label)
	return t, err
}

// AddCaseTag implements CaseRepository.
func (r *caseRepo) AddCaseTag(ctx context.Context, caseID, label, callerEmail string) (domain.Tag, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return domain.Tag{}, fmt.Errorf("add case tag: begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Find or create the tag by name, case-insensitively. tag.name has no
	// UNIQUE constraint (migration 000021), so this can race with a
	// concurrent AddCaseTag for the same never-before-seen label and
	// produce two rows with the same name -- a cosmetic duplicate (each
	// still links correctly via its own id), not a correctness bug, and not
	// fixable here without a schema change (out of scope).
	tag, err := scanTag(tx.QueryRow(ctx, `SELECT id, name FROM tag WHERE LOWER(name) = LOWER($1) LIMIT 1`, label))
	if errors.Is(err, pgx.ErrNoRows) {
		tag, err = scanTag(tx.QueryRow(ctx,
			`INSERT INTO tag (id, created_on, updated_on, created_by, updated_by, name)
			 VALUES (gen_random_uuid(), NOW(), NOW(), $1, $1, $2)
			 RETURNING id, name`, callerEmail, label))
	}
	if err != nil {
		return domain.Tag{}, fmt.Errorf("find or create tag: %w", err)
	}

	// Idempotent attach: a second AddCaseTag for a label already on this
	// case returns the existing tag rather than erroring or duplicating the
	// work_item_tag row. work_item_tag has no UNIQUE constraint on
	// (work_item_id, tag_id) (migration 000021), so this is guarded with
	// "AND NOT EXISTS" rather than "ON CONFLICT DO NOTHING", which would
	// need one to match against.
	_, err = tx.Exec(ctx, `
		INSERT INTO work_item_tag (id, created_on, updated_on, created_by, updated_by, work_item_id, tag_id)
		SELECT gen_random_uuid(), NOW(), NOW(), $1, $1, wi.id, $3
		FROM work_item wi
		WHERE wi.id = $2
		  AND NOT EXISTS (SELECT 1 FROM work_item_tag wit WHERE wit.work_item_id = $2 AND wit.tag_id = $3)`,
		callerEmail, caseID, tag.ID)
	if err != nil {
		if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return domain.Tag{}, &apierror.ValidationError{Msg: "case not found: " + pgErr.Detail}
		}
		return domain.Tag{}, fmt.Errorf("attach tag to case: %w", err)
	}

	// The INSERT ... SELECT above silently inserts zero rows (rather than
	// erroring) when caseID doesn't reference an existing work_item, since
	// the FROM work_item WHERE wi.id = $2 clause just matches nothing. Check
	// caseID separately so that case is reported as a ValidationError
	// instead of a misleadingly successful response.
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM work_item WHERE id = $1)`, caseID).Scan(&exists); err != nil {
		return domain.Tag{}, fmt.Errorf("verify case exists: %w", err)
	}
	if !exists {
		return domain.Tag{}, &apierror.ValidationError{Msg: "case not found: " + caseID}
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Tag{}, fmt.Errorf("add case tag: commit tx: %w", err)
	}

	return tag, nil
}

// RemoveCaseTag implements CaseRepository.
func (r *caseRepo) RemoveCaseTag(ctx context.Context, caseID, tagID, _ string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM work_item_tag WHERE work_item_id = $1 AND tag_id = $2`, caseID, tagID)
	if err != nil {
		return fmt.Errorf("remove case tag: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return &apierror.NotFoundError{Msg: "tag not found on this case"}
	}
	return nil
}

// SearchTags implements CaseRepository.
func (r *caseRepo) SearchTags(ctx context.Context, searchQuery, _ string, limit int) ([]domain.Tag, error) {
	where := "WHERE 1=1"
	args := []any{}
	if searchQuery != "" {
		escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(searchQuery)
		args = append(args, "%"+escaped+"%")
		where += " AND name ILIKE $1 ESCAPE '\\'"
	}
	args = append(args, limit)

	query := fmt.Sprintf(`SELECT id, name FROM tag %s ORDER BY created_on DESC, id LIMIT $%d`, where, len(args))

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("search tags: %w", err)
	}
	defer rows.Close()

	tags := make([]domain.Tag, 0, limit)
	for rows.Next() {
		t, err := scanTag(rows)
		if err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}
		tags = append(tags, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tags: %w", err)
	}
	return tags, nil
}
