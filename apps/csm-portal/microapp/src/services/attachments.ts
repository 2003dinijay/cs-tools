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

import { queryOptions } from "@tanstack/react-query";
import { ATTACHMENT_CONTENT_ENDPOINT, ATTACHMENTS_ENDPOINT, ATTACHMENTS_SEARCH_ENDPOINT } from "@config/endpoints";
import type {
  AttachmentCreatePayloadDto,
  AttachmentCreateResponseDto,
  AttachmentDetailDto,
  AttachmentReferenceType,
  AttachmentSearchResponseDto,
} from "@src/types";
import { toCaseAttachment, type CaseAttachment } from "@src/types";
import { isSafeAttachmentContentType, normalizeContentType } from "@utils/attachmentPreview";
import apiClient from "./apiClient";

const createAttachment = async (payload: AttachmentCreatePayloadDto): Promise<AttachmentDetailDto> => {
  const { data } = await apiClient.post<AttachmentCreateResponseDto>(ATTACHMENTS_ENDPOINT, payload);
  return data.attachment;
};

const searchAttachments = async (
  referenceId: string,
  referenceType: AttachmentReferenceType,
): Promise<CaseAttachment[]> => {
  const { data } = await apiClient.post<AttachmentSearchResponseDto>(ATTACHMENTS_SEARCH_ENDPOINT, {
    referenceId,
    referenceType,
    pagination: { limit: 50 },
  });
  return (data.attachments ?? []).map(toCaseAttachment);
};

// Fetches an attachment's raw bytes via `GET /attachments/{id}/content`, for the preview dialog.
// The content endpoint always responds with `Content-Disposition: attachment` and streams behind
// auth, so it's fetched as a Blob rather than pointed at directly (a plain `openUrl`/`<img src>`
// would miss the auth headers and would force a download instead of rendering inline) — mirrors
// the webapp's useGetCsmCaseAttachmentContent (useCsmCaseAttachments.ts).
//
// The response's Content-Type is only forwarded as-is for the backend's safe-content-type
// allowlist (case_handler.go) — anything else is coerced to application/octet-stream server-side
// to block a stored-XSS via a crafted upstream type. The attachment's own `type` from
// /attachments/search list metadata is whatever the uploader claimed, unverified — so it's only
// used to relabel the fetched blob when that metadata type is itself in the same allowlist
// (mirrored client-side as isSafeAttachmentContentType); otherwise the blob is left as
// application/octet-stream, matching what the backend already coerced it to.
const getAttachmentContent = async (attachment: CaseAttachment): Promise<Blob> => {
  const { data } = await apiClient.get<Blob>(ATTACHMENT_CONTENT_ENDPOINT(attachment.id), { responseType: "blob" });
  if (!isSafeAttachmentContentType(attachment.type)) return data;
  // Compare/relabel with the normalized form, not the raw uploader-provided string — a stray
  // ";charset=..." parameter would otherwise make it into the relabeled blob's type verbatim.
  const safeType = normalizeContentType(attachment.type);
  return data.type === safeType ? data : data.slice(0, data.size, safeType);
};

// For inline `<img>` references embedded in comment/description HTML, where there's no
// CaseAttachment metadata to cross-check a claimed content type against (see
// useResolvedInlineImageHtml) — the backend already coerces unsafe upstream content types to
// application/octet-stream server-side (case_handler.go), so the blob's own reported type is
// trustworthy as-is.
const getAttachmentContentById = async (id: string): Promise<Blob> => {
  const { data } = await apiClient.get<Blob>(ATTACHMENT_CONTENT_ENDPOINT(id), { responseType: "blob" });
  return data;
};

export const attachments = {
  create: createAttachment,
  forCase: (caseId: string) =>
    queryOptions({
      queryKey: ["case", caseId, "attachments"],
      queryFn: () => searchAttachments(caseId, "case"),
    }),
  getContent: getAttachmentContent,
  getContentById: getAttachmentContentById,
};
