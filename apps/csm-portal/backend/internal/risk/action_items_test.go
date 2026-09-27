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

package risk

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func actionItemRowCols() []string {
	return []string{"id", "risk_id", "project_sys_id", "account_sys_id", "title", "description", "priority",
		"status", "assigned_to_email", "due_date", "resolution_comment", "resolved_by_email", "resolved_on",
		"created_by_email", "created_on", "updated_on"}
}

func TestUpdateActionItemStatus_RequiresResolutionCommentWhenResolving(t *testing.T) {
	c, mock := newTestClient(t)
	now := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)

	mock.ExpectQuery("SELECT (.+) FROM risk_action_item WHERE id = ?").
		WithArgs(5).
		WillReturnRows(sqlmock.NewRows(actionItemRowCols()).
			AddRow(5, 42, "proj-1", "acct-1", "Fix it", nil, "high", "open", nil, nil, nil, nil, nil, "a@b.com", now, now))

	_, err := c.UpdateActionItemStatus(context.Background(), 5, "resolved", nil, "user@example.com")
	var valErr *ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("expected *ValidationError, got %T: %v", err, err)
	}
	if valErr.Message != "resolutionComment is required when status is 'resolved' or 'cancelled'" {
		t.Errorf("message = %q", valErr.Message)
	}
}

func TestUpdateActionItemStatus_MissingItemIsNotFoundError(t *testing.T) {
	c, mock := newTestClient(t)
	mock.ExpectQuery("SELECT (.+) FROM risk_action_item WHERE id = ?").
		WithArgs(999).
		WillReturnRows(sqlmock.NewRows(actionItemRowCols()))

	_, err := c.UpdateActionItemStatus(context.Background(), 999, "in_progress", nil, "user@example.com")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	var valErr *ValidationError
	if errors.As(err, &valErr) {
		t.Fatalf("expected a plain (non-ValidationError) not-found error, matching the Ballerina source's "+
			"untyped error() for a missing action item lookup — got *ValidationError: %v", err)
	}
	if !errors.Is(err, errRecordNotFound) {
		t.Errorf("expected errRecordNotFound, got %v", err)
	}
}

func TestUpdateActionItem_RejectsEditWhenNotOpenOrInProgress(t *testing.T) {
	c, mock := newTestClient(t)
	now := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)

	mock.ExpectQuery("SELECT (.+) FROM risk_action_item WHERE id = ?").
		WithArgs(5).
		WillReturnRows(sqlmock.NewRows(actionItemRowCols()).
			AddRow(5, 42, "proj-1", "acct-1", "Fix it", nil, "high", "resolved", nil, nil, nil, nil, nil, "a@b.com", now, now))

	_, err := c.UpdateActionItem(context.Background(), 5, UpdateActionItemRequest{Title: "New title", Priority: "low"})
	var valErr *ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("expected *ValidationError, got %T: %v", err, err)
	}
	want := "Cannot edit action item with status 'resolved'. Only 'open' or 'in_progress' items can be edited."
	if valErr.Message != want {
		t.Errorf("message = %q, want %q", valErr.Message, want)
	}
}

func TestCreateActionItemComment_RejectsBlankComment(t *testing.T) {
	c, _ := newTestClient(t)
	_, err := c.CreateActionItemComment(context.Background(), 5, "   ", "user@example.com")
	var valErr *ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("expected *ValidationError, got %T: %v", err, err)
	}
	if valErr.Message != "Comment cannot be empty." {
		t.Errorf("message = %q", valErr.Message)
	}
}

func TestEnrichWithCommentCounts_PopulatesCounts(t *testing.T) {
	c, mock := newTestClient(t)
	now := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)
	items := []RiskActionItem{
		{ID: 1, CreatedOn: civilToString(now), UpdatedOn: civilToString(now)},
		{ID: 2, CreatedOn: civilToString(now), UpdatedOn: civilToString(now)},
	}

	mock.ExpectQuery("SELECT action_item_id, COUNT\\(\\*\\)").
		WithArgs(1, 2).
		WillReturnRows(sqlmock.NewRows([]string{"action_item_id", "comment_count"}).
			AddRow(1, 3))

	enriched, err := c.enrichWithCommentCounts(context.Background(), items)
	if err != nil {
		t.Fatalf("enrichWithCommentCounts returned error: %v", err)
	}
	if enriched[0].CommentCount != 3 {
		t.Errorf("item 1 CommentCount = %d, want 3", enriched[0].CommentCount)
	}
	if enriched[1].CommentCount != 0 {
		t.Errorf("item 2 CommentCount = %d, want 0 (no row returned)", enriched[1].CommentCount)
	}
}

func TestEnrichWithCommentCounts_EmptyInputSkipsQuery(t *testing.T) {
	c, mock := newTestClient(t)
	enriched, err := c.enrichWithCommentCounts(context.Background(), nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(enriched) != 0 {
		t.Errorf("expected empty result, got %v", enriched)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unexpected DB call: %v", err)
	}
}
