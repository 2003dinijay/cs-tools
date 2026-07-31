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

import { keepPreviousData, useInfiniteQuery } from "@tanstack/react-query";
import { useMemo } from "react";
import { ApiQueryKeys, BE_MAX_PAGE_LIMIT } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import type { BeProductSearchPayload, BeProductSearchResponse } from "@api/backend/types";

/**
 * Page size for the lazy-loaded (scroll-to-load-more) product filter. Larger
 * than the equivalent Project/Assignee page size (10): `POST /products/search`
 * returns one row per product model/version, many of which share the same
 * family name (e.g. dozens of "API Manager" variants) — a small raw page size
 * would risk the first page surfacing only a couple of distinct names.
 */
export const PRODUCT_PAGE_SIZE = BE_MAX_PAGE_LIMIT;

/** Flattened, deduplicated, paginated result for the lazy-loaded product filter. */
export interface InfiniteProductSearch {
  /** Distinct, non-empty product family names accumulated across every
   * fetched page so far, sorted for a stable dropdown order. */
  productNames: string[];
  isFetching: boolean;
  isFetchingNextPage: boolean;
  hasNextPage: boolean;
  isError: boolean;
  /** Fetch the next page (wired to the dropdown's scroll). */
  fetchNextPage: () => void;
}

/**
 * Paginated product-family-name search for the cases product filter. Loads
 * the first {@link PRODUCT_PAGE_SIZE} product rows as soon as it is enabled
 * (the dropdown opens) — no typing required — and pages through the rest on
 * demand via {@link InfiniteProductSearch.fetchNextPage} (wired to the
 * listbox scroll). An empty query lists the whole catalogue a page at a
 * time; a typed query narrows it (`searchQuery` matches ServiceNow's
 * `product.name`) and re-pages from the first match.
 *
 * Pagination tracks raw rows (matching the backend's own `hasMore`/offset
 * contract), while the returned `productNames` is a deduplicated view over
 * every row accumulated so far — the two counts are not the same, since many
 * rows (versions) can share one family name.
 */
export function useInfiniteProductSearch(
  query: string,
  enabled: boolean,
): InfiniteProductSearch {
  const api = useBackendApi();
  const q = query.trim();

  const result = useInfiniteQuery<BeProductSearchResponse, Error>({
    queryKey: [ApiQueryKeys.PRODUCTS, "search-paged", q],
    queryFn: ({ pageParam }) => {
      const pagination = { offset: pageParam as number, limit: PRODUCT_PAGE_SIZE };
      return api.post<BeProductSearchPayload, BeProductSearchResponse>(
        "/products/search",
        q.length > 0 ? { searchQuery: q, pagination } : { pagination },
      );
    },
    initialPageParam: 0,
    getNextPageParam: (lastPage, allPages) => {
      if (!lastPage.hasMore) return undefined;
      // Stop if the last page came back empty: the offset wouldn't advance, so
      // scroll-fetching would otherwise loop on the same page forever.
      if ((lastPage.products?.length ?? 0) === 0) return undefined;
      // Next offset = rows already loaded; robust even if the backend does not
      // echo back the offset we sent.
      return allPages.reduce((n, p) => n + (p.products?.length ?? 0), 0);
    },
    enabled,
    // Keep prior pages on screen while a new query's first page loads, so the
    // dropdown doesn't flash empty between searches.
    placeholderData: keepPreviousData,
    staleTime: 60_000,
  });

  const productNames = useMemo(() => {
    const names = new Set<string>();
    (result.data?.pages ?? []).forEach((page) => {
      (page.products ?? []).forEach((p) => {
        const name = p.name?.trim();
        if (name) names.add(name);
      });
    });
    return [...names].sort((a, b) => a.localeCompare(b));
  }, [result.data]);

  return {
    productNames,
    isFetching: result.isFetching,
    isFetchingNextPage: result.isFetchingNextPage,
    hasNextPage: result.hasNextPage,
    isError: result.isError,
    fetchNextPage: () => {
      void result.fetchNextPage();
    },
  };
}
