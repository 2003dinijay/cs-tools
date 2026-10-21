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

import { render, screen, fireEvent } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import AllConversationsPage from "@features/support/pages/AllConversationsPage";

const mockNavigate = vi.fn();
const mockShowLoader = vi.fn();
const mockHideLoader = vi.fn();

const mockUseParams = vi.fn();
const mockUseLocation = vi.fn();
const mockUseSearchParams = vi.fn();
const mockUseSessionState = vi.fn();
const mockUseGetProjectFilters = vi.fn();
const mockUseSearchConversations = vi.fn();

vi.mock("react-router", async (importOriginal) => {
  const actual = (await importOriginal()) as object;
  return {
    ...actual,
    useParams: () => mockUseParams(),
    useLocation: () => mockUseLocation(),
    useSearchParams: () => mockUseSearchParams(),
  };
});

vi.mock("@hooks/useModifierAwareNavigate", () => ({
  useModifierAwareNavigate: () => mockNavigate,
}));

vi.mock("@hooks/useSessionState", () => ({
  useSessionState: (...args: unknown[]) => mockUseSessionState(...args),
}));

vi.mock("@context/linear-loader/LoaderContext", () => ({
  useLoader: () => ({ showLoader: mockShowLoader, hideLoader: mockHideLoader }),
}));

vi.mock("@api/useGetProjectFilters", () => ({
  default: () => mockUseGetProjectFilters(),
}));

// The close-chat flow needs the error banner provider; it plays no part in these tests.
vi.mock("@features/support/hooks/useCloseConversationFlow", () => ({
  useCloseConversationFlow: () => ({
    isConfirmOpen: false,
    isClosing: false,
    requestClose: vi.fn(),
    cancelClose: vi.fn(),
    confirmClose: vi.fn(),
  }),
}));

vi.mock("@features/support/api/useSearchConversations", () => ({
  useSearchConversations: (...args: unknown[]) =>
    mockUseSearchConversations(...args),
}));

vi.mock("@components/list-view/ListPageHeader", () => ({
  default: ({ title }: { title: string }) => <h1>{title}</h1>,
}));

vi.mock("@components/list-view/ListSearchBar", () => ({
  default: () => <div data-testid="search-bar">SearchBar</div>,
}));

vi.mock("@features/support/components/all-conversations/AllConversationsList", () => ({
  default: ({
    onConversationClick,
  }: {
    onConversationClick: (conv: {
      id: string;
      number: string;
      initialMessage: string;
      createdOn: string;
      messageCount: number;
      state: { label: string };
      createdBy: string;
      case?: { id: string; label: string } | null;
    }) => void;
  }) => (
    <>
      <button
        onClick={() =>
          onConversationClick({
            id: "conv-1",
            number: "CHAT-1",
            initialMessage: "Need help",
            createdOn: "2026-05-01T00:00:00Z",
            messageCount: 2,
            state: { label: "Open" },
            createdBy: "dev@wso2.com",
          })
        }
      >
        Open Conversation
      </button>
      <button
        onClick={() =>
          onConversationClick({
            id: "conv-2",
            number: "CHAT-2",
            initialMessage: "Need a case",
            createdOn: "2026-05-01T00:00:00Z",
            messageCount: 4,
            state: { label: "Converted" },
            createdBy: "dev@wso2.com",
            case: { id: "case-99", label: "CS0000099" },
          })
        }
      >
        Open Converted Conversation
      </button>
      <button
        onClick={() =>
          onConversationClick({
            id: "conv-3",
            number: "CHAT-3",
            initialMessage: "Blank case id",
            createdOn: "2026-05-01T00:00:00Z",
            messageCount: 1,
            state: { label: "Open" },
            createdBy: "dev@wso2.com",
            case: { id: "   ", label: "" },
          })
        }
      >
        Open Blank Case Conversation
      </button>
      <button
        onClick={() =>
          onConversationClick({
            id: "conv-4",
            number: "CHAT-4",
            initialMessage: "Empty case id",
            createdOn: "2026-05-01T00:00:00Z",
            messageCount: 1,
            state: { label: "Open" },
            createdBy: "dev@wso2.com",
            case: { id: "", label: "" },
          })
        }
      >
        Open Empty Case Conversation
      </button>
    </>
  ),
}));

vi.mock("@components/list-view/ListResultsBar", () => ({
  default: () => <div>ResultsBar</div>,
}));

vi.mock("@components/list-view/ListPagination", () => ({
  default: () => <div>Pagination</div>,
}));

describe("AllConversationsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseParams.mockReturnValue({ projectId: "project-1" });
    mockUseLocation.mockReturnValue({ state: { returnTo: "/projects/project-1/support" } });
    mockUseSearchParams.mockReturnValue([new URLSearchParams(), vi.fn()]);
    mockUseGetProjectFilters.mockReturnValue({ data: { conversationStates: [] } });
    mockUseSearchConversations.mockReturnValue({
      data: { conversations: [], totalRecords: 0 },
      isLoading: false,
      isError: false,
    });

    mockUseSessionState.mockImplementation(
      (_key: string, initialValue: unknown) => [initialValue, vi.fn()],
    );
  });

  it("should hide search bar in status-filter mode", () => {
    mockUseSearchParams.mockReturnValue([
      new URLSearchParams("statusFilter=active"),
      vi.fn(),
    ]);

    render(<AllConversationsPage />);

    expect(screen.queryByTestId("search-bar")).not.toBeInTheDocument();
    expect(screen.getByText("Active Chats")).toBeInTheDocument();
  });

  // The Support card says "Resolved via Chat (Last 30d)"; its list must be limited the same way.
  it("limits the resolved-via-chat list to the last 30 days", () => {
    mockUseSearchParams.mockReturnValue([
      new URLSearchParams("statusFilter=resolvedViaChat"),
      vi.fn(),
    ]);
    mockUseGetProjectFilters.mockReturnValue({
      data: { conversationStates: [{ id: "3", label: "Resolved" }] },
    });

    const before = Date.now();
    render(<AllConversationsPage />);

    const request = mockUseSearchConversations.mock.calls.at(-1)?.[1] as {
      filters: { stateKeys?: number[]; startUpdatedDate?: string; endUpdatedDate?: string };
    };
    expect(request.filters.stateKeys).toEqual([3]);
    const since = Date.parse(request.filters.startUpdatedDate ?? "");
    const thirtyDays = 30 * 24 * 60 * 60 * 1000;
    expect(Number.isNaN(since)).toBe(false);
    // within a few seconds of exactly 30 days before the render
    expect(Math.abs(before - thirtyDays - since)).toBeLessThan(5000);
    expect(request.filters.endUpdatedDate).toBeUndefined();
  });

  it("does not limit the active-chats list by date", () => {
    mockUseSearchParams.mockReturnValue([
      new URLSearchParams("statusFilter=active"),
      vi.fn(),
    ]);

    render(<AllConversationsPage />);

    const request = mockUseSearchConversations.mock.calls.at(-1)?.[1] as {
      filters: { startUpdatedDate?: string };
    };
    expect(request.filters.startUpdatedDate).toBeUndefined();
  });

  it("should navigate with composed conversation summary", () => {
    mockUseSearchConversations.mockReturnValue({
      data: { conversations: [{ id: "conv-1" }], totalRecords: 1 },
      isLoading: false,
      isError: false,
    });

    render(<AllConversationsPage />);
    fireEvent.click(screen.getByText("Open Conversation"));

    expect(mockNavigate).toHaveBeenCalledWith(
      "/projects/project-1/support/conversations/conv-1",
      {
        state: {
          conversationSummary: expect.objectContaining({
            chatId: "conv-1",
            chatNumber: "CHAT-1",
            messages: 2,
          }),
        },
      },
    );
  });

  it("navigates to the linked case when View is used on a converted chat", () => {
    mockUseSearchConversations.mockReturnValue({
      data: { conversations: [{ id: "conv-2" }], totalRecords: 1 },
      isLoading: false,
      isError: false,
    });

    render(<AllConversationsPage />);
    fireEvent.click(screen.getByText("Open Converted Conversation"));

    expect(mockNavigate).toHaveBeenCalledWith(
      "/projects/project-1/support/cases/case-99",
      { state: { returnTo: "/projects/project-1/support" } },
    );
  });

  it.each([
    ["whitespace-only", "Open Blank Case Conversation", "conv-3", "CHAT-3"],
    ["empty", "Open Empty Case Conversation", "conv-4", "CHAT-4"],
  ])(
    "keeps the conversation summary when the linked case id is %s",
    (_label, buttonText, chatId, chatNumber) => {
      mockUseSearchConversations.mockReturnValue({
        data: { conversations: [{ id: chatId }], totalRecords: 1 },
        isLoading: false,
        isError: false,
      });

      render(<AllConversationsPage />);
      fireEvent.click(screen.getByText(buttonText));

      expect(mockNavigate).toHaveBeenCalledWith(
        `/projects/project-1/support/conversations/${chatId}`,
        {
          state: {
            conversationSummary: expect.objectContaining({ chatId, chatNumber }),
          },
        },
      );
    },
  );
});

