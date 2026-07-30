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
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { useState, type JSX } from "react";
import { incidentStateLabel } from "@features/csm-operations/utils/incidents";
import type { BeIncidentState } from "@api/backend/types";

interface IncidentResolutionDialogProps {
  /** The transition this dialog is confirming — always RESOLVED or CLOSED. */
  target: Extract<BeIncidentState, "RESOLVED" | "CLOSED">;
  isSubmitting: boolean;
  onClose: () => void;
  onSubmit: (fields: { resolutionCode: string; resolutionNotes: string }) => void;
}

/**
 * Collects the resolution code + notes ServiceNow requires before an
 * incident can move to `RESOLVED`/`CLOSED` — confirmed live: those two state
 * values 500 without them (see `BeUpdateIncidentPayload`'s doc comment).
 * Doubles as the confirmation step for these two transitions, same role
 * `ResolutionDialog` plays for a case's close/propose-solution — there is no
 * separate plain confirm dialog for them.
 */
export default function IncidentResolutionDialog({
  target,
  isSubmitting,
  onClose,
  onSubmit,
}: IncidentResolutionDialogProps): JSX.Element {
  const [resolutionCode, setResolutionCode] = useState("");
  const [resolutionNotes, setResolutionNotes] = useState("");
  const [touched, setTouched] = useState(false);

  const hasCode = resolutionCode.trim().length > 0;
  const hasNotes = resolutionNotes.trim().length > 0;
  const canSubmit = hasCode && hasNotes && !isSubmitting;

  return (
    <Dialog open onClose={onClose} maxWidth="xs" fullWidth>
      <DialogTitle>Move to {incidentStateLabel(target)}</DialogTitle>
      <DialogContent dividers sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        <Typography variant="body2" color="text.secondary">
          ServiceNow requires a resolution to move this incident to{" "}
          {incidentStateLabel(target)}.
        </Typography>
        <TextField
          label="Resolution code"
          required
          size="small"
          fullWidth
          value={resolutionCode}
          onChange={(e) => setResolutionCode(e.target.value)}
          onBlur={() => setTouched(true)}
          error={touched && !hasCode}
          helperText={touched && !hasCode ? "Resolution code is required." : undefined}
          disabled={isSubmitting}
        />
        <TextField
          label="Resolution notes"
          required
          fullWidth
          multiline
          minRows={3}
          value={resolutionNotes}
          onChange={(e) => setResolutionNotes(e.target.value)}
          onBlur={() => setTouched(true)}
          error={touched && !hasNotes}
          helperText={
            touched && !hasNotes
              ? "Resolution notes are required."
              : "Describe the resolution…"
          }
          disabled={isSubmitting}
        />
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} disabled={isSubmitting}>
          Cancel
        </Button>
        <Button
          variant="contained"
          color="success"
          disabled={!canSubmit}
          onClick={() => {
            if (!hasCode || !hasNotes) {
              setTouched(true);
              return;
            }
            onSubmit({
              resolutionCode: resolutionCode.trim(),
              resolutionNotes: resolutionNotes.trim(),
            });
          }}
        >
          {isSubmitting ? "Saving…" : `Move to ${incidentStateLabel(target)}`}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
