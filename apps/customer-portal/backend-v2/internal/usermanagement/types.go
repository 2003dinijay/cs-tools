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

import "strings"

// Role name strings the upstream service represents as a single
// semicolon-delimited string on a contact/membership (e.g. "Admin;Lead") —
// see getRoles/hasRole.
const (
	roleAdmin           = "Admin"
	rolePortalUser      = "Portal user"
	roleSecurityContact = "Security Contact"
	roleLead            = "Lead"
)

// Account is the account summary embedded in a Contact.
type Account struct {
	ID             *string `json:"id,omitempty"`
	DomainList     string  `json:"domainList,omitempty"`
	Classification string  `json:"classification,omitempty"`
	IsPartner      *bool   `json:"isPartner,omitempty"`
}

// Contact is a project contact, with the upstream's single role string
// already split out into individual booleans.
type Contact struct {
	ID                  string   `json:"id"`
	Email               string   `json:"email"`
	FirstName           *string  `json:"firstName,omitempty"`
	LastName            string   `json:"lastName"`
	IsCsAdmin           bool     `json:"isCsAdmin"`
	IsLead              bool     `json:"isLead"`
	IsCsIntegrationUser bool     `json:"isCsIntegrationUser"`
	IsPortalUser        bool     `json:"isPortalUser"`
	IsSecurityContact   bool     `json:"isSecurityContact"`
	MembershipStatus    *string  `json:"membershipStatus,omitempty"`
	Account             *Account `json:"account,omitempty"`
}

// wireContact mirrors the upstream service's contact shape: everything
// Contact has, plus a single semicolon-delimited Role string instead of
// four booleans.
type wireContact struct {
	ID                  string   `json:"id"`
	Email               string   `json:"email"`
	FirstName           *string  `json:"firstName,omitempty"`
	LastName            string   `json:"lastName"`
	IsCsIntegrationUser bool     `json:"isCsIntegrationUser"`
	Role                *string  `json:"role,omitempty"`
	MembershipStatus    *string  `json:"membershipStatus,omitempty"`
	Account             *Account `json:"account,omitempty"`
}

func toContact(c wireContact) Contact {
	return Contact{
		ID:                  c.ID,
		Email:               c.Email,
		FirstName:           c.FirstName,
		LastName:            c.LastName,
		IsCsAdmin:           hasRole(c.Role, roleAdmin),
		IsLead:              hasRole(c.Role, roleLead),
		IsCsIntegrationUser: c.IsCsIntegrationUser,
		IsPortalUser:        hasRole(c.Role, rolePortalUser),
		IsSecurityContact:   hasRole(c.Role, roleSecurityContact),
		MembershipStatus:    c.MembershipStatus,
		Account:             c.Account,
	}
}

// OnBoardContactPayload is the input for CreateProjectContact.
type OnBoardContactPayload struct {
	ContactEmail        string
	AdminEmail          string
	ContactFirstName    string
	ContactLastName     string
	IsCsIntegrationUser bool
	IsCsAdmin           bool
	IsLead              bool
	IsPortalUser        bool
	IsSecurityContact   bool
}

// wireOnBoardContactPayload is the upstream service's request shape for
// POST /projects/{projectId}/contact.
type wireOnBoardContactPayload struct {
	ContactEmail        string   `json:"contactEmail"`
	AdminEmail          string   `json:"adminEmail"`
	ContactFirstName    string   `json:"contactFirstName"`
	ContactLastName     string   `json:"contactLastName"`
	IsCsIntegrationUser bool     `json:"isCsIntegrationUser"`
	Role                []string `json:"role,omitempty"`
}

// ContactRef is the minimal contact reference embedded in a Membership.
type ContactRef struct {
	ID    *string `json:"id,omitempty"`
	Email *string `json:"email,omitempty"`
}

// Membership is a contact's membership on a project, with the upstream's
// single role string already split out into individual booleans.
type Membership struct {
	ID                string      `json:"id"`
	State             string      `json:"state"`
	IsCsAdmin         bool        `json:"isCsAdmin"`
	IsLead            bool        `json:"isLead"`
	IsPortalUser      bool        `json:"isPortalUser"`
	IsSecurityContact bool        `json:"isSecurityContact"`
	Contact           *ContactRef `json:"contact,omitempty"`
}

// wireMembership mirrors the upstream service's membership shape.
type wireMembership struct {
	ID      string      `json:"id"`
	State   string      `json:"state"`
	Role    *string     `json:"role,omitempty"`
	Contact *ContactRef `json:"contact,omitempty"`
}

func toMembership(m wireMembership) Membership {
	return Membership{
		ID:                m.ID,
		State:             m.State,
		IsCsAdmin:         hasRole(m.Role, roleAdmin),
		IsLead:            hasRole(m.Role, roleLead),
		IsPortalUser:      hasRole(m.Role, rolePortalUser),
		IsSecurityContact: hasRole(m.Role, roleSecurityContact),
		Contact:           m.Contact,
	}
}

// MembershipRolePayload is the input for UpdateMembershipRole.
type MembershipRolePayload struct {
	AdminEmail        string
	IsCsAdmin         bool
	IsLead            bool
	IsPortalUser      bool
	IsSecurityContact bool
}

// wireMembershipRolePayload is the upstream service's request shape for
// PATCH /projects/{projectId}/contacts/{contactEmail}.
type wireMembershipRolePayload struct {
	AdminEmail string   `json:"adminEmail"`
	Role       []string `json:"role"`
}

// ValidationPayload is the input for ValidateProjectContact.
type ValidationPayload struct {
	ProjectID    string `json:"projectId"`
	ContactEmail string `json:"contactEmail"`
	AdminEmail   string `json:"adminEmail"`
}

// getRoles converts the four role booleans into the role-name slice the
// upstream service expects, in the exact order and shape its role field requires.
func getRoles(isCsAdmin, isLead, isPortalUser, isSecurityContact bool) []string {
	var roles []string
	if isCsAdmin {
		roles = append(roles, roleAdmin)
	}
	if isLead {
		roles = append(roles, roleLead)
	}
	if isPortalUser {
		roles = append(roles, rolePortalUser)
	}
	if isSecurityContact {
		roles = append(roles, roleSecurityContact)
	}
	return roles
}

// hasRole reports whether role appears in the upstream's semicolon-delimited
// role string, trimming whitespace around each part. A nil roleValue never matches.
func hasRole(roleValue *string, role string) bool {
	if roleValue == nil {
		return false
	}
	for _, part := range strings.Split(*roleValue, ";") {
		if strings.TrimSpace(part) == role {
			return true
		}
	}
	return false
}
