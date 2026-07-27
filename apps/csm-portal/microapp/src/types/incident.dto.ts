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

// Incidents (ServiceNow data source only). Mirrors the webapp's BeIncident/BeIncidentDetail
// (api/backend/types.ts). Unlike change requests, every enum here is UPPER_SNAKE_CASE on the wire.
// Read-only in this app for now (no create/edit) — see IncidentsTab.tsx/IncidentDetailPage.tsx.

import type { EntityRefDto } from "./case.dto";

export type IncidentPriority = "CRITICAL" | "HIGH" | "MODERATE" | "LOW" | "PLANNING";

export type IncidentState = "NEW" | "IN_PROGRESS" | "ON_HOLD" | "RESOLVED" | "CLOSED" | "CANCELLED";

/** List-item shape for an incident (`POST /incidents/search`). */
export interface IncidentSearchViewDto {
  id: string | null;
  number: string | null;
  openedOn: string | null;
  subject: string | null;
  caller?: EntityRefDto | null;
  priority: IncidentPriority | null;
  state: IncidentState | null;
  category: string | null;
  parent?: EntityRefDto | null;
  assignmentGroup?: EntityRefDto | null;
  assignedTo?: EntityRefDto | null;
  createdOn?: string;
  createdBy?: string;
  updatedOn?: string;
  updatedBy?: string;
}

export interface IncidentWatchListItemDto {
  id: string;
  name: string;
  email: string;
}

/** Service-request case linked to this incident (its `parent` case). */
export interface LinkedServiceRequestRefDto {
  id: string;
  number: string;
  name: string;
}

/**
 * `GET /incidents/{id}` — the search view plus the fields only the detail endpoint returns.
 * `category`/`subcategory`/`contactType`/`impact`/`urgency` are ServiceNow free-form/enum values
 * rendered as plain text (no label map) — same as the webapp's own detail page.
 */
export interface IncidentDetailDto extends IncidentSearchViewDto {
  subcategory?: string | null;
  service?: EntityRefDto | null;
  serviceOffering?: EntityRefDto | null;
  configurationItem?: EntityRefDto | null;
  contactType?: string | null;
  impact?: string | null;
  urgency?: string | null;
  changeRequest?: EntityRefDto | null;
  problem?: EntityRefDto | null;
  causedBy?: EntityRefDto | null;
  additionalComments?: string | null;
  workNotes?: string | null;
  watchList?: IncidentWatchListItemDto[];
  linkedServiceRequests?: LinkedServiceRequestRefDto[] | null;
}

export interface IncidentSearchPayloadDto {
  filters?: {
    searchQuery?: string;
    priorities?: IncidentPriority[];
  };
  sortBy?: {
    field?: "createdOn" | "updatedOn" | "openedOn";
    order?: "asc" | "desc";
  };
  pagination?: {
    offset?: number;
    limit?: number;
  };
}

export interface IncidentSearchResponseDto {
  incidents: IncidentSearchViewDto[];
  total: number;
  limit: number;
  offset: number;
}
