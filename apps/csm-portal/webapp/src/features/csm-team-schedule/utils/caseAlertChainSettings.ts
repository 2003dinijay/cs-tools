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

import type { CaseAlertRoomLadder, PagingChainCode } from "../types";

export interface ChainSettings {
  /** The chain as the picker names it. */
  label: string;
  ladder: CaseAlertRoomLadder;
  /** Only the teams' chat spaces: no redirects, ladder settings or file
   *  values. What the SRE and SME chains show. */
  roomsOnly?: boolean;
}

/** The chains with settings here. Both SRE chains show the one SRE ladder,
 *  so every SRE team's chat space; SME shows each SME team's. Only the chat
 *  spaces: everything else stays in the alert service's file. */
export const CHAIN_SETTINGS: Partial<Record<PagingChainCode, ChainSettings>> = {
  CRE: { label: "CRE", ladder: "CRE" },
  SRE_SAAS: { label: "SaaS SRE", ladder: "SRE", roomsOnly: true },
  SRE_IAAS: { label: "IaaS SRE", ladder: "SRE", roomsOnly: true },
  SME: { label: "SME", ladder: "SME", roomsOnly: true },
};

/** Whether a chain has settings on the tab (its Settings switch). */
export function chainHasSettings(chain: PagingChainCode): boolean {
  return CHAIN_SETTINGS[chain] !== undefined;
}

