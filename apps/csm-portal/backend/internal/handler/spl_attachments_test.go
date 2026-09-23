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
)

type mockSplAttachmentsClient struct {
	body        []byte
	contentType string
	err         error
}

func (m *mockSplAttachmentsClient) DownloadAttachment(ctx context.Context, attachmentSysID string) ([]byte, string, string, error) {
	return m.body, m.contentType, "", m.err
}

func newAttachmentDownloadRequest(attachmentID string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/attachments/"+attachmentID+"/download", nil)
	req.SetPathValue("attachmentId", attachmentID)
	return withUser(req)
}

func TestDownloadAttachment_Success(t *testing.T) {
	mock := &mockSplAttachmentsClient{body: []byte("%PDF-1.4"), contentType: "application/pdf"}
	h := NewSplAttachmentsHandler(mock, []string{"csm-agents"}, []string{"csm-agents"})
	w := httptest.NewRecorder()

	h.DownloadAttachment(w, newAttachmentDownloadRequest("att-1"))

	assertStatus(t, w, http.StatusOK)
	assertContentType(t, w, "application/pdf")
	if w.Header().Get("Content-Disposition") != "attachment" {
		t.Errorf("Content-Disposition = %q, want %q", w.Header().Get("Content-Disposition"), "attachment")
	}
	if w.Body.String() != "%PDF-1.4" {
		t.Errorf("body = %q", w.Body.String())
	}
}

func TestDownloadAttachment_CoercesUnsafeContentType(t *testing.T) {
	mock := &mockSplAttachmentsClient{body: []byte("<script>"), contentType: "text/html"}
	h := NewSplAttachmentsHandler(mock, []string{"csm-agents"}, []string{"csm-agents"})
	w := httptest.NewRecorder()

	h.DownloadAttachment(w, newAttachmentDownloadRequest("att-1"))

	assertContentType(t, w, "application/octet-stream")
}

func TestDownloadAttachment_RejectsSubGroupMismatch(t *testing.T) {
	h := NewSplAttachmentsHandler(&mockSplAttachmentsClient{}, []string{"csm-agents"}, []string{"attachment-downloaders"})
	w := httptest.NewRecorder()

	h.DownloadAttachment(w, newAttachmentDownloadRequest("att-1"))

	assertStatus(t, w, http.StatusForbidden)
}

func TestDownloadAttachment_RejectsEmptyID(t *testing.T) {
	h := NewSplAttachmentsHandler(&mockSplAttachmentsClient{}, []string{"csm-agents"}, []string{"csm-agents"})
	req := withUser(httptest.NewRequest(http.MethodGet, "/attachments//download", nil))
	req.SetPathValue("attachmentId", "")
	w := httptest.NewRecorder()

	h.DownloadAttachment(w, req)

	assertStatus(t, w, http.StatusBadRequest)
}
