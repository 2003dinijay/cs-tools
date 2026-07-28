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

import {
  Box,
  CircularProgress,
  Dialog,
  DialogContent,
  DialogTitle,
  IconButton,
  Typography,
} from "@wso2/oxygen-ui";
import { X } from "@wso2/oxygen-ui-icons-react";
import { useEffect, useState, type JSX } from "react";
import type { CaseAttachment } from "@features/csm-cases/types/csmCases";
import { getAttachmentPreviewKind } from "@features/csm-cases/utils/attachmentPreview";

interface AttachmentPreviewDialogProps {
  /** Attachment being previewed; the dialog is closed when this is null. */
  attachment: CaseAttachment | null;
  onClose: () => void;
  /**
   * Fetch the attachment's raw bytes. The BE content endpoint always sets
   * `Content-Disposition: attachment` and requires auth headers, so a plain
   * `<img src>` pointed at it would force a download instead of rendering —
   * the bytes are fetched here as a `Blob` and turned into an object URL for
   * the preview element instead.
   */
  fetchContent: (attachment: CaseAttachment) => Promise<Blob>;
}

/**
 * Inline preview for image/PDF attachments (the two families in the
 * backend's safe-content-type allowlist that make sense to render inline —
 * see {@link getAttachmentPreviewKind}). Fetches the attachment's
 * bytes via `fetchContent` (the same authenticated content endpoint the
 * download action uses) and renders them from a `blob:` object URL, which is
 * revoked on close/unmount to avoid leaking memory.
 */
export default function AttachmentPreviewDialog({
  attachment,
  onClose,
  fetchContent,
}: AttachmentPreviewDialogProps): JSX.Element {
  const [objectUrl, setObjectUrl] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Track which attachment the state above belongs to, and reconcile it
  // *synchronously during render* the moment `attachment` changes (React's
  // "adjusting state when a prop changes" pattern). Effects only run after
  // the render has already committed to the DOM, so resetting this state
  // from inside a `useEffect` leaves a frame where the dialog still paints
  // the *previous* attachment's stale objectUrl/error before the effect
  // fires — a visible flash of the wrong preview. Comparing here and calling
  // the setters directly in the render body avoids that: React re-renders
  // immediately with the reset state before the browser paints anything.
  const [renderedFor, setRenderedFor] = useState(attachment);
  if (attachment !== renderedFor) {
    setRenderedFor(attachment);
    setObjectUrl(null);
    setError(null);
    setLoading(!!attachment);
  }

  useEffect(() => {
    if (!attachment) return;

    let cancelled = false;
    let createdUrl: string | null = null;

    void fetchContent(attachment)
      .then((blob) => {
        if (cancelled) return;
        createdUrl = URL.createObjectURL(blob);
        setObjectUrl(createdUrl);
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setError(
            err instanceof Error ? err.message : "Could not load the preview.",
          );
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => {
      cancelled = true;
      if (createdUrl) URL.revokeObjectURL(createdUrl);
    };
    // `fetchContent` is a stable useCallback from the caller's hook.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [attachment]);

  const kind = attachment ? getAttachmentPreviewKind(attachment.contentType) : null;
  const open = !!attachment;

  return (
    <Dialog
      open={open}
      onClose={onClose}
      maxWidth="md"
      fullWidth
      aria-label={attachment ? `Preview ${attachment.filename}` : "Preview"}
    >
      <DialogTitle
        sx={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          gap: 1,
        }}
      >
        <Typography
          component="span"
          variant="subtitle1"
          noWrap
          sx={{ minWidth: 0 }}
        >
          {attachment?.filename}
        </Typography>
        <IconButton size="small" onClick={onClose} aria-label="Close preview">
          <X size={18} />
        </IconButton>
      </DialogTitle>
      <DialogContent
        sx={{
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          minHeight: 320,
          bgcolor: "action.hover",
        }}
      >
        {loading ? (
          <CircularProgress size={28} />
        ) : error ? (
          <Typography variant="body2" color="error">
            {error}
          </Typography>
        ) : objectUrl && kind === "image" ? (
          <Box
            component="img"
            src={objectUrl}
            alt={attachment?.filename}
            sx={{
              maxWidth: "100%",
              maxHeight: "70vh",
              width: "auto",
              height: "auto",
              objectFit: "contain",
            }}
          />
        ) : objectUrl && kind === "pdf" ? (
          <Box
            component="iframe"
            src={objectUrl}
            title={attachment?.filename}
            // The PDF bytes come from an uploaded attachment, so treat this
            // as untrusted content even though it is served from a
            // same-origin `blob:` URL. `allow-same-origin` is required for
            // Chrome/Firefox's built-in PDF viewer to render at all inside a
            // sandboxed iframe (verified: without it the viewer shows a
            // broken-plugin placeholder instead of the PDF). Deliberately NOT
            // combined with `allow-scripts`: `allow-same-origin` +
            // `allow-scripts` together is a known sandbox-defeating pattern
            // (the frame gets both a real origin and the ability to run
            // script, so any embedded PDF JS action could reach this page's
            // own origin/session). The browser's built-in PDF viewer is a
            // native renderer, not scripted web content, so it doesn't need
            // `allow-scripts` to display/scroll/zoom a PDF; `allow-forms`
            // isn't needed for a read-only preview either.
            sandbox="allow-same-origin"
            sx={{ width: "100%", height: "70vh", border: 0 }}
          />
        ) : null}
      </DialogContent>
    </Dialog>
  );
}
