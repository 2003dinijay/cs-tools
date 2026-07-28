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

import { Button } from "@wso2/oxygen-ui";
import { Plus } from "@wso2/oxygen-ui-icons-react";
import { type JSX } from "react";

import CsmIssuesView from "@features/csm-cases/components/CsmIssuesView";
import { useNavTransition } from "@hooks/useNavTransition";

/**
 * Cross-customer engagements list — filters the shared issues view to
 * `type: engagement` and surfaces the engagement-type sub-filter so engineers
 * can narrow by migration, consultancy, onboarding, etc.
 */
export default function CsmEngagementsPage(): JSX.Element {
  const navigate = useNavTransition();

  return (
    <CsmIssuesView
      title="Engagements"
      entityNoun="engagements"
      lockedFilters={{ caseTypes: ["engagement"] }}
      hideTypeFilter
      showEngagementTypeFilter
      detailBasePath="/engagements"
      hideSeverityColumn
      actions={
        <Button
          variant="contained"
          color="primary"
          size="small"
          startIcon={<Plus size={16} />}
          onClick={() => navigate("/engagements/new")}
        >
          Create engagement
        </Button>
      }
    />
  );
}
