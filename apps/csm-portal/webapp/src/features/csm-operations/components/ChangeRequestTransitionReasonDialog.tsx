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

import {
  Alert,
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  TextField,
} from "@wso2/oxygen-ui";
import { useState, type JSX } from "react";
import {
  changeRequestTransitionLabel,
  isCustomerBypassTransition,
} from "@features/csm-operations/utils/changeRequests";

interface TransitionCopy {
  title: string;
  body: string;
  confirmLabel: string;
  /** Destructive off-ramps confirm in the error colour; a customer bypass in the warning colour. */
  confirmColor: "error" | "warning";
}

/** Per-target copy for the two destructive transitions. */
const TRANSITION_COPY: Record<string, TransitionCopy> = {
  rollback: {
    title: "Roll back this change?",
    body:
      "This moves the change request into Rollback to record that the review failed and the implemented change is being reversed. Rollback is final and can't be undone from here.",
    confirmLabel: "Roll back",
    confirmColor: "error",
  },
  canceled: {
    title: "Cancel this change request?",
    body:
      "This closes the change request as canceled. It can't be reopened from here, and any approvals already given are lost.",
    confirmLabel: "Cancel change",
    confirmColor: "error",
  },
};

/**
 * Copy for the two customer bypasses, keyed by their target
 * (`isCustomerBypassTransition`): an engineer answers for the customer, who is
 * not asked.
 */
const BYPASS_COPY: Record<string, TransitionCopy> = {
  scheduled: {
    title: "Bypass customer approval",
    body:
      "This records the customer's approval on their behalf and moves the change request to Scheduled. The customer is not asked.",
    confirmLabel: "Bypass customer approval",
    confirmColor: "warning",
  },
  closed: {
    title: "Bypass customer review",
    body:
      "This records the customer's review on their behalf and moves the change request to Closed. The customer is not asked.",
    confirmLabel: "Bypass customer review",
    confirmColor: "warning",
  },
};

function copyFor(target: string, fromState?: string | null): TransitionCopy {
  if (isCustomerBypassTransition(target, fromState)) {
    const bypass = BYPASS_COPY[target];
    if (bypass) return bypass;
  }
  return (
    TRANSITION_COPY[target] ?? {
      title: `${changeRequestTransitionLabel(target, fromState)}?`,
      body: "This change to the record can't be undone from here.",
      confirmLabel: changeRequestTransitionLabel(target, fromState),
      confirmColor: "error",
    }
  );
}

interface ChangeRequestTransitionReasonDialogProps {
  /** Target lifecycle state being confirmed, e.g. `rollback` or `canceled`. */
  target: string;
  /**
   * The state the change request is leaving. Needed only to tell a customer
   * bypass (`scheduled` out of `customer_approval`, `closed` out of
   * `customer_review`) from the same target reached any other way.
   */
  fromState?: string | null;
  /** True while the reason comment and/or the state change are in flight. */
  isSubmitting: boolean;
  /**
   * User-facing message for the most recent failed attempt, if any. Rendered
   * inline so the engineer sees exactly how far the attempt got — in
   * particular whether the reason was already recorded and must not be
   * retyped.
   */
  error?: string | null;
  /**
   * True once the reason has been recorded as a comment. The field locks and
   * a retry re-sends only the state change, so retrying after a failed patch
   * can't post the same reason twice.
   */
  reasonRecorded?: boolean;
  onClose: () => void;
  onConfirm: (reason: string) => void;
}

/**
 * Confirmation for a change-request transition that needs a stated reason:
 * the destructive `rollback` / `canceled` (effectively irreversible), and the
 * two customer bypasses ("Bypass customer approval" / "Bypass customer review",
 * an engineer answering for the customer). The confirm action stays disabled
 * until the Reason field has content.
 *
 * The reason is *not* part of the patch body — the change-request PATCH
 * contract has no reason or comment field. The caller records it as an
 * ordinary comment on the change request and only then patches the state; see
 * `CsmChangeRequestDetailPage`. This dialog only collects it.
 */
export default function ChangeRequestTransitionReasonDialog({
  target,
  fromState,
  isSubmitting,
  error,
  reasonRecorded,
  onClose,
  onConfirm,
}: ChangeRequestTransitionReasonDialogProps): JSX.Element {
  const [reason, setReason] = useState("");
  const { title, body, confirmLabel, confirmColor } = copyFor(target, fromState);
  const canSubmit = reason.trim().length > 0 && !isSubmitting;

  return (
    <Dialog
      open
      onClose={() => {
        if (!isSubmitting) onClose();
      }}
      maxWidth="xs"
      fullWidth
      aria-labelledby="cr-transition-reason-title"
    >
      <DialogTitle id="cr-transition-reason-title">{title}</DialogTitle>
      <DialogContent dividers>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 0.5 }}>
          {error && <Alert severity="error">{error}</Alert>}
          <DialogContentText variant="body2">{body}</DialogContentText>
          <TextField
            label="Reason"
            required
            autoFocus
            multiline
            minRows={3}
            fullWidth
            size="small"
            value={reason}
            disabled={isSubmitting || reasonRecorded}
            onChange={(e) => setReason(e.target.value)}
            helperText={
              reasonRecorded
                ? "Already recorded as a comment — retrying will only change the state."
                : "Recorded as a comment on this change request before the state changes."
            }
          />
        </Box>
      </DialogContent>
      <DialogActions>
        <Button color="inherit" onClick={onClose} disabled={isSubmitting}>
          Close
        </Button>
        <Button
          variant="contained"
          color={confirmColor}
          onClick={() => onConfirm(reason.trim())}
          disabled={!canSubmit}
          loading={isSubmitting}
        >
          {confirmLabel}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
