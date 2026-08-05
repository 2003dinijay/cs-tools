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

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

// The real client reads runtime config at module load, which isn't present
// under vitest (same approach as CaseActivitiesFeed.test.tsx). Pulled in
// transitively via CsmUploadAttachmentModal -> useCsmCaseAttachments, just
// for its MAX_ATTACHMENT_SIZE_BYTES constant — the mock only exists to keep
// that import chain from throwing on missing runtime config.
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: vi.fn() }),
}));

import CsmCaseCommentInput from "@features/csm-cases/components/CsmCaseCommentInput";

// This component isn't under test here; stub it to a plain textarea (same
// technique EditCaseDetailsDialog.test.tsx uses for the same dependency).
vi.mock("@components/rich-text-editor/Editor", () => ({
  default: ({
    value,
    onChange,
  }: {
    value: string;
    onChange: (v: string) => void;
  }) => (
    <textarea
      aria-label="comment-editor"
      value={value}
      onChange={(e) => onChange(e.target.value)}
    />
  ),
}));

const PAUSED_REASON =
  "This case is paused — customer replies are disabled. Resume work to reply to the customer.";
const NOT_STARTED_REASON =
  "Customer replies are disabled unless the case is actively in progress.";

describe("CsmCaseCommentInput — resume-work quick-fix", () => {
  it("shows the inline resume link when the only lock reason is paused work, and calls onResumeWork", () => {
    const onResumeWork = vi.fn();
    render(
      <CsmCaseCommentInput
        onSubmit={vi.fn()}
        publicCommentDisabledReason={PAUSED_REASON}
        canResumeToUnlockPublicReply
        onResumeWork={onResumeWork}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Resume work" }));
    expect(onResumeWork).toHaveBeenCalledTimes(1);
  });

  it("doesn't also repeat the raw lock reason in the send-row status line once the quick-fix covers it", () => {
    render(
      <CsmCaseCommentInput
        onSubmit={vi.fn()}
        publicCommentDisabledReason={PAUSED_REASON}
        canResumeToUnlockPublicReply
        onResumeWork={vi.fn()}
      />,
    );

    // The quick-fix's own sentence is present once...
    expect(
      screen.getByText("Only resumed work can send public replies to the customer.", {
        exact: false,
      }),
    ).toBeInTheDocument();
    // ...and the send-row status line falls back to the normal hint instead
    // of repeating the raw backend reason a second time.
    expect(screen.queryByText(PAUSED_REASON)).not.toBeInTheDocument();
    expect(screen.getByText("Ctrl/Cmd + Enter to send.")).toBeInTheDocument();
  });

  it("does not show the resume link when the case hasn't started yet (not just paused)", () => {
    render(
      <CsmCaseCommentInput
        onSubmit={vi.fn()}
        publicCommentDisabledReason={NOT_STARTED_REASON}
        canResumeToUnlockPublicReply={false}
        onResumeWork={vi.fn()}
      />,
    );

    expect(
      screen.queryByRole("button", { name: "Resume work" }),
    ).not.toBeInTheDocument();
    expect(screen.getByText(NOT_STARTED_REASON)).toBeInTheDocument();
  });

  it("does not show the resume link when public replies are already allowed", () => {
    render(
      <CsmCaseCommentInput
        onSubmit={vi.fn()}
        publicCommentDisabledReason={null}
        canResumeToUnlockPublicReply
        onResumeWork={vi.fn()}
      />,
    );

    expect(
      screen.queryByRole("button", { name: "Resume work" }),
    ).not.toBeInTheDocument();
  });

  it("disables the resume link and shows a pending label while resuming", () => {
    render(
      <CsmCaseCommentInput
        onSubmit={vi.fn()}
        publicCommentDisabledReason={PAUSED_REASON}
        canResumeToUnlockPublicReply
        onResumeWork={vi.fn()}
        isResumingWork
      />,
    );

    expect(screen.getByRole("button", { name: "Resuming…" })).toBeDisabled();
  });
});
