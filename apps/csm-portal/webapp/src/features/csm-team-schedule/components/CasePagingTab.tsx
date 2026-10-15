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

import { useState, type JSX } from "react";
import { useCaseAlertSettings } from "../api/useCaseAlertSettings";
import {
  useGetPagingChain,
  useGetPagingReadiness,
} from "../api/usePagingChain";
import type {
  CaseAlertSettings as AlertSettingsData,
  PagingChainCode,
  PagingChainMember,
  PagingReadinessGap,
  RotaFamily,
  ScheduleTeam,
} from "../types";
import CaseAlertSettings from "./CaseAlertSettings";
import { mayManageChain } from "../utils/caseAlertChainSettings";
import type { CaseAlertChain } from "../utils/pagingChains";
import PagingReadinessStrip, {
  type PhoneGapAction,
} from "./PagingReadinessStrip";
import { CALLING_LABEL } from "./PhoneChip";
import { usePagingPhoneControls } from "./usePagingPhoneControls";

/** The readiness gaps that are about one person's number. */
const PHONE_GAPS = new Set(["NO_PHONE", "PHONE_UNTESTED", "PHONE_TEST_FAILED"]);

/** What a chain shows: who gets its alerts, or its settings. */
type ChainView = "chain" | "settings";

/** Where the last choice is kept, for this browser. */
const VIEW_KEY = "csm-ts.alert-settings.view";

/** The last choice, else who gets alerts. Storage can be blocked or throw
 *  (private windows, strict settings): then it is just the default. */
function readView(): ChainView {
  try {
    return window.localStorage.getItem(VIEW_KEY) === "settings"
      ? "settings"
      : "chain";
  } catch {
    return "chain";
  }
}

function writeView(view: ChainView): void {
  try {
    window.localStorage.setItem(VIEW_KEY, view);
  } catch {
    // Not kept: the choice still holds until the page is left.
  }
}

/** How far ahead the readiness strip looks. */
export const PAGING_READINESS_DAYS = 7;

/** Where a chain's people come from on the rota: the family, and the rota
 *  within it where the chain is one rota's (SME draws on every SME rota). */
export interface PagingRotaTarget {
  family: RotaFamily;
  rotaCode?: string;
  teamKey?: string;
  zoneCode?: string;
  /** YYYY-MM-DD. */
  date?: string;
}

interface ChainOption {
  code: CaseAlertChain;
  label: string;
  /** Where its people are set, for a chain that takes them from the rota. */
  rota?: { family: RotaFamily; rotaCode?: string };
  disabledReason?: string;
}

/** The chains, in the order the picker offers them: CRE, the three SRE
 *  sub-teams together, then SME. PaaS SRE has no rota yet, so it is shown --
 *  it is a chain people expect -- but cannot be picked. CRE is offered only to
 *  those who may see its settings, and shows them alone: who CRE pages is set
 *  on the rota, not here. */
const CHAINS: readonly ChainOption[] = [
  { code: "CRE", label: "CRE" },
  {
    code: "SRE_SAAS",
    label: "SaaS SRE",
    rota: { family: "SRE", rotaCode: "SRE_SAAS" },
  },
  {
    code: "SRE_IAAS",
    label: "IaaS SRE",
    rota: { family: "SRE", rotaCode: "SRE_IAAS" },
  },
  { code: "PAAS", label: "PaaS SRE", disabledReason: "No PaaS rota yet" },
  { code: "SME", label: "SME", rota: { family: "SME" } },
];

interface Props {
  /** Switch the page to the week view on a rota (and team, zone, day). */
  onOpenRota: (target: PagingRotaTarget) => void;
  /** The chain the tab opens on: the reader's own (SME for an SME rota
   *  admin, their SRE sub-team's for SRE). SaaS SRE when absent. */
  initialChain?: PagingChainCode;
  /** The chains this reader is shown -- their own (caseAlertChainsFor).
   *  Every chain when absent, as for a CSM admin or the CRE and CS heads. */
  visibleChains?: readonly CaseAlertChain[];
  /** The Team Schedule's teams, for the SME chain's chat spaces. */
  teams?: readonly Pick<ScheduleTeam, "key" | "name" | "family">[];
}

/** Whether the reader may change both ladders' settings: what a CSM admin
 *  or portal admin gets, a role not tied to any one team. */
function managesBothLadders(settings?: AlertSettingsData): boolean {
  const ladders = settings?.canManage.roomLadders ?? [];
  return ladders.includes("CRE") && ladders.includes("SRE");
}

/**
 * The Case Paging tab: a chain picker, then the chain on screen. The SRE and
 * SME chains page whoever the rota has on, so the tab says where to set people
 * and how ready the chain is. Each chain opens with its readiness strip.
 *
 * For the people who may see a chain's alert settings, a switch above it
 * flips between who gets the alerts and the settings (CaseAlertSettings); the
 * last choice is kept for this browser. Everyone else, and the SME chain,
 * which has no settings here, sees the chain alone.
 */
export default function CasePagingTab({
  onOpenRota,
  initialChain = "SRE_SAAS",
  visibleChains,
  teams,
}: Props): JSX.Element {
  const alertSettings = useCaseAlertSettings();
  const roomLadders = alertSettings.data?.canManage.roomLadders;
  const manages = (code: CaseAlertChain) =>
    code !== "PAAS" && mayManageChain(code, roomLadders);
  // The reader's own chains (caseAlertChainsFor), and every chain whose
  // settings they may see; CRE only for those.
  const chains = CHAINS.filter((c) =>
    c.code === "CRE"
      ? manages("CRE")
      : !visibleChains || visibleChains.includes(c.code) || manages(c.code),
  );
  const offered = (
    code: PagingChainCode | undefined,
  ): code is PagingChainCode =>
    !!code && chains.some((c) => c.code === code && !c.disabledReason);
  // The reader's pick, else the chain they belong to, else the first one
  // offered. Derived rather than seeded into state, so a profile that settles
  // after the first frame still opens the right chain.
  const [chainChoice, setChainCode] = useState<PagingChainCode | undefined>(
    undefined,
  );
  const firstOffered = chains.find((c) => !c.disabledReason)?.code as
    PagingChainCode | undefined;
  // A CSM or portal admin, who may change both ladders, belongs to no one
  // chain: they open on the first, whatever their own team's family is.
  const ownChain = managesBothLadders(alertSettings.data)
    ? firstOffered
    : initialChain;
  const chainCode: PagingChainCode = offered(chainChoice)
    ? chainChoice
    : offered(ownChain)
      ? ownChain
      : (firstOffered ?? initialChain);
  const readiness = useGetPagingReadiness(PAGING_READINESS_DAYS);
  const hasSettings = manages(chainCode);
  const [view, setView] = useState<ChainView>(readView);
  const chooseView = (next: ChainView) => {
    setView(next);
    writeView(next);
  };
  // The people a phone gap can be acted on for: SRE's while an SRE chain is
  // on screen. SME's people are not served as a chain, so its phone gaps keep
  // the profile hint.
  const sreOnScreen = chainCode === "SRE_SAAS" || chainCode === "SRE_IAAS";
  const sreChain = useGetPagingChain("SRE", sreOnScreen);
  const members: PagingChainMember[] = sreOnScreen
    ? (sreChain.data?.members ?? [])
    : [];
  const readAt = sreOnScreen ? sreChain.dataUpdatedAt : 0;
  const { controls: phone, dialog } = usePagingPhoneControls(members, readAt);

  /** "Test call" on a phone gap for a paging-only number the reader may act
   *  on. A missing number gets no action here: it is added on the person's
   *  profile, which the gap's hint says. */
  const phoneActionFor = (gap: PagingReadinessGap): PhoneGapAction | null => {
    if (!PHONE_GAPS.has(gap.code) || !gap.userId || gap.code === "NO_PHONE")
      return null;
    const m = members.find(
      (x) => x.userId === gap.userId && x.canEditPhone === true,
    );
    if (!m?.pagingPhone) return null;
    const calling = phone.calling(m);
    return {
      label: calling ? CALLING_LABEL : "Test call",
      onClick: () => phone.test(m),
      pending: calling,
    };
  };

  const option =
    chains.find((c) => c.code === chainCode) ?? chains[0] ?? CHAINS[0];
  const chainReadiness = readiness.data?.chains.find(
    (c) => c.chain === chainCode,
  );
  const stripRead = {
    chain: chainReadiness,
    days: PAGING_READINESS_DAYS,
    isLoading: readiness.isLoading,
    isError: readiness.isError,
  };

  /** A rota gap opens the rota the gap is on: its team's, its zone's, or the
   *  chain's own. */
  const openGap = (gap: PagingReadinessGap) =>
    onOpenRota({
      family: option.rota?.family ?? "SRE",
      rotaCode: option.rota?.rotaCode,
      teamKey: gap.teamKey,
      zoneCode: gap.zoneCode,
      date: gap.date,
    });

  // A reader with one chain has nothing to pick between.
  const picker = chains.length < 2 ? null : (
    <div
      className="seg teamseg cp-chains"
      role="tablist"
      aria-label="Alert chain"
    >
      {chains.map((c) => {
        const disabled = Boolean(c.disabledReason);
        return (
          <button
            key={c.code}
            type="button"
            role="tab"
            aria-selected={chainCode === c.code}
            // aria-disabled rather than disabled: a disabled button shows no
            // tooltip and is skipped by the keyboard, and the reason it cannot
            // be picked is the point.
            aria-disabled={disabled || undefined}
            title={c.disabledReason}
            className={
              `${chainCode === c.code ? "on" : ""}${disabled ? " off" : ""}`.trim() ||
              undefined
            }
            onClick={() => {
              if (!disabled) setChainCode(c.code as PagingChainCode);
            }}
          >
            {c.label}
          </button>
        );
      })}
    </div>
  );

  // Nothing to show: their only chain is one with no rota yet (PaaS SRE).
  if (!offered(chainCode)) {
    const only = chains[0];
    return (
      <div className="offnone">
        {only ? `${only.label} has no rota yet, so there is no alert chain to show.` : "There is no alert chain of yours to show."}
      </div>
    );
  }

  const viewSwitch = hasSettings && chainCode !== "CRE" ? (
    <div className="seg cas-view" role="group" aria-label="Show">
      {(
        [
          ["chain", "Who gets alerts"],
          ["settings", "Settings"],
        ] as const
      ).map(([v, label]) => (
        <button
          key={v}
          type="button"
          aria-pressed={view === v}
          className={view === v ? "on" : undefined}
          onClick={() => chooseView(v)}
        >
          {label}
        </button>
      ))}
    </div>
  ) : null;

  // CRE has its settings alone; the other chains switch between who gets
  // alerts and the settings.
  if (hasSettings && (view === "settings" || chainCode === "CRE")) {
    return (
      <>
        <div className="card-head">
          {picker}
          {viewSwitch}
        </div>
        {/* Keyed by chain: a "Saved" line belongs to the chain it was saved on. */}
        <CaseAlertSettings key={chainCode} chain={chainCode} teams={teams} />
        {dialog}
      </>
    );
  }

  const rota = option.rota;
  return (
    <>
      <div className="card-head">
        {picker}
        {viewSwitch}
        <h2>{option.label} alert chain</h2>
      </div>
      <PagingReadinessStrip
        key={chainCode}
        label={option.label}
        {...stripRead}
        onOpenRota={openGap}
        phoneActionFor={phoneActionFor}
      />
      <p className="cp-from">
        People come from the {option.label} rota; set them in the Team Schedule
        views.{" "}
        {rota ? (
          <button
            type="button"
            className="btn sm"
            onClick={() =>
              onOpenRota({ family: rota.family, rotaCode: rota.rotaCode })
            }
          >
            Open the {option.label} rota
          </button>
        ) : null}
      </p>
      {dialog}
    </>
  );
}
