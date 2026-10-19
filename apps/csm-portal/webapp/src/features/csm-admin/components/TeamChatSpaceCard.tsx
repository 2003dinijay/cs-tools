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
import { useGetTeam, useUpdateTeam } from "@features/csm-admin/api/useTeamChatWebhook";
import EditTeamChatWebhookDialog from "@features/csm-admin/components/EditTeamChatWebhookDialog";

interface TeamChatSpaceCardProps {
  teamEntityId: string;
  /** True for an Asgardeo admin or this specific team's own lead — the only
   * two audiences who may view or change this value (a Chat webhook URL is
   * a bearer credential once set). Hidden from everyone else, the same
   * "Add User"-style deliberate exception to the show-and-reject default —
   * see this app's own CLAUDE.md. The backend enforces the identical rule
   * on both GET and PATCH regardless of what this prop is given. */
  canManage: boolean;
}

/**
 * A team's own Google Chat space link, shown and editable only to an admin
 * or that team's own lead. Not rendered at all (by the caller) for anyone
 * else — see `canManage`'s own doc comment.
 */
export default function TeamChatSpaceCard({
  teamEntityId,
  canManage,
}: TeamChatSpaceCardProps): JSX.Element | null {
  const [editOpen, setEditOpen] = useState(false);
  const { data, isLoading, isError } = useGetTeam(teamEntityId, {
    enabled: canManage,
  });
  const update = useUpdateTeam(teamEntityId);

  if (!canManage) return null;

  const chatWebhookUrl = data?.chatWebhookUrl ?? null;

  return (
    <Card variant="outlined" sx={{ p: 2 }}>
      <Box sx={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between" }}>
        <Box>
          <Typography variant="subtitle2">Chat space</Typography>
          <Typography variant="caption" color="text.secondary">
            The team's own Google Chat incoming-webhook link.
          </Typography>
        </Box>
        <IconButton
          size="small"
          aria-label="Edit Chat space link"
          onClick={() => {
            update.reset();
            setEditOpen(true);
          }}
        >
          <Pencil size={14} />
        </IconButton>
      </Box>

      <Box sx={{ mt: 1.5 }}>
        {isLoading ? (
          <Skeleton variant="text" width={220} />
        ) : isError ? (
          <Typography variant="body2" color="text.secondary">
            Couldn't load the Chat space link.
          </Typography>
        ) : chatWebhookUrl ? (
          <Typography
            variant="body2"
            sx={{ wordBreak: "break-all", fontFamily: "monospace" }}
          >
            {chatWebhookUrl}
          </Typography>
        ) : (
          <Typography variant="body2" color="text.secondary">
            Not configured
          </Typography>
        )}
      </Box>

      {editOpen && (
        <EditTeamChatWebhookDialog
          currentUrl={chatWebhookUrl}
          isSaving={update.isPending}
          saveError={update.isError ? update.error.message : null}
          onClose={() => {
            if (!update.isPending) setEditOpen(false);
          }}
          onSave={(chatWebhookUrl) =>
            update.mutate(chatWebhookUrl, {
              onSuccess: () => setEditOpen(false),
            })
          }
        />
      )}
    </Card>
  );
}
