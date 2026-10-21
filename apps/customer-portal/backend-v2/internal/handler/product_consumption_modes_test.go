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
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/productconsumption"
)

func newUsageMuxWithMode(product productConsumptionClient, entityClient entityProjectAccessChecker, mode productconsumption.UsageImportMode) *http.ServeMux {
	h := NewProductConsumptionHandler(product, entityClient).WithUsageImportMode(mode)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /deployment-usages", h.ImportDeploymentUsage)
	return mux
}

// Legacy usage import (the behaviour before the report pre-check): no report
// inspection and no deployment lookup, the upload is forwarded as is.

func TestImportDeploymentUsageLegacy_ForwardsWithoutInspectingTheReport(t *testing.T) {
	msg := "Usage imported successfully."
	product := &fakeLicenseProductConsumptionClient{importRes: productconsumption.ImportUsageResponse{Message: &msg}}
	entityClient := &fakeProjectAccessChecker{}

	mux := newUsageMuxWithMode(product, entityClient, productconsumption.UsageImportModeLegacy)
	body := []byte("not-even-a-real-zip")
	req := authedBinaryRequest(http.MethodPost, "/deployment-usages", body, "application/zip")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if entityClient.searchCalls != 0 {
		t.Errorf("SearchDeployments calls = %d, want 0 in legacy mode", entityClient.searchCalls)
	}
	if product.importCalls != 1 || string(product.gotImportZip) != string(body) || product.gotImportEmail != "customer@example.com" {
		t.Errorf("import not forwarded as is: calls=%d email=%q zip=%q", product.importCalls, product.gotImportEmail, product.gotImportZip)
	}
}

func TestImportDeploymentUsageLegacy_StillRejectsNonZipContentType(t *testing.T) {
	product := &fakeLicenseProductConsumptionClient{}
	mux := newUsageMuxWithMode(product, &fakeProjectAccessChecker{}, productconsumption.UsageImportModeLegacy)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, authedBinaryRequest(http.MethodPost, "/deployment-usages", []byte("x"), "text/plain"))
	if rec.Code != http.StatusBadRequest || product.importCalls != 0 {
		t.Fatalf("status = %d importCalls = %d, want 400 and 0", rec.Code, product.importCalls)
	}
}

func TestImportDeploymentUsageLegacy_MapsUpstreamError(t *testing.T) {
	product := &fakeLicenseProductConsumptionClient{importErr: http.ErrAbortHandler}
	mux := newUsageMuxWithMode(product, &fakeProjectAccessChecker{}, productconsumption.UsageImportModeLegacy)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, authedBinaryRequest(http.MethodPost, "/deployment-usages", []byte("x"), "application/zip"))
	if rec.Code < 500 {
		t.Fatalf("status = %d, want a 5xx for an upstream failure", rec.Code)
	}
}

// TestUsageImportMode_ZeroValueIsService pins the default: a handler built
// without WithUsageImportMode runs the service-mode pre-check.
func TestUsageImportMode_ZeroValueIsService(t *testing.T) {
	product := &fakeLicenseProductConsumptionClient{}
	entityClient := &fakeProjectAccessChecker{}
	mux := newUsageMux(product, entityClient)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, authedBinaryRequest(http.MethodPost, "/deployment-usages", []byte("not-a-zip"), "application/zip"))
	if rec.Code != http.StatusBadRequest || product.importCalls != 0 {
		t.Fatalf("status = %d importCalls = %d, want 400 and 0", rec.Code, product.importCalls)
	}
}

// TestModeCombinations wires the real client and handler against one fake
// upstream for all four license x usage-import combinations and checks that
// each flag dispatches independently of the other.
func TestModeCombinations(t *testing.T) {
	for _, lm := range []string{"", "service", "legacy"} {
		for _, um := range []string{"", "service", "legacy"} {
			t.Run("license="+lm+",usage="+um, func(t *testing.T) {
				licenseMode, err := productconsumption.ParseLicenseMode(lm)
				if err != nil {
					t.Fatal(err)
				}
				usageMode, err := productconsumption.ParseUsageImportMode(um)
				if err != nil {
					t.Fatal(err)
				}
				wantLegacyLicense := lm == "legacy"
				wantLegacyUsage := um == "legacy"

				tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(`{"access_token":"t","token_type":"Bearer","expires_in":3600}`))
				}))
				defer tokenSrv.Close()
				var mu sync.Mutex
				var paths []string
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					mu.Lock()
					paths = append(paths, r.URL.Path)
					mu.Unlock()
					switch {
					case strings.HasSuffix(r.URL.Path, "/consumption/status"):
						_, _ = w.Write([]byte(`{"result":{"status":5,"applicationId":"app-1"}}`))
					case r.URL.Path == "/deployment-usages":
						_, _ = w.Write([]byte(`{"message":"ok"}`))
					default:
						_, _ = w.Write([]byte(`{"result":{"success":true,"license":{"subscriptionData":{},"signature":"sig"}}}`))
					}
				}))
				defer srv.Close()

				client := productconsumption.NewClient(productconsumption.Config{
					SubscriptionBaseURL: srv.URL, TokenURL: tokenSrv.URL, ClientID: "id", ClientSecret: "s",
					LicenseMode: licenseMode,
				})
				entityClient := &fakeProjectAccessChecker{} // sees no deployment
				h := NewProductConsumptionHandler(client, entityClient).WithUsageImportMode(usageMode)
				mux := http.NewServeMux()
				mux.HandleFunc("POST /projects/{projectId}/deployments/{deploymentId}/license", h.GetDeploymentLicense)
				mux.HandleFunc("POST /deployment-usages", h.ImportDeploymentUsage)

				// License.
				rec := httptest.NewRecorder()
				mux.ServeHTTP(rec, authedRequest(http.MethodPost, "/projects/"+testLicenseProjectID+"/deployments/"+testLicenseDeploymentID+"/license", ""))
				if rec.Code != http.StatusOK {
					t.Fatalf("license status = %d: %s", rec.Code, rec.Body.String())
				}
				mu.Lock()
				gotStatusCall := false
				for _, p := range paths {
					if strings.HasSuffix(p, "/consumption/status") {
						gotStatusCall = true
					}
				}
				mu.Unlock()
				if gotStatusCall != wantLegacyLicense {
					t.Errorf("status call = %v, want %v (paths %v)", gotStatusCall, wantLegacyLicense, paths)
				}

				// Usage import: a deployment the caller cannot see.
				report := usageReportZip(t, map[string]string{"usage-report.json": testUsageReport})
				rec = httptest.NewRecorder()
				mux.ServeHTTP(rec, authedBinaryRequest(http.MethodPost, "/deployment-usages", report, "application/zip"))
				wantCode := http.StatusNotFound
				if wantLegacyUsage {
					wantCode = http.StatusOK
				}
				if rec.Code != wantCode {
					t.Errorf("usage status = %d, want %d: %s", rec.Code, wantCode, rec.Body.String())
				}
				if wantLegacyUsage != (entityClient.searchCalls == 0) {
					t.Errorf("SearchDeployments calls = %d with legacy usage = %v", entityClient.searchCalls, wantLegacyUsage)
				}
			})
		}
	}
}
