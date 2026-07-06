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
  Checkbox,
  FormControl,
  InputLabel,
  ListItemText,
  MenuItem,
  Select,
  Tooltip,
  type SelectChangeEvent,
} from "@wso2/oxygen-ui";
import type { JSX } from "react";

export interface MultiSelectFieldProps<T extends string> {
  id: string;
  label: string;
  values: T[];
  options: { value: T; label: string }[];
  onChange: (next: T[]) => void;
  disabled?: boolean;
}

/**
 * Checkbox multi-select for a fixed, small set of options (e.g. an enum) —
 * pairs with {@link SearchableMultiSelect} for larger/dynamic option lists.
 * Extracted from `CasesFilterBar.tsx` so other feature filter bars (e.g.
 * time cards) can reuse the same look and behavior instead of a plain
 * free-text field.
 */
export default function MultiSelectField<T extends string>({
  id,
  label,
  values,
  options,
  onChange,
  disabled,
}: MultiSelectFieldProps<T>): JSX.Element {
  const handleChange = (event: SelectChangeEvent<string[]>): void => {
    const val = event.target.value;
    onChange((Array.isArray(val) ? val : [val]) as T[]);
  };
  return (
    <FormControl fullWidth size="small" disabled={disabled}>
      <InputLabel id={`${id}-label`}>{label}</InputLabel>
      <Select
        multiple
        labelId={`${id}-label`}
        id={id}
        value={values as unknown as string[]}
        label={label}
        onChange={handleChange}
        renderValue={(selected) => {
          if (!Array.isArray(selected) || selected.length === 0) return "";
          const labels = selected.map(
            (v) => options.find((o) => o.value === v)?.label ?? v,
          );
          const text = labels.join(", ");
          if (labels.length === 1) return text;
          return (
            <Tooltip title={text} placement="top">
              <Box
                component="span"
                sx={{
                  display: "block",
                  overflow: "hidden",
                  textOverflow: "ellipsis",
                  whiteSpace: "nowrap",
                }}
              >
                {text}
              </Box>
            </Tooltip>
          );
        }}
      >
        {options.length === 0 ? (
          <MenuItem disabled value="">
            <em>No options</em>
          </MenuItem>
        ) : (
          options.map((opt) => (
            <MenuItem key={opt.value} value={opt.value} sx={{ py: 0.5 }}>
              <Checkbox
                size="small"
                checked={values.includes(opt.value)}
                sx={{ mr: 1, p: 0.25 }}
              />
              <ListItemText primary={opt.label} />
            </MenuItem>
          ))
        )}
      </Select>
    </FormControl>
  );
}
