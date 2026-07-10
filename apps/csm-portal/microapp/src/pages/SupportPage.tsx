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

import { Suspense, type ReactNode } from "react";
import { useSearchParams } from "react-router-dom";
import { Stack, Tab, Tabs, Typography } from "@wso2/oxygen-ui";
import { useQuery, useQueryErrorResetBoundary, useSuspenseQuery } from "@tanstack/react-query";
import { cases } from "@src/services/cases";
import { currentUser } from "@src/services/currentUser";
import type { CaseType } from "@src/types";
import { ErrorBoundary } from "@components/common/ErrorBoundary";
import { CaseCard, CaseCardSkeleton } from "@components/support/CaseCard";
import { EmptyState } from "@components/support/EmptyState";
import { ErrorState } from "@components/support/ErrorState";
import { ItemListHeader } from "@components/support/ItemListHeader";
import { EMPTY_FILTERS, toCaseSearchFilters } from "@components/support/filters";
import { TABS, TAB_CONFIG } from "@components/support/config";

// Recent-items preview above the tab's "View All" link, mirroring the customer-portal
// microapp's SupportPage (ItemListView + a 5-item recent query). Search/filters live on the
// "View All" page only — the recent view is a quick, uncluttered glance.
const RECENT_CASES_LIMIT = 5;

export default function SupportPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  // Derived directly from the URL (not local state) so browser back/forward navigation that
  // changes ?tab= is reflected immediately, instead of showing a stale tab.
  const tab = TABS.find((t) => t === searchParams.get("tab")) ?? "case";

  const { data: currentUserId } = useQuery(currentUser.id());

  const handleTabChange = (value: CaseType) => {
    setSearchParams(
      (prev) => {
        prev.set("tab", value);
        return prev;
      },
      { replace: true },
    );
  };

  return (
    <Stack gap={2}>
      <Typography variant="h5">Support</Typography>

      <Tabs variant="scrollable" value={tab} onChange={(_, value) => handleTabChange(value)}>
        {TABS.map((t) => (
          <Tab key={t} label={TAB_CONFIG[t].title} value={t} disableRipple />
        ))}
      </Tabs>

      <ItemListHeader title={TAB_CONFIG[tab].title} viewAllPath={`/support/${tab}/all`}>
        <CaseListErrorBoundary>
          <Suspense fallback={<CaseListSkeleton />}>
            <CaseListContent type={tab} currentUserId={currentUserId ?? null} />
          </Suspense>
        </CaseListErrorBoundary>
      </ItemListHeader>
    </Stack>
  );
}

function CaseListContent({ type, currentUserId }: { type: CaseType; currentUserId: string | null }) {
  const { data } = useSuspenseQuery(
    cases.all({
      filters: toCaseSearchFilters(type, "", EMPTY_FILTERS, currentUserId),
      sortBy: { field: "updatedOn", order: "desc" },
      pagination: { limit: RECENT_CASES_LIMIT },
    }),
  );

  if (data.items.length === 0) return <EmptyState message={TAB_CONFIG[type].emptyMessage} />;

  return (
    <Stack gap={1.5}>
      {data.items.map((item) => (
        <CaseCard key={item.id} item={item} />
      ))}
    </Stack>
  );
}

function CaseListSkeleton() {
  return (
    <Stack gap={1.5}>
      {Array.from({ length: 4 }).map((_, index) => (
        <CaseCardSkeleton key={index} />
      ))}
    </Stack>
  );
}

function CaseListErrorBoundary({ children }: { children: ReactNode }) {
  const { reset } = useQueryErrorResetBoundary();

  return (
    <ErrorBoundary
      fallback={(_error, resetErrorBoundary) => (
        <ErrorState
          onRetry={() => {
            reset();
            resetErrorBoundary();
          }}
        />
      )}
    >
      {children}
    </ErrorBoundary>
  );
}
