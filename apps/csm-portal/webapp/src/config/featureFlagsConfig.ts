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

/**
 * Runtime feature flags read once from `window.config`. Follows the same
 * module-scope accessor pattern as the other `config/*` modules: read
 * `window.config?.KEY`, apply a safe default, export a typed constant.
 */

/**
 * When true, every page/menu item flagged `wip` (see `csmNavItems.ts`) is hidden
 * from the sidebar and quick-nav, and its routes redirect to the dashboard. Lets
 * a deployment ship with unfinished sections switched off without a rebuild.
 * Defaults to false (nothing hidden) when the key is absent.
 */
export const HIDE_WIP_FEATURES: boolean =
  window.config?.CSM_PORTAL_HIDE_WIP_FEATURES ?? false;
