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
  Box,
  Button,
  Checkbox,
  DatePickers,
  Divider,
  FormControlLabel,
  Grid,
  IconButton,
  InputAdornment,
  Paper,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { ChevronDown, ChevronUp, ListFilter, Search, X } from "@wso2/oxygen-ui-icons-react";
import { useMemo, useState, type JSX } from "react";
import type { BeIncidentPriority } from "@api/backend/types";
import { useTeams } from "@features/csm-dashboard/api/useTeams";
import {
  countActiveIncidentFilters,
  incidentPriorityLabel,
  incidentStateLabel,
  INCIDENT_FILTER_CATEGORIES,
  INCIDENT_PRIORITIES,
  INCIDENT_STATES,
  type IncidentFilters,
} from "@features/csm-operations/utils/incidents";
import {
  readIncidentFiltersFromUrl,
  writeIncidentFiltersToUrl,
} from "@features/csm-operations/utils/incidentsFiltersUrl";
import IncidentProductMultiSelect from "@features/csm-operations/components/IncidentProductMultiSelect";
import SavedViewsMenu from "@features/csm-operations/components/SavedViewsMenu";
import MultiSelectField from "@components/MultiSelectField";
import OperationsAdvancedFilters from "@features/csm-operations/components/OperationsAdvancedFilters";
import {
  countAdvancedRows,
  dateAdvancedField,
  multiAdvancedField,
  type OperationsAdvancedField,
} from "@features/csm-operations/utils/operationsAdvancedFields";
import { useSearchGroups } from "@api/useSearchGroups";
import AsyncEntityMultiSelect from "@components/AsyncEntityMultiSelect";
import { useSearchItServices } from "@api/useSearchItServices";
import AsyncAssignedUserIdMultiSelect from "@features/csm-cases/components/AsyncAssignedUserIdMultiSelect";

const { DatePicker, LocalizationProvider } = DatePickers;

/**
 * "YYYY-MM-DD" to a local-midnight Date (avoids the UTC-parse day-shift
 * `new Date(dateString)` can cause depending on the viewer's timezone) —
 * this is purely how the calendar widget itself displays the picked day; the
 * picked day is then interpreted as a UTC calendar date on the wire (see
 * the "(UTC)" field labels below and `incidentDateOnlyToUTCStart`/`-End`),
 * matching how the change-requests tab's date pickers already work.
 */
function parseDateOnly(value: string): Date | null {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value);
  if (!match) return null;
  const date = new Date(Number(match[1]), Number(match[2]) - 1, Number(match[3]));
  return Number.isNaN(date.getTime()) ? null : date;
}

/** Local-midnight Date back to "YYYY-MM-DD", matching IncidentFilters'
 * createdStartDate/createdEndDate wire format. */
function formatDateOnly(date: Date): string {
  const y = date.getFullYear();
  const m = String(date.getMonth() + 1).padStart(2, "0");
  const d = String(date.getDate()).padStart(2, "0");
  return `${y}-${m}-${d}`;
}

/** Today's UTC calendar date, as a local-midnight Date (see `parseDateOnly`)
 * — the upper bound for both created-date fields, since incidents can't be
 * created in the future. */
function todayUTCDateOnly(): Date {
  const now = new Date();
  return new Date(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate());
}

interface IncidentsFilterBarProps {
  filters: IncidentFilters;
  onChange: (next: IncidentFilters) => void;
  onReset: () => void;
  isFiltersOpen: boolean;
  onFiltersToggle: () => void;
}

/**
 * Search + filters bar for the Incidents tab: state, priority, category,
 * service, assignee, product, SRE team, SLA-violated, and the created and
 * updated date ranges (see `IncidentSearchPayload.filters` in openapi.yaml).
 * Category and the updated range are PostgreSQL-only on the backend. The
 * created-date bounds are inclusive and interpreted in
 * UTC by the backend, so both date fields are labelled "(UTC)" rather than
 * silently letting a viewer in another timezone assume their own — picking
 * "1 May" here means the UTC calendar day, not midnight in the viewer's
 * timezone.
 */
export default function IncidentsFilterBar({
  filters,
  onChange,
  onReset,
  isFiltersOpen,
  onFiltersToggle,
}: IncidentsFilterBarProps): JSX.Element {
  const activeCount = countActiveIncidentFilters(filters);
  const hasActive = activeCount > 0;

  // Recomputed every render (not memoized) — a `useMemo(..., [])` would
  // freeze this at the component's mount date and stop matching "today" for
  // any session left open across a UTC midnight.
  const today = todayUTCDateOnly();
  const createdEndDate = parseDateOnly(filters.createdEndDate);
  const createdStartDate = parseDateOnly(filters.createdStartDate);
  const fromMaxDate = createdEndDate && createdEndDate < today ? createdEndDate : today;

  const priorityOptions = useMemo(
    () =>
      INCIDENT_PRIORITIES.map((p) => ({
        value: p,
        label: incidentPriorityLabel(p),
      })),
    [],
  );

  // Assignment group filter: any group, searched. The SRE teams' own groups
  // (registry `sre-abt` family, below) only supply names for ids carried in
  // from links built before. History of the old "SRE Team" control: incidents
  // are SRE-owned, so it was scoped to the `sre-abt`
  // team family (unlike the cases list's own "SRE Team" filter, which is
  // deliberately scoped to `cre-abt` per an explicit, later-flagged-as-
  // likely-mistaken product instruction — see `CasesFilterBar.tsx`'s
  // `sreTeamOptions`. This is a fresh field on a different resource, not a
  // copy of that decision).
  const { data: teams } = useTeams(true, "sre-abt");
  const sreTeamOptions = useMemo(
    () =>
      (teams ?? [])
        .filter(
          (t): t is typeof t & { sreGroupId: string } =>
            Boolean(t.sreGroupId) && t.family === "sre-abt",
        )
        .map((t) => ({ value: t.sreGroupId, label: t.name })),
    [teams],
  );
  // Names for group ids that arrive from a saved view, a bookmark or a
  // dashboard tile before the picker has searched (those links were built from
  // the SRE teams' groups, so the registry knows their names).
  const groupKnownLabels = useMemo(
    () => Object.fromEntries(sreTeamOptions.map((o) => [o.value, o.label])),
    [sreTeamOptions],
  );

  const stateOptions = useMemo(
    () => INCIDENT_STATES.map((st) => ({ value: st, label: incidentStateLabel(st) })),
    [],
  );
  const categoryOptions = useMemo(
    () =>
      INCIDENT_FILTER_CATEGORIES.map((c) => ({
        value: c,
        label: c
          .toLowerCase()
          .split("_")
          .map((w) => w.charAt(0).toUpperCase() + w.slice(1))
          .join(" "),
      })),
    [],
  );

  // Less-used filters live under Advanced (Field / Operator / Value rows, as on
  // the service request list), so the quick grid stays short.
  const advancedFields = useMemo<OperationsAdvancedField<IncidentFilters>[]>(
    () => [
      multiAdvancedField<IncidentFilters>({
        key: "category",
        label: "Category",
        prop: "categories",
        render: ({ id, values, onChange: set }) => (
          <MultiSelectField id={id} label="Value(s)" values={values} options={categoryOptions} onChange={set} />
        ),
      }),
      multiAdvancedField<IncidentFilters>({
        key: "assignee",
        label: "Assignee",
        prop: "assigneeIds",
        render: ({ id, values, onChange: set }) => (
          <AsyncAssignedUserIdMultiSelect id={id} values={values} onChange={set} />
        ),
      }),
      dateAdvancedField<IncidentFilters>({
        key: "updated",
        label: "Updated",
        from: "updatedStartDate",
        to: "updatedEndDate",
      }),
    ],
    [categoryOptions],
  );
  const advancedCount = countAdvancedRows(advancedFields, filters);
  const [mode, setMode] = useState<"quick" | "advanced">("quick");

  const handlePriorityChange = (next: BeIncidentPriority[]): void => {
    onChange({ ...filters, priorities: next });
  };

  /**
   * `minDate`/`maxDate` only constrain the calendar popup — MUI's DatePicker
   * still fires `onChange` for an out-of-range value typed directly into the
   * field, so each handler re-checks the same bound here before accepting it,
   * rather than trusting the picker's UI-only validation.
   */
  const handleCreatedStartChange = (date: unknown): void => {
    if (!(date instanceof Date) || Number.isNaN(date.getTime())) {
      onChange({ ...filters, createdStartDate: "" });
      return;
    }
    if (date > fromMaxDate) return;
    onChange({ ...filters, createdStartDate: formatDateOnly(date) });
  };

  const handleCreatedEndChange = (date: unknown): void => {
    if (!(date instanceof Date) || Number.isNaN(date.getTime())) {
      onChange({ ...filters, createdEndDate: "" });
      return;
    }
    if (date > today) return;
    if (createdStartDate && date < createdStartDate) return;
    onChange({ ...filters, createdEndDate: formatDateOnly(date) });
  };

  return (
    <Paper sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 1.5 }}>
      <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, flexWrap: "wrap" }}>
        <Box sx={{ position: "relative", flex: 1, minWidth: 240 }}>
          <TextField
            fullWidth
            size="small"
            placeholder="Search by number or subject…"
            value={filters.search}
            onChange={(e) => onChange({ ...filters, search: e.target.value })}
            slotProps={{
              input: {
                startAdornment: (
                  <InputAdornment position="start">
                    <Search size={16} />
                  </InputAdornment>
                ),
                endAdornment: filters.search ? (
                  <InputAdornment position="end">
                    <IconButton
                      size="small"
                      edge="end"
                      onClick={() => onChange({ ...filters, search: "" })}
                      aria-label="Clear search"
                    >
                      <X size={16} />
                    </IconButton>
                  </InputAdornment>
                ) : undefined,
              },
            }}
          />
        </Box>

        <SavedViewsMenu
          currentQs={writeIncidentFiltersToUrl(filters).toString()}
          canonicalizeQs={(qs) =>
            writeIncidentFiltersToUrl(readIncidentFiltersFromUrl(new URLSearchParams(qs))).toString()
          }
          activeCount={activeCount}
          hasSearch={filters.search.trim().length > 0}
          onApply={(qs) => onChange(readIncidentFiltersFromUrl(new URLSearchParams(qs)))}
          listKey="incidents"
        />

        <Button
          variant="outlined"
          size="small"
          color="primary"
          onClick={onFiltersToggle}
          startIcon={<ListFilter size={16} />}
          endIcon={isFiltersOpen ? <ChevronUp size={16} /> : <ChevronDown size={16} />}
        >
          {hasActive ? `Filters (${activeCount})` : "Filters"}
        </Button>
        {hasActive && (
          <Button
            variant="text"
            size="small"
            color="primary"
            onClick={onReset}
            startIcon={<X size={16} />}
          >
            Clear filters
          </Button>
        )}
      </Box>

      {isFiltersOpen && (
        <>
          <Divider />
          <Box role="group" aria-label="Filter mode" sx={{ display: "flex", alignSelf: "flex-start" }}>
            <Button
              size="small"
              variant={mode === "quick" ? "contained" : "outlined"}
              color={mode === "quick" ? "primary" : "inherit"}
              aria-pressed={mode === "quick"}
              onClick={() => setMode("quick")}
              sx={{ borderTopRightRadius: 0, borderBottomRightRadius: 0 }}
            >
              Quick filters
            </Button>
            <Button
              size="small"
              variant={mode === "advanced" ? "contained" : "outlined"}
              color={mode === "advanced" ? "primary" : "inherit"}
              aria-pressed={mode === "advanced"}
              onClick={() => setMode("advanced")}
              sx={{ borderTopLeftRadius: 0, borderBottomLeftRadius: 0, ml: "-1px" }}
            >
              {advancedCount > 0 ? `Advanced (${advancedCount})` : "Advanced"}
            </Button>
          </Box>
          {mode === "advanced" ? (
            <OperationsAdvancedFilters
              idPrefix="incident"
              fields={advancedFields}
              filters={filters}
              onChange={onChange}
            />
          ) : (
          <Grid container spacing={2}>
            <Grid size={{ xs: 12, sm: 6, md: 4 }}>
              <MultiSelectField
                id="incident-filter-state"
                label="State"
                values={filters.states}
                options={stateOptions}
                onChange={(next) => onChange({ ...filters, states: next })}
              />
            </Grid>
            <Grid size={{ xs: 12, sm: 6, md: 4 }}>
              <MultiSelectField
                id="incident-filter-priority"
                label="Priority"
                values={filters.priorities}
                options={priorityOptions}
                onChange={handlePriorityChange}
              />
            </Grid>

            <Grid size={{ xs: 12, sm: 6, md: 4 }}>
              <AsyncEntityMultiSelect
                id="incident-filter-service"
                label="Service"
                placeholder="Search services…"
                values={filters.serviceIds}
                onChange={(next) => onChange({ ...filters, serviceIds: next })}
                useSearch={useSearchItServices}
                getId={(svc) => svc.id}
                getLabel={(svc) => svc.name ?? svc.id}
                compactSelectedValues
              />
            </Grid>

            <Grid size={{ xs: 12, sm: 6, md: 4 }}>
              <IncidentProductMultiSelect
                values={filters.products}
                onChange={(next) => onChange({ ...filters, products: next })}
              />
            </Grid>

            <Grid size={{ xs: 12, sm: 6, md: 4 }}>
              <AsyncEntityMultiSelect
                id="incident-filter-assignment-group"
                label="Assignment group"
                placeholder="Search groups…"
                values={filters.sreTeamIds}
                onChange={(next) => onChange({ ...filters, sreTeamIds: next })}
                useSearch={useSearchGroups}
                getId={(g) => g.id}
                getLabel={(g) => g.name}
                knownLabels={groupKnownLabels}
                compactSelectedValues
              />
            </Grid>

            <Grid
              size={{ xs: 12, sm: 6, md: 4 }}
              sx={{ display: "flex", alignItems: "center", height: 40 }}
            >
              <FormControlLabel
                control={
                  <Checkbox
                    id="incident-filter-sla-violated"
                    size="small"
                    checked={filters.slaViolated}
                    onChange={(e) => onChange({ ...filters, slaViolated: e.target.checked })}
                  />
                }
                label="SLA violated"
              />
            </Grid>

            <Grid size={{ xs: 12, sm: 6, md: 3 }}>
              <LocalizationProvider dateAdapter={AdapterDateFns}>
                <DatePicker
                  label="Created from (UTC)"
                  value={createdStartDate}
                  maxDate={fromMaxDate}
                  onChange={handleCreatedStartChange}
                  slotProps={{
                    textField: { size: "small", fullWidth: true },
                    field: { clearable: true },
                  }}
                />
              </LocalizationProvider>
            </Grid>

            <Grid size={{ xs: 12, sm: 6, md: 3 }}>
              <LocalizationProvider dateAdapter={AdapterDateFns}>
                <DatePicker
                  label="Created to (UTC)"
                  value={createdEndDate}
                  minDate={createdStartDate ?? undefined}
                  maxDate={today}
                  onChange={handleCreatedEndChange}
                  slotProps={{
                    textField: { size: "small", fullWidth: true },
                    field: { clearable: true },
                  }}
                />
              </LocalizationProvider>
            </Grid>
          </Grid>
          )}
          {activeCount > 0 && (
            <Box sx={{ display: "flex", justifyContent: "flex-end" }}>
              <Typography variant="caption" color="text.secondary">
                {activeCount} {activeCount === 1 ? "filter" : "filters"} active
              </Typography>
            </Box>
          )}
        </>
      )}
    </Paper>
  );
}
