/**
 * Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

const getMock = vi.fn();
vi.mock("@api/backend/client", () => ({ useBackendApi: () => ({ get: getMock }) }));
vi.mock("@config/apiConfig", () => ({ apiConfig: { backendUrl: "https://example.test" } }));

import { useRotaNightCrew } from "./useGenerateRota";

function wrapper({ children }: { children: ReactNode }) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
}

describe("useRotaNightCrew", () => {
  it("is null, not an empty crew, when the server has no night crew endpoint (404)", async () => {
    // The client resolves a GET that 404s to null.
    getMock.mockResolvedValueOnce(null);
    const { result } = renderHook(() => useRotaNightCrew("2026-10", "2026-11"), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toBeNull();
    expect(getMock).toHaveBeenCalledWith("/team-schedule/rotas/SRE_SAAS/night-crew?from=2026-10&to=2026-11");
  });

  it("passes the months through", async () => {
    const crew = { rotaCode: "SRE_SAAS", months: [{ month: "2026-11", crew: [{ userId: "u-1", teamKey: "apollo" }] }] };
    getMock.mockResolvedValueOnce(crew);
    const { result } = renderHook(() => useRotaNightCrew("2026-11", "2026-11"), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toEqual(crew);
  });

  it("asks nothing for a month that is not YYYY-MM", () => {
    getMock.mockClear();
    renderHook(() => useRotaNightCrew("2026-1", "2026-11"), { wrapper });
    expect(getMock).not.toHaveBeenCalled();
  });
});
