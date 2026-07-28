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

import { useMemo, useState } from "react";
import { Autocomplete, Chip, TextField } from "@wso2/oxygen-ui";
import { useQuery } from "@tanstack/react-query";
import { useDebouncedValue } from "@utils/useDebouncedValue";
import type { EntityOption } from "@src/services/changeRequestLookups";

interface AsyncEntityMultiSelectProps {
  label: string;
  placeholder?: string;
  value: EntityOption[];
  onChange: (next: EntityOption[]) => void;
  disabled?: boolean;
  helperText?: string;
  search: (query: string, extra?: string) => Promise<EntityOption[]>;
  searchExtra?: string;
}

// Multi-select counterpart of AsyncEntitySelect.tsx, for the incident create form's Watch list
// field — the only multi-value lookup in this app so far. Kept as its own small component rather
// than adding a `multiple` variant to AsyncEntitySelect's props, so that component's five existing
// single-select usages stay untouched.
export function AsyncEntityMultiSelect({
  label,
  placeholder,
  value,
  onChange,
  disabled,
  helperText,
  search,
  searchExtra,
}: AsyncEntityMultiSelectProps) {
  const [inputValue, setInputValue] = useState("");
  const debouncedQuery = useDebouncedValue(inputValue, 300);
  const query = debouncedQuery.trim();
  const { data, isFetching, isError } = useQuery({
    queryKey: ["async-entity-multi-select", label, query, searchExtra ?? ""],
    queryFn: () => search(query, searchExtra),
    enabled: query.length > 0,
    staleTime: 60_000,
  });

  const options = useMemo(() => {
    const results = data ?? [];
    const missingSelected = value.filter((v) => !results.some((o) => o.id === v.id));
    return [...missingSelected, ...results];
  }, [data, value]);

  return (
    <Autocomplete
      multiple
      options={options}
      value={value}
      loading={isFetching}
      disabled={disabled}
      fullWidth
      size="small"
      getOptionLabel={(option) => option.label}
      isOptionEqualToValue={(option, val) => option.id === val.id}
      onChange={(_, next) => onChange(next)}
      onInputChange={(_, next) => setInputValue(next)}
      noOptionsText={
        query.length === 0 ? "Type to search…" : isError ? "Search failed. Try again." : "No matches found"
      }
      renderTags={(tagValue, getTagProps) =>
        tagValue.map((option, index) => (
          <Chip {...getTagProps({ index })} key={option.id} label={option.label} size="small" />
        ))
      }
      renderInput={(params) => (
        <TextField
          {...params}
          label={label}
          placeholder={value.length === 0 ? placeholder : undefined}
          error={isError}
          helperText={isError ? "Search failed." : helperText}
        />
      )}
    />
  );
}
