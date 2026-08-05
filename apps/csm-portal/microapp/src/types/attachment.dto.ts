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

export type AttachmentReferenceType = "case" | "conversation" | "change_request" | "deployment";

export interface AttachmentCreatePayloadDto {
  referenceId: string;
  referenceType: AttachmentReferenceType;
  name: string;
  type: string;
  /** Base64 data URI, e.g. "data:image/png;base64,...". Max 10 MB decoded. */
  file: string;
}

export interface AttachmentDetailDto {
  id: string;
  sizeBytes?: number;
  downloadUrl?: string;
}

export interface AttachmentCreateResponseDto {
  message?: string;
  attachment: AttachmentDetailDto;
}

// The canonical {id, email, name} UserReference shape (openapi.yaml) — confirmed live against a
// real response. Same shape as CaseCommentAuthorDto in case.dto.ts; kept separate per this
// codebase's convention of one type per DTO file rather than a shared cross-file import for it.
export interface AttachmentAuthorDto {
  id: string | null;
  email: string;
  name: string;
}

// List/search view — mirrors openapi.yaml's Attachment schema (the full record, as opposed to
// AttachmentDetailDto's thin create-response ack).
export interface AttachmentViewDto {
  id: string;
  referenceId: string;
  referenceType: AttachmentReferenceType;
  name: string;
  type: string;
  sizeBytes: number;
  description: string | null;
  // openapi.yaml marks this nullable — null is handled defensively in attachmentAuthorLabel
  // (attachment.model.ts) already, but the type should say so too rather than let a future,
  // non-defensive read assume it's always present.
  createdBy: string | AttachmentAuthorDto | null;
  createdOn: string;
  downloadUrl: string | null;
  previewUrl: string | null;
}

export interface AttachmentSearchPayloadDto {
  referenceId: string;
  referenceType: AttachmentReferenceType;
  pagination?: {
    offset?: number;
    limit?: number;
  };
}

export interface AttachmentSearchResponseDto {
  attachments: AttachmentViewDto[];
  total: number;
  limit: number;
  offset: number;
  hasMore?: boolean;
}
