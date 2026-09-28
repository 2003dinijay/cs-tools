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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/servicenow"
)

type mockSplCaseClient struct {
	getAttachmentsInfoFn func(ctx context.Context, caseNumber string, offset, limit int) ([]servicenow.AttachmentInfo, error)
}

func (m *mockSplCaseClient) GetAttachmentsInfo(ctx context.Context, caseNumber string, offset, limit int) ([]servicenow.AttachmentInfo, error) {
	return m.getAttachmentsInfoFn(ctx, caseNumber, offset, limit)
}

func TestSplGetAttachmentsInfo_MissingCaseIDIs400(t *testing.T) {
	h := NewSplCaseHandler(&mockSplCaseClient{}, splAccessGuard)
	r := withUser(httptest.NewRequest(http.MethodGet, "/spl/cases//attachments-info?offset=0&limit=10", nil))
	w := httptest.NewRecorder()
	h.GetAttachmentsInfo(w, r)
	assertStatus(t, w, http.StatusBadRequest)
}
