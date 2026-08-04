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

import { Card, IconButton, Link, Stack, Tooltip, Typography } from "@wso2/oxygen-ui";
import { RefreshCw } from "@wso2/oxygen-ui-icons-react";
import { useNavigate } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { currentUser } from "@src/services/currentUser";
import { dashboard } from "@src/services/dashboard";
import { CaseCard, CaseCardSkeleton } from "@components/support/CaseCard";
import { ErrorState } from "@components/support/ErrorState";
import { EMPTY_FILTERS, filtersToSearchParams } from "@components/support/filters";
import { compareByUpdatedOnDesc, fromNow } from "@utils/dateTime";

// Deep-link to Support, pre-filtered to the caller's cases — mirrors the webapp's VIEW_ALL_HREF
// (MyAssignedCases.tsx): `assignees=@me`/`assignedToMe=1` resolves against the current user
// server-side, so this needs no id of its own.
const VIEW_ALL_HREF = `/support?${filtersToSearchParams("", { ...EMPTY_FILTERS, assignedToMe: true })}`;

// The signed-in user's own non-closed cases — mirrors the webapp's "Assigned to me" widget
// (apps/csm-portal/webapp/src/features/csm-dashboard/components/MyAssignedCases.tsx): same "View
// all" deep-link + refresh control, capped to a short 4-item preview. The webapp puts title/View
// all/Last refreshed/refresh icon all in one row — fine on a wide desktop card, but on a phone
// width that crushes the title itself into wrapping. Split across two rows instead: title + icon
// on top (always fits), "View all" + "Last refreshed" below (each has its own room to breathe).
export function AssignedToMeSection() {
  const navigate = useNavigate();
  const { data: currentUserId } = useQuery(currentUser.id());
  const { data, isPending, isFetching, isError, dataUpdatedAt, refetch } = useQuery(
    dashboard.assignedToMe(currentUserId ?? null),
  );

  // sortBy: updatedOn desc is sent on the request (see dashboard.ts) but isn't reliably honored
  // upstream — re-sort client-side as a backstop. See compareByUpdatedOnDesc for why.
  const items = (data?.items ?? []).sort((a, b) => compareByUpdatedOnDesc(a.updatedOn, b.updatedOn));
  // Mirrors the webapp's `total > 0` guard for showing "View all" at all.
  const hasViewAll = (data?.total ?? 0) > 0;

  return (
    <Stack gap={1}>
      <Stack direction="row" alignItems="center" justifyContent="space-between">
        <Typography variant="subtitle1">Assigned to me</Typography>
        <Tooltip title="Refresh assigned cases">
          {/* span wrapper so the tooltip still shows while the button is disabled */}
          <span>
            <IconButton
              size="small"
              onClick={() => void refetch()}
              disabled={isFetching}
              aria-label="Refresh assigned cases"
            >
              <RefreshCw size={14} />
            </IconButton>
          </span>
        </Tooltip>
      </Stack>

      {(hasViewAll || dataUpdatedAt) && (
        <Stack direction="row" alignItems="center" justifyContent="space-between">
          <span>
            {hasViewAll && (
              <Link
                component="button"
                type="button"
                underline="hover"
                variant="body2"
                onClick={() => navigate(VIEW_ALL_HREF)}
              >
                View all
              </Link>
            )}
          </span>
          <span>
            {dataUpdatedAt ? (
              <Typography variant="caption" color="text.secondary">
                Last refreshed {fromNow(new Date(dataUpdatedAt))}
              </Typography>
            ) : null}
          </span>
        </Stack>
      )}

      {isPending ? (
        // Only 2 (not the 6 Support's own list skeleton shows) — this is a short preview widget,
        // not a case list. Keeping it short leaves the Case composition donuts visible in the
        // same first viewport, so Home's loading state reads as the dashboard it is rather than
        // looking like Support's own (much longer) loading skeleton.
        <Stack gap={1.5}>
          {Array.from({ length: 2 }).map((_, index) => (
            <CaseCardSkeleton key={index} />
          ))}
        </Stack>
      ) : isError ? (
        <ErrorState onRetry={() => void refetch()} />
      ) : items.length === 0 ? (
        // Lighter than the shared EmptyState (no icon, less padding) — a big "nothing here"
        // block reads as too prominent for the first thing on the home page — but still boxed
        // in a card so it's clearly this section's content, not blank/broken-looking space.
        <Card variant="outlined" sx={{ p: 1.5 }}>
          <Typography variant="body2" color="text.secondary">
            No cases assigned to you.
          </Typography>
        </Card>
      ) : (
        <Stack gap={1.5}>
          {items.map((item) => (
            <CaseCard key={item.id} item={item} />
          ))}
        </Stack>
      )}
    </Stack>
  );
}
