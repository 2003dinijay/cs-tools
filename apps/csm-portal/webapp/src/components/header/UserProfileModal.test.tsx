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

import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { UsersMeResponse } from "@features/settings/api/useGetUsersMe";

const mutateMock = vi.fn();
const showErrorMock = vi.fn();
const showSuccessMock = vi.fn();
let userMe: UsersMeResponse | undefined;

vi.mock("@features/settings/api/useGetUsersMe", () => ({
  useGetUsersMe: () => ({ data: userMe, isLoading: false }),
}));
vi.mock("@features/settings/api/usePatchUsersMe", () => ({
  usePatchUsersMe: () => ({ mutate: mutateMock, isPending: false }),
}));
vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: showErrorMock }),
}));
vi.mock("@context/success-banner/SuccessBannerContext", () => ({
  useSuccessBanner: () => ({ showSuccess: showSuccessMock }),
}));
vi.mock("@hooks/useIdTokenClaims", () => ({
  useIdTokenClaims: () => ({ name: "Jane Doe", email: "jane@example.com" }),
}));

import UserProfileModal from "@components/header/UserProfileModal";

const PROFILE = "+15550001111";
const OLD_PLATFORM = "+15559998888";

function openDialog() {
  return render(<UserProfileModal open onClose={vi.fn()} />);
}

describe("UserProfileModal: the number Case Alerts call", () => {
  beforeEach(() => {
    mutateMock.mockReset();
    showErrorMock.mockReset();
    showSuccessMock.mockReset();
    userMe = {
      email: "jane@example.com",
      timeZone: "UTC",
      phoneNumber: PROFILE,
      platformPhone: OLD_PLATFORM,
    };
  });

  it("says so when alerts dial another number, and fixes it in one click", () => {
    openDialog();
    expect(
      screen.getByText(
        `Case Alerts calls go to ${OLD_PLATFORM}, the number on your platform record, not the number above.`,
      ),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Use this number for alerts" }));
    expect(mutateMock).toHaveBeenCalledTimes(1);
    expect(mutateMock.mock.calls[0][0]).toEqual({ phoneNumber: PROFILE });
  });

  it("Save with the shown number unchanged still sends it while alerts dial another", () => {
    openDialog();
    fireEvent.click(screen.getByLabelText("Edit phone number"));
    expect(screen.getByText(/Saving updates both\./)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(mutateMock).toHaveBeenCalledTimes(1);
    expect(mutateMock.mock.calls[0][0]).toEqual({ phoneNumber: PROFILE });
  });

  it("offers the repair when the platform record has no number", () => {
    userMe = { ...userMe, platformPhone: null };
    openDialog();
    expect(
      screen.getByText(/Your platform record has no phone number/),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Use this number for alerts" })).toBeInTheDocument();
  });

  it("has nothing to offer when only the platform record has a number", () => {
    userMe = { ...userMe, phoneNumber: undefined };
    openDialog();
    expect(
      screen.getByText(`Case Alerts calls go to ${OLD_PLATFORM}, which isn't shown above. Set your phone number above to change it.`),
    ).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Use this number for alerts" })).not.toBeInTheDocument();
  });

  it("says nothing, and Save sends nothing, when the two agree", () => {
    userMe = { ...userMe, platformPhone: "+1 555 000 1111" };
    openDialog();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    fireEvent.click(screen.getByLabelText("Edit phone number"));
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(mutateMock).not.toHaveBeenCalled();
  });

  it("says nothing when the platform copy is unknown (an older backend)", () => {
    userMe = { ...userMe, platformPhone: undefined };
    openDialog();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    fireEvent.click(screen.getByLabelText("Edit phone number"));
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(mutateMock).not.toHaveBeenCalled();
  });
});
