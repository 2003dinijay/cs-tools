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

import { describe, expect, it } from "vitest";
import {
  getAttachmentPreviewKind,
  isSafeAttachmentContentType,
} from "@features/csm-cases/utils/attachmentPreview";

describe("getAttachmentPreviewKind", () => {
  it("classifies any image/* type in the backend's safe-type allowlist as previewable", () => {
    expect(getAttachmentPreviewKind("image/png")).toBe("image");
    expect(getAttachmentPreviewKind("IMAGE/JPEG")).toBe("image");
    expect(getAttachmentPreviewKind("image/webp")).toBe("image");
    expect(getAttachmentPreviewKind("image/gif")).toBe("image");
  });

  it("classifies application/pdf as previewable", () => {
    expect(getAttachmentPreviewKind("application/pdf")).toBe("pdf");
  });

  it(
    "does not offer a video preview, because the backend's safe-content-type " +
      "allowlist (`safeAttachmentTypes` in case_handler.go) has no video/* " +
      "entry at all -- GET /attachments/{id}/content always coerces video " +
      "responses to application/octet-stream, so a fetched blob can never be " +
      "trusted as playable video regardless of what the upload metadata claims",
    () => {
      expect(getAttachmentPreviewKind("video/mp4")).toBeNull();
      expect(getAttachmentPreviewKind("video/quicktime")).toBeNull();
      expect(getAttachmentPreviewKind("VIDEO/MP4")).toBeNull();
    },
  );

  it("returns null for allowlisted-but-not-inline types (archives, office docs, text)", () => {
    expect(getAttachmentPreviewKind("application/zip")).toBeNull();
    expect(getAttachmentPreviewKind("application/x-zip-compressed")).toBeNull();
    expect(getAttachmentPreviewKind("application/msword")).toBeNull();
    expect(
      getAttachmentPreviewKind(
        "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
      ),
    ).toBeNull();
    expect(getAttachmentPreviewKind("application/vnd.ms-excel")).toBeNull();
    expect(
      getAttachmentPreviewKind(
        "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
      ),
    ).toBeNull();
    expect(getAttachmentPreviewKind("text/plain")).toBeNull();
  });

  it("returns null for a type outside the backend allowlist entirely", () => {
    expect(getAttachmentPreviewKind("application/x-msdownload")).toBeNull();
  });

  it("normalizes content-type parameters and casing the same way the backend does", () => {
    expect(getAttachmentPreviewKind(" application/pdf ")).toBe("pdf");
    expect(getAttachmentPreviewKind("application/pdf; charset=binary")).toBe(
      "pdf",
    );
    expect(getAttachmentPreviewKind("IMAGE/PNG;foo=bar")).toBe("image");
  });
});

describe("isSafeAttachmentContentType", () => {
  it("accepts every type in the backend's safe-content-type allowlist", () => {
    for (const type of [
      "image/png",
      "image/jpeg",
      "image/gif",
      "image/webp",
      "application/pdf",
      "text/plain",
      "application/zip",
      "application/x-zip-compressed",
      "application/msword",
      "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
      "application/vnd.ms-excel",
      "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
    ]) {
      expect(isSafeAttachmentContentType(type)).toBe(true);
    }
  });

  it("rejects a video type, since it has no entry in the backend allowlist", () => {
    expect(isSafeAttachmentContentType("video/mp4")).toBe(false);
  });

  it("rejects an arbitrary/spoofable type not in the allowlist", () => {
    expect(isSafeAttachmentContentType("application/x-msdownload")).toBe(
      false,
    );
  });

  it("normalizes parameters and casing before checking membership", () => {
    expect(isSafeAttachmentContentType("APPLICATION/PDF;charset=binary")).toBe(
      true,
    );
  });
});
