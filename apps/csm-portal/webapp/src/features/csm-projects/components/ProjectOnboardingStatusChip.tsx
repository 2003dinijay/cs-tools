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

import { Chip } from "@wso2/oxygen-ui";
import type { JSX, ReactNode } from "react";
import { projectOnboardingStatusPresentation } from "@features/csm-projects/utils/projectLifecycle";

interface ProjectOnboardingStatusChipProps {
  onboardingStatus?: string | null;
  /**
   * Rendered in place of the chip when there's no onboarding engagement to
   * show (missing status, or `"Not-Applicable"`). Defaults to `null`
   * (renders nothing) — `CsmProjectDetailPage.tsx` gates this cell's whole
   * `MetaCell` on the same condition, so an empty-string fallback there
   * would just be a dash next to no label, worse than omitting the row
   * entirely. The projects table passes a bare `"—"` to match its other
   * plain-text cells instead, the same convention `ClosureStateChip` uses.
   */
  emptyFallback?: ReactNode;
}

const DEFAULT_EMPTY_FALLBACK: ReactNode = null;

/**
 * Chip for a project's `onboardingStatus`. Renders `emptyFallback` for a
 * project with no onboarding engagement (missing status, or
 * `"Not-Applicable"`).
 */
export default function ProjectOnboardingStatusChip({
  onboardingStatus,
  emptyFallback = DEFAULT_EMPTY_FALLBACK,
}: ProjectOnboardingStatusChipProps): JSX.Element {
  const presentation = projectOnboardingStatusPresentation(onboardingStatus);
  if (!presentation) return <>{emptyFallback}</>;
  return (
    <Chip
      size="small"
      label={presentation.label}
      color={presentation.severity === "default" ? undefined : presentation.severity}
      variant="outlined"
    />
  );
}
