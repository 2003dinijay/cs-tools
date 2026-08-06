// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License. You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

import { Box, CircularProgress, Grid, Pagination, Typography } from "@wso2/oxygen-ui";
import {
  useState,
  useEffect,
  type JSX,
  type ChangeEvent,
} from "react";
import useSearchProjectCaseTimeCards from "@features/usage-metrics/api/useSearchProjectCaseTimeCards";
import ServiceHoursStatCards from "@time-tracking/ServiceHoursStatCards";
import TimeCardsDateFilter from "@time-tracking/TimeCardsDateFilter";
import TimeTrackingCard from "@time-tracking/TimeTrackingCard";
import TimeTrackingCardSkeleton from "@time-tracking/TimeTrackingCardSkeleton";
import TimeTrackingErrorState from "@time-tracking/TimeTrackingErrorState";
import EmptyState from "@components/empty-state/EmptyState";
import TimeCardsCsvExportButton from "@time-tracking/TimeCardsCsvExportButton";

import type { ProjectTimeTrackingProps } from "@features/project-details/types/projectDetailsComponents";

/**
 * ProjectTimeTracking manages the display of time tracking statistics, date filter, and case time cards.
 *
 * @param {ProjectTimeTrackingProps} props - Component props.
 * @returns {JSX.Element} The rendered component.
 */
export default function ProjectTimeTracking({
  projectId,
  project,
  isProjectLoading,
  isProjectError,
}: ProjectTimeTrackingProps): JSX.Element {
  const [startDate, setStartDate] = useState("");
  const [endDate, setEndDate] = useState("");
  const [page, setPage] = useState(1);
  const pageSize = 10;

  const {
    data,
    isLoading: isTimeCardsLoading,
    isFetching: isTimeCardsFetching,
    isError: isTimeCardsError,
  } = useSearchProjectCaseTimeCards({
    projectId,
    startDate,
    endDate,
    states: ["Approved"],
    page,
    pageSize,
    enabled: !!projectId,
  });

  // Reset pagination when the project changes
  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setPage(1);
  }, [projectId]);

  const paginatedTimeCards = data?.caseTimeCards ?? [];
  const totalItems = data?.totalRecords ?? 0;
  const totalPages = Math.ceil(totalItems / pageSize);

  const handlePageChange = (_event: ChangeEvent<unknown>, value: number) => {
    setPage(value);
  };

  const handleStartDateChange = (value: string) => {
    setPage(1);
    setStartDate(value);
  };

  const handleEndDateChange = (value: string) => {
    setPage(1);
    setEndDate(value);
  };

  const handleClearDates = () => {
    setPage(1);
    setStartDate("");
    setEndDate("");
  };

  const hasDateFilters = Boolean(startDate && endDate);

  return (
    <Box>
      <ServiceHoursStatCards
        project={project}
        isLoading={isProjectLoading}
        isError={isProjectError}
      />

      <Box sx={{ mb: 3 }}>
        <TimeCardsDateFilter
          startDate={startDate}
          endDate={endDate}
          onStartDateChange={handleStartDateChange}
          onEndDateChange={handleEndDateChange}
          onClear={handleClearDates}
          hasFilters={hasDateFilters}
        />
      </Box>

      <Box sx={{ mb: 2, display: "flex", alignItems: "center", justifyContent: "space-between" }}>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
          <Typography variant="body2" color="text.secondary">
            Showing {paginatedTimeCards.length} of {totalItems} time cards
          </Typography>
          {!isTimeCardsLoading && isTimeCardsFetching && (
            <CircularProgress size={14} />
          )}
        </Box>
        <TimeCardsCsvExportButton
          projectId={projectId}
          projectName={project?.name}
          filters={{
            ...(startDate && { startDate }),
            ...(endDate && { endDate }),
            states: ["Approved"],
          }}
          prefetchedCards={paginatedTimeCards}
          totalRecords={totalItems}
          disabled={isTimeCardsLoading || isTimeCardsError || totalItems === 0}
        />
      </Box>

      {isTimeCardsError ? (
        <TimeTrackingErrorState />
      ) : (
        <>
          <Grid container spacing={3}>
            {isTimeCardsLoading ? (
              Array.from({ length: 7 }).map((_, index) => (
                <Grid key={`skeleton-${index}`} size={12}>
                  <TimeTrackingCardSkeleton />
                </Grid>
              ))
            ) : paginatedTimeCards.length === 0 ? (
              <Grid size={12}>
                <EmptyState description="No time logs available." />
              </Grid>
            ) : (
              paginatedTimeCards.map((card) => (
                <Grid key={card.case.id} size={12}>
                  <TimeTrackingCard card={card} />
                </Grid>
              ))
            )}
          </Grid>

          {totalPages > 1 && (
            <Box sx={{ display: "flex", justifyContent: "flex-end", mt: 4 }}>
              <Pagination
                count={totalPages}
                page={page}
                onChange={handlePageChange}
                color="primary"
                variant="outlined"
                shape="rounded"
              />
            </Box>
          )}
        </>
      )}
    </Box>
  );
}
