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

import { useCallback, useEffect } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useBackendApi } from "@api/backend/client";
import type {
  BeReorderSavedFilterViewPayload,
  BeSavedFilterViewList,
  BeSaveSavedFilterViewPayload,
} from "@api/backend/types";
import { ApiQueryKeys } from "@constants/apiConstants";
import {
  clearLegacySavedFilterViews,
  readLegacySavedFilterViews,
  type SavedFilterListKey,
  type SavedFilterView,
} from "@features/saved-filter-views/legacyStorage";

export type { SavedFilterListKey, SavedFilterView };

const migrating = new Set<SavedFilterListKey>();

function queryKey(listKey: SavedFilterListKey): [string, SavedFilterListKey] {
  return [ApiQueryKeys.SAVED_FILTER_VIEWS, listKey];
}

function listPath(listKey: SavedFilterListKey): string {
  return `/users/me/saved-filter-views?listKey=${encodeURIComponent(listKey)}`;
}

/**
 * Named list-filter bookmarks for one CSM list, persisted in Postgres via
 * the BFF. `qs` stays the opaque query string the list already serializes.
 * If the server list is empty, existing localStorage views are uploaded
 * once (last-to-first so display order is preserved) and the legacy key
 * is cleared.
 */
export function useSavedFilterViews(listKey: SavedFilterListKey): {
  views: SavedFilterView[];
  isLoading: boolean;
  saveFilterView: (name: string, qs: string) => void;
  deleteFilterView: (name: string) => void;
  moveFilterView: (name: string, direction: "up" | "down") => void;
} {
  const api = useBackendApi();
  const queryClient = useQueryClient();

  const query = useQuery({
    queryKey: queryKey(listKey),
    queryFn: async (): Promise<{ views: SavedFilterView[]; fromServer: boolean }> => {
      const res = await api.get<BeSavedFilterViewList>(listPath(listKey));
      if (res == null) {
        return { views: [], fromServer: false };
      }
      return { views: res.views ?? [], fromServer: true };
    },
  });

  useEffect(() => {
    if (!query.isSuccess || query.data === undefined) return;
    if (!query.data.fromServer || query.data.views.length > 0) return;
    if (migrating.has(listKey)) return;
    const local = readLegacySavedFilterViews(listKey).slice(0, 50);
    if (local.length === 0) return;

    migrating.add(listKey);
    void (async () => {
      try {
        let last: BeSavedFilterViewList | undefined;
        for (let i = local.length - 1; i >= 0; i -= 1) {
          last = await api.put<BeSaveSavedFilterViewPayload, BeSavedFilterViewList>(
            "/users/me/saved-filter-views",
            { listKey, name: local[i].name, qs: local[i].qs },
          );
        }
        clearLegacySavedFilterViews(listKey);
        if (last) {
          queryClient.setQueryData(queryKey(listKey), { views: last.views, fromServer: true });
        } else {
          await queryClient.invalidateQueries({ queryKey: queryKey(listKey) });
        }
      } catch {
        migrating.delete(listKey);
      }
    })();
  }, [api, listKey, query.data, query.isSuccess, queryClient]);

  const saveMutation = useMutation({
    mutationFn: (input: { name: string; qs: string }) =>
      api.put<BeSaveSavedFilterViewPayload, BeSavedFilterViewList>(
        "/users/me/saved-filter-views",
        { listKey, name: input.name, qs: input.qs },
      ),
    onSuccess: (data) => {
      queryClient.setQueryData(queryKey(listKey), { views: data.views, fromServer: true });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (name: string) =>
      api.del<BeSavedFilterViewList>(
        `${listPath(listKey)}&name=${encodeURIComponent(name)}`,
      ),
    onSuccess: (data) => {
      queryClient.setQueryData(queryKey(listKey), {
        views: data?.views ?? [],
        fromServer: true,
      });
    },
  });

  const reorderMutation = useMutation({
    mutationFn: (input: { name: string; direction: "up" | "down" }) =>
      api.post<BeReorderSavedFilterViewPayload, BeSavedFilterViewList>(
        "/users/me/saved-filter-views/reorder",
        { listKey, name: input.name, direction: input.direction },
      ),
    onSuccess: (data) => {
      queryClient.setQueryData(queryKey(listKey), { views: data.views, fromServer: true });
    },
  });

  const saveFilterView = useCallback(
    (name: string, qs: string): void => {
      const trimmed = name.trim();
      if (!trimmed) return;
      saveMutation.mutate({ name: trimmed, qs });
    },
    [saveMutation],
  );

  const deleteFilterView = useCallback(
    (name: string): void => {
      const trimmed = name.trim();
      if (!trimmed) return;
      deleteMutation.mutate(trimmed);
    },
    [deleteMutation],
  );

  const moveFilterView = useCallback(
    (name: string, direction: "up" | "down"): void => {
      const trimmed = name.trim();
      if (!trimmed) return;
      reorderMutation.mutate({ name: trimmed, direction });
    },
    [reorderMutation],
  );

  return {
    views: query.data?.views ?? [],
    isLoading: query.isLoading,
    saveFilterView,
    deleteFilterView,
    moveFilterView,
  };
}
