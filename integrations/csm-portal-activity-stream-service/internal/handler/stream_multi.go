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
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-portal-activity-stream-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-portal-activity-stream-service/internal/middleware"
)

const (
	// maxMultiplexCaseIDs caps how many distinct case ids one multiplexed
	// stream may subscribe to. It bounds the per-request upstream ACL fan-out
	// and the number of hub registrations a single connection can hold.
	maxMultiplexCaseIDs = 20

	// multiplexAuthConcurrency bounds concurrent upstream GetCase ACL checks
	// for one multiplexed request.
	multiplexAuthConcurrency = 5
)

// parseCaseIDs splits a comma-separated `ids` query value into trimmed,
// de-duplicated (case-insensitively, first spelling kept), UUID-validated ids
// in first-seen order. It returns false for an empty list, any malformed id,
// or more than maxMultiplexCaseIDs distinct ids.
func parseCaseIDs(raw string) ([]string, bool) {
	seen := make(map[string]bool)
	var ids []string
	for _, part := range strings.Split(raw, ",") {
		id := strings.TrimSpace(part)
		if id == "" || !uuidRe.MatchString(id) {
			return nil, false
		}
		key := strings.ToLower(id)
		if seen[key] {
			continue
		}
		seen[key] = true
		ids = append(ids, id)
	}
	if len(ids) == 0 || len(ids) > maxMultiplexCaseIDs {
		return nil, false
	}
	return ids, true
}

// authResult is the outcome of the upstream read check for one case id.
type authResult struct {
	ok  bool
	err error
}

// StreamMultipleCaseActivities handles GET /cases/activities/stream?ids=a,b,c:
// one Server-Sent Events connection carrying the same `case_updated` events as
// StreamCaseActivities for every requested case the caller may read. Each
// frame's payload already contains its caseId, so the client demultiplexes.
//
// Partial-authorization policy: every id is checked with the same upstream
// GetCase ACL check as the single-case endpoint, before any hub registration.
//   - An id the caller may not read (upstream 403 or 404) is silently dropped
//     from the subscription: the response never reveals which ids were dropped,
//     and no event for them is ever delivered.
//   - If no id is authorized, the first (in request order) upstream error is
//     returned, mapped exactly as the single-case endpoint maps it (403/404).
//   - Any other upstream failure (401, 5xx, transport error) fails the whole
//     request with its mapped error rather than dropping the id, so a transient
//     outage cannot silently leave a case unsubscribed; the client retries.
//
// Same listener, middleware.Auth chain, headers, 15s heartbeat and known
// limitation (no per-user connection cap) as StreamCaseActivities; this
// endpoint additionally multiplies one connection's hub registrations by up to
// maxMultiplexCaseIDs.
func (h *StreamHandler) StreamMultipleCaseActivities(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	ids, ok := parseCaseIDs(r.URL.Query().Get("ids"))
	if !ok {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	if h.hub == nil {
		writeError(w, http.StatusServiceUnavailable, "Live updates are not available right now.")
		return
	}

	authorized, err := h.authorizeCaseIDs(r, user.UserID, ids)
	if err != nil {
		mapUpstreamErrorGeneric(w, err, "Failed to open the case activity stream.")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ctx := r.Context()
	merged := make(chan string)
	done := make(chan struct{})
	var wg sync.WaitGroup

	// Cleanup order matters: close done so forwarders blocked on `merged`
	// release, Unregister (closes each hub channel, releasing forwarders
	// blocked on receive), then wait so no goroutine outlives the handler.
	chans := make(map[string]chan string, len(authorized))
	defer func() {
		close(done)
		for id, ch := range chans {
			h.hub.Unregister(id, ch)
		}
		wg.Wait()
	}()

	for _, id := range authorized {
		ch := h.hub.Register(id)
		chans[id] = ch
		wg.Add(1)
		go func(ch chan string) {
			defer wg.Done()
			for {
				select {
				case payload, ok := <-ch:
					if !ok {
						return
					}
					select {
					case merged <- payload:
					case <-done:
						return
					}
				case <-done:
					return
				}
			}
		}(ch)
	}

	ticker := time.NewTicker(caseActivityStreamHeartbeat)
	defer ticker.Stop()

	slog.InfoContext(ctx, "multiplexed case activity stream connected", "userID", user.UserID, "requested", len(ids), "subscribed", len(authorized))

	for {
		select {
		case <-ctx.Done():
			slog.InfoContext(ctx, "multiplexed case activity stream disconnected", "userID", user.UserID)
			return
		case <-ticker.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case payload := <-merged:
			// Compact single-line JSON; see StreamCaseActivities.
			if _, err := fmt.Fprintf(w, "event: case_updated\ndata: %s\n\n", payload); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

// authorizeCaseIDs runs the upstream read check for every id (at most
// multiplexAuthConcurrency in flight) and returns the readable ids in request
// order, or the error to report; see StreamMultipleCaseActivities for policy.
func (h *StreamHandler) authorizeCaseIDs(r *http.Request, userID string, ids []string) ([]string, error) {
	ctx := r.Context()
	results := make([]authResult, len(ids))
	sem := make(chan struct{}, multiplexAuthConcurrency)
	var wg sync.WaitGroup
	for i, id := range ids {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			_, err := h.entityClient.GetCase(ctx, id)
			if err != nil {
				slog.ErrorContext(ctx, "entity GetCase failed during multiplexed stream authorization", "userID", userID, "caseID", id, "err", err)
			}
			results[i] = authResult{ok: err == nil, err: err}
		}()
	}
	wg.Wait()

	var authorized []string
	var firstErr error
	for i, res := range results {
		if res.ok {
			authorized = append(authorized, ids[i])
			continue
		}
		if !isNoAccess(res.err) {
			return nil, res.err
		}
		if firstErr == nil {
			firstErr = res.err
		}
	}
	if len(authorized) == 0 {
		return nil, firstErr
	}
	return authorized, nil
}

// isNoAccess reports whether err is the upstream saying the caller cannot read
// the case (403) or it is not visible to them (404).
func isNoAccess(err error) bool {
	var apiErr *apierror.Error
	if !errors.As(err, &apiErr) {
		return false
	}
	return apiErr.StatusCode == http.StatusForbidden || apiErr.StatusCode == http.StatusNotFound
}
