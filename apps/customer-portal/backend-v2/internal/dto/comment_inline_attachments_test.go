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

package dto

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
)

// TestMapSearchComments_EmitsInlineAttachments covers the fields the frontend
// declares on CaseCommentInlineAttachment (features/support/types/attachments.ts)
// and renders in case and conversation threads. entity-service was discarding
// them from the upstream comment payload, so they never reached the portal.
func TestMapSearchComments_EmitsInlineAttachments(t *testing.T) {
	created := time.Date(2026, 8, 18, 9, 0, 0, 0, time.UTC)
	got := MapSearchComments(entity.SearchCommentsResponse{
		Comments: []entity.CommentView{{
			ID:                   "c1",
			Content:              "see screenshot",
			HasInlineAttachments: true,
			InlineAttachments: []entity.InlineAttachment{{
				ID: "a1", FileName: "shot.png", ContentType: "image/png",
				DownloadURL: "https://sn.example/a1.iix", CreatedOn: &created, CreatedBy: "user-1",
			}},
		}},
		Total: 1,
	})

	raw, err := json.Marshal(got.Comments[0])
	if err != nil {
		t.Fatalf("marshal returned error: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}

	if m["hasInlineAttachments"] != true {
		t.Errorf("hasInlineAttachments = %v, want true", m["hasInlineAttachments"])
	}
	list, ok := m["inlineAttachments"].([]any)
	if !ok || len(list) != 1 {
		t.Fatalf("inlineAttachments = %v, want one entry", m["inlineAttachments"])
	}
	a := list[0].(map[string]any)
	for k, want := range map[string]string{
		"id": "a1", "fileName": "shot.png", "contentType": "image/png",
		"downloadUrl": "https://sn.example/a1.iix", "createdBy": "user-1",
	} {
		if a[k] != want {
			t.Errorf("inlineAttachments[0].%s = %v, want %q", k, a[k], want)
		}
	}
}

// TestMapSearchComments_OmitsWhenNoInlineAttachments keeps a plain comment from
// growing an empty array or a false flag — the frontend treats both keys as
// optional, so absent is the correct representation.
func TestMapSearchComments_OmitsWhenNoInlineAttachments(t *testing.T) {
	got := MapSearchComments(entity.SearchCommentsResponse{
		Comments: []entity.CommentView{{ID: "c1", Content: "plain text"}},
	})

	raw, err := json.Marshal(got.Comments[0])
	if err != nil {
		t.Fatalf("marshal returned error: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
	for _, k := range []string{"hasInlineAttachments", "inlineAttachments"} {
		if _, present := m[k]; present {
			t.Errorf("%q present on a comment with no inline images; want omitted", k)
		}
	}
}

// TestMapSearchComments_OmitsUnparseableAttachmentTimestamp is the regression
// guard for the pointer change. entity-service leaves CreatedOn nil when the
// upstream timestamp is missing or unparseable; a value type would have
// serialised Go's zero time as "0001-01-01T00:00:00Z", which reads as a genuine
// date. The key must be absent instead — never a year-one timestamp.
func TestMapSearchComments_OmitsUnparseableAttachmentTimestamp(t *testing.T) {
	got := MapSearchComments(entity.SearchCommentsResponse{
		Comments: []entity.CommentView{{
			ID:                   "c1",
			HasInlineAttachments: true,
			InlineAttachments: []entity.InlineAttachment{{
				ID: "a1", FileName: "shot.png", CreatedOn: nil, CreatedBy: "user-1",
			}},
		}},
	})

	raw, err := json.Marshal(got.Comments[0])
	if err != nil {
		t.Fatalf("marshal returned error: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
	a := m["inlineAttachments"].([]any)[0].(map[string]any)

	if v, present := a["createdOn"]; present && v != nil {
		t.Errorf("createdOn = %v; want the key omitted (or null), never a zero timestamp", v)
	}
	if s, _ := a["createdOn"].(string); strings.HasPrefix(s, "0001-01-01") {
		t.Errorf("createdOn = %q — Go's zero time leaked as a real-looking date", s)
	}
	// The rest of the attachment must still be intact.
	if a["fileName"] != "shot.png" {
		t.Errorf("fileName = %v, wanted the attachment still mapped", a["fileName"])
	}
}
