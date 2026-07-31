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

import { Button, CircularProgress, Tooltip } from "@wso2/oxygen-ui";
import { Download } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { useSuccessBanner } from "@context/success-banner/SuccessBannerContext";
import {
  useFilteredCsvExport,
  type UseFilteredCsvExportConfig,
} from "@hooks/useFilteredCsvExport";

export interface FilteredCsvExportButtonProps<T> extends UseFilteredCsvExportConfig<T> {
  /** Disable the action entirely (e.g. the underlying list query errored, or
   * the caller already knows there's nothing to export). The button also
   * disables itself while an export is in flight regardless of this prop. */
  disabled?: boolean;
  /** Noun used in banner copy, e.g. "incidents", "cases". Defaults to
   * `entityName` with hyphens turned into spaces. */
  entityNounPlural?: string;
}

/**
 * "Export CSV" action for a filtered listing page. Pages the *entire*
 * currently filtered (and sorted) result set via `useFilteredCsvExport` — not
 * just the page currently rendered in the table — and downloads it as one
 * CSV file. Shows a busy state with live progress while paging (this can take
 * a while for a wide filter), and never hands the user a truncated file
 * without saying so plainly.
 */
export default function FilteredCsvExportButton<T>({
  disabled,
  entityNounPlural,
  ...config
}: FilteredCsvExportButtonProps<T>): JSX.Element {
  const { isExporting, progress, runExport } = useFilteredCsvExport(config);
  const { showError } = useErrorBanner();
  const { showSuccess } = useSuccessBanner();
  const noun = entityNounPlural ?? config.entityName.replace(/-/g, " ");

  const handleClick = async (): Promise<void> => {
    try {
      const { truncated, rowCount, total } = await runExport();
      if (truncated) {
        showError(
          `Export stopped at ${rowCount.toLocaleString()} of ${total.toLocaleString()} ${noun} — the file only contains the first ${rowCount.toLocaleString()}. Narrow your filters (e.g. a shorter date range or a specific product) and export again to get the rest.`,
        );
      } else {
        showSuccess(`Exported ${rowCount.toLocaleString()} ${noun} to CSV.`);
      }
    } catch (err) {
      showError(
        "Could not complete the export. No file was downloaded — please try again, or narrow your filters first.",
        err,
      );
    }
  };

  const label =
    isExporting && progress
      ? progress.total > 0
        ? `Exporting ${progress.loaded.toLocaleString()} of ${progress.total.toLocaleString()}…`
        : "Exporting…"
      : "Export CSV";

  const button = (
    <Button
      size="small"
      variant="text"
      startIcon={isExporting ? <CircularProgress size={14} /> : <Download size={14} />}
      disabled={disabled || isExporting}
      onClick={handleClick}
      aria-busy={isExporting}
    >
      {label}
    </Button>
  );

  // Only worth a tooltip while actively paging (the button's own label
  // already shows the count) — explains *why* it's taking a while for
  // anyone who wasn't watching the label tick up. The button is disabled in
  // this state, and a disabled element fires no pointer events for the
  // Tooltip to listen to — MUI's documented fix is a non-disabled wrapper.
  return isExporting ? (
    <Tooltip title="Fetching every page of the current filter before downloading — this can take a while for a wide filter.">
      <span>{button}</span>
    </Tooltip>
  ) : (
    button
  );
}
