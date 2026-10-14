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

package store

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestANewEntryIsNotAnnounced(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	e, err := s.Create(ctx, jane, content(t, "T", "Internal", nil, "<p>x</p>"))
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Get(ctx, e.ID); got.NotifiedAt != nil {
		t.Fatalf("NotifiedAt = %v, want nil", got.NotifiedAt)
	}
}

func TestClaimNotificationOnlyOnce(t *testing.T) {
	s := openTestStore(t)
	s.now = clock(time.Date(2026, 10, 1, 9, 0, 0, 123456789, time.UTC))
	ctx := context.Background()
	e, err := s.Create(ctx, jane, content(t, "T", "Internal", nil, "<p>x</p>"))
	if err != nil {
		t.Fatal(err)
	}

	at, claimed, err := s.ClaimNotification(ctx, e.ID)
	if err != nil || !claimed {
		t.Fatalf("first claim = %v, %v", claimed, err)
	}
	got, _ := s.Get(ctx, e.ID)
	if got.NotifiedAt == nil || !got.NotifiedAt.Equal(at) {
		t.Fatalf("NotifiedAt = %v, want %v", got.NotifiedAt, at)
	}
	if _, claimed, err := s.ClaimNotification(ctx, e.ID); err != nil || claimed {
		t.Fatalf("second claim = %v, %v; want not claimed", claimed, err)
	}
	if _, claimed, err := s.ClaimNotification(ctx, "00000000-0000-4000-8000-000000000000"); err != nil || claimed {
		t.Fatalf("claim of a missing entry = %v, %v; want not claimed", claimed, err)
	}
}

func TestReleaseNotificationUndoesOnlyItsOwnClaim(t *testing.T) {
	s := openTestStore(t)
	s.now = clock(time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC))
	ctx := context.Background()
	e, err := s.Create(ctx, jane, content(t, "T", "Internal", nil, "<p>x</p>"))
	if err != nil {
		t.Fatal(err)
	}
	first, _, _ := s.ClaimNotification(ctx, e.ID)
	if err := s.ReleaseNotification(ctx, e.ID, first); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Get(ctx, e.ID); got.NotifiedAt != nil {
		t.Fatalf("after release NotifiedAt = %v, want nil", got.NotifiedAt)
	}

	// A later claim survives a stale release of the earlier one.
	second, claimed, _ := s.ClaimNotification(ctx, e.ID)
	if !claimed {
		t.Fatal("couldn't claim again after the release")
	}
	if err := s.ReleaseNotification(ctx, e.ID, first); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Get(ctx, e.ID); got.NotifiedAt == nil || !got.NotifiedAt.Equal(second) {
		t.Fatalf("NotifiedAt = %v, want the later claim %v", got.NotifiedAt, second)
	}
}

func TestCheckSchemaNeedsNotifiedAt(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	if _, err := s.db.ExecContext(ctx, "ALTER TABLE til_entry DROP COLUMN notified_at"); err != nil {
		t.Fatal(err)
	}
	err := s.CheckSchema(ctx)
	if err == nil || !strings.Contains(err.Error(), "no notified_at column") || !strings.Contains(err.Error(), "sql/create_til_tables.sql") {
		t.Fatalf("CheckSchema = %v, want it to name the missing column and the script", err)
	}
}

// Running the script again on a table from before notified_at existed adds
// the column, counts the entries already there as announced, and is a no-op
// from then on: an entry left unannounced later stays that way.
func TestSchemaScriptUpgradesAnOlderTable(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	if _, err := s.db.ExecContext(ctx, "ALTER TABLE til_entry DROP COLUMN notified_at"); err != nil {
		t.Fatal(err)
	}
	created := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	if _, err := s.db.ExecContext(ctx,
		"INSERT INTO til_entry (id, title, author_name, author_email, source, content_html, content_text, created_on) "+
			"VALUES ('00000000-0000-4000-8000-000000000001', 'Old', 'Jane', 'jane@example.com', 'Internal', '<p>x</p>', 'x', ?)", created); err != nil {
		t.Fatal(err)
	}

	runSchemaScript(t, s)
	if err := s.CheckSchema(ctx); err != nil {
		t.Fatalf("after the upgrade: %v", err)
	}
	old, err := s.Get(ctx, "00000000-0000-4000-8000-000000000001")
	if err != nil {
		t.Fatal(err)
	}
	if old.NotifiedAt == nil || !old.NotifiedAt.Equal(created) {
		t.Fatalf("existing entry NotifiedAt = %v, want its creation time %v", old.NotifiedAt, created)
	}

	unsent, err := s.Create(ctx, jane, content(t, "New", "Internal", nil, "<p>y</p>"))
	if err != nil {
		t.Fatal(err)
	}
	runSchemaScript(t, s)
	if got, _ := s.Get(ctx, unsent.ID); got.NotifiedAt != nil {
		t.Fatalf("running the script again marked an unannounced entry: NotifiedAt = %v", got.NotifiedAt)
	}
}
