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
  IconButton,
  InputAdornment,
  Skeleton,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TablePagination,
  TableRow,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { Search, X } from "@wso2/oxygen-ui-icons-react";
import { useState, type ChangeEvent, type JSX } from "react";
import QueryErrorState from "@components/QueryErrorState";
import StateChip from "@components/StateChip";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import { formatBackendTimestampForDisplay } from "@utils/dateTime";
import { useSearchAnnouncements } from "@features/csm-announcements/api/useSearchAnnouncements";

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
 * Read-only announcements list. Announcements are cases of
 * `type: "announcement"` surfaced via `POST /cases/search`. Creating /
 * targeting / unpublishing needs the dedicated announcement backend
 * (digiops-cs#2053), which isn't built yet, so this page is view-only for now.
 */
export default function CsmAnnouncementsPage(): JSX.Element {
  const [search, setSearch] = useState("");
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(DEFAULT_ROWS_PER_PAGE);
  const debouncedSearch = useDebouncedValue(search.trim(), 300);

  const { data, isLoading, isFetching, isError, error } = useSearchAnnouncements(
    debouncedSearch,
    page,
    rowsPerPage,
  );

  const announcements = data?.announcements ?? [];
  const total = data?.total ?? 0;

  const handleSearchChange = (e: ChangeEvent<HTMLInputElement>): void => {
    setSearch(e.target.value);
    setPage(0);
  };

  const handleChangeRowsPerPage = (e: ChangeEvent<HTMLInputElement>): void => {
    setRowsPerPage(parseInt(e.target.value, 10));
    setPage(0);
  };

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <Box>
        <Typography variant="h5">Announcements</Typography>
        <Typography variant="body2" color="text.secondary">
          Customer-facing announcements published across projects and tiers.
        </Typography>
      </Box>

      <Box sx={{ maxWidth: 360 }}>
        <TextField
          fullWidth
          size="small"
          placeholder="Search by subject or number…"
          value={search}
          onChange={handleSearchChange}
          slotProps={{
            input: {
              startAdornment: (
                <InputAdornment position="start">
                  <Search size={16} />
                </InputAdornment>
              ),
              endAdornment: search ? (
                <InputAdornment position="end">
                  <IconButton
                    size="small"
                    edge="end"
                    onClick={() => {
                      setSearch("");
                      setPage(0);
                    }}
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

      <Box sx={{ border: 1, borderColor: "divider", borderRadius: 1, overflow: "hidden" }}>
        <TableContainer>
          <Table size="small" sx={{ "& .MuiTableCell-root": { borderColor: "divider" } }}>
            <TableHead>
              <TableRow sx={{ bgcolor: "action.hover" }}>
                <TableCell>Number</TableCell>
                <TableCell>Subject</TableCell>
                <TableCell>Project</TableCell>
                <TableCell>State</TableCell>
                <TableCell>Created by</TableCell>
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
                    <TableCell><Skeleton variant="rounded" width="70%" height={18} /></TableCell>
                    <TableCell><Skeleton variant="rounded" width={80} height={18} /></TableCell>
                  </TableRow>
                ))
              ) : isError ? (
                <TableRow>
                  <TableCell colSpan={6} align="center">
                    <QueryErrorState
                      message={`Failed to load announcements: ${error instanceof Error ? error.message : "unknown error"}`}
                      error={error}
                    />
                  </TableCell>
                </TableRow>
              ) : announcements.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={6} align="center" sx={{ py: 4 }}>
                    <Typography variant="body2" color="text.secondary">
                      No announcements found.
                    </Typography>
                  </TableCell>
                </TableRow>
              ) : (
                announcements.map((a) => (
                  <TableRow key={a.id} hover>
                    <TableCell>{a.number || "—"}</TableCell>
                    <TableCell>{a.subject}</TableCell>
                    <TableCell>{a.projectName}</TableCell>
                    <TableCell>{a.state ? <StateChip state={a.state} /> : "—"}</TableCell>
                    <TableCell>{a.createdBy || "—"}</TableCell>
                    <TableCell>{formatDate(a.updatedAt)}</TableCell>
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
          onPageChange={(_, newPage) => setPage(newPage)}
          rowsPerPage={rowsPerPage}
          onRowsPerPageChange={handleChangeRowsPerPage}
          rowsPerPageOptions={ROWS_PER_PAGE_OPTIONS}
        />
      </Box>
    </Box>
  );
}
