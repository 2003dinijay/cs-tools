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
import { getAttachmentPreviewKind } from "@features/csm-cases/utils/attachmentPreview";

describe("getAttachmentPreviewKind", () => {
  it("classifies any image/* type as previewable", () => {
    expect(getAttachmentPreviewKind("image/png")).toBe("image");
    expect(getAttachmentPreviewKind("IMAGE/JPEG")).toBe("image");
    expect(getAttachmentPreviewKind("image/webp")).toBe("image");
  });

  it("classifies any video/* type as previewable", () => {
    expect(getAttachmentPreviewKind("video/mp4")).toBe("video");
    expect(getAttachmentPreviewKind("video/quicktime")).toBe("video");
  });

  it("classifies application/pdf as previewable", () => {
    expect(getAttachmentPreviewKind("application/pdf")).toBe("pdf");
  });

  it("returns null for everything else, including a content type with parameters", () => {
    expect(getAttachmentPreviewKind("application/zip")).toBeNull();
    expect(getAttachmentPreviewKind("application/msword")).toBeNull();
    expect(getAttachmentPreviewKind("text/plain")).toBeNull();
    expect(getAttachmentPreviewKind(" application/pdf ")).toBe("pdf");
  });
});
