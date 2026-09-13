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

import { LinearProgress } from "@mui/material";

/**
 * Thin, non-intrusive indicator that a background refetch is in flight while
 * `keepPreviousData` keeps the prior filter's data on screen. Absolutely
 * positioned against a `position: relative` ancestor so it reserves no
 * layout space and never shifts content.
 */
export function FetchProgressBar({ active }: { active: boolean }) {
  if (!active) return null;
  return <LinearProgress sx={{ position: "absolute", top: 0, left: 0, right: 0, height: 2, zIndex: 1 }} />;
}
