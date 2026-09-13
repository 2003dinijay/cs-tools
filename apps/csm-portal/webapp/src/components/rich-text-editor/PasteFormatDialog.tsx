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
  Typography,
} from "@wso2/oxygen-ui";
import type { JSX } from "react";

interface PasteFormatDialogProps {
  /** Insert the pasted content with its source formatting intact. */
  onKeepFormatting: () => void;
  /** Discard all source formatting and insert plain text only. */
  onRemoveFormatting: () => void;
  /** Dismiss without inserting anything (e.g. Escape or backdrop click). */
  onClose: () => void;
}

/**
 * Prompt shown when pasted clipboard content is detected as coming from a
 * word processor or online document editor (see
 * `isWordOrGoogleDocsPasteHtml` in richTextEditor.tsx). Every other paste
 * source is inserted silently, exactly as before -- this dialog only ever
 * appears for that one detected case, and only ever offers this one binary
 * choice.
 */
export default function PasteFormatDialog({
  onKeepFormatting,
  onRemoveFormatting,
  onClose,
}: PasteFormatDialogProps): JSX.Element {
  return (
    <Dialog open onClose={onClose} maxWidth="xs" fullWidth>
      <DialogTitle>Paste formatted content?</DialogTitle>
      <DialogContent>
        <Typography variant="body2" color="text.secondary">
          The content you pasted carries formatting from its source document.
          Keep it, or paste as plain text instead.
        </Typography>
      </DialogContent>
      <DialogActions>
        <Button onClick={onRemoveFormatting}>Remove Formatting</Button>
        <Button variant="contained" onClick={onKeepFormatting} autoFocus>
          Keep Formatting
        </Button>
      </DialogActions>
    </Dialog>
  );
}
