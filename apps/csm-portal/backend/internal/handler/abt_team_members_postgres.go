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

package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/servicenow"
)

// entityTeamsClient is the subset of internal/entity.CustomerEntityClient
// this file needs.
type entityTeamsClient interface {
	GetTeamMembers(ctx context.Context, teamID string) ([]byte, error)
}

type entityTeamMember struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Email *string `json:"email"`
	Role  *string `json:"role"`
}

type entityGetTeamMembersResponse struct {
	Members []entityTeamMember `json:"members"`
}

// postgresSplAbtTeamMembersClient implements abtTeamMembersClient by calling
// entity-service's new /teams/{id}/members endpoint (Postgres) instead of
// ServiceNow's sys_user_grmember table directly.
type postgresSplAbtTeamMembersClient struct {
	entity entityTeamsClient
}

// NewPostgresSplAbtTeamMembersClient builds a postgresSplAbtTeamMembersClient.
// entity is the same *entity.CustomerEntityClient every other CS Portal
// handler already uses.
func NewPostgresSplAbtTeamMembersClient(entity entityTeamsClient) *postgresSplAbtTeamMembersClient {
	return &postgresSplAbtTeamMembersClient{entity: entity}
}

var hex32Pattern = regexp.MustCompile(`^[0-9a-fA-F]{32}$`)

// normalizeToUUID converts a bare 32-hex-character ServiceNow sys_id (no
// dashes, e.g. what SPL's teamId query param has always carried, since it
// used to go straight into a ServiceNow Table API query) into standard
// 8-4-4-4-12 dashed UUID form, matching how entity-service's Postgres ids
// are formatted. A value that doesn't match that bare-hex shape (already
// dashed, or some other id shape entirely) is passed through unchanged --
// this never rejects input itself, entity-service's own UUID validation
// does that.
func normalizeToUUID(id string) string {
	if !hex32Pattern.MatchString(id) {
		return id
	}
	lower := strings.ToLower(id)
	return fmt.Sprintf("%s-%s-%s-%s-%s", lower[0:8], lower[8:12], lower[12:16], lower[16:20], lower[20:32])
}

// GetABTTeamMembers implements abtTeamMembersClient.
func (c *postgresSplAbtTeamMembersClient) GetABTTeamMembers(ctx context.Context, teamID string) ([]servicenow.ABTTeamRosterMember, error) {
	raw, err := c.entity.GetTeamMembers(ctx, normalizeToUUID(teamID))
	if err != nil {
		return nil, err
	}
	var resp entityGetTeamMembersResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal entity-service team members response: %w", err)
	}

	roster := make([]servicenow.ABTTeamRosterMember, 0, len(resp.Members))
	for _, m := range resp.Members {
		member := servicenow.ABTTeamRosterMember{Name: m.Name}
		if m.Email != nil {
			member.Email = *m.Email
		}
		if m.Role != nil {
			member.Role = *m.Role
		}
		roster = append(roster, member)
	}
	return roster, nil
}
