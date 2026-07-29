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

import { render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import AppErrorBoundary from "@components/error/AppErrorBoundary";

/** A component that always throws during render, to trip the boundary. */
function Bomb(): never {
  throw new Error("boom");
}

describe("AppErrorBoundary", () => {
  const originalLocation = window.location;
  const originalClipboard = navigator.clipboard;
  let writeText: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    // Silence the expected console.error from componentDidCatch and the
    // "not implemented" jsdom navigation warning.
    vi.spyOn(console, "error").mockImplementation(() => undefined);

    // jsdom throws on direct window.location.href assignment; stub it out
    // like the existing "Reload page" tests (via window.location.reload)
    // would need to, so the click handler can be exercised safely.
    Object.defineProperty(window, "location", {
      configurable: true,
      value: { ...originalLocation, href: "https://csm.example.com/cases/1", reload: vi.fn() },
    });

    writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText },
    });
  });

  afterEach(() => {
    Object.defineProperty(window, "location", {
      configurable: true,
      value: originalLocation,
    });
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: originalClipboard,
    });
    vi.restoreAllMocks();
  });

  function renderCrashed(): void {
    render(
      <AppErrorBoundary>
        <Bomb />
      </AppErrorBoundary>,
    );
  }

  it("shows the fallback UI once a descendant throws", () => {
    renderCrashed();
    expect(screen.getByText("Something went wrong")).toBeInTheDocument();
  });

  it("navigates to the app root when 'Go to home' is clicked", () => {
    renderCrashed();
    screen.getByRole("button", { name: /go to home/i }).click();
    expect(window.location.href).toBe("/");
  });

  it("copies a plain-text error report including the component stack and URL", async () => {
    renderCrashed();
    screen.getByRole("button", { name: /copy error details/i }).click();

    await waitFor(() => expect(writeText).toHaveBeenCalledTimes(1));
    const copied = writeText.mock.calls[0][0] as string;

    expect(copied).toContain("CSM Portal error report");
    expect(copied).toContain("URL: https://csm.example.com/cases/1");
    expect(copied).toContain("Error: Error: boom");
    expect(copied).toContain("Component stack:");
    expect(copied).not.toContain("Correlation ID:");

    await waitFor(() =>
      expect(screen.getByRole("button", { name: /copy error details/i })).toBeInTheDocument(),
    );
  });
});
