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
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import type { ReactElement } from "react";
import { MemoryRouter } from "react-router";
import CsmCaseCommentBubble from "@features/csm-cases/components/CsmCaseCommentBubble";
import type { CsmCaseComment } from "@features/csm-cases/types/csmCases";

vi.mock("@features/csm-cases/api/useResolvedInlineImageHtml", () => ({
  // Pass the sanitized HTML straight through — no attachment resolution in
  // these tests, which don't exercise the react-query/backend-client path.
  useResolvedInlineImageHtml: vi.fn((html: string) => ({
    resolvedHtml: html,
    isLoading: false,
  })),
}));

// The real client reads runtime config at module load, which isn't present
// under vitest (same approach as useQuickCaseSearch.test.tsx). The comment
// author name renders through `UserRefLink`, which resolves an unknown id
// through `useResolvedUserId`, which calls this client.
const searchUsersByEmail = vi.fn().mockResolvedValue({ users: [] });
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: searchUsersByEmail }),
}));

function makeComment(overrides: Partial<CsmCaseComment>): CsmCaseComment {
  return {
    id: "c-1",
    caseId: "case-1",
    authorName: "Jane Doe",
    authorRole: "customer",
    bodyHtml: "<p>Hello there</p>",
    createdAt: "2026-07-01T00:00:00Z",
    ...overrides,
  };
}

// `UserRefLink` (used for the comment author) renders a `react-router` `Link`
// and resolves its id through react-query — needs both a Router and a
// QueryClient context even outside a full app render.
function renderWithProviders(ui: ReactElement): ReturnType<typeof render> {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>{ui}</MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("CsmCaseCommentBubble", () => {
  it("renders comment body HTML", () => {
    renderWithProviders(<CsmCaseCommentBubble comment={makeComment({})} />);
    expect(screen.getByText("Hello there")).toBeInTheDocument();
    expect(screen.getByText("Jane Doe")).toBeInTheDocument();
  });

  it("returns null for a comment with no displayable content", () => {
    const { container } = renderWithProviders(
      <CsmCaseCommentBubble comment={makeComment({ bodyHtml: "<p></p>" })} />,
    );
    expect(container).toBeEmptyDOMElement();
  });

  it("strips a single [code]...[/code] wrapper before rendering", () => {
    renderWithProviders(
      <CsmCaseCommentBubble
        comment={makeComment({ bodyHtml: "[code]<b>raw</b>[/code]" })}
      />,
    );
    expect(screen.getByText("raw")).toBeInTheDocument();
  });

  it("linkifies a bare URL and opens it in a new tab safely", () => {
    renderWithProviders(
      <CsmCaseCommentBubble
        comment={makeComment({ bodyHtml: "See https://example.com/doc" })}
      />,
    );
    const link = screen.getByRole("link", { name: /example\.com/ });
    expect(link).toHaveAttribute("target", "_blank");
    expect(link).toHaveAttribute("rel", "noopener noreferrer");
  });

  it("invokes onImageClick when an inline image is clicked", () => {
    // A relative unresolved-attachment-style src (as an unresolved .iix
    // reference would look) — a bare `https://` src would also get rewritten
    // by linkifyBareUrls, which only special-cases `href=`, so it's avoided
    // here to keep this test focused on the click-to-zoom wiring.
    const onImageClick = vi.fn();
    renderWithProviders(
      <CsmCaseCommentBubble
        comment={makeComment({
          bodyHtml: '<img src="/abc123.iix" alt="a" />',
        })}
        onImageClick={onImageClick}
      />,
    );
    const img = screen.getByRole("button", { name: "Open image preview" });
    fireEvent.click(img);
    expect(onImageClick).toHaveBeenCalledWith(
      expect.stringContaining("abc123.iix"),
      "a",
    );
  });

  it("does not mark inline images as interactive when onImageClick is not provided", () => {
    renderWithProviders(
      <CsmCaseCommentBubble
        comment={makeComment({
          bodyHtml: '<img src="/abc123.iix" alt="a" />',
        })}
      />,
    );
    expect(
      screen.queryByRole("button", { name: "Open image preview" }),
    ).not.toBeInTheDocument();
  });

  it("renders a chatbot comment's markdown body as HTML", () => {
    renderWithProviders(
      <CsmCaseCommentBubble
        comment={makeComment({
          authorRole: "chatbot",
          authorName: "Novera",
          bodyHtml: "**bold answer**",
        })}
      />,
    );
    expect(screen.getByText("bold answer")).toBeInTheDocument();
  });

  it("resolves the author link from the canonical email when there is no legacy email and no id", async () => {
    // Regression for a null canonical id + empty legacy `authorEmail`: the
    // author link must still resolve through `comment.authorUser.email`
    // rather than silently falling back to plain text.
    searchUsersByEmail.mockResolvedValueOnce({
      users: [{ id: "user-42", email: "canonical@example.com" }],
    });
    renderWithProviders(
      <CsmCaseCommentBubble
        comment={makeComment({
          authorName: "Jane Doe",
          authorEmail: undefined,
          authorUser: {
            id: null,
            email: "canonical@example.com",
            name: "Jane Doe",
          },
        })}
      />,
    );
    const link = await screen.findByRole("link", { name: "Jane Doe" });
    expect(link).toHaveAttribute("href", "/people/user-42");
  });

  it("renders a system comment as a compact inline row", () => {
    renderWithProviders(
      <CsmCaseCommentBubble
        comment={makeComment({
          authorRole: "system",
          bodyHtml: "<p>Case reassigned</p>",
        })}
      />,
    );
    expect(screen.getByText("System")).toBeInTheDocument();
    expect(screen.getByText("Case reassigned")).toBeInTheDocument();
  });
});
