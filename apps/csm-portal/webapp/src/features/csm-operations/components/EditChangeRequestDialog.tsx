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
  AdapterDateFns,
  Alert,
  Box,
  Button,
  DatePickers,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControlLabel,
  FormHelperText,
  Switch,
} from "@wso2/oxygen-ui";
import { useMemo, useState, type JSX } from "react";
import { useSearchGroups } from "@api/useSearchGroups";
import type {
  BeChangeRequestDetail,
  BeGroup,
  BePatchChangeRequestPayload,
} from "@api/backend/types";
import AsyncEntitySelect from "@components/AsyncEntitySelect";
import {
  formatDateTimeLocal,
  isPastDateTime,
  parseDateTimeLocal,
} from "@utils/dateTime";

const { DateTimePicker, LocalizationProvider } = DatePickers;

interface EditChangeRequestDialogProps {
  cr: BeChangeRequestDetail;
  /** True while the PATCH is in flight; disables the actions. */
  isSaving: boolean;
  /**
   * User-facing message for the most recent failed save, if any. Rendered
   * inline in the dialog so the rejection is visible even if a page-level
   * error banner is occluded or the dialog is otherwise the only thing the
   * user is looking at.
   */
  saveError?: string | null;
  onClose: () => void;
  /** Submit only the changed fields (`PATCH /change-requests/{id}`). */
  onSave: (patch: BePatchChangeRequestPayload) => void;
}

/**
 * Convert a backend timestamp (`YYYY-MM-DD HH:MM:SS`, or ISO `T`-separated) to
 * the `YYYY-MM-DDTHH:MM` shape this form's state (and the DateTimePicker via
 * {@link parseDateTimeLocal}) uses. The value is treated as plain wall-clock
 * text so no timezone shift is applied.
 */
function toDateTimeLocal(raw?: string | null): string {
  const m = /^(\d{4}-\d{2}-\d{2})[ T](\d{2}:\d{2})/.exec(raw?.trim() ?? "");
  return m ? `${m[1]}T${m[2]}` : "";
}

/** Convert a `datetime-local` value back to the BE's `YYYY-MM-DD HH:MM:SS`. */
function toBackendDateTime(local: string): string {
  return `${local.replace("T", " ")}:00`;
}

/**
 * Edit the change-request fields the BE allows updating: planned start, the
 * assignment group, and the customer approved / reviewed flags. Only changed
 * fields are sent, and the BE requires at least one, so Save is disabled
 * until something differs.
 */
export default function EditChangeRequestDialog({
  cr,
  isSaving,
  saveError,
  onClose,
  onSave,
}: EditChangeRequestDialogProps): JSX.Element {
  const initialPlannedStart = useMemo(
    () => toDateTimeLocal(cr.plannedStartOn),
    [cr.plannedStartOn],
  );
  const initialAssignedTeamId = cr.assignedTeam?.id ?? "";
  const [plannedStart, setPlannedStart] = useState(initialPlannedStart);
  const [approved, setApproved] = useState(!!cr.hasCustomerApproved);
  const [reviewed, setReviewed] = useState(!!cr.hasCustomerReviewed);
  const [assignedTeamId, setAssignedTeamId] = useState(initialAssignedTeamId);

  const patch = useMemo<BePatchChangeRequestPayload>(() => {
    const next: BePatchChangeRequestPayload = {};
    if (plannedStart !== initialPlannedStart && plannedStart) {
      next.plannedStartOn = toBackendDateTime(plannedStart);
    }
    if (approved !== !!cr.hasCustomerApproved) next.isCustomerApproved = approved;
    if (reviewed !== !!cr.hasCustomerReviewed) next.isCustomerReviewed = reviewed;
    if (assignedTeamId !== initialAssignedTeamId && assignedTeamId) {
      next.assignedTeamId = assignedTeamId;
    }
    return next;
  }, [
    plannedStart,
    initialPlannedStart,
    approved,
    reviewed,
    assignedTeamId,
    initialAssignedTeamId,
    cr.hasCustomerApproved,
    cr.hasCustomerReviewed,
  ]);

  const hasChanges = Object.keys(patch).length > 0;
  // Non-blocking: editing a CR's planned start to a past instant is unusual
  // but not forbidden (e.g. recording when it actually started), so this
  // only warns.
  const plannedStartIsPast = isPastDateTime(parseDateTimeLocal(plannedStart));

  // The backend rejects a patch containing both isCustomerApproved and
  // isCustomerReviewed outright — they, and requestApproval, are mutually
  // exclusive. Mirror that here: once one of the two has been changed away
  // from its saved value, lock the other to its current value until this
  // save goes through (or the first change is undone), so the dialog can
  // never build the two-key payload the backend always refuses.
  const approvedChanged = approved !== !!cr.hasCustomerApproved;
  const reviewedChanged = reviewed !== !!cr.hasCustomerReviewed;
  const approvedLocked = reviewedChanged;
  const reviewedLocked = approvedChanged;

  return (
    <Dialog open onClose={onClose} maxWidth="xs" fullWidth>
      <DialogTitle>Edit change request</DialogTitle>
      <DialogContent dividers>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 0.5 }}>
          {saveError && (
            <Alert severity="error" sx={{ width: "100%" }}>
              {saveError}
            </Alert>
          )}
          <LocalizationProvider dateAdapter={AdapterDateFns}>
            <DateTimePicker
              label="Planned start"
              value={parseDateTimeLocal(plannedStart)}
              onChange={(next) =>
                setPlannedStart(
                  next instanceof Date && !Number.isNaN(next.getTime())
                    ? formatDateTimeLocal(next)
                    : "",
                )
              }
              slotProps={{
                textField: {
                  size: "small",
                  fullWidth: true,
                  helperText: plannedStartIsPast
                    ? "This date is in the past."
                    : undefined,
                },
                field: { clearable: true },
              }}
            />
          </LocalizationProvider>
          <Box>
            <FormControlLabel
              control={
                <Switch
                  checked={approved}
                  disabled={isSaving || approvedLocked}
                  // Guard the handler too, not just the `disabled` attribute:
                  // it's the actual mutual-exclusion enforcement, so it must
                  // not depend on the DOM ignoring input while disabled.
                  onChange={(e) => {
                    if (approvedLocked) return;
                    setApproved(e.target.checked);
                  }}
                />
              }
              label="Customer approved"
            />
            {approvedLocked && (
              <FormHelperText>
                Save the customer-reviewed change first — approved and
                reviewed can&apos;t be changed in the same save.
              </FormHelperText>
            )}
          </Box>
          <Box>
            <FormControlLabel
              control={
                <Switch
                  checked={reviewed}
                  disabled={isSaving || reviewedLocked}
                  onChange={(e) => {
                    if (reviewedLocked) return;
                    setReviewed(e.target.checked);
                  }}
                />
              }
              label="Customer reviewed"
            />
            {reviewedLocked && (
              <FormHelperText>
                Save the customer-approved change first — approved and
                reviewed can&apos;t be changed in the same save.
              </FormHelperText>
            )}
          </Box>
          <AsyncEntitySelect<BeGroup>
            id="cr-edit-assigned-team"
            label="Assignment group"
            placeholder="Search groups…"
            value={assignedTeamId}
            onChange={setAssignedTeamId}
            disabled={isSaving}
            useSearch={useSearchGroups}
            getId={(g) => g.id}
            getLabel={(g) => g.name}
            knownLabel={cr.assignedTeam?.name}
            helperText="Required before approval can be requested."
          />
        </Box>
      </DialogContent>
      <DialogActions>
        <Button color="inherit" onClick={onClose} disabled={isSaving}>
          Cancel
        </Button>
        <Button
          variant="contained"
          onClick={() => onSave(patch)}
          disabled={isSaving || !hasChanges}
        >
          {isSaving ? "Saving…" : "Save"}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
