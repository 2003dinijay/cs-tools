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

import { useCallback, useMemo, useState } from "react";

export interface RowSelection<T> {
  /** The selected rows, in the order they were picked. */
  selected: T[];
  count: number;
  isSelected: (row: T) => boolean;
  toggle: (row: T) => void;
  /** Select every selectable row of `rows` (a page), or clear them all when every one is already selected. */
  togglePage: (rows: T[]) => void;
  /** Whether every selectable row of `rows` is selected / some are. */
  pageState: (rows: T[]) => { all: boolean; some: boolean };
  clear: () => void;
}

/**
 * Row selection for a paged list: kept by id across pages (the rows are
 * remembered as they were when picked, so the bulk actions know their state
 * without refetching), with rows `canSelect` refuses never selectable (e.g. a
 * closed incident). The same model as the cases list's selection.
 */
export function useRowSelection<T>(getId: (row: T) => string | null | undefined, canSelect: (row: T) => boolean): RowSelection<T> {
  const [byId, setById] = useState<Map<string, T>>(() => new Map());

  const selectable = useCallback((rows: T[]) => rows.filter((r) => !!getId(r) && canSelect(r)), [getId, canSelect]);

  const toggle = useCallback(
    (row: T) => {
      const id = getId(row);
      if (!id || !canSelect(row)) return;
      setById((prev) => {
        const next = new Map(prev);
        if (next.has(id)) next.delete(id);
        else next.set(id, row);
        return next;
      });
    },
    [getId, canSelect],
  );

  const pageState = useCallback(
    (rows: T[]) => {
      const candidates = selectable(rows);
      const picked = candidates.filter((r) => byId.has(getId(r)!)).length;
      return { all: candidates.length > 0 && picked === candidates.length, some: picked > 0 && picked < candidates.length };
    },
    [byId, getId, selectable],
  );

  const togglePage = useCallback(
    (rows: T[]) => {
      const candidates = selectable(rows);
      setById((prev) => {
        const next = new Map(prev);
        const all = candidates.length > 0 && candidates.every((r) => next.has(getId(r)!));
        for (const r of candidates) {
          if (all) next.delete(getId(r)!);
          else next.set(getId(r)!, r);
        }
        return next;
      });
    },
    [getId, selectable],
  );

  const clear = useCallback(() => setById(new Map()), []);
  const selected = useMemo(() => Array.from(byId.values()), [byId]);

  return {
    selected,
    count: byId.size,
    isSelected: (row: T) => {
      const id = getId(row);
      return !!id && byId.has(id);
    },
    toggle,
    togglePage,
    pageState,
    clear,
  };
}
