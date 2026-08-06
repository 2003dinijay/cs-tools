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

import { useEffect, useRef, useState } from "react";
import { Box, Button, Divider, Skeleton, Stack, Typography } from "@wso2/oxygen-ui";
import { Plus } from "@wso2/oxygen-ui-icons-react";
import { useInfiniteQuery, useQueryClient } from "@tanstack/react-query";
import { timecards } from "@src/services/timecards";
import type { CaseSeverity, CreateTimeCardInput, CsmTimeCard } from "@src/types";
import { cardDateLabel, formatMinutes } from "@utils/timecard";
import { TimeCardStateChip } from "@components/timecards/TimeCardStateChip";
import { LogTimeCardDialog } from "./LogTimeCardDialog";

interface TimeTrackingTabProps {
  caseId: string;
  caseNumber: string;
  caseSeverity: CaseSeverity | null;
  projectId: string;
  projectName: string;
}

// A bordered, tinted box per entry — mirrors the webapp's CaseTimeCardsPanel row
// (border + rounded corners) and this app's own TimeSheetCard's inner
// TimeCardRow (same treatment, plus the `action.hover` tint), so a logged card
// reads as a distinct entry instead of a bare line of text in the list.
function TimeCardRow({ card }: { card: CsmTimeCard }) {
  return (
    <Stack
      gap={0.5}
      sx={{ p: 1.25, bgcolor: "action.hover", border: "1px solid", borderColor: "divider", borderRadius: 1 }}
    >
      <Stack direction="row" alignItems="center" justifyContent="space-between" gap={1}>
        <Typography variant="body2" noWrap sx={{ minWidth: 0 }}>
          {card.userName}
        </Typography>
        <Stack direction="row" alignItems="center" gap={1} flexShrink={0}>
          <Typography variant="body2">{formatMinutes(card.totalMinutes)}</Typography>
          <TimeCardStateChip state={card.state} />
        </Stack>
      </Stack>
      <Typography variant="caption" color="text.secondary">
        {card.billable ? "Billable" : "Non-billable"} · {cardDateLabel(card.createdOn)}
      </Typography>
    </Stack>
  );
}

/**
 * Scoped server-side by `filters.caseId` (see services/timecards.ts's `forCase`
 * and BeSearchTimeCardsFilters.caseId's comment on the backend fix this relies
 * on), paged the same way the "Time cards" > All tab is — an IntersectionObserver
 * sentinel fetches the next page as it scrolls into view — for the rare case
 * whose entries outnumber one page. There's no separate "Open Time Cards" entry
 * point to leave for any of the case's own cards, since this tab shows all of
 * them.
 */
export function TimeTrackingTab({ caseId, caseNumber, caseSeverity, projectId, projectName }: TimeTrackingTabProps) {
  const queryClient = useQueryClient();
  const [logTimeOpen, setLogTimeOpen] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const { data, isLoading, isError, hasNextPage, isFetching, isFetchingNextPage, fetchNextPage } = useInfiniteQuery(
    timecards.forCase(caseId),
  );
  const cards = data?.cards ?? [];
  const totalCount = data?.total ?? cards.length;
  const total = cards.reduce((sum, c) => sum + c.totalMinutes, 0);

  const sentinelRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const el = sentinelRef.current;
    if (!el) return;

    const observer = new IntersectionObserver(
      ([entry]) => {
        // Also gated on `!isFetching` (not just `!isFetchingNextPage`) so the sentinel can't fire
        // a next-page fetch while a *different* fetch is already in flight for this query — e.g.
        // the background refetch invalidateQueries triggers right after logging time reloads the
        // already-loaded pages, during which isFetchingNextPage is false but isFetching is true.
        if (entry?.isIntersecting && hasNextPage && !isFetching) {
          void fetchNextPage();
        }
      },
      { threshold: 0.1 },
    );
    observer.observe(el);
    return () => observer.disconnect();
  }, [hasNextPage, isFetching, fetchNextPage]);

  const handleSubmit = (input: CreateTimeCardInput) => {
    setIsSubmitting(true);
    setError(null);
    timecards
      .create(input)
      .then(() => {
        setLogTimeOpen(false);
        // Root key — one invalidate refreshes every time-cards view (My sheets,
        // All, Approvals, and this case's own list) after this write, per
        // services/timecards.ts's convention.
        void queryClient.invalidateQueries({ queryKey: ["timecards"] });
      })
      .catch(() => setError("Could not log time. Please try again."))
      .finally(() => setIsSubmitting(false));
  };

  return (
    <Stack gap={1.5}>
      <Stack direction="row" alignItems="center" justifyContent="space-between" gap={1}>
        <Box>
          <Typography variant="h6" sx={{ lineHeight: 1 }}>
            {formatMinutes(total)}
          </Typography>
          <Typography variant="caption" color="text.secondary">
            {/* While more pages are still to come, say so explicitly rather than letting the
             * loaded-so-far count read as the case's whole total. */}
            {hasNextPage
              ? `${cards.length} of ${totalCount} ${totalCount === 1 ? "entry" : "entries"} loaded so far`
              : `Across ${cards.length} ${cards.length === 1 ? "entry" : "entries"}`}
          </Typography>
        </Box>
        <Button variant="contained" size="small" startIcon={<Plus size={14} />} onClick={() => setLogTimeOpen(true)}>
          Log time
        </Button>
      </Stack>

      <Divider />

      {isLoading ? (
        <Stack gap={1}>
          {[0, 1].map((i) => (
            <Skeleton key={i} variant="rounded" height={48} />
          ))}
        </Stack>
      ) : isError ? (
        <Typography variant="body2" color="error">
          Could not load time cards.
        </Typography>
      ) : cards.length === 0 && !hasNextPage ? (
        <Typography variant="body2" color="text.secondary">
          No time logged on this case yet.
        </Typography>
      ) : (
        <Stack gap={1.5}>
          {cards.map((c) => (
            <TimeCardRow key={c.id} card={c} />
          ))}

          {/* IntersectionObserver can miss a zero-height target, so give the sentinel 1px to
           * observe. Kept mounted even once every card is in — cheap and avoids a layout jump. */}
          <div ref={sentinelRef} style={{ height: 1 }} />

          {(isFetchingNextPage || (cards.length === 0 && hasNextPage)) && <Skeleton variant="rounded" height={48} />}
        </Stack>
      )}

      {logTimeOpen && (
        <LogTimeCardDialog
          caseId={caseId}
          caseNumber={caseNumber}
          caseSeverity={caseSeverity}
          projectId={projectId}
          projectName={projectName}
          isSubmitting={isSubmitting}
          error={error}
          onClose={() => {
            if (!isSubmitting) {
              setLogTimeOpen(false);
              setError(null);
            }
          }}
          onSubmit={handleSubmit}
        />
      )}
    </Stack>
  );
}
