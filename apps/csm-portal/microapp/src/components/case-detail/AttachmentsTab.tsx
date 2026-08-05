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

import { Suspense, useState, type ReactNode } from "react";
import { useQueryClient, useQueryErrorResetBoundary, useSuspenseQuery } from "@tanstack/react-query";
import { Button, Skeleton, Stack, Typography } from "@wso2/oxygen-ui";
import { Eye, FileText } from "@wso2/oxygen-ui-icons-react";
import { attachments as attachmentsService } from "@src/services/attachments";
import type { CaseAttachment } from "@src/types";
import { ErrorBoundary } from "@components/common/ErrorBoundary";
import { ListItemErrorBoundary } from "@components/common/ListItemErrorBoundary";
import { ErrorState } from "@components/support/ErrorState";
import { AttachmentsField } from "@components/support/AttachmentsField";
import { formatBytes, type PendingAttachment } from "@utils/attachments";
import { formatDate } from "@utils/dateTime";
import { Logger } from "@utils/logger";
import { AttachmentPreviewDialog } from "./AttachmentPreviewDialog";

/**
 * Full attachment list + upload for a case — the standalone flow (10 MB cap), distinct from the
 * comment composer's 5 MB inline attach (see CommentComposer.tsx / utils/attachments.ts).
 */
export function AttachmentsTab({ caseId }: { caseId: string }) {
  return (
    <AttachmentsTabErrorBoundary>
      <Suspense fallback={<AttachmentsTabSkeleton />}>
        <AttachmentsTabContent caseId={caseId} />
      </Suspense>
    </AttachmentsTabErrorBoundary>
  );
}

function AttachmentsTabContent({ caseId }: { caseId: string }) {
  const queryClient = useQueryClient();
  const { data: caseAttachments } = useSuspenseQuery(attachmentsService.forCase(caseId));
  const [pending, setPending] = useState<PendingAttachment[]>([]);
  const [isUploading, setIsUploading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [previewTarget, setPreviewTarget] = useState<CaseAttachment | null>(null);

  const handleUpload = async () => {
    if (pending.length === 0) return;
    setIsUploading(true);
    setError(null);
    const results = await Promise.allSettled(
      pending.map((attachment) =>
        attachmentsService.create({
          referenceId: caseId,
          referenceType: "case",
          name: attachment.name,
          type: attachment.type,
          file: attachment.file,
        }),
      ),
    );
    const failedCount = results.filter((r) => r.status === "rejected").length;
    if (failedCount > 0) {
      Logger.warn(`${failedCount} attachment(s) failed to upload to case ${caseId}`);
      setError(
        failedCount === pending.length
          ? "Could not upload the file(s). Please try again."
          : `${failedCount} file(s) failed to upload.`,
      );
    }
    // Keep only the files that actually failed, so a partial (or total) failure leaves them
    // selected for retry instead of silently dropping them along with the successful ones.
    // Indexes line up with `results` since the field is disabled (via isUploading) for the
    // duration of this upload, so `pending` can't have changed underneath us.
    setPending(pending.filter((_, i) => results[i].status === "rejected"));
    setIsUploading(false);
    void queryClient.invalidateQueries({ queryKey: ["case", caseId, "attachments"] });
  };

  return (
    <Stack gap={2}>
      <AttachmentsField attachments={pending} onChange={setPending} disabled={isUploading} />

      {pending.length > 0 && (
        <Button
          variant="contained"
          size="small"
          disabled={isUploading}
          onClick={() => void handleUpload()}
          sx={{ alignSelf: "end" }}
        >
          Upload
        </Button>
      )}

      {error && (
        <Typography variant="caption" color="error.main">
          {error}
        </Typography>
      )}

      {caseAttachments.length === 0 ? (
        <Typography variant="body2" color="text.secondary">
          No attachments on this case.
        </Typography>
      ) : (
        <Stack gap={1}>
          {caseAttachments.map((attachment) => (
            <ListItemErrorBoundary key={attachment.id} context="attachment row">
              <AttachmentRow attachment={attachment} onPreview={setPreviewTarget} />
            </ListItemErrorBoundary>
          ))}
        </Stack>
      )}

      <AttachmentPreviewDialog attachment={previewTarget} onClose={() => setPreviewTarget(null)} />
    </Stack>
  );
}

function AttachmentRow({
  attachment,
  onPreview,
}: {
  attachment: CaseAttachment;
  onPreview: (attachment: CaseAttachment) => void;
}) {
  return (
    <Stack
      direction="row"
      alignItems="center"
      justifyContent="space-between"
      gap={1}
      onClick={() => onPreview(attachment)}
      sx={{ p: 1, border: "1px solid", borderColor: "divider", borderRadius: 1, cursor: "pointer" }}
    >
      <Stack direction="row" alignItems="center" gap={1} sx={{ minWidth: 0 }}>
        <FileText size={16} />
        <Stack sx={{ minWidth: 0 }}>
          <Typography variant="body2" noWrap>
            {attachment.name}
          </Typography>
          <Typography variant="caption" color="text.secondary" noWrap sx={{ fontSize: "0.7rem" }}>
            {formatBytes(attachment.sizeBytes)} · {attachment.createdBy} · {formatDate(attachment.createdOn)}
          </Typography>
        </Stack>
      </Stack>
      {/* Whole row opens Preview, same as customer-portal microapp's AttachmentCard — no separate
          Download action (neither app's native bridge has a "save file to device" primitive, only
          a small JSON-value local store unrelated to files). Unsupported types still open the
          dialog; it renders its own "Preview not available" state for those rather than leaving
          the row with no action at all. */}
      <Eye size={16} />
    </Stack>
  );
}

function AttachmentsTabSkeleton() {
  return (
    <Stack gap={1}>
      <Skeleton variant="rounded" height={56} />
      <Skeleton variant="rounded" height={56} />
    </Stack>
  );
}

function AttachmentsTabErrorBoundary({ children }: { children: ReactNode }) {
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
