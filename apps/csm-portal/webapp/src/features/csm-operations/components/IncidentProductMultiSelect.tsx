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
  Autocomplete,
  Box,
  Checkbox,
  ListItemText,
  TextField,
  Tooltip,
} from "@wso2/oxygen-ui";
import { useMemo, type JSX } from "react";
import type * as React from "react";
import { useIncidentProductNameOptions } from "@features/csm-operations/api/useIncidentProductNameOptions";

interface IncidentProductMultiSelectProps {
  id?: string;
  label?: string;
  /** Selected service/product names (the filter values themselves). */
  values: string[];
  onChange: (next: string[]) => void;
}

/**
 * Product filter for the incidents list. Incidents carry no product
 * dimension of their own — this matches on the name of the *service* the
 * incident relates to, which is only ~43% populated and mixes real products
 * with customer names and service categories (an explicit, accepted
 * trade-off — see `useIncidentProductNameOptions`), so the helper text below
 * the field says so where a user filtering will actually see it. Modeled on
 * the cases list's `ProductNameMultiSelect`: the bounded catalogue is fetched
 * once and filtered locally as the user types; already-selected names are
 * kept in the option pool so their chips render even before the fetch
 * resolves.
 */
export default function IncidentProductMultiSelect({
  id = "incident-filter-product",
  label = "Product",
  values,
  onChange,
}: IncidentProductMultiSelectProps): JSX.Element {
  const { data, isFetching, isError } = useIncidentProductNameOptions();

  const options: string[] = useMemo(() => {
    const merged = new Set<string>(data ?? []);
    values.forEach((v) => merged.add(v));
    return [...merged].sort((a, b) => a.localeCompare(b));
  }, [data, values]);

  return (
    <Autocomplete<string, true>
      multiple
      size="small"
      id={id}
      options={options}
      value={values}
      loading={isFetching && !data}
      disableCloseOnSelect
      sx={{ "& .MuiAutocomplete-inputRoot": { flexWrap: "nowrap" } }}
      getOptionLabel={(opt) => opt}
      isOptionEqualToValue={(opt, val) => opt === val}
      onChange={(_event, next) => onChange(next)}
      noOptionsText={
        isError
          ? "Couldn't load products. Try again."
          : isFetching
            ? "Loading products…"
            : "No products found"
      }
      renderTags={(value) => {
        const displayText = value.join(", ");
        const content = (
          <Box
            component="span"
            sx={{ flex: "1 1 0", minWidth: 0, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}
          >
            {displayText}
          </Box>
        );
        // Unconditional: a single product name is truncated by the same
        // ellipsis styling as a multi-value list, so skipping the tooltip for
        // one value left a long name with no way to read it in full.
        return (
          <Tooltip title={displayText} placement="top">{content}</Tooltip>
        );
      }}
      renderOption={(props, option, { selected }) => {
        const { key, ...liProps } = props as React.HTMLAttributes<HTMLLIElement> & {
          key: string;
        };
        return (
          <li key={key} {...liProps} style={{ paddingTop: 2, paddingBottom: 2 }}>
            <Checkbox size="small" checked={selected} sx={{ mr: 1, p: 0.25 }} />
            <ListItemText
              primary={option}
              slotProps={{ primary: { style: { fontSize: 13 } } }}
            />
          </li>
        );
      }}
      renderInput={(params) => (
        <TextField
          {...params}
          label={label}
          placeholder={values.length ? undefined : "Type a product…"}
          helperText="Only incidents with a recorded service can match — this covers roughly half of all incidents."
        />
      )}
    />
  );
}
