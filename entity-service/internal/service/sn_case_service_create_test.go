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
	"net/http"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

const testProjectUUID = "66666666-6666-6666-6666-666666666666"

// TestSNCaseService_CreateCase_EngagementValidation verifies that an
// engagement CreateCaseRequest requires subject, description, and a valid
// engagementType, matching the ServiceNow-side validation.
func TestSNCaseService_CreateCase_EngagementValidation(t *testing.T) {
	baseReq := domain.CreateCaseRequest{
		Type:              "engagement",
		ProjectID:         testProjectUUID,
		DeploymentID:      testDeploymentUUID,
		DeployedProductID: testDeployedProdID,
	}

	tests := []struct {
		name string
		req  domain.CreateCaseRequest
	}{
		{name: "missing subject", req: baseReq},
		{name: "missing description", req: func() domain.CreateCaseRequest { r := baseReq; r.Subject = "Migration planning"; return r }()},
		{name: "invalid engagementType", req: func() domain.CreateCaseRequest {
			r := baseReq
			r.Subject = "Migration planning"
			r.Description = "Plan the migration"
			r.EngagementType = "not_a_real_type"
			return r
		}()},
	}

	// client is intentionally nil: every case must fail validation before touching it.
	svc := NewServiceNowCaseService(nil, nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.CreateCase(contextWithUserIDToken("token"), tt.req)
			if _, ok := err.(*apierror.ValidationError); !ok {
				t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
			}
		})
	}
}

// TestSNCaseService_CreateCase_Engagement verifies a valid engagement request
// builds the expected snCreateCasePayload (title/description/engagementType)
// and maps a successful ServiceNow response back to domain.CreateCaseResponse.
func TestSNCaseService_CreateCase_Engagement(t *testing.T) {
	var gotBody map[string]any
	client := newTestCaseClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{
			"message": "Case created successfully",
			"case": {"id": "` + testWLCaseSysid + `", "number": "CS0000001", "createdBy": "engineer@example.com", "createdOn": "2026-01-02 10:00:00", "state": {"id": 1, "label": "Open"}}
		}`))
	})

	svc := NewServiceNowCaseService(client, nil)
	req := domain.CreateCaseRequest{
		Type:              "engagement",
		ProjectID:         testProjectUUID,
		DeploymentID:      testDeploymentUUID,
		DeployedProductID: testDeployedProdID,
		Subject:           "Migration planning",
		Description:       "Plan the migration",
		EngagementType:    domain.EngagementTypeMigration,
	}

	resp, err := svc.CreateCase(contextWithUserIDToken("token"), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Case.Number != "CS0000001" {
		t.Fatalf("unexpected case number: %s", resp.Case.Number)
	}

	if gotBody["title"] != "Migration planning" {
		t.Fatalf("payload title: got %v, want %q", gotBody["title"], "Migration planning")
	}
	if gotBody["description"] != "Plan the migration" {
		t.Fatalf("payload description: got %v, want %q", gotBody["description"], "Plan the migration")
	}
	if gotBody["engagementType"] != float64(1) {
		t.Fatalf("payload engagementType: got %v, want 1", gotBody["engagementType"])
	}
	if gotBody["type"] != "engagement" {
		t.Fatalf("payload type: got %v, want %q", gotBody["type"], "engagement")
	}
}

// TestSNCaseService_CreateCase_SecurityReportAnalysis_AttachmentsOptional verifies
// that a security_report_analysis request with zero attachments is accepted --
// attachments are uploaded via a separate request after the case is created, not
// bundled into this one, so they must not be required here.
func TestSNCaseService_CreateCase_SecurityReportAnalysis_AttachmentsOptional(t *testing.T) {
	var gotBody map[string]any
	client := newTestCaseClient(t, func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{
			"message": "Case created successfully",
			"case": {"id": "` + testWLCaseSysid + `", "number": "CS0000002", "createdBy": "engineer@example.com", "createdOn": "2026-01-02 10:00:00", "state": {"id": 1, "label": "Open"}}
		}`))
	})

	svc := NewServiceNowCaseService(client, nil)
	req := domain.CreateCaseRequest{
		Type:              "security_report_analysis",
		ProjectID:         testProjectUUID,
		DeploymentID:      testDeploymentUUID,
		DeployedProductID: testDeployedProdID,
		Subject:           "Suspicious log entries",
		Description:       "Found several suspicious entries in the access log",
	}

	resp, err := svc.CreateCase(contextWithUserIDToken("token"), req)
	if err != nil {
		t.Fatalf("unexpected error with zero attachments: %v", err)
	}
	if resp.Case.Number != "CS0000002" {
		t.Fatalf("unexpected case number: %s", resp.Case.Number)
	}
	if _, present := gotBody["attachments"]; present {
		t.Fatalf("expected no attachments field in payload when none provided, got %v", gotBody["attachments"])
	}
}
