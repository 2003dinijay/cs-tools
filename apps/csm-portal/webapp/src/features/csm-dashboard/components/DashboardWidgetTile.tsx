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

import { Card, Skeleton, Typography } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import type { BeCaseSearchFilters } from "@api/backend/types";
import { useWidgetCaseCount } from "@features/csm-dashboard/api/useWidgetCaseCount";

interface DashboardWidgetTileProps {
  widgetId: string;
  displayName: string;
  filters: BeCaseSearchFilters;
}

/**
 * Single "single_score" dashboard widget tile: fetches and renders its own
 * count independently of any sibling tile, so one widget's loading/error
 * state never affects another's.
 */
export default function DashboardWidgetTile({
  widgetId,
  displayName,
  filters,
}: DashboardWidgetTileProps): JSX.Element {
  const {
    data: count,
    isLoading,
    isError,
  } = useWidgetCaseCount(widgetId, filters);

  return (
    <Card variant="outlined" sx={{ p: 1.75 }}>
      {isLoading ? (
        <Skeleton variant="rounded" height={48} />
      ) : isError ? (
        <Typography variant="body2" color="text.secondary">
          Could not load this widget.
        </Typography>
      ) : (
        <>
          <Typography variant="caption" color="text.secondary">
            {displayName}
          </Typography>
          <Typography variant="h5" sx={{ mt: 0.5 }}>
            {count}
          </Typography>
        </>
      )}
    </Card>
  );
}
