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
	"encoding/json"
	"net/http"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// AvailabilityHandler triggers the availability recalculation.
//
// *** INTERNAL ONLY, AND IT WRITES. *** Every other /cloud-status route is a
// read the dashboard makes. This one recomputes and REPLACES rows, so it
// lives under /internal/ beside the cloud-status sweep and is reachable only
// by the scheduled task's client credential.
type AvailabilityHandler struct {
	svc service.AvailabilityService
}

// NewAvailabilityHandler constructs the handler.
func NewAvailabilityHandler(svc service.AvailabilityService) *AvailabilityHandler {
	return &AvailabilityHandler{svc: svc}
}

// Sweep handles POST /internal/availability/sweep.
//
// The clock is taken HERE rather than inside the service so the whole run
// shares one instant. Reading time.Now() per subject would let a run that
// straddles midnight compute some subjects against today and the rest
// against tomorrow — producing two different daily periods in one sweep,
// with no error and no obvious symptom beyond a day that never fills in.
func (h *AvailabilityHandler) Sweep(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.Sweep(r.Context(), time.Now().UTC())
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}
