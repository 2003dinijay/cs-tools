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
  DatePickers,
  Divider,
  Grid,
  IconButton,
  InputAdornment,
  Paper,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import {
  ChevronDown,
  ChevronUp,
  ListFilter,
  Search,
  X,
} from "@wso2/oxygen-ui-icons-react";
import { useMemo, type JSX } from "react";
import { useTeams } from "@features/csm-dashboard/api/useTeams";
import {
  CHANGE_REQUEST_FILTER_TYPE_OPTIONS,
  CHANGE_REQUEST_IMPACTS,
  CHANGE_REQUEST_STATES,
  changeRequestImpactLabel,
  changeRequestStateLabel,
  countActiveCRFilters,
  type ChangeRequestFilters,
} from "@features/csm-operations/utils/changeRequests";
import {
  readChangeRequestFiltersFromUrl,
  writeChangeRequestFiltersToUrl,
} from "@features/csm-operations/utils/changeRequestsFiltersUrl";
import SavedViewsMenu from "@features/csm-operations/components/SavedViewsMenu";
import MultiSelectField from "@components/MultiSelectField";
import { useSearchGroups } from "@api/useSearchGroups";
import AsyncEntityMultiSelect from "@components/AsyncEntityMultiSelect";
import AsyncTagMultiSelect from "@features/csm-cases/components/AsyncTagMultiSelect";
import AsyncAssignedUserIdMultiSelect from "@features/csm-cases/components/AsyncAssignedUserIdMultiSelect";
import DateRangeFilterFields from "@features/csm-operations/components/DateRangeFilterFields";
import AsyncProjectMultiSelect from "@features/csm-cases/components/AsyncProjectMultiSelect";

const { DatePicker, LocalizationProvider } = DatePickers;

/** "YYYY-MM-DD" to a local-midnight Date (avoids the UTC-parse day-shift
 * `new Date(dateString)` can cause depending on the viewer's timezone). */
function parseDateOnly(value: string): Date | null {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value);
  if (!match) return null;
  const date = new Date(Number(match[1]), Number(match[2]) - 1, Number(match[3]));
  return Number.isNaN(date.getTime()) ? null : date;
}

/** Local-midnight Date back to "YYYY-MM-DD", matching ChangeRequestFilters'
 * closedStartDate/closedEndDate wire format. */
function formatDateOnly(date: Date): string {
  const y = date.getFullYear();
  const m = String(date.getMonth() + 1).padStart(2, "0");
  const d = String(date.getDate()).padStart(2, "0");
  return `${y}-${m}-${d}`;
}

/** Today's UTC calendar date, as a local-midnight Date (see `parseDateOnly`)
 * — the upper bound for both closed-date fields (`crDateOnlyToISOStart`/`End`
 * send them to the backend as UTC day boundaries, same as
 * `IncidentsFilterBar`'s created-date range), since a case can't close in
 * the future. */
function todayUTCDateOnly(): Date {
  const now = new Date();
  return new Date(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate());
}

interface ChangeRequestsFilterBarProps {
  filters: ChangeRequestFilters;
  onChange: (next: ChangeRequestFilters) => void;
  onReset: () => void;
  isFiltersOpen: boolean;
  onFiltersToggle: () => void;
}


export default function ChangeRequestsFilterBar({
  filters,
  onChange,
  onReset,
  isFiltersOpen,
  onFiltersToggle,
}: ChangeRequestsFilterBarProps): JSX.Element {
  const activeCount = countActiveCRFilters(filters);
  const hasActive = activeCount > 0;

  // Recomputed every render (not memoized) — a `useMemo(..., [])` would
  // freeze this at the component's mount date and stop matching "today" for
  // any session left open across a UTC midnight.
  const today = todayUTCDateOnly();
  const closedStartDate = parseDateOnly(filters.closedStartDate);
  const closedEndDate = parseDateOnly(filters.closedEndDate);
  const fromMaxDate = closedEndDate && closedEndDate < today ? closedEndDate : today;

  const stateOptions = useMemo(
    () =>
      CHANGE_REQUEST_STATES.map((s) => ({
        value: s,
        label: changeRequestStateLabel(s),
      })),
    [],
  );
  const impactOptions = useMemo(
    () =>
      CHANGE_REQUEST_IMPACTS.map((i) => ({
        value: i,
        label: changeRequestImpactLabel(i),
      })),
    [],
  );

  // The Assignment group filter searches every group; the SRE teams' groups
  // (registry `sre-abt` family) only supply names for ids carried in from
  // links built before. The old "SRE Team" control was scoped to the
  // `sre-abt` team family — see `IncidentsFilterBar`'s equivalent note on
  // why this isn't `cre-abt` (the cases list's own "SRE Team" control's
  // family scoping).
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

  /**
   * `minDate`/`maxDate` only constrain the calendar popup — MUI's DatePicker
   * still fires `onChange` for an out-of-range value typed directly into the
   * field, so each handler re-checks the same bound here before accepting it,
   * rather than trusting the picker's UI-only validation.
   */
  const handleClosedStartChange = (date: unknown): void => {
    if (!(date instanceof Date) || Number.isNaN(date.getTime())) {
      onChange({ ...filters, closedStartDate: "" });
      return;
    }
    if (date > fromMaxDate) return;
    onChange({ ...filters, closedStartDate: formatDateOnly(date) });
  };

  const handleClosedEndChange = (date: unknown): void => {
    if (!(date instanceof Date) || Number.isNaN(date.getTime())) {
      onChange({ ...filters, closedEndDate: "" });
      return;
    }
    if (date > today) return;
    if (closedStartDate && date < closedStartDate) return;
    onChange({ ...filters, closedEndDate: formatDateOnly(date) });
  };

  return (
    <Paper sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 1.5 }}>
      {/* Search bar + filters toggle */}
      <Box
        sx={{ display: "flex", alignItems: "center", gap: 1.5, flexWrap: "wrap" }}
      >
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
          currentQs={writeChangeRequestFiltersToUrl(filters).toString()}
          canonicalizeQs={(qs) =>
            writeChangeRequestFiltersToUrl(
              readChangeRequestFiltersFromUrl(new URLSearchParams(qs)),
            ).toString()
          }
          activeCount={activeCount}
          hasSearch={filters.search.trim().length > 0}
          onApply={(qs) => onChange(readChangeRequestFiltersFromUrl(new URLSearchParams(qs)))}
          listKey="change_requests"
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

      {/* Collapsible filter grid */}
      {isFiltersOpen && (
        <>
          <Divider />
          <Grid container spacing={2}>
            <Grid size={{ xs: 12, sm: 6, md: 3 }}>
              <MultiSelectField
                id="cr-filter-state"
                label="State"
                values={filters.states}
                options={stateOptions}
                onChange={(next) => onChange({ ...filters, states: next })}
              />
            </Grid>
            <Grid size={{ xs: 12, sm: 6, md: 3 }}>
              <MultiSelectField
                id="cr-filter-impact"
                label="Impact"
                values={filters.impacts}
                options={impactOptions}
                onChange={(next) => onChange({ ...filters, impacts: next })}
              />
            </Grid>
            <Grid size={{ xs: 12, sm: 6, md: 3 }}>
              <AsyncEntityMultiSelect
                id="cr-filter-assignment-group"
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
            <Grid size={{ xs: 12, sm: 6, md: 3 }}>
              <AsyncProjectMultiSelect
                id="cr-filter-project"
                values={filters.projectIds}
                onChange={(next) => onChange({ ...filters, projectIds: next })}
              />
            </Grid>
            <Grid size={{ xs: 12, sm: 6, md: 3 }}>
              <AsyncAssignedUserIdMultiSelect
                id="cr-filter-assignee"
                label="Assigned engineer"
                values={filters.assigneeIds}
                onChange={(next) => onChange({ ...filters, assigneeIds: next })}
              />
            </Grid>
            <Grid size={{ xs: 12, sm: 6, md: 3 }}>
              <MultiSelectField
                id="cr-filter-type"
                label="Type"
                values={filters.types}
                options={CHANGE_REQUEST_FILTER_TYPE_OPTIONS}
                onChange={(next) => onChange({ ...filters, types: next })}
              />
            </Grid>
            <Grid size={{ xs: 12, sm: 6, md: 3 }}>
              <AsyncTagMultiSelect
                id="cr-filter-tag"
                label="Tags"
                values={filters.tags}
                onChange={(next) => onChange({ ...filters, tags: next })}
              />
            </Grid>
            <Grid size={{ xs: 12, sm: 6, md: 3 }}>
              <LocalizationProvider dateAdapter={AdapterDateFns}>
                <DatePicker
                  label="Closed from"
                  value={closedStartDate}
                  maxDate={fromMaxDate}
                  onChange={handleClosedStartChange}
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
                  label="Closed to"
                  value={closedEndDate}
                  minDate={closedStartDate ?? undefined}
                  maxDate={today}
                  onChange={handleClosedEndChange}
                  slotProps={{
                    textField: { size: "small", fullWidth: true },
                    field: { clearable: true },
                  }}
                />
              </LocalizationProvider>
            </Grid>
            <DateRangeFilterFields
              label="Created"
              value={{ from: filters.createdStartDate, to: filters.createdEndDate }}
              onChange={(r) => onChange({ ...filters, createdStartDate: r.from, createdEndDate: r.to })}
            />
            <DateRangeFilterFields
              label="Updated"
              value={{ from: filters.updatedStartDate, to: filters.updatedEndDate }}
              onChange={(r) => onChange({ ...filters, updatedStartDate: r.from, updatedEndDate: r.to })}
            />
            <DateRangeFilterFields
              label="Planned start"
              capAtToday={false}
              value={{ from: filters.plannedStartFrom, to: filters.plannedStartTo }}
              onChange={(r) => onChange({ ...filters, plannedStartFrom: r.from, plannedStartTo: r.to })}
            />
            <DateRangeFilterFields
              label="Planned end"
              capAtToday={false}
              value={{ from: filters.plannedEndFrom, to: filters.plannedEndTo }}
              onChange={(r) => onChange({ ...filters, plannedEndFrom: r.from, plannedEndTo: r.to })}
            />
          </Grid>
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
