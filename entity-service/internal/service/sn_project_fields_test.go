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

package service

import (
	"encoding/json"
	"testing"
)

// TestSNProjectDetails_DecodesEngagementFields is the guard for the bug class
// this change closes: a field ServiceNow sends but no Go struct declares is
// dropped silently by encoding/json, with no error anywhere. That is how the
// Usage & Metrics page went blank (deployedProductCount) and how these ten
// project fields came to be missing.
func TestSNProjectDetails_DecodesEngagementFields(t *testing.T) {
	// Shaped as ServiceNow sends it, including the fields on the nested account.
	payload := `{
	  "id": "6fa0b42d1bfaa694a002c9d3604bcb77",
	  "name": "Acme Production",
	  "key": "ACME-PROD",
	  "sfId": "0011x00000ABCDE",
	  "createdOn": "2026-01-05 09:12:44",
	  "startDate": "2026-01-06",
	  "endDate": "2027-01-05",
	  "type": {"name": "Managed Cloud Subscription"},
	  "totalQueryHours": 40,
	  "consumedQueryHours": 12.5,
	  "remainingQueryHours": 27.5,
	  "totalOnboardingHours": 80,
	  "consumedOnboardingHours": 80,
	  "remainingOnboardingHours": 0,
	  "goLiveDate": "2026-03-01",
	  "goLivePlanDate": "2026-02-15",
	  "onboardingExpiryDate": "2026-06-30",
	  "onboardingStatus": "Completed",
	  "account": {
	    "id": "aa11bb22cc33dd44ee55ff6677889900",
	    "name": "Acme Corp",
	    "activationDate": "2026-01-06",
	    "deactivationDate": "2027-01-06",
	    "supportTier": "Platinum",
	    "region": "US",
	    "hasAgent": true,
	    "hasKbReferences": true,
	    "ownerEmail": "owner@acme.invalid",
	    "technicalOwnerEmail": "tech@acme.invalid"
	  }
	}`

	var sn snProjectDetailsResponse
	if err := json.Unmarshal([]byte(payload), &sn); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if sn.TotalQueryHours == nil || *sn.TotalQueryHours != 40 {
		t.Errorf("totalQueryHours = %v, want 40", sn.TotalQueryHours)
	}
	if sn.ConsumedQueryHours == nil || *sn.ConsumedQueryHours != 12.5 {
		t.Errorf("consumedQueryHours = %v, want 12.5 (a decimal upstream)", sn.ConsumedQueryHours)
	}
	// Zero must survive as a real value, not collapse to nil — "tracked, none
	// remaining" is a different fact from "not tracked".
	if sn.RemainingOnboardingHours == nil || *sn.RemainingOnboardingHours != 0 {
		t.Errorf("remainingOnboardingHours = %v, want a non-nil 0", sn.RemainingOnboardingHours)
	}
	if sn.OnboardingStatus == nil || *sn.OnboardingStatus != "Completed" {
		t.Errorf("onboardingStatus = %v, want Completed", sn.OnboardingStatus)
	}
	if sn.GoLiveDate == nil || *sn.GoLiveDate != "2026-03-01" {
		t.Errorf("goLiveDate = %v, want 2026-03-01", sn.GoLiveDate)
	}
	if sn.Account.OwnerEmail == nil || *sn.Account.OwnerEmail != "owner@acme.invalid" {
		t.Errorf("account.ownerEmail = %v, want it decoded from the nested account", sn.Account.OwnerEmail)
	}
	if sn.Account.TechnicalOwnerEmail == nil || *sn.Account.TechnicalOwnerEmail != "tech@acme.invalid" {
		t.Errorf("account.technicalOwnerEmail = %v", sn.Account.TechnicalOwnerEmail)
	}
}

// TestSNProjectDetails_OmittedFieldsStayNil checks a project with none of these
// tracked decodes cleanly with nil balances rather than zeroes, so the portal can
// tell "not tracked" from "none remaining".
func TestSNProjectDetails_OmittedFieldsStayNil(t *testing.T) {
	var sn snProjectDetailsResponse
	if err := json.Unmarshal([]byte(`{"id":"x","name":"n","key":"k","account":{"id":"a"}}`), &sn); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for name, got := range map[string]*float64{
		"totalQueryHours":      sn.TotalQueryHours,
		"consumedQueryHours":   sn.ConsumedQueryHours,
		"totalOnboardingHours": sn.TotalOnboardingHours,
	} {
		if got != nil {
			t.Errorf("%s = %v, want nil when absent upstream", name, *got)
		}
	}
	if sn.OnboardingStatus != nil || sn.Account.OwnerEmail != nil {
		t.Error("absent optional strings should stay nil")
	}
}

// TestSNProjectSearch_DecodesActiveCasesCount covers the search-list item, where
// activeCasesCount lives — the portal's ProjectListItem types it as required.
func TestSNProjectSearch_DecodesActiveCasesCount(t *testing.T) {
	var p snProject
	if err := json.Unmarshal([]byte(`{
	  "id":"6fa0b42d1bfaa694a002c9d3604bcb77","name":"Acme","key":"ACME",
	  "createdOn":"2026-01-05 09:12:44","type":{"name":"Managed Cloud Subscription"},
	  "account":{"id":"aa11","name":"Acme Corp"},"activeCasesCount":7}`), &p); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if p.ActiveCasesCount != 7 {
		t.Errorf("activeCasesCount = %d, want 7", p.ActiveCasesCount)
	}
}

// TestOptionalSNProjectDate covers the shared helper: absent and empty both yield
// nil, a valid date parses, and a malformed one is an error rather than a silent
// nil — matching how account activationDate already behaved.
func TestOptionalSNProjectDate(t *testing.T) {
	if got, err := optionalSNProjectDate("goLiveDate", nil); err != nil || got != nil {
		t.Errorf("nil input: got %v, %v; want nil, nil", got, err)
	}
	empty := ""
	if got, err := optionalSNProjectDate("goLiveDate", &empty); err != nil || got != nil {
		t.Errorf("empty input: got %v, %v; want nil, nil", got, err)
	}
	valid := "2026-03-01"
	got, err := optionalSNProjectDate("goLiveDate", &valid)
	if err != nil {
		t.Fatalf("valid date returned an error: %v", err)
	}
	if got == nil || got.Format("2006-01-02") != valid {
		t.Errorf("got %v, want %s", got, valid)
	}
	bad := "01/03/2026"
	if _, err := optionalSNProjectDate("goLiveDate", &bad); err == nil {
		t.Error("malformed date returned no error; a silent nil would hide upstream data problems")
	}
}
