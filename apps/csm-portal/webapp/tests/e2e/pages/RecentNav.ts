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

import { type Locator, type Page, expect } from "@playwright/test";

/**
 * App-shell widgets under `src/features/csm-recent`: `PinThisPageButton`,
 * `PinnedTabs`, `QuickNav`, and `RecentViewsButton`. All state is
 * localStorage-only (`useRecentViews` hook) — there's no backing route or
 * API, so this POM has no `goto()`; use it from whatever page the pin/quick
 * nav/recent-views action is being exercised on.
 */
export class RecentNav {
  constructor(private readonly page: Page) {}

  // ── Pin the current page (`PinThisPageButton.tsx`) ──────────────────────

  /** Toggle button whose accessible name flips between "Pin this page to top
   * nav bar" and "Unpin this page" depending on state — also exposes
   * `aria-pressed`. */
  pinThisPageButton(): Locator {
    return this.page.getByRole("button", {
      name: /^(Pin this page to top nav bar|Unpin this page)$/,
    });
  }

  async isCurrentPagePinned(): Promise<boolean> {
    const pressed = await this.pinThisPageButton().getAttribute("aria-pressed");
    return pressed === "true";
  }

  /** Pins the current page if not already pinned; no-op otherwise. */
  async pinCurrentPage(): Promise<void> {
    if (!(await this.isCurrentPagePinned())) await this.pinThisPageButton().click();
  }

  /** Unpins the current page if pinned; no-op otherwise. */
  async unpinCurrentPage(): Promise<void> {
    if (await this.isCurrentPagePinned()) await this.pinThisPageButton().click();
  }

  // ── Pinned tabs strip (`PinnedTabs.tsx`) ────────────────────────────────

  /** A pinned-tab chip by its short label (the part of the recent-view title
   * before " · " — see `shortLabel` in the source).
   * `aria-label="{label} — pinned tab"`. */
  pinnedTab(label: string): Locator {
    return this.page.getByRole("button", { name: `${label} — pinned tab` });
  }

  async openPinnedTab(label: string): Promise<void> {
    await this.pinnedTab(label).click();
  }

  /** Deletes a pinned tab via its chip's built-in delete icon (MUI `Chip`
   * `onDelete` renders a small delete affordance nested inside the chip,
   * which itself carries the `{label} — pinned tab` accessible name). */
  async deletePinnedTab(label: string): Promise<void> {
    await this.pinnedTab(label).locator(".MuiChip-deleteIcon").click();
  }

  // ── Quick nav palette (`QuickNav.tsx`, ⌘K / Ctrl+K) ─────────────────────

  quickNavTrigger(): Locator {
    return this.page.getByRole("button", { name: "Search or jump to (open quick nav)" });
  }

  /** Right after a fresh navigation, the trigger button can be in the DOM
   * before React has finished attaching its click handler — a bare
   * click-then-assert can land in that gap and silently no-op. Retry the
   * click until the palette actually opens instead of trusting the first
   * click landed. */
  async openQuickNav(): Promise<void> {
    await expect(async () => {
      await this.quickNavTrigger().click();
      await expect(this.quickNavSearchInput()).toBeVisible({ timeout: 1_000 });
    }).toPass({ timeout: 10_000 });
  }

  /** `aria-label="Quick nav search"` — the palette's own search input,
   * distinct from any page-level search box. */
  quickNavSearchInput(): Locator {
    return this.page.getByLabel("Quick nav search");
  }

  async quickNavSearch(query: string): Promise<void> {
    await this.quickNavSearchInput().fill(query);
  }

  /** A quick-nav result row, by its visible label text (case id/subject,
   * pinned/recent title, or page name — see `Result.label` in the source;
   * rendered as a `Form.CardButton`/`QuickNavCaseCard`, neither of which
   * carries a fixed `aria-label`, so this matches on visible text instead). */
  quickNavResult(label: string): Locator {
    // Scope to the open QuickNav palette (a MUI `Modal` — `role="presentation"`
    // wrapper containing the search input) so a page-wide match can't hit
    // duplicate sidebar/page text outside the palette.
    const palette = this.page.locator('[role="presentation"]', {
      has: this.quickNavSearchInput(),
    });
    return palette.getByText(label, { exact: false }).first();
  }

  async chooseQuickNavResult(label: string): Promise<void> {
    await this.quickNavResult(label).click();
  }

  // ── Recently viewed panel (`RecentViewsButton.tsx`) ─────────────────────

  recentViewsButton(): Locator {
    return this.page.getByRole("button", { name: "Recently viewed" });
  }

  /** Same early-click race as {@link openQuickNav} — retry until the panel
   * is actually open rather than trusting the first click landed. */
  async openRecentViews(): Promise<void> {
    await expect(async () => {
      await this.recentViewsButton().click();
      // MUI's default variant mapping renders `subtitle2` as an `<h6>`, so the
      // panel's "Recently viewed" title registers as a heading.
      await expect(
        this.page.getByRole("heading", { name: "Recently viewed" }),
      ).toBeVisible({ timeout: 1_000 });
    }).toPass({ timeout: 10_000 });
  }

  /** A row in the Recently viewed panel, by its title text. The title is
   * regex-escaped so a value containing regex metacharacters (e.g. `[`, `(`)
   * doesn't break locator construction or match the wrong row. */
  recentViewRow(title: string): Locator {
    const escaped = title.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
    return this.page.getByRole("button", { name: new RegExp(`^${escaped}`) });
  }

  async openRecentView(title: string): Promise<void> {
    await this.recentViewRow(title).click();
  }

  /** Per-row pin toggle inside the Recently viewed panel —
   * `aria-label="Pin {title} to top nav bar"` /
   * `"Unpin {title} from top nav bar"`. */
  recentViewPinToggle(title: string): Locator {
    return this.page.getByRole("button", {
      name: new RegExp(`^(Pin|Unpin) ${title} (to|from) top nav bar$`),
    });
  }

  async togglePinFromRecentViews(title: string): Promise<void> {
    await this.recentViewPinToggle(title).click();
  }

  clearHistoryButton(): Locator {
    return this.page.getByRole("button", { name: "Clear history" });
  }

  async clearHistory(): Promise<void> {
    await this.clearHistoryButton().click();
  }
}
