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
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import FilteredCsvExportButton from "@components/FilteredCsvExportButton";

const showErrorMock = vi.fn();
const showSuccessMock = vi.fn();

vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: showErrorMock }),
}));
vi.mock("@context/success-banner/SuccessBannerContext", () => ({
  useSuccessBanner: () => ({ showSuccess: showSuccessMock }),
}));
vi.mock("@utils/saveBlob", () => ({ saveBlob: vi.fn() }));

interface Row {
  id: number;
}

function deferred<T>(): { promise: Promise<T>; resolve: (v: T) => void; reject: (e: unknown) => void } {
  let resolve!: (v: T) => void;
  let reject!: (e: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

describe("FilteredCsvExportButton", () => {
  beforeEach(() => {
    showErrorMock.mockClear();
    showSuccessMock.mockClear();
  });

  it("is disabled when the caller says there's nothing to export", () => {
    render(
      <FilteredCsvExportButton<Row>
        entityName="widgets"
        header={["ID"]}
        toRow={(r) => [String(r.id)]}
        fetchPage={async () => ({ items: [], total: 0 })}
        disabled
      />,
    );
    expect(screen.getByRole("button", { name: /export csv/i })).toBeDisabled();
  });

  it("shows a success banner with the row count after a clean export", async () => {
    const fetchPage = vi.fn(async () => ({ items: [{ id: 1 }, { id: 2 }], total: 2 }));
    render(
      <FilteredCsvExportButton<Row>
        entityName="widgets"
        entityNounPlural="widgets"
        header={["ID"]}
        toRow={(r) => [String(r.id)]}
        fetchPage={fetchPage}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: /export csv/i }));
    await waitFor(() => expect(showSuccessMock).toHaveBeenCalledWith("Exported 2 widgets to CSV."));
    expect(showErrorMock).not.toHaveBeenCalled();
  });

  it("shows a busy state with a live count while paging, then re-enables", async () => {
    const first = deferred<{ items: Row[]; total: number }>();
    const fetchPage = vi
      .fn()
      .mockReturnValueOnce(first.promise)
      .mockResolvedValueOnce({ items: [{ id: 2 }], total: 2 });

    render(
      <FilteredCsvExportButton<Row>
        entityName="widgets"
        header={["ID"]}
        toRow={(r) => [String(r.id)]}
        fetchPage={fetchPage}
        pageSize={1}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: /export csv/i }));

    // The busy variant is wrapped in a Tooltip (different subtree), so
    // re-query rather than holding onto the pre-click button node.
    await waitFor(() =>
      expect(screen.getByRole("button")).toHaveAttribute("aria-busy", "true"),
    );
    expect(screen.getByRole("button")).toBeDisabled();

    first.resolve({ items: [{ id: 1 }], total: 2 });

    await waitFor(() =>
      expect(screen.getByRole("button", { name: /export csv/i })).not.toBeDisabled(),
    );
  });

  it("surfaces a clear truncation notice (not a generic error) when the row cap is hit", async () => {
    const fetchPage = vi.fn(async () => ({
      items: [{ id: 1 }],
      total: 500,
    }));
    render(
      <FilteredCsvExportButton<Row>
        entityName="widgets"
        entityNounPlural="widgets"
        header={["ID"]}
        toRow={(r) => [String(r.id)]}
        fetchPage={fetchPage}
        pageSize={1}
        maxRows={1}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: /export csv/i }));
    await waitFor(() =>
      expect(showErrorMock).toHaveBeenCalledWith(
        expect.stringMatching(/stopped at 1 of 500 widgets.*narrow your filters/i),
      ),
    );
  });

  it("reports a failure without claiming an export happened", async () => {
    const fetchPage = vi.fn().mockRejectedValue(new Error("network down"));
    render(
      <FilteredCsvExportButton<Row>
        entityName="widgets"
        header={["ID"]}
        toRow={(r) => [String(r.id)]}
        fetchPage={fetchPage}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: /export csv/i }));
    await waitFor(() =>
      expect(showErrorMock).toHaveBeenCalledWith(
        expect.stringMatching(/could not complete the export.*no file was downloaded/i),
        expect.any(Error),
      ),
    );
    expect(showSuccessMock).not.toHaveBeenCalled();
  });
});
