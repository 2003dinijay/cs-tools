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
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-portal-activity-stream-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-portal-activity-stream-service/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-portal-activity-stream-service/internal/stream"
)

const (
	multiCaseA = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	multiCaseB = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	multiCaseC = "cccccccc-cccc-cccc-cccc-cccccccccccc"
)

func multiReq(ids string) *http.Request {
	return withUser(httptest.NewRequest(http.MethodGet, "/cases/activities/stream?ids="+ids, nil))
}

func denyCases(denied map[string]int) *mockEntityCaseClient {
	return &mockEntityCaseClient{getCaseFn: func(_ context.Context, id string) ([]byte, error) {
		if code, ok := denied[id]; ok {
			return nil, &apierror.Error{StatusCode: code}
		}
		return []byte(`{}`), nil
	}}
}

func TestStreamMultiple_Unauthenticated(t *testing.T) {
	h := NewStreamHandler(&mockEntityCaseClient{}, stream.NewBroadcastHub())
	req := httptest.NewRequest(http.MethodGet, "/cases/activities/stream?ids="+multiCaseA, nil)
	w := httptest.NewRecorder()
	h.StreamMultipleCaseActivities(w, req)
	assertStatus(t, w, http.StatusUnauthorized)
}

func TestStreamMultiple_InvalidIDs(t *testing.T) {
	var tooMany []string
	for i := 0; i < maxMultiplexCaseIDs+1; i++ {
		tooMany = append(tooMany, "00000000-0000-0000-0000-0000000000"+string(rune('0'+i/10))+string(rune('0'+i%10)))
	}
	cases := map[string]string{
		"missing":      "",
		"blank":        "%20",
		"not uuid":     "not-a-uuid",
		"one bad":      multiCaseA + ",nope",
		"empty member": multiCaseA + ",," + multiCaseB,
		"too many":     strings.Join(tooMany, ","),
	}
	for name, ids := range cases {
		t.Run(name, func(t *testing.T) {
			h := NewStreamHandler(&mockEntityCaseClient{}, stream.NewBroadcastHub())
			w := httptest.NewRecorder()
			h.StreamMultipleCaseActivities(w, multiReq(ids))
			assertStatus(t, w, http.StatusBadRequest)
		})
	}
}

func TestParseCaseIDs_DedupeTrimAndLimit(t *testing.T) {
	ids, ok := parseCaseIDs(" " + multiCaseA + " ," + multiCaseB + "," + strings.ToUpper(multiCaseA))
	if !ok || len(ids) != 2 || ids[0] != multiCaseA || ids[1] != multiCaseB {
		t.Fatalf("got %v ok=%v, want [A B]", ids, ok)
	}
	// Exactly the cap is fine, even when repeated entries push the raw count over it.
	var parts []string
	for i := 0; i < maxMultiplexCaseIDs; i++ {
		parts = append(parts, "00000000-0000-0000-0000-0000000000"+string(rune('0'+i/10))+string(rune('0'+i%10)))
	}
	raw := strings.Join(parts, ",") + "," + parts[0]
	if ids, ok := parseCaseIDs(raw); !ok || len(ids) != maxMultiplexCaseIDs {
		t.Fatalf("got %d ids ok=%v, want %d", len(ids), ok, maxMultiplexCaseIDs)
	}
}

func TestStreamMultiple_HubNotConfigured(t *testing.T) {
	h := NewStreamHandler(&mockEntityCaseClient{}, nil)
	w := httptest.NewRecorder()
	h.StreamMultipleCaseActivities(w, multiReq(multiCaseA))
	assertStatus(t, w, http.StatusServiceUnavailable)
}

func TestStreamMultiple_AllUnauthorized_ReturnsMappedError(t *testing.T) {
	h := NewStreamHandler(denyCases(map[string]int{multiCaseA: 403, multiCaseB: 404}), stream.NewBroadcastHub())
	w := httptest.NewRecorder()
	h.StreamMultipleCaseActivities(w, multiReq(multiCaseA+","+multiCaseB))
	assertStatus(t, w, http.StatusForbidden) // first in request order
	if w.Header().Get("Content-Type") == "text/event-stream" {
		t.Error("stream headers written although no id is authorized")
	}
}

func TestStreamMultiple_TransientUpstreamErrorFailsRequest(t *testing.T) {
	h := NewStreamHandler(denyCases(map[string]int{multiCaseB: 503}), stream.NewBroadcastHub())
	w := httptest.NewRecorder()
	h.StreamMultipleCaseActivities(w, multiReq(multiCaseA+","+multiCaseB))
	assertStatus(t, w, http.StatusServiceUnavailable)
}

// runMulti starts the handler on a cancellable context and returns its recorder.
func runMulti(t *testing.T, h *StreamHandler, ids string) (*syncRecorder, context.CancelFunc, chan struct{}) {
	t.Helper()
	ctx, cancel := context.WithCancel(middleware.WithUserInfo(context.Background(), testUser))
	req := httptest.NewRequest(http.MethodGet, "/cases/activities/stream?ids="+ids, nil).WithContext(ctx)
	w := newSyncRecorder()
	done := make(chan struct{})
	go func() {
		h.StreamMultipleCaseActivities(w, req)
		close(done)
	}()
	return w, cancel, done
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestStreamMultiple_TwoCasesOneStream_AndCleanup(t *testing.T) {
	hub := stream.NewBroadcastHub()
	h := NewStreamHandler(&mockEntityCaseClient{}, hub)
	baseline := runtime.NumGoroutine()
	w, cancel, done := runMulti(t, h, multiCaseA+","+multiCaseB+","+multiCaseA)
	defer cancel()

	payloadA := `{"caseId":"` + multiCaseA + `","type":"case.comment_added"}`
	payloadB := `{"caseId":"` + multiCaseB + `","type":"case.status_changed"}`
	waitFor(t, "both frames", func() bool {
		hub.Publish(multiCaseA, payloadA)
		hub.Publish(multiCaseB, payloadB)
		b := w.String()
		return strings.Contains(b, payloadA) && strings.Contains(b, payloadB)
	})
	if got := strings.Count(w.String(), "event: case_updated\ndata: "); got < 2 {
		t.Errorf("want at least 2 framed events, got %d: %s", got, w.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q", ct)
	}

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not return after context cancellation")
	}

	// After disconnect both cases must have no subscribers: a new registration
	// must be the only one, i.e. publishing reaches exactly it and nothing else
	// panics (send on closed channel) or lingers.
	for _, id := range []string{multiCaseA, multiCaseB} {
		ch := hub.Register(id)
		hub.Publish(id, "x")
		if got := <-ch; got != "x" {
			t.Errorf("probe got %q", got)
		}
		hub.Unregister(id, ch)
	}
	// Forwarder goroutines (one per id) must be gone, not parked on a channel.
	waitFor(t, "goroutines to drain", func() bool { return runtime.NumGoroutine() <= baseline })
	before := w.String()
	hub.Publish(multiCaseA, "late")
	time.Sleep(20 * time.Millisecond)
	if w.String() != before {
		t.Error("stream wrote after handler returned")
	}
}

func TestStreamMultiple_PartialAuthorization_DropsUnauthorized(t *testing.T) {
	hub := stream.NewBroadcastHub()
	h := NewStreamHandler(denyCases(map[string]int{multiCaseB: 403, multiCaseC: 404}), hub)
	w, cancel, done := runMulti(t, h, multiCaseA+","+multiCaseB+","+multiCaseC)
	defer cancel()

	payloadA := `{"caseId":"` + multiCaseA + `"}`
	waitFor(t, "authorized frame", func() bool {
		hub.Publish(multiCaseA, payloadA)
		return strings.Contains(w.String(), payloadA)
	})
	for i := 0; i < 20; i++ {
		hub.Publish(multiCaseB, `{"caseId":"`+multiCaseB+`"}`)
		hub.Publish(multiCaseC, `{"caseId":"`+multiCaseC+`"}`)
	}
	time.Sleep(50 * time.Millisecond)
	if b := w.String(); strings.Contains(b, multiCaseB) || strings.Contains(b, multiCaseC) {
		t.Errorf("events for unauthorized cases leaked: %s", b)
	}
	if w.Status() != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Status())
	}
	cancel()
	<-done
}

func TestStreamMultiple_MuxRoutesDoNotConflict(t *testing.T) {
	hub := stream.NewBroadcastHub()
	h := NewStreamHandler(&mockEntityCaseClient{}, hub)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /cases/{id}/activities/stream", h.StreamCaseActivities)
	mux.HandleFunc("GET /cases/activities/stream", h.StreamMultipleCaseActivities)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mux.ServeHTTP(w, r.WithContext(middleware.WithUserInfo(r.Context(), testUser)))
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/cases/activities/stream?ids="+multiCaseA+","+multiCaseB, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}

	payloadA := `{"caseId":"` + multiCaseA + `","type":"case.comment_added"}`
	payloadB := `{"caseId":"` + multiCaseB + `","type":"case.status_changed"}`
	frames := make(chan string, 8)
	go func() {
		sc := bufio.NewScanner(resp.Body)
		for sc.Scan() {
			if strings.HasPrefix(sc.Text(), "data: ") {
				frames <- strings.TrimPrefix(sc.Text(), "data: ")
			}
		}
		close(frames)
	}()
	got := map[string]bool{}
	deadline := time.After(2 * time.Second)
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for len(got) < 2 {
		select {
		case f := <-frames:
			got[f] = true
		case <-tick.C:
			hub.Publish(multiCaseA, payloadA)
			hub.Publish(multiCaseB, payloadB)
		case <-deadline:
			t.Fatalf("frames received: %v", got)
		}
	}
	if !got[payloadA] || !got[payloadB] {
		t.Errorf("missing frame: %v", got)
	}
	cancel()
}
