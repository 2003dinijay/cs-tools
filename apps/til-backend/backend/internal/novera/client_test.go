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

package novera

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/til-backend/backend/internal/entry"
)

func ptr(s string) *string { return &s }

// newNovera fakes the gateway's token endpoint and Novera's notification
// endpoint, which answers with status and hands back what it received.
func newNovera(t *testing.T, status int) (*httptest.Server, <-chan map[string]any) {
	t.Helper()
	received := make(chan map[string]any, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"gateway-token","token_type":"Bearer","expires_in":3600}`))
	})
	mux.HandleFunc("POST /notify", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer gateway-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		received <- body
		w.WriteHeader(status)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, received
}

func newClient(srv *httptest.Server) *Client {
	return New(Config{NotifyURL: srv.URL + "/notify", TokenURL: srv.URL + "/token", ClientID: "til", ClientSecret: "s3cret"})
}

func TestNotifySendsTheEntry(t *testing.T) {
	srv, received := newNovera(t, http.StatusAccepted)
	e := entry.Entry{
		ID:          "e1",
		Title:       `Use <b> & "quotes"`,
		Who:         "Jane <Doe>",
		Where:       "Customer",
		WhereDetail: ptr("Acme & <Co>"),
		What:        "<p>Learned <strong>a lot</strong>.</p><ul><li>One</li></ul>",
	}
	newClient(srv).Notify(context.Background(), e, "https://one.example/knowledge-base/e1?view=article")

	// Title, who and whereDetail go as plain text; Novera escapes them.
	got := <-received
	want := map[string]any{
		"title":       `Use <b> & "quotes"`,
		"who":         "Jane <Doe>",
		"where":       "Customer",
		"whereDetail": "Acme & <Co>",
		"what":        "Learned <b>a lot</b>.<br>• One",
		"entryUrl":    "https://one.example/knowledge-base/e1?view=article",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %#v, want %#v", k, got[k], v)
		}
	}
}

func TestNotifySendsNullsForAbsentValues(t *testing.T) {
	srv, received := newNovera(t, http.StatusOK)
	newClient(srv).Notify(context.Background(), entry.Entry{ID: "e1", Title: "T", Who: "J", Where: "Internal", What: "<p>x</p>"}, "")
	got := <-received
	for _, k := range []string{"whereDetail", "entryUrl"} {
		if v, present := got[k]; !present || v != nil {
			t.Errorf("%s = %#v (present %v), want null", k, v, present)
		}
	}
}

func TestNotifySwallowsFailures(t *testing.T) {
	srv, received := newNovera(t, http.StatusInternalServerError)
	newClient(srv).Notify(context.Background(), entry.Entry{ID: "e1"}, "")
	<-received

	// Nothing listening at all.
	New(Config{NotifyURL: "http://127.0.0.1:1/notify", TokenURL: srv.URL + "/token", ClientID: "til", ClientSecret: "s"}).
		Notify(context.Background(), entry.Entry{ID: "e2"}, "")
}
