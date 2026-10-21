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

import { Box, Card, IconButton, Skeleton, Typography } from "@wso2/oxygen-ui";
import { Pencil } from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX } from "react";
import { useGetTeamMembers } from "@features/csm-admin/api/useTeamMembers";
import { findTeamLead } from "@features/csm-admin/utils/teamLeadAccess";
import ReassignTeamLeadDialog from "@features/csm-admin/components/ReassignTeamLeadDialog";

interface TeamLeadCardProps {
  teamEntityId: string;
  /** Admin only — see `ReassignTeamLeadDialog`'s own doc comment for why: the
   * reassigned role is the real, paging-connected one the incident
   * call-escalation ladder reads. Hidden entirely for anyone else, the same
   * deliberate "Add User"-style exception this app's own CLAUDE.md
   * documents for an admin-only action with real backend enforcement. */
  isAdmin: boolean;
}

/**
 * Shows the team's current lead and, for an admin only, a control to
 * reassign it. The roster (`useGetTeamMembers`) is the one source of truth
 * for who currently holds the role — no separate "who is lead" endpoint
 * exists.
 */
export default function TeamLeadCard({
  teamEntityId,
  isAdmin,
}: TeamLeadCardProps): JSX.Element {
  const [reassignOpen, setReassignOpen] = useState(false);
  const { data: members, isLoading, isError } = useGetTeamMembers(teamEntityId);
  const lead = findTeamLead(members);

  return (
    <Card variant="outlined" sx={{ p: 2 }}>
      <Box sx={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between" }}>
        <Box>
          <Typography variant="subtitle2">Team lead</Typography>
          <Typography variant="caption" color="text.secondary">
            Also who entity-service's incident escalation ladder pages for
            this team.
          </Typography>
        </Box>
        {isAdmin && (
          <IconButton
            size="small"
            aria-label="Change team lead"
            onClick={() => setReassignOpen(true)}
          >
            <Pencil size={14} />
          </IconButton>
        )}
      </Box>

      <Box sx={{ mt: 1.5 }}>
        {isLoading ? (
          <Skeleton variant="text" width={180} />
        ) : isError ? (
          <Typography variant="body2" color="text.secondary">
            Couldn't load the team's roster.
          </Typography>
        ) : lead ? (
          <Typography variant="body2">
            {lead.name}
            {lead.email ? ` (${lead.email})` : ""}
          </Typography>
        ) : (
          <Typography variant="body2" color="text.secondary">
            No lead assigned
          </Typography>
        )}
      </Box>

      {reassignOpen && (
        <ReassignTeamLeadDialog
          teamEntityId={teamEntityId}
          currentLeadName={lead?.name}
          onClose={() => setReassignOpen(false)}
          onAssigned={() => setReassignOpen(false)}
        />
      )}
    </Card>
  );
}
