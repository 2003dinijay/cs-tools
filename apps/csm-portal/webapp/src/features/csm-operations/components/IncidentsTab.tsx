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
  Box,
  Button,
  Chip,
  Skeleton,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TablePagination,
  TableRow,
  Typography,
} from "@wso2/oxygen-ui";
import { Plus } from "@wso2/oxygen-ui-icons-react";
import { useCallback, useMemo, useState, type ChangeEvent, type JSX } from "react";
import { useNavTransition } from "@hooks/useNavTransition";
import QueryErrorState from "@components/QueryErrorState";
import FilteredCsvExportButton from "@components/FilteredCsvExportButton";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import { useBackendApi } from "@api/backend/client";
import { formatBackendTimestampForDisplay } from "@utils/dateTime";
import { useSearchIncidents } from "@features/csm-operations/api/useSearchIncidents";
import {
  DEFAULT_INCIDENT_FILTERS,
  incidentPriorityColor,
  incidentPriorityLabel,
  incidentStateColor,
  incidentStateLabel,
  type IncidentFilters,
} from "@features/csm-operations/utils/incidents";
import IncidentsFilterBar from "@features/csm-operations/components/IncidentsFilterBar";
import type { BeIncident, BeIncidentSearchPayload, BeIncidentSearchResponse } from "@api/backend/types";

const DEFAULT_ROWS_PER_PAGE = 20;
const ROWS_PER_PAGE_OPTIONS = [10, 20, 50];

function formatDate(value?: string | null): string {
  return (
    formatBackendTimestampForDisplay(value, {
      year: "numeric",
      month: "short",
      day: "numeric",
    }) ?? "—"
  );
}

/**
 * Incidents listing for the Operations → Incidents tab. Searches
 * `POST /incidents/search` with server-side pagination, free-text search,
 * and a priority filter (the only filter field the backend supports beyond
 * search — see `IncidentsFilterBar`).
 */
export default function IncidentsTab(): JSX.Element {
  const navigate = useNavTransition();
  const api = useBackendApi();
  const [filters, setFilters] = useState<IncidentFilters>(DEFAULT_INCIDENT_FILTERS);
  const [isFiltersOpen, setIsFiltersOpen] = useState(false);
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(DEFAULT_ROWS_PER_PAGE);
  const debouncedSearch = useDebouncedValue(filters.search.trim(), 300);

  const payload = useMemo(
    () => ({
      filters: {
        ...(debouncedSearch.length > 0 && { searchQuery: debouncedSearch }),
        ...(filters.priorities.length > 0 && { priorities: filters.priorities }),
      },
      sortBy: { field: "createdOn" as const, order: "desc" as const },
      pagination: { offset: page * rowsPerPage, limit: rowsPerPage },
    }),
    [debouncedSearch, filters.priorities, page, rowsPerPage],
  );

  const { data, isLoading, isError, error, isFetching } = useSearchIncidents(payload);

  const incidents = data?.incidents ?? [];
  const total = data?.total ?? 0;

  const handleFiltersChange = (next: IncidentFilters): void => {
    setFilters(next);
    setPage(0);
  };

  const handleReset = (): void => {
    setFilters(DEFAULT_INCIDENT_FILTERS);
    setPage(0);
  };

  const handleChangeRowsPerPage = (e: ChangeEvent<HTMLInputElement>): void => {
    setRowsPerPage(parseInt(e.target.value, 10));
    setPage(0);
  };

  // Pages `/incidents/search` with the *currently applied* filters/sort
  // (same `filters`/`sortBy` as `payload` above, just re-built per page with
  // its own offset/limit instead of the table's) until the full filtered
  // result set has been fetched — see `useFilteredCsvExport`/`fetchAllPages`.
  // Bound fresh on every render via the hook's ref pattern, so a filter
  // change is picked up even mid-typing without this identity needing to be
  // stable.
  const fetchIncidentsPage = useCallback(
    async (offset: number, limit: number) => {
      const res = await api.post<BeIncidentSearchPayload, BeIncidentSearchResponse>(
        "/incidents/search",
        {
          filters: payload.filters,
          sortBy: payload.sortBy,
          pagination: { offset, limit },
        },
      );
      return { items: res.incidents, total: res.total };
    },
    [api, payload.filters, payload.sortBy],
  );

  const incidentToCsvRow = useCallback(
    (incident: BeIncident): string[] => [
      incident.number ?? "",
      incident.subject ?? "",
      incident.caller?.name ?? "",
      incidentStateLabel(incident.state),
      incidentPriorityLabel(incident.priority),
      formatDate(incident.openedOn),
      formatDate(incident.updatedOn),
    ],
    [],
  );

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      <Box sx={{ display: "flex", justifyContent: "flex-end", gap: 1 }}>
        <FilteredCsvExportButton<BeIncident>
          entityName="incidents"
          header={["Number", "Subject", "Caller", "State", "Priority", "Opened", "Updated"]}
          toRow={incidentToCsvRow}
          fetchPage={fetchIncidentsPage}
          disabled={isError}
        />
        <Button
          variant="contained"
          color="primary"
          size="small"
          startIcon={<Plus size={16} />}
          onClick={() => navigate("/operations/incidents/new")}
        >
          Create incident
        </Button>
      </Box>

      <IncidentsFilterBar
        filters={filters}
        onChange={handleFiltersChange}
        onReset={handleReset}
        isFiltersOpen={isFiltersOpen}
        onFiltersToggle={() => setIsFiltersOpen((prev) => !prev)}
      />

      <Box sx={{ border: 1, borderColor: "divider", borderRadius: 1, overflow: "hidden" }}>
        <TableContainer>
          <Table size="small" sx={{ "& .MuiTableCell-root": { borderColor: "divider" } }}>
            <TableHead>
              <TableRow sx={{ bgcolor: "action.hover" }}>
                <TableCell>Number</TableCell>
                <TableCell>Subject</TableCell>
                <TableCell>Caller</TableCell>
                <TableCell>State</TableCell>
                <TableCell>Priority</TableCell>
                <TableCell>Opened</TableCell>
                <TableCell>Updated</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {isLoading || isFetching ? (
                Array.from({ length: rowsPerPage }).map((_, i) => (
                  <TableRow key={i}>
                    <TableCell><Skeleton variant="rounded" width="80%" height={18} /></TableCell>
                    <TableCell><Skeleton variant="rounded" width="90%" height={18} /></TableCell>
                    <TableCell><Skeleton variant="rounded" width="85%" height={18} /></TableCell>
                    <TableCell><Skeleton variant="rounded" width={72} height={22} /></TableCell>
                    <TableCell><Skeleton variant="rounded" width={60} height={22} /></TableCell>
                    <TableCell><Skeleton variant="rounded" width={80} height={18} /></TableCell>
                    <TableCell><Skeleton variant="rounded" width={80} height={18} /></TableCell>
                  </TableRow>
                ))
              ) : isError ? (
                <TableRow>
                  <TableCell colSpan={7} align="center">
                    <QueryErrorState
                      message={`Failed to load incidents: ${error instanceof Error ? error.message : "unknown error"}`}
                      error={error}
                    />
                  </TableCell>
                </TableRow>
              ) : incidents.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={7} align="center" sx={{ py: 4 }}>
                    <Typography variant="body2" color="text.secondary">
                      No incidents found.
                    </Typography>
                  </TableCell>
                </TableRow>
              ) : (
                incidents.map((incident, index) => (
                  <TableRow
                    // incident.id is nullable — fall back to the index so
                    // multiple incidents with a null id (an edge case the
                    // type allows) still get distinct React keys.
                    key={incident.id ?? `incident-${index}`}
                    hover
                    onClick={() => incident.id && navigate(`/operations/incidents/${incident.id}`)}
                    sx={{ cursor: "pointer" }}
                  >
                    <TableCell>{incident.number || "—"}</TableCell>
                    <TableCell sx={{ maxWidth: 360 }}>
                      <Typography variant="body2" noWrap title={incident.subject ?? undefined}>
                        {incident.subject || "—"}
                      </Typography>
                    </TableCell>
                    <TableCell>{incident.caller?.name || "—"}</TableCell>
                    <TableCell>
                      {incident.state ? (
                        <Chip
                          size="small"
                          variant="outlined"
                          color={incidentStateColor(incident.state)}
                          label={incidentStateLabel(incident.state)}
                        />
                      ) : (
                        "—"
                      )}
                    </TableCell>
                    <TableCell>
                      {incident.priority ? (
                        <Chip
                          size="small"
                          variant="outlined"
                          color={incidentPriorityColor(incident.priority)}
                          label={incidentPriorityLabel(incident.priority)}
                        />
                      ) : (
                        "—"
                      )}
                    </TableCell>
                    <TableCell>{formatDate(incident.openedOn)}</TableCell>
                    <TableCell>{formatDate(incident.updatedOn)}</TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </TableContainer>
        <TablePagination
          component="div"
          count={total}
          page={page}
          onPageChange={(_: unknown, newPage: number) => setPage(newPage)}
          rowsPerPage={rowsPerPage}
          onRowsPerPageChange={handleChangeRowsPerPage}
          rowsPerPageOptions={ROWS_PER_PAGE_OPTIONS}
          labelDisplayedRows={({ from, to, count }) => `Showing ${from}–${to} of ${count}`}
          showFirstButton
          showLastButton
        />
      </Box>
    </Box>
  );
}
