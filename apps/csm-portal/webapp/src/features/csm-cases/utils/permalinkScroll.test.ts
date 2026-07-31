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

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { scrollToFragmentWithRetry } from "./permalinkScroll";

beforeEach(() => {
  vi.useFakeTimers();
});

afterEach(() => {
  vi.useRealTimers();
  document.body.innerHTML = "";
});

describe("scrollToFragmentWithRetry", () => {
  it("retries until an asynchronously-loaded target appears, then scrolls and highlights it", () => {
    // Simulates the exact race the bug report describes: the comment trail
    // (and therefore the fragment target) isn't in the DOM yet when the first
    // lookup attempt fires — it's inserted only after the activity feed's
    // async sources finish loading, a few "retries" later.
    let target: HTMLElement | null = null;
    const getElementById = vi.fn(() => target);
    const scrollTo = vi.fn();
    const findScrollAncestor = vi.fn(() => {
      const container = document.createElement("div");
      Object.defineProperty(container, "scrollTo", { value: scrollTo });
      Object.defineProperty(container, "getBoundingClientRect", {
        value: () => ({ top: 0 }),
      });
      return container as unknown as HTMLElement;
    });

    const onNotFound = vi.fn();
    scrollToFragmentWithRetry("comment-42", {
      initialDelayMs: 10,
      retryMs: 10,
      maxAttempts: 5,
      getElementById,
      findScrollAncestor,
      onNotFound,
    });

    // First attempt: nothing in the DOM yet.
    vi.advanceTimersByTime(10);
    expect(getElementById).toHaveBeenCalledTimes(1);
    expect(scrollTo).not.toHaveBeenCalled();

    // Second attempt still misses.
    vi.advanceTimersByTime(10);
    expect(getElementById).toHaveBeenCalledTimes(2);

    // Now the target lands in the DOM (as if the async feed just finished
    // rendering it) before the next retry fires.
    target = document.createElement("div");
    document.body.appendChild(target);
    Object.defineProperty(target, "getBoundingClientRect", {
      value: () => ({ top: 500 }),
    });

    vi.advanceTimersByTime(10);
    expect(scrollTo).toHaveBeenCalledTimes(1);
    expect(target.style.backgroundColor).not.toBe("");
    expect(onNotFound).not.toHaveBeenCalled();
  });

  it("reports not-found once every attempt misses, instead of retrying forever or failing silently", () => {
    const getElementById = vi.fn(() => null);
    const onNotFound = vi.fn();

    scrollToFragmentWithRetry("comment-deleted", {
      initialDelayMs: 10,
      retryMs: 10,
      maxAttempts: 3,
      getElementById,
      onNotFound,
    });

    vi.advanceTimersByTime(10 * 4);
    expect(getElementById).toHaveBeenCalledTimes(3);
    expect(onNotFound).toHaveBeenCalledTimes(1);
  });

  it("scrolls and highlights immediately when the target already exists", () => {
    const target = document.createElement("div");
    document.body.appendChild(target);
    Object.defineProperty(target, "getBoundingClientRect", {
      value: () => ({ top: 300 }),
    });
    const scrollTo = vi.fn();
    const findScrollAncestor = vi.fn(() => {
      const container = document.createElement("div");
      Object.defineProperty(container, "scrollTo", { value: scrollTo });
      Object.defineProperty(container, "getBoundingClientRect", {
        value: () => ({ top: 0 }),
      });
      return container as unknown as HTMLElement;
    });

    scrollToFragmentWithRetry("comment-1", {
      initialDelayMs: 10,
      getElementById: () => target,
      findScrollAncestor,
    });

    vi.advanceTimersByTime(10);
    expect(scrollTo).toHaveBeenCalledTimes(1);
    // Highlight tint applied.
    expect(target.style.backgroundColor).toBe("rgba(255, 213, 79, 0.35)");
  });

  it("cancels pending timers on cleanup, so a fast unmount / hash change can't fire late", () => {
    const getElementById = vi.fn(() => null);
    const onNotFound = vi.fn();

    const cleanup = scrollToFragmentWithRetry("comment-1", {
      initialDelayMs: 10,
      retryMs: 10,
      maxAttempts: 3,
      getElementById,
      onNotFound,
    });

    cleanup();
    vi.advanceTimersByTime(1000);
    expect(getElementById).not.toHaveBeenCalled();
    expect(onNotFound).not.toHaveBeenCalled();
  });
});
