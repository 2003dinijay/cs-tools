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

import { Chip, Typography } from "@wso2/oxygen-ui";
import type { JSX, ReactNode } from "react";
import { closureStatePresentation } from "@features/csm-projects/utils/projectLifecycle";

interface ClosureStateChipProps {
  closureState?: string | null;
  /**
   * Rendered in place of the chip when `closureState` doesn't resolve to a
   * known presentation. Defaults to the detail page's convention (a
   * `Typography` dash, matching its other `MetaCell` values); the list page
   * passes a bare `"—"` to match its plain-text table cells instead.
   */
  emptyFallback?: ReactNode;
}

const DEFAULT_EMPTY_FALLBACK = <Typography variant="body2">—</Typography>;

export default function ClosureStateChip({
  closureState,
  emptyFallback = DEFAULT_EMPTY_FALLBACK,
}: ClosureStateChipProps): JSX.Element {
  const closure = closureStatePresentation(closureState);
  if (!closure) return <>{emptyFallback}</>;
  return (
    <Chip
      size="small"
      label={closure.label}
      color={closure.severity === "default" ? undefined : closure.severity}
      variant="outlined"
    />
  );
}
