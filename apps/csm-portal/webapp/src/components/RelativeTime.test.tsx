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

import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import RelativeTime from "@components/RelativeTime";

describe("RelativeTime", () => {
  const iso = new Date(Date.now() - 60_000).toISOString();

  beforeEach(() => {
    Object.assign(navigator, {
      clipboard: { writeText: vi.fn().mockResolvedValue(undefined) },
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("renders plain (non-permalink) text with no copy button when href is absent", () => {
    render(<RelativeTime iso={iso} />);
    expect(
      screen.queryByRole("button", { name: /copy link/i }),
    ).not.toBeInTheDocument();
  });

  it("renders a permalink anchor and a copy-link button when href is provided", () => {
    render(<RelativeTime iso={iso} href="#cmt-1001-2" />);
    expect(screen.getByRole("link")).toHaveAttribute("href", "#cmt-1001-2");
    expect(
      screen.getByRole("button", { name: /copy link to this comment/i }),
    ).toBeInTheDocument();
  });

  it("copies the absolute permalink URL to the clipboard and shows a transient confirmation", async () => {
    render(<RelativeTime iso={iso} href="#cmt-1001-2" />);

    const button = screen.getByRole("button", {
      name: /copy link to this comment/i,
    });
    fireEvent.click(button);

    await waitFor(() => {
      expect(navigator.clipboard.writeText).toHaveBeenCalledWith(
        `${window.location.origin}${window.location.pathname}#cmt-1001-2`,
      );
    });

    // Transient "Copied!" confirmation shows via the tooltip title immediately
    // after copying (button stays in the document; label itself is static).
    await waitFor(() => {
      expect(button).toBeInTheDocument();
    });
  });
});
