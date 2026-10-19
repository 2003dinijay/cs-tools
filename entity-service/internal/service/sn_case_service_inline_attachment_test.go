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
	"net/http"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// TestSNCaseService_SearchCaseComments_ConvertsInlineAttachmentDownloadURL
// pins a real, reported bug: ServiceNow's inlineAttachments[].downloadUrl
// embeds the same raw sysid as its own id field, but the customer portal's
// GET /attachments/{id}/content only accepts the UUID form -- confirmed live
// via a HAR capture where the identical attachment 404'd under the raw sysid
// and 200'd under the converted UUID. ia.ID was already converted with
// sysidToUUID; DownloadURL was passed through unconverted, so the inline
// image a comment resolves to still pointed at the broken, raw-sysid URL
// even once the frontend's own id-matching bug (support.ts,
// replaceInlineImageSources) was separately fixed.
func TestSNCaseService_SearchCaseComments_ConvertsInlineAttachmentDownloadURL(t *testing.T) {
	const sysID = "9eee0466eb378750fcf5f5dabad0cd32"
	const wantUUID = "9eee0466-eb37-8750-fcf5-f5dabad0cd32"

	client := newTestSNClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"comments": [{
				"id": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				"referenceId": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				"content": "see attached",
				"type": "comments",
				"createdOn": "2026-08-01 10:00:00",
				"createdBy": "jane.doe@example.com",
				"hasInlineAttachments": true,
				"inlineAttachments": [{
					"id": "` + sysID + `",
					"fileName": "screenshot.png",
					"contentType": "image/png",
					"downloadUrl": "/attachments/` + sysID + `/content",
					"createdOn": "2026-08-01 10:00:00",
					"createdBy": "jane.doe@example.com"
				}]
			}],
			"offset": 0,
			"limit": 50,
			"totalRecords": 1
		}`))
	}))

	svc := NewServiceNowCaseService(client, nil, nil, nil, nil, "", nil)

	resp, err := svc.SearchCaseComments(context.Background(), domain.SearchCaseCommentsRequest{
		CaseID:     testUUID,
		Pagination: domain.Pagination{Limit: 50},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Comments) != 1 || len(resp.Comments[0].InlineAttachments) != 1 {
		t.Fatalf("got %+v, want exactly one comment with one inline attachment", resp)
	}

	ia := resp.Comments[0].InlineAttachments[0]
	if ia.ID != wantUUID {
		t.Errorf("InlineAttachments[0].ID = %q, want %q", ia.ID, wantUUID)
	}
	wantURL := "/attachments/" + wantUUID + "/content"
	if ia.DownloadURL != wantURL {
		t.Errorf("InlineAttachments[0].DownloadURL = %q, want %q (the raw sysid must be replaced with the converted UUID)", ia.DownloadURL, wantURL)
	}
}
