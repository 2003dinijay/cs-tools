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

import { Box, Typography } from "@wso2/oxygen-ui";
import { useMemo, type JSX } from "react";
import { sanitizeRichTextHtml } from "@utils/sanitizeHtml";

/**
 * Shows how HTML typed in the source box will look, without touching it.
 *
 * The rich editor cannot be used for this when the markup has styling it cannot
 * represent (a bordered box, coloured links): loading it there rewrites it. So
 * the source view gets its own read-only rendering instead, prepared the way
 * the portals prepare backend HTML for display: DOMPurify's default policy, the
 * same as the announcement views, with the styling left as written (including
 * in dark mode, so the preview matches what the announcement will look like).
 * Scripts, event handlers and `javascript:` URLs are removed, exactly as they
 * are for customers.
 */
export default function HtmlSourcePreview({ html }: { html: string }): JSX.Element | null {
  const safeHtml = useMemo(() => sanitizeRichTextHtml(html), [html]);

  if (!html.trim()) return null;

  return (
    <Box sx={{ mt: 1.5 }}>
      <Typography variant="caption" color="text.secondary" sx={{ display: "block", mb: 0.5 }}>
        Preview: how this will look when shown to customers
      </Typography>
      <Box
        data-testid="html-source-preview"
        sx={{
          border: 1,
          borderColor: "divider",
          borderRadius: 1,
          p: 1.5,
          maxHeight: 420,
          overflow: "auto",
          overflowWrap: "anywhere",
          fontSize: "0.875rem",
        }}
        // Sanitised above with the same DOMPurify policy the portals use to show backend HTML.
        dangerouslySetInnerHTML={{ __html: safeHtml }}
      />
    </Box>
  );
}
