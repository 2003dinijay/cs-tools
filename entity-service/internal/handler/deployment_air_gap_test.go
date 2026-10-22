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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

type fakeAirGapSvc struct {
	gotID  string
	gotReq domain.SetDeploymentAirGappedRequest
	err    error
}

func (f *fakeAirGapSvc) SetAirGapped(_ context.Context, id string, req domain.SetDeploymentAirGappedRequest) (domain.DeploymentAirGapped, error) {
	f.gotID, f.gotReq = id, req
	if f.err != nil {
		return domain.DeploymentAirGapped{}, f.err
	}
	return domain.DeploymentAirGapped{ID: id, AirGapped: *req.AirGapped, UpdatedBy: "csm.user@wso2.com"}, nil
}

func TestDeploymentAirGapHandler(t *testing.T) {
	route := func(h *DeploymentAirGapHandler) *http.ServeMux {
		mux := http.NewServeMux()
		mux.HandleFunc("PUT /deployments/{id}/air-gapped", h.SetAirGapped)
		return mux
	}

	t.Run("200 passes the path id and body on", func(t *testing.T) {
		svc := &fakeAirGapSvc{}
		w := httptest.NewRecorder()
		route(NewDeploymentAirGapHandler(svc)).ServeHTTP(w, httptest.NewRequest(http.MethodPut, "/deployments/dep-1/air-gapped", strings.NewReader(`{"airGapped":true}`)))
		if w.Code != http.StatusOK {
			t.Fatalf("status %d", w.Code)
		}
		var resp domain.DeploymentAirGapped
		_ = json.NewDecoder(w.Body).Decode(&resp)
		if svc.gotID != "dep-1" || svc.gotReq.AirGapped == nil || !*svc.gotReq.AirGapped || !resp.AirGapped {
			t.Errorf("service saw %q %+v, response %+v", svc.gotID, svc.gotReq, resp)
		}
	})

	t.Run("an unknown deployment is 404", func(t *testing.T) {
		svc := &fakeAirGapSvc{err: &apierror.NotFoundError{Msg: "deployment not found"}}
		w := httptest.NewRecorder()
		route(NewDeploymentAirGapHandler(svc)).ServeHTTP(w, httptest.NewRequest(http.MethodPut, "/deployments/dep-1/air-gapped", strings.NewReader(`{"airGapped":false}`)))
		if w.Code != http.StatusNotFound {
			t.Fatalf("status %d, want 404", w.Code)
		}
	})

	t.Run("a malformed body is 400", func(t *testing.T) {
		w := httptest.NewRecorder()
		route(NewDeploymentAirGapHandler(&fakeAirGapSvc{})).ServeHTTP(w, httptest.NewRequest(http.MethodPut, "/deployments/dep-1/air-gapped", strings.NewReader(`{"airGapped":"yes"}`)))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status %d, want 400", w.Code)
		}
	})
}
