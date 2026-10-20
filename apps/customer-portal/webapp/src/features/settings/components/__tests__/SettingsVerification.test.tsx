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
import SettingsVerification from "@features/settings/components/SettingsVerification";

const mutateMock = vi.fn();
const mockUseGetProjectDetails = vi.fn();

vi.mock("@api/useGetProjectDetails", () => ({
  default: () => mockUseGetProjectDetails(),
}));

vi.mock("@features/settings/api/usePatchProject", () => ({
  usePatchProject: () => ({
    mutate: mutateMock,
    isPending: false,
  }),
}));

vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: vi.fn() }),
}));

vi.mock("@context/success-banner/SuccessBannerContext", () => ({
  useSuccessBanner: () => ({ showSuccess: vi.fn() }),
}));

vi.mock("@utils/useDarkMode", () => ({ useDarkMode: () => false }));

describe("SettingsVerification", () => {
  it("renders and toggles the verification setting", () => {
    mockUseGetProjectDetails.mockReturnValue({
      data: { verificationEnabled: false },
      isLoading: false,
      isError: false,
    });

    render(<SettingsVerification projectId="p-1" canEdit />);

    expect(screen.getByText("Inactive")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("switch"));
    expect(mutateMock).toHaveBeenCalledWith({ verificationEnabled: true }, expect.any(Object));
  });

  // Regression test for the CodeRabbit finding: a React Query *disabled*
  // query (e.g. a brief authentication-loading transition after the
  // component has already mounted) reports isLoading=false and isError=false
  // with no data at all -- neither of the two existing guards catches that,
  // so the switch stayed enabled and showed "Inactive" even though the
  // project's real verificationEnabled value was never actually fetched.
  it("disables the switch when project details are unavailable but not reported as loading or erroring", () => {
    mockUseGetProjectDetails.mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: false,
    });

    render(<SettingsVerification projectId="p-1" canEdit />);

    expect(screen.getByRole("switch")).toBeDisabled();
  });

  it("disables the switch while project details are loading", () => {
    mockUseGetProjectDetails.mockReturnValue({
      data: undefined,
      isLoading: true,
      isError: false,
    });

    render(<SettingsVerification projectId="p-1" canEdit />);

    expect(screen.getByRole("switch")).toBeDisabled();
  });

  // Regression test for the CodeRabbit finding: a failed project-details
  // fetch must not be treated the same as "verification is off" -- the
  // switch stays disabled and the UI says so, instead of silently showing
  // "Inactive" and letting an admin PATCH a value whose real current state
  // is unknown.
  it("disables the switch and shows a load error when project details fail to fetch", () => {
    mockUseGetProjectDetails.mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: true,
    });

    render(<SettingsVerification projectId="p-1" canEdit />);

    expect(screen.getByRole("switch")).toBeDisabled();
    expect(screen.queryByText("Inactive")).not.toBeInTheDocument();
    expect(screen.queryByText("Active")).not.toBeInTheDocument();
    expect(screen.getByTestId("error-indicator")).toBeInTheDocument();
    expect(
      screen.getByText(/Could not load the current Pending Verification setting/),
    ).toBeInTheDocument();
  });
});
