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
  DatePickers,
  FormControl,
  InputLabel,
  MenuItem,
  Select,
  Stack,
  TextField,
} from "@wso2/oxygen-ui";
import type { CatalogItemVariableDto } from "@src/types";
import {
  formatDateTimeLocal,
  isChoiceField,
  isDateTimeField,
  isDescriptionField,
  isFileCopyPathField,
  isMultiLineField,
  parseDateTimeLocal,
  variableLabel,
} from "@utils/catalogVariables";

const { DateTimePicker, LocalizationProvider } = DatePickers;

interface CatalogVariableFieldsProps {
  /** Already filtered to user-editable variables (no context/hidden fields). */
  variables: CatalogItemVariableDto[];
  values: Record<string, string>;
  onChange: (id: string, value: string) => void;
  disabled?: boolean;
}

// Mobile counterpart of the webapp's CatalogVariableFields.tsx: single-column Stack rather than a
// responsive Grid, and the Description field is a plain multiline TextField rather than a
// rich-text editor (see utils/catalogVariables.ts's file comment for why). Otherwise renders the
// same control per variable type/label: Yes/No select, multi-line text, datetime picker, or
// single-line text.
export function CatalogVariableFields({ variables, values, onChange, disabled }: CatalogVariableFieldsProps) {
  return (
    <LocalizationProvider dateAdapter={AdapterDateFns}>
      <Stack gap={2}>
        {variables.map((v) => {
          const value = values[v.id] ?? "";
          const label = variableLabel(v);

          if (isChoiceField(v)) {
            const labelId = `sr-var-${v.id}-label`;
            return (
              <FormControl key={v.id} size="small" fullWidth required disabled={disabled}>
                <InputLabel id={labelId}>{label}</InputLabel>
                <Select
                  labelId={labelId}
                  label={label}
                  value={value}
                  onChange={(e) => onChange(v.id, String(e.target.value))}
                >
                  <MenuItem value="Yes">Yes</MenuItem>
                  <MenuItem value="No">No</MenuItem>
                </Select>
              </FormControl>
            );
          }

          if (isDescriptionField(v.questionText ?? "")) {
            return (
              <TextField
                key={v.id}
                label={label}
                size="small"
                fullWidth
                required
                multiline
                minRows={5}
                disabled={disabled}
                value={value}
                onChange={(e) => onChange(v.id, e.target.value)}
              />
            );
          }

          if (isMultiLineField(v)) {
            return (
              <TextField
                key={v.id}
                label={label}
                size="small"
                fullWidth
                required
                multiline
                minRows={4}
                disabled={disabled}
                value={value}
                onChange={(e) => onChange(v.id, e.target.value)}
              />
            );
          }

          if (isDateTimeField(v)) {
            return (
              <DateTimePicker
                key={v.id}
                label={label}
                value={parseDateTimeLocal(value)}
                disabled={disabled}
                onChange={(next) =>
                  onChange(v.id, next instanceof Date && !Number.isNaN(next.getTime()) ? formatDateTimeLocal(next) : "")
                }
                slotProps={{
                  textField: { size: "small", fullWidth: true, required: true },
                  field: { clearable: true },
                }}
              />
            );
          }

          // File Copy Path is an optional plain text input.
          const optional = isFileCopyPathField(v);
          return (
            <TextField
              key={v.id}
              label={label}
              size="small"
              fullWidth
              required={!optional}
              disabled={disabled}
              value={value}
              onChange={(e) => onChange(v.id, e.target.value)}
            />
          );
        })}
      </Stack>
    </LocalizationProvider>
  );
}
