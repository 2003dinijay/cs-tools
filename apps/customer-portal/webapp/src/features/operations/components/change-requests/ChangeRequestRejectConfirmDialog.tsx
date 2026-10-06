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
  Button,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Typography,
} from "@wso2/oxygen-ui";
import type { JSX } from "react";
import { getCustomerRejectConfirmCopy } from "@features/operations/utils/changeRequests";
import { ChangeRequestDecisionMode } from "@features/operations/types/changeRequests";

export type ChangeRequestRejectConfirmDialogProps = {
  open: boolean;
  /** Which answer is being given; decides the wording (reject vs. unsuccessful). */
  mode: ChangeRequestDecisionMode;
  isPending: boolean;
  onClose: () => void;
  onConfirm: () => void;
};

/**
 * Asks the customer to confirm the answer that cannot be taken back: rejecting
 * a change request cancels it, and marking it unsuccessful sends it into
 * rollback. Approving needs no confirmation and does not come through here.
 *
 * @param {ChangeRequestRejectConfirmDialogProps} props - Dialog state and callbacks.
 * @returns {JSX.Element} The confirmation dialog.
 */
export default function ChangeRequestRejectConfirmDialog({
  open,
  mode,
  isPending,
  onClose,
  onConfirm,
}: ChangeRequestRejectConfirmDialogProps): JSX.Element {
  const copy = getCustomerRejectConfirmCopy(mode);

  const handleClose = (): void => {
    if (isPending) return;
    onClose();
  };

  return (
    <Dialog
      open={open}
      onClose={handleClose}
      maxWidth="xs"
      fullWidth
      aria-labelledby="cr-reject-confirm-title"
      aria-describedby="cr-reject-confirm-description"
    >
      <DialogTitle id="cr-reject-confirm-title">{copy.title}</DialogTitle>
      <DialogContent id="cr-reject-confirm-description">
        <Typography variant="body1" color="text.primary">
          {copy.message}
        </Typography>
        {copy.hint && (
          <Typography variant="body2" color="text.secondary" sx={{ mt: 1.5 }}>
            {copy.hint}
          </Typography>
        )}
      </DialogContent>
      <DialogActions sx={{ px: 3, py: 2 }}>
        <Button
          variant="outlined"
          color="inherit"
          onClick={handleClose}
          disabled={isPending}
        >
          Go back
        </Button>
        <Button
          variant="contained"
          color="error"
          onClick={() => {
            if (!isPending) onConfirm();
          }}
          disabled={isPending}
          startIcon={
            isPending ? <CircularProgress size={16} color="inherit" /> : undefined
          }
        >
          {isPending ? "Submitting..." : copy.confirmLabel}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
