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

import { Box, Button, Menu, MenuItem, Tooltip, Typography } from "@wso2/oxygen-ui";
import {
  ArrowRight,
  Ban,
  CalendarClock,
  CheckCircle,
  ChevronDown,
  Play,
  Send,
  SkipForward,
  Undo2,
  UserCheck,
} from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX } from "react";
import {
  changeRequestTransitionLabel,
  customerBypassTarget,
  customerRequestPendingReason,
  isCustomerBypassTransition,
  isDestructiveChangeRequestTransition,
  type PendingCustomerRequest,
} from "@features/csm-operations/utils/changeRequests";
import type { BeChangeRequestDetail } from "@api/backend/types";

/**
 * Icon and colour for a transition *into* a given state. The button LABEL is
 * never stored here — it always comes from `changeRequestTransitionLabel`,
 * same "no invented verbs" convention as `IncidentActionBar`'s `TargetConfig`.
 */
type TargetConfig = {
  color: "primary" | "success" | "warning" | "error";
  icon: JSX.Element;
};

const TARGET_CONFIG: Record<string, TargetConfig> = {
  assess: { color: "primary", icon: <Send size={16} /> },
  implement: { color: "primary", icon: <Play size={16} /> },
  review: { color: "primary", icon: <CheckCircle size={16} /> },
  customer_review: { color: "primary", icon: <UserCheck size={16} /> },
  // Plain Close, out of Review when no customer review is required. Leaving
  // `customer_review` the same target is a customer bypass: `BYPASS_CONFIG`.
  closed: { color: "primary", icon: <CheckCircle size={16} /> },
  // Only ever rendered from `customer_approval` ("Re-schedule").
  authorize: { color: "primary", icon: <CalendarClock size={16} /> },
  rollback: { color: "error", icon: <Undo2 size={16} /> },
  canceled: { color: "error", icon: <Ban size={16} /> },
};

/**
 * Presentation of a customer bypass (`isCustomerBypassTransition`): an
 * engineer answering for the customer. Warning, not error -- it is not
 * destructive, it skips a gate -- with a skip icon so it never reads as an
 * ordinary forward step.
 */
const BYPASS_CONFIG: TargetConfig = {
  color: "warning",
  icon: <SkipForward size={16} />,
};

/**
 * Presentation for a transition this bar has no curated config for — a state
 * the backend added, or one of the lifecycle states with no agreed action
 * verb yet. Same fallback convention as `IncidentActionBar`'s
 * `DEFAULT_TARGET_CONFIG`, so a new backend state stays renderable and
 * clickable with no frontend change.
 */
const DEFAULT_TARGET_CONFIG: TargetConfig = {
  color: "primary",
  icon: <ArrowRight size={16} />,
};

/**
 * Forward progression through the lifecycle. Used for one thing only: picking
 * which single target gets the primary button when several are legal at once.
 * This array owns ordering, never legality.
 *
 * Membership doubles as primary-button eligibility, so it deliberately
 * excludes the destructive off-ramps and every uncurated state: without a
 * curated action label there is no evidence a target is *the* expected
 * forward move, so it goes in the overflow menu instead. It also excludes
 * `scheduled`, which is only ever offered as the customer bypass out of
 * `customer_approval`. `closed` is a member (Close out of Review when no
 * customer review is required), but a customer bypass is never primary
 * whatever its target: see `isPrimaryEligible`.
 */
const FORWARD_ORDER: readonly string[] = [
  "assess",
  "implement",
  "review",
  "customer_review",
  "closed",
];

/**
 * Whether `target` may take the primary button from `state`: a forward move,
 * and never a customer bypass. `closed` is the case that needs the state:
 * the same target is the primary Close out of Review and a menu-only bypass
 * out of `customer_review`.
 */
function isPrimaryEligible(target: string, state: string | null | undefined): boolean {
  return FORWARD_ORDER.includes(target) && !isCustomerBypassTransition(target, state);
}

/**
 * Actions shown as an outlined (secondary) button beside the primary one
 * rather than inside the overflow menu: `authorize` is "Re-schedule", the
 * non-destructive loop back from `customer_approval` (see `isOfferedTarget`).
 */
const SECONDARY_ORDER: readonly string[] = ["authorize"];

/**
 * Menu ordering: forward moves first, then the customer bypass, then the
 * destructive off-ramps. Uncurated states sort after all of them.
 */
const MENU_ORDER_BEFORE_BYPASS: readonly string[] = [...FORWARD_ORDER, ...SECONDARY_ORDER];
const MENU_ORDER_AFTER_BYPASS: readonly string[] = ["rollback", "canceled"];

/**
 * States this bar never offers, no matter what `legalNextStates` contains.
 *
 * Do not delete this filter because "the list doesn't include them anyway".
 * `customer_approval`: not human-enterable in the backing system — of its 38
 * UI actions on the change-request table, none sets it. It is reached only by
 * the approval process itself. Setting it by hand from here would leave a
 * record sitting in an approval state with no approver record behind it,
 * which is an audit hole rather than a shortcut.
 *
 * `authorize` is a different case: it IS reachable by a human action, just
 * never this one. It's the automatic side effect of an approver approving in
 * the Approvers section (`ChangeRequestApprovals.tsx` / the decide-approval
 * endpoint), which already correctly cascades the change request's own state
 * forward on approval — not "automation-only" the way the other two are, but
 * gated by a real human decision made somewhere else in the UI, not here.
 * Offering it as a directly-clickable button/menu item from here would let
 * someone skip the actual approval process entirely and land the record in
 * Authorize with no approval behind it — the same audit hole as above, by a
 * different route. The one exception is keyed on the record's own state: from
 * `customer_approval` it means "Re-schedule" (the planned time changed, so the
 * change goes back through internal approval -- more approval, not less), and
 * the page collects the new planned window before sending it.
 *
 * `scheduled` is the same shape as `authorize`: a CR is moved to Scheduled
 * automatically the moment its approval is granted (CAB/ECAB, or Standard's
 * Request Approval) -- or, when the CR requires customer approval, it first
 * waits in `customer_approval`. There is no manual "Schedule" action anywhere,
 * with exactly one exception: leaving `customer_approval`, where a manual
 * `PATCH {state:"scheduled"}` is the customer bypass ("Bypass customer
 * approval": the engineer records the customer's approval for them). So
 * `scheduled` is filtered out unless the CR's current state is
 * `customer_approval` -- see `isOfferedTarget` -- and there it is menu-only.
 *
 * `rollback` is the failed-review off-ramp of the process diagram: a human
 * action ("Roll back"), but only from the two review states, `review` and
 * `customer_review` (the backend offers and accepts it from nowhere else).
 * It is a destructive, menu-only item that requires a stated reason. It used
 * to be excluded here as "automation-only"; the backend now owns the manual
 * transition. The carve-out is still keyed on the record's own state.
 *
 * The exclusions are deliberately unconditional (the `scheduled` and
 * `rollback` carve-outs are keyed on the record's own state, never on what
 * `legalNextStates` claims) so a future backend change that starts returning
 * any of these cannot silently reopen them.
 */
const NEVER_OFFERED_TARGETS: readonly string[] = ["customer_approval"];

/** States a change request can be manually rolled back from. */
const ROLLBACK_FROM_STATES: readonly string[] = ["review", "customer_review"];

/**
 * `scheduled` (the "Bypass customer approval" move) and `authorize`
 * ("Re-schedule") are manual actions only from `customer_approval`;
 * `rollback` only from the two review states. Everywhere else they are not
 * offered.
 */
function isOfferedTarget(target: string, currentState: string | null | undefined): boolean {
  if (!target || target === currentState) return false;
  if (target === "scheduled" || target === "authorize") {
    return currentState === "customer_approval";
  }
  if (target === "rollback") {
    return !!currentState && ROLLBACK_FROM_STATES.includes(currentState);
  }
  return !NEVER_OFFERED_TARGETS.includes(target);
}

/**
 * Sort key for a target in the "Change state" menu: forward moves first, a
 * customer bypass next, Roll back / Cancel change after it (destructive last),
 * uncurated states at the very end.
 */
function menuRank(target: string, state: string | null | undefined): number {
  if (isCustomerBypassTransition(target, state)) return MENU_ORDER_BEFORE_BYPASS.length;
  const before = MENU_ORDER_BEFORE_BYPASS.indexOf(target);
  if (before !== -1) return before;
  const after = MENU_ORDER_AFTER_BYPASS.indexOf(target);
  const afterBase = MENU_ORDER_BEFORE_BYPASS.length + 1;
  return after !== -1 ? afterBase + after : afterBase + MENU_ORDER_AFTER_BYPASS.length;
}

/** What the per-target blockers may consult besides the record itself. */
interface BlockedReasonContext {
  /** The customer's answer the change is waiting for, when the caller knows of one. */
  pendingCustomerRequest: PendingCustomerRequest | null;
}

/**
 * Prerequisites the state machine itself doesn't express. A target can be
 * legal per `legalNextStates` and still be blocked by a missing field the
 * backing system checks on write — offering it anyway just round-trips into a
 * rejection, so it renders disabled with the reason instead.
 *
 * Deliberately a per-target map rather than a special case for any one
 * target: the same situation (legal transition, unmet prerequisite) can
 * apply to any target.
 *
 * `assess` ("Request Approval") requires `assignedTeam` — by explicit product decision, confirmed
 * compulsory: the assigned team's own members are what populate the Assess
 * stage's approvers the moment the transition lands (see
 * `PatchChangeRequest`'s own doc comment in `change_request_repo.go`), so
 * there is no such thing as entering Assess with no team to assign that
 * stage to. This was previously removed as a stale leftover from when
 * New → Assess sent a ServiceNow "Request Approval" action — that removal
 * was wrong: the requirement is real under the current plain
 * `{ state: "assess" }` PATCH too, just enforced for a different reason now
 * (who gets provisioned as an approver), and the backend itself rejects the
 * transition with no team regardless of what this map does — this entry is
 * what keeps the button from round-tripping into that rejection.
 *
 * `scheduled` / `closed` are blocked only as the two customer bypasses (the
 * entry checks the record's own state: a plain Close out of Review is never
 * blocked by a customer request): while the customer group's request is
 * pending the backend refuses to let anyone answer for the customer, since
 * they answer in the customer portal.
 */
const TARGET_BLOCKED_REASON: Record<
  string,
  (cr: BeChangeRequestDetail, context: BlockedReasonContext) => string | null
> = {
  assess: (cr) =>
    cr.assignedTeam ? null : "Set an assigned team before requesting approval",
  scheduled: (cr, { pendingCustomerRequest }) =>
    isCustomerBypassTransition("scheduled", cr.state)
      ? customerRequestPendingReason(pendingCustomerRequest)
      : null,
  closed: (cr, { pendingCustomerRequest }) =>
    isCustomerBypassTransition("closed", cr.state)
      ? customerRequestPendingReason(pendingCustomerRequest)
      : null,
};

interface ChangeRequestActionBarProps {
  cr: BeChangeRequestDetail;
  /** True while a state-changing request for this CR is in flight. */
  isPending: boolean;
  /**
   * The customer's answer the change is still waiting for (a live Customer
   * Approval / Customer Review request), derived by the caller from the
   * change's approval stages (`pendingCustomerRequest`). While one is pending
   * the customer bypass is shown disabled with the reason, even though the
   * backend has already left it out of `legalNextStates` (it refuses it). Leave
   * it `null`/absent while the approvals are loading or when nobody is being
   * asked: the bypass is then enabled exactly when `legalNextStates` offers it.
   */
  pendingCustomerRequest?: PendingCustomerRequest | null;
  /**
   * Fired with the target state the engineer picked. The caller decides how
   * to apply it — a direct patch for most targets, or (for the ones flagged by
   * `changeRequestTransitionRequiresReason`: the destructive off-ramps and
   * the customer bypasses) opening a dialog to collect the reason first. Same
   * split of responsibility as `IncidentActionBar` + `CsmIncidentDetailPage`.
   */
  onAction: (target: string) => void;
}

/**
 * Lifecycle action bar for the change-request detail page. Every button comes
 * straight from the record's own `legalNextStates` — there is no client-side
 * state machine here and no hardcoded state list, so a transition the backend
 * starts offering appears with no frontend change. Renders nothing when
 * `legalNextStates` is empty or absent (a terminal state, or a record the
 * caller may not transition).
 *
 * Exactly one target — the first forward move present, by `FORWARD_ORDER` —
 * gets a primary button; "Re-schedule" (only from `customer_approval`) is an
 * outlined button beside it; everything else sits behind a "Change state"
 * overflow menu. The header this sits in already carries Back, Clone and
 * Edit, so a row of eight buttons would bury the one action the engineer
 * actually wants.
 *
 * The two customer bypasses ("Bypass customer approval" out of
 * `customer_approval`, "Bypass customer review" out of `customer_review`) are
 * never the primary button and never an outlined button: an engineer
 * answering for the customer is a deliberate override, so it lives only in the
 * menu, in the warning colour, between the forward moves and Roll back /
 * Cancel change.
 */
export default function ChangeRequestActionBar({
  cr,
  isPending,
  pendingCustomerRequest = null,
  onAction,
}: ChangeRequestActionBarProps): JSX.Element | null {
  const [stateMenuAnchor, setStateMenuAnchor] = useState<HTMLElement | null>(null);

  // Single choke point for what is renderable: the exclusion below therefore
  // covers the primary button, the overflow menu, and states rendered through
  // `DEFAULT_TARGET_CONFIG` alike.
  const offered = Array.from(
    new Set(
      (cr.legalNextStates ?? []).filter((s) => isOfferedTarget(s, cr.state)),
    ),
  );
  if (offered.length === 0) return null;

  // While a customer request is pending the backend leaves the bypass out of
  // `legalNextStates` (it would refuse it), so there is nothing to render a
  // disabled entry from: add it here, so the engineer sees why it is not
  // available instead of wondering where it went. It is only ever added
  // alongside targets the backend did offer (so a record the caller may not
  // transition still renders nothing), is menu-only, and `TARGET_BLOCKED_REASON`
  // keeps it disabled -- this never makes anything clickable.
  const bypassTarget = customerBypassTarget(cr.state);
  const targets = (
    pendingCustomerRequest && bypassTarget && !offered.includes(bypassTarget)
      ? [...offered, bypassTarget]
      : offered
  ).sort((a, b) => menuRank(a, cr.state) - menuRank(b, cr.state));

  const primaryTarget = targets.find((t) => isPrimaryEligible(t, cr.state));
  const secondaryTargets = targets.filter((t) => SECONDARY_ORDER.includes(t));
  const menuTargets = targets.filter((t) => t !== primaryTarget && !SECONDARY_ORDER.includes(t));

  const dispatch = (target: string): void => {
    setStateMenuAnchor(null);
    onAction(target);
  };

  const configFor = (target: string): TargetConfig =>
    isCustomerBypassTransition(target, cr.state)
      ? BYPASS_CONFIG
      : (TARGET_CONFIG[target] ?? DEFAULT_TARGET_CONFIG);

  const blockedReason = (target: string): string | null =>
    TARGET_BLOCKED_REASON[target]?.(cr, { pendingCustomerRequest }) ?? null;

  const renderPrimary = (target: string): JSX.Element => {
    const { color, icon } = configFor(target);
    const label = changeRequestTransitionLabel(target, cr.state);
    const reason = blockedReason(target);
    if (reason) {
      return (
        <Tooltip title={reason}>
          {/* A disabled button is not focusable, so the tooltip alone would be
              unreachable by keyboard — this focusable, labelled wrapper is
              what exposes the reason to assistive tech. */}
          <Box
            component="span"
            tabIndex={0}
            aria-label={`${label}: ${reason}`}
            sx={{ flexShrink: 0 }}
          >
            <Button
              size="small"
              variant="contained"
              color={color}
              startIcon={icon}
              disabled
              sx={{ flexShrink: 0 }}
            >
              {label}
            </Button>
          </Box>
        </Tooltip>
      );
    }
    return (
      <Button
        size="small"
        variant="contained"
        color={color}
        startIcon={icon}
        loading={isPending}
        onClick={() => dispatch(target)}
        sx={{ flexShrink: 0 }}
      >
        {label}
      </Button>
    );
  };

  return (
    <Box sx={{ display: "flex", gap: 1, flexShrink: 0 }}>
      {primaryTarget && renderPrimary(primaryTarget)}
      {secondaryTargets.map((target) => {
        const { color, icon } = configFor(target);
        return (
          <Button
            key={target}
            size="small"
            variant="outlined"
            color={color}
            startIcon={icon}
            disabled={isPending}
            onClick={() => dispatch(target)}
            sx={{ flexShrink: 0 }}
          >
            {changeRequestTransitionLabel(target, cr.state)}
          </Button>
        );
      })}
      {menuTargets.length > 0 && (
        <>
          <Button
            size="small"
            variant={primaryTarget ? "outlined" : "contained"}
            color="primary"
            endIcon={<ChevronDown size={16} />}
            disabled={isPending}
            aria-haspopup="menu"
            onClick={(e) => setStateMenuAnchor(e.currentTarget)}
          >
            Change state
          </Button>
          <Menu
            anchorEl={stateMenuAnchor}
            open={!!stateMenuAnchor}
            onClose={() => setStateMenuAnchor(null)}
            // `disabledItemsFocusable` keeps a blocked entry reachable by
            // keyboard so its reason can actually be read, instead of the item
            // being skipped over silently.
            MenuListProps={{
              "aria-label": "Change state",
              disabledItemsFocusable: true,
            }}
          >
            {menuTargets.map((target) => {
              const { color, icon } = configFor(target);
              const label = changeRequestTransitionLabel(target, cr.state);
              const reason = blockedReason(target);
              const destructive = isDestructiveChangeRequestTransition(target);
              const bypass = isCustomerBypassTransition(target, cr.state);
              const disabled = isPending || !!reason;
              return (
                <MenuItem
                  key={target}
                  disabled={disabled}
                  aria-label={reason ? `${label}: ${reason}` : label}
                  onClick={() => {
                    if (reason) return;
                    dispatch(target);
                  }}
                  sx={{
                    gap: 1.25,
                    minHeight: 36,
                    alignItems: "flex-start",
                    py: 1,
                    // A blocked item dims its own icon and label (below), not
                    // the whole row: the reason beside it has to stay legible.
                    "&.Mui-disabled": { opacity: 1 },
                  }}
                >
                  <Box
                    sx={{
                      color: `${color}.main`,
                      display: "flex",
                      mt: 0.25,
                      opacity: disabled ? 0.5 : 1,
                    }}
                  >
                    {icon}
                  </Box>
                  <Box sx={{ display: "flex", flexDirection: "column", minWidth: 0, maxWidth: 300 }}>
                    <Box
                      component="span"
                      sx={{
                        color: destructive ? "error.main" : bypass ? "warning.dark" : "inherit",
                        opacity: disabled ? 0.5 : 1,
                      }}
                    >
                      {label}
                    </Box>
                    {reason && (
                      <Typography
                        variant="caption"
                        color="text.secondary"
                        sx={{ whiteSpace: "normal" }}
                      >
                        {reason}
                      </Typography>
                    )}
                  </Box>
                </MenuItem>
              );
            })}
          </Menu>
        </>
      )}
    </Box>
  );
}
