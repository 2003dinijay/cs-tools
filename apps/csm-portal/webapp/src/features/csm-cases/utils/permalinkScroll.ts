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

/**
 * Walk the parent chain to find the nearest vertically-scrollable element.
 * Falls back to the document scrolling element if none is found.
 */
export function findVerticalScrollAncestor(el: HTMLElement): HTMLElement {
  let cur: HTMLElement | null = el.parentElement;
  while (cur && cur !== document.body) {
    const style = window.getComputedStyle(cur);
    const overflowY = style.overflowY;
    if (
      (overflowY === "auto" || overflowY === "scroll") &&
      cur.scrollHeight > cur.clientHeight
    ) {
      return cur;
    }
    cur = cur.parentElement;
  }
  return (document.scrollingElement as HTMLElement | null) ?? document.documentElement;
}

export interface ScrollToFragmentOptions {
  /** Delay before the first lookup attempt, in ms. Default 100. */
  initialDelayMs?: number;
  /** Delay between retries once the first attempt misses, in ms. Default 150. */
  retryMs?: number;
  /** Total lookup attempts before giving up. Default 10. */
  maxAttempts?: number;
  /** Vertical offset (px) kept above the target once scrolled. Default 96. */
  scrollOffset?: number;
  /** How long the highlight tint stays, in ms, before fading. Default 1500. */
  highlightMs?: number;
  /** Called once every attempt has missed — the id never appeared in the DOM
   * (deleted entry, or one the viewer can't see), as opposed to "still
   * loading". */
  onNotFound?: () => void;
  /** Injectable for tests. Defaults to `document.getElementById`. */
  getElementById?: (id: string) => HTMLElement | null;
  /** Injectable for tests. Defaults to {@link findVerticalScrollAncestor}. */
  findScrollAncestor?: (el: HTMLElement) => HTMLElement;
}

/**
 * Scrolls a fragment-linked entry (a comment, audit entry, or attachment row)
 * into view and briefly highlights it, retrying the DOM lookup until the
 * target actually exists.
 *
 * Twitter-style permalinks land on a target that's only in the DOM once the
 * activity feed has finished loading — comments, the linked chat transcript,
 * and the audit trail can each resolve at different times, especially on a
 * cold (new-tab) load. A single fixed-delay attempt is a race; this retries on
 * an interval and only reports "not found" once every attempt has missed.
 *
 * The browser's own hash-anchor `scrollIntoView` also drags ancestors
 * horizontally if any of them is wider than the viewport (e.g. a comment with
 * a wide `<pre>` block) — undone here by zeroing `scrollLeft` on every
 * ancestor before doing our own vertical-only scroll.
 *
 * @returns A cleanup function that cancels any pending timers.
 */
export function scrollToFragmentWithRetry(
  hash: string,
  options: ScrollToFragmentOptions = {},
): () => void {
  const {
    initialDelayMs = 100,
    retryMs = 150,
    maxAttempts = 10,
    scrollOffset = 96,
    highlightMs = 1500,
    onNotFound,
    getElementById = (id: string) => document.getElementById(id),
    findScrollAncestor = findVerticalScrollAncestor,
  } = options;

  const timers: ReturnType<typeof setTimeout>[] = [];
  let attempt = 0;

  /**
   * One attempt to locate the target and bring it into view. Re-schedules itself
   * up to `maxAttempts` times while the element is absent — the activity feed
   * renders from three independently-resolving sources, so "not found yet" is
   * the normal case on a cold load rather than an error. Calls `onNotFound` only
   * once the attempts are exhausted, which is the genuine "this entry does not
   * exist or you cannot see it" signal.
   */
  const tryScrollAndHighlight = (): void => {
    const target = getElementById(hash);
    if (!target) {
      attempt += 1;
      if (attempt < maxAttempts) {
        timers.push(setTimeout(tryScrollAndHighlight, retryMs));
      } else {
        onNotFound?.();
      }
      return;
    }

    let cur: HTMLElement | null = target.parentElement;
    while (cur && cur !== document.body) {
      if (cur.scrollLeft !== 0) cur.scrollLeft = 0;
      cur = cur.parentElement;
    }
    if (document.documentElement.scrollLeft !== 0) {
      document.documentElement.scrollLeft = 0;
    }
    if (document.body.scrollLeft !== 0) document.body.scrollLeft = 0;

    const container = findScrollAncestor(target);
    const containerTop =
      container === document.documentElement
        ? 0
        : container.getBoundingClientRect().top;
    const targetTop = target.getBoundingClientRect().top;
    const delta = targetTop - containerTop - scrollOffset;
    container.scrollTo({
      top: container.scrollTop + delta,
      behavior: "smooth",
    });

    const prevTransition = target.style.transition;
    const prevBg = target.style.backgroundColor;
    target.style.transition = "background-color 200ms ease-out";
    target.style.backgroundColor = "rgba(255, 213, 79, 0.35)";
    timers.push(
      setTimeout(() => {
        target.style.backgroundColor = prevBg;
        timers.push(
          setTimeout(() => {
            target.style.transition = prevTransition;
          }, 350),
        );
      }, highlightMs),
    );
  };

  timers.push(setTimeout(tryScrollAndHighlight, initialDelayMs));
  return () => timers.forEach(clearTimeout);
}
