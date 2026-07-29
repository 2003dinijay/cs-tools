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

import { Box, Button, Card, Chip, Skeleton, Stack, Typography } from "@wso2/oxygen-ui";
import { ArrowLeft } from "@wso2/oxygen-ui-icons-react";
import { useMemo, type JSX, type ReactNode } from "react";
import { useParams } from "react-router";
import QueryErrorState from "@components/QueryErrorState";
import { useNavTransition } from "@hooks/useNavTransition";
import { useSearchUsers } from "@features/csm-users/api/useSearchUsers";
import {
  INTERNAL_USER_ROLES,
  type NormalizedUser,
} from "@features/csm-users/types/csmUsers";

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
 * True when `user` is internal (WSO2 staff): either the postgres source's
 * direct `userType === "internal"`, or the ServiceNow source's `roles`
 * containing one of {@link INTERNAL_USER_ROLES}. Absent both signals, the
 * user is treated as a customer.
 */
function isInternalUser(user: NormalizedUser): boolean {
  if (user.userType) return user.userType === "internal";
  return (user.roles ?? []).some((r) =>
    (INTERNAL_USER_ROLES as string[]).includes(r),
  );
}

/**
 * Placeholder section for data the backend doesn't expose yet, following the
 * same visual language as `CsmComingSoonPage` and the admin `roles`/`groups`
 * stub routes: a "Coming soon" chip plus a short description of what's
 * blocked and why, so it reads as an intentional gap rather than a bug.
 */
function NotAvailableYetSection({
  title,
  description,
  blockedOn,
}: {
  title: string;
  description: string;
  blockedOn: string;
}): JSX.Element {
  return (
    <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 1.5 }}>
      <Box sx={{ display: "flex", alignItems: "center", gap: 1.5 }}>
        <Typography variant="subtitle2">{title}</Typography>
        <Chip size="small" label="Coming soon" color="warning" variant="outlined" />
      </Box>
      <Typography variant="body2" color="text.secondary">
        {description}
      </Typography>
      <Typography variant="caption" color="text.secondary">
        Blocked on: {blockedOn}
      </Typography>
    </Card>
  );
}

/**
 * A person's profile page, reachable by clicking any user reference in the
 * portal (case creator, assignee, watchers, comment authors, attachment
 * uploaders). Shows only what `POST /users/search` already returns today —
 * name, email, timezone, roles/type, active status. There is no backend
 * endpoint yet for a customer's projects+roles or an internal user's
 * groups/team, so that section renders the same "not available yet"
 * placeholder used by the admin roles/groups stub routes rather than
 * fabricating data.
 */
export default function UserProfilePage(): JSX.Element {
  const { email: emailParam } = useParams<{ email: string }>();
  const navigate = useNavTransition();
  const email = useMemo(() => {
    if (!emailParam) return undefined;
    try {
      return decodeURIComponent(emailParam);
    } catch {
      // Malformed percent-encoding (e.g. a lone "%") — fall through to the
      // existing not-found state instead of throwing during render.
      return undefined;
    }
  }, [emailParam]);

  const { data, isLoading, isError, error } = useSearchUsers({
    filters: email ? { emails: [email] } : undefined,
  });

  const user = data?.users?.[0];

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
          No user with email <code>{email}</code>.
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
          <MetaCell label="Roles">
            {user.roles && user.roles.length > 0 ? (
              <Stack direction="row" spacing={0.5} sx={{ flexWrap: "wrap", gap: 0.5 }}>
                {user.roles.map((r) => (
                  <Chip
                    key={r}
                    size="small"
                    label={r}
                    color={(INTERNAL_USER_ROLES as string[]).includes(r) ? "primary" : "default"}
                    variant="outlined"
                  />
                ))}
              </Stack>
            ) : (
              <Typography variant="body2">—</Typography>
            )}
          </MetaCell>
        </Box>
      </Card>

      {internal ? (
        <NotAvailableYetSection
          title="Groups, roles, and team"
          description="This engineer's group memberships, permission roles, and CRE/SRE team assignment aren't available here yet."
          blockedOn="csm-portal/backend groups and roles endpoints"
        />
      ) : (
        <NotAvailableYetSection
          title="Projects and roles per project"
          description="The projects this customer has access to, and their role on each, aren't available here yet."
          blockedOn="csm-portal/backend per-project role endpoints"
        />
      )}
    </Box>
  );
}
