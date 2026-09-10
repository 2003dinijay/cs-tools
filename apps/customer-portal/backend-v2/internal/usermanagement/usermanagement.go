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

package usermanagement

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/apierror"
)

// GetProjectContacts calls GET /projects/{projectId}/contacts.
func (c *Client) GetProjectContacts(ctx context.Context, projectID string) ([]Contact, error) {
	raw, err := c.doJSON(ctx, http.MethodGet, "/projects/"+url.PathEscape(projectID)+"/contacts", nil, http.StatusOK)
	if err != nil {
		return nil, err
	}
	var wire []wireContact
	if err := json.Unmarshal(raw, &wire); err != nil {
		return nil, fmt.Errorf("usermanagement: decode contacts response: %w", err)
	}
	out := make([]Contact, 0, len(wire))
	for _, c := range wire {
		out = append(out, toContact(c))
	}
	return out, nil
}

// CreateProjectContact calls POST /projects/{projectId}/contact (singular —
// matches the upstream service's own path, unlike every other route here).
func (c *Client) CreateProjectContact(ctx context.Context, projectID string, req OnBoardContactPayload) (Membership, error) {
	body, err := json.Marshal(wireOnBoardContactPayload{
		ContactEmail:        req.ContactEmail,
		AdminEmail:          req.AdminEmail,
		ContactFirstName:    req.ContactFirstName,
		ContactLastName:     req.ContactLastName,
		IsCsIntegrationUser: req.IsCsIntegrationUser,
		Role:                getRoles(req.IsCsAdmin, req.IsLead, req.IsPortalUser, req.IsSecurityContact),
	})
	if err != nil {
		return Membership{}, fmt.Errorf("usermanagement: encode create contact request: %w", err)
	}
	raw, err := c.doJSON(ctx, http.MethodPost, "/projects/"+url.PathEscape(projectID)+"/contact", body, http.StatusCreated)
	if err != nil {
		return Membership{}, err
	}
	var wire wireMembership
	if err := json.Unmarshal(raw, &wire); err != nil {
		return Membership{}, fmt.Errorf("usermanagement: decode create contact response: %w", err)
	}
	return toMembership(wire), nil
}

// RemoveProjectContact calls DELETE /projects/{projectId}/contacts/{contactEmail}?adminEmail=...
func (c *Client) RemoveProjectContact(ctx context.Context, projectID, contactEmail, adminEmail string) (Membership, error) {
	path := "/projects/" + url.PathEscape(projectID) + "/contacts/" + url.PathEscape(contactEmail) +
		"?" + url.Values{"adminEmail": {adminEmail}}.Encode()
	raw, err := c.doJSON(ctx, http.MethodDelete, path, nil, http.StatusOK)
	if err != nil {
		return Membership{}, err
	}
	var wire wireMembership
	if err := json.Unmarshal(raw, &wire); err != nil {
		return Membership{}, fmt.Errorf("usermanagement: decode remove contact response: %w", err)
	}
	return toMembership(wire), nil
}

// UpdateMembershipRole calls PATCH /projects/{projectId}/contacts/{contactEmail}.
func (c *Client) UpdateMembershipRole(ctx context.Context, projectID, contactEmail string, req MembershipRolePayload) (Membership, error) {
	body, err := json.Marshal(wireMembershipRolePayload{
		AdminEmail: req.AdminEmail,
		Role:       getRoles(req.IsCsAdmin, req.IsLead, req.IsPortalUser, req.IsSecurityContact),
	})
	if err != nil {
		return Membership{}, fmt.Errorf("usermanagement: encode update membership role request: %w", err)
	}
	path := "/projects/" + url.PathEscape(projectID) + "/contacts/" + url.PathEscape(contactEmail)
	raw, err := c.doJSON(ctx, http.MethodPatch, path, body, http.StatusOK)
	if err != nil {
		return Membership{}, err
	}
	var wire wireMembership
	if err := json.Unmarshal(raw, &wire); err != nil {
		return Membership{}, fmt.Errorf("usermanagement: decode update membership role response: %w", err)
	}
	return toMembership(wire), nil
}

// ValidateProjectContact calls POST /validate-project-contact. The upstream
// service uses the HTTP status code itself to distinguish three outcomes:
//   - 201 Created: a deactivated contact with this email already exists —
//     contact is non-nil, conflict is false, err is nil.
//   - 202 Accepted: the contact is new and can be onboarded — contact is
//     nil, conflict is false, err is nil.
//   - 409 Conflict: an active contact with this email already exists —
//     contact is nil, conflict is true, err carries the upstream message.
//   - anything else: contact is nil, conflict is false, err carries the
//     upstream message.
func (c *Client) ValidateProjectContact(ctx context.Context, req ValidationPayload) (contact *Contact, conflict bool, err error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, false, fmt.Errorf("usermanagement: encode validation request: %w", err)
	}

	statusCode, raw, err := c.do(ctx, http.MethodPost, "/validate-project-contact", body)
	if err != nil {
		return nil, false, err
	}

	switch statusCode {
	case http.StatusCreated:
		var wire wireContact
		if err := json.Unmarshal(raw, &wire); err != nil {
			return nil, false, fmt.Errorf("usermanagement: decode validation response: %w", err)
		}
		mapped := toContact(wire)
		return &mapped, false, nil
	case http.StatusAccepted:
		return nil, false, nil
	case http.StatusConflict:
		return nil, true, apierror.NewUpstreamError(statusCode, raw)
	default:
		return nil, false, apierror.NewUpstreamError(statusCode, raw)
	}
}
