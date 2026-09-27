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
	"net/url"
)

// DownloadAttachment retrieves an attachment's raw content by its
// ServiceNow sys_id — ported from Ballerina downloadAttachment
// (modules/operations/operations.bal). Returns the raw bytes together with
// the upstream Content-Type and Content-Disposition headers; the caller
// (handler) is responsible for deciding which of those to trust before
// writing them to the HTTP response (see internal/handler's existing
// GetCaseAttachmentContent for this backend's established
// content-type-allowlist convention). Authorization
// (downloadAttachmentGroups) is enforced by the caller, not here — this
// method has no access to the caller's group membership.
func (c *Client) DownloadAttachment(ctx context.Context, attachmentSysID string) (body []byte, contentType string, contentDisposition string, err error) {
	return c.GetBinary(ctx, "/api/now/attachment/"+url.PathEscape(attachmentSysID)+"/file", nil)
}
