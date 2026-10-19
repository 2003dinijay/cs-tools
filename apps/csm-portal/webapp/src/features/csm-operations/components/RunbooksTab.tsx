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
  Chip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  IconButton,
  InputAdornment,
  Link,
  Paper,
  Skeleton,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TablePagination,
  TableRow,
  TextField,
  ToggleButton,
  ToggleButtonGroup,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import { ExternalLink, Pencil, Plus, Search, Trash2, Upload, X } from "@wso2/oxygen-ui-icons-react";
import { useMemo, useRef, useState, type ChangeEvent, type JSX } from "react";
import QueryErrorState from "@components/QueryErrorState";
import { usePortalAccess } from "@context/current-user/usePortalAccess";
import {
  useCreateRunbook,
  useDeleteRunbook,
  useImportRunbooks,
  useRunbooks,
  useUpdateRunbook,
} from "@features/csm-operations/api/useRunbooks";
import {
  findRunbookForIncident,
  isValidRunbookUrl,
  parseRunbookImport,
  runbookWorkNote,
  type Runbook,
  type RunbookCoverage,
} from "@features/csm-operations/utils/runbooks";

const DEFAULT_ROWS_PER_PAGE = 20;
const ROWS_PER_PAGE_OPTIONS = [10, 20, 50];

interface RunbookDialogState {
  open: boolean;
  editing?: Runbook;
}

function errorText(err: unknown, fallback: string): string {
  return err instanceof Error && err.message.trim() ? err.message : fallback;
}

/**
 * Operations -> Runbooks: the alert-name to runbook catalog. entity-service adds
 * a work note to every new incident whose subject contains an alert name listed
 * here: the runbook link, or that none is available.
 */
export default function RunbooksTab(): JSX.Element {
  const { canWrite } = usePortalAccess();
  const { data: runbooks = [], isLoading, isError, error, refetch } = useRunbooks();
  const deleteRunbook = useDeleteRunbook();
  const importRunbooks = useImportRunbooks();
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [search, setSearch] = useState("");
  const [coverage, setCoverage] = useState<RunbookCoverage>("all");
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(DEFAULT_ROWS_PER_PAGE);
  const [dialog, setDialog] = useState<RunbookDialogState>({ open: false });
  const [pendingDelete, setPendingDelete] = useState<Runbook | null>(null);
  const [notice, setNotice] = useState<{ severity: "success" | "error"; text: string } | null>(null);

  const linkedCount = runbooks.filter((r) => r.runbookUrl).length;

  const filtered = useMemo(() => {
    const term = search.trim().toLowerCase();
    return runbooks.filter((r) => {
      if (coverage === "linked" && !r.runbookUrl) return false;
      if (coverage === "missing" && r.runbookUrl) return false;
      return !term || r.alertName.toLowerCase().includes(term) || (r.runbookUrl ?? "").toLowerCase().includes(term);
    });
  }, [runbooks, search, coverage]);

  const pageRows = filtered.slice(page * rowsPerPage, page * rowsPerPage + rowsPerPage);

  const handleDelete = (): void => {
    if (!pendingDelete) return;
    deleteRunbook.mutate(pendingDelete.id, {
      onSuccess: () => setPendingDelete(null),
    });
  };

  const handleImportFile = async (e: ChangeEvent<HTMLInputElement>): Promise<void> => {
    const file = e.target.files?.[0];
    e.target.value = "";
    if (!file) return;
    try {
      const entries = parseRunbookImport(await file.text());
      if (entries.length === 0) {
        setNotice({ severity: "error", text: "The file has no runbooks." });
        return;
      }
      importRunbooks.mutate(entries, {
        onSuccess: (res) =>
          setNotice({ severity: "success", text: `Imported ${res.created} new and updated ${res.updated} existing runbooks.` }),
        onError: (err) => setNotice({ severity: "error", text: errorText(err, "Failed to import runbooks.") }),
      });
    } catch (err) {
      setNotice({ severity: "error", text: errorText(err, "The file could not be read.") });
    }
  };

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      <Box sx={{ display: "flex", justifyContent: "space-between", alignItems: "center", gap: 1, flexWrap: "wrap" }}>
        <Typography variant="body2" color="text.secondary">
          {isLoading ? "Loading runbooks…" : `${runbooks.length} alerts, ${linkedCount} with a runbook, ${runbooks.length - linkedCount} without`}
        </Typography>
        {canWrite && (
          <Box sx={{ display: "flex", gap: 1 }}>
            <input
              ref={fileInputRef}
              type="file"
              accept="application/json,.json"
              hidden
              onChange={(e) => void handleImportFile(e)}
              data-testid="runbook-import-input"
            />
            <Button
              variant="outlined"
              size="small"
              startIcon={<Upload size={16} />}
              disabled={importRunbooks.isPending}
              onClick={() => fileInputRef.current?.click()}
            >
              {importRunbooks.isPending ? "Importing…" : "Import JSON"}
            </Button>
            <Button
              variant="contained"
              color="primary"
              size="small"
              startIcon={<Plus size={16} />}
              onClick={() => setDialog({ open: true })}
            >
              Add runbook
            </Button>
          </Box>
        )}
      </Box>

      {notice && (
        <Alert severity={notice.severity} onClose={() => setNotice(null)}>
          {notice.text}
        </Alert>
      )}

      <Paper sx={{ p: 2.5, display: "flex", alignItems: "center", gap: 1.5, flexWrap: "wrap" }}>
        <TextField
          size="small"
          placeholder="Search by alert name or runbook link…"
          value={search}
          onChange={(e) => {
            setSearch(e.target.value);
            setPage(0);
          }}
          sx={{ flex: 1, minWidth: 240 }}
          slotProps={{
            input: {
              startAdornment: (
                <InputAdornment position="start">
                  <Search size={16} />
                </InputAdornment>
              ),
              endAdornment: search ? (
                <InputAdornment position="end">
                  <IconButton size="small" edge="end" onClick={() => setSearch("")} aria-label="Clear search">
                    <X size={16} />
                  </IconButton>
                </InputAdornment>
              ) : undefined,
            },
          }}
        />
        <ToggleButtonGroup
          size="small"
          exclusive
          value={coverage}
          onChange={(_: unknown, next: RunbookCoverage | null) => {
            if (next) {
              setCoverage(next);
              setPage(0);
            }
          }}
          aria-label="Runbook coverage"
        >
          <ToggleButton value="all">All</ToggleButton>
          <ToggleButton value="linked">With runbook</ToggleButton>
          <ToggleButton value="missing">No runbook</ToggleButton>
        </ToggleButtonGroup>
      </Paper>

      <Box sx={{ border: 1, borderColor: "divider", borderRadius: 1, overflow: "hidden" }}>
        <TableContainer>
          <Table size="small" sx={{ "& .MuiTableCell-root": { borderColor: "divider" } }}>
            <TableHead>
              <TableRow sx={{ bgcolor: "action.hover" }}>
                <TableCell>Alert name</TableCell>
                <TableCell sx={{ width: 220 }}>Runbook</TableCell>
                {canWrite && <TableCell align="right" sx={{ width: 96 }}>Actions</TableCell>}
              </TableRow>
            </TableHead>
            <TableBody>
              {isLoading ? (
                Array.from({ length: rowsPerPage }).map((_, i) => (
                  <TableRow key={i}>
                    <TableCell><Skeleton variant="rounded" width="80%" height={18} /></TableCell>
                    <TableCell><Skeleton variant="rounded" width={110} height={22} /></TableCell>
                    {canWrite && <TableCell><Skeleton variant="rounded" width={56} height={18} /></TableCell>}
                  </TableRow>
                ))
              ) : isError ? (
                <TableRow>
                  <TableCell colSpan={canWrite ? 3 : 2} align="center">
                    <QueryErrorState message={errorText(error, "Failed to load runbooks.")} error={error} onRetry={() => void refetch()} />
                  </TableCell>
                </TableRow>
              ) : pageRows.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={canWrite ? 3 : 2} align="center" sx={{ py: 4 }}>
                    <Typography variant="body2" color="text.secondary">
                      No runbooks found.
                    </Typography>
                  </TableCell>
                </TableRow>
              ) : (
                pageRows.map((rb) => (
                  <TableRow key={rb.id} hover>
                    <TableCell sx={{ wordBreak: "break-word", maxWidth: 720 }}>
                      <Typography variant="body2">{rb.alertName}</Typography>
                    </TableCell>
                    <TableCell>
                      {rb.runbookUrl ? (
                        <Link
                          href={rb.runbookUrl}
                          target="_blank"
                          rel="noopener noreferrer"
                          sx={{ display: "inline-flex", alignItems: "center", gap: 0.5 }}
                        >
                          Open runbook
                          <ExternalLink size={14} />
                        </Link>
                      ) : (
                        <Chip size="small" variant="outlined" color="warning" label="No runbook" />
                      )}
                    </TableCell>
                    {canWrite && (
                      <TableCell align="right">
                        <Tooltip title="Edit">
                          <IconButton size="small" aria-label={`Edit ${rb.alertName}`} onClick={() => setDialog({ open: true, editing: rb })}>
                            <Pencil size={16} />
                          </IconButton>
                        </Tooltip>
                        <Tooltip title="Delete">
                          <IconButton size="small" aria-label={`Delete ${rb.alertName}`} onClick={() => setPendingDelete(rb)}>
                            <Trash2 size={16} />
                          </IconButton>
                        </Tooltip>
                      </TableCell>
                    )}
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </TableContainer>
        <TablePagination
          component="div"
          count={filtered.length}
          page={page}
          onPageChange={(_: unknown, next: number) => setPage(next)}
          rowsPerPage={rowsPerPage}
          onRowsPerPageChange={(e: ChangeEvent<HTMLInputElement>) => {
            setRowsPerPage(parseInt(e.target.value, 10));
            setPage(0);
          }}
          rowsPerPageOptions={ROWS_PER_PAGE_OPTIONS}
          labelDisplayedRows={({ from, to, count }) => `Showing ${from}–${to} of ${count}`}
          showFirstButton
          showLastButton
        />
      </Box>

      <WorkNotePreview runbooks={runbooks} />

      {dialog.open && (
        <RunbookDialog
          editing={dialog.editing}
          existing={runbooks}
          onClose={() => setDialog({ open: false })}
        />
      )}

      <Dialog open={pendingDelete !== null} onClose={() => setPendingDelete(null)} maxWidth="xs" fullWidth>
        <DialogTitle>Delete runbook?</DialogTitle>
        <DialogContent sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
          <Typography variant="body2" sx={{ wordBreak: "break-word" }}>
            New incidents for <strong>{pendingDelete?.alertName}</strong> will no longer get a runbook work note.
          </Typography>
          {deleteRunbook.isError && (
            <Alert severity="error">{errorText(deleteRunbook.error, "Failed to delete the runbook.")}</Alert>
          )}
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setPendingDelete(null)}>Go back</Button>
          <Button color="error" variant="contained" onClick={handleDelete} disabled={deleteRunbook.isPending}>
            Delete
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
}

interface RunbookDialogProps {
  editing?: Runbook;
  existing: Runbook[];
  onClose: () => void;
}

function RunbookDialog({ editing, existing, onClose }: RunbookDialogProps): JSX.Element {
  const createRunbook = useCreateRunbook();
  const updateRunbook = useUpdateRunbook();
  const [alertName, setAlertName] = useState(editing?.alertName ?? "");
  const [runbookUrl, setRunbookUrl] = useState(editing?.runbookUrl ?? "");

  const saving = createRunbook.isPending || updateRunbook.isPending;
  const saveError = createRunbook.error ?? updateRunbook.error;
  const trimmedName = alertName.trim();
  const trimmedUrl = runbookUrl.trim();
  const duplicate = existing.some(
    (r) => r.id !== editing?.id && r.alertName.trim().toLowerCase() === trimmedName.toLowerCase(),
  );
  const urlInvalid = !isValidRunbookUrl(runbookUrl);
  const canSave = trimmedName.length > 0 && !duplicate && !urlInvalid && !saving;

  const handleSave = (): void => {
    const input = { alertName: trimmedName, runbookUrl: trimmedUrl || null };
    if (editing) {
      updateRunbook.mutate({ id: editing.id, input }, { onSuccess: onClose });
    } else {
      createRunbook.mutate(input, { onSuccess: onClose });
    }
  };

  return (
    <Dialog open onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>{editing ? "Edit runbook" : "Add runbook"}</DialogTitle>
      <DialogContent sx={{ display: "flex", flexDirection: "column", gap: 2, pt: "8px !important" }}>
        <TextField
          label="Alert name"
          required
          autoFocus
          multiline
          value={alertName}
          onChange={(e) => setAlertName(e.target.value)}
          error={duplicate}
          helperText={
            duplicate
              ? "This alert already has an entry."
              : "Matched against a new incident's subject (contains, case-insensitive)."
          }
        />
        <TextField
          label="Runbook link"
          placeholder="https://docs.google.com/document/d/…"
          value={runbookUrl}
          onChange={(e) => setRunbookUrl(e.target.value)}
          error={urlInvalid}
          helperText={urlInvalid ? "Enter a full http(s) link." : "Leave empty to record that no runbook exists yet."}
        />
        <Alert severity={trimmedUrl && !urlInvalid ? "info" : "warning"} variant="outlined">
          Work note on matching incidents: {runbookWorkNote({ runbookUrl: trimmedUrl || null })}
        </Alert>
        {saveError && <Alert severity="error">{errorText(saveError, "Failed to save the runbook.")}</Alert>}
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Cancel</Button>
        <Button variant="contained" disabled={!canSave} onClick={handleSave}>
          {editing ? "Save" : "Add"}
        </Button>
      </DialogActions>
    </Dialog>
  );
}

function WorkNotePreview({ runbooks }: { runbooks: Runbook[] }): JSX.Element {
  const [subject, setSubject] = useState("");
  const match = subject.trim() ? findRunbookForIncident(runbooks, subject) : undefined;

  return (
    <Paper sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 1.5 }}>
      <Box>
        <Typography variant="subtitle2">Try it with an incident</Typography>
        <Typography variant="body2" color="text.secondary">
          Paste an incident subject to see the work note it would get when created by an alert.
        </Typography>
      </Box>
      <TextField
        size="small"
        placeholder="[Alert]Fired:Sev0 Azure Monitor Alert example-gw-CriticalHTTP5xxCountLimitExceeded-prod-001 on …"
        value={subject}
        onChange={(e) => setSubject(e.target.value)}
      />
      {subject.trim() && (
        <Alert severity={match?.runbookUrl ? "success" : "warning"}>
          <Typography variant="body2" sx={{ wordBreak: "break-word" }}>
            {match ? `Matched alert: ${match.alertName}` : "No alert in the catalog matches this incident."}
          </Typography>
          <Typography variant="body2" sx={{ mt: 0.5, fontWeight: 600, wordBreak: "break-word" }}>
            Work note: {runbookWorkNote(match)}
          </Typography>
        </Alert>
      )}
    </Paper>
  );
}
