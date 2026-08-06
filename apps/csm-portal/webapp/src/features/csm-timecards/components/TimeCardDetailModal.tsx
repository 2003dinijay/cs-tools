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

import type { JSX } from "react";
import { Link as RouterLink } from "react-router";
import { Box, Button, Dialog, DialogActions, DialogContent, DialogTitle, Typography } from "@wso2/oxygen-ui";
import RelativeTime from "@components/RelativeTime";
import TimeCardStatusChip from "@features/csm-timecards/components/TimeCardStatusChip";
import { billableLabel } from "@features/csm-timecards/constants/timeCardConstants";
import { decisionSummary } from "@features/csm-timecards/utils/timeCardDecision";
import { isBlankHtml, sanitizeRichTextHtml } from "@utils/sanitizeHtml";
import type { CsmTimeCard } from "@features/csm-timecards/types/timeCards";

interface TimeCardDetailModalProps {
  card: CsmTimeCard;
  onClose: () => void;
}

function Field({ label, value }: { label: string; value: JSX.Element | string }): JSX.Element {
  return (
    <Box sx={{ minWidth: 0, display: "flex", flexDirection: "column", gap: 0.5 }}>
      <Typography variant="caption" color="text.secondary">
        {label}
      </Typography>
      {typeof value === "string" ? <Typography variant="body2">{value}</Typography> : value}
    </Box>
  );
}

/**
 * The engineer's own work-log comment from submission — ServiceNow rich-text
 * HTML, so it's sanitized and rendered as HTML (same policy as a case
 * comment's body). Renders nothing when the card has none (e.g. logged
 * before this field was mapped).
 */
function WorkLogComment({ html }: { html?: string }): JSX.Element | null {
  if (!html || isBlankHtml(html)) return null;
  const safeHtml = sanitizeRichTextHtml(html);
  return (
    <Field
      label="Engineer's comment"
      value={
        <Box
          sx={{
            fontSize: "0.875rem",
            lineHeight: 1.5,
            wordBreak: "break-word",
            "& p": { my: 0.5 },
            "& p:first-of-type": { mt: 0 },
            "& p:last-child": { mb: 0 },
            "& ul, & ol": { my: 0.5, pl: 3 },
            "& a": { color: "primary.main" },
            "& img": { maxWidth: "100%", height: "auto" },
          }}
          dangerouslySetInnerHTML={{ __html: safeHtml }}
        />
      }
    />
  );
}

/**
 * Read-only detail view of a single time card, opened via the eye icon in
 * `TimeCardsTable`'s Actions column. Shows only what the backend actually
 * returns on read — issue complexity and the per-activity minute breakdown
 * are accepted on create but never echoed back, so (like
 * `TimeCardReviewDialog`) they aren't shown here.
 */
export default function TimeCardDetailModal({ card, onClose }: TimeCardDetailModalProps): JSX.Element {
  const decision = decisionSummary(card);

  return (
    <Dialog open onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>
        Time card · {card.caseNumber} · {card.totalMinutes} min
      </DialogTitle>
      <DialogContent dividers>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
          <Box sx={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 2 }}>
            <Field
              label="Case"
              value={
                <Typography
                  variant="body2"
                  component={RouterLink}
                  to={`/cases/${card.caseId}`}
                  sx={{
                    display: "block",
                    color: "primary.main",
                    textDecoration: "none",
                    "&:hover": { textDecoration: "underline" },
                  }}
                >
                  {card.caseNumber}
                </Typography>
              }
            />
            <Field label="Engineer" value={card.userName} />
            <Field label="Project" value={card.projectName} />
            <Field label="Billable" value={billableLabel(card.billable)} />
            <Field label="Logged" value={<Typography variant="body2"><RelativeTime iso={card.workDate} /></Typography>} />
            <Field label="Minutes" value={`${card.totalMinutes} min`} />
          </Box>

          <Field label="State" value={<Box sx={{ mt: 0.5 }}><TimeCardStatusChip state={card.state} /></Box>} />

          <WorkLogComment html={card.workLogComment} />

          {decision && (
            <Field
              label="Decision"
              value={
                <Typography variant="body2" sx={{ whiteSpace: "normal", wordBreak: "break-word" }}>
                  {decision}
                </Typography>
              }
            />
          )}
        </Box>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Close</Button>
      </DialogActions>
    </Dialog>
  );
}
