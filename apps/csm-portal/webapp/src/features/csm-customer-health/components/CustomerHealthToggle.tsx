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

// `value` is entirely derived from `page`, no local state to sync via an
// effect. Uses the same ToggleButtonGroup/ToggleButton pattern already
// established elsewhere in this app (features/plg/pages/organizations/
// ProductTab.tsx's own health-state toggle) instead of the Tabs-in-a-pill
// hack this previously was -- that version relied on a raw inline `style`
// with a magic `height: "49.5px"` and an invalid `boxShadow: "insert"`
// (a no-op typo for "inset").
import { ToggleButton, ToggleButtonGroup } from "@wso2/oxygen-ui";

export type AccountScopeState = "my-accounts" | "all-accounts";

interface CustomerHealthToggleProps {
  onSwitchClick: (state: AccountScopeState) => void;
  page: AccountScopeState;
}

export default function CustomerHealthToggle({ onSwitchClick, page }: CustomerHealthToggleProps) {
  return (
    <ToggleButtonGroup
      exclusive
      size="small"
      color="primary"
      value={page}
      sx={{ height: 40 }}
      onChange={(_, next: AccountScopeState | null) => {
        // next is null when the pressed button was already selected --
        // ignored so the scope always stays chosen, same guard ProductTab's
        // own ToggleButtonGroup uses.
        if (next) onSwitchClick(next);
      }}
    >
      <ToggleButton value="all-accounts">All Accounts</ToggleButton>
      <ToggleButton value="my-accounts">My Accounts</ToggleButton>
    </ToggleButtonGroup>
  );
}
