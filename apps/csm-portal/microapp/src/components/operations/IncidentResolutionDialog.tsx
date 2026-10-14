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

import { useState } from "react";
import {
  Button,
  Dialog,
  FormControl,
  InputLabel,
  MenuItem,
  Select,
  Stack,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import type { IncidentResolutionCode } from "@src/types";
import { DialogPaper } from "@components/common/DialogPaper";
import { INCIDENT_RESOLUTION_CODE_LABELS, INCIDENT_RESOLUTION_CODES } from "./incidentConfig";

interface IncidentResolutionDialogProps {
  /** Which transition this resolution is being collected for — only used for the title. */
  target: "RESOLVED" | "CLOSED";
  isSubmitting: boolean;
  onClose: () => void;
  onSubmit: (fields: { resolutionCode: IncidentResolutionCode; resolutionNotes: string }) => void;
}

const TARGET_TITLE: Record<IncidentResolutionDialogProps["target"], string> = {
  RESOLVED: "Resolve incident",
  CLOSED: "Close incident",
};

/** Collects the resolution code and notes ServiceNow requires before an incident can move to
 * Resolved or Closed — mirrors the webapp's IncidentResolutionDialog, where both are required. */
export function IncidentResolutionDialog({ target, isSubmitting, onClose, onSubmit }: IncidentResolutionDialogProps) {
  const [resolutionCode, setResolutionCode] = useState<IncidentResolutionCode | "">("");
  const [resolutionNotes, setResolutionNotes] = useState("");

  const canSubmit = !!resolutionCode && resolutionNotes.trim().length > 0 && !isSubmitting;

  return (
    <Dialog
      open
      // Backdrop click / Esc must not dismiss the dialog mid-request; Cancel is disabled then too.
      onClose={() => {
        if (!isSubmitting) onClose();
      }}
      slots={{ paper: DialogPaper }}
      slotProps={{ paper: { sx: { bgcolor: "background.default", p: 1.5, gap: 2, m: 2 } } }}
    >
      <Typography variant="h6" fontWeight={650}>
        {TARGET_TITLE[target]}
      </Typography>

      <FormControl size="small" fullWidth>
        <InputLabel id="incident-resolution-code-label">Resolution code</InputLabel>
        <Select
          labelId="incident-resolution-code-label"
          label="Resolution code"
          value={resolutionCode}
          onChange={(e) => setResolutionCode(e.target.value as IncidentResolutionCode)}
        >
          {INCIDENT_RESOLUTION_CODES.map((code) => (
            <MenuItem key={code} value={code}>
              {INCIDENT_RESOLUTION_CODE_LABELS[code]}
            </MenuItem>
          ))}
        </Select>
      </FormControl>

      <TextField
        label="Resolution notes"
        multiline
        minRows={3}
        fullWidth
        size="small"
        value={resolutionNotes}
        onChange={(e) => setResolutionNotes(e.target.value)}
      />

      <Stack direction="row" justifyContent="end" gap={1}>
        <Button variant="outlined" onClick={onClose} disabled={isSubmitting}>
          Cancel
        </Button>
        <Button
          variant="contained"
          disabled={!canSubmit}
          onClick={() => {
            if (!resolutionCode) return;
            onSubmit({ resolutionCode, resolutionNotes: resolutionNotes.trim() });
          }}
        >
          Submit
        </Button>
      </Stack>
    </Dialog>
  );
}
