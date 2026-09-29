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

package employeeinfo

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
)

func newTestTokenServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "test-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	}))
}

func newTestClient(t *testing.T, tokenSrv, apiSrv *httptest.Server) *Client {
	t.Helper()
	return NewClient(Config{
		BaseURL:      apiSrv.URL,
		TokenURL:     tokenSrv.URL,
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
	})
}

func TestGetEmployeeData_ReturnsEmployee(t *testing.T) {
	var capturedAuth string
	var capturedVars struct {
		Variables struct {
			WorkEmail string `json:"workEmail"`
		} `json:"variables"`
	}
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&capturedVars); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"employee": map[string]any{
					"firstName":         "Jane",
					"lastName":          "Doe",
					"employeeThumbnail": "https://example.com/thumb.png",
				},
			},
		})
	}))
	defer apiSrv.Close()
	tokenSrv := newTestTokenServer(t)
	defer tokenSrv.Close()

	c := newTestClient(t, tokenSrv, apiSrv)

	employee, err := c.GetEmployeeData(context.Background(), "jane.doe@example.com")
	if err != nil {
		t.Fatalf("GetEmployeeData returned error: %v", err)
	}
	if employee.FirstName != "Jane" || employee.LastName != "Doe" {
		t.Errorf("employee = %+v, want FirstName=Jane LastName=Doe", employee)
	}
	if employee.EmployeeThumbnail == nil || *employee.EmployeeThumbnail != "https://example.com/thumb.png" {
		t.Errorf("EmployeeThumbnail = %v, want a pointer to the thumbnail URL", employee.EmployeeThumbnail)
	}
	if capturedAuth != "Bearer test-token" {
		t.Errorf("Authorization header = %q, want %q", capturedAuth, "Bearer test-token")
	}
	if capturedVars.Variables.WorkEmail != "jane.doe@example.com" {
		t.Errorf("workEmail variable = %q, want %q", capturedVars.Variables.WorkEmail, "jane.doe@example.com")
	}
}

func TestGetEmployeeData_NilThumbnail(t *testing.T) {
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"employee": map[string]any{
					"firstName":         "Jane",
					"lastName":          "Doe",
					"employeeThumbnail": nil,
				},
			},
		})
	}))
	defer apiSrv.Close()
	tokenSrv := newTestTokenServer(t)
	defer tokenSrv.Close()

	c := newTestClient(t, tokenSrv, apiSrv)

	employee, err := c.GetEmployeeData(context.Background(), "jane.doe@example.com")
	if err != nil {
		t.Fatalf("GetEmployeeData returned error: %v", err)
	}
	if employee.EmployeeThumbnail != nil {
		t.Errorf("EmployeeThumbnail = %v, want nil", *employee.EmployeeThumbnail)
	}
}

func TestGetEmployeeData_MapsGraphQLErrorsToBadGateway(t *testing.T) {
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data":   map[string]any{"employee": map[string]any{}},
			"errors": []map[string]any{{"message": "employee not found"}},
		})
	}))
	defer apiSrv.Close()
	tokenSrv := newTestTokenServer(t)
	defer tokenSrv.Close()

	c := newTestClient(t, tokenSrv, apiSrv)

	_, err := c.GetEmployeeData(context.Background(), "nobody@example.com")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	var apiErr *apierror.Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *apierror.Error, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusBadGateway {
		t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, http.StatusBadGateway)
	}
}

func TestGetEmployeeData_MapsHTTPStatusError(t *testing.T) {
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("upstream down"))
	}))
	defer apiSrv.Close()
	tokenSrv := newTestTokenServer(t)
	defer tokenSrv.Close()

	c := newTestClient(t, tokenSrv, apiSrv)

	_, err := c.GetEmployeeData(context.Background(), "jane.doe@example.com")
	var apiErr *apierror.Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *apierror.Error, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, http.StatusServiceUnavailable)
	}
}
