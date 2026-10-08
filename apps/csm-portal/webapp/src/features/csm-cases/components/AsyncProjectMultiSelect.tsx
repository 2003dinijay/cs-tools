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
import { useMemo, useState, type JSX } from "react";
import type * as React from "react";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import { useInfiniteProjectSearch } from "@features/csm-cases/api/useProjectSearch";

interface ProjectOption {
  id: string;
  name: string;
}

interface AsyncProjectMultiSelectProps {
  id?: string;
  label?: string;
  /** Selected project ids. */
  values: string[];
  onChange: (next: string[]) => void;
  /**
   * Known id → name pairs (e.g. from the cases currently on screen) used to
   * label already-selected projects before any search has run.
   */
  nameSeed?: Map<string, string>;
}

/**
 * Project filter that searches the backend as the user types instead of
 * loading the whole project catalogue up front. Selected project names are
 * remembered (captured at selection time, plus any seed) so the chips stay
 * labelled even after the search results change.
 */
export default function AsyncProjectMultiSelect({
  id = "cases-filter-project",
  label = "Project",
  values,
  onChange,
  nameSeed,
}: AsyncProjectMultiSelectProps): JSX.Element {
  const [input, setInput] = useState("");
  const [open, setOpen] = useState(false);
  const debounced = useDebouncedValue(input, 300);
  const query = debounced.trim();

  // Enabled while the dropdown is open, so it loads the first page of projects
  // on open (no typing needed) and re-pages as the user types. Closed → the
  // query idles (cached pages stay for an instant re-open).
  const {
    projects,
    isFetching,
    isFetchingNextPage,
    hasNextPage,
    isError,
    fetchNextPage,
  } = useInfiniteProjectSearch(query, open);

  // Lazy-load the next page when the listbox is scrolled near its end.
  const handleListboxScroll = (event: React.UIEvent<HTMLElement>): void => {
    const el = event.currentTarget;
    if (
      hasNextPage &&
      !isFetchingNextPage &&
      el.scrollHeight - el.scrollTop - el.clientHeight < 80
    ) {
      fetchNextPage();
    }
  };

  // Names captured when the user picks a project, so a chip keeps its label
  // even once the search moves on to a different term.
  const [pickedNames, setPickedNames] = useState<Map<string, string>>(
    () => new Map(),
  );

  const nameById = useMemo(() => {
    const m = new Map<string, string>(nameSeed);
    projects.forEach((p) => {
      if (p.name) m.set(p.id, p.name);
    });
    pickedNames.forEach((name, pid) => m.set(pid, name));
    return m;
  }, [nameSeed, projects, pickedNames]);

  const selectedOptions: ProjectOption[] = useMemo(
    () => values.map((v) => ({ id: v, name: nameById.get(v) ?? v })),
    [values, nameById],
  );

  // Pool = current selection (so the field can render its chips) + the search
  // results, de-duplicated by id.
  const options: ProjectOption[] = useMemo(() => {
    const results = projects.map((p) => ({ id: p.id, name: p.name || p.id }));
    const seen = new Set(values);
    return [...selectedOptions, ...results.filter((o) => !seen.has(o.id))];
  }, [projects, values, selectedOptions]);

  return (
    <Autocomplete<ProjectOption, true>
      multiple
      size="small"
      id={id}
      options={options}
      value={selectedOptions}
      open={open}
      onOpen={() => setOpen(true)}
      onClose={() => {
        setOpen(false);
        // Clear the stale search term once the user is done picking from it
        // (the dropdown only closes on blur/Escape/click-away, never on a
        // selection itself — see disableCloseOnSelect below) — otherwise it
        // would resurface the next time renderTags shows the selected-names
        // summary and visually collide with it (see renderTags' own comment).
        setInput("");
      }}
      // Spinner only while the first page loads; later pages append on scroll.
      loading={isFetching && projects.length === 0}
      disableCloseOnSelect
      sx={{
        "& .MuiAutocomplete-inputRoot": { flexWrap: "nowrap", minHeight: 40 },
      }}
      // The backend already filtered by the typed term; don't re-filter locally.
      filterOptions={(opts) => opts}
      getOptionLabel={(opt) => opt.name}
      isOptionEqualToValue={(opt, val) => opt.id === val.id}
      // Fixed max-height (rather than relying on the default 40vh popper
      // sizing) so the listbox is reliably scrollable as soon as the first
      // page of results loads, on any screen size. Without this, a tall or
      // otherwise short-content viewport can let the default sizing fit the
      // whole first PROJECT_PAGE_SIZE page (10 rows) with room to spare --
      // nothing overflows, so onScroll's near-the-bottom check
      // (handleListboxScroll) never fires and fetchNextPage never runs,
      // making the picker look hard-capped at 10 projects even though the
      // pagination itself supports the full catalogue.
      slotProps={{ listbox: { onScroll: handleListboxScroll, style: { maxHeight: 280 } } }}
      onChange={(_event, next) => {
        setPickedNames((prev) => {
          const m = new Map(prev);
          next.forEach((o) => m.set(o.id, o.name));
          return m;
        });
        onChange(next.map((o) => o.id));
        // MUI's own attempt to clear the input after a pick (ignored above,
        // reason "reset") still leaves the cursor mid-string. Restore it to
        // the end once the DOM settles.
        requestAnimationFrame(() => {
          const el = document.getElementById(id) as HTMLInputElement | null;
          el?.setSelectionRange(el.value.length, el.value.length);
        });
      }}
      inputValue={input}
      onInputChange={(_event, value, reason) => {
        // Keep the typed term after a selection (reason "reset") so the user can
        // pick several from one search; clear only on explicit input/clear.
        if (reason === "input" || reason === "clear") setInput(value);
      }}
      noOptionsText={
        isError
          ? "Couldn't load projects. Try again."
          : isFetching
            ? "Loading projects…"
            : "No projects found"
      }
      renderTags={(value) => {
        // While the dropdown is open, the live search text in `input` is
        // what the user is actively looking at — rendering the selected-
        // names summary in the same single-line field at the same time
        // visually concatenates the two (reported live: "Customer 3
        // Project - Managed Cloud Subscription            managed" on one
        // line, with "managed" the still-typed search term). The summary is
        // only useful once the field is collapsed/closed — the checkboxes
        // in the dropdown already show what's picked while it's open — so
        // it's suppressed until then; onClose (above) clears the stale
        // search text at the same moment this starts rendering again.
        if (open) return null;
        const displayText = value.map((o) => o.name).join(", ");
        const content = (
          <Box
            component="span"
            sx={{ flex: "1 1 0", minWidth: 0, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", pl: 1 }}
          >
            {displayText}
          </Box>
        );
        return value.length === 1 ? content : (
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
            {/* A project name can run long with no space near the end
                (e.g. a slash-joined account/subscription name) --
                without `overflowWrap`, it only breaks at a space/hyphen,
                then overflows and gets clipped by the popup's own
                overflow instead of wrapping onto a further line. */}
            <ListItemText
              primary={option.name}
              slotProps={{
                primary: { style: { fontSize: 13, overflowWrap: "anywhere" } },
              }}
            />
          </li>
        );
      }}
      renderInput={(params) => (
        <TextField
          {...params}
          label={label}
          placeholder={values.length ? undefined : "Type a project…"}
        />
      )}
    />
  );
}
