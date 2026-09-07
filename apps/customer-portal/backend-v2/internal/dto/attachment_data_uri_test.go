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

import "testing"

// TestAttachmentUpload_RebuildsDataURI is the regression guard for a 400 seen in
// staging: `file must be a base64 data URI (e.g. data:image/png;base64,...)`.
//
// The frontend strips the "data:<mime>;base64," prefix before sending and passes
// the MIME type separately, while entity-service requires the prefix. Both upload
// paths must reconstruct it.
func TestAttachmentUpload_RebuildsDataURI(t *testing.T) {
	const content = "JVBERi0xLjQK"
	const mime = "application/pdf"
	want := "data:" + mime + ";base64," + content

	caseReq := BuildEntityCreateCaseAttachmentRequest("case-1", CreateCaseAttachmentRequest{
		Name: "runbook.pdf", Type: mime, Content: content,
	})
	if caseReq.File != want {
		t.Errorf("case upload File = %q, want %q", caseReq.File, want)
	}

	depReq := BuildEntityCreateDeploymentAttachmentRequest("dep-1", CreateDeploymentAttachmentRequest{
		Name: "runbook.pdf", Type: mime, Content: content,
	})
	if depReq.File != want {
		t.Errorf("deployment upload File = %q, want %q", depReq.File, want)
	}
}

// TestAttachmentUpload_PassesThroughAnExistingDataURI keeps a caller that already
// sends the full URI working — the value must not be double-prefixed.
func TestAttachmentUpload_PassesThroughAnExistingDataURI(t *testing.T) {
	full := "data:image/png;base64,iVBORw0KGgo="
	got := BuildEntityCreateDeploymentAttachmentRequest("dep-1", CreateDeploymentAttachmentRequest{
		Name: "shot.png", Type: "image/png", Content: full,
	})
	if got.File != full {
		t.Errorf("File = %q, want it passed through unchanged", got.File)
	}
}

// TestAttachmentUpload_FallsBackWhenTypeMissing covers a client omitting `type`:
// entity-service still needs a syntactically valid data URI, so a generic MIME
// type is used rather than emitting "data:;base64,..." which would fail its check.
func TestAttachmentUpload_FallsBackWhenTypeMissing(t *testing.T) {
	got := BuildEntityCreateCaseAttachmentRequest("case-1", CreateCaseAttachmentRequest{
		Name: "blob.bin", Type: "", Content: "AAAA",
	})
	if got.File != "data:application/octet-stream;base64,AAAA" {
		t.Errorf("File = %q, want the octet-stream fallback", got.File)
	}
}

// TestAttachmentUpload_EmptyContentStaysEmpty leaves validation to entity-service,
// which returns "file is required" — this must not become "data:...;base64,".
func TestAttachmentUpload_EmptyContentStaysEmpty(t *testing.T) {
	got := BuildEntityCreateDeploymentAttachmentRequest("dep-1", CreateDeploymentAttachmentRequest{
		Name: "empty", Type: "text/plain", Content: "",
	})
	if got.File != "" {
		t.Errorf("File = %q, want empty so entity-service reports 'file is required'", got.File)
	}
}
