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

package servicenow

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDownloadAttachment_ReturnsBodyAndHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/now/attachment/att-sys-id/file" {
			t.Errorf("path = %q, want /api/now/attachment/att-sys-id/file", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", `attachment; filename="report.pdf"`)
		_, _ = w.Write([]byte("%PDF-1.4 fake content"))
	}))
	defer srv.Close()

	c := NewClient(Config{BaseURL: srv.URL, Username: "u", Password: "p"})

	body, ct, cd, err := c.DownloadAttachment(context.Background(), "att-sys-id")
	if err != nil {
		t.Fatalf("DownloadAttachment returned error: %v", err)
	}
	if string(body) != "%PDF-1.4 fake content" {
		t.Errorf("body = %q", body)
	}
	if ct != "application/pdf" {
		t.Errorf("contentType = %q, want application/pdf", ct)
	}
	if cd == "" {
		t.Error("contentDisposition should be forwarded from upstream")
	}
}
