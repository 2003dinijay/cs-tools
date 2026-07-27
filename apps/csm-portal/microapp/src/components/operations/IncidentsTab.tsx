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

import { Suspense, useEffect, useRef, useState, type ReactNode } from "react";
import { Badge, Chip, IconButton, Stack } from "@wso2/oxygen-ui";
import { SlidersHorizontal } from "@wso2/oxygen-ui-icons-react";
import { useQueryErrorResetBoundary, useSuspenseInfiniteQuery } from "@tanstack/react-query";
import { incidents } from "@src/services/incidents";
import { ErrorBoundary } from "@components/common/ErrorBoundary";
import { EmptyState } from "@components/support/EmptyState";
import { ErrorState } from "@components/support/ErrorState";
import { SearchBar } from "@components/support/SearchBar";
import { useDebouncedValue } from "@utils/useDebouncedValue";
import { IncidentCard, IncidentCardSkeleton } from "./IncidentCard";
import { IncidentsFiltersSheet } from "./IncidentsFiltersSheet";
import {
  countActiveIncidentFilters,
  EMPTY_INCIDENT_FILTERS,
  INCIDENT_PRIORITY_LABELS,
  toIncidentSearchFilters,
  type IncidentFilters,
} from "./incidentConfig";

// Mirrors the shape of ChangeRequestsTab.tsx's own list content (search + filter sheet + infinite
// scroll), scoped to incidents. Read-only for now — no create Fab (see the scope note on
// OperationsPage.tsx).
export function IncidentsTab() {
  const [search, setSearch] = useState("");
  const debouncedSearch = useDebouncedValue(search, 300);
  const [filters, setFilters] = useState<IncidentFilters>(EMPTY_INCIDENT_FILTERS);
  const [filtersOpen, setFiltersOpen] = useState(false);
  const activeFilterCount = countActiveIncidentFilters(filters);

  return (
    <Stack gap={2}>
      <Stack direction="row" gap={1} alignItems="center">
        <SearchBar value={search} onChange={setSearch} placeholder="Search by incident #, subject…" />
        <Badge badgeContent={activeFilterCount} color="primary" invisible={activeFilterCount === 0}>
          <IconButton aria-label="Filters" onClick={() => setFiltersOpen(true)}>
            <SlidersHorizontal size={18} />
          </IconButton>
        </Badge>
      </Stack>

      {activeFilterCount > 0 && <ActiveFiltersRow filters={filters} onChange={setFilters} />}

      <IncidentListErrorBoundary>
        <Suspense fallback={<IncidentListSkeleton />}>
          <IncidentListContent search={debouncedSearch} filters={filters} />
        </Suspense>
      </IncidentListErrorBoundary>

      <IncidentsFiltersSheet
        open={filtersOpen}
        onClose={() => setFiltersOpen(false)}
        filters={filters}
        onApply={setFilters}
      />
    </Stack>
  );
}

// One removable chip per active filter — mirrors ChangeRequestsTab.tsx's own ActiveFiltersRow.
function ActiveFiltersRow({
  filters,
  onChange,
}: {
  filters: IncidentFilters;
  onChange: (filters: IncidentFilters) => void;
}) {
  return (
    <Stack direction="row" gap={1} flexWrap="wrap">
      {filters.priorities.map((priority) => (
        <Chip
          key={`priority-${priority}`}
          label={INCIDENT_PRIORITY_LABELS[priority]}
          size="small"
          onDelete={() => onChange({ ...filters, priorities: filters.priorities.filter((p) => p !== priority) })}
        />
      ))}
    </Stack>
  );
}

function IncidentListContent({ search, filters }: { search: string; filters: IncidentFilters }) {
  const { data, fetchNextPage, hasNextPage, isFetchingNextPage } = useSuspenseInfiniteQuery(
    incidents.infinite({ filters: toIncidentSearchFilters(search, filters) }),
  );

  const sentinelRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const el = sentinelRef.current;
    if (!el) return;

    const observer = new IntersectionObserver(
      ([entry]) => {
        if (entry?.isIntersecting && hasNextPage && !isFetchingNextPage) {
          void fetchNextPage();
        }
      },
      { threshold: 0.1 },
    );
    observer.observe(el);
    return () => observer.disconnect();
  }, [hasNextPage, isFetchingNextPage, fetchNextPage]);

  const items = data.pages.flatMap((page) => page.items);

  if (items.length === 0) return <EmptyState message="No incidents found." />;

  return (
    <Stack gap={1.5}>
      {items.map((item, index) => (
        // item.id is nullable (see incident.dto.ts) — fall back to the index so multiple
        // incidents with a null id (an edge case the type allows) still get distinct React keys.
        <IncidentCard key={item.id ?? `incident-${index}`} item={item} />
      ))}

      <div ref={sentinelRef} style={{ height: 1 }} />

      {isFetchingNextPage && <IncidentCardSkeleton />}
    </Stack>
  );
}

function IncidentListSkeleton() {
  return (
    <Stack gap={1.5}>
      {[0, 1, 2].map((i) => (
        <IncidentCardSkeleton key={i} />
      ))}
    </Stack>
  );
}

function IncidentListErrorBoundary({ children }: { children: ReactNode }) {
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
