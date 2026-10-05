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

import { Box, Button, Menu, MenuItem } from "@wso2/oxygen-ui";
import {
  ArrowRight,
  Ban,
  CheckCircle,
  ChevronDown,
  PauseCircle,
  Play,
} from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX } from "react";
import { getLegalNextIncidentStates, incidentStateLabel } from "@features/csm-operations/utils/incidents";
import type { BeIncidentDetail, BeIncidentState } from "@api/backend/types";

/**
 * Presentation for a transition *into* a given state. The button LABEL is
 * never stored here — it always comes from `incidentStateLabel(target)`, same
 * "no invented verbs" convention as `CaseActionBar`'s `TargetConfig`. Only the
 * icon/colour live here.
 */
type TargetConfig = {
  color: "primary" | "success" | "warning" | "error";
  icon: JSX.Element;
};

const TARGET_CONFIG: Partial<Record<BeIncidentState, TargetConfig>> = {
  IN_PROGRESS: { color: "primary", icon: <Play size={16} /> },
  ON_HOLD: { color: "warning", icon: <PauseCircle size={16} /> },
  RESOLVED: { color: "success", icon: <CheckCircle size={16} /> },
  CLOSED: { color: "success", icon: <CheckCircle size={16} /> },
  CANCELLED: { color: "error", icon: <Ban size={16} /> },
};

/**
 * Presentation for a transition into a state this bar has no curated config
 * for (e.g. a state added on the backend). Keeps the button renderable and
 * safe to click — same fallback convention as `CaseActionBar`'s
 * `DEFAULT_TARGET_CONFIG` — so a new backend state needs no FE change.
 */
const DEFAULT_TARGET_CONFIG: TargetConfig = {
  color: "primary",
  icon: <ArrowRight size={16} />,
};

interface IncidentActionBarProps {
  incident: BeIncidentDetail;
  /** True while a `PATCH /incidents/{id}` state transition is in flight. */
  isPending: boolean;
  /**
   * Fired with the target state the engineer picked. The caller decides how
   * to apply it — a direct `PATCH { state: target }` for most targets, or
   * (for `RESOLVED`/`CLOSED`) opening a dialog to collect the
   * `resolutionCode`/`resolutionNotes` ServiceNow requires for those two
   * transitions (confirmed live — see `BeUpdateIncidentPayload`'s doc
   * comment) — this bar has no opinion on that, same split of
   * responsibility as `CaseActionBar` + `CsmCaseDetailPage.onAction`.
   */
  onAction: (target: BeIncidentState) => void;
}

/**
 * Lifecycle action bar for the incident detail page. Buttons are driven
 * directly by `getLegalNextIncidentStates` (a CSM-platform-only guardrail —
 * ServiceNow itself enforces no old-state -> new-state legality for
 * incidents in this org, only role-gating) so the button set always matches
 * that single source of truth. Renders nothing once the incident is in a
 * terminal state (`CLOSED`/`CANCELLED`).
 */
export default function IncidentActionBar({
  incident,
  isPending,
  onAction,
}: IncidentActionBarProps): JSX.Element | null {
  const [stateMenuAnchor, setStateMenuAnchor] = useState<HTMLElement | null>(null);

  const current = incident.state;
  if (!current) return null;
  const targets = getLegalNextIncidentStates(current).filter((s) => s !== current);
  if (targets.length === 0) return null;

  const buttons = targets.map((target) => ({
    target,
    label: incidentStateLabel(target),
    ...(TARGET_CONFIG[target] ?? DEFAULT_TARGET_CONFIG),
  }));

  const dispatch = (target: BeIncidentState): void => {
    setStateMenuAnchor(null);
    onAction(target);
  };

  if (buttons.length === 1) {
    const b = buttons[0];
    return (
      <Button
        size="small"
        variant="contained"
        color={b.color}
        startIcon={b.icon}
        disabled={isPending}
        onClick={() => dispatch(b.target)}
      >
        {b.label}
      </Button>
    );
  }

  return (
    <Box sx={{ display: "flex" }}>
      <Button
        size="small"
        variant="contained"
        color="primary"
        endIcon={<ChevronDown size={16} />}
        disabled={isPending}
        onClick={(e) => setStateMenuAnchor(e.currentTarget)}
      >
        Change state
      </Button>
      <Menu
        anchorEl={stateMenuAnchor}
        open={!!stateMenuAnchor}
        onClose={() => setStateMenuAnchor(null)}
      >
        {buttons.map((b) => (
          <MenuItem
            key={b.target}
            onClick={() => dispatch(b.target)}
            sx={{ gap: 1.25, minHeight: 36 }}
          >
            <Box sx={{ color: `${b.color}.main`, display: "flex" }}>{b.icon}</Box>
            {b.label}
          </MenuItem>
        ))}
      </Menu>
    </Box>
  );
}
