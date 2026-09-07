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
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/middleware"
)

// TestUserIDTokenFromRequest covers the Sec-WebSocket-Protocol token smuggling
// the frontend relies on, because a browser cannot set a custom header on a
// WebSocket handshake. The "via Choreo" case is the one that actually runs in
// a deployed environment — the gateway strips the leading
// "choreo-oauth2-token, <accessToken>" pair before the request reaches here.
func TestUserIDTokenFromRequest(t *testing.T) {
	tests := map[string]struct {
		header    string
		protocols string
		want      string
	}{
		"header wins when present": {
			header:    "header-token",
			protocols: "cs-customer-portal, protocol-token",
			want:      "header-token",
		},
		"via Choreo — gateway already stripped the oauth2 pair": {
			protocols: "cs-customer-portal, user-id-token",
			want:      "user-id-token",
		},
		"direct connection — full offer from the browser": {
			protocols: "choreo-oauth2-token, access-token, cs-customer-portal, user-id-token",
			want:      "user-id-token",
		},
		"no padding around the separator": {
			protocols: "cs-customer-portal,user-id-token",
			want:      "user-id-token",
		},
		"subprotocol alone carries no token": {
			protocols: "cs-customer-portal",
			want:      "",
		},
		"nothing at all": {
			want: "",
		},
		"blank header falls through to the subprotocol": {
			header:    "   ",
			protocols: "cs-customer-portal, user-id-token",
			want:      "user-id-token",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/ws?sessionId=x", nil)
			if tc.header != "" {
				r.Header.Set(userIDTokenHeader, tc.header)
			}
			if tc.protocols != "" {
				r.Header.Set("Sec-WebSocket-Protocol", tc.protocols)
			}
			if got := userIDTokenFromRequest(r); got != tc.want {
				t.Errorf("userIDTokenFromRequest() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestUserIDTokenFromRequest_RepeatedHeaders proves a client that splits its
// offer across repeated headers is read the same as a single comma-separated
// one — the two forms are equivalent on the wire.
func TestUserIDTokenFromRequest_RepeatedHeaders(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/ws?sessionId=x", nil)
	r.Header.Add("Sec-WebSocket-Protocol", "cs-customer-portal")
	r.Header.Add("Sec-WebSocket-Protocol", "user-id-token")

	if got := userIDTokenFromRequest(r); got != "user-id-token" {
		t.Errorf("userIDTokenFromRequest() = %q, want %q", got, "user-id-token")
	}
}

// TestBuildUpstreamPayload guards the fields the AI agent reads straight off
// the message. accountId in particular is required by the agent — it uses it
// for the session, the per-account token budget, and analytics — and an
// earlier field whitelist dropped it, which made every chat message fail with
// "accountId is required".
func TestBuildUpstreamPayload(t *testing.T) {
	const serverConvID = "11111111-1111-1111-1111-111111111111"
	const serverAccountID = "22222222-2222-2222-2222-222222222222"

	t.Run("forwards type, message and envProducts untouched", func(t *testing.T) {
		parsed := map[string]any{
			"type":        "user_message",
			"message":     "hi",
			"envProducts": map[string]any{"dep-1": []any{"apim"}},
		}

		raw, err := buildUpstreamPayload(parsed, serverConvID, serverAccountID)
		if err != nil {
			t.Fatalf("buildUpstreamPayload returned error: %v", err)
		}

		var got map[string]any
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("result is not valid JSON: %v", err)
		}
		if got["type"] != "user_message" {
			t.Errorf("type = %v, want user_message", got["type"])
		}
		if got["message"] != "hi" {
			t.Errorf("message = %v, want hi", got["message"])
		}
		if _, ok := got["envProducts"]; !ok {
			t.Error("envProducts was dropped")
		}
		// accountId must be present even though the client sent none — the
		// agent rejects the message outright without it.
		if got["accountId"] != serverAccountID {
			t.Errorf("accountId = %v, want the server-resolved %v", got["accountId"], serverAccountID)
		}
	})

	// The security-relevant case: a caller naming someone else's account must
	// not have that account's token budget or analytics charged.
	t.Run("server accountId overrides a client-supplied one", func(t *testing.T) {
		raw, err := buildUpstreamPayload(map[string]any{
			"accountId": "someone-elses-account",
			"message":   "hi",
		}, serverConvID, serverAccountID)
		if err != nil {
			t.Fatalf("buildUpstreamPayload returned error: %v", err)
		}

		var got map[string]any
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("result is not valid JSON: %v", err)
		}
		if got["accountId"] != serverAccountID {
			t.Errorf("accountId = %v, want the server-resolved %v — a client value must never win",
				got["accountId"], serverAccountID)
		}
	})

	t.Run("server conversationId overrides the client's", func(t *testing.T) {
		raw, err := buildUpstreamPayload(map[string]any{
			"conversationId": "whatever-the-client-said",
			"message":        "hi",
		}, serverConvID, serverAccountID)
		if err != nil {
			t.Fatalf("buildUpstreamPayload returned error: %v", err)
		}

		var got map[string]any
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("result is not valid JSON: %v", err)
		}
		if got["conversationId"] != serverConvID {
			t.Errorf("conversationId = %v, want the server-resolved %v", got["conversationId"], serverConvID)
		}
	})

	t.Run("does not mutate the caller's map", func(t *testing.T) {
		parsed := map[string]any{"conversationId": "client-value", "message": "hi"}

		if _, err := buildUpstreamPayload(parsed, serverConvID, serverAccountID); err != nil {
			t.Fatalf("buildUpstreamPayload returned error: %v", err)
		}
		if parsed["conversationId"] != "client-value" {
			t.Errorf("input map was mutated: conversationId = %v", parsed["conversationId"])
		}
	})

	t.Run("nil input does not panic", func(t *testing.T) {
		raw, err := buildUpstreamPayload(nil, serverConvID, serverAccountID)
		if err != nil {
			t.Fatalf("buildUpstreamPayload returned error: %v", err)
		}

		var got map[string]any
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("result is not valid JSON: %v", err)
		}
		if got["conversationId"] != serverConvID {
			t.Errorf("conversationId = %v, want %v", got["conversationId"], serverConvID)
		}
	})
}

// stubValidator is a wsTokenValidator that accepts exactly one token.
type stubValidator struct {
	accept string
	called int
}

func (s *stubValidator) DecodeUnverified(token string) (*middleware.UserInfo, error) {
	s.called++
	if token != s.accept {
		return nil, errors.New("invalid token")
	}
	return &middleware.UserInfo{UserID: "user-1", Email: "u@example.com"}, nil
}

// TestHandleWebSocket_RejectsUnauthenticated asserts the upgrade never happens
// without a valid token — the handler must answer with a normal HTTP error
// rather than completing the handshake, since it runs on a listener that has
// no Auth middleware in front of it.
func TestHandleWebSocket_RejectsUnauthenticated(t *testing.T) {
	tests := map[string]struct {
		protocols  string
		wantStatus int
		wantCalls  int
	}{
		"no token at all": {
			wantStatus: http.StatusUnauthorized,
			wantCalls:  0,
		},
		"token present but not valid": {
			protocols:  "cs-customer-portal, wrong-token",
			wantStatus: http.StatusUnauthorized,
			wantCalls:  1,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			v := &stubValidator{accept: "good-token"}
			h := NewWebSocketHandler(nil, nil, v, nil)

			r := httptest.NewRequest(http.MethodGet, "/ws?sessionId=11111111-1111-1111-1111-111111111111", nil)
			if tc.protocols != "" {
				r.Header.Set("Sec-WebSocket-Protocol", tc.protocols)
			}
			w := httptest.NewRecorder()

			h.HandleWebSocket(w, r)

			if w.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tc.wantStatus)
			}
			if v.called != tc.wantCalls {
				t.Errorf("DecodeUnverified called %d times, want %d", v.called, tc.wantCalls)
			}
		})
	}
}

// stubEntity is an entityCommentCreator whose GetProject result is scripted.
type stubEntity struct {
	project entity.ProjectDetailsView
	err     error
	calls   int

	// conversation/conversationErr script GetConversation, which handleMessage
	// consults before downgrading a CONVERTED conversation to RESOLVED.
	conversation    entity.ConversationDetails
	conversationErr error
	// updatedStates records every state UpdateConversation was asked to set, so
	// a test can assert the transition was skipped rather than merely reordered.
	updatedStates []string
}

func (s *stubEntity) GetProject(_ context.Context, _ string) (entity.ProjectDetailsView, error) {
	s.calls++
	return s.project, s.err
}

func (s *stubEntity) CreateComment(_ context.Context, _ entity.CreateCommentRequest) (entity.CreateCommentResponse, error) {
	return entity.CreateCommentResponse{}, nil
}

func (s *stubEntity) UpdateConversation(_ context.Context, _ string, req entity.UpdateConversationRequest) (entity.UpdateConversationResponse, error) {
	s.updatedStates = append(s.updatedStates, req.State)
	return entity.UpdateConversationResponse{}, nil
}

func (s *stubEntity) GetConversation(_ context.Context, _ string) (entity.ConversationDetails, error) {
	return s.conversation, s.conversationErr
}

// TestHandleWebSocket_ProjectAccessGatesTheUpgrade asserts the account lookup
// doubles as an authorization gate: a project the caller can't see must be
// rejected with an HTTP status *before* any upgrade happens, not closed after.
func TestHandleWebSocket_ProjectAccessGatesTheUpgrade(t *testing.T) {
	v := &stubValidator{accept: "good-token"}
	e := &stubEntity{err: apierror.NewUpstreamError(http.StatusForbidden, nil)}
	h := NewWebSocketHandler(nil, e, v, nil)

	r := httptest.NewRequest(http.MethodGet, "/ws?sessionId=11111111-1111-1111-1111-111111111111", nil)
	r.Header.Set("Sec-WebSocket-Protocol", "cs-customer-portal, good-token")
	w := httptest.NewRecorder()

	h.HandleWebSocket(w, r)

	if e.calls != 1 {
		t.Errorf("GetProject called %d times, want 1", e.calls)
	}
	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
	// A completed upgrade would have written 101 and hijacked the connection.
	if w.Code == http.StatusSwitchingProtocols {
		t.Error("connection was upgraded despite the project being inaccessible")
	}
}

// TestHandleWebSocket_ValidatesSessionIDAfterAuth pins the ordering: an
// authenticated caller with a malformed sessionId gets 400, not 401.
func TestHandleWebSocket_ValidatesSessionIDAfterAuth(t *testing.T) {
	v := &stubValidator{accept: "good-token"}
	h := NewWebSocketHandler(nil, nil, v, nil)

	r := httptest.NewRequest(http.MethodGet, "/ws?sessionId=not-a-uuid", nil)
	r.Header.Set("Sec-WebSocket-Protocol", "cs-customer-portal, good-token")
	w := httptest.NewRecorder()

	h.HandleWebSocket(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}
