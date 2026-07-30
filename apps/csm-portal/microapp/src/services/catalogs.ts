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

import { queryOptions } from "@tanstack/react-query";
import { CATALOGS_SEARCH_ENDPOINT, CATALOG_ITEM_VARIABLES_ENDPOINT } from "@config/endpoints";
import type {
  CatalogItemVariableDto,
  CatalogItemVariablesResponseDto,
  CatalogRefDto,
  CatalogSearchResponseDto,
} from "@src/types";
import apiClient from "./apiClient";

// Matches deployments.ts's SEARCH_PAGE_LIMIT — the live entity service rejects the openapi-declared
// 100 max with a 400.
const SEARCH_PAGE_LIMIT = 50;

// Service catalogs (and the catalog items they embed) only exist for ServiceNow-sourced deployed
// products; managed-cloud products resolve to an empty list. The catalog select has no
// infinite-scroll UI of its own, so fetch every page rather than just the first — same technique
// as deployments.ts's listDeployments. Advances the offset by the actual page size (the backend
// may clamp `limit` below SEARCH_PAGE_LIMIT) and stops on an empty page or once the reported total
// is reached.
const searchCatalogs = async (deployedProductId: string): Promise<CatalogRefDto[]> => {
  const all: CatalogRefDto[] = [];
  let offset = 0;
  for (;;) {
    const { data } = await apiClient.post<CatalogSearchResponseDto>(CATALOGS_SEARCH_ENDPOINT, {
      deployedProductId,
      pagination: { offset, limit: SEARCH_PAGE_LIMIT },
    });
    const page = data?.catalogs ?? [];
    all.push(...page);
    offset += page.length;
    const total = data?.total;
    if (page.length === 0) break;
    if (total != null && all.length >= total) break;
  }
  return all;
};

// Sorted by the backend's display order so the form renders fields in the catalog's intended
// sequence, mirroring the webapp's useCatalogItemVariables.ts.
const getCatalogItemVariables = async (catalogId: string, catalogItemId: string): Promise<CatalogItemVariableDto[]> => {
  const { data } = await apiClient.get<CatalogItemVariablesResponseDto>(
    CATALOG_ITEM_VARIABLES_ENDPOINT(catalogId, catalogItemId),
  );
  const variables = data?.variables ?? [];
  return [...variables].sort((a, b) => (a.order ?? Number.MAX_SAFE_INTEGER) - (b.order ?? Number.MAX_SAFE_INTEGER));
};

export const catalogs = {
  search: (deployedProductId: string) =>
    queryOptions({
      queryKey: ["catalogs", deployedProductId],
      queryFn: () => searchCatalogs(deployedProductId),
      enabled: !!deployedProductId,
      staleTime: 60_000,
    }),

  itemVariables: (catalogId: string, catalogItemId: string) =>
    queryOptions({
      queryKey: ["catalog-item-variables", catalogId, catalogItemId],
      queryFn: () => getCatalogItemVariables(catalogId, catalogItemId),
      enabled: !!catalogId && !!catalogItemId,
      staleTime: 60_000,
    }),
};
