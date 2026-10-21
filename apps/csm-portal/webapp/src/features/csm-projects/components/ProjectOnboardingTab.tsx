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

import { Box, Button, Card, Typography } from "@wso2/oxygen-ui";
import { Pencil } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import ProjectMetaCell from "@features/csm-projects/components/ProjectMetaCell";
import ProjectOnboardingOwnerValue from "@features/csm-projects/components/ProjectOnboardingOwnerValue";
import ProjectOnboardingStatusChip from "@features/csm-projects/components/ProjectOnboardingStatusChip";
import type { ProjectDetails } from "@features/csm-projects/types/csmProjects";
import { formatUtcDateForDisplay } from "@utils/dateTime";

interface ProjectOnboardingTabProps {
  project: ProjectDetails;
  /** Show the Edit button (`canEditProjectOnboarding`); the backend is the
   *  real gate. */
  canEdit: boolean;
  onEdit: () => void;
}

/** The dates are calendar days stored at midnight UTC: show the UTC day, not
 *  the viewer's local one. */
function dateCell(value?: string | null): string {
  return formatUtcDateForDisplay(value) ?? "—";
}

/**
 * Project detail "Onboarding" tab: onboarding status, owner and the four
 * onboarding dates, read-only unless `canEdit`. The per-contact onboarding
 * step ledger is not repeated here; it already lives in the Project contacts
 * tab.
 */
export default function ProjectOnboardingTab({
  project,
  canEdit,
  onEdit,
}: ProjectOnboardingTabProps): JSX.Element {
  return (
    <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
      <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 1 }}>
        <Typography variant="subtitle2">Onboarding</Typography>
        {canEdit && (
          <Button
            variant="outlined"
            size="small"
            className="csm-print-hide"
            startIcon={<Pencil size={14} />}
            onClick={onEdit}
          >
            Edit
          </Button>
        )}
      </Box>
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
        <ProjectMetaCell label="Onboarding status">
          <ProjectOnboardingStatusChip
            onboardingStatus={project.onboardingStatus}
            emptyFallback={<Typography variant="body2">—</Typography>}
          />
        </ProjectMetaCell>
        <ProjectMetaCell label="Onboarding owner">
          <ProjectOnboardingOwnerValue owner={project.onboardingOwner} />
        </ProjectMetaCell>
        <ProjectMetaCell label="Onboarding start date">
          <Typography variant="body2">{dateCell(project.onboardingStartDate)}</Typography>
        </ProjectMetaCell>
        <ProjectMetaCell label="Go-live plan date">
          <Typography variant="body2">{dateCell(project.goLivePlanDate)}</Typography>
        </ProjectMetaCell>
        <ProjectMetaCell label="Go-live date">
          <Typography variant="body2">{dateCell(project.goLiveDate)}</Typography>
        </ProjectMetaCell>
        <ProjectMetaCell label="Onboarding expiry date">
          <Typography variant="body2">{dateCell(project.onboardingExpiryDate)}</Typography>
        </ProjectMetaCell>
      </Box>
    </Card>
  );
}
