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
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { DeviceType, MobileOs } from "@/types/mobileDevice";

const mockConfig = {
  enabled: true,
  iosStoreUrl: "https://apps.apple.com/app/example",
  androidStoreUrl: "https://play.google.com/store/apps/details?id=example",
  includeTablets: false,
};

vi.mock("@utils/deviceDetection", () => ({
  detectMobileDevice: vi.fn(),
}));

vi.mock("@config/mobileAppConfig", () => ({
  getMobileAppConfig: vi.fn(() => mockConfig),
  getMobileAppStoreUrl: (os: string) =>
    os === "ios" ? mockConfig.iosStoreUrl : mockConfig.androidStoreUrl,
}));

import { detectMobileDevice } from "@utils/deviceDetection";
import MobileAppBanner from "@components/mobile-app-banner/MobileAppBanner";

describe("MobileAppBanner", () => {
  beforeEach(() => {
    mockConfig.enabled = true;
    mockConfig.iosStoreUrl = "https://apps.apple.com/app/example";
    mockConfig.androidStoreUrl =
      "https://play.google.com/store/apps/details?id=example";
    mockConfig.includeTablets = false;
    vi.stubGlobal("open", vi.fn());
  });

  afterEach(() => {
    vi.clearAllMocks();
    vi.unstubAllGlobals();
  });

  it("renders nothing on desktop (no device detected)", () => {
    vi.mocked(detectMobileDevice).mockReturnValue(null);

    render(<MobileAppBanner />);

    expect(screen.queryByText("Get the WSO2 Super App")).toBeNull();
  });

  it("renders nothing when the prompt is disabled even on a mobile device", () => {
    mockConfig.enabled = false;
    vi.mocked(detectMobileDevice).mockReturnValue({
      os: MobileOs.Ios,
      deviceType: DeviceType.Phone,
    });

    render(<MobileAppBanner />);

    expect(screen.queryByText("Get the WSO2 Super App")).toBeNull();
  });

  it("renders nothing when no store URL is configured for the detected OS", () => {
    mockConfig.iosStoreUrl = undefined as unknown as string;
    vi.mocked(detectMobileDevice).mockReturnValue({
      os: MobileOs.Ios,
      deviceType: DeviceType.Phone,
    });

    render(<MobileAppBanner />);

    expect(screen.queryByText("Get the WSO2 Super App")).toBeNull();
  });

  it("shows the banner on a detected mobile phone", () => {
    vi.mocked(detectMobileDevice).mockReturnValue({
      os: MobileOs.Android,
      deviceType: DeviceType.Phone,
    });

    render(<MobileAppBanner />);

    expect(screen.getByText("Get the WSO2 Super App")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /download/i })).toBeInTheDocument();
  });

  it("dismisses on close and stays hidden until re-mounted with a new visible state", () => {
    vi.mocked(detectMobileDevice).mockReturnValue({
      os: MobileOs.Ios,
      deviceType: DeviceType.Phone,
    });

    render(<MobileAppBanner />);
    expect(screen.getByText("Get the WSO2 Super App")).toBeInTheDocument();

    const dismissButton = screen.getByRole("button", { name: /close/i });
    fireEvent.click(dismissButton);

    expect(screen.queryByText("Get the WSO2 Super App")).toBeNull();
  });

  it("opens the store URL via window.open when the download action is used", () => {
    vi.mocked(detectMobileDevice).mockReturnValue({
      os: MobileOs.Ios,
      deviceType: DeviceType.Phone,
    });

    render(<MobileAppBanner />);
    fireEvent.click(screen.getByRole("button", { name: /download/i }));

    expect(window.open).toHaveBeenCalledWith(
      "https://apps.apple.com/app/example",
      "_blank",
      "noopener,noreferrer",
    );
  });

  it("blocks a javascript: store URL instead of opening it", () => {
    mockConfig.iosStoreUrl = "javascript:alert('xss')";
    vi.mocked(detectMobileDevice).mockReturnValue({
      os: MobileOs.Ios,
      deviceType: DeviceType.Phone,
    });

    render(<MobileAppBanner />);
    fireEvent.click(screen.getByRole("button", { name: /download/i }));

    expect(window.open).not.toHaveBeenCalled();
  });
});
