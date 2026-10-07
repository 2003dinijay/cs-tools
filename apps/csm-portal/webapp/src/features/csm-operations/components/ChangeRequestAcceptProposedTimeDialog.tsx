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
  Checkbox,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControlLabel,
  Typography,
} from "@wso2/oxygen-ui";
import { useState, type JSX } from "react";
import type { BeChangeRequestCustomerProposal, BeChangeRequestDetail } from "@api/backend/types";
import {
  customerProposalProposer,
  customerProposalProposerLabel,
  customerProposalWording,
  formatCrDateTime,
  formatCrWindow,
  PROPOSER_NOT_RECORDED,
  PROPOSER_NOT_RECORDED_ADVICE,
  proposedWindowMs,
} from "@features/csm-operations/utils/changeRequests";

interface ChangeRequestAcceptProposedTimeDialogProps {
  cr: BeChangeRequestDetail;
  /** The customer's proposal waiting for WSO2's answer. */
  proposal: BeChangeRequestCustomerProposal;
  /** True while the PATCH is in flight. */
  isSubmitting: boolean;
  /** The backend's refusal for the last attempt, shown verbatim. */
  error?: string | null;
  onClose: () => void;
  /** Sends `{confirmCustomerUpdatedDate: "agree", expectedCustomerUpdatedOn, expectedPlannedStartOn, expectedPlannedEndOn}`. */
  onConfirm: () => void;
}

/**
 * "Accept the proposed time?": the confirmation behind the banner's primary action. Shows the
 * planned window beside the proposed one, so what is about to be scheduled is in front of the
 * engineer, and says what follows: the change goes straight to Scheduled, the customer is not asked
 * again, no CAB approval.
 *
 * When the proposer is not recorded (the date is also written by WSO2 users in the previous system, or can be
 * left over from an earlier round), it says so, labels the window "Proposed time" instead of "Proposed by
 * the customer", and Accept needs an explicit confirmation that the time really came from the customer:
 * accepting it is the engineer's decision, never a default.
 */
export default function ChangeRequestAcceptProposedTimeDialog({
  cr,
  proposal,
  isSubmitting,
  error,
  onClose,
  onConfirm,
}: ChangeRequestAcceptProposedTimeDialogProps): JSX.Element {
  const proposer = customerProposalProposer(proposal);
  const wording = customerProposalWording(proposer);
  const proposed = proposedWindowMs(cr, proposal);
  const [checked, setChecked] = useState(false);
  const needsConfirmation = !proposer;
  const canConfirm = !isSubmitting && (!needsConfirmation || checked);

  return (
    <Dialog open onClose={onClose} maxWidth="xs" fullWidth aria-labelledby="cr-accept-proposal-title">
      <DialogTitle id="cr-accept-proposal-title">Accept the proposed time?</DialogTitle>
      <DialogContent dividers>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 0.5 }}>
          {error && (
            <Alert severity="error" role="alert">
              {error}
            </Alert>
          )}
          <Typography variant="body2" color="text.secondary">
            {`The change will be scheduled for ${formatCrWindow(proposal.startOn, proposed?.endMs ?? null)}. ` +
              "The customer sees that you accepted it and is not asked again. No CAB approval is needed."}
          </Typography>
          <Box sx={{ display: "flex", flexDirection: "column", gap: 0.25 }}>
            <Typography variant="caption" color="text.secondary" sx={{ textTransform: "uppercase", letterSpacing: 0.4 }}>
              Planned now
            </Typography>
            <Typography variant="body2">{formatCrWindow(cr.plannedStartOn, cr.plannedEndOn)}</Typography>
          </Box>
          <Box sx={{ display: "flex", flexDirection: "column", gap: 0.25 }}>
            <Typography variant="caption" color="text.secondary" sx={{ textTransform: "uppercase", letterSpacing: 0.4 }}>
              {wording.windowLabel}
            </Typography>
            <Typography variant="body2" sx={{ fontWeight: 600 }}>
              {formatCrWindow(proposal.startOn, proposed?.endMs ?? null)}
            </Typography>
          </Box>
          {proposer ? (
            <Typography variant="body2" color="text.secondary">
              Proposed by {customerProposalProposerLabel(proposer)}
              {proposer.on ? ` on ${formatCrDateTime(proposer.on)}` : ""}.
            </Typography>
          ) : (
            <Alert severity="warning" role="status">
              <strong>{PROPOSER_NOT_RECORDED}</strong> {PROPOSER_NOT_RECORDED_ADVICE}
            </Alert>
          )}
          {needsConfirmation && (
            <FormControlLabel
              sx={{ alignItems: "center", m: 0 }}
              disabled={isSubmitting}
              control={
                <Checkbox size="small" checked={checked} onChange={(e) => setChecked(e.target.checked)} />
              }
              label={<Typography variant="body2">I have checked that the customer proposed this time.</Typography>}
            />
          )}
        </Box>
      </DialogContent>
      <DialogActions>
        <Button color="inherit" onClick={onClose} disabled={isSubmitting}>
          Close
        </Button>
        <Button variant="contained" color="success" onClick={onConfirm} disabled={!canConfirm} loading={isSubmitting}>
          Accept proposed time
        </Button>
      </DialogActions>
    </Dialog>
  );
}
