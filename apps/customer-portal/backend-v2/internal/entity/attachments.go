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

package entity

import (
	"context"
	"fmt"
	"net/url"
)

// CreateAttachment calls POST /attachments.
func (c *Client) CreateAttachment(ctx context.Context, req CreateAttachmentRequest) (CreateAttachmentResponse, error) {
	var out CreateAttachmentResponse
	err := c.postJSON(ctx, "/attachments", req, &out)
	return out, err
}

// SearchAttachments calls POST /attachments/search.
func (c *Client) SearchAttachments(ctx context.Context, req SearchAttachmentsRequest) (SearchAttachmentsResponse, error) {
	var out SearchAttachmentsResponse
	err := c.postJSON(ctx, "/attachments/search", req, &out)
	return out, err
}

// GetAttachmentContent calls GET /attachments/{id}/content and returns the
// raw file bytes together with its Content-Type.
func (c *Client) GetAttachmentContent(ctx context.Context, id string) (body []byte, contentType string, err error) {
	return c.doBinary(ctx, fmt.Sprintf("/attachments/%s/content", url.PathEscape(id)))
}

// DeleteAttachment calls DELETE /attachments/{id}.
func (c *Client) DeleteAttachment(ctx context.Context, id string) (DeleteAttachmentResponse, error) {
	var out DeleteAttachmentResponse
	err := c.deleteJSON(ctx, fmt.Sprintf("/attachments/%s", url.PathEscape(id)), &out)
	return out, err
}

// GetAttachment calls GET /attachments/{id} — metadata plus base64-encoded
// content, distinct from GetAttachmentContent's raw binary stream.
func (c *Client) GetAttachment(ctx context.Context, id string) (AttachmentDetails, error) {
	var out AttachmentDetails
	err := c.getJSON(ctx, fmt.Sprintf("/attachments/%s", url.PathEscape(id)), &out)
	return out, err
}

// UpdateAttachment calls PATCH /attachments/{id}.
func (c *Client) UpdateAttachment(ctx context.Context, id string, req UpdateAttachmentRequest) (UpdateAttachmentResponse, error) {
	var out UpdateAttachmentResponse
	err := c.patchJSON(ctx, fmt.Sprintf("/attachments/%s", url.PathEscape(id)), req, &out)
	return out, err
}
