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
  Chip,
  Paper,
  Skeleton,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Typography,
} from "@wso2/oxygen-ui";
import type { JSX } from "react";
import QueryErrorState from "@components/QueryErrorState";
import UserRefLink from "@components/UserRefLink";
import { useSearchProjectContacts } from "@features/csm-projects/api/useSearchProjectContacts";

const COLUMN_COUNT = 4;

interface ProjectContactsTabProps {
  projectId: string;
}

/**
 * Lists a project's contacts (`POST /projects/{id}/contacts/search`). Every
 * row with a linked contact record is click-through to that person's profile
 * via `UserRefLink` (so nullable-id resolution and the plain-text fallback
 * come for free); a row with no `id` — meaning it has no linked contact
 * record — renders unlinked and flagged, since that's precisely the case a
 * support engineer needs to notice: an orphaned contact row means that
 * person silently can't see this project's cases.
 *
 * A real project can return dozens of these rows with `name` *and* `email`
 * both empty (no natural identifier at all, not even an email to show) — the
 * explanatory line below the row is always rendered inline, not tucked behind
 * a hover tooltip, so scanning the table surfaces every "can't see their
 * cases" row without hovering each one. (If an upstream fix ever starts
 * populating `email` for these rows, they fall into the ordinary
 * has-some-info orphaned case below — same flag, same inline reason, just
 * with a real address shown instead of "No linked contact record".)
 */
export default function ProjectContactsTab({
  projectId,
}: ProjectContactsTabProps): JSX.Element {
  const { data, isLoading, isError, error } = useSearchProjectContacts(projectId);
  const contacts = data ?? [];

  return (
    <Paper variant="outlined">
      <TableContainer>
        <Table size="small">
          <TableHead>
            <TableRow>
              <TableCell>Name</TableCell>
              <TableCell>Email</TableCell>
              <TableCell>Roles</TableCell>
              <TableCell>Registration</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {isLoading ? (
              Array.from({ length: 3 }).map((_, i) => (
                <TableRow key={i}>
                  {Array.from({ length: COLUMN_COUNT }).map((__, c) => (
                    <TableCell key={c}>
                      <Skeleton variant="rounded" width="70%" height={18} />
                    </TableCell>
                  ))}
                </TableRow>
              ))
            ) : isError ? (
              <TableRow>
                <TableCell colSpan={COLUMN_COUNT} align="center">
                  <QueryErrorState
                    message={`Failed to load project contacts: ${error instanceof Error ? error.message : "unknown error"}`}
                    error={error}
                  />
                </TableCell>
              </TableRow>
            ) : contacts.length === 0 ? (
              <TableRow>
                <TableCell colSpan={COLUMN_COUNT} align="center" sx={{ py: 4 }}>
                  <Typography variant="body2" color="text.secondary">
                    No contacts found for this project.
                  </Typography>
                </TableCell>
              </TableRow>
            ) : (
              contacts.map((c, i) => {
                const orphaned = !c.id;
                // No natural identifier at all when both are empty — say so
                // plainly instead of showing a bare "—" that reads as a data
                // glitch rather than the operationally meaningful fact that
                // this row has no linked contact record.
                const name = c.name || c.email || "No linked contact record";
                return (
                  // Contacts have no stable identifier when unlinked (no `id`);
                  // email is the closest thing to a natural key, and index
                  // disambiguates the rare case of a duplicate/blank email.
                  <TableRow key={c.id ?? `${c.email ?? "unknown"}-${i}`} hover>
                    <TableCell>
                      <UserRefLink name={name} email={c.email} userId={c.id ?? null} />
                      {orphaned && (
                        <Chip
                          size="small"
                          label="Orphaned"
                          color="error"
                          variant="outlined"
                          sx={{ ml: 1 }}
                        />
                      )}
                      {orphaned && (
                        <Typography
                          variant="caption"
                          color="error.main"
                          component="div"
                          sx={{ mt: 0.25 }}
                        >
                          No contact record is linked to this row — this person
                          can't see this project's cases.
                        </Typography>
                      )}
                    </TableCell>
                    <TableCell sx={{ wordBreak: "break-all" }}>{c.email || "—"}</TableCell>
                    <TableCell>
                      {c.roles && c.roles.length > 0
                        ? c.roles.map((r) => (
                            <Chip
                              key={r}
                              size="small"
                              label={r}
                              variant="outlined"
                              sx={{ mr: 0.5, mb: 0.5 }}
                            />
                          ))
                        : "—"}
                    </TableCell>
                    <TableCell>{c.registrationState || "—"}</TableCell>
                  </TableRow>
                );
              })
            )}
          </TableBody>
        </Table>
      </TableContainer>
    </Paper>
  );
}
