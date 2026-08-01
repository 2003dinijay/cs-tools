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
  Card,
  Chip,
  Skeleton,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableRow,
  Typography,
} from "@wso2/oxygen-ui";
import { ArrowLeft } from "@wso2/oxygen-ui-icons-react";
import { type JSX, type ReactNode } from "react";
import { Link as RouterLink, useParams } from "react-router";
import QueryErrorState from "@components/QueryErrorState";
import { useNavTransition } from "@hooks/useNavTransition";
import { formatBackendTimestampForDisplay } from "@utils/dateTime";
import { useGetUserById } from "@features/csm-users/api/useGetUserById";
import DirectoryEntityChip from "@features/csm-admin/components/DirectoryEntityChip";
import {
  INTERNAL_USER_ROLES,
  type NormalizedUserDetail,
  type UserProjectAccess,
} from "@features/csm-users/types/csmUsers";

function formatDateTime(value?: string | null): string {
  return (
    formatBackendTimestampForDisplay(value, {
      dateStyle: "medium",
      timeStyle: "short",
    }) ?? "—"
  );
}

function BackButton({ onClick }: { onClick: () => void }): JSX.Element {
  return (
    <Button
      variant="text"
      size="small"
      startIcon={<ArrowLeft size={16} />}
      onClick={onClick}
      sx={{ alignSelf: "flex-start" }}
    >
      Back
    </Button>
  );
}

function MetaCell({
  label,
  children,
}: {
  label: string;
  children: ReactNode;
}): JSX.Element {
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 0.25, minWidth: 0 }}>
      <Typography
        variant="caption"
        color="text.secondary"
        sx={{ textTransform: "uppercase", letterSpacing: 0.4 }}
      >
        {label}
      </Typography>
      <Box sx={{ minWidth: 0 }}>{children}</Box>
    </Box>
  );
}

/**
 * True when `user` is internal (WSO2 staff): either a direct
 * `userType === "internal"`, or (on a backend response predating `userType`)
 * `roles` containing one of {@link INTERNAL_USER_ROLES}. Every other
 * `userType` value (`external`, `customer`, `system`) is treated alike as
 * "not internal" — the two data sources don't agree on the exact label, so
 * branching on `=== "external"` alone would silently miss the postgres
 * source's `customer`/`system` users.
 */
function isInternalUser(user: NormalizedUserDetail): boolean {
  if (user.userType) return user.userType === "internal";
  return (user.roles ?? []).some((r) =>
    (INTERNAL_USER_ROLES as string[]).includes(r),
  );
}

/**
 * Human-readable reasons a project-access row doesn't grant case access.
 * `grantsCaseAccess` is the verdict the backend already computed; these are
 * the "why" a support engineer is looking for. Order matters: the missing
 * contact record is the more fundamental failure, so it's listed first when
 * both are true.
 */
function blockedReasons(pa: UserProjectAccess): string[] {
  const reasons: string[] = [];
  if (!pa.contactRecordPresent) {
    reasons.push("No contact record is linked to this project for this user.");
  } else if (!pa.emailMatchesLogin) {
    reasons.push(
      pa.contactRecordEmail
        ? `Contact record email (${pa.contactRecordEmail}) doesn't match the login email (${pa.contactEmail}).`
        : "The linked contact record's email doesn't match the login email.",
    );
  }
  return reasons;
}

/** One row of an external user's per-project access, with the reason surfaced when blocked. */
function ProjectAccessRow({ pa }: { pa: UserProjectAccess }): JSX.Element {
  const reasons = blockedReasons(pa);
  return (
    <Box
      sx={{
        display: "flex",
        flexDirection: "column",
        gap: 0.75,
        p: 1.5,
        border: 1,
        borderColor: "divider",
        borderRadius: 1,
      }}
    >
      <Box sx={{ display: "flex", alignItems: "center", gap: 1, flexWrap: "wrap" }}>
        <Typography
          component={RouterLink}
          to={`/customers/projects/${pa.projectId}`}
          variant="body2"
          sx={(t) => ({
            fontWeight: 600,
            textDecoration: "none",
            color: t.palette.primary.dark,
            ...t.applyStyles("dark", { color: t.palette.primary.main }),
            "&:hover": { textDecoration: "underline" },
          })}
        >
          {pa.projectName}
        </Typography>
        <Chip
          size="small"
          label={pa.grantsCaseAccess ? "Has case access" : "Blocked"}
          color={pa.grantsCaseAccess ? "success" : "error"}
          variant="outlined"
        />
        {pa.registrationState && (
          <Chip size="small" label={pa.registrationState} variant="outlined" />
        )}
        {pa.roles?.map((r) => (
          <Chip key={r} size="small" label={r} variant="outlined" />
        ))}
      </Box>

      {!pa.grantsCaseAccess && reasons.length > 0 && (
        <Stack spacing={0.25}>
          {reasons.map((reason) => (
            <Typography key={reason} variant="caption" color="error.main">
              {reason}
            </Typography>
          ))}
        </Stack>
      )}

      <Typography variant="caption" color="text.secondary">
        Contact email: {pa.contactEmail}
        {pa.notificationsEnabled !== undefined &&
          ` · Notifications ${pa.notificationsEnabled ? "on" : "off"}`}
      </Typography>
    </Box>
  );
}

/** One row a {@link MembershipTable} renders: a role, group, or team the
 * profile's user belongs to, plus enough to link it to its directory page. */
interface MembershipRow {
  key: string;
  id: string;
  label: string;
  routeBase: string;
  color?: "default" | "primary";
}

/**
 * A single-column table of role/group/team memberships, one row per entry,
 * each name a link to that entity's directory page (see
 * `DirectoryEntityChip`). Rendered even when `rows` is empty — "no
 * memberships" is itself an answer worth showing rather than hiding the
 * section, per {@link emptyMessage}.
 *
 * Deliberately no header row naming the column ("Role"/"Group"/"Team") —
 * the enclosing card's own title already says that, and repeating it inside
 * read as "Groups" containing a table headed "Group".
 */
function MembershipTable({
  ariaLabel,
  rows,
  emptyMessage,
}: {
  ariaLabel: string;
  rows: MembershipRow[];
  emptyMessage: string;
}): JSX.Element {
  if (rows.length === 0) {
    return (
      <Typography variant="body2" color="text.secondary">
        {emptyMessage}
      </Typography>
    );
  }
  return (
    <TableContainer sx={{ border: 1, borderColor: "divider", borderRadius: 1 }}>
      <Table size="small" aria-label={ariaLabel}>
        <TableBody>
          {rows.map((row) => (
            <TableRow key={row.key}>
              <TableCell>
                <DirectoryEntityChip
                  id={row.id}
                  name={row.label}
                  routeBase={row.routeBase}
                  color={row.color}
                />
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </TableContainer>
  );
}

/**
 * Roles/groups/teams as three cards in a row (wrapping to stacked on narrow
 * viewports), so the three membership kinds read as siblings rather than a
 * single long vertical list. Each card's title is the only place its kind is
 * named — the table inside carries no repeated column header.
 */
function MembershipCard({
  title,
  rows,
  emptyMessage,
}: {
  title: string;
  rows: MembershipRow[];
  emptyMessage: string;
}): JSX.Element {
  return (
    <Card sx={{ p: 2.5, flex: "1 1 260px", minWidth: 220, display: "flex", flexDirection: "column", gap: 2 }}>
      <Typography variant="subtitle2">{title}</Typography>
      <MembershipTable ariaLabel={`${title} memberships`} rows={rows} emptyMessage={emptyMessage} />
    </Card>
  );
}

/** This user's assigned roles (all user types). */
function RolesSection({ user }: { user: NormalizedUserDetail }): JSX.Element {
  const rows: MembershipRow[] = (user.roles ?? []).map((r) => ({
    key: r,
    id: r,
    label: r,
    routeBase: "/admin/roles",
    color: (INTERNAL_USER_ROLES as string[]).includes(r) ? "primary" : "default",
  }));
  return <MembershipCard title="Roles" rows={rows} emptyMessage="No roles assigned." />;
}

/** An internal user's group memberships. */
function GroupsSection({ user }: { user: NormalizedUserDetail }): JSX.Element {
  const rows: MembershipRow[] = (user.groups ?? []).map((g) => ({
    key: g.id,
    id: g.id,
    label: g.name,
    routeBase: "/admin/groups",
  }));
  return <MembershipCard title="Groups" rows={rows} emptyMessage="No group memberships." />;
}

/** An internal user's CRE/SRE team assignments. */
function TeamsSection({ user }: { user: NormalizedUserDetail }): JSX.Element {
  const rows: MembershipRow[] = (user.teams ?? []).map((t) => ({
    key: t.id,
    id: t.id,
    label: t.family ? `${t.name} (${t.family})` : t.name,
    routeBase: "/admin/teams",
    color: "primary",
  }));
  return <MembershipCard title="Teams" rows={rows} emptyMessage="No team assignments." />;
}

/**
 * An external user's per-project access, with the reason called out whenever
 * a project doesn't grant case access — this card exists to answer "why
 * can't this customer see their cases?" at a glance.
 */
function ProjectAccessSection({ user }: { user: NormalizedUserDetail }): JSX.Element {
  const access = user.projectAccess ?? [];
  const blockedCount = access.filter((pa) => !pa.grantsCaseAccess).length;

  return (
    <Card sx={{ p: 2.5, flex: "2 1 400px", display: "flex", flexDirection: "column", gap: 2 }}>
      <Typography variant="subtitle2">Project access</Typography>

      {user.active === false && (
        <Alert severity="error" variant="outlined">
          This user's account is inactive — they can't access any project's cases,
          regardless of the per-project rows below.
        </Alert>
      )}

      {access.length === 0 ? (
        <Typography variant="body2" color="text.secondary">
          No project access records found for this user.
        </Typography>
      ) : (
        <>
          {blockedCount > 0 && (
            <Alert severity="warning" variant="outlined">
              Blocked on {blockedCount} of {access.length} project
              {access.length === 1 ? "" : "s"} — see the reason under each row below.
            </Alert>
          )}
          <Stack spacing={1.5}>
            {access.map((pa) => (
              <ProjectAccessRow key={pa.projectId} pa={pa} />
            ))}
          </Stack>
        </>
      )}
    </Card>
  );
}

/**
 * A person's profile page, reachable by clicking any user reference in the
 * portal (case creator, assignee, watchers, comment authors, attachment
 * uploaders) once its id is known or resolved (see `UserRefLink` /
 * `useResolvedUserId` — most actor fields carry only an email, resolved to an
 * id through a cached lookup before the link ever appears).
 *
 * Renders everything `GET /users/{id}` returns: name, email, timezone, phone,
 * roles, created/updated times, plus — split by `userType` — an internal
 * user's group/team memberships or an external user's per-project access
 * (with the reason surfaced whenever a project doesn't grant case access, per
 * `UserProjectAccess.grantsCaseAccess`).
 */
export default function UserProfilePage(): JSX.Element {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavTransition();

  const { data: user, isLoading, isError, error } = useGetUserById(id);

  if (isLoading) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
        <Skeleton variant="rounded" height={32} width={240} />
        <Skeleton variant="rounded" height={220} />
      </Box>
    );
  }

  if (isError) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        <BackButton onClick={() => navigate(-1)} />
        <QueryErrorState
          message={`Failed to load user: ${error instanceof Error ? error.message : "unknown error"}`}
          error={error}
        />
      </Box>
    );
  }

  if (!user) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        <BackButton onClick={() => navigate(-1)} />
        <Typography variant="h5">User not found</Typography>
        <Typography variant="body2" color="text.secondary">
          No user with id <code>{id}</code>.
        </Typography>
      </Box>
    );
  }

  const internal = isInternalUser(user);

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2.5 }}>
      <BackButton onClick={() => navigate(-1)} />

      <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, flexWrap: "wrap" }}>
          <Typography variant="h5">{user.name || user.userName || "—"}</Typography>
          <Chip
            size="small"
            label={internal ? "Internal" : "Customer"}
            color={internal ? "primary" : "default"}
            variant="outlined"
          />
          {user.active !== undefined && (
            <Chip
              size="small"
              label={user.active ? "Active" : "Inactive"}
              color={user.active ? "success" : "default"}
              variant="outlined"
            />
          )}
        </Box>
        <Typography variant="body2" color="text.secondary">
          {user.email}
        </Typography>
      </Box>

      <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
        <Typography variant="subtitle2">Overview</Typography>
        <Box
          sx={{
            display: "grid",
            gap: 2,
            gridTemplateColumns: {
              xs: "1fr",
              sm: "repeat(2, minmax(0, 1fr))",
              md: "repeat(3, minmax(0, 1fr))",
            },
          }}
        >
          <MetaCell label="Username">
            <Typography variant="body2">{user.userName}</Typography>
          </MetaCell>
          <MetaCell label="Email">
            <Typography variant="body2" sx={{ wordBreak: "break-all" }}>
              {user.email}
            </Typography>
          </MetaCell>
          <MetaCell label="Timezone">
            <Typography variant="body2">{user.timezone ?? "Not set"}</Typography>
          </MetaCell>
          {internal && (
            <MetaCell label="Phone">
              <Typography variant="body2">{user.phone ?? "Not set"}</Typography>
            </MetaCell>
          )}
          <MetaCell label="Created on">
            <Typography variant="body2">{formatDateTime(user.createdOn)}</Typography>
          </MetaCell>
          <MetaCell label="Updated on">
            <Typography variant="body2">{formatDateTime(user.updatedOn)}</Typography>
          </MetaCell>
        </Box>
      </Card>

      <Box sx={{ display: "flex", flexWrap: "wrap", gap: 2.5 }}>
        <RolesSection user={user} />
        {internal ? (
          <>
            <GroupsSection user={user} />
            <TeamsSection user={user} />
          </>
        ) : (
          <ProjectAccessSection user={user} />
        )}
      </Box>
    </Box>
  );
}
