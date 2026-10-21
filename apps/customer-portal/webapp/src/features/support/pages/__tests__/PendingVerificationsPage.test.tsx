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
import PendingVerificationsPage from "@features/support/pages/PendingVerificationsPage";
import type { PendingVerificationView } from "@features/support/types/pendingVerification";

const mockNavigate = vi.fn();
const mockShowSuccess = vi.fn();
const mockShowError = vi.fn();
const mockMutate = vi.fn();

const mockUseParams = vi.fn();
const mockUseLocation = vi.fn();
const mockUseSearchParams = vi.fn();
const mockUseGetProjectDetails = vi.fn();
const mockUsePendingVerificationsListSearch = vi.fn();

const fakeRecord: PendingVerificationView = {
  id: "pv-1",
  workItemId: "work-item-1",
  caseId: "CS0441241",
  caseTitle: "A case",
  recordType: "Case",
  addedReason: "manual",
  addedAt: "2026-10-01T00:00:00Z",
  addedBy: "someone@test.local",
};

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

vi.mock("@hooks/useDebouncedValue", () => ({
  useDebouncedValue: (value: string) => value,
}));

vi.mock("@api/useGetProjectDetails", () => ({
  default: () => mockUseGetProjectDetails(),
}));

vi.mock("@features/support/api/usePendingVerificationsListSearch", () => ({
  usePendingVerificationsListSearch: (...args: unknown[]) =>
    mockUsePendingVerificationsListSearch(...args),
}));

vi.mock("@features/support/api/useVerifyPendingVerification", () => ({
  useVerifyPendingVerification: () => ({ mutate: mockMutate, isPending: false }),
}));

vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: mockShowError }),
}));

vi.mock("@context/success-banner/SuccessBannerContext", () => ({
  useSuccessBanner: () => ({ showSuccess: mockShowSuccess }),
}));

vi.mock("@features/support/utils/pendingVerification", () => ({
  buildPendingVerificationRecordPath: () => "/projects/project-1/support/cases/work-item-1",
}));

vi.mock("@components/list-view/ListPageHeader", () => ({
  default: ({
    title,
    description,
    onBack,
  }: {
    title: string;
    description: string;
    onBack?: () => void;
  }) => (
    <div>
      <h1>{title}</h1>
      <p>{description}</p>
      {onBack && <button onClick={onBack}>Back</button>}
    </div>
  ),
}));

vi.mock("@components/list-view/ListSearchBar", () => ({
  default: ({
    searchTerm,
    onSearchChange,
  }: {
    searchTerm: string;
    onSearchChange: (value: string) => void;
  }) => (
    <input
      aria-label="search"
      value={searchTerm}
      onChange={(e) => onSearchChange(e.target.value)}
    />
  ),
}));

vi.mock("@components/list-view/ListResultsBar", () => ({
  default: () => <div>ResultsBar</div>,
}));

vi.mock("@components/list-view/ListPagination", () => ({
  default: () => <div>Pagination</div>,
}));

vi.mock("@features/support/components/pending-verifications/PendingVerificationsList", () => ({
  default: ({
    hasListRefinement,
    emptyMessage,
    onViewCase,
    onMarkVerified,
  }: {
    hasListRefinement?: boolean;
    emptyMessage?: string;
    onViewCase?: (record: PendingVerificationView) => void;
    onMarkVerified?: (record: PendingVerificationView) => void;
  }) => (
    <div>
      <span data-testid="has-list-refinement">{String(!!hasListRefinement)}</span>
      <span data-testid="empty-message">{emptyMessage ?? ""}</span>
      <button onClick={() => onViewCase?.(fakeRecord)}>View Case</button>
      <button onClick={() => onMarkVerified?.(fakeRecord)}>Mark Verified</button>
    </div>
  ),
}));

vi.mock("@features/support/components/case-details/dialogs/CaseStateConfirmDialog", () => ({
  default: ({
    open,
    onConfirm,
  }: {
    open: boolean;
    onConfirm: () => void;
  }) => (open ? <button onClick={onConfirm}>Confirm Verify</button> : null),
}));

describe("PendingVerificationsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseParams.mockReturnValue({ projectId: "project-1" });
    mockUseLocation.mockReturnValue({ pathname: "/projects/project-1/verifications", search: "" });
    mockUseSearchParams.mockReturnValue([new URLSearchParams(), vi.fn()]);
    mockUseGetProjectDetails.mockReturnValue({
      data: { verificationEnabled: true },
    });
    mockUsePendingVerificationsListSearch.mockReturnValue({
      data: { pendingVerifications: [], totalRecords: 0 },
      isLoading: false,
      isError: false,
    });
  });

  it("redirects away when verification is not enabled for the project", () => {
    mockUseGetProjectDetails.mockReturnValue({ data: { verificationEnabled: false } });

    render(<PendingVerificationsPage />);

    expect(mockNavigate).toHaveBeenCalledWith("/projects/project-1/dashboard", { replace: true });
  });

  it("does not redirect while verification is enabled", () => {
    render(<PendingVerificationsPage />);

    expect(mockNavigate).not.toHaveBeenCalled();
  });

  it("defaults to the unverified tab with no addedReason filter", () => {
    render(<PendingVerificationsPage />);

    expect(screen.getByText("Closed records awaiting your sign-off for this project")).toBeInTheDocument();
    const [, filters] = mockUsePendingVerificationsListSearch.mock.calls[0];
    expect(filters).toEqual({ searchQuery: undefined, includeVerified: false, addedReason: undefined });
  });

  it("reads tab=verified from the URL and searches with verifiedOnly", () => {
    mockUseSearchParams.mockReturnValue([new URLSearchParams("tab=verified"), vi.fn()]);

    render(<PendingVerificationsPage />);

    expect(screen.getByText("Records you've already signed off on for this project")).toBeInTheDocument();
    const [, filters] = mockUsePendingVerificationsListSearch.mock.calls[0];
    expect(filters).toEqual({ searchQuery: undefined, verifiedOnly: true });
  });

  it("reads addedReason from the URL and narrows the unverified search", () => {
    mockUseSearchParams.mockReturnValue([
      new URLSearchParams("tab=unverified&addedReason=auto-closed"),
      vi.fn(),
    ]);

    render(<PendingVerificationsPage />);

    const [, filters] = mockUsePendingVerificationsListSearch.mock.calls[0];
    expect(filters).toEqual({ searchQuery: undefined, includeVerified: false, addedReason: "auto-closed" });
  });

  it("ignores an unrecognized addedReason value", () => {
    mockUseSearchParams.mockReturnValue([
      new URLSearchParams("tab=unverified&addedReason=bogus"),
      vi.fn(),
    ]);

    render(<PendingVerificationsPage />);

    const [, filters] = mockUsePendingVerificationsListSearch.mock.calls[0];
    expect(filters).toEqual({ searchQuery: undefined, includeVerified: false, addedReason: undefined });
  });

  // Regression tests for the CodeRabbit finding: a stat-box deep link
  // (?addedReason=...) matching zero records must show the "try adjusting
  // your filters" empty state, not the generic one, which would otherwise
  // falsely claim there's nothing pending at all.
  it("treats an addedReason filter on the unverified tab as a list refinement", () => {
    mockUseSearchParams.mockReturnValue([
      new URLSearchParams("tab=unverified&addedReason=auto-closed"),
      vi.fn(),
    ]);

    render(<PendingVerificationsPage />);

    expect(screen.getByTestId("has-list-refinement")).toHaveTextContent("true");
  });

  it("does not treat addedReason as a refinement on the verified tab, where it has no effect", () => {
    mockUseSearchParams.mockReturnValue([
      new URLSearchParams("tab=verified&addedReason=auto-closed"),
      vi.fn(),
    ]);

    render(<PendingVerificationsPage />);

    expect(screen.getByTestId("has-list-refinement")).toHaveTextContent("false");
  });

  it("passes a tab-specific empty message for the verified tab", () => {
    mockUseSearchParams.mockReturnValue([new URLSearchParams("tab=verified"), vi.fn()]);

    render(<PendingVerificationsPage />);

    expect(screen.getByTestId("empty-message")).toHaveTextContent("No verified records yet.");
  });

  it("leaves the empty message as the component's own default on the unverified tab", () => {
    render(<PendingVerificationsPage />);

    expect(screen.getByTestId("empty-message")).toHaveTextContent("");
  });

  it("navigates back to the support page, not browser-back", () => {
    render(<PendingVerificationsPage />);

    fireEvent.click(screen.getByText("Back"));

    expect(mockNavigate).toHaveBeenCalledWith("/projects/project-1/support");
  });

  it("includes the current path and query string as returnTo when opening a case", () => {
    mockUseLocation.mockReturnValue({
      pathname: "/projects/project-1/verifications",
      search: "?tab=unverified&addedReason=auto-closed",
    });

    render(<PendingVerificationsPage />);
    fireEvent.click(screen.getByText("View Case"));

    expect(mockNavigate).toHaveBeenCalledWith(
      "/projects/project-1/support/cases/work-item-1",
      { state: { returnTo: "/projects/project-1/verifications?tab=unverified&addedReason=auto-closed" } },
    );
  });

  it("passes the debounced search term through to the search hook", () => {
    render(<PendingVerificationsPage />);

    fireEvent.change(screen.getByLabelText("search"), { target: { value: "CS0441241" } });

    const lastCall = mockUsePendingVerificationsListSearch.mock.calls.at(-1);
    expect(lastCall?.[1]).toEqual({ searchQuery: "CS0441241", includeVerified: false, addedReason: undefined });
  });

  it("marks a record verified through the confirm dialog, then shows a success banner", () => {
    mockMutate.mockImplementation((_id, { onSuccess }: { onSuccess: () => void }) => onSuccess());

    render(<PendingVerificationsPage />);

    fireEvent.click(screen.getByText("Mark Verified"));
    fireEvent.click(screen.getByText("Confirm Verify"));

    expect(mockMutate).toHaveBeenCalledWith("pv-1", expect.objectContaining({ onSuccess: expect.any(Function) }));
    expect(mockShowSuccess).toHaveBeenCalledWith("Case marked as verified.");
  });

  it("shows an error banner when marking verified fails", () => {
    mockMutate.mockImplementation((_id, { onError }: { onError: (err: Error) => void }) =>
      onError(new Error("boom")),
    );

    render(<PendingVerificationsPage />);

    fireEvent.click(screen.getByText("Mark Verified"));
    fireEvent.click(screen.getByText("Confirm Verify"));

    expect(mockShowError).toHaveBeenCalledWith("boom");
  });
});
