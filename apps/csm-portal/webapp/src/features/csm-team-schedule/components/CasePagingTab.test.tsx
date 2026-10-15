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

import type { ReactElement } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  fireEvent,
  render as rtlRender,
  screen,
  within,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import type {
  CaseAlertSettings,
  PagingChainResponse,
  PagingReadinessResponse,
} from "../types";
import {
  pagingChain,
  pagingMember,
  readinessChain,
  readinessGap,
} from "../test/fixtures";
import { ADD_TO_PROFILE_HINT } from "../utils/pagingPhone";
// vi.mock below is hoisted above this import, so the panel gets the mocks.
import CasePagingTab from "./CasePagingTab";

const mutateAsync = vi.fn();
const putMutate = vi.fn();
const delMutate = vi.fn();
const testMutate = vi.fn();
const readiness: { data?: PagingReadinessResponse } = {};
const chain: { data?: PagingChainResponse } = {};

function render(ui: ReactElement) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return rtlRender(<QueryClientProvider client={qc}>{ui}</QueryClientProvider>);
}

vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {
    status = 0;
  },
  useBackendApi: () => ({}),
}));
vi.mock("@config/apiConfig", () => ({
  apiConfig: { backendUrl: "https://example.test" },
}));
vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: vi.fn() }),
}));
const alertSettings: { data?: CaseAlertSettings } = {};
const noopMutation = {
  mutate: vi.fn(),
  mutateAsync: vi.fn(),
  isPending: false,
};
vi.mock("../api/useCaseAlertSettings", () => ({
  useCaseAlertSettings: () => ({
    data: alertSettings.data,
    isLoading: false,
    isError: false,
  }),
  canViewCaseAlertSettings: (s?: CaseAlertSettings) =>
    Boolean(s?.canManage.view),
  useSaveCaseAlertRoom: () => noopMutation,
  useSetCaseAlertWebhook: () => noopMutation,
  useTestCaseAlertRoom: () => noopMutation,
  useDeleteCaseAlertRoom: () => noopMutation,
  useSaveCaseAlertRoute: () => noopMutation,
  useDeleteCaseAlertRoute: () => noopMutation,
  useSaveCaseAlertValues: () => noopMutation,
  useDeleteCaseAlertValue: () => noopMutation,
}));

/** Settings a CSM admin may see and change. */
function settingsFor(view: boolean): CaseAlertSettings {
  return {
    canManage: {
      view,
      roomLadders: view ? ["CRE", "SRE"] : [],
      routeLadders: view ? ["CRE", "SRE"] : [],
    },
    environment: "staging",
    webhooksEnabled: true,
    revision: { saved: 1 },
    rooms: [],
    routes: [],
    values: [],
  };
}

const VIEW_KEY = "csm-ts.alert-settings.view";

vi.mock("../api/usePagingChain", () => ({
  useGetPagingChain: () => ({
    data: chain.data,
    isLoading: false,
    isError: false,
    dataUpdatedAt: 0,
  }),
  usePatchPagingMember: () => ({ mutateAsync }),
  useGetPagingReadiness: () => ({
    data: readiness.data,
    isLoading: false,
    isError: false,
  }),
  usePutPagingContact: () => ({ mutate: putMutate, isPending: false }),
  useDeletePagingContact: () => ({ mutate: delMutate, isPending: false }),
  useTestPagingContact: () => ({ mutate: testMutate, isPending: false }),
}));

beforeEach(() => {
  vi.restoreAllMocks();
  window.localStorage.clear();
  alertSettings.data = undefined;
  mutateAsync.mockReset();
  putMutate.mockReset();
  delMutate.mockReset();
  testMutate.mockReset();
  chain.data = pagingChain();
  readiness.data = {
    generatedAt: "2026-10-08T03:00:00Z",
    from: "2026-10-08",
    to: "2026-10-14",
    chains: [
      readinessChain({ chain: "CRE", label: "CRE" }),
      readinessChain({
        chain: "SRE_SAAS",
        label: "SaaS SRE",
        ready: false,
        gaps: [
          readinessGap({
            code: "NO_L1",
            message: "No L1 in TZ2",
            fix: "rota",
            zoneCode: "TZ2",
            date: "2026-10-10",
          }),
        ],
      }),
      readinessChain({ chain: "SRE_IAAS", label: "IaaS SRE" }),
      readinessChain({ chain: "SME", label: "SME" }),
    ],
  };
});

describe("CasePagingTab", () => {
  it("opens on CRE: the editable chain, with its readiness", () => {
    render(<CasePagingTab onOpenRota={vi.fn()} />);
    expect(screen.getByRole("tab", { name: "CRE" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    expect(
      screen.getByRole("button", { name: /Edit alert chain/ }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("table", { name: "CRE alert chain by team and level" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("columnheader", { name: /^Alpha/ }),
    ).toBeInTheDocument();
    expect(screen.getByText(/✓ ready for the next 7 days/)).toBeInTheDocument();
  });

  it("switches to a rota'd chain: read-only, its own readiness, and a way to the rota", () => {
    const onOpenRota = vi.fn();
    render(<CasePagingTab onOpenRota={onOpenRota} />);
    fireEvent.click(screen.getByRole("tab", { name: "SaaS SRE" }));

    expect(screen.getByRole("tab", { name: "SaaS SRE" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    expect(
      screen.queryByRole("button", { name: /Edit alert chain/ }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByText(
        /People come from the SaaS SRE rota; set them in the Team Schedule views/,
      ),
    ).toBeInTheDocument();
    expect(screen.getByText(/⚠ 1 gap/)).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Open the SaaS SRE rota" }),
    );
    expect(onOpenRota).toHaveBeenLastCalledWith({
      family: "SRE",
      rotaCode: "SRE_SAAS",
    });

    // A rota gap opens the rota on its zone and day.
    fireEvent.click(screen.getByRole("button", { name: /Open the rota on/ }));
    expect(onOpenRota).toHaveBeenLastCalledWith({
      family: "SRE",
      rotaCode: "SRE_SAAS",
      teamKey: undefined,
      zoneCode: "TZ2",
      date: "2026-10-10",
    });
  });

  it("opens SME on its family, leaving the rota to the page", () => {
    const onOpenRota = vi.fn();
    render(<CasePagingTab onOpenRota={onOpenRota} />);
    fireEvent.click(screen.getByRole("tab", { name: "SME" }));
    fireEvent.click(screen.getByRole("button", { name: "Open the SME rota" }));
    expect(onOpenRota).toHaveBeenCalledWith({
      family: "SME",
      rotaCode: undefined,
    });
  });

  it("opens on the reader's own chain when given one, and the picker still switches", () => {
    render(<CasePagingTab onOpenRota={vi.fn()} initialChain="SME" />);
    expect(screen.getByRole("tab", { name: "SME" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    expect(
      screen.getByRole("button", { name: "Open the SME rota" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("table", {
        name: "CRE alert chain by team and level",
      }),
    ).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("tab", { name: "CRE" }));
    expect(screen.getByRole("tab", { name: "CRE" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    expect(
      screen.getByRole("table", { name: "CRE alert chain by team and level" }),
    ).toBeInTheDocument();
  });

  it("opens an IaaS SRE reader on the IaaS SRE chain", () => {
    render(<CasePagingTab onOpenRota={vi.fn()} initialChain="SRE_IAAS" />);
    expect(screen.getByRole("tab", { name: "IaaS SRE" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    expect(
      screen.getByRole("button", { name: "Open the IaaS SRE rota" }),
    ).toBeInTheDocument();
  });

  it("offers every chain when given none to keep to (a CSM admin), opening on the reader's own", () => {
    render(<CasePagingTab onOpenRota={vi.fn()} initialChain="SME" />);
    expect(screen.getAllByRole("tab")).toHaveLength(5);
    expect(screen.getByRole("tab", { name: "SME" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
  });

  it("offers the chains in order: CRE, the three SRE sub-teams together, then SME", () => {
    render(<CasePagingTab onOpenRota={vi.fn()} />);
    expect(screen.getAllByRole("tab").map((tab) => tab.textContent)).toEqual([
      "CRE",
      "SaaS SRE",
      "IaaS SRE",
      "PaaS SRE",
      "SME",
    ]);
  });

  it("leaves CRE out for an SRE rota admin, opening on SaaS SRE", () => {
    alertSettings.data = {
      ...settingsFor(true),
      canManage: { view: true, roomLadders: ["SRE"], routeLadders: ["SRE"] },
    };
    chain.data = pagingChain({
      canEdit: {
        responderTeams: [],
        teamLeadTeams: [],
        americasTeamLead: false,
        heads: false,
      },
    });
    render(<CasePagingTab onOpenRota={vi.fn()} />);
    expect(screen.getAllByRole("tab").map((tab) => tab.textContent)).toEqual([
      "SaaS SRE",
      "IaaS SRE",
      "PaaS SRE",
      "SME",
    ]);
    expect(screen.getByRole("tab", { name: "SaaS SRE" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
  });

  it("keeps CRE for an SRE rota admin who also leads a CRE team", () => {
    alertSettings.data = {
      ...settingsFor(true),
      canManage: { view: true, roomLadders: ["SRE"], routeLadders: ["SRE"] },
    };
    chain.data = pagingChain({
      canEdit: {
        responderTeams: ["alpha"],
        teamLeadTeams: [],
        americasTeamLead: false,
        heads: false,
      },
    });
    render(<CasePagingTab onOpenRota={vi.fn()} />);
    expect(screen.getByRole("tab", { name: "CRE" })).toBeInTheDocument();
  });

  it("keeps CRE for a CSM admin, and for anyone without settings rights", () => {
    alertSettings.data = settingsFor(true);
    const { unmount } = render(<CasePagingTab onOpenRota={vi.fn()} />);
    expect(screen.getByRole("tab", { name: "CRE" })).toBeInTheDocument();
    unmount();
    alertSettings.data = undefined;
    chain.data = pagingChain({
      canEdit: {
        responderTeams: [],
        teamLeadTeams: [],
        americasTeamLead: false,
        heads: false,
      },
    });
    render(<CasePagingTab onOpenRota={vi.fn()} />);
    expect(screen.getByRole("tab", { name: "CRE" })).toBeInTheDocument();
  });

  it("opens a CSM or portal admin on CRE, whatever their own team's chain, and still offers every chain", () => {
    alertSettings.data = settingsFor(true);
    render(<CasePagingTab onOpenRota={vi.fn()} initialChain="SRE_SAAS" />);
    expect(screen.getByRole("tab", { name: "CRE" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    expect(screen.getAllByRole("tab")).toHaveLength(5);
    // Their own pick still wins.
    fireEvent.click(screen.getByRole("tab", { name: "IaaS SRE" }));
    expect(screen.getByRole("tab", { name: "IaaS SRE" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
  });

  it("still opens an SRE rota admin on SaaS SRE", () => {
    alertSettings.data = {
      ...settingsFor(true),
      canManage: { view: true, roomLadders: ["SRE"], routeLadders: ["SRE"] },
    };
    render(<CasePagingTab onOpenRota={vi.fn()} initialChain="SRE_SAAS" />);
    expect(screen.getByRole("tab", { name: "SaaS SRE" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
  });

  it("still opens a reader without settings rights on their own chain", () => {
    alertSettings.data = settingsFor(false);
    render(<CasePagingTab onOpenRota={vi.fn()} initialChain="SRE_IAAS" />);
    expect(screen.getByRole("tab", { name: "IaaS SRE" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
  });

  it("shows PaaS SRE, says why it cannot be picked, and stays put when clicked", () => {
    render(<CasePagingTab onOpenRota={vi.fn()} />);
    const paas = screen.getByRole("tab", { name: "PaaS SRE" });
    expect(paas).toHaveAttribute("aria-disabled", "true");
    expect(paas).toHaveAttribute("title", "No PaaS rota yet");
    fireEvent.click(paas);
    expect(screen.getByRole("tab", { name: "CRE" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    expect(paas).toHaveAttribute("aria-selected", "false");
  });

  it("offers an SRE rota admin only the SRE chains they run, SaaS and IaaS", () => {
    render(<CasePagingTab onOpenRota={vi.fn()} initialChain="SRE_SAAS" visibleChains={["SRE_SAAS", "SRE_IAAS"]} />);
    expect(screen.getAllByRole("tab").map((tab) => tab.textContent)).toEqual(["SaaS SRE", "IaaS SRE"]);
    expect(screen.getByRole("tab", { name: "SaaS SRE" })).toHaveAttribute("aria-selected", "true");
    expect(screen.queryByRole("tab", { name: "CRE" })).not.toBeInTheDocument();
  });

  it("shows a reader with one chain just that chain, with no picker", () => {
    render(<CasePagingTab onOpenRota={vi.fn()} initialChain="SRE_IAAS" visibleChains={["SRE_IAAS"]} />);
    expect(screen.queryByRole("tab")).not.toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "IaaS SRE alert chain" })).toBeInTheDocument();
  });

  it("keeps a CRE reader to CRE's chain, whatever chain they would open on", () => {
    render(<CasePagingTab onOpenRota={vi.fn()} initialChain="SME" visibleChains={["CRE"]} />);
    expect(screen.queryByRole("tab")).not.toBeInTheDocument();
    expect(screen.queryByText(/SME alert chain/)).not.toBeInTheDocument();
    expect(screen.getByRole("table", { name: "CRE alert chain by team and level" })).toBeInTheDocument();
  });

  it("tells a PaaS lead there is no chain yet, rather than showing another team's", () => {
    render(<CasePagingTab onOpenRota={vi.fn()} initialChain="SRE_SAAS" visibleChains={["PAAS"]} />);
    expect(screen.getByText("PaaS SRE has no rota yet, so there is no alert chain to show.")).toBeInTheDocument();
    expect(screen.queryByText(/SaaS SRE alert chain/)).not.toBeInTheDocument();
  });

  it("goes back to the editable chain from a read-only one", () => {
    render(<CasePagingTab onOpenRota={vi.fn()} />);
    fireEvent.click(screen.getByRole("tab", { name: "IaaS SRE" }));
    expect(
      screen.getByText(/People come from the IaaS SRE rota/),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("tab", { name: "CRE" }));
    expect(
      screen.getByRole("button", { name: /Edit alert chain/ }),
    ).toBeInTheDocument();
  });

  it("shows a phone chip by each person and runs the paging-number actions", () => {
    const base = pagingChain();
    chain.data = {
      ...base,
      members: base.members.map((m) =>
        m.membershipId === "a2"
          ? {
              ...m,
              canEditPhone: true,
              hasProfilePhone: false,
              pagingPhone: {
                masked: "+94•••••123",
                phone: "+94771234123",
                setBy: "x",
                setAt: "2026-10-01T09:00:00Z",
              },
            }
          : m.membershipId === "a1"
            ? { ...m, hasProfilePhone: true }
            : m,
      ),
    };
    render(<CasePagingTab onOpenRota={vi.fn()} />);
    // Jane Doe (lead) has a profile number, which shows nothing; John Roe
    // (1st responder) a paging one.
    expect(screen.queryByText(/Profile ✓/)).not.toBeInTheDocument();
    expect(screen.getByText(/Alert number · not tested/)).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Test call John Roe's alert number" }),
    );
    expect(testMutate).toHaveBeenCalledWith("u-a2", expect.anything());

    // A number is changed on the profile, not here.
    expect(
      screen.queryByRole("button", { name: "Change John Roe's alert number" }),
    ).not.toBeInTheDocument();
    expect(putMutate).not.toHaveBeenCalled();

    fireEvent.click(
      screen.getByRole("button", { name: "Remove John Roe's alert number" }),
    );
    fireEvent.click(
      within(screen.getByRole("dialog")).getByRole("button", {
        name: "Remove",
      }),
    );
    expect(delMutate).toHaveBeenCalledWith("u-a2", expect.anything());
  });

  it("points a missing number to the profile, even for a person the reader may edit", () => {
    const base = pagingChain();
    chain.data = {
      ...base,
      members: [
        ...base.members,
        pagingMember({
          membershipId: "a4",
          name: "Sam Lee",
          canEditPhone: true,
          hasProfilePhone: false,
        }),
      ],
    };
    readiness.data!.chains[0] = readinessChain({
      chain: "CRE",
      label: "CRE",
      ready: false,
      gaps: [
        readinessGap({
          code: "NO_PHONE",
          fix: "profile",
          userId: "u-a4",
          message: "Sam Lee (L1) has no number",
        }),
        readinessGap({
          code: "NO_PHONE",
          fix: "profile",
          userId: "u-a3",
          message: "Ann Poe (L1) has no number",
        }),
      ],
    });
    render(<CasePagingTab onOpenRota={vi.fn()} />);
    const strip = screen.getByRole("list", { name: "CRE gaps" });
    const [sam, ann] = within(strip).getAllByRole("listitem");
    expect(
      within(sam).queryByRole("button", { name: "Add number" }),
    ).not.toBeInTheDocument();
    expect(sam).toHaveTextContent(ADD_TO_PROFILE_HINT);
    expect(ann).toHaveTextContent(ADD_TO_PROFILE_HINT);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("shows no switch to somebody without settings rights", () => {
    alertSettings.data = settingsFor(false);
    render(<CasePagingTab onOpenRota={vi.fn()} />);
    expect(
      screen.queryByRole("group", { name: "Show" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Settings" }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("table", { name: "CRE alert chain by team and level" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("heading", { name: "CRE settings" }),
    ).not.toBeInTheDocument();
  });

  it("switches between who gets alerts and the settings, and remembers the choice", () => {
    alertSettings.data = settingsFor(true);
    const { unmount } = render(<CasePagingTab onOpenRota={vi.fn()} />);
    const chainBtn = screen.getByRole("button", { name: "Who gets alerts" });
    const settingsBtn = screen.getByRole("button", { name: "Settings" });
    expect(chainBtn).toHaveAttribute("aria-pressed", "true");
    expect(settingsBtn).toHaveAttribute("aria-pressed", "false");
    expect(
      screen.getByRole("table", { name: "CRE alert chain by team and level" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("heading", { name: "CRE settings" }),
    ).not.toBeInTheDocument();

    fireEvent.click(settingsBtn);
    expect(screen.getByRole("button", { name: "Settings" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    expect(
      screen.getByRole("heading", { name: "CRE settings" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("table", {
        name: "CRE alert chain by team and level",
      }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByText(/ready for the next 7 days/),
    ).not.toBeInTheDocument();
    expect(window.localStorage.getItem(VIEW_KEY)).toBe("settings");

    // The SRE chains keep the choice too.
    fireEvent.click(screen.getByRole("tab", { name: "SaaS SRE" }));
    expect(
      screen.getByRole("heading", { name: "SaaS SRE settings" }),
    ).toBeInTheDocument();

    unmount();
    render(<CasePagingTab onOpenRota={vi.fn()} />);
    expect(screen.getByRole("button", { name: "Settings" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    expect(
      screen.getByRole("heading", { name: "CRE settings" }),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Who gets alerts" }));
    expect(
      screen.getByRole("table", { name: "CRE alert chain by team and level" }),
    ).toBeInTheDocument();
    expect(window.localStorage.getItem(VIEW_KEY)).toBe("chain");
  });

  it("shows no switch on SME, which has no settings here", () => {
    alertSettings.data = settingsFor(true);
    window.localStorage.setItem(VIEW_KEY, "settings");
    render(<CasePagingTab onOpenRota={vi.fn()} initialChain="SME" />);
    // An admin opens on CRE; their own pick of SME has no switch.
    fireEvent.click(screen.getByRole("tab", { name: "SME" }));
    expect(
      screen.queryByRole("group", { name: "Show" }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Open the SME rota" }),
    ).toBeInTheDocument();
  });

  it("still switches when the browser's storage throws", () => {
    alertSettings.data = settingsFor(true);
    vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
      throw new Error("blocked");
    });
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
      throw new Error("blocked");
    });
    render(<CasePagingTab onOpenRota={vi.fn()} />);
    expect(
      screen.getByRole("button", { name: "Who gets alerts" }),
    ).toHaveAttribute("aria-pressed", "true");
    fireEvent.click(screen.getByRole("button", { name: "Settings" }));
    expect(
      screen.getByRole("heading", { name: "CRE settings" }),
    ).toBeInTheDocument();
  });
});
