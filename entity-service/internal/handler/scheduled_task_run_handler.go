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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// ScheduledTaskRunHandler handles HTTP requests for the scheduled_task_run
// resource — see domain.ScheduledTaskRun's doc comment for what it's for.
type ScheduledTaskRunHandler struct {
	svc service.ScheduledTaskRunService
}

// NewScheduledTaskRunHandler constructs a ScheduledTaskRunHandler with the
// given service.
func NewScheduledTaskRunHandler(svc service.ScheduledTaskRunService) *ScheduledTaskRunHandler {
	return &ScheduledTaskRunHandler{svc: svc}
}

// AttemptScheduledTaskRun handles POST /scheduled-tasks/attempts.
func (h *ScheduledTaskRunHandler) AttemptScheduledTaskRun(w http.ResponseWriter, r *http.Request) {
	var req domain.ClaimScheduledTaskRunRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.Attempt(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// UpdateScheduledTaskRunAttempt handles
// PATCH /scheduled-tasks/attempts/{id} — reports an attempt's outcome
// (succeeded or failed, per the request body's status). Replaces what used
// to be two separate action-style endpoints (POST .../complete and
// POST .../fail).
func (h *ScheduledTaskRunHandler) UpdateScheduledTaskRunAttempt(w http.ResponseWriter, r *http.Request) {
	var req domain.UpdateScheduledTaskRunAttemptRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	run, err := h.svc.UpdateAttempt(r.Context(), r.PathValue("id"), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(run)
}

// ListScheduledTaskRuns handles GET /scheduled-tasks/attempts?status=<filter>.
// Monitoring only — not used by operations/csm-scheduled-tasks' own engine.
func (h *ScheduledTaskRunHandler) ListScheduledTaskRuns(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.List(r.Context(), r.URL.Query().Get("status"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// DeleteScheduledTaskRuns handles
// DELETE /scheduled-tasks/attempts?resolvedBefore=<RFC3339 timestamp>.
func (h *ScheduledTaskRunHandler) DeleteScheduledTaskRuns(w http.ResponseWriter, r *http.Request) {
	raw := r.URL.Query().Get("resolvedBefore")
	cutoff, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		apierror.WriteJSON(w, http.StatusBadRequest, "resolvedBefore must be an RFC3339 timestamp")
		return
	}
	resp, err := h.svc.DeleteResolvedBefore(r.Context(), cutoff)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
