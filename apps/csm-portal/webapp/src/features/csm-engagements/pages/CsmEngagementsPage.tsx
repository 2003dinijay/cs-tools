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

import { type JSX } from "react";

import CsmIssuesView from "@features/csm-cases/components/CsmIssuesView";

/**
 * Cross-customer engagements list — filters the shared issues view to
 * `type: engagement` and surfaces the engagement-type sub-filter so engineers
 * can narrow by migration, consultancy, onboarding, etc.
 *
 * No "Create engagement" entry point exists here: creating an engagement has
 * no working path anywhere in the stack yet (ServiceNow's case-creation
 * dispatcher has no branch for `type: engagement` at all, so a request would
 * 500) — see the CSM Portal PROGRESS notes for the tracked follow-up. Do not
 * add a create button that points at a flow which can't actually succeed.
 */
export default function CsmEngagementsPage(): JSX.Element {
  return (
    <CsmIssuesView
      title="Engagements"
      entityNoun="engagements"
      lockedFilters={{ caseTypes: ["engagement"] }}
      hideTypeFilter
      showEngagementTypeFilter
      detailBasePath="/engagements"
      hideSeverityColumn
    />
  );
}
