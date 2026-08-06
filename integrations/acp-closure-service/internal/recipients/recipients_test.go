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

package recipients

import "testing"

func TestAccountManagerEmail_ReturnsEmailWhenPopulated(t *testing.T) {
	email := "am@wso2.example"
	am := &PersonRef{ID: "am-1", Name: "Jordan Perera", Email: &email}

	if got := AccountManagerEmail(am); got != email {
		t.Errorf("AccountManagerEmail() = %q, want %q", got, email)
	}
}

func TestAccountManagerEmail_ReturnsEmptyWhenNoAccountManager(t *testing.T) {
	if got := AccountManagerEmail(nil); got != "" {
		t.Errorf("AccountManagerEmail(nil) = %q, want \"\"", got)
	}
}

func TestAccountManagerEmail_ReturnsEmptyWhenAccountManagerHasNoEmail(t *testing.T) {
	am := &PersonRef{ID: "am-1", Name: "Jordan Perera", Email: nil}

	if got := AccountManagerEmail(am); got != "" {
		t.Errorf("AccountManagerEmail() = %q, want \"\"", got)
	}
}

func TestResolveCustomerContact_PrefersBusinessContact(t *testing.T) {
	projectContacts := []ProjectContact{
		{Name: "Alice", Email: "alice@customer.example", Roles: []string{"developer"}},
		{Name: "Bob", Email: "bob@customer.example", Roles: []string{businessContactRole, "developer"}},
	}
	accountContacts := []AccountContact{
		{Name: "Carol", Email: "carol@customer.example", IsPrimary: true},
	}

	got := ResolveCustomerContact(projectContacts, accountContacts)

	if got.NeedsAMNudge {
		t.Fatalf("NeedsAMNudge = true, want false")
	}
	if got.ResolvedVia != ResolvedViaBusinessContact {
		t.Errorf("ResolvedVia = %v, want %v", got.ResolvedVia, ResolvedViaBusinessContact)
	}
	if got.CustomerContact == nil || got.CustomerContact.Email != "bob@customer.example" {
		t.Errorf("CustomerContact = %+v, want Bob", got.CustomerContact)
	}
}

func TestResolveCustomerContact_FallsBackToPrimaryContact(t *testing.T) {
	projectContacts := []ProjectContact{
		{Name: "Alice", Email: "alice@customer.example", Roles: []string{"developer"}},
	}
	accountContacts := []AccountContact{
		{Name: "Dana", Email: "dana@customer.example", IsPrimary: false},
		{Name: "Carol", Email: "carol@customer.example", IsPrimary: true},
	}

	got := ResolveCustomerContact(projectContacts, accountContacts)

	if got.NeedsAMNudge {
		t.Fatalf("NeedsAMNudge = true, want false")
	}
	if got.ResolvedVia != ResolvedViaPrimaryContact {
		t.Errorf("ResolvedVia = %v, want %v", got.ResolvedVia, ResolvedViaPrimaryContact)
	}
	if got.CustomerContact == nil || got.CustomerContact.Email != "carol@customer.example" {
		t.Errorf("CustomerContact = %+v, want Carol", got.CustomerContact)
	}
}

// TestResolveCustomerContact_SkipsBusinessContactWithEmptyEmail covers a
// real, unremarkable data state (mirrors AccountManagerEmail's treatment of
// "assigned but no email" elsewhere in this package): a Project Contact has
// the business-contact role but no email on file. Accepting it anyway would
// resolve to Recipient: "" instead of falling through to a usable tier.
func TestResolveCustomerContact_SkipsBusinessContactWithEmptyEmail(t *testing.T) {
	projectContacts := []ProjectContact{
		{Name: "Bob", Email: "", Roles: []string{businessContactRole}},
	}
	accountContacts := []AccountContact{
		{Name: "Carol", Email: "carol@customer.example", IsPrimary: true},
	}

	got := ResolveCustomerContact(projectContacts, accountContacts)

	if got.NeedsAMNudge {
		t.Fatalf("NeedsAMNudge = true, want false")
	}
	if got.ResolvedVia != ResolvedViaPrimaryContact {
		t.Errorf("ResolvedVia = %v, want %v", got.ResolvedVia, ResolvedViaPrimaryContact)
	}
	if got.CustomerContact == nil || got.CustomerContact.Email != "carol@customer.example" {
		t.Errorf("CustomerContact = %+v, want Carol", got.CustomerContact)
	}
}

// TestResolveCustomerContact_NudgesAccountManagerWhenOnlyContactsHaveEmptyEmail
// covers both tiers resolving to a real contact, but neither having a usable
// email — must fall through to NeedsAMNudge rather than resolving to an
// empty Recipient.
func TestResolveCustomerContact_NudgesAccountManagerWhenOnlyContactsHaveEmptyEmail(t *testing.T) {
	projectContacts := []ProjectContact{
		{Name: "Bob", Email: "", Roles: []string{businessContactRole}},
	}
	accountContacts := []AccountContact{
		{Name: "Carol", Email: "", IsPrimary: true},
	}

	got := ResolveCustomerContact(projectContacts, accountContacts)

	if !got.NeedsAMNudge {
		t.Fatalf("NeedsAMNudge = false, want true")
	}
	if got.CustomerContact != nil {
		t.Errorf("CustomerContact = %+v, want nil", got.CustomerContact)
	}
	if got.ResolvedVia != ResolvedViaNone {
		t.Errorf("ResolvedVia = %v, want %v", got.ResolvedVia, ResolvedViaNone)
	}
}

func TestResolveCustomerContact_NudgesAccountManagerWhenNoContactFound(t *testing.T) {
	projectContacts := []ProjectContact{
		{Name: "Alice", Email: "alice@customer.example", Roles: []string{"developer"}},
	}
	accountContacts := []AccountContact{
		{Name: "Dana", Email: "dana@customer.example", IsPrimary: false},
	}

	got := ResolveCustomerContact(projectContacts, accountContacts)

	if !got.NeedsAMNudge {
		t.Fatalf("NeedsAMNudge = false, want true")
	}
	if got.CustomerContact != nil {
		t.Errorf("CustomerContact = %+v, want nil", got.CustomerContact)
	}
	if got.ResolvedVia != ResolvedViaNone {
		t.Errorf("ResolvedVia = %v, want %v", got.ResolvedVia, ResolvedViaNone)
	}
}

func TestResolveCustomerContact_NudgesAccountManagerWhenNoContactsAtAll(t *testing.T) {
	got := ResolveCustomerContact(nil, nil)

	if !got.NeedsAMNudge {
		t.Fatalf("NeedsAMNudge = false, want true")
	}
	if got.CustomerContact != nil {
		t.Errorf("CustomerContact = %+v, want nil", got.CustomerContact)
	}
	if got.ResolvedVia != ResolvedViaNone {
		t.Errorf("ResolvedVia = %v, want %v", got.ResolvedVia, ResolvedViaNone)
	}
}
