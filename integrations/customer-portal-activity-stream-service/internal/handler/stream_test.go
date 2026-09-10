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
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/customer-portal-activity-stream-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/integrations/customer-portal-activity-stream-service/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/integrations/customer-portal-activity-stream-service/internal/stream"
)

const streamTestCaseID = "11111111-1111-1111-1111-111111111111"

var testUser = &middleware.UserInfo{
	Email:  "agent@wso2.com",
	UserID: "uid-123",
	Groups: []string{"customer-portal-users"},
}

type mockEntityCaseClient struct {
	getCaseFn func(ctx context.Context, caseID string) ([]byte, error)
}

func (m *mockEntityCaseClient) GetCase(ctx context.Context, caseID string) ([]byte, error) {
	if m.getCaseFn != nil {
		return m.getCaseFn(ctx, caseID)
	}
	return []byte(`{"id":"` + caseID + `"}`), nil
}

func TestStreamCaseActivities_Unauthorized(t *testing.T) {
	h := NewStreamHandler(&mockEntityCaseClient{}, stream.NewBroadcastHub())
	req := httptest.NewRequest(http.MethodGet, "/cases/"+streamTestCaseID+"/activities/stream", nil)
	req.SetPathValue("id", streamTestCaseID)
	w := httptest.NewRecorder()

	h.StreamCaseActivities(w, req)

	assertStatus(t, w, http.StatusUnauthorized)
}

func TestStreamCaseActivities_InvalidCaseID(t *testing.T) {
	h := NewStreamHandler(&mockEntityCaseClient{}, stream.NewBroadcastHub())
	req := withUser(httptest.NewRequest(http.MethodGet, "/cases/not-a-uuid/activities/stream", nil))
	req.SetPathValue("id", "not-a-uuid")
	w := httptest.NewRecorder()

	h.StreamCaseActivities(w, req)

	assertStatus(t, w, http.StatusBadRequest)
}

func TestStreamCaseActivities_EmptyCaseID(t *testing.T) {
	h := NewStreamHandler(&mockEntityCaseClient{}, stream.NewBroadcastHub())
	req := withUser(httptest.NewRequest(http.MethodGet, "/cases//activities/stream", nil))
	req.SetPathValue("id", "")
	w := httptest.NewRecorder()

	h.StreamCaseActivities(w, req)

	assertStatus(t, w, http.StatusBadRequest)
}

func TestStreamCaseActivities_HubNotConfigured(t *testing.T) {
	h := NewStreamHandler(&mockEntityCaseClient{}, nil)
	req := withUser(httptest.NewRequest(http.MethodGet, "/cases/"+streamTestCaseID+"/activities/stream", nil))
	req.SetPathValue("id", streamTestCaseID)
	w := httptest.NewRecorder()

	h.StreamCaseActivities(w, req)

	assertStatus(t, w, http.StatusServiceUnavailable)
}

// A caller with a valid token but no read access to the requested case must
// not be able to subscribe to it — see the GetCase authorization check added
// ahead of hub.Register.
func TestStreamCaseActivities_UnauthorizedCase(t *testing.T) {
	client := &mockEntityCaseClient{
		getCaseFn: func(ctx context.Context, caseID string) ([]byte, error) {
			return nil, &apierror.Error{StatusCode: http.StatusForbidden}
		},
	}
	hub := stream.NewBroadcastHub()
	h := NewStreamHandler(client, hub)
	req := withUser(httptest.NewRequest(http.MethodGet, "/cases/"+streamTestCaseID+"/activities/stream", nil))
	req.SetPathValue("id", streamTestCaseID)
	w := httptest.NewRecorder()

	h.StreamCaseActivities(w, req)

	assertStatus(t, w, http.StatusForbidden)
	if ct := w.Header().Get("Content-Type"); ct == "text/event-stream" {
		t.Error("stream headers were written for a case the caller cannot read")
	}
}

// syncRecorder is a minimal, mutex-protected http.ResponseWriter/http.Flusher
// used only by TestStreamCaseActivities_StreamsPublishedEvent below.
// httptest.ResponseRecorder's Body is a plain *bytes.Buffer with no internal
// locking, so a test that reads it from one goroutine while the handler
// under test writes to it from another (unavoidable here — StreamCaseActivities
// blocks for the life of the connection) trips the race detector make test
// runs under.
type syncRecorder struct {
	mu     sync.Mutex
	header http.Header
	status int
	body   bytes.Buffer
}

func newSyncRecorder() *syncRecorder {
	return &syncRecorder{header: http.Header{}, status: http.StatusOK}
}

func (r *syncRecorder) Header() http.Header { return r.header }

func (r *syncRecorder) WriteHeader(status int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.status = status
}

func (r *syncRecorder) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.body.Write(p)
}

func (r *syncRecorder) Flush() {}

func (r *syncRecorder) String() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.body.String()
}

func (r *syncRecorder) Status() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.status
}

func TestStreamCaseActivities_StreamsPublishedEvent(t *testing.T) {
	hub := stream.NewBroadcastHub()
	h := NewStreamHandler(&mockEntityCaseClient{}, hub)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx = middleware.WithUserInfo(ctx, testUser)
	req := httptest.NewRequest(http.MethodGet, "/cases/"+streamTestCaseID+"/activities/stream", nil).WithContext(ctx)
	req.SetPathValue("id", streamTestCaseID)
	w := newSyncRecorder()

	done := make(chan struct{})
	go func() {
		h.StreamCaseActivities(w, req)
		close(done)
	}()

	// StreamCaseActivities registers with the hub asynchronously relative to
	// this goroutine, so publish on a short interval until it lands (or the
	// deadline below fires) rather than racing a single Publish against an
	// unknown registration time.
	const wantPayload = `{"caseId":"` + streamTestCaseID + `","type":"case.comment_added"}`
	deadline := time.After(2 * time.Second)
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
waitForEvent:
	for {
		hub.Publish(streamTestCaseID, wantPayload)
		if strings.Contains(w.String(), "event: case_updated") {
			break waitForEvent
		}
		select {
		case <-ticker.C:
			continue
		case <-deadline:
			t.Fatal("timed out waiting for a case_updated event in the stream body")
		}
	}

	if w.Status() != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Status(), http.StatusOK)
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want %q", ct, "text/event-stream")
	}
	if !strings.Contains(w.String(), wantPayload) {
		t.Errorf("stream body missing published payload: %s", w.String())
	}

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not return after context cancellation")
	}
}

// ---- test helpers ----

func withUser(r *http.Request) *http.Request {
	return r.WithContext(middleware.WithUserInfo(r.Context(), testUser))
}

func assertStatus(t *testing.T, w *httptest.ResponseRecorder, want int) {
	t.Helper()
	if w.Code != want {
		t.Errorf("status = %d, want %d; body: %s", w.Code, want, w.Body.String())
	}
}
