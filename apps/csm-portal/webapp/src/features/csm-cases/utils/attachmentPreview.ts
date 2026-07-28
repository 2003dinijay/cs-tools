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

/** The content-type families {@link AttachmentPreviewDialog} knows how to render inline. */
export type AttachmentPreviewKind = "image" | "video" | "pdf";

/**
 * Classify an attachment's content type into a previewable kind, or `null`
 * when it has no inline preview (docs, archives, etc. stay download-only).
 */
export function getAttachmentPreviewKind(
  contentType: string,
): AttachmentPreviewKind | null {
  const type = contentType.toLowerCase().trim();
  if (type.startsWith("image/")) return "image";
  if (type.startsWith("video/")) return "video";
  if (type === "application/pdf") return "pdf";
  return null;
}
