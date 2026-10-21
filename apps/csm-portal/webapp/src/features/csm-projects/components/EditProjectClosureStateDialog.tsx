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
  FormControl,
  InputLabel,
  MenuItem,
  Select,
  Typography,
} from "@wso2/oxygen-ui";
import { useState, type JSX } from "react";
import {
  PROJECT_CLOSURE_STATES,
  type ProjectClosureState,
} from "@features/csm-projects/api/usePatchProjectClosureState";

const CLOSURE_STATE_LABELS: Record<ProjectClosureState, string> = {
  OPEN: "Open",
  READ_ONLY: "Read Only",
  CLOSED: "Closed",
  RESTRICTED: "Restricted",
  SUSPENDED: "Suspended",
};

/** Matches `closureStatePresentation`'s own case-insensitive lookup against
 *  the current display text (e.g. "Read Only", "open"), so the dialog opens
 *  on whatever state the project is actually in rather than always defaulting
 *  to the first option. `undefined` when the current value isn't one of the
 *  five known states (a project the overall-state column has never been set
 *  on) — the caller then starts with nothing selected. */
function matchCurrentClosureState(current: string | null): ProjectClosureState | "" {
  const normalized = current?.trim().toUpperCase().replace(/\s+/g, "_");
  return (
    PROJECT_CLOSURE_STATES.find((s) => s === normalized) ?? ""
  );
}

interface EditProjectClosureStateDialogProps {
  /** The project's current `closureState`, as returned by `GET /projects/{id}`
   *  (display text such as "Read Only", or `null`). */
  currentClosureState: string | null;
  /** True while the PATCH is in flight; disables the picker/actions. */
  isSaving: boolean;
  /** User-facing message for the most recent failed save, if any. */
  saveError?: string | null;
  onClose: () => void;
  /** Submit the newly selected state (`PATCH /projects/{id} {closureState}`). */
  onSave: (closureState: ProjectClosureState) => void;
}

/**
 * Admin-only direct override of a project's overall closure state. A
 * deliberately simple picker over the five real values — there is no
 * mechanism here for reviewing or editing the three sub-state fields the
 * value is normally derived from (see entity-service's own CLAUDE.md,
 * "PATCH /projects/{id}: closureState is a direct admin override", for the
 * standing caveat that a later edit to those fields silently recomputes over
 * whatever is set here). Mirrors `EditAccountTeamsDialog`'s shape: a plain
 * controlled dialog: the caller owns the mutation and passes
 * `isSaving`/`saveError` down rather than this component driving its own
 * PATCH call.
 */
export default function EditProjectClosureStateDialog({
  currentClosureState,
  isSaving,
  saveError,
  onClose,
  onSave,
}: EditProjectClosureStateDialogProps): JSX.Element {
  const initial = matchCurrentClosureState(currentClosureState);
  const [closureState, setClosureState] = useState<ProjectClosureState | "">(initial);

  const hasChanges = closureState !== "" && closureState !== initial;

  return (
    <Dialog open onClose={onClose} maxWidth="xs" fullWidth>
      <DialogTitle>Edit closure state</DialogTitle>
      <DialogContent dividers>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 0.5 }}>
          {saveError && <Alert severity="error">{saveError}</Alert>}

          <FormControl size="small" fullWidth required>
            <InputLabel
              id="edit-project-closure-state-label"
              shrink={closureState !== ""}
              sx={{ top: "0px !important" }}
            >
              Closure state
            </InputLabel>
            <Select
              labelId="edit-project-closure-state-label"
              label="Closure state"
              value={closureState}
              onChange={(e) => setClosureState(e.target.value as ProjectClosureState)}
              disabled={isSaving}
              notched={closureState !== ""}
            >
              {PROJECT_CLOSURE_STATES.map((s) => (
                <MenuItem key={s} value={s}>
                  {CLOSURE_STATE_LABELS[s]}
                </MenuItem>
              ))}
            </Select>
          </FormControl>

          <Typography variant="caption" color="text.secondary">
            This sets the project&apos;s overall closure state directly,
            bypassing how it&apos;s normally derived. A later update that
            touches any of the underlying closure sub-states will recompute
            and silently overwrite this value.
          </Typography>
        </Box>
      </DialogContent>
      <DialogActions>
        <Button color="inherit" onClick={onClose} disabled={isSaving}>
          Cancel
        </Button>
        <Button
          variant="contained"
          onClick={() => {
            if (closureState) onSave(closureState);
          }}
          disabled={isSaving || !hasChanges}
        >
          {isSaving ? "Saving…" : "Save"}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
