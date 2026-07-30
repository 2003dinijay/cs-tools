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
  AdapterDateFns,
  Box,
  Button,
  DatePickers,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControlLabel,
  Switch,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { useState, type JSX } from "react";
import { formatDateOnly, isPastDateOnly, parseDateOnly } from "@utils/dateTime";

const { DesktopDatePicker: DatePicker, LocalizationProvider } = DatePickers;

/** Combined `PATCH /cases/{id}` payload this dialog can produce in one save. */
export interface FixEtaSavePayload {
  bestCaseFixEta?: string;
  mostLikelyFixEta?: string;
  worstCaseFixEta?: string;
  addPublicComment?: boolean;
  product?: string;
  publicTicket?: string;
}

interface SetFixEtaDialogProps {
  /** Current internal-only best-case estimate, if any (date-only "YYYY-MM-DD"). */
  currentBestCaseFixEta?: string | null;
  /** Current internal-only most-likely estimate, if any (date-only "YYYY-MM-DD"). */
  currentMostLikelyFixEta?: string | null;
  /** Current internal-only worst-case estimate, if any (date-only "YYYY-MM-DD"). */
  currentWorstCaseFixEta?: string | null;
  /** True while the combined PATCH is in flight; disables the whole form. */
  isSaving: boolean;
  onClose: () => void;
  /** Apply the combined patch in one `PATCH` call. */
  onSave: (patch: FixEtaSavePayload) => void;
}

interface FixEtaDatePickerProps {
  label: string;
  value: string;
  disabled: boolean;
  onChange: (valueDateOnly: string) => void;
}

/**
 * One date-only picker in the combined form. Unlike the old per-row layout,
 * this no longer owns a Save action — the whole dialog saves together.
 */
function FixEtaDatePicker({
  label,
  value,
  disabled,
  onChange,
}: FixEtaDatePickerProps): JSX.Element {
  const parsed = parseDateOnly(value);
  // Non-blocking: a past estimate is unusual but not forbidden (e.g. logging
  // an estimate that was already missed), so this only warns, unlike the
  // hard-block some other pickers apply to a past value.
  const isPast = isPastDateOnly(parsed);

  return (
    <LocalizationProvider dateAdapter={AdapterDateFns}>
      <DatePicker
        label={label}
        value={parsed}
        onChange={(next) =>
          onChange(
            next instanceof Date && !Number.isNaN(next.getTime())
              ? formatDateOnly(next)
              : "",
          )
        }
        disabled={disabled}
        slotProps={{
          textField: {
            fullWidth: true,
            size: "small",
            helperText: isPast ? "This date is in the past." : undefined,
          },
        }}
      />
    </LocalizationProvider>
  );
}

/**
 * Set the case's three independent internal-only fix-ETA estimates
 * (`bestCaseFixEta` / `mostLikelyFixEta` / `worstCaseFixEta`) and, optionally,
 * post a customer-visible comment summarizing them in the same `PATCH`
 * (`addPublicComment` + `product` + `publicTicket`; see `BeCaseUpdatePayload`).
 * The three dates remain independently optional — saving one doesn't require
 * the others — but they now share a single Save action instead of three.
 * ServiceNow-source only; the caller surfaces a rejection on another source.
 */
export default function SetFixEtaDialog({
  currentBestCaseFixEta,
  currentMostLikelyFixEta,
  currentWorstCaseFixEta,
  isSaving,
  onClose,
  onSave,
}: SetFixEtaDialogProps): JSX.Element {
  const [bestCaseFixEta, setBestCaseFixEta] = useState(
    currentBestCaseFixEta ?? "",
  );
  const [mostLikelyFixEta, setMostLikelyFixEta] = useState(
    currentMostLikelyFixEta ?? "",
  );
  const [worstCaseFixEta, setWorstCaseFixEta] = useState(
    currentWorstCaseFixEta ?? "",
  );
  const [shareWithCustomer, setShareWithCustomer] = useState(false);
  const [product, setProduct] = useState("");
  const [publicTicket, setPublicTicket] = useState("");

  const hasAnyEta = !!(bestCaseFixEta || mostLikelyFixEta || worstCaseFixEta);
  const hasProduct = product.trim().length > 0;
  const hasPublicTicket = publicTicket.trim().length > 0;

  // Mirrors the backend's validation for `addPublicComment: true`: a public
  // comment must summarize at least one estimate, and needs a product +
  // ticket reference to be meaningful to the customer.
  const shareValidationError = shareWithCustomer
    ? !hasAnyEta
      ? "Pick at least one fix ETA to share with the customer."
      : !hasProduct
        ? "Product is required to share with the customer."
        : !hasPublicTicket
          ? "Public ticket is required to share with the customer."
          : undefined
    : undefined;

  const canSubmit =
    (hasAnyEta || shareWithCustomer) && !shareValidationError;

  const handleSave = (): void => {
    if (!canSubmit) return;
    onSave({
      ...(bestCaseFixEta && { bestCaseFixEta }),
      ...(mostLikelyFixEta && { mostLikelyFixEta }),
      ...(worstCaseFixEta && { worstCaseFixEta }),
      ...(shareWithCustomer && {
        addPublicComment: true,
        product: product.trim(),
        publicTicket: publicTicket.trim(),
      }),
    });
  };

  return (
    <Dialog open onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>Set fix ETA</DialogTitle>
      <DialogContent>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 0.5 }}>
          <Typography variant="body2" color="text.secondary">
            Internal-only estimates. Fill in as many as you have — none are
            required on their own — then save them together.
          </Typography>

          <FixEtaDatePicker
            label="Best case"
            value={bestCaseFixEta}
            disabled={isSaving}
            onChange={setBestCaseFixEta}
          />
          <FixEtaDatePicker
            label="Most likely"
            value={mostLikelyFixEta}
            disabled={isSaving}
            onChange={setMostLikelyFixEta}
          />
          <FixEtaDatePicker
            label="Worst case"
            value={worstCaseFixEta}
            disabled={isSaving}
            onChange={setWorstCaseFixEta}
          />

          <FormControlLabel
            control={
              <Switch
                checked={shareWithCustomer}
                onChange={(e) => setShareWithCustomer(e.target.checked)}
                disabled={isSaving}
              />
            }
            label="Share fix ETA with customer"
          />

          {shareWithCustomer && (
            <>
              <Typography variant="caption" color="text.secondary">
                Posts a customer-visible comment on this case summarizing the
                estimate(s) above.
              </Typography>
              <TextField
                label="Product"
                size="small"
                fullWidth
                required
                value={product}
                onChange={(e) => setProduct(e.target.value)}
                disabled={isSaving}
              />
              <TextField
                label="Public ticket"
                placeholder="e.g. a public GitHub issue URL"
                size="small"
                fullWidth
                required
                value={publicTicket}
                onChange={(e) => setPublicTicket(e.target.value)}
                disabled={isSaving}
              />
            </>
          )}

          {shareValidationError && (
            <Typography variant="caption" color="error">
              {shareValidationError}
            </Typography>
          )}
        </Box>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} disabled={isSaving}>
          Close
        </Button>
        <Button
          variant="contained"
          disabled={!canSubmit || isSaving}
          loading={isSaving}
          onClick={handleSave}
        >
          Save
        </Button>
      </DialogActions>
    </Dialog>
  );
}
