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

import { act, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";

const getMock = vi.fn();
const postMock = vi.fn();

vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ get: getMock, post: postMock }),
}));

import { useImportRunbooks, useRunbooks, useUnmergeRunbookAlerts } from "@features/csm-operations/api/useRunbooks";

const runbook = {
  id: "1",
  number: "RB0000001",
  runbookUrl: null,
  alerts: ["a", "b"],
  createdOn: "",
  createdBy: "",
  updatedOn: "",
  updatedBy: "",
};

function setup() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  const wrapper = ({ children }: { children: ReactNode }) => <QueryClientProvider client={client}>{children}</QueryClientProvider>;
  return renderHook(() => ({ list: useRunbooks(), unmerge: useUnmergeRunbookAlerts(), importRunbooks: useImportRunbooks() }), {
    wrapper,
  });
}

describe("useRunbooks mutations", () => {
  beforeEach(() => {
    getMock.mockReset().mockResolvedValue({ runbooks: [runbook] });
    postMock.mockReset();
  });

  it("reads the catalog again after an unmerge, and posts the alerts to the runbook's unmerge route", async () => {
    postMock.mockResolvedValue({ runbooks: [runbook] });
    const { result } = setup();
    await waitFor(() => expect(result.current.list.data).toHaveLength(1));
    expect(getMock).toHaveBeenCalledTimes(1);

    await act(async () => {
      await result.current.unmerge.mutateAsync({ id: "1", alerts: ["a"] });
    });
    expect(postMock).toHaveBeenCalledWith("/runbooks/1/unmerge", { alerts: ["a"] });
    await waitFor(() => expect(getMock).toHaveBeenCalledTimes(2));
  });

  it("reads the catalog again after a refused unmerge too, and hands the error to the caller", async () => {
    postMock.mockRejectedValue(new Error("a runbook must keep at least one alert"));
    const { result } = setup();
    await waitFor(() => expect(result.current.list.data).toHaveLength(1));

    let error: unknown;
    await act(async () => {
      await result.current.unmerge.mutateAsync({ id: "1", alerts: ["a", "b"] }).catch((e: unknown) => (error = e));
    });
    expect((error as Error).message).toBe("a runbook must keep at least one alert");
    await waitFor(() => expect(getMock).toHaveBeenCalledTimes(2));
  });

  it("posts an import and reads the catalog again", async () => {
    postMock.mockResolvedValue({ created: 1, updated: 0, removed: [] });
    const { result } = setup();
    await waitFor(() => expect(result.current.list.data).toHaveLength(1));

    await act(async () => {
      await result.current.importRunbooks.mutateAsync([{ runbookUrl: null, alerts: ["c"] }]);
    });
    expect(postMock).toHaveBeenCalledWith("/runbooks/import", { runbooks: [{ runbookUrl: null, alerts: ["c"] }] });
    await waitFor(() => expect(getMock).toHaveBeenCalledTimes(2));
  });
});
