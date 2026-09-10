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

import type { BeIncidentResolutionCode } from "@api/backend/types";

/** All backend incident resolution codes, in the order declared in openapi.yaml. */
export const INCIDENT_RESOLUTION_CODES: BeIncidentResolutionCode[] = [
  "Solved (Work Around)",
  "Solved (Permanently)",
  "Not Solved (Not Reproducible)",
  "False Alarm",
  "Duplicate",
  "Not Actionable Alert",
];

/**
 * Display text for each incident resolution code. Matches the backing data
 * source's choice-list values verbatim, except "Duplicate" — the source's UI
 * label for that choice is "Duplicate Alert", but this platform isn't an
 * alerting feature, so it's shown here as just "Duplicate".
 */
export const INCIDENT_RESOLUTION_CODE_LABELS: Record<BeIncidentResolutionCode, string> = {
  "Solved (Work Around)": "Solved (Work Around)",
  "Solved (Permanently)": "Solved (Permanently)",
  "Not Solved (Not Reproducible)": "Not Solved (Not Reproducible)",
  "False Alarm": "False Alarm",
  Duplicate: "Duplicate",
  "Not Actionable Alert": "Not Actionable Alert",
};
