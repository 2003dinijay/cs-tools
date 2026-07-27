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

import { useEffect, useState } from "react";
import {
  Button,
  Chip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Divider,
  IconButton,
  Stack,
  Typography,
} from "@wso2/oxygen-ui";
import { X } from "@wso2/oxygen-ui-icons-react";
import {
  EMPTY_INCIDENT_FILTERS,
  INCIDENT_PRIORITIES,
  INCIDENT_PRIORITY_LABELS,
  type IncidentFilters,
} from "./incidentConfig";

function toggle<T>(list: T[], value: T): T[] {
  return list.includes(value) ? list.filter((v) => v !== value) : [...list, value];
}

interface IncidentsFiltersSheetProps {
  open: boolean;
  onClose: () => void;
  filters: IncidentFilters;
  onApply: (filters: IncidentFilters) => void;
}

// Mirrors ChangeRequestsFiltersSheet.tsx's bottom-sheet pattern, trimmed to the one filter
// dimension the backend actually supports for incidents (priority — see incidentConfig.ts).
export function IncidentsFiltersSheet({ open, onClose, filters, onApply }: IncidentsFiltersSheetProps) {
  const [draft, setDraft] = useState<IncidentFilters>(filters);

  useEffect(() => {
    if (open) setDraft(filters);
    // eslint-disable-next-line react-hooks/exhaustive-deps -- only re-seed on open
  }, [open]);

  return (
    <Dialog open={open} onClose={onClose} fullWidth maxWidth="xs">
      <DialogTitle sx={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
        Filters
        <IconButton size="small" aria-label="Close filters" onClick={onClose}>
          <X size={18} />
        </IconButton>
      </DialogTitle>

      <Divider />

      <DialogContent>
        <Stack gap={1}>
          <Typography variant="subtitle2">Priority</Typography>
          <Stack direction="row" gap={1} flexWrap="wrap">
            {INCIDENT_PRIORITIES.map((priority) => {
              const isSelected = draft.priorities.includes(priority);
              return (
                <Chip
                  key={priority}
                  label={INCIDENT_PRIORITY_LABELS[priority]}
                  size="small"
                  variant={isSelected ? "filled" : "outlined"}
                  color={isSelected ? "primary" : "default"}
                  onClick={() => setDraft({ ...draft, priorities: toggle(draft.priorities, priority) })}
                />
              );
            })}
          </Stack>
        </Stack>
      </DialogContent>

      <Divider />

      <DialogActions>
        <Button
          onClick={() => {
            setDraft(EMPTY_INCIDENT_FILTERS);
            onApply(EMPTY_INCIDENT_FILTERS);
            onClose();
          }}
        >
          Clear all
        </Button>
        <Button
          variant="contained"
          onClick={() => {
            onApply(draft);
            onClose();
          }}
        >
          Apply
        </Button>
      </DialogActions>
    </Dialog>
  );
}
