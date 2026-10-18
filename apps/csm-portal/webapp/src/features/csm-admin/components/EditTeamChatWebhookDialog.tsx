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
  DialogTitle,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { useMemo, useState, type JSX } from "react";
import { chatWebhookUrlError } from "@features/csm-admin/utils/chatWebhookValidation";

interface EditTeamChatWebhookDialogProps {
  currentUrl: string | null;
  /** True while the PATCH is in flight; disables the action/field. */
  isSaving: boolean;
  /** User-facing message for the most recent failed save, if any. */
  saveError?: string | null;
  onClose: () => void;
  onSave: (chatWebhookUrl: string) => void;
}

/**
 * Edit a team's own Google Chat space link. Validates the same
 * `https://chat.googleapis.com/...`-only rule entity-service enforces
 * (`chatWebhookUrlError`) before the field is even submitted — a display
 * convenience, since the backend is the real, authoritative check and its
 * own refusal reason (`saveError`) is shown verbatim if the two ever
 * disagree. An empty value clears the team's webhook.
 */
export default function EditTeamChatWebhookDialog({
  currentUrl,
  isSaving,
  saveError,
  onClose,
  onSave,
}: EditTeamChatWebhookDialogProps): JSX.Element {
  const [value, setValue] = useState(currentUrl ?? "");
  const [touched, setTouched] = useState(false);

  const fieldError = useMemo(() => chatWebhookUrlError(value), [value]);
  const hasChanged = value.trim() !== (currentUrl ?? "").trim();

  return (
    <Dialog open onClose={onClose} maxWidth="xs" fullWidth>
      <DialogTitle>Edit Chat space link</DialogTitle>
      <DialogContent dividers>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 0.5 }}>
          {saveError && <Alert severity="error">{saveError}</Alert>}

          <TextField
            id="edit-team-chat-webhook-url"
            label="Google Chat webhook URL"
            placeholder="https://chat.googleapis.com/v1/spaces/.../messages?..."
            value={value}
            onChange={(e) => setValue(e.target.value)}
            onBlur={() => setTouched(true)}
            disabled={isSaving}
            fullWidth
            error={touched && !!fieldError}
            helperText={
              touched && fieldError
                ? fieldError
                : "Leave blank to remove this team's Chat space link."
            }
          />

          <Typography variant="caption" color="text.secondary">
            Anyone holding this link can post into the team's Chat space —
            treat it as a credential. Only entity-service's own check is
            final; this is a quick client-side check only.
          </Typography>
        </Box>
      </DialogContent>
      <DialogActions>
        <Button color="inherit" onClick={onClose} disabled={isSaving}>
          Cancel
        </Button>
        <Button
          variant="contained"
          onClick={() => onSave(value.trim())}
          disabled={isSaving || !hasChanged || !!fieldError}
        >
          {isSaving ? "Saving…" : "Save"}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
