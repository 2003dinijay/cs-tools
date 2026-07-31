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

import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import type { CloneChangeRequestNavState } from "@features/csm-operations/utils/changeRequests";

const navigateMock = vi.fn();
const postChangeRequestMock = vi.fn();
let locationState: CloneChangeRequestNavState | undefined;

// The backend client reads runtime config at module load, which isn't
// present under vitest — same stub technique as
// CsmChangeRequestDetailPage.test.tsx.
vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {
    status: number;
    constructor(status: number, message: string) {
      super(message);
      this.status = status;
    }
  },
}));

vi.mock("react-router", () => ({
  useNavigate: () => navigateMock,
  useLocation: () => ({ state: locationState }),
}));

vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: vi.fn() }),
}));

vi.mock("@features/csm-operations/api/usePostChangeRequest", () => ({
  usePostChangeRequest: () => ({
    mutate: postChangeRequestMock,
    isPending: false,
  }),
}));

vi.mock("@features/settings/api/useGetUsersMe", () => ({
  useGetUsersMe: () => ({ data: undefined }),
}));

// This form's Lexical-based editor renders real content in a browser but not
// under jsdom in a way vitest can drive reliably — stub it to a plain
// textarea, same technique as EditCaseDetailsDialog.test.tsx.
vi.mock("@components/rich-text-editor/Editor", () => ({
  default: ({ value }: { value: string }) => (
    <textarea readOnly aria-label="editor" value={value} />
  ),
}));

const emptySearchResult = { data: [], isFetching: false, isError: false };
vi.mock("@api/useSearchGroups", () => ({ useSearchGroups: () => emptySearchResult }));
vi.mock("@api/useSearchItServices", () => ({ useSearchItServices: () => emptySearchResult }));
vi.mock("@api/useSearchServiceOfferings", () => ({
  useSearchServiceOfferings: () => emptySearchResult,
}));
vi.mock("@api/useSearchConfigurationItems", () => ({
  useSearchConfigurationItems: () => emptySearchResult,
}));
vi.mock("@api/useSearchUsersByName", () => ({
  useSearchUsersByName: () => emptySearchResult,
}));

// Imported after the mocks above so the module picks them up.
import CreateChangeRequestPage from "@features/csm-operations/pages/CreateChangeRequestPage";

describe("CreateChangeRequestPage — Clone prefill", () => {
  it("renders a blank form with no clone banner when opened directly (not cloned)", () => {
    locationState = undefined;
    render(<CreateChangeRequestPage />);
    expect(screen.getByLabelText(/subject/i)).toHaveValue("");
    expect(screen.queryByText(/cloned from/i)).not.toBeInTheDocument();
  });

  it("prefills subject, type, and impact from the clone state", () => {
    locationState = {
      sourceNumber: "CHG0009988",
      subject: "Upgrade the gateway cluster",
      type: "emergency",
      impact: "high",
    };
    render(<CreateChangeRequestPage />);
    expect(screen.getByLabelText(/subject/i)).toHaveValue("Upgrade the gateway cluster");
    expect(screen.getByText("Emergency")).toBeInTheDocument();
    expect(screen.getByText("High")).toBeInTheDocument();
  });

  it("shows a banner naming the source record and the fields that could not be copied", () => {
    locationState = { sourceNumber: "CHG0009988", subject: "Upgrade the gateway cluster" };
    render(<CreateChangeRequestPage />);
    expect(screen.getByText(/cloned from chg0009988/i)).toBeInTheDocument();
    expect(screen.getByText(/category, priority, risk/i)).toBeInTheDocument();
  });

  it("shows a generic banner when the source number is unavailable", () => {
    locationState = { subject: "Upgrade the gateway cluster" };
    render(<CreateChangeRequestPage />);
    expect(screen.getByText(/cloned from an existing change request/i)).toBeInTheDocument();
  });

  it("always resets state to 'new' regardless of the clone source", () => {
    locationState = { subject: "Upgrade the gateway cluster" };
    render(<CreateChangeRequestPage />);
    expect(screen.getByText("New")).toBeInTheDocument();
  });

  it("leaves the planned start/end schedule empty even when cloning", () => {
    locationState = { subject: "Upgrade the gateway cluster" };
    const { container } = render(<CreateChangeRequestPage />);
    // The MUI date-time picker renders a segmented group (day/month/year/…)
    // rather than a single-value input, so "empty" shows up as every segment
    // carrying an `aria-valuetext="Empty"` — there's no cloned start/end
    // date to display, for either the start or the end picker.
    const emptySegments = container.querySelectorAll('[aria-valuetext="Empty"]');
    expect(emptySegments.length).toBeGreaterThan(0);
  });
});
