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
import { CASES } from "../utils/selectors";

export type CaseDetailTabId = keyof typeof CASES.detailTabs;

/**
 * Page object for `/cases/:caseId` (`CsmCaseDetailPage.tsx`) — also reused
 * for `/engagements/:caseId`, `/operations/service-requests/:caseId`, and
 * `/announcements/:caseId` (the same component renders all four, branching
 * only on the route prefix — see `isEngagementRoute` etc. in the source).
 * Pass the matching `basePath` to the constructor for those.
 */
export class CaseDetailPage {
  constructor(
    private readonly page: Page,
    private readonly basePath: string = CASES.path,
  ) {}

  async goto(caseId: string): Promise<void> {
    await this.page.goto(`${this.basePath}/${caseId}`);
    // The heading is the case's own subject (dynamic), so wait on the
    // case-identity line + tab strip instead of any fixed text.
    await expect(this.page.getByRole("tablist")).toBeVisible();
  }

  // ── Tabs ─────────────────────────────────────────────────────────────────

  /** A tab by its stable id (see `CASES.detailTabs`). Matched by a
   * label-prefix regex, not an exact string — once a tab's widget has data
   * its accessible name gets a trailing " (N)" count appended (e.g.
   * "Attachments (3)"; see `TAB_DEFS`/`count` in `CsmCaseDetailPage.tsx`). */
  tab(id: CaseDetailTabId): Locator {
    const label = CASES.detailTabs[id];
    return this.page.getByRole("tab", { name: new RegExp(`^${label}(\\s\\(\\d+\\))?$`) });
  }

  async openTab(id: CaseDetailTabId): Promise<void> {
    await this.tab(id).click();
  }

  // ── Action bar: primary lifecycle button / "Change state" menu ──────────

  /** True when the case has exactly one reachable next state — the action
   * bar renders it directly as a single contained button in that case
   * (`primary.length === 1` in `CaseActionBar.tsx`) instead of a "Change
   * state" dropdown. */
  async hasSingleLifecycleButton(): Promise<boolean> {
    return this.page.getByRole("button", { name: "Change state" }).isHidden();
  }

  changeStateButton(): Locator {
    return this.page.getByRole("button", { name: "Change state" });
  }

  /** Runs a lifecycle transition by its button label (e.g. "Start progress",
   * "Propose solution", "Request information", "Wait on WSO2", "Close",
   * "Assign to me", "Resume work") — opens the "Change state" menu first if
   * there's more than one reachable transition, otherwise clicks the single
   * primary button directly. */
  async changeState(label: string): Promise<void> {
    const single = this.page.getByRole("button", { name: label, exact: true });
    if (await single.isVisible().catch(() => false)) {
      await single.click();
      return;
    }
    await this.changeStateButton().click();
    await this.page.getByRole("menuitem", { name: label, exact: true }).click();
  }

  // ── "More" overflow menu ─────────────────────────────────────────────────

  moreButton(): Locator {
    return this.page.getByRole("button", { name: "More" });
  }

  /** Opens the "More" menu and clicks the named item (e.g. "Assign /
   * reassign engineer…", "Change severity…", "Edit case details…", "Manage
   * watchers…", "Hold auto-closure…", "Link to another case…", "Create
   * task…", "Set fix ETA…", "Request a call…", "Log time…", "Copy case
   * link", "Pause work"/"Resume work", "Raise internal Git issue…") — see
   * `buildSecondaryItems` in `CaseActionBar.tsx` for the full, state-gated
   * list. */
  async moreAction(label: string): Promise<void> {
    await this.moreButton().click();
    await this.page.getByRole("menuitem", { name: label }).click();
  }

  // ── Comment composer (Activities tab) ───────────────────────────────────

  /** The collapsed "Compose a reply…" / "Add an internal work note…" faux
   * input — click it to expand the real composer. No-op if already open
   * (the button doesn't render once expanded). */
  async openComposer(): Promise<void> {
    const opener = this.page.getByRole("button", {
      name: /Compose a reply to the customer…|Add an internal work note…/,
    });
    if (await opener.isVisible().catch(() => false)) await opener.click();
  }

  /** The "Internal note" switch — checked means the comment posts as a
   * work note (not customer-visible). */
  internalNoteSwitch(): Locator {
    return this.page.getByRole("switch", { name: "Internal note" });
  }

  /** The comment editor's content-editable region (fixed `data-testid` from
   * `Editor.tsx`, shared with every rich-text field in this app — not an
   * id invented for this suite). Scoped to the composer Card so it doesn't
   * collide with, e.g., the description editor on a create page. */
  commentEditor(): Locator {
    return this.page.getByTestId("case-description-editor");
  }

  /** Submit button — label flips between "Send to customer" and "Save work
   * note" depending on the Internal note switch (`CsmCaseCommentInput.tsx`). */
  commentSubmitButton(): Locator {
    return this.page.getByRole("button", { name: /Send to customer|Save work note/ });
  }

  /** Opens the composer (if collapsed), sets internal/public, types `text`,
   * and submits. Only meaningful on the Activities tab. */
  async addComment(text: string, opts: { internal?: boolean } = {}): Promise<void> {
    await this.openComposer();
    const wantInternal = !!opts.internal;
    const isChecked = await this.internalNoteSwitch().isChecked();
    if (isChecked !== wantInternal) await this.internalNoteSwitch().click();
    await this.commentEditor().click();
    await this.commentEditor().fill(text);
    await this.commentSubmitButton().click();
  }

  // ── Attachments (Attachments tab) ────────────────────────────────────────

  /** Attachments-tab upload button ("Upload", `AttachmentsWidget` in
   * `CaseDetailWidgets.tsx`) — distinct from the comment composer's
   * "Choose a file to attach" flow below. */
  async uploadAttachment(filePath: string): Promise<void> {
    // The Upload button reveals a hidden native file input alongside it
    // (no accessible name of its own — `aria-hidden`); Playwright's
    // `setInputFiles` works on a hidden input directly, no need to click
    // "Upload" first.
    await this.page.locator('input[type="file"]').first().setInputFiles(filePath);
  }

  downloadAttachmentButton(filename: string): Locator {
    return this.page.getByRole("button", { name: `Download ${filename}` });
  }

  async downloadAttachment(filename: string): Promise<void> {
    await this.downloadAttachmentButton(filename).click();
  }

  deleteAttachmentButton(filename: string): Locator {
    return this.page.getByRole("button", { name: `Delete ${filename}` });
  }

  /** Clicks the per-row delete icon and confirms the "Delete attachment?"
   * dialog. */
  async deleteAttachment(filename: string): Promise<void> {
    await this.deleteAttachmentButton(filename).click();
    await this.page.getByRole("dialog").getByRole("button", { name: "Delete" }).click();
  }

  // ── Attach-to-comment flow (composer's paperclip button) ────────────────

  /** Opens the comment composer's "Attach a file" modal via the rich-text
   * toolbar's attachment button, and picks `filePath` — this is the
   * `CsmUploadAttachmentModal.tsx` flow (distinct from the Attachments tab's
   * direct upload above): the file is staged, not uploaded, until the
   * comment is sent. */
  async attachFileToComment(filePath: string): Promise<void> {
    await this.page.locator('input[type="file"]').first().setInputFiles(filePath);
    await this.page.getByRole("button", { name: "Add" }).click();
  }

  // ── Tags (Details tab) ───────────────────────────────────────────────────

  /** "Tag" button in the Tags widget (Details tab). */
  async openAddTagDialog(): Promise<void> {
    await this.page.getByRole("button", { name: "Tag" }).click();
  }

  tagField(): Locator {
    return this.page.getByRole("combobox", { name: /^Tag\s*\*?$/ });
  }

  async addTag(label: string): Promise<void> {
    await this.openAddTagDialog();
    await this.tagField().fill(label);
    await this.page.getByRole("button", { name: "Add tag" }).click();
  }

  /** Removes a tag chip by its label (MUI `Chip`'s built-in delete icon,
   * whose accessible name is undecorated — matched via the chip's own
   * text). */
  async removeTag(label: string): Promise<void> {
    await this.page
      .locator(".MuiChip-root", { hasText: label })
      .locator(".MuiChip-deleteIcon")
      .click();
  }

  // ── Watchers (Related tab) ───────────────────────────────────────────────

  /** The "Add watcher" toggle on the Watchers widget (Related tab) — opens
   * the inline `WatcherAddPicker` search panel below the watcher chips. There
   * is no separate "Manage watchers" dialog anymore (`WatchersWidget` in
   * `CaseDetailWidgets.tsx`) — the "Manage watchers…" item in the "More" menu
   * just jumps to this tab. */
  addWatcherButton(): Locator {
    return this.page.getByRole("button", { name: "Add watcher" });
  }

  watcherSearchField(): Locator {
    return this.page.getByPlaceholder("Search people to add…");
  }

  /** Candidate rows render as plain buttons containing the person's name and
   * email — matching on "@" picks the first real candidate row without
   * needing to know who staging happens to return (mirrors the create-task
   * assignee picker). */
  watcherCandidate(): Locator {
    return this.page.getByRole("button").filter({ hasText: "@" }).first();
  }

  /** Opens the inline add-watcher panel, searches `query`, and clicks the
   * first matching candidate. Returns the candidate's display name (parsed
   * out of the button's combined "name" + "email" text) so the caller can
   * assert the resulting watcher chip without depending on the empty-state
   * text. Only meaningful on the Related tab. */
  async addWatcher(query: string): Promise<string> {
    await this.addWatcherButton().click();
    await this.watcherSearchField().fill(query);
    const candidate = this.watcherCandidate();
    await candidate.waitFor({ state: "visible", timeout: 10_000 });
    const raw = (await candidate.textContent()) ?? "";
    const email = raw.match(/[\w.+-]+@[\w.-]+\.[A-Za-z]{2,}/)?.[0] ?? "";
    const name = raw.replace(email, "").trim();
    await candidate.click();
    return name;
  }

  /** A watcher chip by its display name (`WatchersWidget`'s `Chip` list). */
  watcherChip(name: string): Locator {
    return this.page.locator(".MuiChip-root", { hasText: name });
  }

  // ── Tasks (reachable only via "More" → "Create task…" — see note below) ──

  /** The Tasks tab itself is currently `hidden: true` (unreachable via tab
   * navigation — see `CASES.detailTabs`'s doc comment), but the underlying
   * feature is untouched: "Create task…" in the "More" menu still opens
   * `CreateTaskDialog` and posts `POST /cases/{id}/tasks` regardless. There is
   * no visible list to confirm the created task against while the tab stays
   * hidden — assert on the "Task created." feedback banner instead (see
   * `onCreateTask` in `CsmCaseDetailPage.tsx`). */
  createTaskButton(): Locator {
    return this.page.getByRole("button", { name: "Create task" });
  }

  taskSubjectField(): Locator {
    return this.page.getByRole("textbox", { name: /^Subject\s*\*?$/ });
  }

  /** Submits the "Create task" dialog (opened via "More" → "Create task…")
   * with just the required Subject field. */
  async createTask(subject: string): Promise<void> {
    await this.taskSubjectField().fill(subject);
    await this.page.getByRole("dialog").getByRole("button", { name: "Create task" }).click();
  }

  /** A task row on the Tasks tab, by its accessible name
   * (`aria-label="View task {subject}"`, `TasksWidget.tsx`). Not reachable
   * while the Tasks tab stays hidden — kept for when it's un-hidden. */
  taskRow(subject: string): Locator {
    return this.page.getByRole("button", { name: `View task ${subject}` });
  }

  async refreshTasks(): Promise<void> {
    await this.page.getByRole("button", { name: "Refresh tasks" }).click();
  }

  // ── Call requests tab ─────────────────────────────────────────────────────

  createCallRequestButton(): Locator {
    return this.page.getByRole("button", { name: "Create call request" });
  }

  /** Fills and submits the "Create a call request" dialog: reason, duration,
   * and at least one preferred time slot. `slotDateTime` must already
   * satisfy the case-severity-dependent minimum lead time (see
   * `LEAD_TIME_MINUTES_BY_SEVERITY` in `CreateCallRequestDialog.tsx`) — pass
   * something comfortably in the future (e.g. +1 week) to avoid flakiness. */
  async createCallRequest(opts: {
    reason: string;
    durationMinutes: number;
    slotDateTime: Date;
  }): Promise<void> {
    await this.createCallRequestButton().click();
    const dialog = this.page.getByRole("dialog");
    await dialog.getByRole("textbox", { name: /^Reason for the call\s*\*?$/ }).fill(opts.reason);
    await dialog.getByRole("textbox", { name: /^Duration \(minutes\)\s*\*?$/ }).fill(String(opts.durationMinutes));
    await this.fillDateTimeField(
      dialog.getByRole("group").first(),
      opts.slotDateTime,
    );
    await dialog.getByRole("button", { name: "Submit request" }).click();
  }

  /** Opens the "Schedule call"/"Reschedule call" dialog for a call-request
   * row (via `CallRequestsTable`'s row action) and submits it. Callers must
   * open the row's action menu themselves first (table markup isn't fixed
   * here) — this only drives the dialog once it's open. */
  async submitScheduleDialog(opts: {
    durationMinutes?: number;
    assignee?: string;
  }): Promise<void> {
    const dialog = this.page.getByRole("dialog");
    if (opts.durationMinutes !== undefined) {
      await dialog.getByRole("textbox", { name: /^Duration \(minutes\)\s*\*?$/ }).fill(String(opts.durationMinutes));
    }
    if (opts.assignee) {
      await dialog.getByRole("textbox", { name: /^Assignee \(optional\)\s*\*?$/ }).fill(opts.assignee);
    }
    await dialog.getByRole("button", { name: /^(Schedule|Reschedule)$/ }).click();
  }

  /** Submits the "Reject call request" dialog (optional reason). Caller
   * opens the row's "Reject" action first. */
  async submitRejectDialog(reason?: string): Promise<void> {
    const dialog = this.page.getByRole("dialog");
    if (reason) await dialog.getByRole("textbox", { name: /^Reason \(optional\)\s*\*?$/ }).fill(reason);
    await dialog.getByRole("button", { name: "Reject" }).click();
  }

  /**
   * Best-effort helper for MUI X `DateTimePicker` fields, which render as a
   * single sectioned `role="group"` (no plain `<input>` to `.fill()`). Clicks
   * the group then types the US-locale `MM/DD/YYYY hh:mm AM/PM` string
   * key-by-key — MUI X's field auto-advances between sections on a valid
   * separator/value, which is the standard way these fields are driven in
   * Playwright without a bespoke MUI adapter. Not exercised by this repo's
   * own tests yet — verify against a live picker before relying on it.
   */
  private async fillDateTimeField(group: Locator, value: Date): Promise<void> {
    const mm = String(value.getMonth() + 1).padStart(2, "0");
    const dd = String(value.getDate()).padStart(2, "0");
    const yyyy = String(value.getFullYear());
    let hours = value.getHours();
    const ampm = hours >= 12 ? "PM" : "AM";
    hours = hours % 12 || 12;
    const hh = String(hours).padStart(2, "0");
    const min = String(value.getMinutes()).padStart(2, "0");
    await group.click();
    await this.page.keyboard.type(`${mm}${dd}${yyyy}${hh}${min}${ampm}`, { delay: 30 });
  }
}
