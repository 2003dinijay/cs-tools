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
import type { ReactNode } from "react";
import SupportPage from "@features/support/pages/SupportPage";

const mockNavigate = vi.fn();
const mockLogger = { error: vi.fn(), debug: vi.fn() };

const mockUseParams = vi.fn();
const mockUseAsgardeo = vi.fn();
const mockUseGetProjectDetails = vi.fn();
const mockUseGetProjectFeatures = vi.fn();
const mockUseGetProjectSupportStats = vi.fn();
const mockUseGetProjectFilters = vi.fn();
const mockUseGetProjectCasesPage = vi.fn();
const mockUseSearchConversations = vi.fn();

vi.mock("react-router", () => ({
  useParams: () => mockUseParams(),
}));

vi.mock("@hooks/useModifierAwareNavigate", () => ({
  useModifierAwareNavigate: () => mockNavigate,
}));

vi.mock("@hooks/useLogger", () => ({
  useLogger: () => mockLogger,
}));

vi.mock("@asgardeo/react", () => ({
  useAsgardeo: () => mockUseAsgardeo(),
}));

vi.mock("@api/useGetProjectDetails", () => ({
  default: () => mockUseGetProjectDetails(),
}));

vi.mock("@api/useGetProjectFeatures", () => ({
  default: () => mockUseGetProjectFeatures(),
}));

vi.mock("@features/support/api/useGetProjectSupportStats", () => ({
  useGetProjectSupportStats: () => mockUseGetProjectSupportStats(),
}));

vi.mock("@api/useGetProjectFilters", () => ({
  default: () => mockUseGetProjectFilters(),
}));

vi.mock("@api/useGetProjectCasesPage", () => ({
  useGetProjectCasesPage: () => mockUseGetProjectCasesPage(),
}));

vi.mock("@features/support/api/useSearchConversations", () => ({
  useSearchConversations: () => mockUseSearchConversations(),
}));

vi.mock("@features/support/hooks/useCloseConversationFlow", () => ({
  useCloseConversationFlow: () => ({
    isConfirmOpen: false,
    isClosing: false,
    requestClose: vi.fn(),
    confirmClose: vi.fn(),
    cancelClose: vi.fn(),
  }),
}));

vi.mock("@utils/permission", () => ({
  getProjectPermissions: () => ({ includeS0InSupportMetrics: true }),
}));

vi.mock("@features/support/components/cases-overview-stats/CasesOverviewStatCard", () => ({
  default: ({
    isLoading,
    isError,
    onStatClick,
  }: {
    isLoading: boolean;
    isError: boolean;
    onStatClick: (key: string) => void;
  }) => (
    <div>
      <span data-testid="stats-loading">{String(isLoading)}</span>
      <span data-testid="stats-error">{String(isError)}</span>
      <button onClick={() => onStatClick("ongoingCases")}>Open Active Cases</button>
    </div>
  ),
}));

vi.mock("@features/support/components/support-overview-cards/SupportOverviewCard", () => ({
  default: ({
    children,
    footerButtons,
  }: {
    children: ReactNode;
    footerButtons?: Array<{ label: string; onClick: () => void }>;
  }) => (
    <div>
      {children}
      {footerButtons?.map((b) => (
        <button key={b.label} onClick={b.onClick}>
          {b.label}
        </button>
      ))}
    </div>
  ),
}));

vi.mock("@features/support/components/support-overview-cards/OutstandingCasesList", () => ({
  default: ({
    onCaseClick,
  }: {
    onCaseClick?: (caseItem: { id: string }) => void;
  }) => (
    <button onClick={() => onCaseClick?.({ id: "case-123" })}>Open Case</button>
  ),
}));

vi.mock("@features/support/components/support-overview-cards/ChatHistoryList", () => ({
  default: ({
    onItemAction,
  }: {
    onItemAction?: (chatId: string, action: "resume" | "view") => void;
  }) => (
    <>
      <button onClick={() => onItemAction?.("chat-1", "resume")}>Resume Chat</button>
      <button onClick={() => onItemAction?.("chat-1", "view")}>View Chat</button>
      <button onClick={() => onItemAction?.("chat-2", "view")}>View Converted Chat</button>
      <button onClick={() => onItemAction?.("chat-3", "view")}>View Blank Case Chat</button>
      <button onClick={() => onItemAction?.("chat-4", "view")}>View Empty Case Chat</button>
    </>
  ),
}));

describe("SupportPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();

    mockUseParams.mockReturnValue({ projectId: "project-1" });
    mockUseAsgardeo.mockReturnValue({ isLoading: false });
    mockUseGetProjectDetails.mockReturnValue({
      data: { type: { label: "Enterprise" } },
    });
    mockUseGetProjectFeatures.mockReturnValue({
      data: {},
      isLoading: false,
    });
    mockUseGetProjectSupportStats.mockReturnValue({
      data: { ongoingCases: 2 },
      isLoading: false,
      isError: false,
    });
    mockUseGetProjectFilters.mockReturnValue({
      data: { caseStates: [] },
      isLoading: false,
      isError: false,
    });
    mockUseGetProjectCasesPage.mockReturnValue({
      data: { cases: [{ id: "case-123" }] },
      isLoading: false,
      isError: false,
    });
    mockUseSearchConversations.mockReturnValue({
      data: {
        conversations: [
          {
            id: "chat-1",
            number: "CHAT-1",
            initialMessage: "Need help",
            createdOn: "2026-05-01T00:00:00Z",
            messageCount: 3,
            state: { label: "Open" },
            createdBy: "dev@wso2.com",
          },
          {
            id: "chat-2",
            number: "CHAT-2",
            initialMessage: "Need a case",
            createdOn: "2026-05-01T00:00:00Z",
            messageCount: 4,
            state: { label: "Converted" },
            createdBy: "dev@wso2.com",
            case: { id: "case-99", label: "CS0000099" },
          },
          {
            id: "chat-3",
            number: "CHAT-3",
            initialMessage: "Blank case id",
            createdOn: "2026-05-01T00:00:00Z",
            messageCount: 1,
            state: { label: "Open" },
            createdBy: "dev@wso2.com",
            case: { id: "   ", label: "" },
          },
          {
            id: "chat-4",
            number: "CHAT-4",
            initialMessage: "Empty case id",
            createdOn: "2026-05-01T00:00:00Z",
            messageCount: 1,
            state: { label: "Open" },
            createdBy: "dev@wso2.com",
            case: { id: "", label: "" },
          },
        ],
      },
      isLoading: false,
      isError: false,
    });
  });

  it("should compose loading and error flags for stats card", () => {
    mockUseAsgardeo.mockReturnValue({ isLoading: true });

    render(<SupportPage />);

    expect(screen.getByTestId("stats-loading")).toHaveTextContent("true");
    expect(screen.getByTestId("stats-error")).toHaveTextContent("false");
  });

  it("should navigate for stats and list interaction branches", () => {
    render(<SupportPage />);

    fireEvent.click(screen.getByText("Open Active Cases"));
    fireEvent.click(screen.getByText("Open Case"));
    fireEvent.click(screen.getByText("Resume Chat"));
    fireEvent.click(screen.getByText("View Chat"));

    expect(mockNavigate).toHaveBeenCalledWith("cases?statusFilter=active", {
      state: { returnTo: "/projects/project-1/support" },
    });
    expect(mockNavigate).toHaveBeenCalledWith(
      "/projects/project-1/support/cases/case-123",
      { state: { returnTo: "/projects/project-1/support" } },
    );
    expect(mockNavigate).toHaveBeenCalledWith(
      "/projects/project-1/support/chat/chat-1",
      { state: { chatNumber: "CHAT-1" } },
    );
    expect(mockNavigate).toHaveBeenCalledWith(
      "/projects/project-1/support/conversations/chat-1",
      {
        state: {
          conversationSummary: expect.objectContaining({ chatId: "chat-1" }),
          returnTo: "/projects/project-1/support",
        },
      },
    );
  });

  it("navigates to the linked case when viewing a converted chat", () => {
    render(<SupportPage />);
    fireEvent.click(screen.getByText("View Converted Chat"));
    expect(mockNavigate).toHaveBeenCalledWith(
      "/projects/project-1/support/cases/case-99",
      { state: { returnTo: "/projects/project-1/support" } },
    );
  });

  it.each([
    ["whitespace-only", "View Blank Case Chat", "chat-3"],
    ["empty", "View Empty Case Chat", "chat-4"],
  ])(
    "keeps the conversation summary when the linked case id is %s",
    (_label, buttonText, chatId) => {
      render(<SupportPage />);
      fireEvent.click(screen.getByText(buttonText));
      expect(mockNavigate).toHaveBeenCalledWith(
        `/projects/project-1/support/conversations/${chatId}`,
        {
          state: {
            conversationSummary: expect.objectContaining({ chatId }),
            returnTo: "/projects/project-1/support",
          },
        },
      );
    },
  );
});

