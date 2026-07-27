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

// Service catalogs (ServiceNow only) — drive service-request creation. Mirrors the webapp's
// BeCatalogRef/BeCatalogItemRef/BeCatalogItemVariable (api/backend/types.ts).

export interface CatalogItemRefDto {
  id: string;
  name?: string;
}

/** A service catalog and the catalog items it offers. */
export interface CatalogRefDto {
  id: string;
  name?: string;
  catalogItems?: CatalogItemRefDto[];
}

export interface CatalogSearchPayloadDto {
  deployedProductId: string;
  pagination?: { offset?: number; limit?: number };
}

export interface CatalogSearchResponseDto {
  catalogs?: CatalogRefDto[];
  total?: number;
  limit?: number;
  offset?: number;
}

/**
 * A catalog-item variable (form field). The contract carries the question text, display order,
 * and a free-form `type` hint, but no choice/option list or mandatory flag — every variable
 * renders as a text field unless its type/questionText matches one of the classifiers in
 * utils/catalogVariables.ts.
 */
export interface CatalogItemVariableDto {
  id: string;
  questionText?: string;
  order?: number;
  type?: string;
}

export interface CatalogItemVariablesResponseDto {
  variables?: CatalogItemVariableDto[];
}
