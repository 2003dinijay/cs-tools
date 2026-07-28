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

// Package recipients implements the ACP Phase 1 customer-contact
// resolution: given a project's already-fetched Project Contacts and an
// account's Account Contacts, decide who should receive the customer-facing
// closure notice. Pure — no I/O, same philosophy as the closure package.
package recipients

// ProjectContact mirrors entity-service's project-contact shape (Supported
// by the ServiceNow data source only).
type ProjectContact struct {
	Name  string
	Email string
	Roles []string
}

// AccountContact mirrors entity-service's account-contact shape (Supported
// by the ServiceNow data source only).
type AccountContact struct {
	Name      string
	Email     string
	IsPrimary bool
}

// Contact is a resolved recipient for the closure notice.
type Contact struct {
	Name  string
	Email string
}

// ResolvedVia records which tier of the fallback chain produced the
// resolution, for observability into how often each tier is actually used.
type ResolvedVia string

const (
	ResolvedViaBusinessContact ResolvedVia = "business_contact"
	ResolvedViaPrimaryContact  ResolvedVia = "primary_contact"
	ResolvedViaNone            ResolvedVia = "none"
)

// businessContactRole is the literal Roles value matched against each
// ProjectContact. Pending confirmation from the API team (Sajith) of the
// exact ServiceNow-side string — swap this constant once confirmed; no
// other code in this package needs to change.
const businessContactRole = "business_contact" // PLACEHOLDER

// Resolution is the outcome of resolving who should receive a project's
// customer-facing ACP notice. NeedsAMNudge signals a *different* email (a
// "please configure a business contact" nudge) than the closure notice
// itself — it is not additive with CustomerContact.
type Resolution struct {
	CustomerContact *Contact
	NeedsAMNudge    bool
	ResolvedVia     ResolvedVia
}

// ResolveCustomerContact implements the three-tier fallback: a Project
// Contact with the business-contact role first, then the account's Primary
// Contact, then a signal to nudge the Account Manager instead.
func ResolveCustomerContact(projectContacts []ProjectContact, accountContacts []AccountContact) Resolution {
	for _, c := range projectContacts {
		if hasBusinessContactRole(c) {
			return Resolution{
				CustomerContact: &Contact{Name: c.Name, Email: c.Email},
				ResolvedVia:     ResolvedViaBusinessContact,
			}
		}
	}

	for _, c := range accountContacts {
		if c.IsPrimary {
			return Resolution{
				CustomerContact: &Contact{Name: c.Name, Email: c.Email},
				ResolvedVia:     ResolvedViaPrimaryContact,
			}
		}
	}

	return Resolution{NeedsAMNudge: true, ResolvedVia: ResolvedViaNone}
}

// StubOwnerEmail is the seam for resolving an account's Account Manager
// email from ownerId. No path exists yet today, but one is confirmed in
// progress (Sajith): a new endpoint returning users with id, name, and email
// directly, covering both Owner and the in-progress Renewal Account
// Manager. Swap this function's body once that ships; no other code needs
// to change.
func StubOwnerEmail(accountID string) (string, error) {
	return "", nil
}

func hasBusinessContactRole(c ProjectContact) bool {
	for _, r := range c.Roles {
		if r == businessContactRole {
			return true
		}
	}
	return false
}
