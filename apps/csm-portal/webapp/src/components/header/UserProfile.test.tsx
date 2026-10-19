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
import UserProfile from "@components/header/UserProfile";
import { ThemePreferenceProvider } from "@context/theme/ThemePreferenceContext";
import { CaseTabsBehaviorProvider } from "@context/case-tabs/CaseTabsBehaviorContext";
import { ErrorBannerProvider } from "@context/error-banner/ErrorBannerContext";
import { SuccessBannerProvider } from "@context/success-banner/SuccessBannerContext";

vi.mock("@asgardeo/react", () => ({
  useAsgardeo: () => ({
    isSignedIn: true,
    signOut: vi.fn(),
    getDecodedIdToken: vi.fn(),
  }),
}));

vi.mock("@hooks/useIdTokenClaims", () => ({
  useIdTokenClaims: () => ({
    name: "Jane Doe",
    email: "jane.doe@example.com",
  }),
}));

vi.mock("@hooks/useLogger", () => ({
  useLogger: () => ({ error: vi.fn(), warn: vi.fn(), info: vi.fn(), debug: vi.fn() }),
}));

vi.mock("@features/settings/api/useGetUsersMe", () => ({
  useGetUsersMe: () => ({ data: undefined, isLoading: false }),
}));

vi.mock("@features/settings/api/usePatchUsersMe", () => ({
  usePatchUsersMe: () => ({ mutate: vi.fn(), isPending: false }),
}));

// usePortalAccess (which the Feature Preview gating reads) derives from
// useCurrentUser's roles -- mocked the same way CsmUsersPage.test.tsx does,
// with a mutable roles/id a test can set before rendering. Defaults to no
// roles, matching every pre-existing test in this file.
let mockRoles: string[] | undefined;
let mockUserId: string | undefined = "user-1";
vi.mock("@context/current-user/CurrentUserContext", () => ({
  useCurrentUser: () => ({
    user: { roles: mockRoles, id: mockUserId },
    isLoading: false,
    isError: false,
  }),
}));

function renderUserProfile(props: { hideProjectControls?: boolean } = {}) {
  return render(
    <ErrorBannerProvider>
      <SuccessBannerProvider>
        <ThemePreferenceProvider>
          <CaseTabsBehaviorProvider>
            <UserProfile {...props} />
          </CaseTabsBehaviorProvider>
        </ThemePreferenceProvider>
      </SuccessBannerProvider>
    </ErrorBannerProvider>,
  );
}

describe("UserProfile", () => {
  beforeEach(() => {
    localStorage.clear();
    mockRoles = undefined;
    mockUserId = "user-1";
  });

  it("offers Preferences below Profile in the profile menu, and no separate Sign out gap", async () => {
    renderUserProfile();
    // Open the profile popup menu via its trigger.
    fireEvent.click(screen.getByRole("button", { name: "Account" }));

    const profileItem = await screen.findByText("Profile");
    const preferencesItem = await screen.findByText("Preferences");
    const signOutItem = await screen.findByText("Sign out");

    // "Preferences" sits between "Profile" and "Sign out" — DOM order proxies
    // visual order here since all three are simple sibling menu items.
    const position = (el: Element) =>
      Array.from(el.closest("ul")?.children ?? []).indexOf(el.closest("li") ?? el);
    expect(position(preferencesItem)).toBeGreaterThan(position(profileItem));
    expect(position(signOutItem)).toBeGreaterThan(position(preferencesItem));
  });

  it("opens the Preferences dialog (not the Profile modal) from its own menu item", async () => {
    renderUserProfile();
    fireEvent.click(screen.getByRole("button", { name: "Account" }));
    fireEvent.click(await screen.findByText("Preferences"));

    // The Preferences dialog's own content — theme + case-tabs settings —
    // not the Profile modal's (roles/phone/time zone).
    expect(await screen.findByLabelText("Select theme")).toBeInTheDocument();
    expect(screen.getByLabelText("Open cases in tabs")).toBeInTheDocument();
    expect(screen.queryByText("Time zone")).not.toBeInTheDocument();
  });

  it("still opens the Profile modal from its own menu item", async () => {
    renderUserProfile();
    fireEvent.click(screen.getByRole("button", { name: "Account" }));
    fireEvent.click(await screen.findByText("Profile"));

    expect(await screen.findByText("Time zone")).toBeInTheDocument();
    expect(screen.queryByLabelText("Select theme")).not.toBeInTheDocument();
  });

  it("hides Profile/Preferences but keeps identity and Sign out when hideProjectControls is set", async () => {
    renderUserProfile({ hideProjectControls: true });
    fireEvent.click(screen.getByRole("button", { name: "Account" }));

    expect(await screen.findByText("Jane Doe")).toBeInTheDocument();
    expect(screen.getByText("Sign out")).toBeInTheDocument();
    expect(screen.queryByText("Profile")).not.toBeInTheDocument();
    expect(screen.queryByText("Preferences")).not.toBeInTheDocument();
  });

  describe("Feature Preview", () => {
    it("does not render for a non-admin", async () => {
      mockRoles = ["cs_engineer"];
      renderUserProfile();
      fireEvent.click(screen.getByRole("button", { name: "Account" }));
      await screen.findByText("Preferences");
      expect(screen.queryByText("Feature Preview")).not.toBeInTheDocument();
    });

    it("renders for an admin, below Preferences and above Sign out", async () => {
      mockRoles = ["admin"];
      renderUserProfile();
      fireEvent.click(screen.getByRole("button", { name: "Account" }));

      const preferencesItem = await screen.findByText("Preferences");
      const previewItem = await screen.findByText("Feature Preview");
      const signOutItem = await screen.findByText("Sign out");

      const position = (el: Element) =>
        Array.from(el.closest("ul")?.children ?? []).indexOf(el.closest("li") ?? el);
      expect(position(previewItem)).toBeGreaterThan(position(preferencesItem));
      expect(position(signOutItem)).toBeGreaterThan(position(previewItem));
    });

    it("does not render for an admin when hideProjectControls is set", async () => {
      mockRoles = ["admin"];
      renderUserProfile({ hideProjectControls: true });
      fireEvent.click(screen.getByRole("button", { name: "Account" }));
      await screen.findByText("Sign out");
      expect(screen.queryByText("Feature Preview")).not.toBeInTheDocument();
    });

    it("opens the Feature Preview dialog from its own menu item", async () => {
      mockRoles = ["admin"];
      renderUserProfile();
      fireEvent.click(screen.getByRole("button", { name: "Account" }));
      fireEvent.click(await screen.findByText("Feature Preview"));

      expect(await screen.findByText("Feature Preview", { selector: "h6" })).toBeInTheDocument();
      expect(
        screen.getByText(/in-progress features this deployment has hidden/i),
      ).toBeInTheDocument();
    });
  });
});
