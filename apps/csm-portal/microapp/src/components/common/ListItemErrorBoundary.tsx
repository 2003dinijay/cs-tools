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

import type { ReactNode } from "react";
import { Box, Typography } from "@wso2/oxygen-ui";
import { TriangleAlert } from "@wso2/oxygen-ui-icons-react";
import { Logger } from "@utils/logger";
import { ErrorBoundary } from "./ErrorBoundary";

interface ListItemErrorBoundaryProps {
  /** Short label identifying what kind of item this wraps, for the log line only — never shown
   * to the user, e.g. "comment", "attachment row", "case card". */
  context: string;
  children: ReactNode;
}

/**
 * Wraps a single item in a list of externally-shaped data (a comment, an attachment row, a case
 * card, ...) so a render error in *one* item degrades to a small inline placeholder instead of
 * taking down the whole list. Without this, the list's own section-level ErrorBoundary (the
 * existing convention — AttachmentsTab, CaseListErrorBoundary, ...) is the nearest boundary above
 * a crashing item, and it unmounts *everything* below it, not just the offending item — every
 * other case, comment, or attachment included. This is exactly how one attachment with a bad
 * `createdBy` value took down the entire comments feed alongside it.
 *
 * Deliberately has no built-in retry: a render error here is a code/data-shape bug, not a
 * transient failure a tap can fix, and the item is keyed by its own id at the call site — a
 * corrected payload for that id (a refetch) still won't clear this boundary's error state until
 * the page reloads, same as every other ErrorBoundary already in this app.
 */
export function ListItemErrorBoundary({ context, children }: ListItemErrorBoundaryProps) {
  return (
    <ErrorBoundary
      onError={(error) => Logger.error(`Failed to render ${context}`, error)}
      fallback={
        <Box
          sx={{
            display: "flex",
            alignItems: "center",
            gap: 1,
            p: 1,
            border: "1px dashed",
            borderColor: "divider",
            borderRadius: 1,
            color: "text.disabled",
          }}
        >
          <TriangleAlert size={14} />
          <Typography variant="caption">Couldn't display this item.</Typography>
        </Box>
      }
    >
      {children}
    </ErrorBoundary>
  );
}
