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
  FormHelperText,
  InputLabel,
  MenuItem,
  Select,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { useMemo, useState, type JSX } from "react";
import { useSearchGroups } from "@api/useSearchGroups";
import { useSearchItServices } from "@api/useSearchItServices";
import { useSearchInternalUsersByName } from "@api/useSearchUsersByName";
import AsyncEntitySelect from "@components/AsyncEntitySelect";
import {
  CATEGORY_OPTIONS,
  IMPACT_OPTIONS,
  URGENCY_OPTIONS,
  itServiceLabel,
  userLabel,
} from "@features/csm-operations/utils/incidentFormOptions";
import { bulkIncidentTargetStates, incidentStateLabel } from "@features/csm-operations/utils/incidents";
import { INCIDENT_RESOLUTION_CODES, INCIDENT_RESOLUTION_CODE_LABELS } from "@features/csm-operations/utils/incidentResolution";
import type {
  BeGroup,
  BeIncident,
  BeIncidentCategory,
  BeIncidentImpact,
  BeIncidentResolutionCode,
  BeIncidentState,
  BeIncidentUrgency,
  BeItService,
  BeUpdateIncidentPayload,
  BeUser,
} from "@api/backend/types";

interface BulkEditIncidentsDialogProps {
  /** The selected incidents (none closed or cancelled: the list does not let those be selected). */
  incidents: BeIncident[];
  isApplying: boolean;
  onClose: () => void;
  /** The one change to apply to every selected incident: only the fields set here. */
  onApply: (patch: BeUpdateIncidentPayload) => void;
}

const NO_CHANGE = "";

/**
 * Bulk edit for the incidents selected in the list: every field left on
 * "No change" is not sent, so each incident keeps its own value. The State
 * offers only what every selected incident can move to (the Edit dialog's own
 * table), Resolved and Closed included; those two need a resolution code and
 * notes, as on a single incident, applied to all. The backend stays the
 * authority: what it refuses for one incident (e.g. Closed by somebody who is
 * not the assignee) is reported for that incident and the rest go through.
 */
export default function BulkEditIncidentsDialog({ incidents, isApplying, onClose, onApply }: BulkEditIncidentsDialogProps): JSX.Element {
  const [state, setState] = useState<BeIncidentState | "">(NO_CHANGE);
  const [resolutionCode, setResolutionCode] = useState<BeIncidentResolutionCode | "">(NO_CHANGE);
  const [resolutionNotes, setResolutionNotes] = useState("");
  const [impact, setImpact] = useState<BeIncidentImpact | "">(NO_CHANGE);
  const [urgency, setUrgency] = useState<BeIncidentUrgency | "">(NO_CHANGE);
  const [category, setCategory] = useState<BeIncidentCategory | "">(NO_CHANGE);
  const [serviceId, setServiceId] = useState("");
  const [assignmentGroupId, setAssignmentGroupId] = useState("");
  const [assignedEngineerId, setAssignedEngineerId] = useState("");
  const [workNotes, setWorkNotes] = useState("");

  const targetStates = useMemo(() => bulkIncidentTargetStates(incidents.map((i) => i.state)), [incidents]);
  const needsResolution = state === "RESOLVED" || state === "CLOSED";
  const resolutionMissing = needsResolution && (!resolutionCode || !resolutionNotes.trim());

  const patch = useMemo<BeUpdateIncidentPayload>(() => {
    const next: BeUpdateIncidentPayload = {};
    if (state) next.state = state;
    if (needsResolution && resolutionCode) next.resolutionCode = resolutionCode;
    if (needsResolution && resolutionNotes.trim()) next.resolutionNotes = resolutionNotes.trim();
    if (impact) next.impact = impact;
    if (urgency) next.urgency = urgency;
    if (category) next.category = category;
    if (serviceId) next.serviceId = serviceId;
    if (assignmentGroupId) next.assignmentGroupId = assignmentGroupId;
    if (assignedEngineerId) next.assignedEngineerId = assignedEngineerId;
    if (workNotes.trim()) next.workNotes = workNotes.trim();
    return next;
  }, [state, needsResolution, resolutionCode, resolutionNotes, impact, urgency, category, serviceId, assignmentGroupId, assignedEngineerId, workNotes]);

  const canApply = Object.keys(patch).length > 0 && !resolutionMissing && !isApplying;
  const count = incidents.length;

  const select = <V extends string>(
    id: string,
    label: string,
    value: V | "",
    onChange: (v: V | "") => void,
    options: Array<{ value: V; label: string }>,
    helper?: string,
  ): JSX.Element => (
    <FormControl fullWidth size="small" disabled={isApplying}>
      <InputLabel id={`${id}-label`} shrink>
        {label}
      </InputLabel>
      <Select
        labelId={`${id}-label`}
        label={label}
        value={value}
        displayEmpty
        onChange={(e) => onChange(String(e.target.value) as V | "")}
      >
        <MenuItem value={NO_CHANGE}>
          <Typography component="span" color="text.secondary">
            No change
          </Typography>
        </MenuItem>
        {options.map((o) => (
          <MenuItem key={o.value} value={o.value}>
            {o.label}
          </MenuItem>
        ))}
      </Select>
      {helper && <FormHelperText>{helper}</FormHelperText>}
    </FormControl>
  );

  return (
    <Dialog open onClose={isApplying ? undefined : onClose} maxWidth="sm" fullWidth>
      <DialogTitle>
        Edit {count} incident{count === 1 ? "" : "s"}
      </DialogTitle>
      <DialogContent dividers sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        <Typography variant="body2" color="text.secondary">
          Only the fields you change are applied; the rest keep each incident's own value.
        </Typography>
        {select(
          "bulk-incident-state",
          "State",
          state,
          setState,
          targetStates.map((s) => ({ value: s, label: incidentStateLabel(s) })),
          targetStates.length === 0
            ? "The selected incidents have no state they can all move to."
            : "Only the states every selected incident can move to.",
        )}
        {needsResolution && (
          <Box sx={{ display: "flex", flexDirection: "column", gap: 2, pl: 1.5, borderLeft: 2, borderColor: "divider" }}>
            {select(
              "bulk-incident-resolution-code",
              "Resolution code",
              resolutionCode,
              setResolutionCode,
              INCIDENT_RESOLUTION_CODES.map((c) => ({ value: c, label: INCIDENT_RESOLUTION_CODE_LABELS[c] })),
            )}
            <TextField
              id="bulk-incident-resolution-notes"
              label="Resolution notes"
              required
              multiline
              minRows={2}
              size="small"
              value={resolutionNotes}
              onChange={(e) => setResolutionNotes(e.target.value)}
              disabled={isApplying}
              helperText={`Required to move an incident to ${incidentStateLabel(state)}; the same for every selected incident.`}
            />
          </Box>
        )}
        <Box sx={{ display: "grid", gridTemplateColumns: { xs: "1fr", sm: "1fr 1fr 1fr" }, gap: 2 }}>
          {select("bulk-incident-impact", "Impact", impact, setImpact, IMPACT_OPTIONS)}
          {select("bulk-incident-urgency", "Urgency", urgency, setUrgency, URGENCY_OPTIONS)}
          {select("bulk-incident-category", "Category", category, setCategory, CATEGORY_OPTIONS)}
        </Box>
        <AsyncEntitySelect<BeItService>
          id="bulk-incident-service"
          label="Service"
          placeholder="No change"
          value={serviceId}
          onChange={setServiceId}
          disabled={isApplying}
          useSearch={useSearchItServices}
          getId={(s) => s.id}
          getLabel={itServiceLabel}
        />
        <Box sx={{ display: "grid", gridTemplateColumns: { xs: "1fr", sm: "1fr 1fr" }, gap: 2 }}>
          <AsyncEntitySelect<BeGroup>
            id="bulk-incident-assignment-group"
            label="Assignment group"
            placeholder="No change"
            value={assignmentGroupId}
            onChange={setAssignmentGroupId}
            disabled={isApplying}
            useSearch={useSearchGroups}
            getId={(g) => g.id}
            getLabel={(g) => g.name}
          />
          <AsyncEntitySelect<BeUser>
            id="bulk-incident-assigned-to"
            label="Assigned to"
            placeholder="No change"
            value={assignedEngineerId}
            onChange={setAssignedEngineerId}
            disabled={isApplying}
            useSearch={useSearchInternalUsersByName}
            getId={(u) => u.id!}
            getLabel={userLabel}
          />
        </Box>
        <TextField
          id="bulk-incident-work-notes"
          label="Work note (internal)"
          multiline
          minRows={2}
          size="small"
          value={workNotes}
          onChange={(e) => setWorkNotes(e.target.value)}
          disabled={isApplying}
          helperText="Added to every selected incident; never shown to the customer."
        />
        {resolutionMissing && (
          <Alert severity="info" role="status">
            {incidentStateLabel(state)} needs a resolution code and notes.
          </Alert>
        )}
      </DialogContent>
      <DialogActions>
        <Button color="inherit" onClick={onClose} disabled={isApplying}>
          Cancel
        </Button>
        <Button variant="contained" onClick={() => onApply(patch)} disabled={!canApply}>
          {isApplying ? "Applying…" : `Apply to ${count} incident${count === 1 ? "" : "s"}`}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
